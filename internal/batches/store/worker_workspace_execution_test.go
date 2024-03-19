package store

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/log/logtest"

	bt "github.com/sourcegraph/sourcegraph/internal/batches/testing"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	"github.com/sourcegraph/sourcegraph/lib/batches/execution"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestBatchSpecWorkspaceExecutionWorkerStore_MarkComplete(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	user := bt.CreateTestUser(t, db, true)

	repo, _ := bt.CreateTestRepo(t, ctx, db)
	s := New(db, &observation.TestContext, nil)
	workStore := dbworkerstore.New(&observation.TestContext, s.Handle(), batchSpecWorkspaceExecutionWorkerStoreOptions)

	// Setup all the associations
	batchSpec := &btypes.BatchSpec{UserID: user.ID, NamespaceUserID: user.ID, RawSpec: "horse", Spec: &batcheslib.BatchSpec{
		ChangesetTemplate: &batcheslib.ChangesetTemplate{},
	}}
	if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	// See the `output` var below
	cacheEntryKeys := []string{
		"JkC7Q0OOCZZ3Acv79QfwSA-step-0",
		"0ydsSXJ77syIPdwNrsGlzQ-step-1",
		"utgLpuQ3njDtLe3eztArAQ-step-2",
		"RoG8xSgpganc5BJ0_D3XGA-step-3",
		"Nsw12JxoLSHN4ta6D3G7FQ-step-4",
	}

	// Log entries with cache entries that'll be used to build the changeset specs.
	output := `
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"JkC7Q0OOCZZ3Acv79QfwSA-step-0","value":{"stepIndex":0,"diff":"ZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCg==","outputs":{},"previousStepResult":{"Files":null,"Stdout":null,"Stderr":null}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"0ydsSXJ77syIPdwNrsGlzQ-step-1","value":{"stepIndex":1,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLjVjMmI3MmQgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDQgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCmRpZmYgLS1naXQgUkVBRE1FLnR4dCBSRUFETUUudHh0Cm5ldyBmaWxlIG1vZGUgMTAwNjQ0CmluZGV4IDAwMDAwMDAuLjg4OGUxZWMKLS0tIC9kZXYvbnVsbAorKysgUkVBRE1FLnR4dApAQCAtMCwwICsxIEBACit0aGlzIGlzIHN0ZXAgMQo=","outputs":{},"previousStepResult":{"Files":{"modified":null,"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"utgLpuQ3njDtLe3eztArAQ-step-2","value":{"stepIndex":2,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmNkMmNjYmYgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDUgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwpkaWZmIC0tZ2l0IFJFQURNRS50eHQgUkVBRE1FLnR4dApuZXcgZmlsZSBtb2RlIDEwMDY0NAppbmRleCAwMDAwMDAwLi44ODhlMWVjCi0tLSAvZGV2L251bGwKKysrIFJFQURNRS50eHQKQEAgLTAsMCArMSBAQAordGhpcyBpcyBzdGVwIDEK","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"RoG8xSgpganc5BJ0_D3XGA-step-3","value":{"stepIndex":3,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmQ2NzgyZDMgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDcgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwordGhpcyBpcyBzdGVwIDQKK3ByZXZpb3VzX3N0ZXAubW9kaWZpZWRfZmlsZXM9W1JFQURNRS5tZF0KZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCg==","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"Nsw12JxoLSHN4ta6D3G7FQ-step-4","value":{"stepIndex":4,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmQ2NzgyZDMgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDcgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwordGhpcyBpcyBzdGVwIDQKK3ByZXZpb3VzX3N0ZXAubW9kaWZpZWRfZmlsZXM9W1JFQURNRS5tZF0KZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCmRpZmYgLS1naXQgbXktb3V0cHV0LnR4dCBteS1vdXRwdXQudHh0Cm5ldyBmaWxlIG1vZGUgMTAwNjQ0CmluZGV4IDAwMDAwMDAuLjI1N2FlOGUKLS0tIC9kZXYvbnVsbAorKysgbXktb3V0cHV0LnR4dApAQCAtMCwwICsxIEBACit0aGlzIGlzIHN0ZXAgNQo=","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}`

	entry := executor.ExecutionLogEntry{
		Key:        "step.src.batch-exec",
		Command:    []string{"src", "batch", "preview", "-f", "spec.yml", "-text-only"},
		StartTime:  time.Now().Add(-5 * time.Second),
		Out:        output,
		DurationMs: intptr(200),
	}

	executionStore := &batchSpecWorkspaceExecutionWorkerStore{
		Store:          workStore,
		observationCtx: &observation.TestContext,
		logger:         logtest.Scoped(t),
	}
	opts := dbworkerstore.MarkFinalOptions{WorkerHostname: "worker-1"}

	setProcessing := func(t *testing.T, job *btypes.BatchSpecWorkspaceExecutionJob) {
		t.Helper()
		job.State = btypes.BatchSpecWorkspaceExecutionJobStateProcessing
		job.WorkerHostname = opts.WorkerHostname
		bt.UpdateJobState(t, ctx, s, job)
	}

	assertJobState := func(t *testing.T, job *btypes.BatchSpecWorkspaceExecutionJob, want btypes.BatchSpecWorkspaceExecutionJobState) {
		t.Helper()
		reloadedJob, err := s.GetBatchSpecWorkspaceExecutionJob(ctx, GetBatchSpecWorkspaceExecutionJobOpts{ID: job.ID})
		if err != nil {
			t.Fatalf("failed to reload job: %s", err)
		}

		if have := reloadedJob.State; have != want {
			t.Fatalf("wrong job state: want=%s, have=%s", want, have)
		}
	}

	assertWorkspaceChangesets := func(t *testing.T, job *btypes.BatchSpecWorkspaceExecutionJob, want []int64) {
		t.Helper()
		w, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{ID: job.BatchSpecWorkspaceID})
		if err != nil {
			t.Fatalf("failed to load workspace: %s", err)
		}

		if diff := cmp.Diff(w.ChangesetSpecIDs, want); diff != "" {
			t.Fatalf("wrong job changeset spec IDs: diff=%s", diff)
		}
	}

	assertNoChangesetSpecsCreated := func(t *testing.T) {
		t.Helper()
		specs, _, err := s.ListChangesetSpecs(ctx, ListChangesetSpecsOpts{BatchSpecID: batchSpec.ID})
		if err != nil {
			t.Fatalf("failed to load changeset specs: %s", err)
		}
		if have, want := len(specs), 0; have != want {
			t.Fatalf("invalid number of changeset specs created: have=%d want=%d", have, want)
		}
	}

	setupEntities := func(t *testing.T) (*btypes.BatchSpecWorkspaceExecutionJob, *btypes.BatchSpecWorkspace) {
		if err := s.DeleteChangesetSpecs(ctx, DeleteChangesetSpecsOpts{BatchSpecID: batchSpec.ID}); err != nil {
			t.Fatal(err)
		}
		workspace := &btypes.BatchSpecWorkspace{BatchSpecID: batchSpec.ID, RepoID: repo.ID}
		if err := s.CreateBatchSpecWorkspace(ctx, workspace); err != nil {
			t.Fatal(err)
		}

		job := &btypes.BatchSpecWorkspaceExecutionJob{BatchSpecWorkspaceID: workspace.ID, UserID: 1}
		if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
			t.Fatal(err)
		}

		_, err := workStore.AddExecutionLogEntry(ctx, int(job.ID), entry, dbworkerstore.ExecutionLogEntryOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return job, workspace
	}

	t.Run("success", func(t *testing.T) {
		job, workspace := setupEntities(t)
		setProcessing(t, job)

		ok, err := executionStore.MarkComplete(context.Background(), int(job.ID), opts)
		if !ok || err != nil {
			t.Fatalf("MarkComplete failed. ok=%t, err=%s", ok, err)
		}

		// Now reload the involved entities and make sure they've been updated correctly
		assertJobState(t, job, btypes.BatchSpecWorkspaceExecutionJobStateCompleted)

		reloadedWorkspace, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{ID: workspace.ID})
		if err != nil {
			t.Fatalf("failed to reload workspace: %s", err)
		}

		specs, _, err := s.ListChangesetSpecs(ctx, ListChangesetSpecsOpts{BatchSpecID: batchSpec.ID})
		if err != nil {
			t.Fatalf("failed to load changeset specs: %s", err)
		}
		if have, want := len(specs), 1; have != want {
			t.Fatalf("invalid number of changeset specs created: have=%d want=%d", have, want)
		}
		changesetSpecIDs := make([]int64, 0, len(specs))
		for _, reloadedSpec := range specs {
			changesetSpecIDs = append(changesetSpecIDs, reloadedSpec.ID)
			if reloadedSpec.BatchSpecID != batchSpec.ID {
				t.Fatalf("reloaded changeset spec does not have correct batch spec id: %d", reloadedSpec.BatchSpecID)
			}
		}

		if diff := cmp.Diff(changesetSpecIDs, reloadedWorkspace.ChangesetSpecIDs); diff != "" {
			t.Fatalf("reloaded workspace has wrong changeset spec IDs: %s", diff)
		}

		assertWorkspaceChangesets(t, job, changesetSpecIDs)

		for _, wantKey := range cacheEntryKeys {
			entries, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
				UserID: user.ID,
				Keys:   []string{wantKey},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatal("cache entry not found")
			}
			entry := entries[0]

			var cachedExecutionResult *execution.AfterStepResult
			if err := json.Unmarshal([]byte(entry.Value), &cachedExecutionResult); err != nil {
				t.Fatal(err)
			}
			if len(cachedExecutionResult.Diff) == 0 {
				t.Fatalf("wrong diff extracted")
			}
		}
	})

	t.Run("worker hostname mismatch", func(t *testing.T) {
		job, _ := setupEntities(t)
		setProcessing(t, job)

		opts := opts
		opts.WorkerHostname = "DOESNT-MATCH"

		ok, err := executionStore.MarkComplete(context.Background(), int(job.ID), opts)
		if ok || err != nil {
			t.Fatalf("MarkComplete returned wrong result. ok=%t, err=%s", ok, err)
		}

		assertJobState(t, job, btypes.BatchSpecWorkspaceExecutionJobStateProcessing)

		assertWorkspaceChangesets(t, job, []int64{})

		assertNoChangesetSpecsCreated(t)
	})
}

