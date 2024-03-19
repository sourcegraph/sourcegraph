package store

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches/search"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Comparing the IDs is good enough, no need to bloat the tests here.
var cmtRewirerMappingsOpts = cmp.FilterPath(func(p cmp.Path) bool {
	switch p.String() {
	case "Changeset", "ChangesetSpec", "Repo":
		return true
	default:
		return false
	}
}, cmp.Ignore())

func testStoreChangesetSpecs(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)
	deletedRepo := bt.TestRepo(t, esStore, extsvc.KindGitHub).With(typestest.Opt.RepoDeletedAt(clock.Now()))

	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := repoStore.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}

	// The diff may contain non ascii, cover for this.
	testDiff := []byte("git diff here\\x20")

	changesetSpecs := make(btypes.ChangesetSpecs, 0, 3)
	for i := range cap(changesetSpecs) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(i + 1234),
			BatchSpecID: int64(i + 910),
			BaseRepoID:  repo.ID,

			DiffStatAdded:   579,
			DiffStatDeleted: 1245,
		}
		if i == 0 {
			c.BaseRef = "refs/heads/main"
			c.BaseRev = "deadbeef"
			c.HeadRef = "refs/heads/branch"
			c.Title = "The title"
			c.Body = "The body"
			c.Published = batcheslib.PublishedValue{Val: false}
			c.CommitMessage = "Test message"
			c.Diff = testDiff
			c.CommitAuthorName = "name"
			c.CommitAuthorEmail = "email"
			c.Type = btypes.ChangesetSpecTypeBranch
		} else {
			c.ExternalID = "123456"
			c.Type = btypes.ChangesetSpecTypeExisting
		}

		if i == cap(changesetSpecs)-1 {
			c.BatchSpecID = 0
			forkNamespace := "fork"
			c.ForkNamespace = &forkNamespace
		}
		changesetSpecs = append(changesetSpecs, c)
	}

	// We create this ChangesetSpec to make sure that it's not returned when
	// listing or getting ChangesetSpecs, since we don't want to load
	// ChangesetSpecs whose repository has been (soft-)deleted.
	changesetSpecDeletedRepo := &btypes.ChangesetSpec{
		UserID:      int32(424242),
		BatchSpecID: int64(424242),
		BaseRepoID:  deletedRepo.ID,

		ExternalID: "123",
		Type:       btypes.ChangesetSpecTypeExisting,
	}

	t.Run("Create", func(t *testing.T) {
		toCreate := make(btypes.ChangesetSpecs, 0, len(changesetSpecs)+1)
		toCreate = append(toCreate, changesetSpecs...)
		toCreate = append(toCreate, changesetSpecDeletedRepo)

		for i, c := range toCreate {
			want := c.Clone()
			have := c

			err := s.CreateChangesetSpec(ctx, have)
			if err != nil {
				t.Fatal(err)
			}

			if have.ID == 0 {
				t.Fatal("ID should not be zero")
			}

			if have.RandID == "" {
				t.Fatal("RandID should not be empty")
			}

			want.ID = have.ID
			want.RandID = have.RandID
			want.CreatedAt = clock.Now()
			want.UpdatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}

			if typ, _, err := basestore.ScanFirstString(s.Query(ctx, sqlf.Sprintf("SELECT type FROM changeset_specs WHERE id = %d", have.ID))); err != nil {
				t.Fatal(err)
			} else if i == 0 && typ != string(btypes.ChangesetSpecTypeBranch) {
				t.Fatalf("got incorrect changeset spec type %s", typ)
			} else if i != 0 && typ != string(btypes.ChangesetSpecTypeExisting) {
				t.Fatalf("got incorrect changeset spec type %s", typ)
			}
		}
	})

	t.Run("Count", func(t *testing.T) {
		count, err := s.CountChangesetSpecs(ctx, CountChangesetSpecsOpts{})
		if err != nil {
			t.Fatal(err)
		}

		if have, want := count, len(changesetSpecs); have != want {
			t.Fatalf("have count: %d, want: %d", have, want)
		}

		t.Run("WithBatchSpecID", func(t *testing.T) {
			testsRan := false
			for _, c := range changesetSpecs {
				if c.BatchSpecID == 0 {
					continue
				}

				opts := CountChangesetSpecsOpts{BatchSpecID: c.BatchSpecID}
				subCount, err := s.CountChangesetSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if have, want := subCount, 1; have != want {
					t.Fatalf("have count: %d, want: %d", have, want)
				}
				testsRan = true
			}

			if !testsRan {
				t.Fatal("no changesetSpec has a non-zero BatchSpecID")
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		t.Run("NoLimit", func(t *testing.T) {
			// Empty limit should return all entries.
			opts := ListChangesetSpecsOpts{}
			ts, next, err := s.ListChangesetSpecs(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := next, int64(0); have != want {
				t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
			}

			have, want := ts, changesetSpecs
			if len(have) != len(want) {
				t.Fatalf("listed %d changesetSpecs, want: %d", len(have), len(want))
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		})

		t.Run("WithLimit", func(t *testing.T) {
			for i := 1; i <= len(changesetSpecs); i++ {
				cs, next, err := s.ListChangesetSpecs(ctx, ListChangesetSpecsOpts{LimitOpts: LimitOpts{Limit: i}})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(changesetSpecs) {
						want = changesetSpecs[i].ID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := cs, changesetSpecs[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d changesetSpecs, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}
		})

		t.Run("WithLimitAndCursor", func(t *testing.T) {
			var cursor int64
			for i := 1; i <= len(changesetSpecs); i++ {
				opts := ListChangesetSpecsOpts{Cursor: cursor, LimitOpts: LimitOpts{Limit: 1}}
				have, next, err := s.ListChangesetSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := changesetSpecs[i-1 : i]
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}

				cursor = next
			}
		})

		t.Run("WithBatchSpecID", func(t *testing.T) {
			for _, c := range changesetSpecs {
				if c.BatchSpecID == 0 {
					continue
				}
				opts := ListChangesetSpecsOpts{BatchSpecID: c.BatchSpecID}
				have, _, err := s.ListChangesetSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := btypes.ChangesetSpecs{c}
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}
		})

		t.Run("WithRandIDs", func(t *testing.T) {
			for _, c := range changesetSpecs {
				opts := ListChangesetSpecsOpts{RandIDs: []string{c.RandID}}
				have, _, err := s.ListChangesetSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := btypes.ChangesetSpecs{c}
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			opts := ListChangesetSpecsOpts{}
			for _, c := range changesetSpecs {
				opts.RandIDs = append(opts.RandIDs, c.RandID)
			}

			have, _, err := s.ListChangesetSpecs(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			// ListChangesetSpecs should not return ChangesetSpecs whose
			// repository was (soft-)deleted.
			if diff := cmp.Diff(have, changesetSpecs); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		})

		t.Run("WithIDs", func(t *testing.T) {
			for _, c := range changesetSpecs {
				opts := ListChangesetSpecsOpts{IDs: []int64{c.ID}}
				have, _, err := s.ListChangesetSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := btypes.ChangesetSpecs{c}
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			opts := ListChangesetSpecsOpts{}
			for _, c := range changesetSpecs {
				opts.IDs = append(opts.IDs, c.ID)
			}

			have, _, err := s.ListChangesetSpecs(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			// ListChangesetSpecs should not return ChangesetSpecs whose
			// repository was (soft-)deleted.
			if diff := cmp.Diff(have, changesetSpecs); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		})
	})

	t.Run("UpdateChangesetSpecBatchSpecID", func(t *testing.T) {
		for _, c := range changesetSpecs {
			c.BatchSpecID = 10001
			want := c.Clone()
			if err := s.UpdateChangesetSpecBatchSpecID(ctx, []int64{c.ID}, 10001); err != nil {
				t.Fatal(err)
			}
			have, err := s.GetChangesetSpecByID(ctx, c.ID)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("Get", func(t *testing.T) {
		want := changesetSpecs[1]
		tests := map[string]GetChangesetSpecOpts{
			"ByID":          {ID: want.ID},
			"ByRandID":      {RandID: want.RandID},
			"ByIDAndRandID": {ID: want.ID, RandID: want.RandID},
		}

		for name, opts := range tests {
			t.Run(name, func(t *testing.T) {
				have, err := s.GetChangesetSpec(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			})
		}

		t.Run("NoResults", func(t *testing.T) {
			opts := GetChangesetSpecOpts{ID: 0xdeadbeef}

			_, have := s.GetChangesetSpec(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("DeleteChangesetSpec", func(t *testing.T) {
		for i := range changesetSpecs {
			err := s.DeleteChangesetSpec(ctx, changesetSpecs[i].ID)
			if err != nil {
				t.Fatal(err)
			}

			count, err := s.CountChangesetSpecs(ctx, CountChangesetSpecsOpts{})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, len(changesetSpecs)-(i+1); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		}
	})

	t.Run("DeleteChangesetSpecs", func(t *testing.T) {
		t.Run("ByBatchSpecID", func(t *testing.T) {

			for i := range 3 {
				spec := &btypes.ChangesetSpec{
					BatchSpecID: int64(i + 1),
					BaseRepoID:  repo.ID,
					ExternalID:  "123",
					Type:        btypes.ChangesetSpecTypeExisting,
				}
				err := s.CreateChangesetSpec(ctx, spec)
				if err != nil {
					t.Fatal(err)
				}

				if err := s.DeleteChangesetSpecs(ctx, DeleteChangesetSpecsOpts{
					BatchSpecID: spec.BatchSpecID,
				}); err != nil {
					t.Fatal(err)
				}

				count, err := s.CountChangesetSpecs(ctx, CountChangesetSpecsOpts{BatchSpecID: spec.ID})
				if err != nil {
					t.Fatal(err)
				}

				if have, want := count, 0; have != want {
					t.Fatalf("have count: %d, want: %d", have, want)
				}
			}
		})

		t.Run("ByID", func(t *testing.T) {
			for i := range 3 {
				spec := &btypes.ChangesetSpec{
					BatchSpecID: int64(i + 1),
					BaseRepoID:  repo.ID,
					ExternalID:  "123",
					Type:        btypes.ChangesetSpecTypeExisting,
				}
				err := s.CreateChangesetSpec(ctx, spec)
				if err != nil {
					t.Fatal(err)
				}

				if err := s.DeleteChangesetSpecs(ctx, DeleteChangesetSpecsOpts{
					IDs: []int64{spec.ID},
				}); err != nil {
					t.Fatal(err)
				}

				_, err = s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: spec.ID})
				if err != ErrNoResults {
					t.Fatal("changeset spec not deleted")
				}
			}
		})
	})

	t.Run("DeleteUnattachedExpiredChangesetSpecs", func(t *testing.T) {
		underTTL := clock.Now().Add(-btypes.ChangesetSpecTTL + 24*time.Hour)
		overTTL := clock.Now().Add(-btypes.ChangesetSpecTTL - 24*time.Hour)

		type testCase struct {
			createdAt   time.Time
			wantDeleted bool
		}

		printTestCase := func(tc testCase) string {
			var tooOld bool
			if tc.createdAt.Equal(overTTL) {
				tooOld = true
			}

			return fmt.Sprintf("[tooOld=%t]", tooOld)
		}

		tests := []testCase{
			// ChangesetSpec was created but never attached to a BatchSpec
			{createdAt: underTTL, wantDeleted: false},
			{createdAt: overTTL, wantDeleted: true},
		}

		for _, tc := range tests {

			changesetSpec := &btypes.ChangesetSpec{
				// Need to set a RepoID otherwise GetChangesetSpec filters it out.
				BaseRepoID: repo.ID,
				ExternalID: "123",
				Type:       btypes.ChangesetSpecTypeExisting,
				CreatedAt:  tc.createdAt,
			}

			if err := s.CreateChangesetSpec(ctx, changesetSpec); err != nil {
				t.Fatal(err)
			}

			if err := s.DeleteUnattachedExpiredChangesetSpecs(ctx); err != nil {
				t.Fatal(err)
			}

			_, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: changesetSpec.ID})
			if err != nil && err != ErrNoResults {
				t.Fatal(err)
			}

			if tc.wantDeleted && err == nil {
				t.Fatalf("tc=%s\n\t want changeset spec to be deleted, but was NOT", printTestCase(tc))
			}

			if !tc.wantDeleted && err == ErrNoResults {
				t.Fatalf("tc=%s\n\t want changeset spec NOT to be deleted, but got deleted", printTestCase(tc))
			}
		}
	})

	t.Run("DeleteExpiredChangesetSpecs", func(t *testing.T) {
		underTTL := clock.Now().Add(-btypes.ChangesetSpecTTL + 24*time.Hour)
		overTTL := clock.Now().Add(-btypes.ChangesetSpecTTL - 24*time.Hour)
		overBatchSpecTTL := clock.Now().Add(-btypes.BatchSpecTTL - 24*time.Hour)

		type testCase struct {
			createdAt time.Time

			batchSpecApplied bool

			isCurrentSpec  bool
			isPreviousSpec bool

			wantDeleted bool
		}

		printTestCase := func(tc testCase) string {
			var tooOld bool
			if tc.createdAt.Equal(overTTL) || tc.createdAt.Equal(overBatchSpecTTL) {
				tooOld = true
			}

			return fmt.Sprintf(
				"[tooOld=%t, batchSpecApplied=%t, isCurrentSpec=%t, isPreviousSpec=%t]",
				tooOld, tc.batchSpecApplied, tc.isCurrentSpec, tc.isPreviousSpec,
			)
		}

		tests := []testCase{
			// Attached to BatchSpec that's applied to a BatchChange
			{batchSpecApplied: true, isCurrentSpec: true, createdAt: underTTL, wantDeleted: false},
			{batchSpecApplied: true, isCurrentSpec: true, createdAt: overTTL, wantDeleted: false},

			// BatchSpec is not applied to a BatchChange anymore and the
			// ChangesetSpecs are now the PreviousSpec.
			{isPreviousSpec: true, createdAt: underTTL, wantDeleted: false},
			{isPreviousSpec: true, createdAt: overTTL, wantDeleted: false},

			// Has a BatchSpec, but that BatchSpec is not applied
			// anymore, and the ChangesetSpec is neither the current, nor the
			// previous spec.
			{createdAt: underTTL, wantDeleted: false},
			{createdAt: overTTL, wantDeleted: false},
			{createdAt: overBatchSpecTTL, wantDeleted: true},
		}

		for _, tc := range tests {
			batchSpec := &btypes.BatchSpec{UserID: 4567, NamespaceUserID: 4567}

			if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
				t.Fatal(err)
			}

			if tc.batchSpecApplied {
				batchChange := &btypes.BatchChange{
					Name:            fmt.Sprintf("batch-change-for-spec-%d", batchSpec.ID),
					BatchSpecID:     batchSpec.ID,
					CreatorID:       batchSpec.UserID,
					NamespaceUserID: batchSpec.NamespaceUserID,
					LastApplierID:   batchSpec.UserID,
					LastAppliedAt:   time.Now(),
				}
				if err := s.CreateBatchChange(ctx, batchChange); err != nil {
					t.Fatal(err)
				}
			}

			changesetSpec := &btypes.ChangesetSpec{
				BatchSpecID: batchSpec.ID,
				// Need to set a RepoID otherwise GetChangesetSpec filters it out.
				BaseRepoID: repo.ID,
				ExternalID: "123",
				Type:       btypes.ChangesetSpecTypeExisting,
				CreatedAt:  tc.createdAt,
			}

			if err := s.CreateChangesetSpec(ctx, changesetSpec); err != nil {
				t.Fatal(err)
			}

			if tc.isCurrentSpec {
				changeset := &btypes.Changeset{
					ExternalServiceType: "github",
					RepoID:              1,
					CurrentSpecID:       changesetSpec.ID,
				}
				if err := s.CreateChangeset(ctx, changeset); err != nil {
					t.Fatal(err)
				}
			}

			if tc.isPreviousSpec {
				changeset := &btypes.Changeset{
					ExternalServiceType: "github",
					RepoID:              1,
					PreviousSpecID:      changesetSpec.ID,
				}
				if err := s.CreateChangeset(ctx, changeset); err != nil {
					t.Fatal(err)
				}
			}

			if err := s.DeleteExpiredChangesetSpecs(ctx); err != nil {
				t.Fatal(err)
			}

			_, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: changesetSpec.ID})
			if err != nil && err != ErrNoResults {
				t.Fatal(err)
			}

			if tc.wantDeleted && err == nil {
				t.Fatalf("tc=%s\n\t want changeset spec to be deleted, but was NOT", printTestCase(tc))
			}

			if !tc.wantDeleted && err == ErrNoResults {
				t.Fatalf("tc=%s\n\t want changeset spec NOT to be deleted, but got deleted", printTestCase(tc))
			}
		}
	})

	t.Run("GetRewirerMappings", func(t *testing.T) {
		// Create some test data
		user := bt.CreateTestUser(t, s.DatabaseDB(), true)
		ctx = actor.WithInternalActor(ctx)
		batchSpec := bt.CreateBatchSpec(t, ctx, s, "get-rewirer-mappings", user.ID, 0)
		var mappings = make(btypes.RewirerMappings, 3)
		changesetSpecIDs := make([]int64, 0, cap(mappings))
		for i := range cap(mappings) {
			spec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
				HeadRef:   fmt.Sprintf("refs/heads/test-get-rewirer-mappings-%d", i),
				Typ:       btypes.ChangesetSpecTypeBranch,
				Repo:      repo.ID,
				BatchSpec: batchSpec.ID,
			})
			changesetSpecIDs = append(changesetSpecIDs, spec.ID)
			mappings[i] = &btypes.RewirerMapping{
				ChangesetSpecID: spec.ID,
				RepoID:          repo.ID,
			}
		}

		t.Run("NoLimit", func(t *testing.T) {
			// Empty limit should return all entries.
			opts := GetRewirerMappingsOpts{
				BatchSpecID: batchSpec.ID,
			}
			ts, err := s.GetRewirerMappings(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			{
				have, want := ts.RepoIDs(), []api.RepoID{repo.ID}
				if len(have) != len(want) {
					t.Fatalf("listed %d repo ids, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			{
				have, want := ts.ChangesetIDs(), []int64{}
				if len(have) != len(want) {
					t.Fatalf("listed %d changeset ids, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			{
				have, want := ts.ChangesetSpecIDs(), changesetSpecIDs
				if len(have) != len(want) {
					t.Fatalf("listed %d changeset spec ids, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}

			{
				have, want := ts, mappings
				if len(have) != len(want) {
					t.Fatalf("listed %d mappings, want: %d", len(have), len(want))
				}

				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}
			}
		})

		t.Run("WithLimit", func(t *testing.T) {
			for i := 1; i <= len(mappings); i++ {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					opts := GetRewirerMappingsOpts{
						BatchSpecID: batchSpec.ID,
						LimitOffset: &database.LimitOffset{Limit: i},
					}
					ts, err := s.GetRewirerMappings(ctx, opts)
					if err != nil {
						t.Fatal(err)
					}

					{
						have, want := ts.RepoIDs(), []api.RepoID{repo.ID}
						if len(have) != len(want) {
							t.Fatalf("listed %d repo ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts.ChangesetIDs(), []int64{}
						if len(have) != len(want) {
							t.Fatalf("listed %d changeset ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts.ChangesetSpecIDs(), changesetSpecIDs[:i]
						if len(have) != len(want) {
							t.Fatalf("listed %d changeset spec ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts, mappings[:i]
						if len(have) != len(want) {
							t.Fatalf("listed %d mappings, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatal(diff)
						}
					}
				})
			}
		})

		t.Run("WithLimitAndOffset", func(t *testing.T) {
			offset := 0
			for i := 1; i <= len(mappings); i++ {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					opts := GetRewirerMappingsOpts{
						BatchSpecID: batchSpec.ID,
						LimitOffset: &database.LimitOffset{Limit: 1, Offset: offset},
					}
					ts, err := s.GetRewirerMappings(ctx, opts)
					if err != nil {
						t.Fatal(err)
					}

					{
						have, want := ts.RepoIDs(), []api.RepoID{repo.ID}
						if len(have) != len(want) {
							t.Fatalf("listed %d repo ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts.ChangesetIDs(), []int64{}
						if len(have) != len(want) {
							t.Fatalf("listed %d changeset ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts.ChangesetSpecIDs(), changesetSpecIDs[i-1:i]
						if len(have) != len(want) {
							t.Fatalf("listed %d changeset spec ids, want: %d", len(have), len(want))
						}

						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					{
						have, want := ts, mappings[i-1:i]
						if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
							t.Fatalf("opts: %+v, diff: %s", opts, diff)
						}
					}

					offset++
				})
			}
		})
	})

	t.Run("ListChangesetSpecsWithConflictingHeadRef", func(t *testing.T) {
		user := bt.CreateTestUser(t, s.DatabaseDB(), true)

		repo2 := bt.TestRepo(t, esStore, extsvc.KindGitHub)
		if err := repoStore.Create(ctx, repo2); err != nil {
			t.Fatal(err)
		}
		repo3 := bt.TestRepo(t, esStore, extsvc.KindGitHub)
		if err := repoStore.Create(ctx, repo3); err != nil {
			t.Fatal(err)
		}

		conflictingBatchSpec := bt.CreateBatchSpec(t, ctx, s, "no-conflicts", user.ID, 0)
		conflictingRef := "refs/heads/conflicting-head-ref"
		for _, opts := range []bt.TestSpecOpts{
			{ExternalID: "4321", Typ: btypes.ChangesetSpecTypeExisting, Repo: repo.ID, BatchSpec: conflictingBatchSpec.ID},
			{HeadRef: conflictingRef, Typ: btypes.ChangesetSpecTypeBranch, Repo: repo.ID, BatchSpec: conflictingBatchSpec.ID},
			{HeadRef: conflictingRef, Typ: btypes.ChangesetSpecTypeBranch, Repo: repo.ID, BatchSpec: conflictingBatchSpec.ID},
			{HeadRef: conflictingRef, Typ: btypes.ChangesetSpecTypeBranch, Repo: repo2.ID, BatchSpec: conflictingBatchSpec.ID},
			{HeadRef: conflictingRef, Typ: btypes.ChangesetSpecTypeBranch, Repo: repo2.ID, BatchSpec: conflictingBatchSpec.ID},
			{HeadRef: conflictingRef, Typ: btypes.ChangesetSpecTypeBranch, Repo: repo3.ID, BatchSpec: conflictingBatchSpec.ID},
		} {
			bt.CreateChangesetSpec(t, ctx, s, opts)
		}

		conflicts, err := s.ListChangesetSpecsWithConflictingHeadRef(ctx, conflictingBatchSpec.ID)
		if err != nil {
			t.Fatal(err)
		}
		if have, want := len(conflicts), 2; have != want {
			t.Fatalf("wrong number of conflicts. want=%d, have=%d", want, have)
		}
		for _, c := range conflicts {
			if c.RepoID != repo.ID && c.RepoID != repo2.ID {
				t.Fatalf("conflict has wrong RepoID: %d", c.RepoID)
			}
		}

		nonConflictingBatchSpec := bt.CreateBatchSpec(t, ctx, s, "no-conflicts", user.ID, 0)
		for _, opts := range []bt.TestSpecOpts{
			{ExternalID: "1234", Typ: btypes.ChangesetSpecTypeExisting, Repo: repo.ID, BatchSpec: nonConflictingBatchSpec.ID},
			{HeadRef: "refs/heads/branch-1", Typ: btypes.ChangesetSpecTypeBranch, Repo: repo.ID, BatchSpec: nonConflictingBatchSpec.ID},
			{HeadRef: "refs/heads/branch-2", Typ: btypes.ChangesetSpecTypeBranch, Repo: repo.ID, BatchSpec: nonConflictingBatchSpec.ID},
			{HeadRef: "refs/heads/branch-1", Typ: btypes.ChangesetSpecTypeBranch, Repo: repo2.ID, BatchSpec: nonConflictingBatchSpec.ID},
			{HeadRef: "refs/heads/branch-2", Typ: btypes.ChangesetSpecTypeBranch, Repo: repo2.ID, BatchSpec: nonConflictingBatchSpec.ID},
			{HeadRef: "refs/heads/branch-1", Typ: btypes.ChangesetSpecTypeBranch, Repo: repo3.ID, BatchSpec: nonConflictingBatchSpec.ID},
		} {
			bt.CreateChangesetSpec(t, ctx, s, opts)
		}

		conflicts, err = s.ListChangesetSpecsWithConflictingHeadRef(ctx, nonConflictingBatchSpec.ID)
		if err != nil {
			t.Fatal(err)
		}
		if have, want := len(conflicts), 0; have != want {
			t.Fatalf("wrong number of conflicts. want=%d, have=%d", want, have)
		}
	})
}

func testStoreGetRewirerMappingWithArchivedChangesets(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	user := bt.CreateTestUser(t, s.DatabaseDB(), false)

	// Create old batch spec and batch change
	oldBatchSpec := bt.CreateBatchSpec(t, ctx, s, "old", user.ID, 0)
	batchChange := bt.CreateBatchChange(t, ctx, s, "text", user.ID, oldBatchSpec.ID)

	// Create an archived changeset with a changeset spec
	oldSpec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repo.ID,
		BatchSpec: oldBatchSpec.ID,
		Title:     "foobar",
		Published: true,
		HeadRef:   "refs/heads/foobar",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})

	opts := bt.TestChangesetOpts{}
	opts.ExternalState = btypes.ChangesetExternalStateOpen
	opts.ExternalID = "1223"
	opts.ExternalServiceType = repo.ExternalRepo.ServiceType
	opts.Repo = repo.ID
	opts.BatchChange = batchChange.ID
	opts.PreviousSpec = oldSpec.ID
	opts.CurrentSpec = oldSpec.ID
	opts.OwnedByBatchChange = batchChange.ID
	opts.IsArchived = true

	bt.CreateChangeset(t, ctx, s, opts)

	// Get preview for new batch spec without any changeset specs
	newBatchSpec := bt.CreateBatchSpec(t, ctx, s, "new", user.ID, 0)
	mappings, err := s.GetRewirerMappings(ctx, GetRewirerMappingsOpts{
		BatchSpecID:   newBatchSpec.ID,
		BatchChangeID: batchChange.ID,
	})
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}

	if len(mappings) != 0 {
		t.Errorf("mappings returned, but none were expected")
	}
}

func testStoreChangesetSpecsCurrentState(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	// Let's set up a batch change with one of every changeset state.

	// First up, let's create a repo.
	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// Create a user.
	user := bt.CreateTestUser(t, s.DatabaseDB(), false)
	ctx = actor.WithInternalActor(ctx)

	// Next, we need old and new batch specs.
	oldBatchSpec := bt.CreateBatchSpec(t, ctx, s, "old", user.ID, 0)
	newBatchSpec := bt.CreateBatchSpec(t, ctx, s, "new", user.ID, 0)

	// That's enough to create a batch change, so let's do that.
	batchChange := bt.CreateBatchChange(t, ctx, s, "text", user.ID, oldBatchSpec.ID)

	// Now for some changeset specs.
	var (
		changesets = map[btypes.ChangesetState]*btypes.Changeset{}
		oldSpecs   = map[btypes.ChangesetState]*btypes.ChangesetSpec{}
		newSpecs   = map[btypes.ChangesetState]*btypes.ChangesetSpec{}

		// The keys are the desired current state that we'll search for; the
		// values the changeset options we need to set on the changeset.
		states = map[btypes.ChangesetState]*bt.TestChangesetOpts{
			btypes.ChangesetStateRetrying:    {ReconcilerState: btypes.ReconcilerStateErrored},
			btypes.ChangesetStateFailed:      {ReconcilerState: btypes.ReconcilerStateFailed},
			btypes.ChangesetStateScheduled:   {ReconcilerState: btypes.ReconcilerStateScheduled},
			btypes.ChangesetStateProcessing:  {ReconcilerState: btypes.ReconcilerStateQueued, PublicationState: btypes.ChangesetPublicationStateUnpublished},
			btypes.ChangesetStateUnpublished: {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStateUnpublished},
			btypes.ChangesetStateDraft:       {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateDraft},
			btypes.ChangesetStateOpen:        {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateOpen},
			btypes.ChangesetStateClosed:      {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateClosed},
			btypes.ChangesetStateMerged:      {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateMerged},
			btypes.ChangesetStateDeleted:     {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateDeleted},
			btypes.ChangesetStateReadOnly:    {ReconcilerState: btypes.ReconcilerStateCompleted, PublicationState: btypes.ChangesetPublicationStatePublished, ExternalState: btypes.ChangesetExternalStateReadOnly},
		}
	)
	for state, opts := range states {
		specOpts := bt.TestSpecOpts{
			User:      user.ID,
			Repo:      repo.ID,
			BatchSpec: oldBatchSpec.ID,
			Title:     string(state),
			Published: true,
			HeadRef:   string(state),
			Typ:       btypes.ChangesetSpecTypeBranch,
		}
		oldSpecs[state] = bt.CreateChangesetSpec(t, ctx, s, specOpts)

		specOpts.BatchSpec = newBatchSpec.ID
		newSpecs[state] = bt.CreateChangesetSpec(t, ctx, s, specOpts)

		if opts.ExternalState != "" {
			opts.ExternalID = string(state)
		}
		opts.ExternalServiceType = repo.ExternalRepo.ServiceType
		opts.Repo = repo.ID
		opts.BatchChange = batchChange.ID
		opts.CurrentSpec = oldSpecs[state].ID
		opts.OwnedByBatchChange = batchChange.ID
		opts.Metadata = map[string]any{"Title": string(state)}
		changesets[state] = bt.CreateChangeset(t, ctx, s, *opts)
	}

	// OK, there's lots of good stuff here. Let's work our way through the
	// rewirer options and see what we get.
	for state := range states {
		t.Run(string(state), func(t *testing.T) {
			mappings, err := s.GetRewirerMappings(ctx, GetRewirerMappingsOpts{
				BatchSpecID:   newBatchSpec.ID,
				BatchChangeID: batchChange.ID,
				CurrentState:  &state,
			})
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			have := []int64{}
			for _, mapping := range mappings {
				have = append(have, mapping.ChangesetID)
			}

			want := []int64{changesets[state].ID}
			if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
				t.Errorf("unexpected changesets (-have +want):\n%s", diff)
			}
		})
	}
}

func testStoreChangesetSpecsCurrentStateAndTextSearch(t *testing.T, ctx context.Context, s *Store, _ bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	// Let's set up a batch change with one of every changeset state.

	// First up, let's create a repo.
	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)
	if err := repoStore.Create(ctx, repo); err != nil {
		t.Fatal(err)
	}

	// Create a user.
	user := bt.CreateTestUser(t, s.DatabaseDB(), false)
	ctx = actor.WithInternalActor(ctx)

	// Next, we need old and new batch specs.
	oldBatchSpec := bt.CreateBatchSpec(t, ctx, s, "old", user.ID, 0)
	newBatchSpec := bt.CreateBatchSpec(t, ctx, s, "new", user.ID, 0)

	// That's enough to create a batch change, so let's do that.
	batchChange := bt.CreateBatchChange(t, ctx, s, "text", user.ID, oldBatchSpec.ID)

	// Now we'll add three old and new pairs of changeset specs. Two will have
	// matching statuses, and a different two will have matching names.
	createChangesetSpecPair := func(t *testing.T, ctx context.Context, s *Store, oldBatchSpec, newBatchSpec *btypes.BatchSpec, opts bt.TestSpecOpts) (old *btypes.ChangesetSpec) {
		opts.BatchSpec = oldBatchSpec.ID
		old = bt.CreateChangesetSpec(t, ctx, s, opts)

		opts.BatchSpec = newBatchSpec.ID
		_ = bt.CreateChangesetSpec(t, ctx, s, opts)

		return old
	}
	oldOpenFoo := createChangesetSpecPair(t, ctx, s, oldBatchSpec, newBatchSpec, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repo.ID,
		BatchSpec: oldBatchSpec.ID,
		Title:     "foo",
		Published: true,
		HeadRef:   "open-foo",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	oldOpenBar := createChangesetSpecPair(t, ctx, s, oldBatchSpec, newBatchSpec, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repo.ID,
		BatchSpec: oldBatchSpec.ID,
		Title:     "bar",
		Published: true,
		HeadRef:   "open-bar",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	oldClosedFoo := createChangesetSpecPair(t, ctx, s, oldBatchSpec, newBatchSpec, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repo.ID,
		BatchSpec: oldBatchSpec.ID,
		Title:     "foo",
		Published: true,
		HeadRef:   "closed-foo",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})

	// Finally, the changesets.
	openFoo := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repo.ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldOpenFoo.ID,
		ExternalServiceType: repo.ExternalRepo.ServiceType,
		ExternalID:          "5678",
		ExternalState:       btypes.ChangesetExternalStateOpen,
		ReconcilerState:     btypes.ReconcilerStateCompleted,
		PublicationState:    btypes.ChangesetPublicationStatePublished,
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"Title": "foo",
		},
	})
	openBar := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repo.ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldOpenBar.ID,
		ExternalServiceType: repo.ExternalRepo.ServiceType,
		ExternalID:          "5679",
		ExternalState:       btypes.ChangesetExternalStateOpen,
		ReconcilerState:     btypes.ReconcilerStateCompleted,
		PublicationState:    btypes.ChangesetPublicationStatePublished,
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"Title": "bar",
		},
	})
	_ = bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repo.ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldClosedFoo.ID,
		ExternalServiceType: repo.ExternalRepo.ServiceType,
		ExternalID:          "5680",
		ExternalState:       btypes.ChangesetExternalStateClosed,
		ReconcilerState:     btypes.ReconcilerStateCompleted,
		PublicationState:    btypes.ChangesetPublicationStatePublished,
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"Title": "foo",
		},
	})

	for name, tc := range map[string]struct {
		opts GetRewirerMappingsOpts
		want []*btypes.Changeset
	}{
		"state and text": {
			opts: GetRewirerMappingsOpts{
				TextSearch:   []search.TextSearchTerm{{Term: "foo"}},
				CurrentState: pointers.Ptr(btypes.ChangesetStateOpen),
			},
			want: []*btypes.Changeset{openFoo},
		},
		"state and not text": {
			opts: GetRewirerMappingsOpts{
				TextSearch:   []search.TextSearchTerm{{Term: "foo", Not: true}},
				CurrentState: pointers.Ptr(btypes.ChangesetStateOpen),
			},
			want: []*btypes.Changeset{openBar},
		},
		"state match only": {
			opts: GetRewirerMappingsOpts{
				TextSearch:   []search.TextSearchTerm{{Term: "bar"}},
				CurrentState: pointers.Ptr(btypes.ChangesetStateClosed),
			},
			want: []*btypes.Changeset{},
		},
		"text match only": {
			opts: GetRewirerMappingsOpts{
				TextSearch:   []search.TextSearchTerm{{Term: "foo"}},
				CurrentState: pointers.Ptr(btypes.ChangesetStateMerged),
			},
			want: []*btypes.Changeset{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.opts.BatchSpecID = newBatchSpec.ID
			tc.opts.BatchChangeID = batchChange.ID
			mappings, err := s.GetRewirerMappings(ctx, tc.opts)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			have := []int64{}
			for _, mapping := range mappings {
				have = append(have, mapping.ChangesetID)
			}

			want := []int64{}
			for _, changeset := range tc.want {
				want = append(want, changeset.ID)
			}

			if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
				t.Errorf("unexpected changesets (-have +want):\n%s", diff)
			}
		})
	}
}

func testStoreChangesetSpecsTextSearch(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	// OK, let's set up an interesting scenario. We're going to set up a
	// batch change that tracks two changesets in different repositories, and
	// creates two changesets in those same repositories with different names.

	// First up, let's create the repos.
	repos := []*types.Repo{
		bt.TestRepo(t, esStore, extsvc.KindGitHub),
		bt.TestRepo(t, esStore, extsvc.KindGitLab),
	}
	for _, repo := range repos {
		if err := repoStore.Create(ctx, repo); err != nil {
			t.Fatal(err)
		}
	}

	// Create a user.
	user := bt.CreateTestUser(t, s.DatabaseDB(), false)
	ctx = actor.WithInternalActor(ctx)

	// Next, we need a batch spec.
	oldBatchSpec := bt.CreateBatchSpec(t, ctx, s, "text", user.ID, 0)

	// That's enough to create a batch change, so let's do that.
	batchChange := bt.CreateBatchChange(t, ctx, s, "text", user.ID, oldBatchSpec.ID)

	// Now we can create the changeset specs.
	oldTrackedGitHubSpec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:       user.ID,
		Repo:       repos[0].ID,
		BatchSpec:  oldBatchSpec.ID,
		ExternalID: "1234",
		Typ:        btypes.ChangesetSpecTypeExisting,
	})
	oldTrackedGitLabSpec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:       user.ID,
		Repo:       repos[1].ID,
		BatchSpec:  oldBatchSpec.ID,
		ExternalID: "1234",
		Typ:        btypes.ChangesetSpecTypeExisting,
	})
	oldBranchGitHubSpec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repos[0].ID,
		BatchSpec: oldBatchSpec.ID,
		HeadRef:   "main",
		Published: true,
		Title:     "GitHub branch",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	oldBranchGitLabSpec := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repos[1].ID,
		BatchSpec: oldBatchSpec.ID,
		HeadRef:   "main",
		Published: true,
		Title:     "GitLab branch",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})

	// We also need actual changesets.
	oldTrackedGitHub := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repos[0].ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldTrackedGitHubSpec.ID,
		ExternalServiceType: repos[0].ExternalRepo.ServiceType,
		ExternalID:          "1234",
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"Title": "Tracked GitHub",
		},
	})
	oldTrackedGitLab := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repos[1].ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldTrackedGitLabSpec.ID,
		ExternalServiceType: repos[1].ExternalRepo.ServiceType,
		ExternalID:          "1234",
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"title": "Tracked GitLab",
		},
	})
	oldBranchGitHub := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repos[0].ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldBranchGitHubSpec.ID,
		ExternalServiceType: repos[0].ExternalRepo.ServiceType,
		ExternalID:          "5678",
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"Title": "GitHub branch",
		},
	})
	oldBranchGitLab := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{
		Repo:                repos[1].ID,
		BatchChange:         batchChange.ID,
		CurrentSpec:         oldBranchGitLabSpec.ID,
		ExternalServiceType: repos[1].ExternalRepo.ServiceType,
		ExternalID:          "5678",
		OwnedByBatchChange:  batchChange.ID,
		Metadata: map[string]any{
			"title": "GitLab branch",
		},
	})
	// Cool. Now let's set up a new batch spec.
	newBatchSpec := bt.CreateBatchSpec(t, ctx, s, "text", user.ID, 0)

	// And we need all new changeset specs to go into that spec.
	newTrackedGitHub := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:       user.ID,
		Repo:       repos[0].ID,
		BatchSpec:  newBatchSpec.ID,
		ExternalID: "1234",
		Typ:        btypes.ChangesetSpecTypeExisting,
	})
	newTrackedGitLab := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:       user.ID,
		Repo:       repos[1].ID,
		BatchSpec:  newBatchSpec.ID,
		ExternalID: "1234",
		Typ:        btypes.ChangesetSpecTypeExisting,
	})
	newBranchGitHub := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repos[0].ID,
		BatchSpec: newBatchSpec.ID,
		HeadRef:   "main",
		Published: true,
		Title:     "New GitHub branch",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})
	newBranchGitLab := bt.CreateChangesetSpec(t, ctx, s, bt.TestSpecOpts{
		User:      user.ID,
		Repo:      repos[1].ID,
		BatchSpec: newBatchSpec.ID,
		HeadRef:   "main",
		Published: true,
		Title:     "New GitLab branch",
		Typ:       btypes.ChangesetSpecTypeBranch,
	})

	// A couple of hundred lines of boilerplate later, we have a scenario! Let's
	// use it.

	// Well, OK, I lied: we're not _quite_ done with the boilerplate. To keep
	// the test cases somewhat readable, we'll define the four possible mappings
	// we can get before we get to defining the test cases.
	trackedGitHub := &btypes.RewirerMapping{
		ChangesetSpecID: newTrackedGitHub.ID,
		ChangesetID:     oldTrackedGitHub.ID,
		RepoID:          repos[0].ID,
	}
	trackedGitLab := &btypes.RewirerMapping{
		ChangesetSpecID: newTrackedGitLab.ID,
		ChangesetID:     oldTrackedGitLab.ID,
		RepoID:          repos[1].ID,
	}
	branchGitHub := &btypes.RewirerMapping{
		ChangesetSpecID: newBranchGitHub.ID,
		ChangesetID:     oldBranchGitHub.ID,
		RepoID:          repos[0].ID,
	}
	branchGitLab := &btypes.RewirerMapping{
		ChangesetSpecID: newBranchGitLab.ID,
		ChangesetID:     oldBranchGitLab.ID,
		RepoID:          repos[1].ID,
	}

	for name, tc := range map[string]struct {
		search []search.TextSearchTerm
		want   btypes.RewirerMappings
	}{
		"nil search": {
			want: btypes.RewirerMappings{trackedGitHub, trackedGitLab, branchGitHub, branchGitLab},
		},
		"empty search": {
			search: []search.TextSearchTerm{},
			want:   btypes.RewirerMappings{trackedGitHub, trackedGitLab, branchGitHub, branchGitLab},
		},
		"no matches": {
			search: []search.TextSearchTerm{{Term: "this is not a thing"}},
			want:   nil,
		},
		"no matches due to conflicting requirements": {
			search: []search.TextSearchTerm{
				{Term: "GitHub"},
				{Term: "GitLab"},
			},
			want: nil,
		},
		"no matches due to even more conflicting requirements": {
			search: []search.TextSearchTerm{
				{Term: "GitHub"},
				{Term: "GitHub", Not: true},
			},
			want: nil,
		},
		"one term, matched on title": {
			search: []search.TextSearchTerm{{Term: "New GitHub branch"}},
			want:   btypes.RewirerMappings{branchGitHub},
		},
		"two terms, matched on title AND title": {
			search: []search.TextSearchTerm{
				{Term: "New GitHub"},
				{Term: "branch"},
			},
			want: btypes.RewirerMappings{branchGitHub},
		},
		"two terms, matched on title AND repo": {
			search: []search.TextSearchTerm{
				{Term: "New"},
				{Term: string(repos[0].Name)},
			},
			want: btypes.RewirerMappings{branchGitHub},
		},
		"one term, matched on repo": {
			search: []search.TextSearchTerm{{Term: string(repos[0].Name)}},
			want:   btypes.RewirerMappings{trackedGitHub, branchGitHub},
		},
		"one negated term, three title matches": {
			search: []search.TextSearchTerm{{Term: "New GitHub branch", Not: true}},
			want:   btypes.RewirerMappings{trackedGitHub, trackedGitLab, branchGitLab},
		},
		"two negated terms, one title AND repo match": {
			search: []search.TextSearchTerm{
				{Term: "New", Not: true},
				{Term: string(repos[0].Name), Not: true},
			},
			want: btypes.RewirerMappings{trackedGitLab},
		},
		"mixed positive and negative terms": {
			search: []search.TextSearchTerm{
				{Term: "New", Not: true},
				{Term: string(repos[0].Name)},
			},
			want: btypes.RewirerMappings{trackedGitHub},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("no limits", func(t *testing.T) {
				have, err := s.GetRewirerMappings(ctx, GetRewirerMappingsOpts{
					BatchSpecID:   newBatchSpec.ID,
					BatchChangeID: batchChange.ID,
					TextSearch:    tc.search,
				})
				if err != nil {
					t.Errorf("unexpected error: %+v", err)
				}

				if diff := cmp.Diff(have, tc.want, cmtRewirerMappingsOpts); diff != "" {
					t.Errorf("unexpected mappings (-have +want):\n%s", diff)
				}
			})

			t.Run("with limit", func(t *testing.T) {
				have, err := s.GetRewirerMappings(ctx, GetRewirerMappingsOpts{
					BatchSpecID:   newBatchSpec.ID,
					BatchChangeID: batchChange.ID,
					LimitOffset:   &database.LimitOffset{Limit: 1},
					TextSearch:    tc.search,
				})
				if err != nil {
					t.Errorf("unexpected error: %+v", err)
				}

				var want btypes.RewirerMappings
				if len(tc.want) > 0 {
					want = tc.want[0:1]
				}
				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Errorf("unexpected mappings (-have +want):\n%s", diff)
				}
			})

			t.Run("with offset and limit", func(t *testing.T) {
				have, err := s.GetRewirerMappings(ctx, GetRewirerMappingsOpts{
					BatchSpecID:   newBatchSpec.ID,
					BatchChangeID: batchChange.ID,
					LimitOffset:   &database.LimitOffset{Offset: 1, Limit: 1},
					TextSearch:    tc.search,
				})
				if err != nil {
					t.Errorf("unexpected error: %+v", err)
				}

				var want btypes.RewirerMappings
				if len(tc.want) > 1 {
					want = tc.want[1:2]
				}
				if diff := cmp.Diff(have, want, cmtRewirerMappingsOpts); diff != "" {
					t.Errorf("unexpected mappings (-have +want):\n%s", diff)
				}
			})
		})
	}
}

