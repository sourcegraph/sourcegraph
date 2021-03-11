package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/batch-change-utils/overridable"

	ct "github.com/sourcegraph/sourcegraph/enterprise/internal/batches/testing"
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

func testStoreBatchSpecs(t *testing.T, ctx context.Context, s *Store, clock ct.Clock) {
	batchSpecs := make([]*batches.BatchSpec, 0, 3)

	t.Run("Create", func(t *testing.T) {
		for i := 0; i < cap(batchSpecs); i++ {
			c := &batches.BatchSpec{
				RawSpec: `{"name": "Foobar", "description": "My description"}`,
				Spec: batches.BatchSpecFields{
					Name:        "Foobar",
					Description: "My description",
					ChangesetTemplate: batches.ChangesetTemplate{
						Title:  "Hello there",
						Body:   "This is the body",
						Branch: "my-branch",
						Commit: batches.CommitTemplate{
							Message: "commit message",
						},
						Published: overridable.FromBoolOrString(false),
					},
				},
				UserID: int32(i + 1234),
			}

			if i%2 == 0 {
				c.NamespaceOrgID = 23
			} else {
				c.NamespaceUserID = c.UserID
			}

			want := c.Clone()
			have := c

			err := s.CreateBatchSpec(ctx, have)
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

			batchSpecs = append(batchSpecs, c)
		}
	})

	if len(batchSpecs) != cap(batchSpecs) {
		t.Fatalf("batchSpecs is empty. creation failed")
	}

	t.Run("Count", func(t *testing.T) {
		count, err := s.CountBatchSpecs(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if have, want := count, len(batchSpecs); have != want {
			t.Fatalf("have count: %d, want: %d", have, want)
		}
	})

	t.Run("List", func(t *testing.T) {
		t.Run("NoLimit", func(t *testing.T) {
			// Empty should return all entries
			opts := ListBatchSpecsOpts{}

			ts, next, err := s.ListBatchSpecs(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := next, int64(0); have != want {
				t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
			}

			have, want := ts, batchSpecs
			if len(have) != len(want) {
				t.Fatalf("listed %d batchSpecs, want: %d", len(have), len(want))
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		})

		t.Run("WithLimit", func(t *testing.T) {
			for i := 1; i <= len(batchSpecs); i++ {
				cs, next, err := s.ListBatchSpecs(ctx, ListBatchSpecsOpts{LimitOpts: LimitOpts{Limit: i}})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(batchSpecs) {
						want = batchSpecs[i].ID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := cs, batchSpecs[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d batchSpecs, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}

		})

		t.Run("WithLimitAndCursor", func(t *testing.T) {
			var cursor int64
			for i := 1; i <= len(batchSpecs); i++ {
				opts := ListBatchSpecsOpts{Cursor: cursor, LimitOpts: LimitOpts{Limit: 1}}
				have, next, err := s.ListBatchSpecs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := batchSpecs[i-1 : i]
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}

				cursor = next
			}
		})
	})

	t.Run("Update", func(t *testing.T) {
		for _, c := range batchSpecs {
			c.UserID += 1234

			clock.Add(1 * time.Second)

			want := c
			want.UpdatedAt = clock.Now()

			have := c.Clone()
			if err := s.UpdateBatchSpec(ctx, have); err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("Get", func(t *testing.T) {
		want := batchSpecs[1]
		tests := map[string]GetBatchSpecOpts{
			"ByID":          {ID: want.ID},
			"ByRandID":      {RandID: want.RandID},
			"ByIDAndRandID": {ID: want.ID, RandID: want.RandID},
		}

		for name, opts := range tests {
			t.Run(name, func(t *testing.T) {
				have, err := s.GetBatchSpec(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatal(diff)
				}
			})
		}

		t.Run("NoResults", func(t *testing.T) {
			opts := GetBatchSpecOpts{ID: 0xdeadbeef}

			_, have := s.GetBatchSpec(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("GetNewestBatchSpec", func(t *testing.T) {
		t.Run("NotFound", func(t *testing.T) {
			opts := GetNewestBatchSpecOpts{
				NamespaceUserID: 1235,
				Name:            "Foobar",
				UserID:          1234567,
			}

			_, err := s.GetNewestBatchSpec(ctx, opts)
			if err != ErrNoResults {
				t.Errorf("unexpected error: have=%v want=%v", err, ErrNoResults)
			}
		})

		t.Run("NamespaceUser", func(t *testing.T) {
			opts := GetNewestBatchSpecOpts{
				NamespaceUserID: 1235,
				Name:            "Foobar",
				UserID:          1235 + 1234,
			}

			have, err := s.GetNewestBatchSpec(ctx, opts)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			want := batchSpecs[1]
			if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("unexpected batch spec:\n%s", diff)
			}
		})

		t.Run("NamespaceOrg", func(t *testing.T) {
			opts := GetNewestBatchSpecOpts{
				NamespaceOrgID: 23,
				Name:           "Foobar",
				UserID:         1234 + 1234,
			}

			have, err := s.GetNewestBatchSpec(ctx, opts)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			want := batchSpecs[0]
			if diff := cmp.Diff(have, want); diff != "" {
				t.Errorf("unexpected batch spec:\n%s", diff)
			}
		})
	})

	t.Run("Delete", func(t *testing.T) {
		for i := range batchSpecs {
			err := s.DeleteBatchSpec(ctx, batchSpecs[i].ID)
			if err != nil {
				t.Fatal(err)
			}

			count, err := s.CountBatchSpecs(ctx)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := count, len(batchSpecs)-(i+1); have != want {
				t.Fatalf("have count: %d, want: %d", have, want)
			}
		}
	})

	t.Run("DeleteExpiredBatchSpecs", func(t *testing.T) {
		underTTL := clock.Now().Add(-batches.BatchSpecTTL + 1*time.Minute)
		overTTL := clock.Now().Add(-batches.BatchSpecTTL - 1*time.Minute)

		tests := []struct {
			createdAt         time.Time
			hasBatchChange    bool
			hasChangesetSpecs bool
			wantDeleted       bool
		}{
			{hasBatchChange: false, hasChangesetSpecs: false, createdAt: underTTL, wantDeleted: false},
			{hasBatchChange: false, hasChangesetSpecs: false, createdAt: overTTL, wantDeleted: true},

			{hasBatchChange: false, hasChangesetSpecs: true, createdAt: underTTL, wantDeleted: false},
			{hasBatchChange: false, hasChangesetSpecs: true, createdAt: overTTL, wantDeleted: false},

			{hasBatchChange: true, hasChangesetSpecs: true, createdAt: underTTL, wantDeleted: false},
			{hasBatchChange: true, hasChangesetSpecs: true, createdAt: overTTL, wantDeleted: false},
		}

		for _, tc := range tests {
			batchSpec := &batches.BatchSpec{
				UserID:          1,
				NamespaceUserID: 1,
				CreatedAt:       tc.createdAt,
			}

			if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
				t.Fatal(err)
			}

			if tc.hasBatchChange {
				batchChange := &batches.BatchChange{
					Name:             "not-blank",
					InitialApplierID: 1,
					NamespaceUserID:  1,
					BatchSpecID:      batchSpec.ID,
					LastApplierID:    1,
					LastAppliedAt:    time.Now(),
				}
				if err := s.CreateBatchChange(ctx, batchChange); err != nil {
					t.Fatal(err)
				}
			}

			if tc.hasChangesetSpecs {
				changesetSpec := &batches.ChangesetSpec{
					RepoID:      1,
					BatchSpecID: batchSpec.ID,
				}
				if err := s.CreateChangesetSpec(ctx, changesetSpec); err != nil {
					t.Fatal(err)
				}
			}

			if err := s.DeleteExpiredBatchSpecs(ctx); err != nil {
				t.Fatal(err)
			}

			haveBatchSpecs, err := s.GetBatchSpec(ctx, GetBatchSpecOpts{ID: batchSpec.ID})
			if err != nil && err != ErrNoResults {
				t.Fatal(err)
			}

			if tc.wantDeleted && err == nil {
				t.Fatalf("tc=%+v\n\t want batch spec to be deleted. got: %v", tc, haveBatchSpecs)
			}

			if !tc.wantDeleted && err == ErrNoResults {
				t.Fatalf("tc=%+v\n\t want batch spec NOT to be deleted, but got deleted", tc)
			}
		}
	})
}
