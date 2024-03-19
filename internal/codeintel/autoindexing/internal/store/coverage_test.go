package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestTopRepositoriesToConfigure(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	insertEvent := func(name string, repositoryID int, maxAge time.Duration) {
		query := `
			INSERT INTO event_logs (name, argument, url, user_id, anonymous_user_id, source, version, timestamp)
			VALUES ($1, $2, '', 0, 'internal', 'test', 'dev', NOW() - ($3 * '1 hour'::interval))
		`
		if _, err := db.ExecContext(ctx, query, name, fmt.Sprintf(`{"repositoryId": %d}`, repositoryID), int(maxAge/time.Hour)); err != nil {
			t.Fatalf("unexpected error inserting events: %s", err)
		}
	}

	for i := range 50 {
		insertRepo(t, db, 50+i, fmt.Sprintf("test%d", i))
	}
	for i := range 10 {
		insertEvent("codeintel.searchHover", 60+i%3, 1)
	}
	for j := range 10 {
		insertEvent("codeintel.searchHover", 70+j, 1)
	}

	insertEvent("codeintel.searchDefinitions", 50, 1)
	insertEvent("codeintel.searchDefinitions", 50, 1)
	insertEvent("codeintel.searchDefinitions.xrepo", 50, 1)
	insertEvent("search.symbol", 50, 1)                               // unmatched name
	insertEvent("codeintel.searchDefinitions", 50, eventLogsWindow*2) // out of window

	repositoriesWithCount, err := store.TopRepositoriesToConfigure(ctx, 7)
	if err != nil {
		t.Fatalf("unexpected error getting top repositories to configure: %s", err)
	}
	expected := []uploadsshared.RepositoryWithCount{
		{RepositoryID: 60, Count: 4}, // i=0,3,6,9
		{RepositoryID: 50, Count: 3}, // manual
		{RepositoryID: 61, Count: 3}, // i=1,4,7
		{RepositoryID: 62, Count: 3}, // i=2,5,8
		{RepositoryID: 70, Count: 1}, // j=0
		{RepositoryID: 71, Count: 1}, // j=1
		{RepositoryID: 72, Count: 1}, // j=2
	}
	if diff := cmp.Diff(expected, repositoriesWithCount); diff != "" {
		t.Errorf("unexpected repositories (-want +got):\n%s", diff)
	}
}

func TestRepositoryIDsWithConfiguration(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)
	store := New(&observation.TestContext, db)

	testIndexerList := map[string]uploadsshared.AvailableIndexer{
		"test-indexer": {
			Roots: []string{"proj1", "proj2", "proj3"},
			Indexer: uploadsshared.CodeIntelIndexer{
				Name: "test-indexer",
			},
		},
	}

	for i := range 20 {
		insertRepo(t, db, 50+i, fmt.Sprintf("test%d", i))

		if err := store.SetConfigurationSummary(ctx, 50+i, i*300, testIndexerList); err != nil {
			t.Fatalf("unexpected error setting configuration summary: %s", err)
		}
	}

	if err := store.TruncateConfigurationSummary(ctx, 10); err != nil {
		t.Fatalf("unexpected error truncating configuration summary: %s", err)
	}

	repositoriesWithCount, totalCount, err := store.RepositoryIDsWithConfiguration(ctx, 0, 5)
	if err != nil {
		t.Fatalf("unexpected error getting repositories with configuration: %s", err)
	}
	if expected := 10; totalCount != expected {
		t.Fatalf("unexpected total number of repositories. want=%d have=%d", expected, totalCount)
	}
	expected := []uploadsshared.RepositoryWithAvailableIndexers{
		{RepositoryID: 69, AvailableIndexers: testIndexerList},
		{RepositoryID: 68, AvailableIndexers: testIndexerList},
		{RepositoryID: 67, AvailableIndexers: testIndexerList},
		{RepositoryID: 66, AvailableIndexers: testIndexerList},
		{RepositoryID: 65, AvailableIndexers: testIndexerList},
	}
	if diff := cmp.Diff(expected, repositoriesWithCount); diff != "" {
		t.Errorf("unexpected repositories (-want +got):\n%s", diff)
	}
}

func TestGetLastIndexScanForRepository(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	ts, err := store.GetLastIndexScanForRepository(ctx, 50)
	if err != nil {
		t.Fatalf("unexpected error querying last index scan: %s", err)
	}
	if ts != nil {
		t.Fatalf("unexpected timestamp for repository. want=%v have=%s", nil, ts)
	}

	expected := time.Unix(1587396557, 0).UTC()

	if err := basestore.NewWithHandle(db.Handle()).Exec(ctx, sqlf.Sprintf(`
		INSERT INTO lsif_last_index_scan (repository_id, last_index_scan_at)
		VALUES (%s, %s)
	`, 50, expected)); err != nil {
		t.Fatalf("unexpected error inserting timestamp: %s", err)
	}

	ts, err = store.GetLastIndexScanForRepository(ctx, 50)
	if err != nil {
		t.Fatalf("unexpected error querying last index scan: %s", err)
	}

	if ts == nil || !ts.Equal(expected) {
		t.Fatalf("unexpected timestamp for repository. want=%s have=%s", expected, ts)
	}
}