func testStoreChangesetSpecsPublishedValues(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)

	err := repoStore.Create(ctx, repo)
	require.NoError(t, err)

	t.Run("NULL", func(t *testing.T) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(1234),
			BatchSpecID: int64(910),
			BaseRepoID:  repo.ID,
			Published:   batcheslib.PublishedValue{},
		}

		err := s.CreateChangesetSpec(ctx, c)
		require.NoError(t, err)
		t.Cleanup(func() {
			s.DeleteChangesetSpec(ctx, c.ID)
		})

		val, _, err := basestore.ScanFirstNullString(s.Query(ctx, sqlf.Sprintf("SELECT published FROM changeset_specs WHERE id = %d", c.ID)))
		require.NoError(t, err)
		assert.Empty(t, val)

		actual, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: c.ID})
		require.NoError(t, err)
		assert.True(t, actual.Published.Nil())
	})

	t.Run("True", func(t *testing.T) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(1234),
			BatchSpecID: int64(910),
			BaseRepoID:  repo.ID,
			Published:   batcheslib.PublishedValue{Val: true},
		}

		err := s.CreateChangesetSpec(ctx, c)
		require.NoError(t, err)
		t.Cleanup(func() {
			s.DeleteChangesetSpec(ctx, c.ID)
		})

		val, _, err := basestore.ScanFirstBool(s.Query(ctx, sqlf.Sprintf("SELECT published FROM changeset_specs WHERE id = %d", c.ID)))
		require.NoError(t, err)
		assert.True(t, val)

		actual, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: c.ID})
		require.NoError(t, err)
		assert.True(t, actual.Published.True())
	})

	t.Run("False", func(t *testing.T) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(1234),
			BatchSpecID: int64(910),
			BaseRepoID:  repo.ID,
			Published:   batcheslib.PublishedValue{Val: false},
		}

		err := s.CreateChangesetSpec(ctx, c)
		require.NoError(t, err)
		t.Cleanup(func() {
			s.DeleteChangesetSpec(ctx, c.ID)
		})

		val, _, err := basestore.ScanFirstBool(s.Query(ctx, sqlf.Sprintf("SELECT published FROM changeset_specs WHERE id = %d", c.ID)))
		require.NoError(t, err)
		assert.False(t, val)

		actual, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: c.ID})
		require.NoError(t, err)
		assert.True(t, actual.Published.False())
	})

	t.Run("Draft", func(t *testing.T) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(1234),
			BatchSpecID: int64(910),
			BaseRepoID:  repo.ID,
			Published:   batcheslib.PublishedValue{Val: "draft"},
		}

		err := s.CreateChangesetSpec(ctx, c)
		require.NoError(t, err)
		t.Cleanup(func() {
			s.DeleteChangesetSpec(ctx, c.ID)
		})

		val, _, err := basestore.ScanFirstNullString(s.Query(ctx, sqlf.Sprintf("SELECT published FROM changeset_specs WHERE id = %d", c.ID)))
		require.NoError(t, err)
		assert.Equal(t, `"draft"`, val)

		actual, err := s.GetChangesetSpec(ctx, GetChangesetSpecOpts{ID: c.ID})
		require.NoError(t, err)
		assert.True(t, actual.Published.Draft())
	})

	t.Run("Invalid", func(t *testing.T) {
		c := &btypes.ChangesetSpec{
			UserID:      int32(1234),
			BatchSpecID: int64(910),
			BaseRepoID:  repo.ID,
			Published:   batcheslib.PublishedValue{Val: "foo-bar"},
		}

		err := s.CreateChangesetSpec(ctx, c)
		assert.Error(t, err)
		assert.Equal(t, "json: error calling MarshalJSON for type batches.PublishedValue: invalid PublishedValue: foo-bar (string)", err.Error())
	})
}
