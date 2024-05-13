package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestRepoStatistics(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	createTestRepos(ctx, t, db, repos)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, NotCloned: 6, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: "", Total: 6, NotCloned: 6},
	})

	// Move to shards[0] as cloning
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, NotCloned: 4, Cloning: 2,
	}, []GitserverReposStatistic{
		{ShardID: "", Total: 4, NotCloned: 4},
		{ShardID: shards[0], Total: 2, Cloning: 2},
	})

	// Move two repos to shards[1] as cloning
	setCloneStatus(t, db, repos[2].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[3].Name, shards[1], types.CloneStatusCloning)
	// Move two repos to shards[2] as cloning
	setCloneStatus(t, db, repos[4].Name, shards[2], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloning)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, Cloning: 6,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2},
		{ShardID: shards[1], Total: 2, Cloning: 2},
		{ShardID: shards[2], Total: 2, Cloning: 2},
	})

	// Move from shards[0] to shards[2] and change status
	setCloneStatus(t, db, repos[2].Name, shards[2], types.CloneStatusCloned)
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, Cloning: 5, Cloned: 1,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2},
		{ShardID: shards[1], Total: 1, Cloning: 1},
		{ShardID: shards[2], Total: 3, Cloning: 2, Cloned: 1},
	})

	// Soft delete repos
	if err := db.Repos().Delete(ctx, repos[2].ID); err != nil {
		t.Fatal(err)
	}
	deletedRepoName := queryRepoName(t, ctx, s, repos[2].ID)

	// Deletion is reflected in repoStatistics
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 5, SoftDeleted: 1, Cloning: 5,
	}, []GitserverReposStatistic{
		// But gitserverReposStatistics is unchanged
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2},
		{ShardID: shards[1], Total: 1, Cloning: 1},
		{ShardID: shards[2], Total: 3, Cloning: 2, Cloned: 1},
	})

	// Until we remove it from disk in gitserver, which causes the clone status
	// to be set to not_cloned:
	setCloneStatus(t, db, deletedRepoName, shards[2], types.CloneStatusNotCloned)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// Global stats are unchanged
		Total: 5, SoftDeleted: 1, Cloning: 5,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2},
		{ShardID: shards[1], Total: 1, Cloning: 1},
		// But now it's reflected as NotCloned
		{ShardID: shards[2], Total: 3, Cloning: 2, NotCloned: 1},
	})

	// Now we set errors on 2 non-deleted repositories
	setLastError(t, db, repos[0].Name, shards[0], "internet broke repo-1")
	setLastError(t, db, repos[4].Name, shards[2], "internet broke repo-3")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// Only FailedFetch changed
		Total: 5, SoftDeleted: 1, Cloning: 5, FailedFetch: 2,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[1], Total: 1, Cloning: 1, FailedFetch: 0},
		{ShardID: shards[2], Total: 3, Cloning: 2, NotCloned: 1, FailedFetch: 1},
	})
	// Now we move a repo and set an error
	setLastError(t, db, repos[1].Name, shards[1], "internet broke repo-2")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// Only FailedFetch changed
		Total: 5, SoftDeleted: 1, Cloning: 5, FailedFetch: 3,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 1, Cloning: 1, FailedFetch: 1},
		{ShardID: shards[1], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[2], Total: 3, Cloning: 2, NotCloned: 1, FailedFetch: 1},
	})

	// Two repos got cloned again
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloned)
	setCloneStatus(t, db, repos[1].Name, shards[1], types.CloneStatusCloned)
	// One repo gets corrupted
	logCorruption(t, db, repos[1].Name, shards[1], "internet corrupted repo")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// Total, Cloning changed. Added Cloned and Corrupted
		Total: 5, SoftDeleted: 1, Cloned: 2, Cloning: 3, FailedFetch: 3, Corrupted: 1,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 1, Cloned: 1, Cloning: 0, FailedFetch: 1, NotCloned: 0, Corrupted: 0},
		{ShardID: shards[1], Total: 2, Cloned: 1, Cloning: 1, FailedFetch: 1, NotCloned: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 3, Cloned: 0, Cloning: 2, FailedFetch: 1, NotCloned: 1, Corrupted: 0},
	})
	// Another repo gets corrupted!
	logCorruption(t, db, repos[0].Name, shards[0], "corrupted! the internet is unhinged")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// Only Corrupted changed
		Total: 5, SoftDeleted: 1, Cloned: 2, Cloning: 3, FailedFetch: 3, Corrupted: 2,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 1, Cloned: 1, Cloning: 0, FailedFetch: 1, NotCloned: 0, Corrupted: 1},
		{ShardID: shards[1], Total: 2, Cloned: 1, Cloning: 1, FailedFetch: 1, NotCloned: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 3, Cloned: 0, Cloning: 2, FailedFetch: 1, NotCloned: 1, Corrupted: 0},
	})
}

