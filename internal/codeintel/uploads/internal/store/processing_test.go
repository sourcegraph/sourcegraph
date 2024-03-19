package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestInsertUploadUploading(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "", false)

	id, err := store.InsertUpload(context.Background(), shared.Upload{
		Commit:       makeCommit(1),
		Root:         "sub/",
		State:        "uploading",
		RepositoryID: 50,
		Indexer:      "lsif-go",
		NumParts:     3,
	})
	if err != nil {
		t.Fatalf("unexpected error enqueueing upload: %s", err)
	}

	expected := shared.Upload{
		ID:             id,
		Commit:         makeCommit(1),
		Root:           "sub/",
		VisibleAtTip:   false,
		UploadedAt:     time.Time{},
		State:          "uploading",
		FailureMessage: nil,
		StartedAt:      nil,
		FinishedAt:     nil,
		RepositoryID:   50,
		RepositoryName: "n-50",
		Indexer:        "lsif-go",
		NumParts:       3,
		UploadedParts:  []int{},
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), id); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else {
		// Update auto-generated timestamp
		expected.UploadedAt = upload.UploadedAt

		if diff := cmp.Diff(expected, upload); diff != "" {
			t.Errorf("unexpected upload (-want +got):\n%s", diff)
		}
	}
}

func TestInsertUploadQueued(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "", false)

	id, err := store.InsertUpload(context.Background(), shared.Upload{
		Commit:        makeCommit(1),
		Root:          "sub/",
		State:         "queued",
		RepositoryID:  50,
		Indexer:       "lsif-go",
		NumParts:      1,
		UploadedParts: []int{0},
	})
	if err != nil {
		t.Fatalf("unexpected error enqueueing upload: %s", err)
	}

	rank := 1
	expected := shared.Upload{
		ID:             id,
		Commit:         makeCommit(1),
		Root:           "sub/",
		VisibleAtTip:   false,
		UploadedAt:     time.Time{},
		State:          "queued",
		FailureMessage: nil,
		StartedAt:      nil,
		FinishedAt:     nil,
		RepositoryID:   50,
		RepositoryName: "n-50",
		Indexer:        "lsif-go",
		NumParts:       1,
		UploadedParts:  []int{0},
		Rank:           &rank,
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), id); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else {
		// Update auto-generated timestamp
		expected.UploadedAt = upload.UploadedAt

		if diff := cmp.Diff(expected, upload); diff != "" {
			t.Errorf("unexpected upload (-want +got):\n%s", diff)
		}
	}
}

func TestInsertUploadWithAssociatedIndexID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertRepo(t, db, 50, "", false)

	associatedIndexIDArg := 42
	id, err := store.InsertUpload(context.Background(), shared.Upload{
		Commit:            makeCommit(1),
		Root:              "sub/",
		State:             "queued",
		RepositoryID:      50,
		Indexer:           "lsif-go",
		NumParts:          1,
		UploadedParts:     []int{0},
		AssociatedIndexID: &associatedIndexIDArg,
	})
	if err != nil {
		t.Fatalf("unexpected error enqueueing upload: %s", err)
	}

	rank := 1
	associatedIndexIDResult := 42
	expected := shared.Upload{
		ID:                id,
		Commit:            makeCommit(1),
		Root:              "sub/",
		VisibleAtTip:      false,
		UploadedAt:        time.Time{},
		State:             "queued",
		FailureMessage:    nil,
		StartedAt:         nil,
		FinishedAt:        nil,
		RepositoryID:      50,
		RepositoryName:    "n-50",
		Indexer:           "lsif-go",
		NumParts:          1,
		UploadedParts:     []int{0},
		Rank:              &rank,
		AssociatedIndexID: &associatedIndexIDResult,
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), id); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else {
		// Update auto-generated timestamp
		expected.UploadedAt = upload.UploadedAt

		if diff := cmp.Diff(expected, upload); diff != "" {
			t.Errorf("unexpected upload (-want +got):\n%s", diff)
		}
	}
}

func TestAddUploadPart(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "uploading"})

	for _, part := range []int{1, 5, 2, 3, 2, 2, 1, 6} {
		if err := store.AddUploadPart(context.Background(), 1, part); err != nil {
			t.Fatalf("unexpected error adding upload part: %s", err)
		}
	}
	if upload, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else {
		sort.Ints(upload.UploadedParts)
		if diff := cmp.Diff([]int{1, 2, 3, 5, 6}, upload.UploadedParts); diff != "" {
			t.Errorf("unexpected upload parts (-want +got):\n%s", diff)
		}
	}
}

