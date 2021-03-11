package store

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func testStoreChangesets(t *testing.T, ctx context.Context, s *Store, clock ct.Clock) {
	githubActor := github.Actor{
		AvatarURL: "https://avatars2.githubusercontent.com/u/1185253",
		Login:     "mrnugget",
		URL:       "https://github.com/mrnugget",
	}
	githubPR := &github.PullRequest{
		ID:           "FOOBARID",
		Title:        "Fix a bunch of bugs",
		Body:         "This fixes a bunch of bugs",
		URL:          "https://github.com/sourcegraph/sourcegraph/pull/12345",
		Number:       12345,
		Author:       githubActor,
		Participants: []github.Actor{githubActor},
		CreatedAt:    clock.Now(),
		UpdatedAt:    clock.Now(),
		HeadRefName:  "batch-changes/test",
	}

	rs := database.ReposWith(s)
	es := database.ExternalServicesWith(s)

	repo := ct.TestRepo(t, es, extsvc.KindGitHub)
	otherRepo := ct.TestRepo(t, es, extsvc.KindGitHub)
	gitlabRepo := ct.TestRepo(t, es, extsvc.KindGitLab)

	if err := rs.Create(ctx, repo, otherRepo, gitlabRepo); err != nil {
		t.Fatal(err)
	}
	deletedRepo := otherRepo.With(types.Opt.RepoDeletedAt(clock.Now()))
	if err := rs.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}

	changesets := make(batches.Changesets, 0, 3)

	deletedRepoChangeset := &batches.Changeset{
		RepoID:              deletedRepo.ID,
		ExternalID:          fmt.Sprintf("foobar-%d", cap(changesets)),
		ExternalServiceType: extsvc.TypeGitHub,
	}

	var (
		added   int32 = 77
		deleted int32 = 88
		changed int32 = 99
	)

	t.Run("Create", func(t *testing.T) {
		var i int
		for i = 0; i < cap(changesets); i++ {
			failureMessage := fmt.Sprintf("failure-%d", i)
			th := &batches.Changeset{
				RepoID:              repo.ID,
				CreatedAt:           clock.Now(),
				UpdatedAt:           clock.Now(),
				Metadata:            githubPR,
				BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: int64(i) + 1}},
				ExternalID:          fmt.Sprintf("foobar-%d", i),
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      fmt.Sprintf("refs/heads/batch-changes/test/%d", i),
				ExternalUpdatedAt:   clock.Now(),
				ExternalState:       batches.ChangesetExternalStateOpen,
				ExternalReviewState: batches.ChangesetReviewStateApproved,
				ExternalCheckState:  batches.ChangesetCheckStatePassed,

				CurrentSpecID:        int64(i) + 1,
				PreviousSpecID:       int64(i) + 1,
				OwnedByBatchChangeID: int64(i) + 1,
				PublicationState:     batches.ChangesetPublicationStatePublished,

				ReconcilerState: batches.ReconcilerStateCompleted,
				FailureMessage:  &failureMessage,
				NumResets:       18,
				NumFailures:     25,

				Closing: true,
			}

			if i != 0 {
				th.PublicationState = batches.ChangesetPublicationStateUnpublished
			}

			// Only set these fields on a subset to make sure that
			// we handle nil pointers correctly
			if i != cap(changesets)-1 {
				th.DiffStatAdded = &added
				th.DiffStatChanged = &changed
				th.DiffStatDeleted = &deleted

				th.StartedAt = clock.Now()
				th.FinishedAt = clock.Now()
				th.ProcessAfter = clock.Now()
			}

			if err := s.CreateChangeset(ctx, th); err != nil {
				t.Fatal(err)
			}

			changesets = append(changesets, th)
		}

		if err := s.CreateChangeset(ctx, deletedRepoChangeset); err != nil {
			t.Fatal(err)
		}

		for _, have := range changesets {
			if have.ID == 0 {
				t.Fatal("id should not be zero")
			}

			if have.IsDeleted() {
				t.Fatal("changeset is deleted")
			}

			if !have.ReconcilerState.Valid() {
				t.Fatalf("reconciler state is invalid: %s", have.ReconcilerState)
			}

			want := have.Clone()

			want.ID = have.ID
			want.CreatedAt = clock.Now()
			want.UpdatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		changeset := &batches.Changeset{
			RepoID:               repo.ID,
			CreatedAt:            clock.Now(),
			UpdatedAt:            clock.Now(),
			Metadata:             githubPR,
			BatchChanges:         []batches.BatchChangeAssoc{{BatchChangeID: 1}},
			ExternalID:           "foobar-123",
			ExternalServiceType:  extsvc.TypeGitHub,
			ExternalBranch:       "refs/heads/batch-changes/test",
			ExternalUpdatedAt:    clock.Now(),
			ExternalState:        batches.ChangesetExternalStateOpen,
			ExternalReviewState:  batches.ChangesetReviewStateApproved,
			ExternalCheckState:   batches.ChangesetCheckStatePassed,
			PreviousSpecID:       1,
			OwnedByBatchChangeID: 1,
			PublicationState:     batches.ChangesetPublicationStatePublished,
			ReconcilerState:      batches.ReconcilerStateCompleted,
			StartedAt:            clock.Now(),
			FinishedAt:           clock.Now(),
			ProcessAfter:         clock.Now(),
		}

		if err := s.UpsertChangeset(ctx, changeset); err != nil {
			t.Fatal(err)
		}

		if changeset.ID == 0 {
			t.Fatal("id should not be zero")
		}

		prev := changeset.Clone()

		if err := s.UpsertChangeset(ctx, changeset); err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(changeset, prev); diff != "" {
			t.Fatal(diff)
		}

		if err := s.DeleteChangeset(ctx, changeset.ID); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ReconcilerState database representation", func(t *testing.T) {
		// batches.ReconcilerStates are defined as "enum" string constants.
		// The string values are uppercase, because that way they can easily be
		// serialized/deserialized in the GraphQL resolvers, since GraphQL
		// expects the `ChangesetReconcilerState` values to be uppercase.
		//
		// But workerutil.Worker expects those values to be lowercase.
		//
		// So, what we do is to lowercase the Changeset.ReconcilerState value
		// before it enters the database and uppercase it when it leaves the
		// DB.
		//
		// If workerutil.Worker supports custom mappings for the state-machine
		// states, we can remove this.

		// This test ensures that the database representation is lowercase.

		queryRawReconcilerState := func(ch *batches.Changeset) (string, error) {
			q := sqlf.Sprintf("SELECT reconciler_state FROM changesets WHERE id = %s", ch.ID)
			rawState, ok, err := basestore.ScanFirstString(s.Query(ctx, q))
			if err != nil || !ok {
				return rawState, err
			}
			return rawState, nil
		}

		for _, ch := range changesets {
			have, err := queryRawReconcilerState(ch)
			if err != nil {
				t.Fatal(err)
			}

			want := strings.ToLower(string(ch.ReconcilerState))
			if have != want {
				t.Fatalf("wrong database representation. want=%q, have=%q", want, have)
			}
		}
	})

	t.Run("GetChangesetExternalIDs", func(t *testing.T) {
		refs := make([]string, len(changesets))
		for i, c := range changesets {
			refs[i] = c.ExternalBranch
		}
		have, err := s.GetChangesetExternalIDs(ctx, repo.ExternalRepo, refs)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"foobar-0", "foobar-1", "foobar-2"}
		if diff := cmp.Diff(want, have); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("GetChangesetExternalIDs no branch", func(t *testing.T) {
		spec := api.ExternalRepoSpec{
			ID:          "external-id",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		}
		have, err := s.GetChangesetExternalIDs(ctx, spec, []string{"foo"})
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		if diff := cmp.Diff(want, have); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("GetChangesetExternalIDs invalid external-id", func(t *testing.T) {
		spec := api.ExternalRepoSpec{
			ID:          "invalid",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "https://github.com/",
		}
		have, err := s.GetChangesetExternalIDs(ctx, spec, []string{"batch-changes/test"})
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		if diff := cmp.Diff(want, have); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("GetChangesetExternalIDs invalid external service id", func(t *testing.T) {
		spec := api.ExternalRepoSpec{
			ID:          "external-id",
			ServiceType: extsvc.TypeGitHub,
			ServiceID:   "invalid",
		}
		have, err := s.GetChangesetExternalIDs(ctx, spec, []string{"batch-changes/test"})
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		if diff := cmp.Diff(want, have); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("Count", func(t *testing.T) {
		t.Run("No options", func(t *testing.T) {
			count, err := s.CountChangesets(ctx, CountChangesetsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, len(changesets); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}

		})

		t.Run("BatchChangeID", func(t *testing.T) {
			count, err := s.CountChangesets(ctx, CountChangesetsOpts{BatchChangeID: 1})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, 1; have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		})

		t.Run("ReconcilerState", func(t *testing.T) {
			completed := batches.ReconcilerStateCompleted
			countCompleted, err := s.CountChangesets(ctx, CountChangesetsOpts{ReconcilerStates: []batches.ReconcilerState{completed}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countCompleted, len(changesets); have != want {
				t.Fatalf("have countCompleted: %d, want: %d", have, want)
			}

			processing := batches.ReconcilerStateProcessing
			countProcessing, err := s.CountChangesets(ctx, CountChangesetsOpts{ReconcilerStates: []batches.ReconcilerState{processing}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countProcessing, 0; have != want {
				t.Fatalf("have countProcessing: %d, want: %d", have, want)
			}
		})

		t.Run("OwnedByBatchChangeID", func(t *testing.T) {
			count, err := s.CountChangesets(ctx, CountChangesetsOpts{OwnedByBatchChangeID: int64(1)})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, 1; have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		for i := 1; i <= len(changesets); i++ {
			opts := ListChangesetsOpts{BatchChangeID: int64(i)}

			ts, next, err := s.ListChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := next, int64(0); have != want {
				t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
			}

			have, want := ts, changesets[i-1:i]
			if len(have) != len(want) {
				t.Fatalf("listed %d changesets, want: %d", len(have), len(want))
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		}

		for i := 1; i <= len(changesets); i++ {
			ts, next, err := s.ListChangesets(ctx, ListChangesetsOpts{LimitOpts: LimitOpts{Limit: i}})
			if err != nil {
				t.Fatal(err)
			}

			{
				have, want := next, int64(0)
				if i < len(changesets) {
					want = changesets[i].ID
				}

				if have != want {
					t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
				}
			}

			{
				have, want := ts, changesets[:i]
				if len(have) != len(want) {
					t.Fatalf("listed %d changesets, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			}
		}

		{
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{IDs: changesets.IDs()})
			if err != nil {
				t.Fatal(err)
			}

			want := changesets
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}

		{
			var cursor int64
			for i := 1; i <= len(changesets); i++ {
				opts := ListChangesetsOpts{Cursor: cursor, LimitOpts: LimitOpts{Limit: 1}}
				have, next, err := s.ListChangesets(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := changesets[i-1 : i]
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}

				cursor = next
			}
		}

		{
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{WithoutDeleted: true})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != len(changesets) {
				t.Fatalf("have 0 changesets. want %d", len(changesets))
			}

			for _, c := range changesets {
				c.SetDeleted()
				c.UpdatedAt = clock.Now()

				if err := s.UpdateChangeset(ctx, c); err != nil {
					t.Fatal(err)
				}
			}

			have, _, err = s.ListChangesets(ctx, ListChangesetsOpts{WithoutDeleted: true})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 0 {
				t.Fatalf("have %d changesets. want 0", len(changesets))
			}
		}

		{
			gitlabMR := &gitlab.MergeRequest{
				ID:        gitlab.ID(1),
				Title:     "Fix a bunch of bugs",
				CreatedAt: gitlab.Time{Time: clock.Now()},
				UpdatedAt: gitlab.Time{Time: clock.Now()},
			}
			gitlabChangeset := &batches.Changeset{
				Metadata:            gitlabMR,
				RepoID:              gitlabRepo.ID,
				ExternalServiceType: extsvc.TypeGitLab,
			}
			if err := s.CreateChangeset(ctx, gitlabChangeset); err != nil {
				t.Fatal(err)
			}
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{ExternalServiceID: "https://gitlab.com/"})
			if err != nil {
				t.Fatal(err)
			}

			want := 1
			if len(have) != want {
				t.Fatalf("have %d changesets; want %d", len(have), want)
			}

			if have[0].ID != gitlabChangeset.ID {
				t.Fatalf("unexpected changeset: have %+v; want %+v", have[0], gitlabChangeset)
			}
			if err := s.DeleteChangeset(ctx, gitlabChangeset.ID); err != nil {
				t.Fatal(err)
			}
		}

		// No Limit should return all Changesets
		{
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 3 {
				t.Fatalf("have %d changesets. want 3", len(have))
			}
		}

		statePublished := batches.ChangesetPublicationStatePublished
		stateUnpublished := batches.ChangesetPublicationStateUnpublished
		stateQueued := batches.ReconcilerStateQueued
		stateCompleted := batches.ReconcilerStateCompleted
		stateOpen := batches.ChangesetExternalStateOpen
		stateClosed := batches.ChangesetExternalStateClosed
		stateApproved := batches.ChangesetReviewStateApproved
		stateChangesRequested := batches.ChangesetReviewStateChangesRequested
		statePassed := batches.ChangesetCheckStatePassed
		stateFailed := batches.ChangesetCheckStateFailed

		filterCases := []struct {
			opts      ListChangesetsOpts
			wantCount int
		}{
			{
				opts: ListChangesetsOpts{
					PublicationState: &statePublished,
				},
				wantCount: 1,
			},
			{
				opts: ListChangesetsOpts{
					PublicationState: &stateUnpublished,
				},
				wantCount: 2,
			},
			{
				opts: ListChangesetsOpts{
					ReconcilerStates: []batches.ReconcilerState{stateQueued},
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ReconcilerStates: []batches.ReconcilerState{stateCompleted},
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					ExternalState: &stateOpen,
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					ExternalState: &stateClosed,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalReviewState: &stateApproved,
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					ExternalReviewState: &stateChangesRequested,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalCheckState: &statePassed,
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					ExternalCheckState: &stateFailed,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalState:      &stateOpen,
					ExternalCheckState: &stateFailed,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalState:       &stateOpen,
					ExternalReviewState: &stateChangesRequested,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					OwnedByBatchChangeID: int64(1),
				},
				wantCount: 1,
			},
		}

		for i, tc := range filterCases {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				have, _, err := s.ListChangesets(ctx, tc.opts)
				if err != nil {
					t.Fatal(err)
				}
				if len(have) != tc.wantCount {
					t.Fatalf("opts: %+v. have %d changesets. want %d", tc.opts, len(have), tc.wantCount)
				}
			})
		}
	})

	t.Run("Null changeset external state", func(t *testing.T) {
		cs := &batches.Changeset{
			RepoID:              repo.ID,
			Metadata:            githubPR,
			BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: 1}},
			ExternalID:          fmt.Sprintf("foobar-%d", 42),
			ExternalServiceType: extsvc.TypeGitHub,
			ExternalBranch:      "refs/heads/batch-changes/test",
			ExternalUpdatedAt:   clock.Now(),
			ExternalState:       "",
			ExternalReviewState: "",
			ExternalCheckState:  "",
		}

		err := s.CreateChangeset(ctx, cs)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := s.DeleteChangeset(ctx, cs.ID)
			if err != nil {
				t.Fatal(err)
			}
		}()

		fromDB, err := s.GetChangeset(ctx, GetChangesetOpts{
			ID: cs.ID,
		})
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(cs.ExternalState, fromDB.ExternalState); diff != "" {
			t.Error(diff)
		}
		if diff := cmp.Diff(cs.ExternalReviewState, fromDB.ExternalReviewState); diff != "" {
			t.Error(diff)
		}
		if diff := cmp.Diff(cs.ExternalCheckState, fromDB.ExternalCheckState); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("ByID", func(t *testing.T) {
			want := changesets[0]
			opts := GetChangesetOpts{ID: want.ID}

			have, err := s.GetChangeset(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("ByExternalID", func(t *testing.T) {
			want := changesets[0]
			opts := GetChangesetOpts{
				ExternalID:          want.ExternalID,
				ExternalServiceType: want.ExternalServiceType,
			}

			have, err := s.GetChangeset(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("ByRepoID", func(t *testing.T) {
			want := changesets[0]
			opts := GetChangesetOpts{
				RepoID: want.RepoID,
			}

			have, err := s.GetChangeset(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := GetChangesetOpts{ID: 0xdeadbeef}

			_, have := s.GetChangeset(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})

		t.Run("RepoDeleted", func(t *testing.T) {
			opts := GetChangesetOpts{ID: deletedRepoChangeset.ID}

			_, have := s.GetChangeset(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})

		t.Run("ExternalBranch", func(t *testing.T) {
			for _, c := range changesets {
				opts := GetChangesetOpts{ExternalBranch: c.ExternalBranch}

				have, err := s.GetChangeset(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				want := c

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			}
		})

		t.Run("ReconcilerState", func(t *testing.T) {
			for _, c := range changesets {
				opts := GetChangesetOpts{ID: c.ID, ReconcilerState: c.ReconcilerState}

				have, err := s.GetChangeset(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				want := c

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				if c.ReconcilerState == batches.ReconcilerStateErrored {
					c.ReconcilerState = batches.ReconcilerStateCompleted
				} else {
					opts.ReconcilerState = batches.ReconcilerStateErrored
				}
				_, err = s.GetChangeset(ctx, opts)
				if err != ErrNoResults {
					t.Fatalf("unexpected error, want=%q have=%q", ErrNoResults, err)
				}
			}
		})

		t.Run("PublicationState", func(t *testing.T) {
			for _, c := range changesets {
				opts := GetChangesetOpts{ID: c.ID, PublicationState: c.PublicationState}

				have, err := s.GetChangeset(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				want := c

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}

				// Toggle publication state
				if c.PublicationState == batches.ChangesetPublicationStateUnpublished {
					opts.PublicationState = batches.ChangesetPublicationStatePublished
				} else {
					opts.PublicationState = batches.ChangesetPublicationStateUnpublished
				}

				_, err = s.GetChangeset(ctx, opts)
				if err != ErrNoResults {
					t.Fatalf("unexpected error, want=%q have=%q", ErrNoResults, err)
				}
			}
		})
	})

	t.Run("Update", func(t *testing.T) {
		want := make([]*batches.Changeset, 0, len(changesets))
		have := make([]*batches.Changeset, 0, len(changesets))

		clock.Add(1 * time.Second)
		for _, c := range changesets {
			c.Metadata = &bitbucketserver.PullRequest{ID: 1234}
			c.ExternalServiceType = extsvc.TypeBitbucketServer

			c.CurrentSpecID = c.CurrentSpecID + 1
			c.PreviousSpecID = c.PreviousSpecID + 1
			c.OwnedByBatchChangeID = c.OwnedByBatchChangeID + 1

			c.PublicationState = batches.ChangesetPublicationStatePublished
			c.ReconcilerState = batches.ReconcilerStateErrored
			c.FailureMessage = nil
			c.StartedAt = clock.Now()
			c.FinishedAt = clock.Now()
			c.ProcessAfter = clock.Now()
			c.NumResets = 987
			c.NumFailures = 789

			clone := c.Clone()
			have = append(have, clone)

			c.UpdatedAt = clock.Now()
			want = append(want, c)

			if err := s.UpdateChangeset(ctx, clone); err != nil {
				t.Fatal(err)
			}
		}

		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatal(diff)
		}

		for i := range have {
			// Test that duplicates are not introduced.
			have[i].BatchChanges = append(have[i].BatchChanges, have[i].BatchChanges...)

			if err := s.UpdateChangeset(ctx, have[i]); err != nil {
				t.Fatal(err)
			}

		}

		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatal(diff)
		}

		for i := range have {
			// Test we can add to the set.
			have[i].BatchChanges = append(have[i].BatchChanges, batches.BatchChangeAssoc{BatchChangeID: 42})
			want[i].BatchChanges = append(want[i].BatchChanges, batches.BatchChangeAssoc{BatchChangeID: 42})

			if err := s.UpdateChangeset(ctx, have[i]); err != nil {
				t.Fatal(err)
			}

		}

		for i := range have {
			sort.Slice(have[i].BatchChanges, func(a, b int) bool {
				return have[i].BatchChanges[a].BatchChangeID < have[i].BatchChanges[b].BatchChangeID
			})

			if diff := cmp.Diff(have[i], want[i]); diff != "" {
				t.Fatal(diff)
			}
		}

		for i := range have {
			// Test we can remove from the set.
			have[i].BatchChanges = have[i].BatchChanges[:0]
			want[i].BatchChanges = want[i].BatchChanges[:0]

			if err := s.UpdateChangeset(ctx, have[i]); err != nil {
				t.Fatal(err)
			}
		}

		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatal(diff)
		}

		clock.Add(1 * time.Second)
		want = want[0:0]
		have = have[0:0]
		for _, c := range changesets {
			c.Metadata = &gitlab.MergeRequest{ID: 1234, IID: 123}
			c.ExternalServiceType = extsvc.TypeGitLab

			clone := c.Clone()
			have = append(have, clone)

			c.UpdatedAt = clock.Now()
			want = append(want, c)

			if err := s.UpdateChangeset(ctx, clone); err != nil {
				t.Fatal(err)
			}

		}

		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("CancelQueuedBatchChangeChangesets", func(t *testing.T) {
		var batchChangeID int64 = 99999

		c1 := ct.CreateChangeset(t, ctx, s, ct.TestChangesetOpts{
			Repo:               repo.ID,
			BatchChange:        batchChangeID,
			OwnedByBatchChange: batchChangeID,
			ReconcilerState:    batches.ReconcilerStateQueued,
		})

		c2 := ct.CreateChangeset(t, ctx, s, ct.TestChangesetOpts{
			Repo:               repo.ID,
			BatchChange:        batchChangeID,
			OwnedByBatchChange: batchChangeID,
			ReconcilerState:    batches.ReconcilerStateErrored,
			NumFailures:        1,
		})

		c3 := ct.CreateChangeset(t, ctx, s, ct.TestChangesetOpts{
			Repo:               repo.ID,
			BatchChange:        batchChangeID,
			OwnedByBatchChange: batchChangeID,
			ReconcilerState:    batches.ReconcilerStateCompleted,
		})

		c4 := ct.CreateChangeset(t, ctx, s, ct.TestChangesetOpts{
			Repo:               repo.ID,
			BatchChange:        batchChangeID,
			OwnedByBatchChange: 0,
			PublicationState:   batches.ChangesetPublicationStateUnpublished,
			ReconcilerState:    batches.ReconcilerStateQueued,
		})

		c5 := ct.CreateChangeset(t, ctx, s, ct.TestChangesetOpts{
			Repo:               repo.ID,
			BatchChange:        batchChangeID,
			OwnedByBatchChange: batchChangeID,
			ReconcilerState:    batches.ReconcilerStateProcessing,
		})

		if err := s.CancelQueuedBatchChangeChangesets(ctx, batchChangeID); err != nil {
			t.Fatal(err)
		}

		ct.ReloadAndAssertChangeset(t, ctx, s, c1, ct.ChangesetAssertions{
			Repo:               repo.ID,
			ReconcilerState:    batches.ReconcilerStateFailed,
			OwnedByBatchChange: batchChangeID,
			FailureMessage:     &CanceledChangesetFailureMessage,
			AttachedTo:         []int64{batchChangeID},
		})

		ct.ReloadAndAssertChangeset(t, ctx, s, c2, ct.ChangesetAssertions{
			Repo:               repo.ID,
			ReconcilerState:    batches.ReconcilerStateFailed,
			OwnedByBatchChange: batchChangeID,
			FailureMessage:     &CanceledChangesetFailureMessage,
			NumFailures:        1,
			AttachedTo:         []int64{batchChangeID},
		})

		ct.ReloadAndAssertChangeset(t, ctx, s, c3, ct.ChangesetAssertions{
			Repo:               repo.ID,
			ReconcilerState:    batches.ReconcilerStateCompleted,
			OwnedByBatchChange: batchChangeID,
			AttachedTo:         []int64{batchChangeID},
		})

		ct.ReloadAndAssertChangeset(t, ctx, s, c4, ct.ChangesetAssertions{
			Repo:             repo.ID,
			ReconcilerState:  batches.ReconcilerStateQueued,
			PublicationState: batches.ChangesetPublicationStateUnpublished,
			AttachedTo:       []int64{batchChangeID},
		})

		ct.ReloadAndAssertChangeset(t, ctx, s, c5, ct.ChangesetAssertions{
			Repo:               repo.ID,
			ReconcilerState:    batches.ReconcilerStateFailed,
			FailureMessage:     &CanceledChangesetFailureMessage,
			OwnedByBatchChange: batchChangeID,
			AttachedTo:         []int64{batchChangeID},
		})
	})

	t.Run("EnqueueChangesetsToClose", func(t *testing.T) {
		var batchChangeID int64 = 99999

		wantEnqueued := ct.ChangesetAssertions{
			Repo:               repo.ID,
			OwnedByBatchChange: batchChangeID,
			ReconcilerState:    batches.ReconcilerStateQueued,
			PublicationState:   batches.ChangesetPublicationStatePublished,
			NumFailures:        0,
			FailureMessage:     nil,
			Closing:            true,
		}

		tests := []struct {
			have ct.TestChangesetOpts
			want ct.ChangesetAssertions
		}{
			{
				have: ct.TestChangesetOpts{
					ReconcilerState:  batches.ReconcilerStateQueued,
					PublicationState: batches.ChangesetPublicationStatePublished,
				},
				want: wantEnqueued,
			},
			{
				have: ct.TestChangesetOpts{
					ReconcilerState:  batches.ReconcilerStateProcessing,
					PublicationState: batches.ChangesetPublicationStatePublished,
				},
				want: wantEnqueued,
			},
			{
				have: ct.TestChangesetOpts{
					ReconcilerState:  batches.ReconcilerStateErrored,
					PublicationState: batches.ChangesetPublicationStatePublished,
					FailureMessage:   "failed",
					NumFailures:      1,
				},
				want: wantEnqueued,
			},
			{
				have: ct.TestChangesetOpts{
					ExternalState:    batches.ChangesetExternalStateOpen,
					ReconcilerState:  batches.ReconcilerStateCompleted,
					PublicationState: batches.ChangesetPublicationStatePublished,
				},
				want: ct.ChangesetAssertions{
					ReconcilerState:  batches.ReconcilerStateQueued,
					PublicationState: batches.ChangesetPublicationStatePublished,
					Closing:          true,
					ExternalState:    batches.ChangesetExternalStateOpen,
				},
			},
			{
				have: ct.TestChangesetOpts{
					ExternalState:    batches.ChangesetExternalStateClosed,
					ReconcilerState:  batches.ReconcilerStateCompleted,
					PublicationState: batches.ChangesetPublicationStatePublished,
				},
				want: ct.ChangesetAssertions{
					ReconcilerState:  batches.ReconcilerStateCompleted,
					ExternalState:    batches.ChangesetExternalStateClosed,
					PublicationState: batches.ChangesetPublicationStatePublished,
				},
			},
			{
				have: ct.TestChangesetOpts{
					ReconcilerState:  batches.ReconcilerStateCompleted,
					PublicationState: batches.ChangesetPublicationStateUnpublished,
				},
				want: ct.ChangesetAssertions{
					ReconcilerState:  batches.ReconcilerStateCompleted,
					PublicationState: batches.ChangesetPublicationStateUnpublished,
				},
			},
		}

		changesets := make(map[*batches.Changeset]ct.ChangesetAssertions)
		for _, tc := range tests {
			opts := tc.have
			opts.Repo = repo.ID
			opts.BatchChange = batchChangeID
			opts.OwnedByBatchChange = batchChangeID

			c := ct.CreateChangeset(t, ctx, s, opts)
			changesets[c] = tc.want
		}

		if err := s.EnqueueChangesetsToClose(ctx, batchChangeID); err != nil {
			t.Fatal(err)
		}

		for changeset, want := range changesets {
			want.Repo = repo.ID
			want.OwnedByBatchChange = batchChangeID
			want.AttachedTo = []int64{batchChangeID}
			ct.ReloadAndAssertChangeset(t, ctx, s, changeset, want)
		}
	})

	t.Run("GetChangesetsStats", func(t *testing.T) {
		currentStats, err := s.GetChangesetsStats(ctx, GetChangesetsStatsOpts{})
		if err != nil {
			t.Fatal(err)
		}
		var batchChangeID int64 = 191918
		currentBatchChangeStats, err := s.GetChangesetsStats(ctx, GetChangesetsStatsOpts{BatchChangeID: batchChangeID})
		if err != nil {
			t.Fatal(err)
		}

		baseOpts := ct.TestChangesetOpts{Repo: repo.ID}

		// Closed changeset
		opts1 := baseOpts
		opts1.BatchChange = batchChangeID
		opts1.ExternalState = batches.ChangesetExternalStateClosed
		opts1.ReconcilerState = batches.ReconcilerStateCompleted
		opts1.PublicationState = batches.ChangesetPublicationStatePublished
		ct.CreateChangeset(t, ctx, s, opts1)

		// Deleted changeset
		opts2 := baseOpts
		opts2.BatchChange = batchChangeID
		opts2.ExternalState = batches.ChangesetExternalStateDeleted
		opts2.ReconcilerState = batches.ReconcilerStateCompleted
		opts2.PublicationState = batches.ChangesetPublicationStatePublished
		ct.CreateChangeset(t, ctx, s, opts2)

		// Open changeset
		opts3 := baseOpts
		opts3.BatchChange = batchChangeID
		opts3.OwnedByBatchChange = batchChangeID
		opts3.ExternalState = batches.ChangesetExternalStateOpen
		opts3.ReconcilerState = batches.ReconcilerStateCompleted
		opts3.PublicationState = batches.ChangesetPublicationStatePublished
		ct.CreateChangeset(t, ctx, s, opts3)

		// Open changeset in a deleted repository
		opts4 := baseOpts
		// In a deleted repository.
		opts4.Repo = deletedRepo.ID
		opts4.BatchChange = batchChangeID
		opts4.ExternalState = batches.ChangesetExternalStateOpen
		opts4.ReconcilerState = batches.ReconcilerStateCompleted
		opts4.PublicationState = batches.ChangesetPublicationStatePublished
		ct.CreateChangeset(t, ctx, s, opts4)

		// Open changeset in a different batch change
		opts5 := baseOpts
		opts5.BatchChange = batchChangeID + 999
		opts5.ExternalState = batches.ChangesetExternalStateOpen
		opts5.ReconcilerState = batches.ReconcilerStateCompleted
		opts5.PublicationState = batches.ChangesetPublicationStatePublished
		ct.CreateChangeset(t, ctx, s, opts5)

		t.Run("global", func(t *testing.T) {
			haveStats, err := s.GetChangesetsStats(ctx, GetChangesetsStatsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			wantStats := currentStats
			wantStats.Open += 2
			wantStats.Closed += 1
			wantStats.Deleted += 1
			wantStats.Total += 4

			if diff := cmp.Diff(wantStats, haveStats); diff != "" {
				t.Fatalf("wrong stats returned. diff=%s", diff)
			}
		})
		t.Run("single ampaign", func(t *testing.T) {
			haveStats, err := s.GetChangesetsStats(ctx, GetChangesetsStatsOpts{BatchChangeID: batchChangeID})
			if err != nil {
				t.Fatal(err)
			}

			wantStats := currentBatchChangeStats
			wantStats.Open += 1
			wantStats.Closed += 1
			wantStats.Deleted += 1
			wantStats.Total += 3

			if diff := cmp.Diff(wantStats, haveStats); diff != "" {
				t.Fatalf("wrong stats returned. diff=%s", diff)
			}
		})
	})
}

func testStoreListChangesetSyncData(t *testing.T, ctx context.Context, s *Store, clock ct.Clock) {
	githubActor := github.Actor{
		AvatarURL: "https://avatars2.githubusercontent.com/u/1185253",
		Login:     "mrnugget",
		URL:       "https://github.com/mrnugget",
	}
	githubPR := &github.PullRequest{
		ID:           "FOOBARID",
		Title:        "Fix a bunch of bugs",
		Body:         "This fixes a bunch of bugs",
		URL:          "https://github.com/sourcegraph/sourcegraph/pull/12345",
		Number:       12345,
		Author:       githubActor,
		Participants: []github.Actor{githubActor},
		CreatedAt:    clock.Now(),
		UpdatedAt:    clock.Now(),
		HeadRefName:  "batch-changes/test",
	}
	gitlabMR := &gitlab.MergeRequest{
		ID:        gitlab.ID(1),
		Title:     "Fix a bunch of bugs",
		CreatedAt: gitlab.Time{Time: clock.Now()},
		UpdatedAt: gitlab.Time{Time: clock.Now()},
	}
	issueComment := &github.IssueComment{
		DatabaseID: 443827703,
		Author: github.Actor{
			AvatarURL: "https://avatars0.githubusercontent.com/u/1976?v=4",
			Login:     "sqs",
			URL:       "https://github.com/sqs",
		},
		Editor:              nil,
		AuthorAssociation:   "MEMBER",
		Body:                "> Just to be sure: you mean the \"searchFilters\" \"Filters\" should be lowercase, not the \"Search Filters\" from the description, right?\r\n\r\nNo, the prose “Search Filters” should have the F lowercased to fit with our style guide preference for sentence case over title case. (Can’t find this comment on the GitHub mobile interface anymore so quoting the email.)",
		URL:                 "https://github.com/sourcegraph/sourcegraph/pull/999#issuecomment-443827703",
		CreatedAt:           clock.Now(),
		UpdatedAt:           clock.Now(),
		IncludesCreatedEdit: false,
	}

	rs := database.ReposWith(s)
	es := database.ExternalServicesWith(s)

	githubRepo := ct.TestRepo(t, es, extsvc.KindGitHub)
	gitlabRepo := ct.TestRepo(t, es, extsvc.KindGitLab)

	if err := rs.Create(ctx, githubRepo, gitlabRepo); err != nil {
		t.Fatal(err)
	}

	changesets := make(batches.Changesets, 0, 3)
	events := make([]*batches.ChangesetEvent, 0)

	for i := 0; i < cap(changesets); i++ {
		ch := &batches.Changeset{
			RepoID:              githubRepo.ID,
			CreatedAt:           clock.Now(),
			UpdatedAt:           clock.Now(),
			Metadata:            githubPR,
			BatchChanges:        []batches.BatchChangeAssoc{{BatchChangeID: int64(i) + 1}},
			ExternalID:          fmt.Sprintf("foobar-%d", i),
			ExternalServiceType: extsvc.TypeGitHub,
			ExternalBranch:      "refs/heads/batch-changes/test",
			ExternalUpdatedAt:   clock.Now(),
			ExternalState:       batches.ChangesetExternalStateOpen,
			ExternalReviewState: batches.ChangesetReviewStateApproved,
			ExternalCheckState:  batches.ChangesetCheckStatePassed,
			PublicationState:    batches.ChangesetPublicationStatePublished,
			ReconcilerState:     batches.ReconcilerStateCompleted,
		}

		if i == cap(changesets)-1 {
			ch.Metadata = gitlabMR
			ch.ExternalServiceType = extsvc.TypeGitLab
			ch.RepoID = gitlabRepo.ID
		}

		if err := s.CreateChangeset(ctx, ch); err != nil {
			t.Fatal(err)
		}

		changesets = append(changesets, ch)
	}

	// We need batch changes attached to each changeset
	for _, cs := range changesets {
		c := &batches.BatchChange{
			Name:           "ListChangesetSyncData test",
			NamespaceOrgID: 23,
			LastApplierID:  1,
			LastAppliedAt:  time.Now(),
			BatchSpecID:    42,
		}
		err := s.CreateBatchChange(ctx, c)
		if err != nil {
			t.Fatal(err)
		}

		cs.BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: c.ID}}

		if err := s.UpdateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
	}

	// The changesets, except one, get changeset events
	for _, cs := range changesets[:len(changesets)-1] {
		e := &batches.ChangesetEvent{
			ChangesetID: cs.ID,
			Kind:        batches.ChangesetEventKindGitHubCommented,
			Key:         issueComment.Key(),
			CreatedAt:   clock.Now(),
			Metadata:    issueComment,
		}

		events = append(events, e)
	}
	if err := s.UpsertChangesetEvents(ctx, events...); err != nil {
		t.Fatal(err)
	}

	checkChangesetIDs := func(t *testing.T, hs []*batches.ChangesetSyncData, want []int64) {
		t.Helper()

		haveIDs := []int64{}
		for _, sd := range hs {
			haveIDs = append(haveIDs, sd.ChangesetID)
		}
		if diff := cmp.Diff(want, haveIDs); diff != "" {
			t.Fatalf("wrong changesetIDs in changeset sync data (-want +got):\n%s", diff)
		}
	}

	t.Run("success", func(t *testing.T) {
		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{})
		if err != nil {
			t.Fatal(err)
		}
		want := []*batches.ChangesetSyncData{
			{
				ChangesetID:           changesets[0].ID,
				UpdatedAt:             clock.Now(),
				LatestEvent:           clock.Now(),
				ExternalUpdatedAt:     clock.Now(),
				RepoExternalServiceID: "https://github.com/",
			},
			{
				ChangesetID:           changesets[1].ID,
				UpdatedAt:             clock.Now(),
				LatestEvent:           clock.Now(),
				ExternalUpdatedAt:     clock.Now(),
				RepoExternalServiceID: "https://github.com/",
			},
			{
				// No events
				ChangesetID:           changesets[2].ID,
				UpdatedAt:             clock.Now(),
				ExternalUpdatedAt:     clock.Now(),
				RepoExternalServiceID: "https://gitlab.com/",
			},
		}
		if diff := cmp.Diff(want, hs); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("only for specific external service", func(t *testing.T) {
		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{ExternalServiceID: "https://gitlab.com/"})
		if err != nil {
			t.Fatal(err)
		}
		want := []*batches.ChangesetSyncData{
			{
				ChangesetID:           changesets[2].ID,
				UpdatedAt:             clock.Now(),
				ExternalUpdatedAt:     clock.Now(),
				RepoExternalServiceID: "https://gitlab.com/",
			},
		}
		if diff := cmp.Diff(want, hs); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("ignore closed batch change", func(t *testing.T) {
		closedBatchChangeID := changesets[0].BatchChanges[0].BatchChangeID
		c, err := s.GetBatchChange(ctx, CountBatchChangeOpts{ID: closedBatchChangeID})
		if err != nil {
			t.Fatal(err)
		}
		c.ClosedAt = clock.Now()
		err = s.UpdateBatchChange(ctx, c)
		if err != nil {
			t.Fatal(err)
		}

		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{})
		if err != nil {
			t.Fatal(err)
		}
		checkChangesetIDs(t, hs, changesets[1:].IDs())

		// If a changeset has ANY open batch changes we should list it
		// Attach cs1 to both an open and closed batch change
		openBatchChangeID := changesets[1].BatchChanges[0].BatchChangeID
		changesets[0].BatchChanges = []batches.BatchChangeAssoc{{BatchChangeID: closedBatchChangeID}, {BatchChangeID: openBatchChangeID}}
		err = s.UpdateChangeset(ctx, changesets[0])
		if err != nil {
			t.Fatal(err)
		}

		hs, err = s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{})
		if err != nil {
			t.Fatal(err)
		}
		checkChangesetIDs(t, hs, changesets.IDs())
	})

	t.Run("ignore processing changesets", func(t *testing.T) {
		ch := changesets[0]
		ch.PublicationState = batches.ChangesetPublicationStatePublished
		ch.ReconcilerState = batches.ReconcilerStateProcessing
		if err := s.UpdateChangeset(ctx, ch); err != nil {
			t.Fatal(err)
		}

		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{})
		if err != nil {
			t.Fatal(err)
		}
		checkChangesetIDs(t, hs, changesets[1:].IDs())
	})

	t.Run("ignore unpublished changesets", func(t *testing.T) {
		ch := changesets[0]
		ch.PublicationState = batches.ChangesetPublicationStateUnpublished
		ch.ReconcilerState = batches.ReconcilerStateCompleted
		if err := s.UpdateChangeset(ctx, ch); err != nil {
			t.Fatal(err)
		}

		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{})
		if err != nil {
			t.Fatal(err)
		}
		checkChangesetIDs(t, hs, changesets[1:].IDs())
	})
}

func testStoreListChangesetsTextSearch(t *testing.T, ctx context.Context, s *Store, clock ct.Clock) {
	// This is similar to the setup in testStoreChangesets(), but we need a more
	// fine grained set of changesets to handle the different scenarios. Namely,
	// we need to cover:
	//
	// 1. Metadata from each code host type to test title search.
	// 2. Unpublished changesets that don't have metadata to test the title
	//    search fallback to the spec title.
	// 3. Repo name search.
	// 4. Negation of all of the above.

	// Let's define some helpers.
	createChangesetSpec := func(title string) *batches.ChangesetSpec {
		spec := &batches.ChangesetSpec{
			Spec: &batches.ChangesetSpecDescription{
				Title: title,
			},
		}
		if err := s.CreateChangesetSpec(ctx, spec); err != nil {
			t.Fatalf("creating changeset spec: %v", err)
		}
		return spec
	}

	createChangeset := func(
		esType string,
		repo *types.Repo,
		externalID string,
		metadata interface{},
		spec *batches.ChangesetSpec,
	) *batches.Changeset {
		var specID int64
		if spec != nil {
			specID = spec.ID
		}

		cs := &batches.Changeset{
			RepoID:              repo.ID,
			CreatedAt:           clock.Now(),
			UpdatedAt:           clock.Now(),
			Metadata:            metadata,
			ExternalID:          externalID,
			ExternalServiceType: esType,
			ExternalBranch:      "refs/heads/batch-changes/test",
			ExternalUpdatedAt:   clock.Now(),
			ExternalState:       batches.ChangesetExternalStateOpen,
			ExternalReviewState: batches.ChangesetReviewStateApproved,
			ExternalCheckState:  batches.ChangesetCheckStatePassed,

			CurrentSpecID:    specID,
			PublicationState: batches.ChangesetPublicationStatePublished,
		}

		if err := s.CreateChangeset(ctx, cs); err != nil {
			t.Fatalf("creating changeset:\nerr: %+v\nchangeset: %+v", err, cs)
		}
		return cs
	}

	rs := database.ReposWith(s)
	es := database.ExternalServicesWith(s)

	// Set up repositories for each code host type we want to test.
	var (
		githubRepo = ct.TestRepo(t, es, extsvc.KindGitHub)
		bbsRepo    = ct.TestRepo(t, es, extsvc.KindBitbucketServer)
		gitlabRepo = ct.TestRepo(t, es, extsvc.KindGitLab)
	)
	if err := rs.Create(ctx, githubRepo, bbsRepo, gitlabRepo); err != nil {
		t.Fatal(err)
	}

	// Now let's create ourselves some changesets to test against.
	githubActor := github.Actor{
		AvatarURL: "https://avatars2.githubusercontent.com/u/1185253",
		Login:     "mrnugget",
		URL:       "https://github.com/mrnugget",
	}

	githubChangeset := createChangeset(
		extsvc.TypeGitHub,
		githubRepo,
		"12345",
		&github.PullRequest{
			ID:           "FOOBARID",
			Title:        "Fix a bunch of bugs on GitHub",
			Body:         "This fixes a bunch of bugs",
			URL:          "https://github.com/sourcegraph/sourcegraph/pull/12345",
			Number:       12345,
			Author:       githubActor,
			Participants: []github.Actor{githubActor},
			CreatedAt:    clock.Now(),
			UpdatedAt:    clock.Now(),
			HeadRefName:  "batch-changes/test",
		},
		createChangesetSpec("Fix a bunch of bugs"),
	)

	gitlabChangeset := createChangeset(
		extsvc.TypeGitLab,
		gitlabRepo,
		"12345",
		&gitlab.MergeRequest{
			ID:           12345,
			IID:          12345,
			ProjectID:    123,
			Title:        "Fix a bunch of bugs on GitLab",
			Description:  "This fixes a bunch of bugs",
			State:        gitlab.MergeRequestStateOpened,
			WebURL:       "https://gitlab.org/sourcegraph/sourcegraph/pull/12345",
			SourceBranch: "batch-changes/test",
		},
		createChangesetSpec("Fix a bunch of bugs"),
	)

	bbsChangeset := createChangeset(
		extsvc.TypeBitbucketServer,
		bbsRepo,
		"12345",
		&bitbucketserver.PullRequest{
			ID:          12345,
			Version:     1,
			Title:       "Fix a bunch of bugs on Bitbucket Server",
			Description: "This fixes a bunch of bugs",
			State:       "open",
			Open:        true,
			Closed:      false,
			FromRef:     bitbucketserver.Ref{ID: "batch-changes/test"},
		},
		createChangesetSpec("Fix a bunch of bugs"),
	)

	unpublishedChangeset := createChangeset(
		extsvc.TypeGitHub,
		githubRepo,
		"",
		map[string]interface{}{},
		createChangesetSpec("Eventually fix some bugs, but not a bunch"),
	)

	importedChangeset := createChangeset(
		extsvc.TypeGitHub,
		githubRepo,
		"123456",
		&github.PullRequest{
			ID:           "XYZ",
			Title:        "Do some stuff",
			Body:         "This does some stuff",
			URL:          "https://github.com/sourcegraph/sourcegraph/pull/123456",
			Number:       123456,
			Author:       githubActor,
			Participants: []github.Actor{githubActor},
			CreatedAt:    clock.Now(),
			UpdatedAt:    clock.Now(),
			HeadRefName:  "batch-changes/stuff",
		},
		nil,
	)

	// All right, let's run some searches!
	for name, tc := range map[string]struct {
		textSearch []search.TextSearchTerm
		want       batches.Changesets
	}{
		"single changeset based on GitHub metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on GitHub"},
			},
			want: batches.Changesets{githubChangeset},
		},
		"single changeset based on GitLab metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on GitLab"},
			},
			want: batches.Changesets{gitlabChangeset},
		},
		"single changeset based on Bitbucket Server metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on Bitbucket Server"},
			},
			want: batches.Changesets{bbsChangeset},
		},
		"all published changesets based on metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Fix a bunch of bugs"},
			},
			want: batches.Changesets{
				githubChangeset,
				gitlabChangeset,
				bbsChangeset,
			},
		},
		"imported changeset based on metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Do some stuff"},
			},
			want: batches.Changesets{importedChangeset},
		},
		"unpublished changeset based on spec title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually"},
			},
			want: batches.Changesets{unpublishedChangeset},
		},
		"negated metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "bunch of bugs", Not: true},
			},
			want: batches.Changesets{
				unpublishedChangeset,
				importedChangeset,
			},
		},
		"negated spec title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually", Not: true},
			},
			want: batches.Changesets{
				githubChangeset,
				gitlabChangeset,
				bbsChangeset,
				importedChangeset,
			},
		},
		"repo name": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name)},
			},
			want: batches.Changesets{
				githubChangeset,
				unpublishedChangeset,
				importedChangeset,
			},
		},
		"title and repo name together": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name)},
				{Term: "Eventually"},
			},
			want: batches.Changesets{
				unpublishedChangeset,
			},
		},
		"multiple title matches together": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually"},
				{Term: "fix"},
			},
			want: batches.Changesets{
				unpublishedChangeset,
			},
		},
		"negated repo name": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name), Not: true},
			},
			want: batches.Changesets{
				gitlabChangeset,
				bbsChangeset,
			},
		},
		"combined negated repo names": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name), Not: true},
				{Term: string(gitlabRepo.Name), Not: true},
			},
			want: batches.Changesets{bbsChangeset},
		},
		"no results due to conflicting requirements": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name)},
				{Term: string(gitlabRepo.Name)},
			},
			want: batches.Changesets{},
		},
		"no results due to a subset of a word": {
			textSearch: []search.TextSearchTerm{
				{Term: "unch"},
			},
			want: batches.Changesets{},
		},
		"no results due to text that doesn't exist in the search scope": {
			textSearch: []search.TextSearchTerm{
				{Term: "she dreamt she was a bulldozer, she dreamt she was in an empty field"},
			},
			want: batches.Changesets{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{
				TextSearch: tc.textSearch,
			})
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			if diff := cmp.Diff(tc.want, have); diff != "" {
				t.Errorf("unexpected result (-want +have):\n%s", diff)
			}
		})
	}
}
