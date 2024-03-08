package reposcheduler

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
)

func TestSelectRepositoriesForIndexScan(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	preciseStore := testPreciseStoreWithoutConfigurationPolicies(t, db)
	syntacticStore := testSyntacticStoreWithoutConfigurationPolicies(t, db)

	now := timeutil.Now()
	insertRepo(t, db, 50, "r0")
	insertRepo(t, db, 51, "r1")
	insertRepo(t, db, 52, "r2")
	insertRepo(t, db, 53, "r3")
	updateGitserverUpdatedAt(t, db, now)

	query := `
		INSERT INTO lsif_configuration_policies (
			id,
			repository_id,
			name,
			type,
			pattern,
			repository_patterns,
			retention_enabled,
			retention_duration_hours,
			retain_intermediate_commits,
			indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
			(101, 50, 'policy 1', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  0, false),
			(102, 51, 'policy 2', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  0, false),
			(103, 52, 'policy 3', 'GIT_TREE',   'ef/',  null, true, 0, false, true,  0, false),
			(104, 53, 'policy 4', 'GIT_TREE',   'gh/',  null, true, 0, false, true,  0, false),
			(105, 54, 'policy 5', 'GIT_TREE',   'gh/',  null, true, 0, false, false, 0, false)
	`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	defaultOptions := NewBatchOptions(time.Hour, true, nil, 100)
	repos, _ := syntacticStore.GetRepositoriesForIndexScan(context.Background(), defaultOptions, now)
	fmt.Printf("repos: %v\n", repos)

	// Can return null last_index_scan
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 2), now); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{50, 51}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 20 minutes later, first two repositories are still on cooldown
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*20)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{52, 53}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 30 minutes later, all repositories are still on cooldown
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*30)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int(nil), repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 90 minutes later, all repositories are visible
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*90)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{50, 51, 52, 53}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// Make new invisible repository
	insertRepo(t, db, 54, "r4")

	// 95 minutes later, new repository is not yet visible
	if repositoryIDs, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*95)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int(nil), repositoryIDs); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	query = `UPDATE lsif_configuration_policies SET indexing_enabled = true WHERE id = 105`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// 100 minutes later, only new repository is visible
	if repositoryIDs, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*100)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{54}, repositoryIDs); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 110 minutes later, nothing is ready to go (too close to last index scan)
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int(nil), repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// Update repo 50 (GIT_COMMIT/HEAD policy), and 51 (GIT_TREE policy)
	gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s WHERE repo_id IN (50, 52)`, now.Add(time.Minute*105))
	if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
	}

	// 110 minutes later, updated repositories are ready for re-indexing
	if repositories, err := preciseStore.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{50}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}
}

func TestSelectRepositoriesForIndexScanWithGlobalPolicy(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testPreciseStoreWithoutConfigurationPolicies(t, db)

	now := timeutil.Now()
	insertRepo(t, db, 50, "r0")
	insertRepo(t, db, 51, "r1")
	insertRepo(t, db, 52, "r2")
	insertRepo(t, db, 53, "r3")
	updateGitserverUpdatedAt(t, db, now)

	query := `
		INSERT INTO lsif_configuration_policies (
			id,
			repository_id,
			name,
			type,
			pattern,
			repository_patterns,
			retention_enabled,
			retention_duration_hours,
			retain_intermediate_commits,
			indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
			(101, NULL, 'policy 1', 'GIT_TREE', 'ab/', null, true, 0, false, true, 0, false)
	`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// Returns nothing when disabled
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, false, nil, 100), now); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int(nil), repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// Returns at most configured limit
	limit := 2

	// Can return null last_index_scan
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, &limit, 100), now); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{50, 51}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 20 minutes later, first two repositories are still on cooldown
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*20)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{52, 53}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 30 minutes later, all repositories are still on cooldown
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(), NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*30)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int(nil), repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}

	// 90 minutes later, all repositories are visible
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(),
		NewBatchOptions(time.Hour, true, nil, 100),
		now.Add(time.Minute*90)); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff([]int{50, 51, 52, 53}, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}
}

// func TestMarkRepoRevsAsProcessed(t *testing.T) {
// 	ctx := context.Background()
// 	logger := logtest.Scoped(t)
// 	db := database.NewDB(logger, dbtest.NewDB(t))
// 	store := NewPreciseStore(&observation.TestContext, db)

// 	expected := []RepoRev{
// 		{1, 50, "HEAD"},
// 		{2, 50, "HEAD~1"},
// 		{3, 50, "HEAD~2"},
// 		{4, 51, "HEAD"},
// 		{5, 51, "HEAD~1"},
// 		{6, 51, "HEAD~2"},
// 		{7, 52, "HEAD"},
// 		{8, 52, "HEAD~1"},
// 		{9, 52, "HEAD~2"},
// 	}
// 	for _, repoRev := range expected {
// 		if err := store.QueueRepoRev(ctx, repoRev.RepositoryID, repoRev.Rev); err != nil {
// 			t.Fatalf("unexpected error: %s", err)
// 		}
// 	}

// 	// entire set
// 	repoRevs, err := store.GetQueuedRepoRev(ctx, 50)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}
// 	if diff := cmp.Diff(expected, repoRevs); diff != "" {
// 		t.Errorf("unexpected repo revs (-want +got):\n%s", diff)
// 	}

// 	// mark first elements as complete; re-request remaining
// 	if err := store.MarkRepoRevsAsProcessed(ctx, []int{1, 2, 3, 4, 5}); err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}
// 	repoRevs, err = store.GetQueuedRepoRev(ctx, 50)
// 	if err != nil {
// 		t.Fatalf("unexpected error: %s", err)
// 	}
// 	if diff := cmp.Diff(expected[5:], repoRevs); diff != "" {
// 		t.Errorf("unexpected repo revs (-want +got):\n%s", diff)
// 	}
// }

//
//

// removes default configuration policies
func testPreciseStoreWithoutConfigurationPolicies(t *testing.T, db database.DB) RepositorySchedulingStore {
	if _, err := db.ExecContext(context.Background(), `TRUNCATE lsif_configuration_policies`); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	return NewPreciseStore(&observation.TestContext, db)
}

func testSyntacticStoreWithoutConfigurationPolicies(t *testing.T, db database.DB) RepositorySchedulingStore {
	if _, err := db.ExecContext(context.Background(), `TRUNCATE lsif_configuration_policies`); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	return NewSyntacticStore(&observation.TestContext, db)
}

func updateGitserverUpdatedAt(t *testing.T, db database.DB, now time.Time) {
	gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s`, now.Add(-time.Hour*24))
	if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
	}
}

