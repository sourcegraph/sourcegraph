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

/*
This set of assertions verifies that repo scheduling logic works with syntactic
indexing in isolation - i.e. we assume that there is no overlapping precise/syntactic scheduling running.
For that we have a separate test.
*/
func TestSelectRepositoriesForSyntacticIndexing(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	store := testSyntacticStoreWithoutConfigurationPolicies(t, db)

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
			syntactic_indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
	        --                                                         indexing |      | syntactic indexing
	        --                                                                  v      v
			(101, 50, 'policy 1', 'GIT_COMMIT', 'HEAD', null, true, 0, false, false, true,  0, false),
			(102, 51, 'policy 2', 'GIT_COMMIT', 'HEAD', null, true, 0, false, false, true,  0, false),
			(103, 52, 'policy 3', 'GIT_TREE',   'ef/',  null, true, 0, false, false, true,  0, false),
			(104, 53, 'policy 4', 'GIT_TREE',   'gh/',  null, true, 0, false, false, true,  0, false),
			(105, 54, 'policy 5', 'GIT_TREE',   'gh/',  null, true, 0, false, false, false, 0, false)
	`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// Can return null last_index_scan
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 2), now, []int{50, 51})

	// 20 minutes later, first two repositories are still on cooldown
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*20), []int{52, 53})

	// 30 minutes later, all repositories are still on cooldown
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*30), []int(nil))

	// 90 minutes later, all repositories are visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*89), []int{50, 51, 52, 53})

	// Make new invisible repository
	insertRepo(t, db, 54, "r4")

	// 95 minutes later, new repository is not yet visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*95), []int(nil))

	query = `UPDATE lsif_configuration_policies SET syntactic_indexing_enabled = true WHERE id = 105`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// 100 minutes later, only new repository is visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*100), []int{54})

	// 110 minutes later, nothing is ready to go (too close to last index scan)
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110), []int(nil))

	// Update repo 50 (GIT_COMMIT/HEAD policy), and 51 (GIT_TREE policy)
	gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s WHERE repo_id IN (50, 52)`, now.Add(time.Minute*105))
	if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
	}

	// 110 minutes later, updated repositories are ready for re-indexing
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110), []int{50})
}

/*
This set of assertions verifies that repo scheduling logic works with syntactic
indexing in isolation - i.e. we assume that there is no overlapping precise/syntactic scheduling running.
For that we have a separate test.
*/
func TestSelectRepositoriesForPreciseIndexing(t *testing.T) {
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
			syntactic_indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
	        --                                                         indexing |      | syntactic indexing
	        --                                                                  v      v
			(101, 50, 'policy 1', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  false,  0, false),
			(102, 51, 'policy 2', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  false,  0, false),
			(103, 52, 'policy 3', 'GIT_TREE',   'ef/',  null, true, 0, false, true,  false,  0, false),
			(104, 53, 'policy 4', 'GIT_TREE',   'gh/',  null, true, 0, false, true,  false,  0, false),
			(105, 54, 'policy 5', 'GIT_TREE',   'gh/',  null, true, 0, false, false, false, 0, false)
	`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// Can return null last_index_scan
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 2), now, []int{50, 51})

	// 20 minutes later, first two repositories are still on cooldown
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*20), []int{52, 53})

	// 30 minutes later, all repositories are still on cooldown
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*30), []int(nil))

	// 90 minutes later, all repositories are visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*89), []int{50, 51, 52, 53})

	// Make new invisible repository
	insertRepo(t, db, 54, "r4")

	// 95 minutes later, new repository is not yet visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*95), []int(nil))

	query = `UPDATE lsif_configuration_policies SET indexing_enabled = true WHERE id = 105`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	// 100 minutes later, only new repository is visible
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*100), []int{54})

	// 110 minutes later, nothing is ready to go (too close to last index scan)
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110), []int(nil))

	// Update repo 50 (GIT_COMMIT/HEAD policy), and 51 (GIT_TREE policy)
	gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s WHERE repo_id IN (50, 52)`, now.Add(time.Minute*105))
	if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
		t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
	}

	// 110 minutes later, updated repositories are ready for re-indexing
	assertRepoList(t, store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110), []int{50})
}

func TestSelectRepositoriesForIndexScanWithGlobalPolicy(t *testing.T) {
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
			syntactic_indexing_enabled,
			index_commit_max_age_hours,
			index_intermediate_commits
		) VALUES
	        --                                                       indexing |     | syntactic indexing
	        --                                                                v     v
			(101, NULL, 'policy 1', 'GIT_TREE', 'ab/', null, true, 0, false, true, true, 0, false)
	`
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("unexpected error while inserting configuration policies: %s", err)
	}

	var tests = []struct {
		name  string
		store RepositorySchedulingStore
	}{
		{"precise store", preciseStore},
		{"syntactic store", syntacticStore},
	}

	// Returns at most configured limit
	limit := 2

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Returns nothing when disabled
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, false, nil, 100), now, []int(nil))

			// Can return null last_index_scan
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, true, &limit, 100), now, []int{50, 51})

			// 20 minutes later, first two repositories are still on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*20), []int{52, 53})

			// 30 minutes later, all repositories are still on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*30), []int(nil))

			// 90 minutes later, all repositories are visible
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*90), []int{50, 51, 52, 53})

		})
	}

}

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

func assertRepoList(t *testing.T, store RepositorySchedulingStore, batchOptions RepositoryBatchOptions, now time.Time, want []int) {
	t.Helper()
	wantedRepos := make([]RepositoryToIndex, len(want))
	for i, repoId := range want {
		wantedRepos[i] = RepositoryToIndex{ID: repoId}
	}
	if repositories, err := store.GetRepositoriesForIndexScan(context.Background(), batchOptions, now); err != nil {
		t.Fatalf("unexpected error fetching repositories for index scan: %s", err)
	} else if diff := cmp.Diff(wantedRepos, repositories); diff != "" {
		t.Fatalf("unexpected repository list (-want +got):\n%s", diff)
	}
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
