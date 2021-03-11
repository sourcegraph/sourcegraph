package state

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestCalcCounts(t *testing.T) {
	t.Parallel()

	now := timeutil.Now()
	daysAgo := func(days int) time.Time { return now.AddDate(0, 0, -days) }

	tests := []struct {
		codehosts  string
		name       string
		changesets []*batches.Changeset
		start      time.Time
		end        time.Time
		events     []*batches.ChangesetEvent
		want       []*ChangesetCounts
	}{
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			name: "start end time on subset of events",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			end:   daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset created and closed before start time",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(8)),
			},
			start: daysAgo(4),
			end:   daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(7), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 1, Merged: 1},
				{Time: daysAgo(3), Total: 1, Merged: 1},
				{Time: daysAgo(2), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset created and closed before start time",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(8)),
			},
			start: daysAgo(4),
			end:   daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(7), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 1, Merged: 1},
				{Time: daysAgo(3), Total: 1, Merged: 1},
				{Time: daysAgo(2), Total: 1, Merged: 1},
			},
		},
		{
			name: "start time not even x*24hours before end time",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(3),
			end:   now.Add(-18 * time.Hour),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2).Add(-18 * time.Hour), Total: 0, Merged: 0},
				{Time: daysAgo(1).Add(-18 * time.Hour), Total: 1, Open: 1, OpenPending: 1},
				{Time: now.Add(-18 * time.Hour), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "multiple changesets open merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
				ghChangeset(2, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 2, Open: 2, OpenPending: 2},
				{Time: daysAgo(1), Total: 2, Merged: 2},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "multiple changesets open merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(2)),
				bbsChangeset(2, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 2, Open: 2, OpenPending: 2},
				{Time: daysAgo(1), Total: 2, Merged: 2},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "multiple changesets open merged different times",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
				ghChangeset(2, daysAgo(2)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubMerged, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 2, Open: 1, OpenPending: 1, Merged: 1},
				{Time: daysAgo(1), Total: 2, Merged: 2},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "multiple changesets open merged different times",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
				bbsChangeset(2, daysAgo(2)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerMerged, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 2, Open: 1, OpenPending: 1, Merged: 1},
				{Time: daysAgo(1), Total: 2, Merged: 2},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "changeset merged and closed at same time",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "changeset merged and closed at same time, reversed order in slice",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open closed reopened merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(4)),
			},
			start: daysAgo(5),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubReopened, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(5), Total: 0, Open: 0},
				{Time: daysAgo(4), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(3), Total: 1, Open: 0, Closed: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open declined reopened merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(4)),
			},
			start: daysAgo(5),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerReopened, 1),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(5), Total: 0, Open: 0},
				{Time: daysAgo(4), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(3), Total: 1, Open: 0, Closed: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "multiple changesets open closed reopened merged different times",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(5)),
				ghChangeset(2, daysAgo(4)),
			},
			start: daysAgo(6),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(4), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubClosed, 2),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubReopened, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubReopened, 2),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(6), Total: 0, Open: 0},
				{Time: daysAgo(5), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(4), Total: 2, Open: 1, OpenPending: 1, Closed: 1},
				{Time: daysAgo(3), Total: 2, Open: 1, OpenPending: 1, Closed: 1},
				{Time: daysAgo(2), Total: 2, Open: 2, OpenPending: 2},
				{Time: daysAgo(1), Total: 2, Open: 1, OpenPending: 1, Merged: 1},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "multiple changesets open declined reopened merged different times",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(5)),
				bbsChangeset(2, daysAgo(4)),
			},
			start: daysAgo(6),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(4), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerDeclined, 2),
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerReopened, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerReopened, 2),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindBitbucketServerMerged, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(6), Total: 0, Open: 0},
				{Time: daysAgo(5), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(4), Total: 2, Open: 1, OpenPending: 1, Closed: 1},
				{Time: daysAgo(3), Total: 2, Open: 1, OpenPending: 1, Closed: 1},
				{Time: daysAgo(2), Total: 2, Open: 2, OpenPending: 2},
				{Time: daysAgo(1), Total: 2, Open: 1, OpenPending: 1, Merged: 1},
				{Time: daysAgo(0), Total: 2, Merged: 2},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open closed reopened merged, unsorted events",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(4)),
			},
			start: daysAgo(5),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(5), Total: 0, Open: 0},
				{Time: daysAgo(4), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(3), Total: 1, Open: 0, Closed: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open closed reopened merged, unsorted events",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(4)),
			},
			start: daysAgo(5),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(5), Total: 0, Open: 0},
				{Time: daysAgo(4), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(3), Total: 1, Open: 0, Closed: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, approved, merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "APPROVED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, changes-requested, unapproved",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsParticipantEvent(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerDismissed),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, approved, closed, reopened",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "APPROVED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, declined, reopened",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindBitbucketServerReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, approved, closed, merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "APPROVED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubReopened, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, closed, merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindBitbucketServerReopened, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, changes-requested, closed, reopened",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, changes-requested, closed, reopened",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerDeclined, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindBitbucketServerReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, changes-requested, closed, merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, comment review, approved, merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(3), "user1", "COMMENTED"),
				ghReview(1, daysAgo(2), "user2", "APPROVED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, comment review, approved, merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(3), "user1", batches.ChangesetEventKindBitbucketServerCommented),
				bbsActivity(1, daysAgo(2), "user2", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset multiple approvals counting once",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "APPROVED"),
				ghReview(1, daysAgo(0), "user2", "APPROVED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset multiple approvals counting once",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerApproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset multiple changes-requested reviews counting once",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "CHANGES_REQUESTED"),
				ghReview(1, daysAgo(0), "user2", "CHANGES_REQUESTED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset multiple changes-requested reviews counting once",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerReviewed),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset open, changes-requested, merged",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, changes-requested, merged",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Merged: 1},
				{Time: daysAgo(0), Total: 1, Merged: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "multiple changesets open different review stages before merge",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(6)),
				ghChangeset(2, daysAgo(6)),
				ghChangeset(3, daysAgo(6)),
			},
			start: daysAgo(7),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(5), "user1", "APPROVED"),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubMerged, 1),
				ghReview(2, daysAgo(4), "user1", "APPROVED"),
				ghReview(2, daysAgo(3), "user2", "APPROVED"),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubMerged, 2),
				ghReview(3, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				ghReview(3, daysAgo(1), "user2", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 3),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(7), Total: 0, Open: 0},
				{Time: daysAgo(6), Total: 3, Open: 3, OpenPending: 3},
				{Time: daysAgo(5), Total: 3, Open: 3, OpenPending: 2, OpenApproved: 1},
				{Time: daysAgo(4), Total: 3, Open: 3, OpenPending: 1, OpenApproved: 2},
				{Time: daysAgo(3), Total: 3, Open: 2, OpenPending: 1, OpenApproved: 1, Merged: 1},
				{Time: daysAgo(2), Total: 3, Open: 1, OpenPending: 0, OpenChangesRequested: 1, Merged: 2},
				{Time: daysAgo(1), Total: 3, Merged: 3},
				{Time: daysAgo(0), Total: 3, Merged: 3},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "multiple changesets open different review stages before merge",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(6)),
				bbsChangeset(2, daysAgo(6)),
				bbsChangeset(3, daysAgo(6)),
			},
			start: daysAgo(7),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(5), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerMerged, 1),
				bbsActivity(2, daysAgo(4), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(2, daysAgo(3), "user2", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerMerged, 2),
				bbsActivity(3, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(3, daysAgo(1), "user2", batches.ChangesetEventKindBitbucketServerReviewed),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 3),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(7), Total: 0, Open: 0},
				{Time: daysAgo(6), Total: 3, Open: 3, OpenPending: 3},
				{Time: daysAgo(5), Total: 3, Open: 3, OpenPending: 2, OpenApproved: 1},
				{Time: daysAgo(4), Total: 3, Open: 3, OpenPending: 1, OpenApproved: 2},
				{Time: daysAgo(3), Total: 3, Open: 2, OpenPending: 1, OpenApproved: 1, Merged: 1},
				{Time: daysAgo(2), Total: 3, Open: 1, OpenPending: 0, OpenChangesRequested: 1, Merged: 2},
				{Time: daysAgo(1), Total: 3, Merged: 3},
				{Time: daysAgo(0), Total: 3, Merged: 3},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "time slice of multiple changesets in different stages before merge",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(6)),
				ghChangeset(2, daysAgo(6)),
				ghChangeset(3, daysAgo(6)),
			},
			// Same test as above, except we only look at 3 days in the middle
			start: daysAgo(4),
			end:   daysAgo(2),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(5), "user1", "APPROVED"),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubMerged, 1),
				ghReview(2, daysAgo(4), "user1", "APPROVED"),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubMerged, 2),
				ghReview(3, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 3),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 3, Open: 3, OpenPending: 1, OpenApproved: 2},
				{Time: daysAgo(3), Total: 3, Open: 2, OpenPending: 1, OpenApproved: 1, Merged: 1},
				{Time: daysAgo(2), Total: 3, Open: 1, OpenPending: 0, OpenChangesRequested: 1, Merged: 2},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with changes-requested then approved by same person",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "CHANGES_REQUESTED"),
				ghReview(1, daysAgo(0), "user1", "APPROVED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with changes-requested then approved by same person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(0), "user1", batches.ChangesetEventKindBitbucketServerApproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with approved then changes-requested by same person",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "APPROVED"),
				ghReview(1, daysAgo(0), "user1", "CHANGES_REQUESTED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with approved then changes-requested by same person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with approval by one person then changes-requested by another",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "APPROVED"),
				ghReview(1, daysAgo(0), "user2", "CHANGES_REQUESTED"), // This has higher precedence
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with approval by one person then changes-requested by another",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerReviewed),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with changes-requested by one person then approval by another",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "CHANGES_REQUESTED"),
				ghReview(1, daysAgo(0), "user2", "APPROVED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with changes-requested by one person then approval by another",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerApproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with changes-requested by one person, approval by another, then approval by first person",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				ghReview(1, daysAgo(1), "user2", "APPROVED"),
				ghReview(1, daysAgo(0), "user1", "APPROVED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with changes-requested by one person, approval by another, then approval by first person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(1), "user2", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user1", batches.ChangesetEventKindBitbucketServerApproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with approval by one person, changes-requested by another, then changes-requested by first person",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "APPROVED"),
				ghReview(1, daysAgo(1), "user2", "CHANGES_REQUESTED"),
				ghReview(1, daysAgo(0), "user1", "CHANGES_REQUESTED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset with approval by one person, changes-requested by another, then changes-requested by first person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(1), "user2", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(0), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenChangesRequested: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, unapproved",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerUnapproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1, OpenApproved: 0},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1, OpenApproved: 0},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, changes requested, approved, unapproved",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user1", batches.ChangesetEventKindBitbucketServerUnapproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenChangesRequested: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1, OpenApproved: 0},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, unapproved, approved by another person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(1), "user1", batches.ChangesetEventKindBitbucketServerUnapproved),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerApproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1, OpenApproved: 0},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
			},
		},
		{
			codehosts: "bitbucketserver",
			name:      "single changeset open, approved, then approved and unapproved by another person",
			changesets: []*batches.Changeset{
				bbsChangeset(1, daysAgo(3)),
			},
			start: daysAgo(4),
			events: []*batches.ChangesetEvent{
				bbsActivity(1, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(1), "user2", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(1, daysAgo(0), "user2", batches.ChangesetEventKindBitbucketServerUnapproved),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(4), Total: 0, Open: 0},
				{Time: daysAgo(3), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 0, OpenApproved: 1},
			},
		},
		{
			codehosts: "github and bitbucketserver",
			name:      "multiple changesets on different code hosts in different review stages before merge",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(6)),
				bbsChangeset(2, daysAgo(6)),
				ghChangeset(3, daysAgo(6)),
				bbsChangeset(4, daysAgo(6)),
				ghChangeset(5, daysAgo(6)),
				bbsChangeset(6, daysAgo(6)),
			},
			start: daysAgo(7),
			events: []*batches.ChangesetEvent{
				// GitHub Events
				ghReview(1, daysAgo(5), "user1", "APPROVED"),
				event(t, daysAgo(3), batches.ChangesetEventKindGitHubMerged, 1),
				ghReview(3, daysAgo(4), "user1", "APPROVED"),
				ghReview(3, daysAgo(3), "user2", "APPROVED"),
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubMerged, 3),
				ghReview(5, daysAgo(2), "user1", "CHANGES_REQUESTED"),
				ghReview(5, daysAgo(1), "user2", "CHANGES_REQUESTED"),
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubMerged, 5),
				// Bitbucket Server Events
				bbsActivity(2, daysAgo(5), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(3), batches.ChangesetEventKindBitbucketServerMerged, 2),
				bbsActivity(4, daysAgo(4), "user1", batches.ChangesetEventKindBitbucketServerApproved),
				bbsActivity(4, daysAgo(3), "user2", batches.ChangesetEventKindBitbucketServerApproved),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerMerged, 4),
				bbsActivity(6, daysAgo(2), "user1", batches.ChangesetEventKindBitbucketServerReviewed),
				bbsActivity(6, daysAgo(1), "user2", batches.ChangesetEventKindBitbucketServerReviewed),
				event(t, daysAgo(1), batches.ChangesetEventKindBitbucketServerMerged, 6),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(7), Total: 0, Open: 0},
				{Time: daysAgo(6), Total: 6, Open: 6, OpenPending: 6},
				{Time: daysAgo(5), Total: 6, Open: 6, OpenPending: 4, OpenApproved: 2},
				{Time: daysAgo(4), Total: 6, Open: 6, OpenPending: 2, OpenApproved: 4},
				{Time: daysAgo(3), Total: 6, Open: 4, OpenPending: 2, OpenApproved: 2, Merged: 2},
				{Time: daysAgo(2), Total: 6, Open: 2, OpenPending: 0, OpenChangesRequested: 2, Merged: 4},
				{Time: daysAgo(1), Total: 6, Merged: 6},
				{Time: daysAgo(0), Total: 6, Merged: 6},
			},
		},
		{
			codehosts: "github and bitbucketserver",
			name:      "multiple changesets open and deleted",
			changesets: []*batches.Changeset{
				setExternalDeletedAt(ghChangeset(1, daysAgo(2)), daysAgo(1)),
				setExternalDeletedAt(bbsChangeset(1, daysAgo(2)), daysAgo(1)),
			},
			start: daysAgo(2),
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 2, Open: 2, OpenPending: 2},
				// We count deleted as closed
				{Time: daysAgo(1), Total: 2, Closed: 2},
				{Time: daysAgo(0), Total: 2, Closed: 2},
			},
		},
		{
			codehosts: "github and bitbucketserver",
			name:      "multiple changesets open, closed and deleted",
			changesets: []*batches.Changeset{
				setExternalDeletedAt(ghChangeset(1, daysAgo(3)), daysAgo(1)),
				setExternalDeletedAt(bbsChangeset(2, daysAgo(3)), daysAgo(1)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(2), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(2), batches.ChangesetEventKindBitbucketServerDeclined, 2),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 2, Open: 2, OpenPending: 2},
				{Time: daysAgo(2), Total: 2, Closed: 2},
				// We count deleted as closed, so they stay closed
				{Time: daysAgo(1), Total: 2, Closed: 2},
				{Time: daysAgo(0), Total: 2, Closed: 2},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with changes-requested then dismissed event by same person with dismissed state",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				// GitHub updates the state of the reviews when they're dismissed
				ghReview(1, daysAgo(0), "user1", "DISMISSED"),
				ghReviewDismissed(1, daysAgo(0), "user2", "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with approval by one person, changes-requested by another, then dismissal of changes-requested",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(2), "user1", "APPROVED"),
				// GitHub updates the state of the changesets when they're dismissed
				ghReview(1, daysAgo(1), "user2", "DISMISSED"),
				ghReviewDismissed(1, daysAgo(1), "user3", "user2"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenApproved: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenApproved: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset with changes-requested, then another dismissed review by same person",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReview(1, daysAgo(1), "user1", "CHANGES_REQUESTED"),
				// After a dismissal, GitHub removes all of the author's
				// reviews from the overall review state, which is why we don't
				// want to fall back to "ChangesRequested" even though _that_
				// was not dismissed.
				ghReview(1, daysAgo(0), "user1", "DISMISSED"),
				ghReviewDismissed(1, daysAgo(0), "user2", "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Open: 1, OpenChangesRequested: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset opened as draft",
			changesets: []*batches.Changeset{
				setDraft(ghChangeset(1, daysAgo(2))),
			},
			start:  daysAgo(1),
			events: []*batches.ChangesetEvent{},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset opened as draft then opened for review",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not in draft anymore".
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				ghReadyForReview(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset opened as draft then opened for review and converted back",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not in draft anymore".
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				ghReadyForReview(1, daysAgo(1), "user1"),
				ghConvertToDraft(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Draft: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "single changeset opened as draft then opened for review, converted back and opened for review again",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not in draft anymore".
				ghChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				ghReadyForReview(1, daysAgo(2), "user1"),
				ghConvertToDraft(1, daysAgo(1), "user1"),
				ghReadyForReview(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Draft: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab single changeset opened as draft",
			changesets: []*batches.Changeset{
				setDraft(glChangeset(1, daysAgo(2))),
			},
			start:  daysAgo(1),
			events: []*batches.ChangesetEvent{},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab single changeset opened as draft then opened for review",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not a draft anymore".
				glChangeset(1, daysAgo(2)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				glUnmarkWorkInProgress(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab single changeset opened as draft then opened for review and converted back",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not a draft anymore".
				glChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				glUnmarkWorkInProgress(1, daysAgo(1), "user1"),
				glMarkWorkInProgress(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Draft: 1},
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab single changeset opened as draft then opened for review, converted back and opened for review again",
			changesets: []*batches.Changeset{
				// Not setDraft, because the current state is "not a draft anymore".
				glChangeset(1, daysAgo(3)),
			},
			start: daysAgo(3),
			events: []*batches.ChangesetEvent{
				glUnmarkWorkInProgress(1, daysAgo(2), "user1"),
				glMarkWorkInProgress(1, daysAgo(1), "user1"),
				glUnmarkWorkInProgress(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(3), Total: 1, Draft: 1},
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(1), Total: 1, Draft: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab unmarked wip while closed",
			changesets: []*batches.Changeset{
				glChangeset(1, daysAgo(1)),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				glClosed(1, daysAgo(1), "user1"),
				glMarkWorkInProgress(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Closed: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab marked wip while closed",
			changesets: []*batches.Changeset{
				setDraft(glChangeset(1, daysAgo(1))),
			},
			start: daysAgo(1),
			events: []*batches.ChangesetEvent{
				glClosed(1, daysAgo(1), "user1"),
				glUnmarkWorkInProgress(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Closed: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "changeset approved by deleted user",
			changesets: []*batches.Changeset{
				ghChangeset(1, daysAgo(2)),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				// An empty author ("") usually means the user has been deleted.
				ghReview(1, daysAgo(1), "", "APPROVED"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Open: 1, OpenPending: 1},
				// A deleted users' review doesn't have an effect on the review state.
				{Time: daysAgo(1), Total: 1, Open: 1, OpenPending: 1},
				{Time: daysAgo(0), Total: 1, Open: 1, OpenPending: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitHub,
			name:      "GitHub still draft after reopen",
			changesets: []*batches.Changeset{
				setDraft(ghChangeset(1, daysAgo(2))),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				event(t, daysAgo(1), batches.ChangesetEventKindGitHubClosed, 1),
				event(t, daysAgo(0), batches.ChangesetEventKindGitHubReopened, 1),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Draft: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
		{
			codehosts: extsvc.TypeGitLab,
			name:      "GitLab still draft after reopen",
			changesets: []*batches.Changeset{
				setDraft(glChangeset(1, daysAgo(2))),
			},
			start: daysAgo(2),
			events: []*batches.ChangesetEvent{
				glClosed(1, daysAgo(1), "user1"),
				glReopen(1, daysAgo(0), "user1"),
			},
			want: []*ChangesetCounts{
				{Time: daysAgo(2), Total: 1, Draft: 1},
				{Time: daysAgo(1), Total: 1, Closed: 1},
				{Time: daysAgo(0), Total: 1, Draft: 1},
			},
		},
	}

	for _, tc := range tests {
		if tc.codehosts != "" {
			tc.name = tc.codehosts + "/" + tc.name
		}
		t.Run(tc.name, func(t *testing.T) {
			if tc.end.IsZero() {
				tc.end = now
			}

			have, err := CalcCounts(tc.start, tc.end, tc.changesets, tc.events...)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, tc.want); diff != "" {
				t.Errorf("wrong counts calculated. diff=%s", diff)
			}
		})
	}
}

func ghChangeset(id int64, t time.Time) *batches.Changeset {
	return &batches.Changeset{ID: id, Metadata: &github.PullRequest{CreatedAt: t}}
}

func bbsChangeset(id int64, t time.Time) *batches.Changeset {
	return &batches.Changeset{
		ID:       id,
		Metadata: &bitbucketserver.PullRequest{CreatedDate: timeToUnixMilli(t)},
	}
}

func glChangeset(id int64, t time.Time) *batches.Changeset {
	return &batches.Changeset{
		ID:       id,
		Metadata: &gitlab.MergeRequest{CreatedAt: gitlab.Time{Time: t}},
	}
}

func setExternalDeletedAt(c *batches.Changeset, t time.Time) *batches.Changeset {
	c.SetDeleted()
	c.ExternalDeletedAt = t
	return c
}

func event(t *testing.T, ti time.Time, kind batches.ChangesetEventKind, id int64) *batches.ChangesetEvent {
	ch := &batches.ChangesetEvent{ChangesetID: id, Kind: kind}

	switch kind {
	case batches.ChangesetEventKindGitHubMerged:
		ch.Metadata = &github.MergedEvent{CreatedAt: ti}
	case batches.ChangesetEventKindGitHubClosed:
		ch.Metadata = &github.ClosedEvent{CreatedAt: ti}
	case batches.ChangesetEventKindGitHubReopened:
		ch.Metadata = &github.ReopenedEvent{CreatedAt: ti}

	case batches.ChangesetEventKindBitbucketServerMerged,
		batches.ChangesetEventKindBitbucketServerDeclined,
		batches.ChangesetEventKindBitbucketServerReopened:

		ch.Metadata = &bitbucketserver.Activity{CreatedDate: timeToUnixMilli(ti)}

	default:
		t.Fatalf("unknown changeset event kind: %s", kind)
	}

	want := ti.UTC().Truncate(time.Millisecond)
	have := ch.Timestamp().UTC().Truncate(time.Millisecond)
	if !have.Equal(want) {
		t.Fatalf("ChangesetEvent.Timestamp() yields wrong timestamp, want=%s, have=%s (make sure to set the right attribute when constructing test event)",
			want, have)
	}

	return ch
}

func ghReview(id int64, t time.Time, login, state string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitHubReviewed,
		Metadata: &github.PullRequestReview{
			UpdatedAt: t,
			State:     state,
			Author: github.Actor{
				Login: login,
			},
		},
	}
}

func ghReviewDismissed(id int64, t time.Time, login, reviewer string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitHubReviewDismissed,
		Metadata: &github.ReviewDismissedEvent{
			CreatedAt: t,
			Actor:     github.Actor{Login: login},
			Review: github.PullRequestReview{
				Author: github.Actor{
					Login: reviewer,
				},
			},
		},
	}
}

func ghReadyForReview(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitHubReadyForReview,
		Metadata: &github.ReadyForReviewEvent{
			CreatedAt: t,
			Actor: github.Actor{
				Login: login,
			},
		},
	}
}

func ghConvertToDraft(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitHubConvertToDraft,
		Metadata: &github.ConvertToDraftEvent{
			CreatedAt: t,
			Actor: github.Actor{
				Login: login,
			},
		},
	}
}

func glUnmarkWorkInProgress(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitLabUnmarkWorkInProgress,
		Metadata: &gitlab.UnmarkWorkInProgressEvent{
			Note: &gitlab.Note{
				System:    true,
				Body:      gitlab.SystemNoteBodyUnmarkedWorkInProgress,
				CreatedAt: gitlab.Time{Time: t},
				Author: gitlab.User{
					Username: login,
				},
			},
		},
	}
}

func glMarkWorkInProgress(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitLabMarkWorkInProgress,
		Metadata: &gitlab.MarkWorkInProgressEvent{
			Note: &gitlab.Note{
				System:    true,
				Body:      gitlab.SystemNoteBodyMarkedWorkInProgress,
				CreatedAt: gitlab.Time{Time: t},
				Author: gitlab.User{
					Username: login,
				},
			},
		},
	}
}

func glClosed(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitLabClosed,
		Metadata: &gitlab.MergeRequestClosedEvent{
			ResourceStateEvent: &gitlab.ResourceStateEvent{
				CreatedAt: gitlab.Time{Time: t},
				User:      gitlab.User{Username: login},
				State:     gitlab.ResourceStateEventStateClosed,
			},
		},
		CreatedAt: t,
	}
}

func glReopen(id int64, t time.Time, login string) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        batches.ChangesetEventKindGitLabReopened,
		Metadata: &gitlab.MergeRequestReopenedEvent{
			ResourceStateEvent: &gitlab.ResourceStateEvent{
				CreatedAt: gitlab.Time{Time: t},
				User:      gitlab.User{Username: login},
				State:     gitlab.ResourceStateEventStateReopened,
			},
		},
		CreatedAt: t,
	}
}

func bbsActivity(id int64, t time.Time, username string, kind batches.ChangesetEventKind) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        kind,
		Metadata: &bitbucketserver.Activity{
			CreatedDate: timeToUnixMilli(t),
			User: bitbucketserver.User{
				Name: username,
			},
		},
	}
}

func bbsParticipantEvent(id int64, t time.Time, username string, kind batches.ChangesetEventKind) *batches.ChangesetEvent {
	return &batches.ChangesetEvent{
		ChangesetID: id,
		Kind:        kind,
		Metadata: &bitbucketserver.ParticipantStatusEvent{
			CreatedDate: timeToUnixMilli(t),
			User: bitbucketserver.User{
				Name: username,
			},
		},
	}
}
