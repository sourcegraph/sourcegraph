package store

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	logger "github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestHardDeleteUploadsByIDs(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	insertUploads(t, db,
		shared.Upload{ID: 51, State: "deleting"},
		shared.Upload{ID: 52, State: "completed"},
		shared.Upload{ID: 53, State: "queued"},
		shared.Upload{ID: 54, State: "completed"},
	)

	if err := store.HardDeleteUploadsByIDs(context.Background(), 51); err != nil {
		t.Fatalf("unexpected error deleting upload: %s", err)
	}

	expectedStates := map[int]string{
		52: "completed",
		53: "queued",
		54: "completed",
	}
	if states, err := getUploadStates(db, 50, 51, 52, 53, 54, 55, 56); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else if diff := cmp.Diff(expectedStates, states); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}
}

func TestDeleteUploadsStuckUploading(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(time.Minute * 1)
	t3 := t1.Add(time.Minute * 2)
	t4 := t1.Add(time.Minute * 3)
	t5 := t1.Add(time.Minute * 4)

	insertUploads(t, db,
		shared.Upload{ID: 1, Commit: makeCommit(1111), UploadedAt: t1, State: "queued"},    // not uploading
		shared.Upload{ID: 2, Commit: makeCommit(1112), UploadedAt: t2, State: "uploading"}, // deleted
		shared.Upload{ID: 3, Commit: makeCommit(1113), UploadedAt: t3, State: "uploading"}, // deleted
		shared.Upload{ID: 4, Commit: makeCommit(1114), UploadedAt: t4, State: "completed"}, // old, not uploading
		shared.Upload{ID: 5, Commit: makeCommit(1115), UploadedAt: t5, State: "uploading"}, // old
	)

	_, count, err := store.DeleteUploadsStuckUploading(context.Background(), t1.Add(time.Minute*3))
	if err != nil {
		t.Fatalf("unexpected error deleting uploads stuck uploading: %s", err)
	}
	if count != 2 {
		t.Errorf("unexpected count. want=%d have=%d", 2, count)
	}

	uploads, totalCount, err := store.GetUploads(context.Background(), shared.GetUploadsOptions{Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error getting uploads: %s", err)
	}

	var ids []int
	for _, upload := range uploads {
		ids = append(ids, upload.ID)
	}
	sort.Ints(ids)

	expectedIDs := []int{1, 4, 5}

	if totalCount != len(expectedIDs) {
		t.Errorf("unexpected total count. want=%d have=%d", len(expectedIDs), totalCount)
	}
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Errorf("unexpected upload ids (-want +got):\n%s", diff)
	}
}

func TestDeleteUploadsWithoutRepository(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	var uploads []shared.Upload
	for i := range 25 {
		for range 10 + i {
			uploads = append(uploads, shared.Upload{ID: len(uploads) + 1, RepositoryID: 50 + i})
		}
	}
	insertUploads(t, db, uploads...)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(-deletedRepositoryGracePeriod + time.Minute)
	t3 := t1.Add(-deletedRepositoryGracePeriod - time.Minute)

	deletions := map[int]time.Time{
		52: t2, 54: t2, 56: t2, // deleted too recently
		61: t3, 63: t3, 65: t3, // deleted
	}

	for repositoryID, deletedAt := range deletions {
		query := sqlf.Sprintf(`UPDATE repo SET deleted_at=%s WHERE id=%s`, deletedAt, repositoryID)

		if _, err := db.QueryContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
			t.Fatalf("Failed to update repository: %s", err)
		}
	}

	_, count, err := store.DeleteUploadsWithoutRepository(context.Background(), t1)
	if err != nil {
		t.Fatalf("unexpected error deleting uploads: %s", err)
	}
	if expected := 21 + 23 + 25; count != expected {
		t.Fatalf("unexpected count. want=%d have=%d", expected, count)
	}

	var uploadIDs []int
	for i := range uploads {
		uploadIDs = append(uploadIDs, i+1)
	}

	// Ensure records were deleted
	if states, err := getUploadStates(db, uploadIDs...); err != nil {
		t.Fatalf("unexpected error getting states: %s", err)
	} else {
		deletedStates := 0
		for _, state := range states {
			if state == "deleted" {
				deletedStates++
			}
		}

		if deletedStates != count {
			t.Errorf("unexpected number of deleted records. want=%d have=%d", count, deletedStates)
		}
	}
}

