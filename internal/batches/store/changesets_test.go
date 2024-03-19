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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/search"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func testStoreChangesets(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	user := bt.CreateTestUser(t, s.DatabaseDB(), false)
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

	rs := database.ReposWith(logger, s)
	es := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, es, extsvc.KindGitHub)
	otherRepo := bt.TestRepo(t, es, extsvc.KindGitHub)
	gitlabRepo := bt.TestRepo(t, es, extsvc.KindGitLab)
	deletedRepo := bt.TestRepo(t, es, extsvc.KindBitbucketCloud)

	if err := rs.Create(ctx, repo, otherRepo, gitlabRepo, deletedRepo); err != nil {
		t.Fatal(err)
	}
	if err := rs.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}

	updateForThisTest := func(t *testing.T, original *btypes.Changeset, mutate func(*btypes.Changeset)) *btypes.Changeset {
		clone := original.Clone()
		mutate(clone)

		if err := s.UpdateChangeset(ctx, clone); err != nil {
			t.Fatal(err)
		}

		t.Cleanup(func() {
			if err := s.UpdateChangeset(ctx, original); err != nil {
				t.Fatal(err)
			}
		})
		return clone
	}

	changesets := make(btypes.Changesets, 0, 3)

	deletedRepoChangeset := &btypes.Changeset{
		RepoID:              deletedRepo.ID,
		ExternalID:          fmt.Sprintf("foobar-%d", cap(changesets)),
		ExternalServiceType: extsvc.TypeGitHub,
	}

	var (
		added   int32 = 77
		deleted int32 = 88
	)

	t.Run("Create", func(t *testing.T) {
		var i int
		for i = 0; i < cap(changesets); i++ {
			failureMessage := fmt.Sprintf("failure-%d", i)
			th := &btypes.Changeset{
				RepoID:              repo.ID,
				CreatedAt:           clock.Now(),
				UpdatedAt:           clock.Now(),
				Metadata:            githubPR,
				BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: int64(i) + 1}},
				ExternalID:          fmt.Sprintf("foobar-%d", i),
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      fmt.Sprintf("refs/heads/batch-changes/test/%d", i),
				ExternalUpdatedAt:   clock.Now(),
				ExternalState:       btypes.ChangesetExternalStateOpen,
				ExternalReviewState: btypes.ChangesetReviewStateApproved,
				ExternalCheckState:  btypes.ChangesetCheckStatePassed,

				CurrentSpecID:        int64(i) + 1,
				PreviousSpecID:       int64(i) + 1,
				OwnedByBatchChangeID: int64(i) + 1,
				PublicationState:     btypes.ChangesetPublicationStatePublished,

				ReconcilerState: btypes.ReconcilerStateCompleted,
				FailureMessage:  &failureMessage,
				NumResets:       18,
				NumFailures:     25,

				Closing: true,
			}

			if i != 0 {
				th.PublicationState = btypes.ChangesetPublicationStateUnpublished
			}

			// Only set these fields on a subset to make sure that
			// we handle nil pointers correctly
			if i != cap(changesets)-1 {
				th.DiffStatAdded = &added
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

	t.Run("UpdateForApply", func(t *testing.T) {
		changeset := &btypes.Changeset{
			RepoID:               repo.ID,
			CreatedAt:            clock.Now(),
			UpdatedAt:            clock.Now(),
			Metadata:             githubPR,
			BatchChanges:         []btypes.BatchChangeAssoc{{BatchChangeID: 1}},
			ExternalID:           "foobar-123",
			ExternalServiceType:  extsvc.TypeGitHub,
			ExternalBranch:       "refs/heads/batch-changes/test",
			ExternalUpdatedAt:    clock.Now(),
			ExternalState:        btypes.ChangesetExternalStateOpen,
			ExternalReviewState:  btypes.ChangesetReviewStateApproved,
			ExternalCheckState:   btypes.ChangesetCheckStatePassed,
			PreviousSpecID:       1,
			OwnedByBatchChangeID: 1,
			PublicationState:     btypes.ChangesetPublicationStatePublished,
			ReconcilerState:      btypes.ReconcilerStateCompleted,
			StartedAt:            clock.Now(),
			FinishedAt:           clock.Now(),
			ProcessAfter:         clock.Now(),
		}

		err := s.CreateChangeset(ctx, changeset)
		require.NoError(t, err)

		assert.NotZero(t, changeset.ID)

		prev := changeset.Clone()

		err = s.UpdateChangesetsForApply(ctx, []*btypes.Changeset{changeset})
		require.NoError(t, err)

		if diff := cmp.Diff(changeset, prev); diff != "" {
			t.Fatal(diff)
		}

		err = s.DeleteChangeset(ctx, changeset.ID)
		require.NoError(t, err)
	})

	t.Run("ReconcilerState database representation", func(t *testing.T) {
		// btypes.ReconcilerStates are defined as "enum" string constants.
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

		queryRawReconcilerState := func(ch *btypes.Changeset) (string, error) {
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
			completed := btypes.ReconcilerStateCompleted
			countCompleted, err := s.CountChangesets(ctx, CountChangesetsOpts{ReconcilerStates: []btypes.ReconcilerState{completed}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countCompleted, len(changesets); have != want {
				t.Fatalf("have countCompleted: %d, want: %d", have, want)
			}

			processing := btypes.ReconcilerStateProcessing
			countProcessing, err := s.CountChangesets(ctx, CountChangesetsOpts{ReconcilerStates: []btypes.ReconcilerState{processing}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countProcessing, 0; have != want {
				t.Fatalf("have countProcessing: %d, want: %d", have, want)
			}
		})

		t.Run("PublicationState", func(t *testing.T) {
			published := btypes.ChangesetPublicationStatePublished
			countPublished, err := s.CountChangesets(ctx, CountChangesetsOpts{PublicationState: &published})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countPublished, 1; have != want {
				t.Fatalf("have countPublished: %d, want: %d", have, want)
			}

			unpublished := btypes.ChangesetPublicationStateUnpublished
			countUnpublished, err := s.CountChangesets(ctx, CountChangesetsOpts{PublicationState: &unpublished})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countUnpublished, len(changesets)-1; have != want {
				t.Fatalf("have countUnpublished: %d, want: %d", have, want)
			}
		})

		t.Run("State", func(t *testing.T) {
			countOpen, err := s.CountChangesets(ctx, CountChangesetsOpts{States: []btypes.ChangesetState{btypes.ChangesetStateOpen}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countOpen, 1; have != want {
				t.Fatalf("have countOpen: %d, want: %d", have, want)
			}

			countClosed, err := s.CountChangesets(ctx, CountChangesetsOpts{States: []btypes.ChangesetState{btypes.ChangesetStateClosed}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countClosed, 0; have != want {
				t.Fatalf("have countClosed: %d, want: %d", have, want)
			}

			countUnpublished, err := s.CountChangesets(ctx, CountChangesetsOpts{States: []btypes.ChangesetState{btypes.ChangesetStateUnpublished}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countUnpublished, 2; have != want {
				t.Fatalf("have countUnpublished: %d, want: %d", have, want)
			}

			countOpenAndUnpublished, err := s.CountChangesets(ctx, CountChangesetsOpts{States: []btypes.ChangesetState{btypes.ChangesetStateOpen, btypes.ChangesetStateUnpublished}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countOpenAndUnpublished, 3; have != want {
				t.Fatalf("have countOpenAndUnpublished: %d, want: %d", have, want)
			}
		})

		t.Run("TextSearch", func(t *testing.T) {
			countMatchingString, err := s.CountChangesets(ctx, CountChangesetsOpts{TextSearch: []search.TextSearchTerm{{Term: "Fix a bunch"}}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countMatchingString, len(changesets); have != want {
				t.Fatalf("have countMatchingString: %d, want: %d", have, want)
			}

			countNotMatchingString, err := s.CountChangesets(ctx, CountChangesetsOpts{TextSearch: []search.TextSearchTerm{{Term: "Very not in the title"}}})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countNotMatchingString, 0; have != want {
				t.Fatalf("have countNotMatchingString: %d, want: %d", have, want)
			}
		})

		t.Run("EnforceAuthz", func(t *testing.T) {
			// No access to repos.
			bt.MockRepoPermissions(t, s.DatabaseDB(), user.ID)
			countAccessible, err := s.CountChangesets(ctx, CountChangesetsOpts{EnforceAuthz: true})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := countAccessible, 0; have != want {
				t.Fatalf("have countAccessible: %d, want: %d", have, want)
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

		t.Run("OnlyArchived", func(t *testing.T) {
			// Changeset is archived
			archivedChangeset := updateForThisTest(t, changesets[0], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].IsArchived = true
			})

			// This changeset is marked as to-be-archived
			_ = updateForThisTest(t, changesets[1], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].BatchChangeID = archivedChangeset.BatchChanges[0].BatchChangeID
				ch.BatchChanges[0].Archive = true
			})

			opts := CountChangesetsOpts{
				OnlyArchived:  true,
				BatchChangeID: archivedChangeset.BatchChanges[0].BatchChangeID,
			}
			count, err := s.CountChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if count != 2 {
				t.Fatalf("got count %d, want: %d", count, 2)
			}

			opts.OnlyArchived = false
			count, err = s.CountChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("got count %d, want: %d", count, 1)
			}
		})

		t.Run("IncludeArchived", func(t *testing.T) {
			// Changeset is archived
			archivedChangeset := updateForThisTest(t, changesets[0], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].IsArchived = true
			})

			// Not archived, not marked as to-be-archived
			_ = updateForThisTest(t, changesets[1], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].BatchChangeID = archivedChangeset.BatchChanges[0].BatchChangeID
				ch.BatchChanges[0].IsArchived = false
			})

			// Marked as to-be-archived
			_ = updateForThisTest(t, changesets[2], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].BatchChangeID = archivedChangeset.BatchChanges[0].BatchChangeID
				ch.BatchChanges[0].Archive = true
			})

			opts := CountChangesetsOpts{
				IncludeArchived: true,
				BatchChangeID:   archivedChangeset.BatchChanges[0].BatchChangeID,
			}
			count, err := s.CountChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if count != 3 {
				t.Fatalf("got count %d, want: %d", count, 3)
			}

			opts.IncludeArchived = false
			count, err = s.CountChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("got count %d, want: %d", count, 1)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("BatchChangeID", func(t *testing.T) {
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
		})

		t.Run("OnlyArchived", func(t *testing.T) {
			archivedChangeset := updateForThisTest(t, changesets[0], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].IsArchived = true
			})

			opts := ListChangesetsOpts{
				OnlyArchived:  true,
				BatchChangeID: archivedChangeset.BatchChanges[0].BatchChangeID,
			}
			cs, _, err := s.ListChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs) != 1 {
				t.Fatalf("listed %d changesets, want: %d", len(cs), 1)
			}
			if cs[0].ID != archivedChangeset.ID {
				t.Errorf("want changeset %d, but got %d", archivedChangeset.ID, cs[0].ID)
			}

			// If OnlyArchived = false, archived changesets should not be included
			opts.OnlyArchived = false
			cs, _, err = s.ListChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs) != 0 {
				t.Fatalf("listed %d changesets, want: %d", len(cs), 1)
			}
		})

		t.Run("IncludeArchived", func(t *testing.T) {
			archivedChangeset := updateForThisTest(t, changesets[0], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].IsArchived = true
			})
			_ = updateForThisTest(t, changesets[1], func(ch *btypes.Changeset) {
				ch.BatchChanges[0].BatchChangeID = archivedChangeset.BatchChanges[0].BatchChangeID
				ch.BatchChanges[0].IsArchived = false
			})

			opts := ListChangesetsOpts{
				IncludeArchived: true,
				BatchChangeID:   archivedChangeset.BatchChanges[0].BatchChangeID,
			}
			cs, _, err := s.ListChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs) != 2 {
				t.Fatalf("listed %d changesets, want: %d", len(cs), 1)
			}

			opts.IncludeArchived = false
			cs, _, err = s.ListChangesets(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if len(cs) != 1 {
				t.Fatalf("listed %d changesets, want: %d", len(cs), 1)
			}
		})

		t.Run("Limit", func(t *testing.T) {
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
		})

		t.Run("IDs", func(t *testing.T) {
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{IDs: changesets.IDs()})
			if err != nil {
				t.Fatal(err)
			}

			want := changesets
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("Cursor pagination", func(t *testing.T) {
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
		})

		// No Limit should return all Changesets
		t.Run("No limit", func(t *testing.T) {
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 3 {
				t.Fatalf("have %d changesets. want 3", len(have))
			}
		})

		t.Run("EnforceAuthz", func(t *testing.T) {
			// No access to repos.
			bt.MockRepoPermissions(t, s.DatabaseDB(), user.ID)
			have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{EnforceAuthz: true})
			if err != nil {
				t.Fatal(err)
			}

			if len(have) != 0 {
				t.Fatalf("have %d changesets. want 0", len(have))
			}
		})

		t.Run("RepoIDs", func(t *testing.T) {
			// Insert two changesets temporarily that are attached to other repos.
			createRepoChangeset := func(repo *types.Repo, baseChangeset *btypes.Changeset) *btypes.Changeset {
				t.Helper()

				c := baseChangeset.Clone()
				c.RepoID = repo.ID
				require.NoError(t, s.CreateChangeset(ctx, c))
				t.Cleanup(func() { s.DeleteChangeset(ctx, c.ID) })

				return c
			}

			otherChangeset := createRepoChangeset(otherRepo, changesets[1])
			gitlabChangeset := createRepoChangeset(gitlabRepo, changesets[1])

			t.Run("single repo", func(t *testing.T) {
				have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{
					RepoIDs: []api.RepoID{repo.ID},
				})
				assert.NoError(t, err)
				assert.ElementsMatch(t, changesets, have)
			})

			t.Run("multiple repos", func(t *testing.T) {
				have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{
					RepoIDs: []api.RepoID{otherRepo.ID, gitlabRepo.ID},
				})
				assert.NoError(t, err)
				assert.ElementsMatch(t, []*btypes.Changeset{otherChangeset, gitlabChangeset}, have)
			})

			t.Run("repo without changesets", func(t *testing.T) {
				have, _, err := s.ListChangesets(ctx, ListChangesetsOpts{
					RepoIDs: []api.RepoID{deletedRepo.ID},
				})
				assert.NoError(t, err)
				assert.ElementsMatch(t, []*btypes.Changeset{}, have)
			})
		})

		statePublished := btypes.ChangesetPublicationStatePublished
		stateUnpublished := btypes.ChangesetPublicationStateUnpublished
		stateQueued := btypes.ReconcilerStateQueued
		stateCompleted := btypes.ReconcilerStateCompleted
		stateOpen := btypes.ChangesetExternalStateOpen
		stateClosed := btypes.ChangesetExternalStateClosed
		stateApproved := btypes.ChangesetReviewStateApproved
		stateChangesRequested := btypes.ChangesetReviewStateChangesRequested
		statePassed := btypes.ChangesetCheckStatePassed
		stateFailed := btypes.ChangesetCheckStateFailed

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
					States: []btypes.ChangesetState{btypes.ChangesetStateUnpublished},
				},
				wantCount: 2,
			},
			{
				opts: ListChangesetsOpts{
					PublicationState: &stateUnpublished,
				},
				wantCount: 2,
			},
			{
				opts: ListChangesetsOpts{
					ReconcilerStates: []btypes.ReconcilerState{stateQueued},
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ReconcilerStates: []btypes.ReconcilerState{stateCompleted},
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					ExternalStates: []btypes.ChangesetExternalState{stateOpen},
				},
				wantCount: 3,
			},
			{
				opts: ListChangesetsOpts{
					States: []btypes.ChangesetState{btypes.ChangesetStateOpen},
				},
				wantCount: 1,
			},
			{
				opts: ListChangesetsOpts{
					ExternalStates: []btypes.ChangesetExternalState{stateClosed},
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalStates: []btypes.ChangesetExternalState{stateOpen, stateClosed},
				},
				wantCount: 3,
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
					ExternalStates:     []btypes.ChangesetExternalState{stateOpen},
					ExternalCheckState: &stateFailed,
				},
				wantCount: 0,
			},
			{
				opts: ListChangesetsOpts{
					ExternalStates:      []btypes.ChangesetExternalState{stateOpen},
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
			t.Run("States_"+strconv.Itoa(i), func(t *testing.T) {
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
		cs := &btypes.Changeset{
			RepoID:              repo.ID,
			Metadata:            githubPR,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: 1}},
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

				if c.ReconcilerState == btypes.ReconcilerStateErrored {
					c.ReconcilerState = btypes.ReconcilerStateCompleted
				} else {
					opts.ReconcilerState = btypes.ReconcilerStateErrored
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
				if c.PublicationState == btypes.ChangesetPublicationStateUnpublished {
					opts.PublicationState = btypes.ChangesetPublicationStatePublished
				} else {
					opts.PublicationState = btypes.ChangesetPublicationStateUnpublished
				}

				_, err = s.GetChangeset(ctx, opts)
				if err != ErrNoResults {
					t.Fatalf("unexpected error, want=%q have=%q", ErrNoResults, err)
				}
			}
		})
	})

	t.Run("Update", func(t *testing.T) {
		want := make([]*btypes.Changeset, 0, len(changesets))
		have := make([]*btypes.Changeset, 0, len(changesets))

		clock.Add(1 * time.Second)
		for _, c := range changesets {
			c.Metadata = &bitbucketserver.PullRequest{ID: 1234}
			c.ExternalServiceType = extsvc.TypeBitbucketServer

			c.CurrentSpecID = c.CurrentSpecID + 1
			c.PreviousSpecID = c.PreviousSpecID + 1
			c.OwnedByBatchChangeID = c.OwnedByBatchChangeID + 1

			c.PublicationState = btypes.ChangesetPublicationStatePublished
			c.ReconcilerState = btypes.ReconcilerStateErrored
			c.PreviousFailureMessage = c.FailureMessage
			c.FailureMessage = nil
			c.StartedAt = clock.Now()
			c.FinishedAt = clock.Now()
			c.ProcessAfter = clock.Now()
			c.NumResets = 987
			c.NumFailures = 789

			c.DetachedAt = clock.Now()

			clone := c.Clone()
			have = append(have, clone)

			c.UpdatedAt = clock.Now()
			c.State = btypes.ChangesetStateRetrying
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
			have[i].BatchChanges = append(have[i].BatchChanges, btypes.BatchChangeAssoc{BatchChangeID: 42})
			want[i].BatchChanges = append(want[i].BatchChanges, btypes.BatchChangeAssoc{BatchChangeID: 42})

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

	t.Run("UpdateChangesetCodeHostState", func(t *testing.T) {
		unpublished := btypes.ChangesetUiPublicationStateUnpublished
		published := btypes.ChangesetUiPublicationStatePublished
		cs := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
			Repo:                repo.ID,
			BatchChange:         123,
			CurrentSpec:         123,
			PreviousSpec:        123,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: 123}},
			ExternalServiceType: "github",
			ExternalID:          "123",
			ExternalBranch:      "refs/heads/branch",
			ExternalState:       btypes.ChangesetExternalStateOpen,
			ExternalReviewState: btypes.ChangesetReviewStatePending,
			ExternalCheckState:  btypes.ChangesetCheckStatePending,
			DiffStatAdded:       10,
			DiffStatDeleted:     10,
			PublicationState:    btypes.ChangesetPublicationStateUnpublished,
			UiPublicationState:  &unpublished,
			ReconcilerState:     btypes.ReconcilerStateQueued,
			FailureMessage:      "very bad",
			NumFailures:         10,
			OwnedByBatchChange:  123,
			Metadata:            &github.PullRequest{Title: "Se titel"},
		})

		cs.ExternalBranch = "refs/heads/branch-2"
		cs.ExternalState = btypes.ChangesetExternalStateDeleted
		cs.ExternalReviewState = btypes.ChangesetReviewStateApproved
		cs.ExternalCheckState = btypes.ChangesetCheckStateFailed
		cs.DiffStatAdded = pointers.Ptr(int32(100))
		cs.DiffStatDeleted = pointers.Ptr(int32(100))
		cs.Metadata = &github.PullRequest{Title: "The title"}
		want := cs.Clone()

		// These should not be updated.
		cs.RepoID = gitlabRepo.ID
		cs.CurrentSpecID = 234
		cs.PreviousSpecID = 234
		cs.BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: 234}}
		cs.ExternalID = "234"
		cs.PublicationState = btypes.ChangesetPublicationStatePublished
		cs.UiPublicationState = &published
		cs.ReconcilerState = btypes.ReconcilerStateCompleted
		cs.FailureMessage = pointers.Ptr("very bad for real this time")
		cs.NumFailures = 100
		cs.OwnedByBatchChangeID = 234
		cs.Closing = true

		// Expect some not changed after update:
		if err := s.UpdateChangesetCodeHostState(ctx, cs); err != nil {
			t.Fatal(err)
		}
		have, err := s.GetChangesetByID(ctx, cs.ID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatalf("invalid changeset state in DB: %s", diff)
		}
	})

	t.Run("GetChangesetsStats", func(t *testing.T) {
		var batchChangeID int64 = 191918
		currentBatchChangeStats, err := s.GetChangesetsStats(ctx, batchChangeID)
		if err != nil {
			t.Fatal(err)
		}

		baseOpts := bt.TestChangesetOpts{Repo: repo.ID}

		// Closed changeset
		opts1 := baseOpts
		opts1.BatchChange = batchChangeID
		opts1.ExternalState = btypes.ChangesetExternalStateClosed
		opts1.ReconcilerState = btypes.ReconcilerStateCompleted
		opts1.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts1)

		// Deleted changeset
		opts2 := baseOpts
		opts2.BatchChange = batchChangeID
		opts2.ExternalState = btypes.ChangesetExternalStateDeleted
		opts2.ReconcilerState = btypes.ReconcilerStateCompleted
		opts2.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts2)

		// Open changeset
		opts3 := baseOpts
		opts3.BatchChange = batchChangeID
		opts3.OwnedByBatchChange = batchChangeID
		opts3.ExternalState = btypes.ChangesetExternalStateOpen
		opts3.ReconcilerState = btypes.ReconcilerStateCompleted
		opts3.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts3)

		// Archived & closed changeset
		opts4 := baseOpts
		opts4.BatchChange = batchChangeID
		opts4.IsArchived = true
		opts4.OwnedByBatchChange = batchChangeID
		opts4.ExternalState = btypes.ChangesetExternalStateClosed
		opts4.ReconcilerState = btypes.ReconcilerStateCompleted
		opts4.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts4)

		// Marked as to-be-archived
		opts5 := baseOpts
		opts5.BatchChange = batchChangeID
		opts5.Archive = true
		opts5.OwnedByBatchChange = batchChangeID
		opts5.ExternalState = btypes.ChangesetExternalStateOpen
		opts5.ReconcilerState = btypes.ReconcilerStateProcessing
		opts5.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts5)

		// Open changeset in a deleted repository
		opts6 := baseOpts
		// In a deleted repository.
		opts6.Repo = deletedRepo.ID
		opts6.BatchChange = batchChangeID
		opts6.ExternalState = btypes.ChangesetExternalStateOpen
		opts6.ReconcilerState = btypes.ReconcilerStateCompleted
		opts6.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts6)

		// Open changeset in a different batch change
		opts7 := baseOpts
		opts7.BatchChange = batchChangeID + 999
		opts7.ExternalState = btypes.ChangesetExternalStateOpen
		opts7.ReconcilerState = btypes.ReconcilerStateCompleted
		opts7.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts7)

		// Processing
		opts8 := baseOpts
		opts8.BatchChange = batchChangeID
		opts8.OwnedByBatchChange = batchChangeID
		opts8.ExternalState = btypes.ChangesetExternalStateOpen
		opts8.ReconcilerState = btypes.ReconcilerStateProcessing
		opts8.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts8)

		haveStats, err := s.GetChangesetsStats(ctx, batchChangeID)
		if err != nil {
			t.Fatal(err)
		}

		wantStats := currentBatchChangeStats
		wantStats.Open += 1
		wantStats.Processing += 1
		wantStats.Closed += 1
		wantStats.Deleted += 1
		wantStats.Archived += 2
		wantStats.Total += 6

		if diff := cmp.Diff(wantStats, haveStats); diff != "" {
			t.Fatalf("wrong stats returned. diff=%s", diff)
		}
	})

	t.Run("GetRepoChangesetsStats", func(t *testing.T) {
		r := bt.TestRepo(t, es, extsvc.KindGitHub)

		if err := rs.Create(ctx, r); err != nil {
			t.Fatal(err)
		}

		baseOpts := bt.TestChangesetOpts{Repo: r.ID, BatchChange: 4747, OwnedByBatchChange: 4747}

		wantStats := btypes.RepoChangesetsStats{}

		// Closed changeset
		opts1 := baseOpts
		opts1.ExternalState = btypes.ChangesetExternalStateClosed
		opts1.ReconcilerState = btypes.ReconcilerStateCompleted
		opts1.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts1)
		wantStats.Closed += 1

		// Open changeset
		opts2 := baseOpts
		opts2.ExternalState = btypes.ChangesetExternalStateOpen
		opts2.ReconcilerState = btypes.ReconcilerStateCompleted
		opts2.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts2)
		wantStats.Open += 1

		// Archived & closed changeset
		opts3 := baseOpts
		opts3.IsArchived = true
		opts3.ExternalState = btypes.ChangesetExternalStateClosed
		opts3.ReconcilerState = btypes.ReconcilerStateCompleted
		opts3.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts3)

		// Marked as to-be-archived
		opts4 := baseOpts
		opts4.Archive = true
		opts4.ExternalState = btypes.ChangesetExternalStateOpen
		opts4.ReconcilerState = btypes.ReconcilerStateProcessing
		opts4.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts4)

		// Open changeset belonging to a different batch change
		opts5 := baseOpts
		opts5.BatchChange = 999
		opts5.ExternalState = btypes.ChangesetExternalStateOpen
		opts5.ReconcilerState = btypes.ReconcilerStateCompleted
		opts5.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts5)
		wantStats.Open += 1

		// Open changeset belonging to multiple batch changes
		opts6 := bt.TestChangesetOpts{Repo: r.ID}
		opts6.BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: 4747}, {BatchChangeID: 4748}, {BatchChangeID: 4749}}
		opts6.ExternalState = btypes.ChangesetExternalStateOpen
		opts6.ReconcilerState = btypes.ReconcilerStateCompleted
		opts6.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts6)
		wantStats.Open += 1

		// Open changeset archived on one batch change but not on another
		opts7 := bt.TestChangesetOpts{Repo: r.ID}
		opts7.BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: 4747, IsArchived: true}, {BatchChangeID: 4748, IsArchived: false}}
		opts7.ExternalState = btypes.ChangesetExternalStateOpen
		opts7.ReconcilerState = btypes.ReconcilerStateCompleted
		opts7.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts7)
		wantStats.Open += 1

		// Open changeset archived on multiple batch changes
		opts8 := bt.TestChangesetOpts{Repo: r.ID}
		opts8.BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: 4747, IsArchived: true}, {BatchChangeID: 4748, IsArchived: true}}
		opts8.ExternalState = btypes.ChangesetExternalStateOpen
		opts8.ReconcilerState = btypes.ReconcilerStateCompleted
		opts8.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts8)

		// Draft changeset
		opts9 := baseOpts
		opts9.ExternalState = btypes.ChangesetExternalStateDraft
		opts9.ReconcilerState = btypes.ReconcilerStateCompleted
		opts9.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts9)
		wantStats.Draft += 1

		haveStats, err := s.GetRepoChangesetsStats(ctx, r.ID)
		if err != nil {
			t.Fatal(err)
		}

		wantStats.Total = wantStats.Open + wantStats.Closed + wantStats.Draft

		if diff := cmp.Diff(wantStats, *haveStats); diff != "" {
			t.Fatalf("wrong stats returned. diff=%s", diff)
		}
	})

	t.Run("GetGlobalChangesetsStats", func(t *testing.T) {
		var batchChangeID int64 = 191918
		currentBatchChangeStats, err := s.GetGlobalChangesetsStats(ctx)
		if err != nil {
			t.Fatal(err)
		}
		baseOpts := bt.TestChangesetOpts{Repo: repo.ID}

		// Closed changeset
		opts1 := baseOpts
		opts1.BatchChange = batchChangeID
		opts1.ExternalState = btypes.ChangesetExternalStateClosed
		opts1.ReconcilerState = btypes.ReconcilerStateCompleted
		opts1.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts1)

		// Open changeset
		opts2 := baseOpts
		opts2.BatchChange = batchChangeID
		opts2.ExternalState = btypes.ChangesetExternalStateOpen
		opts2.ReconcilerState = btypes.ReconcilerStateCompleted
		opts2.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts2)

		// Draft changeset
		opts3 := baseOpts
		opts3.BatchChange = batchChangeID
		opts3.ExternalState = btypes.ChangesetExternalStateDraft
		opts3.ReconcilerState = btypes.ReconcilerStateCompleted
		opts3.PublicationState = btypes.ChangesetPublicationStatePublished
		bt.CreateChangeset(t, ctx, s, opts3)

		haveStats, err := s.GetGlobalChangesetsStats(ctx)
		if err != nil {
			t.Fatal(err)
		}

		wantStats := currentBatchChangeStats
		wantStats.Open += 1
		wantStats.Closed += 1
		wantStats.Draft += 1
		wantStats.Total += 3

		if diff := cmp.Diff(wantStats, haveStats); diff != "" {
			t.Fatalf("wrong stats returned. diff=%s", diff)
		}
	})

	t.Run("EnqueueChangeset", func(t *testing.T) {
		c1 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
			ReconcilerState:  btypes.ReconcilerStateCompleted,
			PublicationState: btypes.ChangesetPublicationStatePublished,
			ExternalState:    btypes.ChangesetExternalStateOpen,
			Repo:             repo.ID,
			NumResets:        1234,
			NumFailures:      4567,
			FailureMessage:   "horse was here",
			SyncErrorMessage: "horse was here",
		})

		// Try with wrong `currentState` and expect error
		err := s.EnqueueChangeset(ctx, c1, btypes.ReconcilerStateQueued, btypes.ReconcilerStateFailed)
		if err == nil {
			t.Fatalf("expected error, received none")
		}

		// Try with correct `currentState` and expected updated changeset
		err = s.EnqueueChangeset(ctx, c1, btypes.ReconcilerStateQueued, c1.ReconcilerState)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		bt.ReloadAndAssertChangeset(t, ctx, s, c1, bt.ChangesetAssertions{
			ReconcilerState:        btypes.ReconcilerStateQueued,
			PublicationState:       btypes.ChangesetPublicationStatePublished,
			ExternalState:          btypes.ChangesetExternalStateOpen,
			Repo:                   repo.ID,
			FailureMessage:         nil,
			NumResets:              0,
			NumFailures:            0,
			SyncErrorMessage:       nil,
			PreviousFailureMessage: pointers.Ptr("horse was here"),
		})
	})

	t.Run("UpdateChangesetBatchChanges", func(t *testing.T) {
		c1 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
			ReconcilerState:  btypes.ReconcilerStateCompleted,
			PublicationState: btypes.ChangesetPublicationStatePublished,
			ExternalState:    btypes.ChangesetExternalStateOpen,
			Repo:             repo.ID,
		})

		// Add 3 batch changes
		c1.Attach(123)
		c1.Attach(456)
		c1.Attach(789)

		// This is what we expect after the update
		want := c1.Clone()

		// These two and other columsn should not be updated in the DB
		c1.ReconcilerState = btypes.ReconcilerStateErrored
		c1.ExternalServiceType = "external-service-type"

		err := s.UpdateChangesetBatchChanges(ctx, c1)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		have := c1
		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatalf("invalid changeset: %s", diff)
		}
	})

	t.Run("UpdateChangesetUiPublicationState", func(t *testing.T) {
		c1 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
			ReconcilerState:  btypes.ReconcilerStateCompleted,
			PublicationState: btypes.ChangesetPublicationStateUnpublished,
			Repo:             repo.ID,
		})

		// Update the UiPublicationState
		c1.UiPublicationState = &btypes.ChangesetUiPublicationStateDraft

		// This is what we expect after the update
		want := c1.Clone()

		// These two and other columsn should not be updated in the DB
		c1.ReconcilerState = btypes.ReconcilerStateErrored
		c1.ExternalServiceType = "external-service-type"

		err := s.UpdateChangesetUiPublicationState(ctx, c1)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		have := c1
		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatalf("invalid changeset: %s", diff)
		}
	})

	t.Run("UpdateChangesetCommitVerification", func(t *testing.T) {
		c1 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{Repo: repo.ID})

		// Once with a verified commit
		commitVerification := github.Verification{
			Verified:  true,
			Reason:    "valid",
			Signature: "*********",
			Payload:   "*********",
		}
		commit := github.RestCommit{
			URL:          "https://api.github.com/repos/Birth-control-tech/birth-control-tech-BE/git/commits/dabd9bb07fdb5b580f168e942f2160b1719fc98f",
			SHA:          "dabd9bb07fdb5b580f168e942f2160b1719fc98f",
			NodeID:       "C_kwDOEW0OxtoAKGRhYmQ5YmIwN2ZkYjViNTgwZjE2OGU5NDJmMjE2MGIxNzE5ZmM5OGY",
			Message:      "Append Hello World to all README.md files",
			Verification: commitVerification,
		}

		c1.CommitVerification = &commitVerification
		want := c1.Clone()

		if err := s.UpdateChangesetCommitVerification(ctx, c1, &commit); err != nil {
			t.Fatal(err)
		}
		have, err := s.GetChangesetByID(ctx, c1.ID)
		if err != nil {
			t.Fatal(err)
		}

		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatalf("found diff with signed commit: %s", diff)
		}

		// Once with a commit that's not verified
		commitVerification = github.Verification{
			Verified: false,
			Reason:   "unsigned",
		}
		commit.Verification = commitVerification
		// A changeset spec with an unsigned commit should not have a commit
		// verification set.
		c1.CommitVerification = nil
		want = c1.Clone()

		if err := s.UpdateChangesetCommitVerification(ctx, c1, &commit); err != nil {
			t.Fatal(err)
		}
		have, err = s.GetChangesetByID(ctx, c1.ID)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(have, want); diff != "" {
			t.Fatalf("found diff with unsigned commit: %s", diff)
		}
	})
}