func TestRepoStatistics_RecloneAndCorruption(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
	}

	createTestRepos(ctx, t, db, repos)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, NotCloned: 2, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: "", Total: 2, NotCloned: 2},
	})
	// Repos start cloning, all onto shard-1
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloning: 2, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 2, Cloning: 2},
	})
	// Cloning complete
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloned)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloned)
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloned: 2, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 2, Cloned: 2},
	})
	// both repos get corrupted
	logCorruption(t, db, repos[0].Name, shards[0], "shard-1 corruption")
	logCorruption(t, db, repos[1].Name, shards[0], "shard-1 corruption")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloned: 2, SoftDeleted: 0, Corrupted: 2,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 2, Cloned: 2, Corrupted: 2},
	})
	// We reclone repo 0 on shard-1 and repo-1 on shard-2
	// Why don't we set the status directly to cloned? A status update requires
	// the status to be distinct from the current status
	//
	// Corrupted should now be 0 for all shards
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[1], types.CloneStatusCloning)
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloning: 2, SoftDeleted: 0, Corrupted: 0,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 1, Cloning: 1, Corrupted: 0},
		{ShardID: "shard-2", Total: 1, Cloning: 1, Corrupted: 0},
	})
	// Done cloning!
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloned)
	setCloneStatus(t, db, repos[1].Name, shards[1], types.CloneStatusCloned)
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloned: 2, SoftDeleted: 0, Corrupted: 0,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 1, Cloned: 1, Corrupted: 0},
		{ShardID: "shard-2", Total: 1, Cloned: 1, Corrupted: 0},
	})
	// Repo 1 now gets corrupted AGAIN on shard-2
	logCorruption(t, db, repos[1].Name, shards[1], "shard-2 corruption")
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 2, Cloned: 2, SoftDeleted: 0, Corrupted: 1,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: "shard-1", Total: 1, Cloned: 1, Corrupted: 0},
		{ShardID: "shard-2", Total: 1, Cloned: 1, Corrupted: 1},
	})
}

func TestRepoStatistics_DeleteAndUndelete(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	createTestRepos(ctx, t, db, repos)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, NotCloned: 6, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: "", Total: 6, NotCloned: 6},
	})

	// Move to to shards[0] as cloning
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[2].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[3].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[4].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[5].Name, shards[0], types.CloneStatusCloning)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, Cloning: 6,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 6, Cloning: 6},
	})

	// Soft delete repos
	if err := db.Repos().Delete(ctx, repos[2].ID); err != nil {
		t.Fatal(err)
	}
	deletedRepoName := queryRepoName(t, ctx, s, repos[2].ID)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		// correct
		Total: 5, SoftDeleted: 1, Cloning: 5,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 6, Cloning: 6},
	})

	// Until we remove it from disk in gitserver, which causes the clone status
	// to be set to not_cloned:
	setCloneStatus(t, db, deletedRepoName, shards[0], types.CloneStatusNotCloned)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 5, SoftDeleted: 1, Cloning: 5,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 6, Cloning: 5, NotCloned: 1},
	})

	// Undelete it
	err := s.Exec(ctx, sqlf.Sprintf("UPDATE repo SET deleted_at = NULL WHERE name = %s;", deletedRepoName))
	if err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}
	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, Cloning: 5, NotCloned: 1,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 6, Cloning: 5, NotCloned: 1},
	})

	// reshard and clone
	setCloneStatus(t, db, deletedRepoName, shards[1], types.CloneStatusCloning)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, SoftDeleted: 0, Cloning: 6, NotCloned: 0,
	}, []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 5, Cloning: 5, NotCloned: 0},
		{ShardID: shards[1], Total: 1, Cloning: 1, NotCloned: 0},
	})
}

