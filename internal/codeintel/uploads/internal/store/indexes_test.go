package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/executor"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGetIndexes(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(-time.Minute * 1)
	t3 := t1.Add(-time.Minute * 2)
	t4 := t1.Add(-time.Minute * 3)
	t5 := t1.Add(-time.Minute * 4)
	t6 := t1.Add(-time.Minute * 5)
	t7 := t1.Add(-time.Minute * 6)
	t8 := t1.Add(-time.Minute * 7)
	t9 := t1.Add(-time.Minute * 8)
	t10 := t1.Add(-time.Minute * 9)
	failureMessage := "unlucky 333"

	indexID1, indexID2, indexID3, indexID4 := 1, 3, 5, 5 // note the duplication
	uploadID1, uploadID2, uploadID3, uploadID4 := 10, 11, 12, 13

	insertIndexes(t, db,
		uploadsshared.Index{ID: 1, Commit: makeCommit(3331), QueuedAt: t1, State: "queued", AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 2, QueuedAt: t2, State: "errored", FailureMessage: &failureMessage},
		uploadsshared.Index{ID: 3, Commit: makeCommit(3333), QueuedAt: t3, State: "queued", AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 4, QueuedAt: t4, State: "queued", RepositoryID: 51, RepositoryName: "foo bar x"},
		uploadsshared.Index{ID: 5, Commit: makeCommit(3333), QueuedAt: t5, State: "processing", AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 6, QueuedAt: t6, State: "processing", RepositoryID: 52, RepositoryName: "foo bar y"},
		uploadsshared.Index{ID: 7, QueuedAt: t7, Indexer: "lsif-typescript"},
		uploadsshared.Index{ID: 8, QueuedAt: t8, Indexer: "scip-ocaml"},
		uploadsshared.Index{ID: 9, QueuedAt: t9, State: "queued"},
		uploadsshared.Index{ID: 10, QueuedAt: t10},
	)
	insertUploads(t, db,
		shared.Upload{ID: uploadID1, AssociatedIndexID: &indexID1},
		shared.Upload{ID: uploadID2, AssociatedIndexID: &indexID2},
		shared.Upload{ID: uploadID3, AssociatedIndexID: &indexID3},
		shared.Upload{ID: uploadID4, AssociatedIndexID: &indexID4},
	)

	testCases := []struct {
		repositoryID  int
		state         string
		states        []string
		term          string
		indexerNames  []string
		withoutUpload bool
		expectedIDs   []int
	}{
		{expectedIDs: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
		{repositoryID: 50, expectedIDs: []int{1, 2, 3, 5, 7, 8, 9, 10}},
		{state: "completed", expectedIDs: []int{7, 8, 10}},
		{term: "003", expectedIDs: []int{1, 3, 5}},                                 // searches commits
		{term: "333", expectedIDs: []int{1, 2, 3, 5}},                              // searches commits and failure message
		{term: "QuEuEd", expectedIDs: []int{1, 3, 4, 9}},                           // searches text status
		{term: "bAr", expectedIDs: []int{4, 6}},                                    // search repo names
		{state: "failed", expectedIDs: []int{2}},                                   // treats errored/failed states equivalently
		{states: []string{"completed", "failed"}, expectedIDs: []int{2, 7, 8, 10}}, // searches multiple states
		{withoutUpload: true, expectedIDs: []int{2, 4, 6, 7, 8, 9, 10}},            // anti-join with upload records
		{indexerNames: []string{"typescript", "ocaml"}, expectedIDs: []int{7, 8}},  // searches indexer name (only)
	}

	for _, testCase := range testCases {
		for lo := range len(testCase.expectedIDs) {
			hi := lo + 3
			if hi > len(testCase.expectedIDs) {
				hi = len(testCase.expectedIDs)
			}

			name := fmt.Sprintf(
				"repositoryID=%d state=%s states=%s term=%s without_upload=%v indexer_names=%v offset=%d",
				testCase.repositoryID,
				testCase.state,
				strings.Join(testCase.states, ","),
				testCase.term,
				testCase.withoutUpload,
				testCase.indexerNames,
				lo,
			)

			t.Run(name, func(t *testing.T) {
				indexes, totalCount, err := store.GetIndexes(ctx, shared.GetIndexesOptions{
					RepositoryID:  testCase.repositoryID,
					State:         testCase.state,
					States:        testCase.states,
					Term:          testCase.term,
					IndexerNames:  testCase.indexerNames,
					WithoutUpload: testCase.withoutUpload,
					Limit:         3,
					Offset:        lo,
				})
				if err != nil {
					t.Fatalf("unexpected error getting indexes for repo: %s", err)
				}
				if totalCount != len(testCase.expectedIDs) {
					t.Errorf("unexpected total count. want=%d have=%d", len(testCase.expectedIDs), totalCount)
				}

				var ids []int
				for _, index := range indexes {
					ids = append(ids, index.ID)
				}

				if diff := cmp.Diff(testCase.expectedIDs[lo:hi], ids); diff != "" {
					t.Errorf("unexpected index ids at offset %d (-want +got):\n%s", lo, diff)
				}
			})
		}
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		indexes, totalCount, err := store.GetIndexes(ctx,
			shared.GetIndexesOptions{
				Limit: 1,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(indexes) > 0 || totalCount > 0 {
			t.Fatalf("Want no index but got %d indexes with totalCount %d", len(indexes), totalCount)
		}
	})
}

func TestGetIndexByID(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	// Index does not exist initially
	if _, exists, err := store.GetIndexByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}

	uploadID := 5
	queuedAt := time.Unix(1587396557, 0).UTC()
	startedAt := queuedAt.Add(time.Minute)
	expected := uploadsshared.Index{
		ID:             1,
		Commit:         makeCommit(1),
		QueuedAt:       queuedAt,
		State:          "processing",
		FailureMessage: nil,
		StartedAt:      &startedAt,
		FinishedAt:     nil,
		RepositoryID:   123,
		RepositoryName: "n-123",
		DockerSteps: []uploadsshared.DockerStep{
			{
				Image:    "cimg/node:12.16",
				Commands: []string{"yarn install --frozen-lockfile --no-progress"},
			},
		},
		LocalSteps:  []string{"echo hello"},
		Root:        "/foo/bar",
		Indexer:     "sourcegraph/scip-typescript:latest",
		IndexerArgs: []string{"index", "--yarn-workspaces"},
		Outfile:     "dump.lsif",
		ExecutionLogs: []executor.ExecutionLogEntry{
			{Command: []string{"op", "1"}, Out: "Indexing\nUploading\nDone with 1.\n"},
			{Command: []string{"op", "2"}, Out: "Indexing\nUploading\nDone with 2.\n"},
		},
		Rank:               nil,
		AssociatedUploadID: &uploadID,
	}

	insertIndexes(t, db, expected)
	insertUploads(t, db, shared.Upload{ID: uploadID, AssociatedIndexID: &expected.ID})

	if index, exists, err := store.GetIndexByID(ctx, 1); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if !exists {
		t.Fatal("expected record to exist")
	} else if diff := cmp.Diff(expected, index); diff != "" {
		t.Errorf("unexpected index (-want +got):\n%s", diff)
	}

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		_, exists, err := store.GetIndexByID(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Fatalf("exists: want false but got %v", exists)
		}
	})
}