func TestBatchSpecWorkspaceExecutionWorkerStore_MarkFailed(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))
	user := bt.CreateTestUser(t, db, true)

	repo, _ := bt.CreateTestRepo(t, ctx, db)
	s := New(db, &observation.TestContext, nil)
	workStore := dbworkerstore.New(&observation.TestContext, s.Handle(), batchSpecWorkspaceExecutionWorkerStoreOptions)

	// Setup all the associations
	batchSpec := &btypes.BatchSpec{UserID: user.ID, NamespaceUserID: user.ID, RawSpec: "horse", Spec: &batcheslib.BatchSpec{
		ChangesetTemplate: &batcheslib.ChangesetTemplate{},
	}}
	if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	workspace := &btypes.BatchSpecWorkspace{BatchSpecID: batchSpec.ID, RepoID: repo.ID}
	if err := s.CreateBatchSpecWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}

	job := &btypes.BatchSpecWorkspaceExecutionJob{BatchSpecWorkspaceID: workspace.ID, UserID: user.ID}
	if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
		t.Fatal(err)
	}

	// See the `output` var below
	cacheEntryKeys := []string{
		"JkC7Q0OOCZZ3Acv79QfwSA-step-0",
		"0ydsSXJ77syIPdwNrsGlzQ-step-1",
		"utgLpuQ3njDtLe3eztArAQ-step-2",
		"RoG8xSgpganc5BJ0_D3XGA-step-3",
		"Nsw12JxoLSHN4ta6D3G7FQ-step-4",
	}

	// Log entries with cache entries that'll be used to build the changeset specs.
	output := `
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"JkC7Q0OOCZZ3Acv79QfwSA-step-0","value":{"stepIndex":0,"diff":"ZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCg==","outputs":{},"previousStepResult":{"Files":null,"Stdout":null,"Stderr":null}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"0ydsSXJ77syIPdwNrsGlzQ-step-1","value":{"stepIndex":1,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLjVjMmI3MmQgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDQgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCmRpZmYgLS1naXQgUkVBRE1FLnR4dCBSRUFETUUudHh0Cm5ldyBmaWxlIG1vZGUgMTAwNjQ0CmluZGV4IDAwMDAwMDAuLjg4OGUxZWMKLS0tIC9kZXYvbnVsbAorKysgUkVBRE1FLnR4dApAQCAtMCwwICsxIEBACit0aGlzIGlzIHN0ZXAgMQo=","outputs":{},"previousStepResult":{"Files":{"modified":null,"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"utgLpuQ3njDtLe3eztArAQ-step-2","value":{"stepIndex":2,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmNkMmNjYmYgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDUgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwpkaWZmIC0tZ2l0IFJFQURNRS50eHQgUkVBRE1FLnR4dApuZXcgZmlsZSBtb2RlIDEwMDY0NAppbmRleCAwMDAwMDAwLi44ODhlMWVjCi0tLSAvZGV2L251bGwKKysrIFJFQURNRS50eHQKQEAgLTAsMCArMSBAQAordGhpcyBpcyBzdGVwIDEK","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"RoG8xSgpganc5BJ0_D3XGA-step-3","value":{"stepIndex":3,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmQ2NzgyZDMgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDcgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwordGhpcyBpcyBzdGVwIDQKK3ByZXZpb3VzX3N0ZXAubW9kaWZpZWRfZmlsZXM9W1JFQURNRS5tZF0KZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCg==","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"Nsw12JxoLSHN4ta6D3G7FQ-step-4","value":{"stepIndex":4,"diff":"ZGlmZiAtLWdpdCBSRUFETUUubWQgUkVBRE1FLm1kCmluZGV4IDE5MTQ0OTEuLmQ2NzgyZDMgMTAwNjQ0Ci0tLSBSRUFETUUubWQKKysrIFJFQURNRS5tZApAQCAtMyw0ICszLDcgQEAgVGhpcyByZXBvc2l0b3J5IGlzIHVzZWQgdG8gdGVzdCBvcGVuaW5nIGFuZCBjbG9zaW5nIHB1bGwgcmVxdWVzdCB3aXRoIEF1dG9tYXRpb24KIAogKGMpIENvcHlyaWdodCBTb3VyY2VncmFwaCAyMDEzLTIwMjAuCiAoYykgQ29weXJpZ2h0IFNvdXJjZWdyYXBoIDIwMTMtMjAyMC4KLShjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLgpcIE5vIG5ld2xpbmUgYXQgZW5kIG9mIGZpbGUKKyhjKSBDb3B5cmlnaHQgU291cmNlZ3JhcGggMjAxMy0yMDIwLnRoaXMgaXMgc3RlcCAyCit0aGlzIGlzIHN0ZXAgMwordGhpcyBpcyBzdGVwIDQKK3ByZXZpb3VzX3N0ZXAubW9kaWZpZWRfZmlsZXM9W1JFQURNRS5tZF0KZGlmZiAtLWdpdCBSRUFETUUudHh0IFJFQURNRS50eHQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uODg4ZTFlYwotLS0gL2Rldi9udWxsCisrKyBSRUFETUUudHh0CkBAIC0wLDAgKzEgQEAKK3RoaXMgaXMgc3RlcCAxCmRpZmYgLS1naXQgbXktb3V0cHV0LnR4dCBteS1vdXRwdXQudHh0Cm5ldyBmaWxlIG1vZGUgMTAwNjQ0CmluZGV4IDAwMDAwMDAuLjI1N2FlOGUKLS0tIC9kZXYvbnVsbAorKysgbXktb3V0cHV0LnR4dApAQCAtMCwwICsxIEBACit0aGlzIGlzIHN0ZXAgNQo=","outputs":{"myOutput":"my-output.txt"},"previousStepResult":{"Files":{"modified":["README.md"],"added":["README.txt"],"deleted":null,"renamed":null},"Stdout":{},"Stderr":{}}}}}`

	entry := executor.ExecutionLogEntry{
		Key:        "step.src.batch-exec",
		Command:    []string{"src", "batch", "preview", "-f", "spec.yml", "-text-only"},
		StartTime:  time.Now().Add(-5 * time.Second),
		Out:        output,
		DurationMs: intptr(200),
	}

	_, err := workStore.AddExecutionLogEntry(ctx, int(job.ID), entry, dbworkerstore.ExecutionLogEntryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	executionStore := &batchSpecWorkspaceExecutionWorkerStore{
		Store:          workStore,
		observationCtx: &observation.TestContext,
		logger:         logtest.Scoped(t),
	}
	opts := dbworkerstore.MarkFinalOptions{WorkerHostname: "worker-1"}
	errMsg := "this job was no good"

	setProcessing := func(t *testing.T) {
		t.Helper()
		job.State = btypes.BatchSpecWorkspaceExecutionJobStateProcessing
		job.WorkerHostname = opts.WorkerHostname
		bt.UpdateJobState(t, ctx, s, job)
	}

	assertJobState := func(t *testing.T, want btypes.BatchSpecWorkspaceExecutionJobState) {
		t.Helper()
		reloadedJob, err := s.GetBatchSpecWorkspaceExecutionJob(ctx, GetBatchSpecWorkspaceExecutionJobOpts{ID: job.ID})
		if err != nil {
			t.Fatalf("failed to reload job: %s", err)
		}

		if have := reloadedJob.State; have != want {
			t.Fatalf("wrong job state: want=%s, have=%s", want, have)
		}
	}

	t.Run("success", func(t *testing.T) {
		setProcessing(t)

		ok, err := executionStore.MarkFailed(context.Background(), int(job.ID), errMsg, opts)
		if !ok || err != nil {
			t.Fatalf("MarkFailed failed. ok=%t, err=%s", ok, err)
		}

		// Now reload the involved entities and make sure they've been updated correctly
		assertJobState(t, btypes.BatchSpecWorkspaceExecutionJobStateFailed)

		reloadedWorkspace, err := s.GetBatchSpecWorkspace(ctx, GetBatchSpecWorkspaceOpts{ID: workspace.ID})
		if err != nil {
			t.Fatalf("failed to reload workspace: %s", err)
		}

		// Assert no changeset specs.
		if diff := cmp.Diff([]int64{}, reloadedWorkspace.ChangesetSpecIDs); diff != "" {
			t.Fatalf("reloaded workspace has wrong changeset spec IDs: %s", diff)
		}

		for _, wantKey := range cacheEntryKeys {
			entries, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
				UserID: user.ID,
				Keys:   []string{wantKey},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatal("cache entry not found")
			}
			entry := entries[0]

			var cachedExecutionResult *execution.AfterStepResult
			if err := json.Unmarshal([]byte(entry.Value), &cachedExecutionResult); err != nil {
				t.Fatal(err)
			}
			if len(cachedExecutionResult.Diff) == 0 {
				t.Fatalf("wrong diff extracted")
			}
		}
	})

	t.Run("no token set", func(t *testing.T) {
		setProcessing(t)

		ok, err := executionStore.MarkFailed(context.Background(), int(job.ID), errMsg, opts)
		if !ok || err != nil {
			t.Fatalf("MarkFailed failed. ok=%t, err=%s", ok, err)
		}

		assertJobState(t, btypes.BatchSpecWorkspaceExecutionJobStateFailed)
	})

	t.Run("worker hostname mismatch", func(t *testing.T) {
		setProcessing(t)

		opts := opts
		opts.WorkerHostname = "DOESNT-MATCH"

		ok, err := executionStore.MarkFailed(context.Background(), int(job.ID), errMsg, opts)
		if ok || err != nil {
			t.Fatalf("MarkFailed returned wrong result. ok=%t, err=%s", ok, err)
		}

		assertJobState(t, btypes.BatchSpecWorkspaceExecutionJobStateProcessing)
	})
}