func TestRepoStatistics_AvoidZeros(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	createTestRepos(ctx, t, db, repos)

	assertRepoStatistics(t, ctx, s, RepoStatistics{
		Total: 6, NotCloned: 6, SoftDeleted: 0,
	}, []GitserverReposStatistic{
		{ShardID: "", Total: 6, NotCloned: 6},
	})

	wantCount := 2 // initial row and then the 6 repos
	if count := queryRepoStatisticsCount(t, ctx, s); count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// Update a repo row, which should _not_ affect the statistics
	err := s.Exec(ctx, sqlf.Sprintf("UPDATE repo SET updated_at = now() WHERE id = %s;", repos[0].ID))
	if err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}

	// Count should stay the same
	if count := queryRepoStatisticsCount(t, ctx, s); count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// Update a gitserver_repos row, which should _not_ affect the statistics
	err = s.Exec(ctx, sqlf.Sprintf("UPDATE gitserver_repos SET updated_at = now() WHERE repo_id = %s;", repos[0].ID))
	if err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}

	// Count should stay the same
	if count := queryRepoStatisticsCount(t, ctx, s); count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}
}

func TestRepoStatistics_Compaction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	// Empty table, should work, even though we should always have a row in
	// there with 0 values.
	if err := s.Exec(ctx, sqlf.Sprintf("DELETE FROM repo_statistics")); err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}
	if err := s.CompactRepoStatistics(ctx); err != nil {
		t.Fatalf("CompactRepoStatistics failed: %s", err)
	}

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	// Trigger 10 insertions into repo_statistics table:
	createTestRepos(ctx, t, db, repos)
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[2].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[3].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[4].Name, shards[2], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloning)
	setLastError(t, db, repos[0].Name, shards[0], "internet broke repo-1")
	setLastError(t, db, repos[4].Name, shards[2], "internet broke repo-5")
	logCorruption(t, db, repos[2].Name, shards[1], "runaway corruption repo-3")
	// Safety check that the counts are right:
	wantRepoStatistics := RepoStatistics{
		Total: 6, Cloning: 6, FailedFetch: 2, Corrupted: 1,
	}
	wantGitserverReposStatistics := []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[1], Total: 2, Cloning: 2, FailedFetch: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 2, Cloning: 2, FailedFetch: 1},
	}
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)

	// The initial insert in the migration also added a row, which means we want:
	wantCount := 11
	count := queryRepoStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// Now we compact the rows into a single row:
	if err := s.CompactRepoStatistics(ctx); err != nil {
		t.Fatalf("GetRepoStatistics failed: %s", err)
	}

	// We should be left with 1 row
	wantCount = 1
	count = queryRepoStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// And counts should still be the same
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)

	// Safety check: add another event and make sure row count goes up again
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloned)
	wantCount = 2
	count = queryRepoStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}
}

