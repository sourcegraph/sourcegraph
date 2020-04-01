package campaigns

import (
	"sort"
	"time"

	cmpgn "github.com/sourcegraph/sourcegraph/internal/campaigns"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
)

// ChangesetEvents is a collection of changeset events
type ChangesetEvents []*cmpgn.ChangesetEvent

func (ce ChangesetEvents) Len() int      { return len(ce) }
func (ce ChangesetEvents) Swap(i, j int) { ce[i], ce[j] = ce[j], ce[i] }

// Less sorts changeset events by their Timestamps
func (ce ChangesetEvents) Less(i, j int) bool {
	return ce[i].Timestamp().Before(ce[j].Timestamp())
}

// reviewState returns the overall review state of the review events in the
// slice.
// It should only be called by ComputeChangesetReviewState.
func (ce ChangesetEvents) reviewState() (cmpgn.ChangesetReviewState, error) {
	reviewsByAuthor := map[string]cmpgn.ChangesetReviewState{}

	for _, e := range ce {
		author, err := e.ReviewAuthor()
		if err != nil {
			return "", err
		}
		if author == "" {
			continue
		}
		s, err := e.ReviewState()
		if err != nil {
			return "", err
		}

		switch s {
		case cmpgn.ChangesetReviewStateApproved,
			cmpgn.ChangesetReviewStateChangesRequested:
			reviewsByAuthor[author] = s
		case cmpgn.ChangesetReviewStateDismissed:
			delete(reviewsByAuthor, author)
		}
	}

	return computeReviewState(reviewsByAuthor), nil
}

// State returns the  state of the changeset to which the events belong and assumes the events
// are sorted by ChangesetEvent.Timestamp().
func (ce ChangesetEvents) State() cmpgn.ChangesetState {
	state := cmpgn.ChangesetStateOpen
	for _, e := range ce {
		switch e.Kind {
		case cmpgn.ChangesetEventKindGitHubClosed, cmpgn.ChangesetEventKindBitbucketServerDeclined:
			state = cmpgn.ChangesetStateClosed
		case cmpgn.ChangesetEventKindGitHubMerged, cmpgn.ChangesetEventKindBitbucketServerMerged:
			// Merged is a final state. We can ignore everything after.
			return cmpgn.ChangesetStateMerged
		case cmpgn.ChangesetEventKindGitHubReopened, cmpgn.ChangesetEventKindBitbucketServerReopened:
			state = cmpgn.ChangesetStateOpen
		}
	}
	return state
}

// UpdateLabelsSince returns the set of current labels based the starting set of labels and looking at events
// that have occurred after "since".
func (ce *ChangesetEvents) UpdateLabelsSince(cs *cmpgn.Changeset) []cmpgn.ChangesetLabel {
	var current []cmpgn.ChangesetLabel
	var since time.Time
	if cs != nil {
		current = cs.Labels()
		since = cs.UpdatedAt
	}
	// Copy slice so that we don't mutate ce
	sorted := make(ChangesetEvents, len(*ce))
	copy(sorted, *ce)
	sort.Sort(sorted)

	// Iterate through all label events to get the current set
	set := make(map[string]cmpgn.ChangesetLabel)
	for _, l := range current {
		set[l.Name] = l
	}
	for _, event := range sorted {
		switch e := event.Metadata.(type) {
		case *github.LabelEvent:
			if e.CreatedAt.Before(since) {
				continue
			}
			if e.Removed {
				delete(set, e.Label.Name)
				continue
			}
			set[e.Label.Name] = cmpgn.ChangesetLabel{
				Name:        e.Label.Name,
				Color:       e.Label.Color,
				Description: e.Label.Description,
			}
		}
	}
	labels := make([]cmpgn.ChangesetLabel, 0, len(set))
	for _, label := range set {
		labels = append(labels, label)
	}
	return labels
}
