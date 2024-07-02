package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/commitgraph"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestHasRepository(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(observation.TestContextTB(t), db)

	testCases := []struct {
		repositoryID int
		exists       bool
	}{
		{50, true},
		{51, false},
		{52, false},
	}

	insertUploads(t, db, shared.Upload{ID: 1, RepositoryID: 50})
	insertUploads(t, db, shared.Upload{ID: 2, RepositoryID: 51, State: "deleted"})

	for _, testCase := range testCases {
		name := fmt.Sprintf("repositoryID=%d", testCase.repositoryID)

		t.Run(name, func(t *testing.T) {
			exists, err := store.HasRepository(context.Background(), api.RepoID(testCase.repositoryID))
			if err != nil {
				t.Fatalf("unexpected error checking if repository exists: %s", err)
			}
			if exists != testCase.exists {
				t.Errorf("unexpected exists. want=%v have=%v", testCase.exists, exists)
			}
		})
	}
}

func TestHasCommit(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(observation.TestContextTB(t), db)

	testCases := []struct {
		repositoryID int
		commit       string
		exists       bool
	}{
		{50, makeCommit(1), true},
		{50, makeCommit(2), false},
		{51, makeCommit(1), false},
	}

	insertNearestUploads(t, db, 50, map[api.CommitID][]commitgraph.UploadMeta{api.CommitID(makeCommit(1)): {{UploadID: 42, Distance: 1}}})
	insertNearestUploads(t, db, 51, map[api.CommitID][]commitgraph.UploadMeta{api.CommitID(makeCommit(2)): {{UploadID: 43, Distance: 2}}})

	for _, testCase := range testCases {
		name := fmt.Sprintf("repositoryID=%d commit=%s", testCase.repositoryID, testCase.commit)

		t.Run(name, func(t *testing.T) {
			exists, err := store.HasCommit(context.Background(), api.RepoID(testCase.repositoryID), api.CommitID(testCase.commit))
			if err != nil {
				t.Fatalf("unexpected error checking if commit exists: %s", err)
			}
			if exists != testCase.exists {
				t.Errorf("unexpected exists. want=%v have=%v", testCase.exists, exists)
			}
		})
	}
}

func TestInsertDependencySyncingJob(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(observation.TestContextTB(t), db)

	uploadID := 42
	insertRepo(t, db, 50, "", false)
	insertUploads(t, db, shared.Upload{
		ID:            uploadID,
		Commit:        makeCommit(1),
		Root:          "sub/",
		State:         "queued",
		RepositoryID:  50,
		Indexer:       "lsif-go",
		NumParts:      1,
		UploadedParts: []int{0},
	})

	// No error if upload exists
	if _, err := store.InsertDependencySyncingJob(context.Background(), uploadID); err != nil {
		t.Fatalf("unexpected error enqueueing dependency indexing job: %s", err)
	}

	// Error with unknown identifier
	if _, err := store.InsertDependencySyncingJob(context.Background(), uploadID+1); err == nil {
		t.Fatalf("expected error enqueueing dependency indexing job for unknown upload")
	}
}