func TestGetQueuedIndexRank(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(+time.Minute * 6)
	t3 := t1.Add(+time.Minute * 3)
	t4 := t1.Add(+time.Minute * 1)
	t5 := t1.Add(+time.Minute * 4)
	t6 := t1.Add(+time.Minute * 2)
	t7 := t1.Add(+time.Minute * 5)

	insertIndexes(t, db,
		uploadsshared.Index{ID: 1, QueuedAt: t1, State: "queued"},
		uploadsshared.Index{ID: 2, QueuedAt: t2, State: "queued"},
		uploadsshared.Index{ID: 3, QueuedAt: t3, State: "queued"},
		uploadsshared.Index{ID: 4, QueuedAt: t4, State: "queued"},
		uploadsshared.Index{ID: 5, QueuedAt: t5, State: "queued"},
		uploadsshared.Index{ID: 6, QueuedAt: t6, State: "processing"},
		uploadsshared.Index{ID: 7, QueuedAt: t1, State: "queued", ProcessAfter: &t7},
	)

	if index, _, _ := store.GetIndexByID(context.Background(), 1); index.Rank == nil || *index.Rank != 1 {
		t.Errorf("unexpected rank. want=%d have=%s", 1, printableRank{index.Rank})
	}
	if index, _, _ := store.GetIndexByID(context.Background(), 2); index.Rank == nil || *index.Rank != 6 {
		t.Errorf("unexpected rank. want=%d have=%s", 5, printableRank{index.Rank})
	}
	if index, _, _ := store.GetIndexByID(context.Background(), 3); index.Rank == nil || *index.Rank != 3 {
		t.Errorf("unexpected rank. want=%d have=%s", 3, printableRank{index.Rank})
	}
	if index, _, _ := store.GetIndexByID(context.Background(), 4); index.Rank == nil || *index.Rank != 2 {
		t.Errorf("unexpected rank. want=%d have=%s", 2, printableRank{index.Rank})
	}
	if index, _, _ := store.GetIndexByID(context.Background(), 5); index.Rank == nil || *index.Rank != 4 {
		t.Errorf("unexpected rank. want=%d have=%s", 4, printableRank{index.Rank})
	}

	// Only considers queued indexes to determine rank
	if index, _, _ := store.GetIndexByID(context.Background(), 6); index.Rank != nil {
		t.Errorf("unexpected rank. want=%s have=%s", "nil", printableRank{index.Rank})
	}

	// Process after takes priority over upload time
	if upload, _, _ := store.GetIndexByID(context.Background(), 7); upload.Rank == nil || *upload.Rank != 5 {
		t.Errorf("unexpected rank. want=%d have=%s", 4, printableRank{upload.Rank})
	}
}

