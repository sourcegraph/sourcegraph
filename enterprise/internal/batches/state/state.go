package state

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

// SetDerivedState will update the external state fields on the Changeset based
// on the current state of the changeset and associated events.
func SetDerivedState(ctx context.Context, repoStore *database.RepoStore, c *batches.Changeset, es []*batches.ChangesetEvent) {
	// Copy so that we can sort without mutating the argument
	events := make(ChangesetEvents, len(es))
	copy(events, es)
	sort.Sort(events)

	c.ExternalCheckState = computeCheckState(c, events)

	history, err := computeHistory(c, events)
	if err != nil {
		log15.Warn("Computing changeset history", "err", err)
		return
	}

	if state, err := computeExternalState(c, history); err != nil {
		log15.Warn("Computing external changeset state", "err", err)
	} else {
		c.ExternalState = state
	}
	if state, err := computeReviewState(c, history); err != nil {
		log15.Warn("Computing changeset review state", "err", err)
	} else {
		c.ExternalReviewState = state
	}

	// If the changeset was "complete" (that is, not open) the last time we
	// synced, and it's still complete, then we don't need to do any further
	// work: the diffstat should still be correct, and this way we don't need to
	// rely on gitserver having the head OID still available.
	if c.SyncState.IsComplete && c.ExternalState != batches.ChangesetExternalStateOpen {
		return
	}

	// Some of the fields on changesets are dependent on the SyncState: this
	// encapsulates fields that we want to cache based on our current
	// understanding of the changeset's state on the external provider that are
	// not part of the metadata that we get from the provider's API.
	//
	// To update this, first we need gitserver's view of the repo.
	repo, err := changesetRepoName(ctx, repoStore, c)
	if err != nil {
		log15.Warn("Retrieving repo name for changeset's repo", "err", err)
		return
	}

	// Now we can update the state. Since we'll want to only perform some
	// actions based on how the state changes, we'll keep references to the old
	// and new states for the duration of this function, although we'll update
	// c.SyncState as soon as we can.
	oldState := c.SyncState
	newState, err := computeSyncState(ctx, c, repo)
	if err != nil {
		log15.Warn("Computing sync state", "err", err)
		return
	}
	c.SyncState = *newState

	// Now we can update fields that are invalidated when the sync state
	// changes.
	if !oldState.Equals(newState) {
		if stat, err := computeDiffStat(ctx, c, repo); err != nil {
			log15.Warn("Computing diffstat", "err", err)
		} else {
			c.SetDiffStat(stat)
		}
	}
}

// computeCheckState computes the overall check state based on the current
// synced check state and any webhook events that have arrived after the most
// recent sync.
func computeCheckState(c *batches.Changeset, events ChangesetEvents) batches.ChangesetCheckState {
	switch m := c.Metadata.(type) {
	case *github.PullRequest:
		return computeGitHubCheckState(c.UpdatedAt, m, events)

	case *bitbucketserver.PullRequest:
		return computeBitbucketBuildStatus(c.UpdatedAt, m, events)

	case *gitlab.MergeRequest:
		return computeGitLabCheckState(c.UpdatedAt, m, events)
	}

	return batches.ChangesetCheckStateUnknown
}

// computeExternalState computes the external state for the changeset and its
// associated events.
func computeExternalState(c *batches.Changeset, history []changesetStatesAtTime) (batches.ChangesetExternalState, error) {
	if len(history) == 0 {
		return computeSingleChangesetExternalState(c)
	}
	newestDataPoint := history[len(history)-1]
	if c.UpdatedAt.After(newestDataPoint.t) {
		return computeSingleChangesetExternalState(c)
	}
	return newestDataPoint.externalState, nil
}

// computeReviewState computes the review state for the changeset and its
// associated events. The events should be presorted.
func computeReviewState(c *batches.Changeset, history []changesetStatesAtTime) (batches.ChangesetReviewState, error) {
	if len(history) == 0 {
		return computeSingleChangesetReviewState(c)
	}

	newestDataPoint := history[len(history)-1]

	// GitHub only stores the ReviewState in events, we can't look at the
	// Changeset.
	if c.ExternalServiceType == extsvc.TypeGitHub {
		return newestDataPoint.reviewState, nil
	}

	// For other codehosts we check whether the Changeset is newer or the
	// events and use the newest entity to get the reviewstate.
	if c.UpdatedAt.After(newestDataPoint.t) {
		return computeSingleChangesetReviewState(c)
	}
	return newestDataPoint.reviewState, nil
}