func TestBatchSpecWorkspaceExecutionWorkerStore_MarkComplete_EmptyDiff(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	user := bt.CreateTestUser(t, db, true)

	repo, _ := bt.CreateTestRepo(t, ctx, db)

	s := New(db, &observation.TestContext, nil)
	workStore := dbworkerstore.New(&observation.TestContext, s.Handle(), batchSpecWorkspaceExecutionWorkerStoreOptions)

	// Setup all the associations
	batchSpec := &btypes.BatchSpec{UserID: user.ID, NamespaceUserID: user.ID, RawSpec: "horse", Spec: &batcheslib.BatchSpec{
		ChangesetTemplate: &batcheslib.ChangesetTemplate{},
	}}
	if err := s.CreateBatchSpec(ctx, batchSpec); err != nil {
		t.Fatal(err)
	}

	workspace := &btypes.BatchSpecWorkspace{BatchSpecID: batchSpec.ID, RepoID: repo.ID}
	if err := s.CreateBatchSpecWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}

	job := &btypes.BatchSpecWorkspaceExecutionJob{BatchSpecWorkspaceID: workspace.ID, UserID: user.ID}
	if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
		t.Fatal(err)
	}

	cacheEntryKeys := []string{"JkC7Q0OOCZZ3Acv79QfwSA-step-0"}

	// Log entries with cache entries that'll be used to build the changeset specs.
	output := `
stdout: {"operation":"CACHE_AFTER_STEP_RESULT","timestamp":"2021-11-04T12:43:19.551Z","status":"SUCCESS","metadata":{"key":"JkC7Q0OOCZZ3Acv79QfwSA-step-0","value":{"stepIndex":0,"diff":"","outputs":{},"previousStepResult":{"Files":null,"Stdout":null,"Stderr":null}}}}`

	entry := executor.ExecutionLogEntry{
		Key:        "step.src.batch-exec",
		Command:    []string{"src", "batch", "preview", "-f", "spec.yml", "-text-only"},
		StartTime:  time.Now().Add(-5 * time.Second),
		Out:        output,
		DurationMs: intptr(200),
	}

	_, err := workStore.AddExecutionLogEntry(ctx, int(job.ID), entry, dbworkerstore.ExecutionLogEntryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	executionStore := &batchSpecWorkspaceExecutionWorkerStore{
		Store:          workStore,
		observationCtx: &observation.TestContext,
	}
	opts := dbworkerstore.MarkFinalOptions{WorkerHostname: "worker-1"}

	job.State = btypes.BatchSpecWorkspaceExecutionJobStateProcessing
	job.WorkerHostname = opts.WorkerHostname
	bt.UpdateJobState(t, ctx, s, job)

	ok, err := executionStore.MarkComplete(context.Background(), int(job.ID), opts)
	if !ok || err != nil {
		t.Fatalf("MarkComplete failed. ok=%t, err=%s", ok, err)
	}

	specs, _, err := s.ListChangesetSpecs(ctx, ListChangesetSpecsOpts{BatchSpecID: batchSpec.ID})
	if err != nil {
		t.Fatalf("failed to load changeset specs: %s", err)
	}
	if have, want := len(specs), 0; have != want {
		t.Fatalf("invalid number of changeset specs created: have=%d want=%d", have, want)
	}

	for _, wantKey := range cacheEntryKeys {
		entries, err := s.ListBatchSpecExecutionCacheEntries(ctx, ListBatchSpecExecutionCacheEntriesOpts{
			UserID: user.ID,
			Keys:   []string{wantKey},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatal("cache entry not found")
		}
	}
}

func TestBatchSpecWorkspaceExecutionWorkerStore_Dequeue_RoundRobin(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	repo, _ := bt.CreateTestRepo(t, ctx, db)

	s := New(db, &observation.TestContext, nil)
	workerStore := dbworkerstore.New(&observation.TestContext, s.Handle(), batchSpecWorkspaceExecutionWorkerStoreOptions)

	user1 := bt.CreateTestUser(t, db, true)
	user2 := bt.CreateTestUser(t, db, true)
	user3 := bt.CreateTestUser(t, db, true)

	user1BatchSpec := setupUserBatchSpec(t, ctx, s, user1)
	user2BatchSpec := setupUserBatchSpec(t, ctx, s, user2)
	user3BatchSpec := setupUserBatchSpec(t, ctx, s, user3)

	// We create multiple jobs for each user because this test ensures jobs are
	// dequeued in a round-robin fashion, starting with the user who dequeued
	// the longest ago.
	job1 := setupBatchSpecAssociation(ctx, s, t, user1BatchSpec, repo) // User_ID: 1
	job2 := setupBatchSpecAssociation(ctx, s, t, user1BatchSpec, repo) // User_ID: 1
	job3 := setupBatchSpecAssociation(ctx, s, t, user2BatchSpec, repo) // User_ID: 2
	job4 := setupBatchSpecAssociation(ctx, s, t, user2BatchSpec, repo) // User_ID: 2
	job5 := setupBatchSpecAssociation(ctx, s, t, user3BatchSpec, repo) // User_ID: 3
	job6 := setupBatchSpecAssociation(ctx, s, t, user3BatchSpec, repo) // User_ID: 3

	want := []int64{job1, job3, job5, job2, job4, job6}
	have := []int64{}

	// We dequeue records until there are no more left. Then, we check in which
	// order they were returned.
	for {
		r, found, err := workerStore.Dequeue(ctx, "test-worker", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
		have = append(have, int64(r.RecordID()))
	}

	if diff := cmp.Diff(want, have); diff != "" {
		t.Fatal(diff)
	}
}

func TestBatchSpecWorkspaceExecutionWorkerStore_Dequeue_RoundRobin_NoDoubleDequeue(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx := context.Background()
	db := database.NewDB(logger, dbtest.NewDB(t))

	repo, _ := bt.CreateTestRepo(t, ctx, db)

	s := New(db, &observation.TestContext, nil)
	workerStore := dbworkerstore.New(&observation.TestContext, s.Handle(), batchSpecWorkspaceExecutionWorkerStoreOptions)

	user1 := bt.CreateTestUser(t, db, true)
	user2 := bt.CreateTestUser(t, db, true)
	user3 := bt.CreateTestUser(t, db, true)

	user1BatchSpec := setupUserBatchSpec(t, ctx, s, user1)
	user2BatchSpec := setupUserBatchSpec(t, ctx, s, user2)
	user3BatchSpec := setupUserBatchSpec(t, ctx, s, user3)

	// We create multiple jobs for each user because this test ensures jobs are
	// dequeued in a round-robin fashion, starting with the user who dequeued
	// the longest ago.
	for range 100 {
		setupBatchSpecAssociation(ctx, s, t, user1BatchSpec, repo)
		setupBatchSpecAssociation(ctx, s, t, user2BatchSpec, repo)
		setupBatchSpecAssociation(ctx, s, t, user3BatchSpec, repo)
	}

	have := []int64{}
	var haveLock sync.Mutex

	errs := make(chan error)

	// We dequeue records until there are no more left. We spawn 8 concurrent
	// "workers" to find potential locking issues.
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				r, found, err := workerStore.Dequeue(ctx, "test-worker", nil)
				if err != nil {
					errs <- err
				}
				if !found {
					break
				}
				haveLock.Lock()
				have = append(have, int64(r.RecordID()))
				haveLock.Unlock()
			}
		}()
	}
	var multiErr error
	errDone := make(chan struct{})
	go func() {
		for err := range errs {
			multiErr = errors.Append(multiErr, err)
		}
		close(errDone)
	}()

	wg.Wait()
	close(errs)
	<-errDone

	if multiErr != nil {
		t.Fatal(multiErr)
	}

	// Check for duplicates.
	seen := make(map[int64]struct{})
	for _, h := range have {
		if _, ok := seen[h]; ok {
			t.Fatal("duplicate dequeue")
		}
		seen[h] = struct{}{}
	}
}