func testStoreListChangesetSyncData(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
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

	rs := database.ReposWith(logger, s)
	es := database.ExternalServicesWith(logger, s)

	githubRepo := bt.TestRepo(t, es, extsvc.KindGitHub)
	gitlabRepo := bt.TestRepo(t, es, extsvc.KindGitLab)

	if err := rs.Create(ctx, githubRepo, gitlabRepo); err != nil {
		t.Fatal(err)
	}

	changesets := make(btypes.Changesets, 0, 3)
	events := make([]*btypes.ChangesetEvent, 0)

	for i := range cap(changesets) {
		ch := &btypes.Changeset{
			RepoID:              githubRepo.ID,
			CreatedAt:           clock.Now(),
			UpdatedAt:           clock.Now(),
			Metadata:            githubPR,
			BatchChanges:        []btypes.BatchChangeAssoc{{BatchChangeID: int64(i) + 1}},
			ExternalID:          fmt.Sprintf("foobar-%d", i),
			ExternalServiceType: extsvc.TypeGitHub,
			ExternalBranch:      "refs/heads/batch-changes/test",
			ExternalUpdatedAt:   clock.Now(),
			ExternalState:       btypes.ChangesetExternalStateOpen,
			ExternalReviewState: btypes.ChangesetReviewStateApproved,
			ExternalCheckState:  btypes.ChangesetCheckStatePassed,
			PublicationState:    btypes.ChangesetPublicationStatePublished,
			ReconcilerState:     btypes.ReconcilerStateCompleted,
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
	for i, cs := range changesets {
		c := &btypes.BatchChange{
			Name:           fmt.Sprintf("ListChangesetSyncData-test-%d", i),
			NamespaceOrgID: 23,
			LastApplierID:  1,
			LastAppliedAt:  time.Now(),
			BatchSpecID:    42,
		}
		err := s.CreateBatchChange(ctx, c)
		if err != nil {
			t.Fatal(err)
		}

		cs.BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: c.ID}}

		if err := s.UpdateChangeset(ctx, cs); err != nil {
			t.Fatal(err)
		}
	}

	// The changesets, except one, get changeset events
	for _, cs := range changesets[:len(changesets)-1] {
		e := &btypes.ChangesetEvent{
			ChangesetID: cs.ID,
			Kind:        btypes.ChangesetEventKindGitHubCommented,
			Key:         issueComment.Key(),
			CreatedAt:   clock.Now(),
			Metadata:    issueComment,
		}

		events = append(events, e)
	}
	if err := s.UpsertChangesetEvents(ctx, events...); err != nil {
		t.Fatal(err)
	}

	checkChangesetIDs := func(t *testing.T, hs []*btypes.ChangesetSyncData, want []int64) {
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
		want := []*btypes.ChangesetSyncData{
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
		want := []*btypes.ChangesetSyncData{
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

	t.Run("only for subset of changesets", func(t *testing.T) {
		hs, err := s.ListChangesetSyncData(ctx, ListChangesetSyncDataOpts{ChangesetIDs: []int64{changesets[0].ID}})
		if err != nil {
			t.Fatal(err)
		}
		want := []*btypes.ChangesetSyncData{
			{
				ChangesetID:           changesets[0].ID,
				UpdatedAt:             clock.Now(),
				LatestEvent:           clock.Now(),
				ExternalUpdatedAt:     clock.Now(),
				RepoExternalServiceID: "https://github.com/",
			},
		}
		if diff := cmp.Diff(want, hs); diff != "" {
			t.Fatal(diff)
		}
	})

	t.Run("ignore closed batch change", func(t *testing.T) {
		closedBatchChangeID := changesets[0].BatchChanges[0].BatchChangeID
		c, err := s.GetBatchChange(ctx, GetBatchChangeOpts{ID: closedBatchChangeID})
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
		changesets[0].BatchChanges = []btypes.BatchChangeAssoc{{BatchChangeID: closedBatchChangeID}, {BatchChangeID: openBatchChangeID}}
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
		ch.PublicationState = btypes.ChangesetPublicationStatePublished
		ch.ReconcilerState = btypes.ReconcilerStateProcessing
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
		ch.PublicationState = btypes.ChangesetPublicationStateUnpublished
		ch.ReconcilerState = btypes.ReconcilerStateCompleted
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

func testStoreListChangesetsTextSearch(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	// This is similar to the setup in testStoreChangesets(), but we need a more
	// fine grained set of changesets to handle the different scenarios. Namely,
	// we need to cover:
	//
	// 1. Metadata from each code host type to test title search.
	// 2. Unpublished changesets that don't have metadata to test the title
	//    search fallback to the spec title.
	// 3. Repo name search.
	// 4. Negation of all of the above.

	logger := logtest.Scoped(t)

	// Let's define some helpers.
	createChangesetSpec := func(title string) *btypes.ChangesetSpec {
		spec := &btypes.ChangesetSpec{
			Title:      title,
			ExternalID: "123",
			Type:       btypes.ChangesetSpecTypeExisting,
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
		metadata any,
		spec *btypes.ChangesetSpec,
	) *btypes.Changeset {
		var specID int64
		if spec != nil {
			specID = spec.ID
		}

		cs := &btypes.Changeset{
			RepoID:              repo.ID,
			CreatedAt:           clock.Now(),
			UpdatedAt:           clock.Now(),
			Metadata:            metadata,
			ExternalID:          externalID,
			ExternalServiceType: esType,
			ExternalBranch:      "refs/heads/batch-changes/test",
			ExternalUpdatedAt:   clock.Now(),
			ExternalState:       btypes.ChangesetExternalStateOpen,
			ExternalReviewState: btypes.ChangesetReviewStateApproved,
			ExternalCheckState:  btypes.ChangesetCheckStatePassed,

			CurrentSpecID:    specID,
			PublicationState: btypes.ChangesetPublicationStatePublished,
		}

		if err := s.CreateChangeset(ctx, cs); err != nil {
			t.Fatalf("creating changeset:\nerr: %+v\nchangeset: %+v", err, cs)
		}
		return cs
	}

	rs := database.ReposWith(logger, s)
	es := database.ExternalServicesWith(logger, s)

	// Set up repositories for each code host type we want to test.
	var (
		githubRepo = bt.TestRepo(t, es, extsvc.KindGitHub)
		bbsRepo    = bt.TestRepo(t, es, extsvc.KindBitbucketServer)
		gitlabRepo = bt.TestRepo(t, es, extsvc.KindGitLab)
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
		map[string]any{},
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
		want       btypes.Changesets
	}{
		"single changeset based on GitHub metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on GitHub"},
			},
			want: btypes.Changesets{githubChangeset},
		},
		"single changeset based on GitLab metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on GitLab"},
			},
			want: btypes.Changesets{gitlabChangeset},
		},
		"single changeset based on Bitbucket Server metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "on Bitbucket Server"},
			},
			want: btypes.Changesets{bbsChangeset},
		},
		"all published changesets based on metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Fix a bunch of bugs"},
			},
			want: btypes.Changesets{
				githubChangeset,
				gitlabChangeset,
				bbsChangeset,
			},
		},
		"imported changeset based on metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Do some stuff"},
			},
			want: btypes.Changesets{importedChangeset},
		},
		"unpublished changeset based on spec title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually"},
			},
			want: btypes.Changesets{unpublishedChangeset},
		},
		"negated metadata title": {
			textSearch: []search.TextSearchTerm{
				{Term: "bunch of bugs", Not: true},
			},
			want: btypes.Changesets{
				unpublishedChangeset,
				importedChangeset,
			},
		},
		"negated spec title": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually", Not: true},
			},
			want: btypes.Changesets{
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
			want: btypes.Changesets{
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
			want: btypes.Changesets{
				unpublishedChangeset,
			},
		},
		"multiple title matches together": {
			textSearch: []search.TextSearchTerm{
				{Term: "Eventually"},
				{Term: "fix"},
			},
			want: btypes.Changesets{
				unpublishedChangeset,
			},
		},
		"negated repo name": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name), Not: true},
			},
			want: btypes.Changesets{
				gitlabChangeset,
				bbsChangeset,
			},
		},
		"combined negated repo names": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name), Not: true},
				{Term: string(gitlabRepo.Name), Not: true},
			},
			want: btypes.Changesets{bbsChangeset},
		},
		"no results due to conflicting requirements": {
			textSearch: []search.TextSearchTerm{
				{Term: string(githubRepo.Name)},
				{Term: string(gitlabRepo.Name)},
			},
			want: btypes.Changesets{},
		},
		"no results due to a subset of a word": {
			textSearch: []search.TextSearchTerm{
				{Term: "unch"},
			},
			want: btypes.Changesets{},
		},
		"no results due to text that doesn't exist in the search scope": {
			textSearch: []search.TextSearchTerm{
				{Term: "she dreamt she was a bulldozer, she dreamt she was in an empty field"},
			},
			want: btypes.Changesets{},
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

// testStoreChangesetScheduling provides tests for schedule-related methods on
// the Store.
func testStoreChangesetScheduling(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	// Like testStoreListChangesetsTextSearch(), this is similar to the setup
	// in testStoreChangesets(), but we need a more fine grained set of
	// changesets to handle the different scenarios.

	logger := logtest.Scoped(t)
	rs := database.ReposWith(logger, s)
	es := database.ExternalServicesWith(logger, s)

	// We can just pre-can a repo. The kind doesn't matter here.
	repo := bt.TestRepo(t, es, extsvc.KindGitHub)
	if err := rs.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// Let's define a quick and dirty helper to create changesets with a
	// specific state and update time, since those are the key fields.
	createChangeset := func(title string, lastUpdated time.Time, state btypes.ReconcilerState) *btypes.Changeset {
		// First, we need to create a changeset spec.
		spec := &btypes.ChangesetSpec{
			Title:      "fake spec",
			ExternalID: "123",
			Type:       btypes.ChangesetSpecTypeExisting,
		}
		if err := s.CreateChangesetSpec(ctx, spec); err != nil {
			t.Fatalf("creating changeset spec: %v", err)
		}

		// Now we can use that to create a changeset.
		cs := &btypes.Changeset{
			RepoID:              repo.ID,
			CreatedAt:           clock.Now(),
			UpdatedAt:           lastUpdated,
			Metadata:            &github.PullRequest{Title: title},
			ExternalServiceType: extsvc.TypeGitHub,
			CurrentSpecID:       spec.ID,
			PublicationState:    btypes.ChangesetPublicationStateUnpublished,
			ReconcilerState:     state,
		}

		if err := s.CreateChangeset(ctx, cs); err != nil {
			t.Fatalf("creating changeset:\nerr: %+v\nchangeset: %+v", err, cs)
		}
		return cs
	}

	// Let's define two changesets that are scheduled out of their "natural"
	// order, and one changeset that is already queued.
	var (
		second = createChangeset("after", time.Now().Add(1*time.Minute), btypes.ReconcilerStateScheduled)
		first  = createChangeset("next", time.Now(), btypes.ReconcilerStateScheduled)
		queued = createChangeset("queued", time.Now().Add(1*time.Minute), btypes.ReconcilerStateQueued)
	)

	// first should be the first in line, and second the second in line.
	if have, err := s.GetChangesetPlaceInSchedulerQueue(ctx, first.ID); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if want := 0; have != want {
		t.Errorf("unexpected place: have=%d want=%d", have, want)
	}

	if have, err := s.GetChangesetPlaceInSchedulerQueue(ctx, second.ID); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if want := 1; have != want {
		t.Errorf("unexpected place: have=%d want=%d", have, want)
	}

	// queued should return an error.
	if _, err := s.GetChangesetPlaceInSchedulerQueue(ctx, queued.ID); err != ErrNoResults {
		t.Errorf("unexpected error: %v", err)
	}

	// By definition, the first changeset should be next, since it has the
	// earliest update time and is in the right state.
	have, err := s.EnqueueNextScheduledChangeset(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if have == nil {
		t.Errorf("unexpected nil changeset")
	} else if have.ID != first.ID {
		t.Errorf("unexpected changeset: have=%v want=%v", have, first)
	}

	// Let's check that first's state was updated.
	if want := btypes.ReconcilerStateQueued; have.ReconcilerState != want {
		t.Errorf("unexpected reconciler state: have=%v want=%v", have.ReconcilerState, want)
	}

	// Now second should be the first in line. (Confused yet?)
	if have, err := s.GetChangesetPlaceInSchedulerQueue(ctx, second.ID); err != nil {
		t.Errorf("unexpected error: %v", err)
	} else if want := 0; have != want {
		t.Errorf("unexpected place: have=%d want=%d", have, want)
	}

	// Both queued and first should return errors, since they are not scheduled.
	if _, err := s.GetChangesetPlaceInSchedulerQueue(ctx, first.ID); err != ErrNoResults {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := s.GetChangesetPlaceInSchedulerQueue(ctx, queued.ID); err != ErrNoResults {
		t.Errorf("unexpected error: %v", err)
	}

	// Given the updated state, second should be the next scheduled changeset.
	have, err = s.EnqueueNextScheduledChangeset(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if have == nil {
		t.Errorf("unexpected nil changeset")
	} else if have.ID != second.ID {
		t.Errorf("unexpected changeset: have=%v want=%v", have, second)
	}

	// Let's check that second's state was updated.
	if want := btypes.ReconcilerStateQueued; have.ReconcilerState != want {
		t.Errorf("unexpected reconciler state: have=%v want=%v", have.ReconcilerState, want)
	}

	// Now we've enqueued the two scheduled changesets, we shouldn't be able to
	// enqueue another.
	if _, err = s.EnqueueNextScheduledChangeset(ctx); err != ErrNoResults {
		t.Errorf("unexpected error: have=%v want=%v", err, ErrNoResults)
	}

	// None of our changesets should have a place in the scheduler queue at this
	// point.
	for _, cs := range []*btypes.Changeset{first, second, queued} {
		if _, err := s.GetChangesetPlaceInSchedulerQueue(ctx, cs.ID); err != ErrNoResults {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestCancelQueuedBatchChangeChangesets(t *testing.T) {
	// We use a separate test for CancelQueuedBatchChangeChangesets because we
	// want to access the database from different connections and the other
	// integration/store tests all execute in a single transaction.

	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	s := New(db, &observation.TestContext, nil)

	user := bt.CreateTestUser(t, db, true)
	spec := bt.CreateBatchSpec(t, ctx, s, "test-batch-change", user.ID, 0)
	batchChange := bt.CreateBatchChange(t, ctx, s, "test-batch-change", user.ID, spec.ID)
	repo, _ := bt.CreateTestRepo(t, ctx, db)

	c1 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateQueued,
	})

	c2 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateErrored,
		NumFailures:        1,
	})

	c3 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateCompleted,
		PublicationState:   btypes.ChangesetPublicationStatePublished,
		ExternalState:      btypes.ChangesetExternalStateOpen,
	})

	c4 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: 0,
		PublicationState:   btypes.ChangesetPublicationStateUnpublished,
		ReconcilerState:    btypes.ReconcilerStateQueued,
	})

	// These two changesets will not be canceled in the first iteration of
	// the loop in CancelQueuedBatchChangeChangesets, because they're both
	// processing.
	c5 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateProcessing,
		PublicationState:   btypes.ChangesetPublicationStateUnpublished,
	})

	c6 := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:               repo.ID,
		BatchChange:        batchChange.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateProcessing,
		PublicationState:   btypes.ChangesetPublicationStateUnpublished,
	})

	// We start this goroutine to simulate the processing of these
	// changesets to stop after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)

		// c5 ends up errored, which would be retried, so it needs to be
		// canceled
		c5.ReconcilerState = btypes.ReconcilerStateErrored
		if err := s.UpdateChangeset(ctx, c5); err != nil {
			t.Errorf("update changeset failed: %s", err)
		}

		time.Sleep(50 * time.Millisecond)

		// c6 ends up completed, so it does not need to be canceled
		c6.ReconcilerState = btypes.ReconcilerStateCompleted
		if err := s.UpdateChangeset(ctx, c6); err != nil {
			t.Errorf("update changeset failed: %s", err)
		}
	}()

	if err := s.CancelQueuedBatchChangeChangesets(ctx, batchChange.ID); err != nil {
		t.Fatal(err)
	}

	bt.ReloadAndAssertChangeset(t, ctx, s, c1, bt.ChangesetAssertions{
		Repo:               repo.ID,
		ReconcilerState:    btypes.ReconcilerStateFailed,
		OwnedByBatchChange: batchChange.ID,
		FailureMessage:     &CanceledChangesetFailureMessage,
		AttachedTo:         []int64{batchChange.ID},
	})

	bt.ReloadAndAssertChangeset(t, ctx, s, c2, bt.ChangesetAssertions{
		Repo:               repo.ID,
		ReconcilerState:    btypes.ReconcilerStateFailed,
		OwnedByBatchChange: batchChange.ID,
		FailureMessage:     &CanceledChangesetFailureMessage,
		NumFailures:        1,
		AttachedTo:         []int64{batchChange.ID},
	})

	bt.ReloadAndAssertChangeset(t, ctx, s, c3, bt.ChangesetAssertions{
		Repo:               repo.ID,
		ReconcilerState:    btypes.ReconcilerStateCompleted,
		PublicationState:   btypes.ChangesetPublicationStatePublished,
		ExternalState:      btypes.ChangesetExternalStateOpen,
		OwnedByBatchChange: batchChange.ID,
		AttachedTo:         []int64{batchChange.ID},
	})

	bt.ReloadAndAssertChangeset(t, ctx, s, c4, bt.ChangesetAssertions{
		Repo:             repo.ID,
		ReconcilerState:  btypes.ReconcilerStateQueued,
		PublicationState: btypes.ChangesetPublicationStateUnpublished,
		AttachedTo:       []int64{batchChange.ID},
	})

	bt.ReloadAndAssertChangeset(t, ctx, s, c5, bt.ChangesetAssertions{
		Repo:               repo.ID,
		ReconcilerState:    btypes.ReconcilerStateFailed,
		PublicationState:   btypes.ChangesetPublicationStateUnpublished,
		FailureMessage:     &CanceledChangesetFailureMessage,
		OwnedByBatchChange: batchChange.ID,
		AttachedTo:         []int64{batchChange.ID},
	})

	bt.ReloadAndAssertChangeset(t, ctx, s, c6, bt.ChangesetAssertions{
		Repo:               repo.ID,
		ReconcilerState:    btypes.ReconcilerStateCompleted,
		PublicationState:   btypes.ChangesetPublicationStateUnpublished,
		OwnedByBatchChange: batchChange.ID,
		AttachedTo:         []int64{batchChange.ID},
	})
}