func insertRepo(t testing.TB, db database.DB, id int, name string) {
	if name == "" {
		name = fmt.Sprintf("n-%d", id)
	}

	deletedAt := sqlf.Sprintf("NULL")
	if strings.HasPrefix(name, "DELETED-") {
		deletedAt = sqlf.Sprintf("%s", time.Unix(1587396557, 0).UTC())
	}
	insertRepoQuery := sqlf.Sprintf(
		`INSERT INTO repo (id, name, deleted_at, private) VALUES (%s, %s, %s, %s) ON CONFLICT (id) DO NOTHING`,
		id,
		name,
		deletedAt,
		false,
	)
	if _, err := db.ExecContext(context.Background(), insertRepoQuery.Query(sqlf.PostgresBindVar), insertRepoQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upserting repository: %s", err)
	}

	status := "cloned"
	if strings.HasPrefix(name, "DELETED-") {
		status = "not_cloned"
	}
	updateGitserverRepoQuery := sqlf.Sprintf(
		`UPDATE gitserver_repos SET clone_status = %s WHERE repo_id = %s`,
		status,
		id,
	)
	if _, err := db.ExecContext(context.Background(), updateGitserverRepoQuery.Query(sqlf.PostgresBindVar), updateGitserverRepoQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upserting gitserver repository: %s", err)
	}
}
