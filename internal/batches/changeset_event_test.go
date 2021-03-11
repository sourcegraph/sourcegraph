package batches

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

func TestChangesetEvent(t *testing.T) {
	type testCase struct {
		name      string
		changeset Changeset
		events    []*ChangesetEvent
	}

	bbsActivity := &bitbucketserver.Activity{
		ID:     1,
		Action: bitbucketserver.OpenedActivityAction,
	}

	cases := []testCase{{
		name: "removes duplicates",
		changeset: Changeset{
			Metadata: &bitbucketserver.PullRequest{
				Activities: []*bitbucketserver.Activity{
					bbsActivity,
					bbsActivity,
				},
			},
		},
		events: []*ChangesetEvent{
			{
				Kind:     ChangesetEventKindBitbucketServerOpened,
				Key:      "1",
				Metadata: bbsActivity,
			},
		},
	}}

	{ // Github

		now := time.Now().UTC()

		reviewComments := []*github.PullRequestReviewComment{
			{DatabaseID: 1, Body: "foo"},
			{DatabaseID: 2, Body: "bar"},
			{DatabaseID: 3, Body: "baz"},
		}

		actor := github.Actor{Login: "john-doe"}

		assignedEvent := &github.AssignedEvent{
			Actor:     actor,
			Assignee:  actor,
			CreatedAt: now,
		}

		unassignedEvent := &github.UnassignedEvent{
			Actor:     actor,
			Assignee:  actor,
			CreatedAt: now,
		}

		closedEvent := &github.ClosedEvent{
			Actor:     actor,
			CreatedAt: now,
		}

		commit := &github.PullRequestCommit{
			Commit: github.Commit{
				OID: "123",
			},
		}

		cases = append(cases, testCase{"github",
			Changeset{
				ID: 23,
				Metadata: &github.PullRequest{
					TimelineItems: []github.TimelineItem{
						{Type: "AssignedEvent", Item: assignedEvent},
						{Type: "PullRequestReviewThread", Item: &github.PullRequestReviewThread{
							Comments: reviewComments[:2],
						}},
						{Type: "UnassignedEvent", Item: unassignedEvent},
						{Type: "PullRequestReviewThread", Item: &github.PullRequestReviewThread{
							Comments: reviewComments[2:],
						}},
						{Type: "ClosedEvent", Item: closedEvent},
						{Type: "PullRequestCommit", Item: commit},
					},
				},
			},
			[]*ChangesetEvent{{
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubAssigned,
				Key:         assignedEvent.Key(),
				Metadata:    assignedEvent,
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubReviewCommented,
				Key:         reviewComments[0].Key(),
				Metadata:    reviewComments[0],
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubReviewCommented,
				Key:         reviewComments[1].Key(),
				Metadata:    reviewComments[1],
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubUnassigned,
				Key:         unassignedEvent.Key(),
				Metadata:    unassignedEvent,
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubReviewCommented,
				Key:         reviewComments[2].Key(),
				Metadata:    reviewComments[2],
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubClosed,
				Key:         closedEvent.Key(),
				Metadata:    closedEvent,
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubCommit,
				Key:         commit.Key(),
				Metadata:    commit,
			}},
		})

		reviewRequestedActorEvent := &github.ReviewRequestedEvent{
			RequestedReviewer: github.Actor{Login: "the-great-tortellini"},
			Actor:             actor,
			CreatedAt:         now,
		}
		reviewRequestedTeamEvent := &github.ReviewRequestedEvent{
			RequestedTeam: github.Team{Name: "the-belgian-waffles"},
			Actor:         actor,
			CreatedAt:     now,
		}

		cases = append(cases, testCase{"github-blank-review-requested",
			Changeset{
				ID: 23,
				Metadata: &github.PullRequest{
					TimelineItems: []github.TimelineItem{
						{Type: "ReviewRequestedEvent", Item: reviewRequestedActorEvent},
						{Type: "ReviewRequestedEvent", Item: reviewRequestedTeamEvent},
						{Type: "ReviewRequestedEvent", Item: &github.ReviewRequestedEvent{
							// Both Team and Reviewer are blank.
							Actor:     actor,
							CreatedAt: now,
						}},
					},
				},
			},
			[]*ChangesetEvent{{
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubReviewRequested,
				Key:         reviewRequestedActorEvent.Key(),
				Metadata:    reviewRequestedActorEvent,
			}, {
				ChangesetID: 23,
				Kind:        ChangesetEventKindGitHubReviewRequested,
				Key:         reviewRequestedTeamEvent.Key(),
				Metadata:    reviewRequestedTeamEvent,
			}},
		})
	}

	{ // bitbucketserver

		user := bitbucketserver.User{Name: "john-doe"}
		reviewer := bitbucketserver.User{Name: "jane-doe"}

		activities := []*bitbucketserver.Activity{{
			ID:     1,
			User:   user,
			Action: bitbucketserver.OpenedActivityAction,
		}, {
			ID:     2,
			User:   reviewer,
			Action: bitbucketserver.ReviewedActivityAction,
		}, {
			ID:     3,
			User:   reviewer,
			Action: bitbucketserver.DeclinedActivityAction,
		}, {
			ID:     4,
			User:   user,
			Action: bitbucketserver.ReopenedActivityAction,
		}, {
			ID:     5,
			User:   user,
			Action: bitbucketserver.MergedActivityAction,
		}}

		cases = append(cases, testCase{"bitbucketserver",
			Changeset{
				ID: 24,
				Metadata: &bitbucketserver.PullRequest{
					Activities: activities,
				},
			},
			[]*ChangesetEvent{{
				ChangesetID: 24,
				Kind:        ChangesetEventKindBitbucketServerOpened,
				Key:         activities[0].Key(),
				Metadata:    activities[0],
			}, {
				ChangesetID: 24,
				Kind:        ChangesetEventKindBitbucketServerReviewed,
				Key:         activities[1].Key(),
				Metadata:    activities[1],
			}, {
				ChangesetID: 24,
				Kind:        ChangesetEventKindBitbucketServerDeclined,
				Key:         activities[2].Key(),
				Metadata:    activities[2],
			}, {
				ChangesetID: 24,
				Kind:        ChangesetEventKindBitbucketServerReopened,
				Key:         activities[3].Key(),
				Metadata:    activities[3],
			}, {
				ChangesetID: 24,
				Kind:        ChangesetEventKindBitbucketServerMerged,
				Key:         activities[4].Key(),
				Metadata:    activities[4],
			}},
		})
	}

	{ // GitLab
		notes := []*gitlab.Note{
			{ID: 11, System: false, Body: "this is a user note"},
			{ID: 12, System: true, Body: "approved this merge request"},
			{ID: 13, System: true, Body: "unapproved this merge request"},
			{ID: 14, System: true, Body: "marked as a **Work In Progress**"},
			{ID: 15, System: true, Body: "unmarked as a **Work In Progress**"},
		}

		pipelines := []*gitlab.Pipeline{
			{ID: 21},
			{ID: 22},
		}

		mr := &gitlab.MergeRequest{
			Notes:     notes,
			Pipelines: pipelines,
		}

		cases = append(cases, testCase{
			name: "gitlab",
			changeset: Changeset{
				ID:       1234,
				Metadata: mr,
			},
			events: []*ChangesetEvent{
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabApproved,
					Key:         notes[1].ToEvent().Key(),
					Metadata:    notes[1].ToEvent(),
				},
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabUnapproved,
					Key:         notes[2].ToEvent().Key(),
					Metadata:    notes[2].ToEvent(),
				},
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabMarkWorkInProgress,
					Key:         notes[3].ToEvent().Key(),
					Metadata:    notes[3].ToEvent(),
				},
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabUnmarkWorkInProgress,
					Key:         notes[4].ToEvent().Key(),
					Metadata:    notes[4].ToEvent(),
				},
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabPipeline,
					Key:         pipelines[0].Key(),
					Metadata:    pipelines[0],
				},
				{
					ChangesetID: 1234,
					Kind:        ChangesetEventKindGitLabPipeline,
					Key:         pipelines[1].Key(),
					Metadata:    pipelines[1],
				},
			},
		})
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			have := tc.changeset.Events()
			want := tc.events

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
