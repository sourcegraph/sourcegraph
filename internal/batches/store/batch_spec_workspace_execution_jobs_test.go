package store

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/api"
	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
)

func testStoreBatchSpecWorkspaceExecutionJobs(t *testing.T, ctx context.Context, s *Store, clock bt.Clock) {
	jobs := make([]*btypes.BatchSpecWorkspaceExecutionJob, 0, 3)
	for i := range cap(jobs) {
		job := &btypes.BatchSpecWorkspaceExecutionJob{
			BatchSpecWorkspaceID: int64(i + 456),
			UserID:               int32(i + 1),
		}

		jobs = append(jobs, job)
	}

	t.Run("Create", func(t *testing.T) {
		for idx, job := range jobs {
			if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
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

			// Always one, since every job is in a separate user queue (see l.23).
			job.PlaceInUserQueue = 1
			job.PlaceInGlobalQueue = int64(idx + 1)
		}
	})

	t.Run("Get", func(t *testing.T) {
		t.Run("GetByID", func(t *testing.T) {
			for i, job := range jobs {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecWorkspaceExecutionJob(ctx, GetBatchSpecWorkspaceExecutionJobOpts{ID: job.ID})
					if err != nil {
						t.Fatal(err)
					}

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("GetByBatchSpecWorkspaceID", func(t *testing.T) {
			for i, job := range jobs {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecWorkspaceExecutionJob(ctx, GetBatchSpecWorkspaceExecutionJobOpts{BatchSpecWorkspaceID: job.BatchSpecWorkspaceID})
					if err != nil {
						t.Fatal(err)
					}

					if diff := cmp.Diff(have, job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("GetWithoutRank", func(t *testing.T) {
			for i, job := range jobs {
				// Copy job so we can modify it
				job := *job
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					have, err := s.GetBatchSpecWorkspaceExecutionJob(ctx, GetBatchSpecWorkspaceExecutionJobOpts{ID: job.ID, ExcludeRank: true})
					if err != nil {
						t.Fatal(err)
					}

					job.PlaceInGlobalQueue = 0
					job.PlaceInUserQueue = 0

					if diff := cmp.Diff(have, &job); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})

		t.Run("NoResults", func(t *testing.T) {
			opts := GetBatchSpecWorkspaceExecutionJobOpts{ID: 0xdeadbeef}

			_, have := s.GetBatchSpecWorkspaceExecutionJob(ctx, opts)
			want := ErrNoResults

			if have != want {
				t.Fatalf("have err %v, want %v", have, want)
			}
		})
	})

	t.Run("List", func(t *testing.T) {
		for i, job := range jobs {
			job.WorkerHostname = fmt.Sprintf("worker-hostname-%d", i)
			switch i {
			case 0:
				job.State = btypes.BatchSpecWorkspaceExecutionJobStateQueued
				job.Cancel = true
				job.PlaceInGlobalQueue = 1
				job.PlaceInUserQueue = 1
			case 1:
				job.State = btypes.BatchSpecWorkspaceExecutionJobStateProcessing
				job.PlaceInUserQueue = 0
				job.PlaceInGlobalQueue = 0
			case 2:
				job.State = btypes.BatchSpecWorkspaceExecutionJobStateFailed
				job.PlaceInUserQueue = 0
				job.PlaceInGlobalQueue = 0
			}

			bt.UpdateJobState(t, ctx, s, job)
		}

		t.Run("All", func(t *testing.T) {
			have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{})
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(have, jobs); diff != "" {
				t.Fatalf("invalid jobs returned: %s", diff)
			}
		})

		t.Run("WorkerHostname", func(t *testing.T) {
			for _, job := range jobs {
				have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
					WorkerHostname: job.WorkerHostname,
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspaceExecutionJob{job}); diff != "" {
					t.Fatalf("invalid batch spec workspace jobs returned: %s", diff)
				}
			}
		})

		t.Run("State", func(t *testing.T) {
			for _, job := range jobs {
				have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
					State: job.State,
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspaceExecutionJob{job}); diff != "" {
					t.Fatalf("invalid batch spec workspace jobs returned: %s", diff)
				}
			}
		})

		t.Run("IDs", func(t *testing.T) {
			for _, job := range jobs {
				have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
					IDs: []int64{job.ID},
				})
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(have, []*btypes.BatchSpecWorkspaceExecutionJob{job}); diff != "" {
					t.Fatalf("invalid batch spec workspace jobs returned: %s", diff)
				}
			}
		})

		t.Run("WithFailureMessage", func(t *testing.T) {
			message1 := "failure message 1"
			message2 := "failure message 2"
			message3 := "failure message 3"

			jobs[0].State = btypes.BatchSpecWorkspaceExecutionJobStateFailed
			jobs[0].FailureMessage = &message1
			bt.UpdateJobState(t, ctx, s, jobs[0])

			// has a failure message, but it's outdated, because job is processing
			jobs[1].State = btypes.BatchSpecWorkspaceExecutionJobStateProcessing
			jobs[1].FailureMessage = &message2
			bt.UpdateJobState(t, ctx, s, jobs[1])

			jobs[2].State = btypes.BatchSpecWorkspaceExecutionJobStateFailed
			jobs[2].FailureMessage = &message3
			bt.UpdateJobState(t, ctx, s, jobs[2])

			wantIDs := []int64{jobs[0].ID, jobs[2].ID}

			have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				OnlyWithFailureMessage: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(have) != 2 {
				t.Fatalf("wrong number of jobs returned. want=%d, have=%d", 2, len(have))
			}
			haveIDs := []int64{have[0].ID, have[1].ID}

			if diff := cmp.Diff(haveIDs, wantIDs); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("ExcludeRank", func(t *testing.T) {
			ranklessJobs := make([]*btypes.BatchSpecWorkspaceExecutionJob, 0, len(jobs))
			for _, job := range jobs {
				// Copy job so we can modify it
				job := *job
				job.PlaceInGlobalQueue = 0
				job.PlaceInUserQueue = 0
				ranklessJobs = append(ranklessJobs, &job)
			}
			have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{ExcludeRank: true})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(have, ranklessJobs); diff != "" {
				t.Fatal(diff)
			}
		})

		t.Run("BatchSpecID", func(t *testing.T) {
			workspaceIDByBatchSpecID := map[int64]int64{}
			for range 3 {
				batchSpec := &btypes.BatchSpec{UserID: 500, NamespaceUserID: 500}
				if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
					t.Fatal(err)
				}

				ws := &btypes.BatchSpecWorkspace{
					BatchSpecID: batchSpec.ID,
				}
				if err := s.CreateBatchSpecWorkspace(ctx, ws); err != nil {
					t.Fatal(err)
				}

				if err := s.CreateBatchSpecWorkspaceExecutionJobs(ctx, ws.BatchSpecID); err != nil {
					t.Fatal(err)
				}
				workspaceIDByBatchSpecID[batchSpec.ID] = ws.ID
			}

			for batchSpecID, workspaceID := range workspaceIDByBatchSpecID {
				have, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
					BatchSpecID: batchSpecID,
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(have) != 1 {
					t.Fatalf("wrong number of jobs returned. want=%d, have=%d", 1, len(have))
				}

				if have[0].BatchSpecWorkspaceID != workspaceID {
					t.Fatalf("wrong job returned. want=%d, have=%d", workspaceID, have[0].BatchSpecWorkspaceID)
				}
			}
		})
	})

	t.Run("CancelBatchSpecWorkspaceExecutionJobs", func(t *testing.T) {
		t.Run("single job by ID", func(t *testing.T) {
			opts := CancelBatchSpecWorkspaceExecutionJobsOpts{IDs: []int64{jobs[0].ID}}

			t.Run("Queued", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'queued', started_at = NULL, finished_at = NULL WHERE id = %s", jobs[0].ID)); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				record := records[0]
				if have, want := record.State, btypes.BatchSpecWorkspaceExecutionJobStateCanceled; have != want {
					t.Errorf("invalid state: have=%q want=%q", have, want)
				}
				if have, want := record.Cancel, true; have != want {
					t.Errorf("invalid cancel value: have=%t want=%t", have, want)
				}
				if record.FinishedAt.IsZero() {
					t.Error("finished_at not set")
				} else if have, want := record.FinishedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid finished_at: have=%s want=%s", have, want)
				}
				if have, want := record.UpdatedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid updated_at: have=%s want=%s", have, want)
				}
			})

			t.Run("Processing", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'processing', started_at = now(), finished_at = NULL WHERE id = %s", jobs[0].ID)); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				record := records[0]
				if have, want := record.State, btypes.BatchSpecWorkspaceExecutionJobStateProcessing; have != want {
					t.Errorf("invalid state: have=%q want=%q", have, want)
				}
				if have, want := record.Cancel, true; have != want {
					t.Errorf("invalid cancel value: have=%t want=%t", have, want)
				}
				if !record.FinishedAt.IsZero() {
					t.Error("finished_at set")
				}
				if have, want := record.UpdatedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid updated_at: have=%s want=%s", have, want)
				}
			})

			t.Run("already completed", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'completed' WHERE id = %s", jobs[0].ID)); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				if len(records) != 0 {
					t.Fatalf("unexpected records returned: %d", len(records))
				}
			})

			t.Run("still queued", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'queued' WHERE id = %s", jobs[0].ID)); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				if len(records) != 1 {
					t.Fatalf("unexpected records returned: %d", len(records))
				}
			})
		})

		t.Run("multiple jobs by BatchSpecID", func(t *testing.T) {
			spec := &btypes.BatchSpec{UserID: 1234, NamespaceUserID: 4567}
			if err := s.CreateBatchSpec(ctx, spec); err != nil {
				t.Fatal(err)
			}

			var specJobIDs []int64
			for i := range 3 {
				ws := &btypes.BatchSpecWorkspace{BatchSpecID: spec.ID, RepoID: api.RepoID(i)}
				if err := s.CreateBatchSpecWorkspace(ctx, ws); err != nil {
					t.Fatal(err)
				}

				job := &btypes.BatchSpecWorkspaceExecutionJob{BatchSpecWorkspaceID: ws.ID, UserID: spec.UserID}
				if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
					t.Fatal(err)
				}
				specJobIDs = append(specJobIDs, job.ID)
			}

			opts := CancelBatchSpecWorkspaceExecutionJobsOpts{BatchSpecID: spec.ID}

			t.Run("Queued", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'queued', started_at = NULL, finished_at = NULL WHERE id = ANY(%s)", pq.Array(specJobIDs))); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				record := records[0]
				if have, want := record.State, btypes.BatchSpecWorkspaceExecutionJobStateCanceled; have != want {
					t.Errorf("invalid state: have=%q want=%q", have, want)
				}
				if have, want := record.Cancel, true; have != want {
					t.Errorf("invalid cancel value: have=%t want=%t", have, want)
				}
				if record.FinishedAt.IsZero() {
					t.Error("finished_at not set")
				} else if have, want := record.FinishedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid finished_at: have=%s want=%s", have, want)
				}
				if have, want := record.UpdatedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid updated_at: have=%s want=%s", have, want)
				}
			})

			t.Run("Processing", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'processing', started_at = now(), finished_at = NULL WHERE id = ANY(%s)", pq.Array(specJobIDs))); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				record := records[0]
				if have, want := record.State, btypes.BatchSpecWorkspaceExecutionJobStateProcessing; have != want {
					t.Errorf("invalid state: have=%q want=%q", have, want)
				}
				if have, want := record.Cancel, true; have != want {
					t.Errorf("invalid cancel value: have=%t want=%t", have, want)
				}
				if !record.FinishedAt.IsZero() {
					t.Error("finished_at set")
				}
				if have, want := record.UpdatedAt, s.now(); !have.Equal(want) {
					t.Errorf("invalid updated_at: have=%s want=%s", have, want)
				}
			})

			t.Run("Already completed", func(t *testing.T) {
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'completed' WHERE id = ANY(%s)", pq.Array(specJobIDs))); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				if len(records) != 0 {
					t.Fatalf("unexpected records returned: %d", len(records))
				}
			})

			t.Run("subset processing, subset completed", func(t *testing.T) {
				completed := specJobIDs[1:]
				processing := specJobIDs[0:1]
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'processing', started_at = now(), finished_at = NULL WHERE id = ANY(%s)", pq.Array(processing))); err != nil {
					t.Fatal(err)
				}
				if err := s.Exec(ctx, sqlf.Sprintf("UPDATE batch_spec_workspace_execution_jobs SET state = 'completed' WHERE id = ANY(%s)", pq.Array(completed))); err != nil {
					t.Fatal(err)
				}
				records, err := s.CancelBatchSpecWorkspaceExecutionJobs(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				if have, want := len(records), len(processing); have != want {
					t.Fatalf("wrong number of canceled records. have=%d, want=%d", have, want)
				}
			})
		})
	})

	t.Run("CreateBatchSpecWorkspaceExecutionJobs", func(t *testing.T) {
		cacheEntry := &btypes.BatchSpecExecutionCacheEntry{Key: "one", Value: "two"}
		if err := s.CreateBatchSpecExecutionCacheEntry(ctx, cacheEntry); err != nil {
			t.Fatal(err)
		}

		createWorkspaces := func(t *testing.T, batchSpec *btypes.BatchSpec, workspaces ...*btypes.BatchSpecWorkspace) {
			t.Helper()

			for i, workspace := range workspaces {
				workspace.BatchSpecID = batchSpec.ID
				workspace.RepoID = 1
				workspace.Branch = fmt.Sprintf("refs/heads/main-%d", i)
				workspace.Commit = fmt.Sprintf("commit-%d", i)
			}

			if err := s.CreateBatchSpecWorkspace(ctx, workspaces...); err != nil {
				t.Fatal(err)
			}
		}

		createJobsAndAssert := func(t *testing.T, batchSpec *btypes.BatchSpec, wantJobsForWorkspaces []int64) {
			t.Helper()

			err := s.CreateBatchSpecWorkspaceExecutionJobs(ctx, batchSpec.ID)
			if err != nil {
				t.Fatal(err)
			}

			jobs, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				BatchSpecWorkspaceIDs: wantJobsForWorkspaces,
			})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := len(jobs), len(wantJobsForWorkspaces); have != want {
				t.Fatalf("wrong number of execution jobs created. want=%d, have=%d", want, have)
			}
		}

		createBatchSpec := func(t *testing.T, batchSpec *btypes.BatchSpec) {
			t.Helper()
			batchSpec.UserID = 1
			batchSpec.NamespaceUserID = 1
			if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
				t.Fatal(err)
			}
		}

		t.Run("success", func(t *testing.T) {
			// TODO: Test we skip jobs where nothing needs to be executed.

			normalWorkspace := &btypes.BatchSpecWorkspace{}
			ignoredWorkspace := &btypes.BatchSpecWorkspace{Ignored: true}
			unsupportedWorkspace := &btypes.BatchSpecWorkspace{Unsupported: true}
			cachedResultWorkspace := &btypes.BatchSpecWorkspace{CachedResultFound: true}

			batchSpec := &btypes.BatchSpec{}

			createBatchSpec(t, batchSpec)
			createWorkspaces(t, batchSpec, normalWorkspace, ignoredWorkspace, unsupportedWorkspace, cachedResultWorkspace)
			createJobsAndAssert(t, batchSpec, []int64{normalWorkspace.ID})
		})

		t.Run("allowIgnored", func(t *testing.T) {
			normalWorkspace := &btypes.BatchSpecWorkspace{}
			ignoredWorkspace := &btypes.BatchSpecWorkspace{Ignored: true}

			batchSpec := &btypes.BatchSpec{AllowIgnored: true}

			createBatchSpec(t, batchSpec)
			createWorkspaces(t, batchSpec, normalWorkspace, ignoredWorkspace)
			createJobsAndAssert(t, batchSpec, []int64{normalWorkspace.ID, ignoredWorkspace.ID})
		})

		t.Run("allowUnsupported", func(t *testing.T) {
			normalWorkspace := &btypes.BatchSpecWorkspace{}
			unsupportedWorkspace := &btypes.BatchSpecWorkspace{Unsupported: true}

			batchSpec := &btypes.BatchSpec{AllowUnsupported: true}

			createBatchSpec(t, batchSpec)
			createWorkspaces(t, batchSpec, normalWorkspace, unsupportedWorkspace)
			createJobsAndAssert(t, batchSpec, []int64{normalWorkspace.ID, unsupportedWorkspace.ID})
		})

		t.Run("allowUnsupported and allowIgnored", func(t *testing.T) {
			normalWorkspace := &btypes.BatchSpecWorkspace{}
			ignoredWorkspace := &btypes.BatchSpecWorkspace{Ignored: true}
			unsupportedWorkspace := &btypes.BatchSpecWorkspace{Unsupported: true}

			batchSpec := &btypes.BatchSpec{AllowUnsupported: true, AllowIgnored: true}

			createBatchSpec(t, batchSpec)
			createWorkspaces(t, batchSpec, normalWorkspace, ignoredWorkspace, unsupportedWorkspace)
			createJobsAndAssert(t, batchSpec, []int64{normalWorkspace.ID, ignoredWorkspace.ID, unsupportedWorkspace.ID})
		})
	})

	t.Run("CreateBatchSpecWorkspaceExecutionJobsForWorkspaces", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			workspaces := createWorkspaces(t, ctx, s)
			ids := workspacesIDs(t, workspaces)

			err := s.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, ids)
			if err != nil {
				t.Fatal(err)
			}

			jobs, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				BatchSpecWorkspaceIDs: ids,
			})
			if err != nil {
				t.Fatal(err)
			}

			if have, want := len(jobs), len(workspaces); have != want {
				t.Fatalf("wrong number of jobs created. want=%d, have=%d", want, have)
			}
		})
	})

	t.Run("DeleteBatchSpecWorkspaceExecutionJobs", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			workspaces := createWorkspaces(t, ctx, s)
			ids := workspacesIDs(t, workspaces)

			err := s.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, ids)
			if err != nil {
				t.Fatal(err)
			}

			jobs, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				BatchSpecWorkspaceIDs: ids,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), len(workspaces); have != want {
				t.Fatalf("wrong number of jobs created. want=%d, have=%d", want, have)
			}

			jobIDs := make([]int64, len(jobs))
			for i, j := range jobs {
				jobIDs[i] = j.ID
			}

			if err := s.DeleteBatchSpecWorkspaceExecutionJobs(ctx, DeleteBatchSpecWorkspaceExecutionJobsOpts{IDs: jobIDs}); err != nil {
				t.Fatal(err)
			}

			jobs, err = s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				IDs: jobIDs,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), 0; have != want {
				t.Fatalf("wrong number of jobs still exists. want=%d, have=%d", want, have)
			}
		})

		t.Run("with wrong IDs", func(t *testing.T) {
			workspaces := createWorkspaces(t, ctx, s)
			ids := workspacesIDs(t, workspaces)

			err := s.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, ids)
			if err != nil {
				t.Fatal(err)
			}

			jobs, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				BatchSpecWorkspaceIDs: ids,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), len(workspaces); have != want {
				t.Fatalf("wrong number of jobs created. want=%d, have=%d", want, have)
			}

			jobIDs := make([]int64, len(jobs))
			for i, j := range jobs {
				jobIDs[i] = j.ID
			}

			jobIDs = append(jobIDs, 999, 888, 777)

			err = s.DeleteBatchSpecWorkspaceExecutionJobs(ctx, DeleteBatchSpecWorkspaceExecutionJobsOpts{IDs: jobIDs})
			if err == nil {
				t.Fatal("error is nil")
			}

			want := fmt.Sprintf("wrong number of jobs deleted: %d instead of %d", len(workspaces), len(workspaces)+3)
			if err.Error() != want {
				t.Fatalf("wrong error message. want=%q, have=%q", want, err.Error())
			}

			jobs, err = s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				IDs: jobIDs,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), 0; have != want {
				t.Fatalf("wrong number of jobs still exists. want=%d, have=%d", want, have)
			}
		})

		t.Run("by workspace IDs", func(t *testing.T) {
			workspaces := createWorkspaces(t, ctx, s)
			ids := workspacesIDs(t, workspaces)

			err := s.CreateBatchSpecWorkspaceExecutionJobsForWorkspaces(ctx, ids)
			if err != nil {
				t.Fatal(err)
			}

			jobs, err := s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				BatchSpecWorkspaceIDs: ids,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), len(workspaces); have != want {
				t.Fatalf("wrong number of jobs created. want=%d, have=%d", want, have)
			}

			jobIDs := make([]int64, len(jobs))
			for i, j := range jobs {
				jobIDs[i] = j.ID
			}

			if err := s.DeleteBatchSpecWorkspaceExecutionJobs(ctx, DeleteBatchSpecWorkspaceExecutionJobsOpts{WorkspaceIDs: ids}); err != nil {
				t.Fatal(err)
			}

			jobs, err = s.ListBatchSpecWorkspaceExecutionJobs(ctx, ListBatchSpecWorkspaceExecutionJobsOpts{
				IDs: jobIDs,
			})
			if err != nil {
				t.Fatal(err)
			}
			if have, want := len(jobs), 0; have != want {
				t.Fatalf("wrong number of jobs still exists. want=%d, have=%d", want, have)
			}
		})

		t.Run("invalid option", func(t *testing.T) {
			err := s.DeleteBatchSpecWorkspaceExecutionJobs(ctx, DeleteBatchSpecWorkspaceExecutionJobsOpts{})
			assert.Equal(t, "invalid options: would delete all jobs", err.Error())
		})

		t.Run("too many options", func(t *testing.T) {
			err := s.DeleteBatchSpecWorkspaceExecutionJobs(ctx, DeleteBatchSpecWorkspaceExecutionJobsOpts{
				IDs:          []int64{1, 2},
				WorkspaceIDs: []int64{3, 4},
			})
			assert.Equal(t, "invalid options: multiple options not supported", err.Error())
		})
	})
}

func createWorkspaces(t *testing.T, ctx context.Context, s *Store) []*btypes.BatchSpecWorkspace {
	t.Helper()

	batchSpec := &btypes.BatchSpec{NamespaceUserID: 1, UserID: 1}
	if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	workspaces := []*btypes.BatchSpecWorkspace{
		{},
		{Ignored: true},
		{Unsupported: true},
	}
	for i, workspace := range workspaces {
		workspace.BatchSpecID = batchSpec.ID
		workspace.RepoID = 1
		workspace.Branch = fmt.Sprintf("refs/heads/main-%d", i)
		workspace.Commit = fmt.Sprintf("commit-%d", i)
	}

	if err := s.CreateBatchSpecWorkspace(ctx, workspaces...); err != nil {
		t.Fatal(err)
	}

	return workspaces
}

func workspacesIDs(t *testing.T, workspaces []*btypes.BatchSpecWorkspace) []int64 {
	t.Helper()
	ids := make([]int64, len(workspaces))
	for i, w := range workspaces {
		ids[i] = w.ID
	}
	return ids
}