func TestGetIndexesByIDs(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	indexID1, indexID2, indexID3, indexID4 := 1, 3, 5, 5 // note the duplication
	uploadID1, uploadID2, uploadID3, uploadID4 := 10, 11, 12, 13

	insertIndexes(t, db,
		uploadsshared.Index{ID: 1, AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 2},
		uploadsshared.Index{ID: 3, AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 4},
		uploadsshared.Index{ID: 5, AssociatedUploadID: &uploadID1},
		uploadsshared.Index{ID: 6},
		uploadsshared.Index{ID: 7},
		uploadsshared.Index{ID: 8},
		uploadsshared.Index{ID: 9},
		uploadsshared.Index{ID: 10},
	)
	insertUploads(t, db,
		shared.Upload{ID: uploadID1, AssociatedIndexID: &indexID1},
		shared.Upload{ID: uploadID2, AssociatedIndexID: &indexID2},
		shared.Upload{ID: uploadID3, AssociatedIndexID: &indexID3},
		shared.Upload{ID: uploadID4, AssociatedIndexID: &indexID4},
	)

	t.Run("fetch", func(t *testing.T) {
		indexes, err := store.GetIndexesByIDs(ctx, 2, 4, 6, 8, 12)
		if err != nil {
			t.Fatalf("unexpected error getting indexes for repo: %s", err)
		}

		var ids []int
		for _, index := range indexes {
			ids = append(ids, index.ID)
		}
		sort.Ints(ids)

		if diff := cmp.Diff([]int{2, 4, 6, 8}, ids); diff != "" {
			t.Errorf("unexpected index ids (-want +got):\n%s", diff)
		}
	})

	t.Run("enforce repository permissions", func(t *testing.T) {
		// Enable permissions user mapping forces checking repository permissions
		// against permissions tables in the database, which should effectively block
		// all access because permissions tables are empty.
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				PermissionsUserMapping: &schema.PermissionsUserMapping{
					Enabled: true,
					BindID:  "email",
				},
			},
		})
		t.Cleanup(func() { conf.Mock(nil) })

		indexes, err := store.GetIndexesByIDs(ctx, 1, 2, 3, 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(indexes) > 0 {
			t.Fatalf("Want no index but got %d indexes", len(indexes))
		}
	})
}

func TestDeleteIndexByID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1})

	if found, err := store.DeleteIndexByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error deleting index: %s", err)
	} else if !found {
		t.Fatalf("expected record to exist")
	}

	// Index no longer exists
	if _, exists, err := store.GetIndexByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}
}