func TestDeleteOldAuditLogs(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	// Sanity check for syntax only
	if _, _, err := store.DeleteOldAuditLogs(context.Background(), time.Second, time.Now()); err != nil {
		t.Fatalf("unexpected error deleting old audit logs: %s", err)
	}
}

func TestReconcileCandidates(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (100, 50, '0000000000000000000000000000000000000001', 'lsif-test', 1, '{}', 'completed');
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (101, 50, '0000000000000000000000000000000000000002', 'lsif-test', 1, '{}', 'completed');
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (102, 50, '0000000000000000000000000000000000000003', 'lsif-test', 1, '{}', 'completed');
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (103, 50, '0000000000000000000000000000000000000004', 'lsif-test', 1, '{}', 'completed');
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (104, 50, '0000000000000000000000000000000000000005', 'lsif-test', 1, '{}', 'completed');
		INSERT INTO lsif_uploads (id, repository_id, commit, indexer, num_parts, uploaded_parts, state) VALUES (105, 50, '0000000000000000000000000000000000000006', 'lsif-test', 1, '{}', 'completed');
	`); err != nil {
		t.Fatalf("unexpected error setting up test: %s", err)
	}

	// Initial batch of records
	ids, err := store.ReconcileCandidates(ctx, 4)
	if err != nil {
		t.Fatalf("failed to get candidate IDs for reconciliation: %s", err)
	}
	expectedIDs := []int{
		100,
		101,
		102,
		103,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}

	// Remaining records, wrap around
	ids, err = store.ReconcileCandidates(ctx, 4)
	if err != nil {
		t.Fatalf("failed to get candidate IDs for reconciliation: %s", err)
	}
	expectedIDs = []int{
		100,
		101,
		104,
		105,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}
}

func TestProcessStaleSourcedCommits(t *testing.T) {
	log := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(log, sqlDB)
	store := &store{
		db:         basestore.NewWithHandle(db.Handle()),
		logger:     logger.Scoped("autoindexing.store"),
		operations: newOperations(&observation.TestContext),
	}

	ctx := context.Background()
	now := time.Unix(1587396557, 0).UTC()

	insertIndexes(t, db,
		uploadsshared.Index{ID: 1, RepositoryID: 50, Commit: makeCommit(1)},
		uploadsshared.Index{ID: 2, RepositoryID: 50, Commit: makeCommit(2)},
		uploadsshared.Index{ID: 3, RepositoryID: 50, Commit: makeCommit(3)},
		uploadsshared.Index{ID: 4, RepositoryID: 51, Commit: makeCommit(6)},
		uploadsshared.Index{ID: 5, RepositoryID: 52, Commit: makeCommit(7)},
	)

	const (
		minimumTimeSinceLastCheck = time.Minute
		commitResolverBatchSize   = 5
	)

	// First update
	deleteCommit3 := func(ctx context.Context, repositoryID int, respositoryName, commit string) (bool, error) {
		return commit == makeCommit(3), nil
	}
	if _, numDeleted, err := store.processStaleSourcedCommits(
		ctx,
		minimumTimeSinceLastCheck,
		commitResolverBatchSize,
		deleteCommit3,
		now,
	); err != nil {
		t.Fatalf("unexpected error processing stale sourced commits: %s", err)
	} else if numDeleted != 1 {
		t.Fatalf("unexpected number of deleted indexes. want=%d have=%d", 1, numDeleted)
	}
	indexStates, err := getIndexStates(db, 1, 2, 3, 4, 5)
	if err != nil {
		t.Fatalf("unexpected error fetching index states: %s", err)
	}
	expectedIndexStates := map[int]string{
		1: "completed",
		2: "completed",
		// 3 was deleted
		4: "completed",
		5: "completed",
	}
	if diff := cmp.Diff(expectedIndexStates, indexStates); diff != "" {
		t.Errorf("unexpected index states (-want +got):\n%s", diff)
	}

	// Too soon after last update
	deleteCommit2 := func(ctx context.Context, repositoryID int, respositoryName, commit string) (bool, error) {
		return commit == makeCommit(2), nil
	}
	if _, numDeleted, err := store.processStaleSourcedCommits(
		ctx,
		minimumTimeSinceLastCheck,
		commitResolverBatchSize,
		deleteCommit2,
		now.Add(minimumTimeSinceLastCheck/2),
	); err != nil {
		t.Fatalf("unexpected error processing stale sourced commits: %s", err)
	} else if numDeleted != 0 {
		t.Fatalf("unexpected number of deleted indexes. want=%d have=%d", 0, numDeleted)
	}
	indexStates, err = getIndexStates(db, 1, 2, 3, 4, 5)
	if err != nil {
		t.Fatalf("unexpected error fetching index states: %s", err)
	}
	// no change in expectedIndexStates
	if diff := cmp.Diff(expectedIndexStates, indexStates); diff != "" {
		t.Errorf("unexpected index states (-want +got):\n%s", diff)
	}

	// Enough time after previous update(s)
	if _, numDeleted, err := store.processStaleSourcedCommits(
		ctx,
		minimumTimeSinceLastCheck,
		commitResolverBatchSize,
		deleteCommit2,
		now.Add(minimumTimeSinceLastCheck/2*3),
	); err != nil {
		t.Fatalf("unexpected error processing stale sourced commits: %s", err)
	} else if numDeleted != 1 {
		t.Fatalf("unexpected number of deleted indexes. want=%d have=%d", 1, numDeleted)
	}
	indexStates, err = getIndexStates(db, 1, 2, 3, 4, 5)
	if err != nil {
		t.Fatalf("unexpected error fetching index states: %s", err)
	}
	expectedIndexStates = map[int]string{
		1: "completed",
		// 2 was deleted
		// 3 was deleted
		4: "completed",
		5: "completed",
	}
	if diff := cmp.Diff(expectedIndexStates, indexStates); diff != "" {
		t.Errorf("unexpected index states (-want +got):\n%s", diff)
	}
}

type s2 interface {
	Store
	GetStaleSourcedCommits(ctx context.Context, minimumTimeSinceLastCheck time.Duration, limit int, now time.Time) ([]SourcedCommits, error)
	UpdateSourcedCommits(ctx context.Context, repositoryID int, commit string, now time.Time) (int, error)
	DeleteSourcedCommits(ctx context.Context, repositoryID int, commit string, maximumCommitLag time.Duration, now time.Time) (int, int, error)
}

func TestGetStaleSourcedCommits(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db).(s2)

	now := time.Unix(1587396557, 0).UTC()

	insertUploads(t, db,
		shared.Upload{ID: 1, RepositoryID: 50, Commit: makeCommit(1)},
		shared.Upload{ID: 2, RepositoryID: 50, Commit: makeCommit(1), Root: "sub/"},
		shared.Upload{ID: 3, RepositoryID: 51, Commit: makeCommit(4)},
		shared.Upload{ID: 4, RepositoryID: 51, Commit: makeCommit(5)},
		shared.Upload{ID: 5, RepositoryID: 52, Commit: makeCommit(7)},
		shared.Upload{ID: 6, RepositoryID: 52, Commit: makeCommit(8)},
	)

	sourcedCommits, err := store.GetStaleSourcedCommits(context.Background(), time.Minute, 5, now)
	if err != nil {
		t.Fatalf("unexpected error getting stale sourced commits: %s", err)
	}
	expectedCommits := []SourcedCommits{
		{RepositoryID: 50, RepositoryName: "n-50", Commits: []string{makeCommit(1)}},
		{RepositoryID: 51, RepositoryName: "n-51", Commits: []string{makeCommit(4), makeCommit(5)}},
		{RepositoryID: 52, RepositoryName: "n-52", Commits: []string{makeCommit(7), makeCommit(8)}},
	}
	if diff := cmp.Diff(expectedCommits, sourcedCommits); diff != "" {
		t.Errorf("unexpected sourced commits (-want +got):\n%s", diff)
	}

	// 120s away from next check (threshold is 60s)
	if _, err := store.UpdateSourcedCommits(context.Background(), 52, makeCommit(7), now); err != nil {
		t.Fatalf("unexpected error refreshing commit resolvability: %s", err)
	}

	// 30s away from next check (threshold is 60s)
	if _, err := store.UpdateSourcedCommits(context.Background(), 52, makeCommit(8), now.Add(time.Second*90)); err != nil {
		t.Fatalf("unexpected error refreshing commit resolvability: %s", err)
	}

	sourcedCommits, err = store.GetStaleSourcedCommits(context.Background(), time.Minute, 5, now.Add(time.Minute*2))
	if err != nil {
		t.Fatalf("unexpected error getting stale sourced commits: %s", err)
	}
	expectedCommits = []SourcedCommits{
		{RepositoryID: 50, RepositoryName: "n-50", Commits: []string{makeCommit(1)}},
		{RepositoryID: 51, RepositoryName: "n-51", Commits: []string{makeCommit(4), makeCommit(5)}},
		{RepositoryID: 52, RepositoryName: "n-52", Commits: []string{makeCommit(7)}},
	}
	if diff := cmp.Diff(expectedCommits, sourcedCommits); diff != "" {
		t.Errorf("unexpected sourced commits (-want +got):\n%s", diff)
	}
}

func TestUpdateSourcedCommits(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db).(s2)

	now := time.Unix(1587396557, 0).UTC()

	insertUploads(t, db,
		shared.Upload{ID: 1, RepositoryID: 50, Commit: makeCommit(1)},
		shared.Upload{ID: 2, RepositoryID: 50, Commit: makeCommit(1), Root: "sub/"},
		shared.Upload{ID: 3, RepositoryID: 51, Commit: makeCommit(4)},
		shared.Upload{ID: 4, RepositoryID: 51, Commit: makeCommit(5)},
		shared.Upload{ID: 5, RepositoryID: 52, Commit: makeCommit(7)},
		shared.Upload{ID: 6, RepositoryID: 52, Commit: makeCommit(7), State: "uploading"},
	)

	uploadsUpdated, err := store.UpdateSourcedCommits(context.Background(), 50, makeCommit(1), now)
	if err != nil {
		t.Fatalf("unexpected error refreshing commit resolvability: %s", err)
	}
	if uploadsUpdated != 2 {
		t.Fatalf("unexpected uploads updated. want=%d have=%d", 2, uploadsUpdated)
	}

	uploadStates, err := getUploadStates(db, 1, 2, 3, 4, 5, 6)
	if err != nil {
		t.Fatalf("unexpected error fetching upload states: %s", err)
	}
	expectedUploadStates := map[int]string{
		1: "completed",
		2: "completed",
		3: "completed",
		4: "completed",
		5: "completed",
		6: "uploading",
	}
	if diff := cmp.Diff(expectedUploadStates, uploadStates); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}
}

func TestGetQueuedUploadRank(t *testing.T) {
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

	insertUploads(t, db,
		shared.Upload{ID: 1, UploadedAt: t1, State: "queued"},
		shared.Upload{ID: 2, UploadedAt: t2, State: "queued"},
		shared.Upload{ID: 3, UploadedAt: t3, State: "queued"},
		shared.Upload{ID: 4, UploadedAt: t4, State: "queued"},
		shared.Upload{ID: 5, UploadedAt: t5, State: "queued"},
		shared.Upload{ID: 6, UploadedAt: t6, State: "processing"},
		shared.Upload{ID: 7, UploadedAt: t1, State: "queued", ProcessAfter: &t7},
	)

	if upload, _, _ := store.GetUploadByID(context.Background(), 1); upload.Rank == nil || *upload.Rank != 1 {
		t.Errorf("unexpected rank. want=%d have=%s", 1, printableRank{upload.Rank})
	}
	if upload, _, _ := store.GetUploadByID(context.Background(), 2); upload.Rank == nil || *upload.Rank != 6 {
		t.Errorf("unexpected rank. want=%d have=%s", 5, printableRank{upload.Rank})
	}
	if upload, _, _ := store.GetUploadByID(context.Background(), 3); upload.Rank == nil || *upload.Rank != 3 {
		t.Errorf("unexpected rank. want=%d have=%s", 3, printableRank{upload.Rank})
	}
	if upload, _, _ := store.GetUploadByID(context.Background(), 4); upload.Rank == nil || *upload.Rank != 2 {
		t.Errorf("unexpected rank. want=%d have=%s", 2, printableRank{upload.Rank})
	}
	if upload, _, _ := store.GetUploadByID(context.Background(), 5); upload.Rank == nil || *upload.Rank != 4 {
		t.Errorf("unexpected rank. want=%d have=%s", 4, printableRank{upload.Rank})
	}

	// Only considers queued uploads to determine rank
	if upload, _, _ := store.GetUploadByID(context.Background(), 6); upload.Rank != nil {
		t.Errorf("unexpected rank. want=%s have=%s", "nil", printableRank{upload.Rank})
	}

	// Process after takes priority over upload time
	if upload, _, _ := store.GetUploadByID(context.Background(), 7); upload.Rank == nil || *upload.Rank != 5 {
		t.Errorf("unexpected rank. want=%d have=%s", 4, printableRank{upload.Rank})
	}
}

func TestDeleteSourcedCommits(t *testing.T) {
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db).(s2)

	now := time.Unix(1587396557, 0).UTC()

	insertUploads(t, db,
		shared.Upload{ID: 1, RepositoryID: 50, Commit: makeCommit(1)},
		shared.Upload{ID: 2, RepositoryID: 50, Commit: makeCommit(1), Root: "sub/"},
		shared.Upload{ID: 3, RepositoryID: 51, Commit: makeCommit(4)},
		shared.Upload{ID: 4, RepositoryID: 51, Commit: makeCommit(5)},
		shared.Upload{ID: 5, RepositoryID: 52, Commit: makeCommit(7)},
		shared.Upload{ID: 6, RepositoryID: 52, Commit: makeCommit(7), State: "uploading", UploadedAt: now.Add(-time.Minute * 90)},
		shared.Upload{ID: 7, RepositoryID: 52, Commit: makeCommit(7), State: "queued", UploadedAt: now.Add(-time.Minute * 30)},
	)

	uploadsUpdated, uploadsDeleted, err := store.DeleteSourcedCommits(context.Background(), 52, makeCommit(7), time.Hour, now)
	if err != nil {
		t.Fatalf("unexpected error refreshing commit resolvability: %s", err)
	}
	if uploadsUpdated != 1 {
		t.Fatalf("unexpected number of uploads updated. want=%d have=%d", 1, uploadsUpdated)
	}
	if uploadsDeleted != 2 {
		t.Fatalf("unexpected number of uploads deleted. want=%d have=%d", 2, uploadsDeleted)
	}

	uploadStates, err := getUploadStates(db, 1, 2, 3, 4, 5, 6, 7)
	if err != nil {
		t.Fatalf("unexpected error fetching upload states: %s", err)
	}
	expectedUploadStates := map[int]string{
		1: "completed",
		2: "completed",
		3: "completed",
		4: "completed",
		5: "deleting",
		6: "deleted",
		7: "queued",
	}
	if diff := cmp.Diff(expectedUploadStates, uploadStates); diff != "" {
		t.Errorf("unexpected upload states (-want +got):\n%s", diff)
	}
}

func TestDeleteIndexesWithoutRepository(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	var indexes []uploadsshared.Index
	for i := range 25 {
		for range 10 + i {
			indexes = append(indexes, uploadsshared.Index{ID: len(indexes) + 1, RepositoryID: 50 + i})
		}
	}
	insertIndexes(t, db, indexes...)

	t1 := time.Unix(1587396557, 0).UTC()
	t2 := t1.Add(-deletedRepositoryGracePeriod + time.Minute)
	t3 := t1.Add(-deletedRepositoryGracePeriod - time.Minute)

	deletions := map[int]time.Time{
		52: t2, 54: t2, 56: t2, // deleted too recently
		61: t3, 63: t3, 65: t3, // deleted
	}

	for repositoryID, deletedAt := range deletions {
		query := sqlf.Sprintf(`UPDATE repo SET deleted_at=%s WHERE id=%s`, deletedAt, repositoryID)

		if _, err := db.QueryContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
			t.Fatalf("Failed to update repository: %s", err)
		}
	}

	_, count, err := store.DeleteIndexesWithoutRepository(context.Background(), t1)
	if err != nil {
		t.Fatalf("unexpected error deleting indexes: %s", err)
	}
	if expected := 21 + 23 + 25; count != expected {
		t.Fatalf("unexpected count. want=%d have=%d", expected, count)
	}
}

func TestExpireFailedRecords(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	ctx := context.Background()
	now := time.Unix(1587396557, 0).UTC()

	insertIndexes(t, db,
		// young failures (none removed)
		uploadsshared.Index{ID: 1, RepositoryID: 50, Commit: makeCommit(1), FinishedAt: pointers.Ptr(now.Add(-time.Minute * 10)), State: "failed"},
		uploadsshared.Index{ID: 2, RepositoryID: 50, Commit: makeCommit(2), FinishedAt: pointers.Ptr(now.Add(-time.Minute * 20)), State: "failed"},
		uploadsshared.Index{ID: 3, RepositoryID: 50, Commit: makeCommit(3), FinishedAt: pointers.Ptr(now.Add(-time.Minute * 20)), State: "failed"},

		// failures prior to a success (both removed)
		uploadsshared.Index{ID: 4, RepositoryID: 50, Commit: makeCommit(4), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 10)), Root: "foo", State: "completed"},
		uploadsshared.Index{ID: 5, RepositoryID: 50, Commit: makeCommit(5), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 12)), Root: "foo", State: "failed"},
		uploadsshared.Index{ID: 6, RepositoryID: 50, Commit: makeCommit(6), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 14)), Root: "foo", State: "failed"},

		// old failures (one is left for debugging)
		uploadsshared.Index{ID: 7, RepositoryID: 51, Commit: makeCommit(7), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 3)), State: "failed"},
		uploadsshared.Index{ID: 8, RepositoryID: 51, Commit: makeCommit(8), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 4)), State: "failed"},
		uploadsshared.Index{ID: 9, RepositoryID: 51, Commit: makeCommit(9), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 5)), State: "failed"},

		// failures prior to queued uploads (one removed; queued does not reset failures)
		uploadsshared.Index{ID: 10, RepositoryID: 52, Commit: makeCommit(10), Root: "foo", State: "queued"},
		uploadsshared.Index{ID: 11, RepositoryID: 52, Commit: makeCommit(11), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 12)), Root: "foo", State: "failed"},
		uploadsshared.Index{ID: 12, RepositoryID: 52, Commit: makeCommit(12), FinishedAt: pointers.Ptr(now.Add(-time.Hour * 14)), Root: "foo", State: "failed"},
	)

	if _, _, err := store.ExpireFailedRecords(ctx, 100, time.Hour, now); err != nil {
		t.Fatalf("unexpected error expiring failed records: %s", err)
	}

	ids, err := basestore.ScanInts(db.QueryContext(ctx, "SELECT id FROM lsif_indexes"))
	if err != nil {
		t.Fatalf("unexpected error fetching index ids: %s", err)
	}

	expectedIDs := []int{
		1, 2, 3, // none deleted
		4,      // 5, 6 deleted
		7,      // 8, 9 deleted
		10, 11, // 12 deleted
	}
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Errorf("unexpected indexes (-want +got):\n%s", diff)
	}
}

//
//
//

func getIndexStates(db database.DB, ids ...int) (map[int]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	q := sqlf.Sprintf(
		`SELECT id, state FROM lsif_indexes WHERE id IN (%s)`,
		sqlf.Join(intsToQueries(ids), ", "),
	)

	return scanStates(db.QueryContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...))
}
