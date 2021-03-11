package state

import (
	"sort"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

// changesetHistory is a collection of external changeset states
// (open/closed/merged state and review state) over time.
type changesetHistory []changesetStatesAtTime

// StatesAtTime returns the changeset's states valid at the given time. If the
// changeset didn't exist yet, the second parameter is false.
func (h changesetHistory) StatesAtTime(t time.Time) (changesetStatesAtTime, bool) {
	if len(h) == 0 {
		return changesetStatesAtTime{}, false
	}

	var (
		states changesetStatesAtTime
		found  bool
	)

	for _, s := range h {
		if s.t.After(t) {
			break
		}
		states = s
		found = true
	}

	return states, found
}

// RequiredEventTypesForHistory keeps track of all event kinds required for calculating the history of a changeset.
//
// We specifically ignore ChangesetEventKindGitHubReviewDismissed
// events since GitHub updates the original
// ChangesetEventKindGitHubReviewed event when a review has been
// dismissed.
// See: https://github.com/sourcegraph/sourcegraph/pull/9461
var RequiredEventTypesForHistory = []batches.ChangesetEventKind{
	batches.ChangesetEventKindGitLabUnmarkWorkInProgress,
	batches.ChangesetEventKindGitHubReadyForReview,
	batches.ChangesetEventKindGitLabMarkWorkInProgress,
	batches.ChangesetEventKindGitHubConvertToDraft,
	batches.ChangesetEventKindGitHubClosed,
	batches.ChangesetEventKindBitbucketServerDeclined,
	batches.ChangesetEventKindGitLabClosed,
	batches.ChangesetEventKindGitHubMerged,
	batches.ChangesetEventKindBitbucketServerMerged,
	batches.ChangesetEventKindGitLabMerged,
	batches.ChangesetEventKindGitHubReopened,
	batches.ChangesetEventKindBitbucketServerReopened,
	batches.ChangesetEventKindGitLabReopened,
	batches.ChangesetEventKindGitHubReviewed,
	batches.ChangesetEventKindBitbucketServerApproved,
	batches.ChangesetEventKindBitbucketServerReviewed,
	batches.ChangesetEventKindGitLabApproved,
	batches.ChangesetEventKindBitbucketServerUnapproved,
	batches.ChangesetEventKindBitbucketServerDismissed,
	batches.ChangesetEventKindGitLabUnapproved,
}

type changesetStatesAtTime struct {
	t             time.Time
	externalState batches.ChangesetExternalState
	reviewState   batches.ChangesetReviewState
}

// computeHistory calculates the changesetHistory for the given Changeset and
// its ChangesetEvents.
// The ChangesetEvents MUST be sorted by their Timestamp.
func computeHistory(ch *batches.Changeset, ce ChangesetEvents) (changesetHistory, error) {
	if !sort.IsSorted(ce) {
		return nil, errors.New("changeset events not sorted")
	}

	var (
		states = []changesetStatesAtTime{}

		currentExtState    = initialExternalState(ch, ce)
		currentReviewState = batches.ChangesetReviewStatePending

		lastReviewByAuthor = map[string]batches.ChangesetReviewState{}
		// The draft state is tracked alongside the "external state" on GitHub and GitLab,
		// that means we need to take changes to this state into account separately. On reopen,
		// we cannot simply say it's open, because it could be it was converted to a draft while
		// it was closed. Hence, we need to track the state using this variable.
		isDraft = currentExtState == batches.ChangesetExternalStateDraft
	)

	pushStates := func(t time.Time) {
		states = append(states, changesetStatesAtTime{
			t:             t,
			externalState: currentExtState,
			reviewState:   currentReviewState,
		})
	}

	openedAt := ch.ExternalCreatedAt()
	if openedAt.IsZero() {
		return nil, errors.New("changeset ExternalCreatedAt has zero value")
	}
	pushStates(openedAt)

	for _, e := range ce {
		et := e.Timestamp()
		if et.IsZero() {
			continue
		}

		// NOTE: If you add any kinds here, make sure they also appear in `RequiredEventTypesForHistory`.
		switch e.Kind {
		case batches.ChangesetEventKindGitHubClosed,
			batches.ChangesetEventKindBitbucketServerDeclined,
			batches.ChangesetEventKindGitLabClosed:
			// Merged is a final state. We can ignore everything after.
			if currentExtState != batches.ChangesetExternalStateMerged {
				currentExtState = batches.ChangesetExternalStateClosed
				pushStates(et)
			}

		case batches.ChangesetEventKindGitHubMerged,
			batches.ChangesetEventKindBitbucketServerMerged,
			batches.ChangesetEventKindGitLabMerged:
			currentExtState = batches.ChangesetExternalStateMerged
			pushStates(et)

		case batches.ChangesetEventKindGitLabMarkWorkInProgress:
			isDraft = true
			// This event only matters when the changeset is open, otherwise a change in the title won't change the overall external state.
			if currentExtState == batches.ChangesetExternalStateOpen {
				currentExtState = batches.ChangesetExternalStateDraft
				pushStates(et)
			}

		case batches.ChangesetEventKindGitHubConvertToDraft:
			isDraft = true
			// Merged is a final state. We can ignore everything after.
			if currentExtState != batches.ChangesetExternalStateMerged {
				currentExtState = batches.ChangesetExternalStateDraft
				pushStates(et)
			}

		case batches.ChangesetEventKindGitLabUnmarkWorkInProgress,
			batches.ChangesetEventKindGitHubReadyForReview:
			isDraft = false
			// This event only matters when the changeset is open, otherwise a change in the title won't change the overall external state.
			if currentExtState == batches.ChangesetExternalStateDraft {
				currentExtState = batches.ChangesetExternalStateOpen
				pushStates(et)
			}

		case batches.ChangesetEventKindGitHubReopened,
			batches.ChangesetEventKindBitbucketServerReopened,
			batches.ChangesetEventKindGitLabReopened:
			// Merged is a final state. We can ignore everything after.
			if currentExtState != batches.ChangesetExternalStateMerged {
				if isDraft {
					currentExtState = batches.ChangesetExternalStateDraft
				} else {
					currentExtState = batches.ChangesetExternalStateOpen
				}
				pushStates(et)
			}

		case batches.ChangesetEventKindGitHubReviewed,
			batches.ChangesetEventKindBitbucketServerApproved,
			batches.ChangesetEventKindBitbucketServerReviewed,
			batches.ChangesetEventKindGitLabApproved:

			s, err := e.ReviewState()
			if err != nil {
				return nil, err
			}

			// We only care about "Approved", "ChangesRequested" or "Dismissed" reviews
			if s != batches.ChangesetReviewStateApproved &&
				s != batches.ChangesetReviewStateChangesRequested &&
				s != batches.ChangesetReviewStateDismissed {
				continue
			}

			author := e.ReviewAuthor()
			// If the user has been deleted, skip their reviews, as they don't count towards the final state anymore.
			if author == "" {
				continue
			}

			// Save current review state, then insert new review or delete
			// dismissed review, then recompute overall review state
			oldReviewState := currentReviewState

			if s == batches.ChangesetReviewStateDismissed {
				// In case of a dismissed review we dismiss _all_ of the
				// previous reviews by the author, since that is what GitHub
				// does in its UI.
				delete(lastReviewByAuthor, author)
			} else {
				lastReviewByAuthor[author] = s
			}

			newReviewState := reduceReviewStates(lastReviewByAuthor)

			if newReviewState != oldReviewState {
				currentReviewState = newReviewState
				pushStates(et)
			}

		case batches.ChangesetEventKindBitbucketServerUnapproved,
			batches.ChangesetEventKindBitbucketServerDismissed,
			batches.ChangesetEventKindGitLabUnapproved:
			author := e.ReviewAuthor()
			// If the user has been deleted, skip their reviews, as they don't count towards the final state anymore.
			if author == "" {
				continue
			}

			if e.Type() == batches.ChangesetEventKindBitbucketServerUnapproved {
				// A BitbucketServer Unapproved can only follow a previous Approved by
				// the same author.
				lastReview, ok := lastReviewByAuthor[author]
				if !ok || lastReview != batches.ChangesetReviewStateApproved {
					log15.Warn("Bitbucket Server Unapproval not following an Approval", "event", e)
					continue
				}
			}

			if e.Type() == batches.ChangesetEventKindBitbucketServerDismissed {
				// A BitbucketServer Dismissed event can only follow a previous "Changes Requested" review by
				// the same author.
				lastReview, ok := lastReviewByAuthor[author]
				if !ok || lastReview != batches.ChangesetReviewStateChangesRequested {
					log15.Warn("Bitbucket Server Dismissal not following a Review", "event", e)
					continue
				}
			}

			// Save current review state, then remove last approval and
			// recompute overall review state
			oldReviewState := currentReviewState
			delete(lastReviewByAuthor, author)
			newReviewState := reduceReviewStates(lastReviewByAuthor)

			if newReviewState != oldReviewState {
				currentReviewState = newReviewState
				pushStates(et)
			}
		}
	}

	// We don't have an event for the deletion of a Changeset, but we set
	// ExternalDeletedAt manually in the Syncer.
	deletedAt := ch.ExternalDeletedAt
	if !deletedAt.IsZero() {
		currentExtState = batches.ChangesetExternalStateClosed
		pushStates(deletedAt)
	}

	return states, nil
}

// reduceReviewStates reduces the given a map of review per author down to a
// single overall ChangesetReviewState.
func reduceReviewStates(statesByAuthor map[string]batches.ChangesetReviewState) batches.ChangesetReviewState {
	states := make(map[batches.ChangesetReviewState]bool)
	for _, s := range statesByAuthor {
		states[s] = true
	}
	return selectReviewState(states)
}

// initialExternalState infers from the changeset state and the list of events in which
// ChangesetExternalState the changeset must have been when it has been created.
func initialExternalState(ch *batches.Changeset, ce ChangesetEvents) batches.ChangesetExternalState {
	open := true
	switch m := ch.Metadata.(type) {
	case *github.PullRequest:
		if m.IsDraft {
			open = false
		}

	case *gitlab.MergeRequest:
		if m.WorkInProgress {
			open = false
		}
	default:
		return batches.ChangesetExternalStateOpen
	}
	// Walk the events backwards, since we need to look from the current time to the past.
	for i := len(ce) - 1; i >= 0; i-- {
		e := ce[i]
		switch e.Metadata.(type) {
		case *gitlab.UnmarkWorkInProgressEvent, *github.ReadyForReviewEvent:
			open = false
		case *gitlab.MarkWorkInProgressEvent, *github.ConvertToDraftEvent:
			open = true
		}
	}
	if open {
		return batches.ChangesetExternalStateOpen
	}
	return batches.ChangesetExternalStateDraft
}