func TestGitserverReposStatistics_Compaction(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	// Empty table, should work, even though we should always have a row in
	// there with 0 values.
	if err := s.Exec(ctx, sqlf.Sprintf("DELETE FROM gitserver_repos_statistics")); err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}
	if err := s.CompactGitserverReposStatistics(ctx); err != nil {
		t.Fatalf("CompactGitserverReposStatistics failed: %s", err)
	}

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	// Trigger 21 insertions into gitserver_repos_statistics table:
	// 6 repos added: 6 rows
	createTestRepos(ctx, t, db, repos)
	// 6 clone status changes and shard assignments: 12 rows
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[2].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[3].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[4].Name, shards[2], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloning)
	// 2 errors: 2 rows
	setLastError(t, db, repos[0].Name, shards[0], "internet broke repo-1")
	setLastError(t, db, repos[4].Name, shards[2], "internet broke repo-5")
	// 1 corruption: 1 row
	logCorruption(t, db, repos[2].Name, shards[1], "runaway corruption repo-3")

	// Safety check that the counts are right:
	wantRepoStatistics := RepoStatistics{Total: 6, Cloning: 6, FailedFetch: 2, Corrupted: 1}
	wantGitserverReposStatistics := []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[1], Total: 2, Cloning: 2, FailedFetch: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 2, Cloning: 2, FailedFetch: 1},
	}
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)

	wantCount := 21
	count := queryGitserverReposStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// Now we compact the rows into a single row:
	if err := s.CompactGitserverReposStatistics(ctx); err != nil {
		t.Fatalf("GetRepoStatistics failed: %s", err)
	}

	// We should be left with 4 rows: one for each shard, plus empty shard
	wantCount = 4
	count = queryGitserverReposStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}

	// And counts should still be the same
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)

	// Safety check: add another event and make sure row count goes up again
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloned)
	wantCount = 5
	count = queryGitserverReposStatisticsCount(t, ctx, s)
	if count != wantCount {
		t.Fatalf("wrong statistics count. have=%d, want=%d", count, wantCount)
	}
}

func TestReposStatisticsStore_DeleteAndRecreateStatistics(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	logger := logtest.Scoped(t)
	db := NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()
	s := &repoStatisticsStore{Store: basestore.NewWithHandle(db.Handle())}

	// Safety check: make sure it works with empty table
	err := s.DeleteAndRecreateStatistics(ctx)
	require.NoError(t, err)

	shards := []string{
		"shard-1",
		"shard-2",
		"shard-3",
	}
	repos := types.Repos{
		&types.Repo{Name: "repo1"},
		&types.Repo{Name: "repo2"},
		&types.Repo{Name: "repo3"},
		&types.Repo{Name: "repo4"},
		&types.Repo{Name: "repo5"},
		&types.Repo{Name: "repo6"},
	}

	// Trigger 21 insertions into gitserver_repos_statistics table:
	// 6 repos added: 6 rows
	createTestRepos(ctx, t, db, repos)
	// 6 clone status changes and shard assignments: 12 rows
	setCloneStatus(t, db, repos[0].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[1].Name, shards[0], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[2].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[3].Name, shards[1], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[4].Name, shards[2], types.CloneStatusCloning)
	setCloneStatus(t, db, repos[5].Name, shards[2], types.CloneStatusCloning)
	// 2 errors: 2 rows
	setLastError(t, db, repos[0].Name, shards[0], "internet broke repo-1")
	setLastError(t, db, repos[4].Name, shards[2], "internet broke repo-5")
	// 1 corruption: 1 row
	logCorruption(t, db, repos[2].Name, shards[1], "runaway corruption repo-3")

	// Safety check that the counts are right:
	wantRepoStatistics := RepoStatistics{Total: 6, Cloning: 6, FailedFetch: 2, Corrupted: 1}
	wantGitserverReposStatistics := []GitserverReposStatistic{
		{ShardID: ""},
		{ShardID: shards[0], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[1], Total: 2, Cloning: 2, FailedFetch: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 2, Cloning: 2, FailedFetch: 1},
	}
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)

	// Now delete and recreate
	err = s.DeleteAndRecreateStatistics(ctx)
	require.NoError(t, err)

	// Should be the same, except the empty shard is gone
	wantGitserverReposStatistics = []GitserverReposStatistic{
		{ShardID: shards[0], Total: 2, Cloning: 2, FailedFetch: 1},
		{ShardID: shards[1], Total: 2, Cloning: 2, FailedFetch: 0, Corrupted: 1},
		{ShardID: shards[2], Total: 2, Cloning: 2, FailedFetch: 1},
	}
	assertRepoStatistics(t, ctx, s, wantRepoStatistics, wantGitserverReposStatistics)
}

