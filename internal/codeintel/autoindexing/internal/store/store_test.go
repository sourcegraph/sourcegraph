package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestMarkRepoRevsAsProcessed(t *testing.T) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, db)

	expected := []RepoRev{
		{1, 50, "HEAD"},
		{2, 50, "HEAD~1"},
		{3, 50, "HEAD~2"},
		{4, 51, "HEAD"},
		{5, 51, "HEAD~1"},
		{6, 51, "HEAD~2"},
		{7, 52, "HEAD"},
		{8, 52, "HEAD~1"},
		{9, 52, "HEAD~2"},
	}
	for _, repoRev := range expected {
		if err := store.QueueRepoRev(ctx, repoRev.RepositoryID, repoRev.Rev); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	// entire set
	repoRevs, err := store.GetQueuedRepoRev(ctx, 50)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expected, repoRevs); diff != "" {
		t.Errorf("unexpected repo revs (-want +got):\n%s", diff)
	}

	// mark first elements as complete; re-request remaining
	if err := store.MarkRepoRevsAsProcessed(ctx, []int{1, 2, 3, 4, 5}); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	repoRevs, err = store.GetQueuedRepoRev(ctx, 50)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if diff := cmp.Diff(expected[5:], repoRevs); diff != "" {
		t.Errorf("unexpected repo revs (-want +got):\n%s", diff)
	}
}

// removes default configuration policies
func testStoreWithoutConfigurationPolicies(t *testing.T, db database.DB) Store {
	if _, err := db.ExecContext(context.Background(), `TRUNCATE lsif_configuration_policies`); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	return New(&observation.TestContext, db)
}

func updateGitserverUpdatedAt(t *testing.T, db database.DB, now time.Time) {
	gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s`, now.Add(-time.Hour*24))
	if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
	}
}
