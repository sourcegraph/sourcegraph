package store

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
)

func testStoreBulkOperations(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	logger := logtest.Scoped(t)
	repoStore := database.ReposWith(logger, s)
	esStore := database.ExternalServicesWith(logger, s)

	repo := bt.TestRepo(t, esStore, extsvc.KindGitHub)
	deletedRepo := bt.TestRepo(t, esStore, extsvc.KindGitHub).With(typestest.Opt.RepoDeletedAt(clock.Now()))

	if err := repoStore.Create(ctx, repo, deletedRepo); err != nil {
		t.Fatal(err)
	}
	if err := repoStore.Delete(ctx, deletedRepo.ID); err != nil {
		t.Fatal(err)
	}

	changeset := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{Repo: repo.ID})
	changesetWithDeletedRepo := bt.CreateChangeset(t, ctx, s, bt.TestChangesetOpts{Repo: deletedRepo.ID})
	var batchChangeID int64 = 12345

	failureMessage := "bad error"
	jobs := make([]*btypes.ChangesetJob, 0, 3)
	bulkOperations := make([]*btypes.BulkOperation, 0, 2)
	for i := range cap(jobs) {
		groupID, err := RandomID()
		if err != nil {
			t.Fatal(err)
		}
		c := &btypes.ChangesetJob{
			BulkGroup:     groupID,
			UserID:        int32(i + 1234),
			BatchChangeID: batchChangeID,
			ChangesetID:   changeset.ID,
			State:         btypes.ChangesetJobStateQueued,
			JobType:       btypes.ChangesetJobTypeComment,
		}

		if i == cap(jobs)-1 {
			c.ChangesetID = changesetWithDeletedRepo.ID
		}
		if i == 0 {
			c.State = btypes.ChangesetJobStateFailed
			failureMessage := "bad error"
			c.FailureMessage = &failureMessage
		}
		jobs = append(jobs, c)
	}
	err := s.CreateChangesetJob(ctx, jobs...)
	if err != nil {
		t.Fatal(err)
	}
	for i := range cap(bulkOperations) {
		j := &btypes.BulkOperation{
			ID:             jobs[i].BulkGroup,
			DBID:           jobs[i].ID,
			State:          btypes.BulkOperationStateProcessing,
			Type:           btypes.ChangesetJobTypeComment,
			ChangesetCount: 1,
			UserID:         jobs[i].UserID,
			CreatedAt:      clock.Now(),
		}
		if i == 0 {
			j.Progress = 1
			j.State = btypes.BulkOperationStateFailed
		}
		bulkOperations = append(bulkOperations, j)
	}

	t.Run("Get", func(t *testing.T) {
		for i, job := range jobs {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				have, err := s.GetBulkOperation(ctx, GetBulkOperationOpts{ID: job.BulkGroup})
				if i == cap(jobs)-1 {
					if err != ErrNoResults {
						t.Fatal("unexpected non-no-results error")
					}
					return
				} else if err != nil {
					t.Fatal(err)
				}

				if diff := cmp.Diff(have, bulkOperations[i]); diff != "" {
					t.Fatal(diff)
				}
			})
		}

		t.Run("NoResults", func(t *testing.T) {
			opts := GetBulkOperationOpts{ID: "deadbeef"}

			_, have := s.GetBulkOperation(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("Count", func(t *testing.T) {
		t.Run("All", func(t *testing.T) {
			want := len(bulkOperations)
			have, err := s.CountBulkOperations(ctx, CountBulkOperationsOpts{BatchChangeID: batchChangeID})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := CountBulkOperationsOpts{BatchChangeID: -1}

			have, err := s.CountBulkOperations(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}
			want := 0

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		reverse := func(s []*btypes.BulkOperation) []*btypes.BulkOperation {
			a := make([]*btypes.BulkOperation, len(s))
			copy(a, s)

			for i := len(a)/2 - 1; i >= 0; i-- {
				opp := len(a) - 1 - i
				a[i], a[opp] = a[opp], a[i]
			}

			return a
		}
		reverseBulkOperations := reverse(bulkOperations)
		t.Run("NoLimit", func(t *testing.T) {
			// Empty limit should return all entries.
			opts := ListBulkOperationsOpts{BatchChangeID: batchChangeID}
			ts, next, err := s.ListBulkOperations(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}

			if have, want := next, int64(0); have != want {
				t.Fatalf("opts: %+v: have next %v, want %v", opts, have, want)
			}

			have, want := ts, reverseBulkOperations
			if len(have) != len(want) {
				t.Fatalf("listed %d bulk operations, want: %d", len(have), len(want))
			}

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatalf("opts: %+v, diff: %s", opts, diff)
			}
		})

		t.Run("WithLimit", func(t *testing.T) {
			for i := 1; i <= len(reverseBulkOperations); i++ {
				cs, next, err := s.ListBulkOperations(ctx, ListBulkOperationsOpts{BatchChangeID: batchChangeID, LimitOpts: LimitOpts{Limit: i}})
				if err != nil {
					t.Fatal(err)
				}

				{
					have, want := next, int64(0)
					if i < len(reverseBulkOperations) {
						want = reverseBulkOperations[i].DBID
					}

					if have != want {
						t.Fatalf("limit: %v: have next %v, want %v", i, have, want)
					}
				}

				{
					have, want := cs, reverseBulkOperations[:i]
					if len(have) != len(want) {
						t.Fatalf("listed %d bulkOperations, want: %d", len(have), len(want))
					}

					if diff := cmp.Diff(have, want); diff != "" {
						t.Fatal(diff)
					}
				}
			}
		})

		t.Run("WithLimitAndCursor", func(t *testing.T) {
			var cursor int64
			for i := 1; i <= len(reverseBulkOperations); i++ {
				opts := ListBulkOperationsOpts{BatchChangeID: batchChangeID, Cursor: cursor, LimitOpts: LimitOpts{Limit: 1}}
				have, next, err := s.ListBulkOperations(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}

				want := reverseBulkOperations[i-1 : i]
				if diff := cmp.Diff(have, want); diff != "" {
					t.Fatalf("opts: %+v, diff: %s", opts, diff)
				}

				cursor = next
			}
		})
	})

	t.Run("ListBulkOperationErrors", func(t *testing.T) {
		for i, job := range jobs {
			errors, err := s.ListBulkOperationErrors(ctx, ListBulkOperationErrorsOpts{
				BulkOperationID: job.BulkGroup,
			})
			if err != nil {
				t.Fatal(err)
			}
			if i != 0 {
				if have, want := len(errors), 0; have != want {
					t.Fatalf("invalid amount of errors returned, want=%d have=%d", want, have)
				}
				continue
			}
			have := errors
			want := []*btypes.BulkOperationError{
				{
					ChangesetID: changeset.ID,
					Error:       failureMessage,
				},
			}
			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})
}