func queryRepoName(t *testing.T, ctx context.Context, s *repoStatisticsStore, repoID api.RepoID) api.RepoName {
	t.Helper()
	var name api.RepoName
	err := s.QueryRow(ctx, sqlf.Sprintf("SELECT name FROM repo WHERE id = %s", repoID)).Scan(&name)
	if err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}
	return name
}

func queryRepoStatisticsCount(t *testing.T, ctx context.Context, s *repoStatisticsStore) int {
	t.Helper()
	var count int
	err := s.QueryRow(ctx, sqlf.Sprintf("SELECT COUNT(*) FROM repo_statistics;")).Scan(&count)
	if err != nil {
		t.Fatalf("failed to query repo name: %s", err)
	}
	return count
}

func queryGitserverReposStatisticsCount(t *testing.T, ctx context.Context, s *repoStatisticsStore) int {
	t.Helper()
	count, err := basestore.ScanInt(s.QueryRow(ctx, sqlf.Sprintf("SELECT COUNT(*) FROM gitserver_repos_statistics;")))
	if err != nil {
		t.Fatalf("failed to query gitserver_repos_statistics count: %s", err)
	}
	return count
}

func setCloneStatus(t *testing.T, db DB, repoName api.RepoName, shard string, status types.CloneStatus) {
	t.Helper()
	switch status {
	case types.CloneStatusCloned:
		if err := db.GitserverRepos().SetCloned(context.Background(), repoName, shard); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", repoName, err)
		}
	case types.CloneStatusCloning:
		if err := db.GitserverRepos().SetCloning(context.Background(), repoName, shard); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", repoName, err)
		}
	case types.CloneStatusNotCloned:
		if err := db.GitserverRepos().SetNotCloned(context.Background(), repoName); err != nil {
			t.Fatalf("failed to set clone status for repo %s: %s", repoName, err)
		}
	}
}

func setLastError(t *testing.T, db DB, repoName api.RepoName, shard string, msg string) {
	t.Helper()
	if err := db.GitserverRepos().SetLastError(context.Background(), repoName, msg, shard); err != nil {
		t.Fatalf("failed to set clone status for repo %s: %s", repoName, err)
	}
}

func logCorruption(t *testing.T, db DB, repoName api.RepoName, shard string, msg string) {
	t.Helper()
	if err := db.GitserverRepos().LogCorruption(context.Background(), repoName, msg, shard); err != nil {
		t.Fatalf("failed to log corruption for repo %s: %s", repoName, err)
	}
}

func assertRepoStatistics(t *testing.T, ctx context.Context, s RepoStatisticsStore, wantRepoStats RepoStatistics, wantGitserverStats []GitserverReposStatistic) {
	t.Helper()

	haveRepoStats, err := s.GetRepoStatistics(ctx)
	if err != nil {
		t.Fatalf("GetRepoStatistics failed: %s", err)
	}

	if diff := cmp.Diff(haveRepoStats, wantRepoStats); diff != "" {
		t.Errorf("repoStatistics differ: %s", diff)
	}

	haveGitserverStats, err := s.GetGitserverReposStatistics(ctx)
	if err != nil {
		t.Fatalf("GetRepoStatistics failed: %s", err)
	}

	type Stat = GitserverReposStatistic
	lessThan := func(s1 Stat, s2 Stat) bool { return s1.ShardID < s2.ShardID }
	if diff := cmp.Diff(haveGitserverStats, wantGitserverStats, cmpopts.SortSlices(lessThan)); diff != "" {
		t.Fatalf("gitserverReposStatistics differ: %s", diff)
	}
}