func TestEnqueueChangesetsToClose(t *testing.T) {
	// We use a separate test for CancelQueuedBatchChangeChangesets because we
	// want to access the database from different connections and the other
	// integration/store tests all execute in a single transaction.

	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	s := New(db, &observation.TestContext, nil)

	user := bt.CreateTestUser(t, db, true)
	spec := bt.CreateBatchSpec(t, ctx, s, "test-batch-change", user.ID, 0)
	batchChange := bt.CreateBatchChange(t, ctx, s, "test-batch-change", user.ID, spec.ID)
	repo, _ := bt.CreateTestRepo(t, ctx, db)

	wantEnqueued := bt.ChangesetAssertions{
		Repo:               repo.ID,
		OwnedByBatchChange: batchChange.ID,
		ReconcilerState:    btypes.ReconcilerStateQueued,
		PublicationState:   btypes.ChangesetPublicationStatePublished,
		NumFailures:        0,
		FailureMessage:     nil,
		Closing:            true,
	}

	tests := []struct {
		have bt.TestChangesetOpts
		want bt.ChangesetAssertions
	}{
		{
			have: bt.TestChangesetOpts{
				ReconcilerState:  btypes.ReconcilerStateQueued,
				PublicationState: btypes.ChangesetPublicationStatePublished,
			},
			want: wantEnqueued,
		},
		{
			have: bt.TestChangesetOpts{
				ReconcilerState:  btypes.ReconcilerStateProcessing,
				PublicationState: btypes.ChangesetPublicationStatePublished,
			},
			want: bt.ChangesetAssertions{
				Repo:               repo.ID,
				OwnedByBatchChange: batchChange.ID,
				ReconcilerState:    btypes.ReconcilerStateQueued,
				PublicationState:   btypes.ChangesetPublicationStatePublished,
				ExternalState:      btypes.ChangesetExternalStateOpen,
				Closing:            true,
			},
		},
		{
			have: bt.TestChangesetOpts{
				ReconcilerState:  btypes.ReconcilerStateErrored,
				PublicationState: btypes.ChangesetPublicationStatePublished,
				FailureMessage:   "failed",
				NumFailures:      1,
			},
			want: wantEnqueued,
		},
		{
			have: bt.TestChangesetOpts{
				ExternalState:    btypes.ChangesetExternalStateOpen,
				ReconcilerState:  btypes.ReconcilerStateCompleted,
				PublicationState: btypes.ChangesetPublicationStatePublished,
			},
			want: bt.ChangesetAssertions{
				ReconcilerState:  btypes.ReconcilerStateQueued,
				PublicationState: btypes.ChangesetPublicationStatePublished,
				Closing:          true,
				ExternalState:    btypes.ChangesetExternalStateOpen,
			},
		},
		{
			have: bt.TestChangesetOpts{
				ExternalState:    btypes.ChangesetExternalStateClosed,
				ReconcilerState:  btypes.ReconcilerStateCompleted,
				PublicationState: btypes.ChangesetPublicationStatePublished,
			},
			want: bt.ChangesetAssertions{
				ReconcilerState:  btypes.ReconcilerStateCompleted,
				ExternalState:    btypes.ChangesetExternalStateClosed,
				PublicationState: btypes.ChangesetPublicationStatePublished,
			},
		},
		{
			have: bt.TestChangesetOpts{
				ReconcilerState:  btypes.ReconcilerStateCompleted,
				PublicationState: btypes.ChangesetPublicationStateUnpublished,
			},
			want: bt.ChangesetAssertions{
				ReconcilerState:  btypes.ReconcilerStateCompleted,
				PublicationState: btypes.ChangesetPublicationStateUnpublished,
			},
		},
	}

	changesets := make(map[*btypes.Changeset]bt.ChangesetAssertions)
	for _, tc := range tests {
		opts := tc.have
		opts.Repo = repo.ID
		opts.BatchChange = batchChange.ID
		opts.OwnedByBatchChange = batchChange.ID

		c := bt.CreateChangeset(t, ctx, s, opts)
		changesets[c] = tc.want

		// If we have a changeset that's still processing we need to make
		// sure that we finish it, otherwise the loop in
		// EnqueueChangesetsToClose will take 2min and then fail.
		if c.ReconcilerState == btypes.ReconcilerStateProcessing {
			go func() {
				time.Sleep(50 * time.Millisecond)

				c.ReconcilerState = btypes.ReconcilerStateCompleted
				c.ExternalState = btypes.ChangesetExternalStateOpen
				if err := s.UpdateChangeset(ctx, c); err != nil {
					t.Errorf("update changeset failed: %s", err)
				}
			}()
		}
	}

	if err := s.EnqueueChangesetsToClose(ctx, batchChange.ID); err != nil {
		t.Fatal(err)
	}

	for changeset, want := range changesets {
		want.Repo = repo.ID
		want.OwnedByBatchChange = batchChange.ID
		want.AttachedTo = []int64{batchChange.ID}
		bt.ReloadAndAssertChangeset(t, ctx, s, changeset, want)
	}
}