func computeBitbucketBuildStatus(lastSynced time.Time, pr *bitbucketserver.PullRequest, events []*batches.ChangesetEvent) batches.ChangesetCheckState {
	var latestCommit bitbucketserver.Commit
	for _, c := range pr.Commits {
		if latestCommit.CommitterTimestamp <= c.CommitterTimestamp {
			latestCommit = *c
		}
	}

	stateMap := make(map[string]batches.ChangesetCheckState)

	// States from last sync
	for _, status := range pr.CommitStatus {
		stateMap[status.Key()] = parseBitbucketBuildState(status.Status.State)
	}

	// Add any events we've received since our last sync
	for _, e := range events {
		switch m := e.Metadata.(type) {
		case *bitbucketserver.CommitStatus:
			if m.Commit != latestCommit.ID {
				continue
			}
			dateAdded := unixMilliToTime(m.Status.DateAdded)
			if dateAdded.Before(lastSynced) {
				continue
			}
			stateMap[m.Key()] = parseBitbucketBuildState(m.Status.State)
		}
	}

	states := make([]batches.ChangesetCheckState, 0, len(stateMap))
	for _, v := range stateMap {
		states = append(states, v)
	}

	return combineCheckStates(states)
}

func parseBitbucketBuildState(s string) batches.ChangesetCheckState {
	switch s {
	case "FAILED":
		return batches.ChangesetCheckStateFailed
	case "INPROGRESS":
		return batches.ChangesetCheckStatePending
	case "SUCCESSFUL":
		return batches.ChangesetCheckStatePassed
	default:
		return batches.ChangesetCheckStateUnknown
	}
}

func computeGitHubCheckState(lastSynced time.Time, pr *github.PullRequest, events []*batches.ChangesetEvent) batches.ChangesetCheckState {
	// We should only consider the latest commit. This could be from a sync or a webhook that
	// has occurred later
	var latestCommitTime time.Time
	var latestOID string
	statusPerContext := make(map[string]batches.ChangesetCheckState)
	statusPerCheckSuite := make(map[string]batches.ChangesetCheckState)
	statusPerCheckRun := make(map[string]batches.ChangesetCheckState)

	if len(pr.Commits.Nodes) > 0 {
		// We only request the most recent commit
		commit := pr.Commits.Nodes[0]
		latestCommitTime = commit.Commit.CommittedDate
		latestOID = commit.Commit.OID
		// Calc status per context for the most recent synced commit
		for _, c := range commit.Commit.Status.Contexts {
			statusPerContext[c.Context] = parseGithubCheckState(c.State)
		}
		for _, c := range commit.Commit.CheckSuites.Nodes {
			if (c.Status == "QUEUED" || c.Status == "COMPLETED") && len(c.CheckRuns.Nodes) == 0 {
				// Ignore queued suites with no runs.
				// It is common for suites to be created and then stay in the QUEUED state
				// forever with zero runs.
				continue
			}
			statusPerCheckSuite[c.ID] = parseGithubCheckSuiteState(c.Status, c.Conclusion)
			for _, r := range c.CheckRuns.Nodes {
				statusPerCheckRun[r.ID] = parseGithubCheckSuiteState(r.Status, r.Conclusion)
			}
		}
	}

	var statuses []*github.CommitStatus
	// Get all status updates that have happened since our last sync
	for _, e := range events {
		switch m := e.Metadata.(type) {
		case *github.CommitStatus:
			if m.ReceivedAt.After(lastSynced) {
				statuses = append(statuses, m)
			}
		case *github.PullRequestCommit:
			if m.Commit.CommittedDate.After(latestCommitTime) {
				latestCommitTime = m.Commit.CommittedDate
				latestOID = m.Commit.OID
				// statusPerContext is now out of date, reset it
				for k := range statusPerContext {
					delete(statusPerContext, k)
				}
			}
		case *github.CheckSuite:
			if (m.Status == "QUEUED" || m.Status == "COMPLETED") && len(m.CheckRuns.Nodes) == 0 {
				// Ignore suites with no runs.
				// See previous comment.
				continue
			}
			if m.ReceivedAt.After(lastSynced) {
				statusPerCheckSuite[m.ID] = parseGithubCheckSuiteState(m.Status, m.Conclusion)
			}
		case *github.CheckRun:
			if m.ReceivedAt.After(lastSynced) {
				statusPerCheckRun[m.ID] = parseGithubCheckSuiteState(m.Status, m.Conclusion)
			}
		}
	}

	if len(statuses) > 0 {
		// Update the statuses using any new webhook events for the latest commit
		sort.Slice(statuses, func(i, j int) bool {
			return statuses[i].ReceivedAt.Before(statuses[j].ReceivedAt)
		})
		for _, s := range statuses {
			if s.SHA != latestOID {
				continue
			}
			statusPerContext[s.Context] = parseGithubCheckState(s.State)
		}
	}
	finalStates := make([]batches.ChangesetCheckState, 0, len(statusPerContext))
	for k := range statusPerContext {
		finalStates = append(finalStates, statusPerContext[k])
	}
	for k := range statusPerCheckSuite {
		finalStates = append(finalStates, statusPerCheckSuite[k])
	}
	for k := range statusPerCheckRun {
		finalStates = append(finalStates, statusPerCheckRun[k])
	}
	return combineCheckStates(finalStates)
}

