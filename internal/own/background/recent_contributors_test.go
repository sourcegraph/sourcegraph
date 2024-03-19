package background

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func Test_RecentContributorIndexFromGitserver(t *testing.T) {
	rcache.SetupForTest(t)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	ctx := context.Background()

	err := db.Repos().Create(ctx, &types.Repo{
		ID:   1,
		Name: "own/repo1",
	})
	require.NoError(t, err)

	commits := []fakeCommit{
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file2.txt"},
		},
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file3.txt"},
		},
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file2.txt", "dir/subdir/file.txt"},
		},
		{
			name:         "bob",
			email:        "bob@example.com",
			changedFiles: []string{"file1.txt", "dir2/file2.txt", "dir2/subdir/file.txt"},
		},
	}

	client := gitserver.NewMockClient()
	client.CommitLogFunc.SetDefaultReturn(fakeCommitsToLog(commits), nil)
	indexer := newRecentContributorsIndexer(client, db, logger)
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultReturn(true)
	checker.EnabledForRepoIDFunc.SetDefaultReturn(false, nil)
	err = indexer.indexRepo(ctx, api.RepoID(1), checker)
	require.NoError(t, err)

	for p, w := range map[string][]database.RecentContributorSummary{
		"dir": {
			{
				AuthorName:        "alice",
				AuthorEmail:       "alice@example.com",
				ContributionCount: 4,
			},
		},
		"file1.txt": {
			{
				AuthorName:        "alice",
				AuthorEmail:       "alice@example.com",
				ContributionCount: 3,
			},
			{
				AuthorName:        "bob",
				AuthorEmail:       "bob@example.com",
				ContributionCount: 1,
			},
		},
		"": {
			{
				AuthorName:        "alice",
				AuthorEmail:       "alice@example.com",
				ContributionCount: 7,
			},
			{
				AuthorName:        "bob",
				AuthorEmail:       "bob@example.com",
				ContributionCount: 3,
			},
		},
	} {
		path := p
		want := w
		t.Run(path, func(t *testing.T) {
			got, err := db.RecentContributionSignals().FindRecentAuthors(ctx, 1, path)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, want, got)
		})
	}
}

func Test_RecentContributorIndex_CanSeePrivateRepos(t *testing.T) {
	rcache.SetupForTest(t)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	err := db.Repos().Create(ctx, &types.Repo{
		ID:      1,
		Name:    "own/repo1",
		Private: true,
	})
	require.NoError(t, err)

	userWithAccess, err := db.Users().Create(ctx, database.NewUser{Username: "user1234"})
	require.NoError(t, err)

	userNoAccess, err := db.Users().Create(ctx, database.NewUser{Username: "user-no-access"})
	require.NoError(t, err)

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			PermissionsUserMapping: &schema.PermissionsUserMapping{
				Enabled: true,
				BindID:  "email",
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	_, err = db.Perms().SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: userWithAccess.ID}}, authz.SourceAPI)
	require.NoError(t, err)

	client := gitserver.NewMockClient()
	indexer := newRecentContributorsIndexer(client, db, logger)

	t.Run("non-internal user", func(t *testing.T) {
		// this is kind of an unrelated test just to provide a baseline that there is actually a difference when
		// we use the internal context. Otherwise, we could accidentally break this and not know it.
		newCtx := actor.WithActor(ctx, actor.FromUser(userNoAccess.ID)) // just to make sure this is a different user
		checker := authz.NewMockSubRepoPermissionChecker()
		checker.EnabledFunc.SetDefaultReturn(true)
		checker.EnabledForRepoIDFunc.SetDefaultReturn(false, nil)
		err := indexer.indexRepo(newCtx, api.RepoID(1), checker)
		assert.ErrorContains(t, err, "repo not found: id=1")
	})

	t.Run("internal user", func(t *testing.T) {
		newCtx := actor.WithInternalActor(ctx)
		checker := authz.NewMockSubRepoPermissionChecker()
		checker.EnabledFunc.SetDefaultReturn(true)
		checker.EnabledForRepoIDFunc.SetDefaultReturn(false, nil)
		err := indexer.indexRepo(newCtx, api.RepoID(1), checker)
		assert.NoError(t, err)
	})
}

func Test_RecentContributorIndexSkipsSubrepoPermsRepos(t *testing.T) {
	rcache.SetupForTest(t)
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))

	ctx := context.Background()

	err := db.Repos().Create(ctx, &types.Repo{
		ID:   1,
		Name: "own/repo1",
	})
	require.NoError(t, err)

	commits := []fakeCommit{
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file2.txt"},
		},
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file3.txt"},
		},
		{
			name:         "alice",
			email:        "alice@example.com",
			changedFiles: []string{"file1.txt", "dir/file2.txt", "dir/subdir/file.txt"},
		},
		{
			name:         "bob",
			email:        "bob@example.com",
			changedFiles: []string{"file1.txt", "dir2/file2.txt", "dir2/subdir/file.txt"},
		},
	}

	client := gitserver.NewMockClient()
	client.CommitLogFunc.SetDefaultReturn(fakeCommitsToLog(commits), nil)
	indexer := newRecentContributorsIndexer(client, db, logger)
	checker := authz.NewMockSubRepoPermissionChecker()
	checker.EnabledFunc.SetDefaultReturn(true)
	checker.EnabledForRepoIDFunc.SetDefaultReturn(true, nil)
	err = indexer.indexRepo(ctx, api.RepoID(1), checker)
	require.NoError(t, err)
	got, err := db.RecentContributionSignals().FindRecentAuthors(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", got)
	assert.Equal(t, 0, len(got))
}

func fakeCommitsToLog(commits []fakeCommit) (results []gitserver.CommitLog) {
	for i, commit := range commits {
		results = append(results, gitserver.CommitLog{
			AuthorEmail:  commit.email,
			AuthorName:   commit.name,
			Timestamp:    time.Now(),
			SHA:          gitSha(fmt.Sprintf("%d", i)),
			ChangedFiles: commit.changedFiles,
		})
	}
	return results
}

type fakeCommit struct {
	email        string
	name         string
	changedFiles []string
}

func gitSha(val string) string {
	writer := sha1.New()
	writer.Write([]byte(val))
	return hex.EncodeToString(writer.Sum(nil))
}