func TestCleanDetachedChangesets(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	s := New(db, &observation.TestContext, nil)
	rs := database.ReposWith(logger, s)
	es := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, es, extsvc.KindGitHub)
	err := rs.Create(ctx, repo)
	require.NoError(t, err)

	tests := []struct {
		name        string
		cs          *btypes.Changeset
		wantDeleted bool
	}{
		{
			name: "old detached changeset deleted",
			cs: &btypes.Changeset{
				RepoID:              repo.ID,
				ExternalID:          fmt.Sprintf("foobar-%d", 42),
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      "refs/heads/batch-changes/test",
				// Set beyond the retention period
				DetachedAt: time.Now().Add(-48 * time.Hour),
			},
			wantDeleted: true,
		},
		{
			name: "new detached changeset not deleted",
			cs: &btypes.Changeset{
				RepoID:              repo.ID,
				ExternalID:          fmt.Sprintf("foobar-%d", 42),
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      "refs/heads/batch-changes/test",
				// Set to now, within the retention period
				DetachedAt: time.Now(),
			},
			wantDeleted: false,
		},
		{
			name: "regular changeset not deleted",
			cs: &btypes.Changeset{
				RepoID:              repo.ID,
				ExternalID:          fmt.Sprintf("foobar-%d", 42),
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalBranch:      "refs/heads/batch-changes/test",
			},
			wantDeleted: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create the changeset
			err = s.CreateChangeset(ctx, test.cs)
			require.NoError(t, err)

			// Attempt to delete old changesets
			err = s.CleanDetachedChangesets(ctx, 24*time.Hour)
			assert.NoError(t, err)

			// check if deleted
			actual, err := s.GetChangesetByID(ctx, test.cs.ID)

			if test.wantDeleted {
				assert.Error(t, err)
				assert.Nil(t, actual)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, actual)
			}

			// cleanup for next test
			err = s.DeleteChangeset(ctx, test.cs.ID)
			require.NoError(t, err)
		})
	}
}