// combineCheckStates combines multiple check states into an overall state
// pending takes highest priority
// followed by error
// success return only if all successful
func combineCheckStates(states []batches.ChangesetCheckState) batches.ChangesetCheckState {
	if len(states) == 0 {
		return batches.ChangesetCheckStateUnknown
	}
	stateMap := make(map[batches.ChangesetCheckState]bool)
	for _, s := range states {
		stateMap[s] = true
	}

	switch {
	case stateMap[batches.ChangesetCheckStateUnknown]:
		// If are pending, overall is Pending
		return batches.ChangesetCheckStateUnknown
	case stateMap[batches.ChangesetCheckStatePending]:
		// If are pending, overall is Pending
		return batches.ChangesetCheckStatePending
	case stateMap[batches.ChangesetCheckStateFailed]:
		// If no pending, but have errors then overall is Failed
		return batches.ChangesetCheckStateFailed
	case stateMap[batches.ChangesetCheckStatePassed]:
		// No pending or errors then overall is Passed
		return batches.ChangesetCheckStatePassed
	}

	return batches.ChangesetCheckStateUnknown
}

func parseGithubCheckState(s string) batches.ChangesetCheckState {
	s = strings.ToUpper(s)
	switch s {
	case "ERROR", "FAILURE":
		return batches.ChangesetCheckStateFailed
	case "EXPECTED", "PENDING":
		return batches.ChangesetCheckStatePending
	case "SUCCESS":
		return batches.ChangesetCheckStatePassed
	default:
		return batches.ChangesetCheckStateUnknown
	}
}

func parseGithubCheckSuiteState(status, conclusion string) batches.ChangesetCheckState {
	status = strings.ToUpper(status)
	conclusion = strings.ToUpper(conclusion)
	switch status {
	case "IN_PROGRESS", "QUEUED", "REQUESTED":
		return batches.ChangesetCheckStatePending
	}
	if status != "COMPLETED" {
		return batches.ChangesetCheckStateUnknown
	}
	switch conclusion {
	case "SUCCESS", "NEUTRAL":
		return batches.ChangesetCheckStatePassed
	case "ACTION_REQUIRED":
		return batches.ChangesetCheckStatePending
	case "CANCELLED", "FAILURE", "TIMED_OUT":
		return batches.ChangesetCheckStateFailed
	}
	return batches.ChangesetCheckStateUnknown
}

func computeGitLabCheckState(lastSynced time.Time, mr *gitlab.MergeRequest, events []*batches.ChangesetEvent) batches.ChangesetCheckState {
	// GitLab pipelines aren't tied to commits in the same way that GitHub
	// checks are. We're simply looking for the most recent pipeline run that
	// was associated with the merge request, which may live in a changeset
	// event (via webhook) or on the Pipelines field of the merge request
	// itself. We don't need to implement the same combinatorial logic that
	// exists for other code hosts because that's essentially what the pipeline
	// is, except GitLab handles the details of combining the job states.

	// Let's figure out what the last pipeline event we saw in the events was.
	var lastPipelineEvent *gitlab.Pipeline
	for _, e := range events {
		switch m := e.Metadata.(type) {
		case *gitlab.Pipeline:
			if lastPipelineEvent == nil || lastPipelineEvent.CreatedAt.Before(m.CreatedAt.Time) {
				lastPipelineEvent = m
			}
		}
	}

	if lastPipelineEvent == nil || lastPipelineEvent.CreatedAt.Before(lastSynced) {
		// OK, so we've either synced since the last pipeline event or there
		// just aren't any events, therefore the source of truth is the merge
		// request. The process here is pretty straightforward: the latest
		// pipeline wins. They _should_ be in descending order, but we'll sort
		// them just to be sure.

		// First up, a special case: if there are no pipelines, we'll try to use
		// HeadPipeline. If that's empty, then we'll shrug and say we don't
		// know.
		if len(mr.Pipelines) == 0 {
			if mr.HeadPipeline != nil {
				return parseGitLabPipelineStatus(mr.HeadPipeline.Status)
			}
			return batches.ChangesetCheckStateUnknown
		}

		// Sort into descending order so that the pipeline at index 0 is the latest.
		pipelines := mr.Pipelines
		sort.Slice(pipelines, func(i, j int) bool {
			return pipelines[i].CreatedAt.After(pipelines[j].CreatedAt.Time)
		})

		return parseGitLabPipelineStatus(pipelines[0].Status)
	}

	return parseGitLabPipelineStatus(lastPipelineEvent.Status)
}