func TestDeleteIndexes(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1, State: "completed"})
	insertIndexes(t, db, uploadsshared.Index{ID: 2, State: "errored"})

	if err := store.DeleteIndexes(context.Background(), shared.DeleteIndexesOptions{
		States:       []string{"errored"},
		Term:         "",
		RepositoryID: 0,
	}); err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}

	// Index no longer exists
	if _, exists, err := store.GetIndexByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if exists {
		t.Fatal("unexpected record")
	}
}

func TestDeleteIndexesWithIndexerKey(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1, Indexer: "sourcegraph/scip-go@sha256:123456"})
	insertIndexes(t, db, uploadsshared.Index{ID: 2, Indexer: "sourcegraph/scip-go"})
	insertIndexes(t, db, uploadsshared.Index{ID: 3, Indexer: "sourcegraph/scip-typescript"})
	insertIndexes(t, db, uploadsshared.Index{ID: 4, Indexer: "sourcegraph/scip-typescript"})

	if err := store.DeleteIndexes(context.Background(), shared.DeleteIndexesOptions{
		IndexerNames: []string{"scip-go"},
	}); err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}

	// Target indexes no longer exist
	for _, id := range []int{1, 2} {
		if _, exists, err := store.GetIndexByID(context.Background(), id); err != nil {
			t.Fatalf("unexpected error getting index: %s", err)
		} else if exists {
			t.Fatal("unexpected record")
		}
	}

	// Unmatched indexes remain
	for _, id := range []int{3, 4} {
		if _, exists, err := store.GetIndexByID(context.Background(), id); err != nil {
			t.Fatalf("unexpected error getting index: %s", err)
		} else if !exists {
			t.Fatal("expected record, got none")
		}
	}
}

func TestReindexIndexByID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1, State: "completed"})
	insertIndexes(t, db, uploadsshared.Index{ID: 2, State: "errored"})

	if err := store.ReindexIndexByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}

	// Index has been marked for reindexing
	if index, exists, err := store.GetIndexByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if !exists {
		t.Fatal("index missing")
	} else if !index.ShouldReindex {
		t.Fatal("index not marked for reindexing")
	}
}

func TestReindexIndexes(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1, State: "completed"})
	insertIndexes(t, db, uploadsshared.Index{ID: 2, State: "errored"})

	if err := store.ReindexIndexes(context.Background(), shared.ReindexIndexesOptions{
		States:       []string{"errored"},
		Term:         "",
		RepositoryID: 0,
	}); err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}

	// Index has been marked for reindexing
	if index, exists, err := store.GetIndexByID(context.Background(), 2); err != nil {
		t.Fatalf("unexpected error getting index: %s", err)
	} else if !exists {
		t.Fatal("index missing")
	} else if !index.ShouldReindex {
		t.Fatal("index not marked for reindexing")
	}
}

func TestReindexIndexesWithIndexerKey(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertIndexes(t, db, uploadsshared.Index{ID: 1, Indexer: "sourcegraph/scip-go@sha256:123456"})
	insertIndexes(t, db, uploadsshared.Index{ID: 2, Indexer: "sourcegraph/scip-go"})
	insertIndexes(t, db, uploadsshared.Index{ID: 3, Indexer: "sourcegraph/scip-typescript"})
	insertIndexes(t, db, uploadsshared.Index{ID: 4, Indexer: "sourcegraph/scip-typescript"})

	if err := store.ReindexIndexes(context.Background(), shared.ReindexIndexesOptions{
		IndexerNames: []string{"scip-go"},
		Term:         "",
		RepositoryID: 0,
	}); err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}

	// Expected indexes marked for re-indexing
	for id, expected := range map[int]bool{
		1: true, 2: true,
		3: false, 4: false,
	} {
		if index, exists, err := store.GetIndexByID(context.Background(), id); err != nil {
			t.Fatalf("unexpected error getting index: %s", err)
		} else if !exists {
			t.Fatal("index missing")
		} else if index.ShouldReindex != expected {
			t.Fatalf("unexpected mark. want=%v have=%v", expected, index.ShouldReindex)
		}
	}
}

func TestDeleteIndexByIDMissingRow(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	if found, err := store.DeleteIndexByID(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error deleting index: %s", err)
	} else if found {
		t.Fatalf("unexpected record")
	}
}
