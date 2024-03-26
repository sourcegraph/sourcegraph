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

// Helper function to setup a configuration policies table
// for either precise or syntactic indexing exclusively
func databaseSetupQuery(enabledColumn string) string {
	var disabledColumn string
	if enabledColumn == "indexing_enabled" {
		disabledColumn = "syntactic_indexing_enabled"
	} else {

		disabledColumn = "indexing_enabled"
	}

	base := `
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
		%s, -- enabled column
		%s, -- disabled column
		index_commit_max_age_hours,
		index_intermediate_commits
	) VALUES
		--                                                          enabled |       | disabled
		--                                                                  v      v
		(101, 50, 'policy 1', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  false,  0, false),
		(102, 51, 'policy 2', 'GIT_COMMIT', 'HEAD', null, true, 0, false, true,  false,  0, false),
		(103, 52, 'policy 3', 'GIT_TREE',   'ef/',  null, true, 0, false, true,  false,  0, false),
		(104, 53, 'policy 4', 'GIT_TREE',   'gh/',  null, true, 0, false, true,  false,  0, false),
		(105, 54, 'policy 5', 'GIT_TREE',   'gh/',  null, true, 0, false, false, false, 0, false)

`

	return fmt.Sprintf(base, enabledColumn, disabledColumn)
}

func TestSelectRepositoriesForSyntacticIndexing(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	preciseStore := testPreciseStoreWithoutConfigurationPolicies(t, db)
	syntacticStore := testSyntacticStoreWithoutConfigurationPolicies(t, db)

	insertRepo(t, db, 50, "r0")
	insertRepo(t, db, 51, "r1")
	insertRepo(t, db, 52, "r2")
	insertRepo(t, db, 53, "r3")

	var tests = []struct {
		name          string
		store         RepositorySchedulingStore
		enabledColumn string
	}{
		{"precise store", preciseStore, "indexing_enabled"},
		{"syntactic store", syntacticStore, "syntactic_indexing_enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if _, err := db.ExecContext(context.Background(), "TRUNCATE lsif_configuration_policies"); err != nil {
				t.Fatalf("unexpected error while truncating configuration policies: %s", err)
			}
			setupQuery := databaseSetupQuery(tt.enabledColumn)
			if _, err := db.ExecContext(context.Background(), setupQuery); err != nil {
				t.Fatalf("unexpected error while inserting configuration policies: %s", err)
			}

			now := timeutil.Now()
			updateGitserverUpdatedAt(t, db, now)

			// The following tests simulate the passage of time (i.e. repeated scheduled invocations of the repo scheduling logic)
			// T is time

			// N.B. We use 1 hour process delay in all those tests,
			// which means that IF the repository id was returned, it will be put on cooldown for an hour

			// T = 0: No records in last index scan table, so we return all repos permitted by the limit parameter
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 2), now, []int{50, 51})

			// T + 20 minutes: first two repositories are still on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*20), []int{52, 53})

			// T + 30 minutes: all repositories are on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*30), []int(nil))

			// T + 90 minutes: all repositories are visible again
			// Repos 50, 51 are visible because they were scheduled at (T + 0) - which is more than 1 hour ago
			// Repos 52, 53 are visible because they were scheduled at (T + 20) - which is more than 1 hour ago
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*90), []int{50, 51, 52, 53})

			// Make a new repository, not yet covered by the configuration policies
			insertRepo(t, db, 54, "r4")

			// T + 95: no repositories are visible
			// Repos 50,51,52,53 are invisible because they were scheduled at (T + 90), which is less than 1 hour ago
			// Repo 54 is invisible because it doesn't have `indexing_enabled=true` in the configuration policies table
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*95), []int(nil))

			// Explicitly enable indexing for repo 54
			query := fmt.Sprintf(`UPDATE lsif_configuration_policies SET %s = true WHERE id = 105`, tt.enabledColumn)
			if _, err := db.ExecContext(context.Background(), query); err != nil {
				t.Fatalf("unexpected error while inserting configuration policies: %s", err)
			}

			// T + 100: only repository 54 is visible
			// Repos 50-53 are still on cooldown as they were scheduled at (T + 90), less than 1 hour ago
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*100), []int{54})

			// T + 110: no repositories are visible
			// Repos 50,51,52,53 are invisible because they were scheduled at (T + 90), which is less than 1 hour ago
			// Repo 54 is invisible because it was scheduled at (T + 100), which is less than 1 hour ago
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*110), []int(nil))

			// Update repo 50 (GIT_COMMIT/HEAD policy), and 51 (GIT_TREE policy)
			gitserverReposQuery := sqlf.Sprintf(`UPDATE gitserver_repos SET last_changed = %s WHERE repo_id IN (50, 52)`, now.Add(time.Minute*105))
			if _, err := db.ExecContext(context.Background(), gitserverReposQuery.Query(sqlf.PostgresBindVar), gitserverReposQuery.Args()...); err != nil {
				t.Fatalf("unexpected error while upodating gitserver_repos last updated time: %s", err)
			}

			// T + 110: only repository 50 is visible
			// Repos 51-54 are invisible because they were scheduled less than 1 hour ago
			// Repo 50 is visible despite it being scheduled less than 1 hour ago - it was recently updated, so that takes precedence
			assertRepoList(t, tt.store, NewBatchOptions(time.Hour, true, nil, 100), now.Add(time.Minute*110), []int{50})
		})
	}
}

/*

This test verifies that repository scheduling works when there's only a single global policy
*/

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
			// The following tests simulate the passage of time (i.e. repeated scheduled invocations of the repo scheduling logic)
			// T is time

			// N.B. We use 1 hour process delay in all those tests,
			// which means that IF the repository id was returned, it will be put on cooldown for an hour

			// Returns nothing when disabled
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, false, nil, 100), now, []int(nil))

			// T = 0: No records in last index scan table, so we return all repos permitted by the limit parameter
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, &limit, 100), now, []int{50, 51})

			// T + 20 minutes: first two repositories are still on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*20), []int{52, 53})

			// T + 30 minutes: all repositories are on cooldown
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*30), []int(nil))

			// T + 90 minutes: all repositories are visible again
			// Repos 50, 51 are visible because they were scheduled at (T + 0) - which is more than 1 hour ago
			// Repos 52, 53 are visible because they were scheduled at (T + 20) - which is more than 1 hour ago
			assertRepoList(t, tt.store, NewBatchOptions(1*time.Hour, true, nil, 100), now.Add(time.Minute*90), []int{50, 51, 52, 53})

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