func parseGitLabPipelineStatus(status gitlab.PipelineStatus) batches.ChangesetCheckState {
	switch status {
	case gitlab.PipelineStatusSuccess:
		return batches.ChangesetCheckStatePassed
	case gitlab.PipelineStatusFailed, gitlab.PipelineStatusCanceled:
		return batches.ChangesetCheckStateFailed
	case gitlab.PipelineStatusPending, gitlab.PipelineStatusRunning, gitlab.PipelineStatusCreated:
		return batches.ChangesetCheckStatePending
	default:
		return batches.ChangesetCheckStateUnknown
	}
}

// computeSingleChangesetExternalState of a Changeset based on the metadata.
// It does NOT reflect the final calculated state, use `ExternalState` instead.
func computeSingleChangesetExternalState(c *batches.Changeset) (s batches.ChangesetExternalState, err error) {
	if !c.ExternalDeletedAt.IsZero() {
		return batches.ChangesetExternalStateDeleted, nil
	}

	switch m := c.Metadata.(type) {
	case *github.PullRequest:
		if m.IsDraft && m.State == string(batches.ChangesetExternalStateOpen) {
			s = batches.ChangesetExternalStateDraft
		} else {
			s = batches.ChangesetExternalState(m.State)
		}
	case *bitbucketserver.PullRequest:
		if m.State == "DECLINED" {
			s = batches.ChangesetExternalStateClosed
		} else {
			s = batches.ChangesetExternalState(m.State)
		}
	case *gitlab.MergeRequest:
		switch m.State {
		case gitlab.MergeRequestStateClosed, gitlab.MergeRequestStateLocked:
			s = batches.ChangesetExternalStateClosed
		case gitlab.MergeRequestStateMerged:
			s = batches.ChangesetExternalStateMerged
		case gitlab.MergeRequestStateOpened:
			if m.WorkInProgress {
				s = batches.ChangesetExternalStateDraft
			} else {
				s = batches.ChangesetExternalStateOpen
			}
		default:
			return "", errors.Errorf("unknown GitLab merge request state: %s", m.State)
		}
	default:
		return "", errors.New("unknown changeset type")
	}

	if !s.Valid() {
		return "", errors.Errorf("changeset state %q invalid", s)
	}

	return s, nil
}

// computeSingleChangesetReviewState computes the review state of a Changeset.
// GitHub doesn't keep the review state on a changeset, so a GitHub Changeset
// will always return ChangesetReviewStatePending.
//
// This method should NOT be called directly. Use computeReviewState instead.
func computeSingleChangesetReviewState(c *batches.Changeset) (s batches.ChangesetReviewState, err error) {
	states := map[batches.ChangesetReviewState]bool{}

	switch m := c.Metadata.(type) {
	case *github.PullRequest:
		// For GitHub we need to use `ChangesetEvents.ReviewState`.
		return batches.ChangesetReviewStatePending, nil

	case *bitbucketserver.PullRequest:
		for _, r := range m.Reviewers {
			switch r.Status {
			case "UNAPPROVED":
				states[batches.ChangesetReviewStatePending] = true
			case "NEEDS_WORK":
				states[batches.ChangesetReviewStateChangesRequested] = true
			case "APPROVED":
				states[batches.ChangesetReviewStateApproved] = true
			}
		}

	case *gitlab.MergeRequest:
		// GitLab has an elaborate approvers workflow, but this doesn't map
		// terribly closely to the GitHub/Bitbucket workflow: most notably,
		// there's no analog of the Changes Requested or Dismissed states.
		//
		// Instead, we'll take a different tack: if we see an approval before
		// any unapproval event, then we'll consider the MR approved. If we see
		// an unapproval, then changes were requested. If we don't see anything,
		// then we're pending.
		for _, note := range m.Notes {
			if e := note.ToEvent(); e != nil {
				switch e.(type) {
				case *gitlab.ReviewApprovedEvent:
					return batches.ChangesetReviewStateApproved, nil
				case *gitlab.ReviewUnapprovedEvent:
					return batches.ChangesetReviewStateChangesRequested, nil
				}
			}
		}
		return batches.ChangesetReviewStatePending, nil

	default:
		return "", errors.New("unknown changeset type")
	}

	return selectReviewState(states), nil
}