func TestMarkQueued(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "uploading"})

	uploadSize := int64(300)
	if err := store.MarkQueued(context.Background(), 1, &uploadSize); err != nil {
		t.Fatalf("unexpected error marking upload as queued: %s", err)
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if upload.State != "queued" {
		t.Errorf("unexpected state. want=%q have=%q", "queued", upload.State)
	} else if upload.UploadSize == nil || *upload.UploadSize != 300 {
		if upload.UploadSize == nil {
			t.Errorf("unexpected upload size. want=%v have=%v", 300, upload.UploadSize)
		} else {
			t.Errorf("unexpected upload size. want=%v have=%v", 300, *upload.UploadSize)
		}
	}
}

func TestMarkQueuedNoSize(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "uploading"})

	if err := store.MarkQueued(context.Background(), 1, nil); err != nil {
		t.Fatalf("unexpected error marking upload as queued: %s", err)
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if upload.State != "queued" {
		t.Errorf("unexpected state. want=%q have=%q", "queued", upload.State)
	} else if upload.UploadSize != nil {
		t.Errorf("unexpected upload size. want=%v have=%v", nil, upload.UploadSize)
	}
}

func TestMarkFailed(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{ID: 1, State: "uploading"})

	failureReason := "didn't like it"
	if err := store.MarkFailed(context.Background(), 1, failureReason); err != nil {
		t.Fatalf("unexpected error marking upload as failed: %s", err)
	}

	if upload, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting upload: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if upload.State != "failed" {
		t.Errorf("unexpected state. want=%q have=%q", "failed", upload.State)
	} else if upload.NumFailures != 1 {
		t.Errorf("unexpected num failures. want=%v have=%v", 1, upload.NumFailures)
	} else if upload.FailureMessage == nil || *upload.FailureMessage != failureReason {
		if upload.FailureMessage == nil {
			t.Errorf("unexpected failure message. want='%s' have='%v'", failureReason, upload.FailureMessage)
		} else {
			t.Errorf("unexpected failure message. want='%s' have='%v'", failureReason, *upload.FailureMessage)
		}
	}
}

func TestDeleteOverlappingCompletedUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{
		ID:      1,
		Commit:  makeCommit(1),
		Root:    "cmd/",
		Indexer: "lsif-go",
	})

	err := store.DeleteOverlappingCompletedUploads(context.Background(), 50, makeCommit(1), "cmd/", "lsif-go")
	if err != nil {
		t.Fatalf("unexpected error deleting dump: %s", err)
	}

	// Ensure record was deleted
	if states, err := getUploadStates(db, 1); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(map[int]string{1: "deleting"}, states); diff != "" {
		t.Errorf("unexpected dump (-want +got):\n%s", diff)
	}
}

func TestDeleteOverlappingCompletedUploadsNoMatches(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{
		ID:      1,
		Commit:  makeCommit(1),
		Root:    "cmd/",
		Indexer: "lsif-go",
	})

	testCases := []struct {
		commit  string
		root    string
		indexer string
	}{
		{makeCommit(2), "cmd/", "lsif-go"},
		{makeCommit(1), "cmds/", "lsif-go"},
		{makeCommit(1), "cmd/", "scip-typescript"},
	}

	for _, testCase := range testCases {
		err := store.DeleteOverlappingCompletedUploads(context.Background(), 50, testCase.commit, testCase.root, testCase.indexer)
		if err != nil {
			t.Fatalf("unexpected error deleting dump: %s", err)
		}
	}

	// Original dump still exists
	if uploads, err := store.GetCompletedUploadsByIDs(context.Background(), []int{1}); err != nil {
		t.Fatalf("unexpected error getting dump: %s", err)
	} else if len(uploads) != 1 {
		t.Fatal("expected dump record to still exist")
	}
}

func TestDeleteOverlappingCompletedUploadsIgnoresIncompleteUploads(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	insertUploads(t, db, shared.Upload{
		ID:      1,
		Commit:  makeCommit(1),
		Root:    "cmd/",
		Indexer: "lsif-go",
		State:   "queued",
	})

	err := store.DeleteOverlappingCompletedUploads(context.Background(), 50, makeCommit(1), "cmd/", "lsif-go")
	if err != nil {
		t.Fatalf("unexpected error deleting dump: %s", err)
	}

	// Original upload still exists
	if _, exists, err := store.GetUploadByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting dump: %s", err)
	} else if !exists {
		t.Fatal("expected dump record to still exist")
	}
}