func setupUserBatchSpec(t *testing.T, ctx context.Context, s *Store, user *types.User) *btypes.BatchSpec {
	t.Helper()
	bs := &btypes.BatchSpec{UserID: user.ID, NamespaceUserID: user.ID, RawSpec: "horse", Spec: &batcheslib.BatchSpec{
		ChangesetTemplate: &batcheslib.ChangesetTemplate{},
	}}
	if err := s.CreateBatchSpec(ctx, bs); err != nil {
		t.Fatal(err)
	}
	return bs
}

func setupBatchSpecAssociation(ctx context.Context, s *Store, t *testing.T, batchSpec *btypes.BatchSpec, repo *types.Repo) int64 {
	workspace := &btypes.BatchSpecWorkspace{BatchSpecID: batchSpec.ID, RepoID: repo.ID}
	if err := s.CreateBatchSpecWorkspace(ctx, workspace); err != nil {
		t.Fatal(err)
	}

	job := &btypes.BatchSpecWorkspaceExecutionJob{BatchSpecWorkspaceID: workspace.ID, UserID: batchSpec.UserID}
	if err := bt.CreateBatchSpecWorkspaceExecutionJob(ctx, s, ScanBatchSpecWorkspaceExecutionJob, job); err != nil {
		t.Fatal(err)
	}

	return job.ID
}

func intptr(i int) *int { return &i }