// selectReviewState computes the single review state for a given set of
// ChangesetReviewStates. Since a pull request, for example, can have multiple
// reviews with different states, we need a function to determine what the
// state for the pull request is.
func selectReviewState(states map[batches.ChangesetReviewState]bool) batches.ChangesetReviewState {
	// If any review requested changes, that state takes precedence over all
	// other review states, followed by explicit approval. Everything else is
	// considered pending.
	for _, state := range [...]batches.ChangesetReviewState{
		batches.ChangesetReviewStateChangesRequested,
		batches.ChangesetReviewStateApproved,
	} {
		if states[state] {
			return state
		}
	}

	return batches.ChangesetReviewStatePending
}

// computeDiffStat computes the up to date diffstat for the changeset, based on
// the values in c.SyncState.
func computeDiffStat(ctx context.Context, c *batches.Changeset, repo api.RepoName) (*diff.Stat, error) {
	iter, err := git.Diff(ctx, git.DiffOptions{
		Repo: repo,
		Base: c.SyncState.BaseRefOid,
		Head: c.SyncState.HeadRefOid,
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	stat := &diff.Stat{}
	for {
		file, err := iter.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		fs := file.Stat()
		stat.Added += fs.Added
		stat.Changed += fs.Changed
		stat.Deleted += fs.Deleted
	}

	return stat, nil
}

// computeSyncState computes the up to date sync state based on the changeset as
// it currently exists on the external provider.
func computeSyncState(ctx context.Context, c *batches.Changeset, repo api.RepoName) (*batches.ChangesetSyncState, error) {
	// We compute the revision by first trying to get the OID, then the Ref. //
	// We then call out to gitserver to ensure that the one we use is available on
	// gitserver.
	base, err := computeRev(ctx, repo, c.BaseRefOid, c.BaseRef)
	if err != nil {
		return nil, err
	}

	head, err := computeRev(ctx, repo, c.HeadRefOid, c.HeadRef)
	if err != nil {
		return nil, err
	}

	return &batches.ChangesetSyncState{
		BaseRefOid: base,
		HeadRefOid: head,
		IsComplete: c.ExternalState != batches.ChangesetExternalStateOpen,
	}, nil
}

func computeRev(ctx context.Context, repo api.RepoName, getOid, getRef func() (string, error)) (string, error) {
	// Try to get the OID first
	rev, err := getOid()
	if err != nil {
		return "", err
	}

	if rev == "" {
		// Fallback to the ref
		rev, err = getRef()
		if err != nil {
			return "", err
		}
	}

	// Resolve the revision to make sure it's on gitserver and, in case we did
	// the fallback to ref, to get the specific revision.
	gitRev, err := git.ResolveRevision(ctx, repo, rev, git.ResolveRevisionOptions{})
	return string(gitRev), err
}

// changesetRepoName looks up a api.RepoName based on the RepoID within a changeset.
func changesetRepoName(ctx context.Context, repoStore *database.RepoStore, c *batches.Changeset) (api.RepoName, error) {
	// We need to use an internal actor here as the repo-updater otherwise has no access to the repo.
	repo, err := repoStore.Get(actor.WithActor(ctx, &actor.Actor{Internal: true}), c.RepoID)
	if err != nil {
		return "", err
	}
	return repo.Name, nil
}

func unixMilliToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}

// ComputeLabels returns a sorted list of current labels based the starting set
// of labels found in the Changeset and looking at ChangesetEvents that have
// occurred after the Changeset.UpdatedAt.
// The events should be presorted.
func ComputeLabels(c *batches.Changeset, events ChangesetEvents) []batches.ChangesetLabel {
	var current []batches.ChangesetLabel
	var since time.Time
	if c != nil {
		current = c.Labels()
		since = c.UpdatedAt
	}

	// Iterate through all label events to get the current set
	set := make(map[string]batches.ChangesetLabel)
	for _, l := range current {
		set[l.Name] = l
	}
	for _, event := range events {
		switch e := event.Metadata.(type) {
		case *github.LabelEvent:
			if e.CreatedAt.Before(since) {
				continue
			}
			if e.Removed {
				delete(set, e.Label.Name)
				continue
			}

			set[e.Label.Name] = batches.ChangesetLabel{
				Name:        e.Label.Name,
				Color:       e.Label.Color,
				Description: e.Label.Description,
			}
		}
	}
	labels := make([]batches.ChangesetLabel, 0, len(set))
	for _, label := range set {
		labels = append(labels, label)
	}

	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})

	return labels
}
