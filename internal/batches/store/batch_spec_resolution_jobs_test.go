package store

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log/logtest"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func testStoreBatchSpecResolutionJobs(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	jobs := make([]*btypes.BatchSpecResolutionJob, 0, 2)
	for i := range cap(jobs) {
		job := &btypes.BatchSpecResolutionJob{
			BatchSpecID: int64(i + 567),
			InitiatorID: int32(i + 123),
		}

		switch i {
		case 0:
			job.State = btypes.BatchSpecResolutionJobStateQueued
		case 1:
			job.State = btypes.BatchSpecResolutionJobStateProcessing
		case 2:
			job.State = btypes.BatchSpecResolutionJobStateFailed
		}

		jobs = append(jobs, job)
	}

	t.Run("Create", func(t *testing.T) {
		for _, job := range jobs {
			if err := s.CreateBatchSpecResolutionJob(ctx, job); err != nil {
				t.Fatal(err)
			}

			have := job
			if have.ID == 0 {
				t.Fatal("ID should not be zero")
			}

			want := have
			want.CreatedAt = clock.Now()
			want.UpdatedAt = clock.Now()

			if diff := cmp.Diff(have, want); diff != "" {
				t.Fatal(diff)
			}
		}
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("GetByID", func(t *testing.T) {
			for i, job := range jobs {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecResolutionJob(ctx, GetBatchSpecResolutionJobOpts{ID: job.ID})
					if err != nil {
						t.Fatal(err)
					}

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("GetByBatchSpecID", func(t *testing.T) {
			for i, job := range jobs {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecResolutionJob(ctx, GetBatchSpecResolutionJobOpts{BatchSpecID: job.BatchSpecID})
					if err != nil {
						t.Fatal(err)
					}

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := GetBatchSpecResolutionJobOpts{ID: 0xdeadbeef}

			_, have := s.GetBatchSpecResolutionJob(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		for i, job := range jobs {
			job.WorkerHostname = fmt.Sprintf("worker-hostname-%d", i)
			if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_resolution_jobs SET worker_hostname = %s, state = %s WHERE id = %s", job.WorkerHostname, job.State, job.ID)); err != nil {
				t.Fatal(err)
			}
		}

		t.Run("All", func(t *testing.T) {
			have, err := s.ListBatchSpecResolutionJobs(ctx, ListBatchSpecResolutionJobsOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(have, jobs); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("WorkerHostname", func(t *testing.T) {
			for _, job := range jobs {
				have, err := s.ListBatchSpecResolutionJobs(ctx, ListBatchSpecResolutionJobsOpts{
					WorkerHostname: job.WorkerHostname,
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(have, []*btypes.BatchSpecResolutionJob{job}); diff != "" {
					t.Fatalf("invalid batch spec workspace jobs returned: %s", diff)
				}
			}
		})

		t.Run("State", func(t *testing.T) {
			for _, job := range jobs {
				have, err := s.ListBatchSpecResolutionJobs(ctx, ListBatchSpecResolutionJobsOpts{
					State: job.State,
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(have, []*btypes.BatchSpecResolutionJob{job}); diff != "" {
					t.Fatalf("invalid batch spec workspace jobs returned: %s", diff)
				}
			}
		})
	})
}

func TestBatchSpecResolutionJobs_BatchSpecIDUnique(t *testing.T) {
	// This test is a separate test so we can test the database constraints,
	// because in the store tests the constraints are all deferred.
	ctx := context.Background()
	c := &bt.TestClock{Time: timeutil.Now()}
	logger := logtest.Scoped(t)

	db := database.NewDB(logger, dbtest.NewDB(t))
	s := NewWithClock(db, &observation.TestContext, nil, c.Now)

	user := bt.CreateTestUser(t, db, true)

	batchSpec := &btypes.BatchSpec{
		UserID:          user.ID,
		NamespaceUserID: user.ID,
	}
	if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	job1 := &btypes.BatchSpecResolutionJob{
		BatchSpecID: batchSpec.ID,
		InitiatorID: user.ID,
	}
	if err := s.CreateBatchSpecResolutionJob(ctx, job1); err != nil {
		t.Fatal(err)
	}

	job2 := &btypes.BatchSpecResolutionJob{
		BatchSpecID: batchSpec.ID,
		InitiatorID: user.ID,
	}
	err := s.CreateBatchSpecResolutionJob(ctx, job2)
	wantErr := ErrResolutionJobAlreadyExists{BatchSpecID: batchSpec.ID}
	if err != wantErr {
		t.Fatalf("wrong error. want=%s, have=%s", wantErr, err)
	}
}
