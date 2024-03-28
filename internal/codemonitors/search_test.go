package codemonitors

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	gitprotocol "github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSnapshot(t *testing.T) {
	t.Run("fails with transaction", func(t *testing.T) {
		ctx := context.Background()
		logger := logtest.Scoped(t)
		db := database.NewDB(logger, dbtest.NewDB(t))
		err := db.WithTransact(ctx, func(tx database.DB) error {
			_, err := Snapshot(ctx, logtest.Scoped(t), tx, "type:commit")
			return err
		})
		require.Error(t, err)
	})
}

func TestAddCodeMonitorHook(t *testing.T) {
	t.Parallel()

	t.Run("errors on non-commit search", func(t *testing.T) {
		erroringJobs := []job.Job{
			jobutil.NewParallelJob(&jobutil.RepoSearchJob{}, &commit.SearchJob{}),
			&jobutil.RepoSearchJob{},
			jobutil.NewAndJob(&searcher.SymbolSearchJob{}, &commit.SearchJob{}),
			jobutil.NewTimeoutJob(0, &jobutil.RepoSearchJob{}),
		}

		for _, j := range erroringJobs {
			t.Run("", func(t *testing.T) {
				_, err := addCodeMonitorHook(j, nil)
				require.Error(t, err)
			})
		}
	})

	t.Run("error on multiple commit search jobs", func(t *testing.T) {
		_, err := addCodeMonitorHook(jobutil.NewAndJob(&commit.SearchJob{}, &commit.SearchJob{}), nil)
		require.Error(t, err)
	})

	t.Run("no errors on only commit search", func(t *testing.T) {
		nonErroringJobs := []job.Job{
			jobutil.NewLimitJob(1000, &commit.SearchJob{}),
			&commit.SearchJob{},
			jobutil.NewTimeoutJob(0, &commit.SearchJob{}),
		}

		for _, j := range nonErroringJobs {
			t.Run("", func(t *testing.T) {
				_, err := addCodeMonitorHook(j, nil)
				require.NoError(t, err)
			})
		}
	})

	t.Run("no errors on allowed queries", func(t *testing.T) {
		test := func(t *testing.T, input string) {
			plan, err := query.Pipeline(query.InitRegexp(input))
			require.NoError(t, err)
			inputs := &search.Inputs{
				UserSettings:        &schema.Settings{},
				PatternType:         query.SearchTypeLiteral,
				Protocol:            search.Streaming,
				Features:            &search.Features{},
				OnSourcegraphDotCom: true,
			}
			j, err := jobutil.NewPlanJob(inputs, plan)
			require.NoError(t, err)
			addCodeMonitorHook(j, nil)
		}

		queries := []string{
			"type:commit a or b",
			"type:diff a or b",
			"type:diff a and b",
			"type:diff a or b",
			"type:diff a or b repo:c",
			"type:commit a or b repo:c",
			"type:commit a or b repo:c case:no",
			"type:commit a or b repo:c context:global",
		}

		for _, q := range queries {
			t.Run("", func(t *testing.T) {
				test(t, q)
			})
		}
	})
}

func TestCodeMonitorHook(t *testing.T) {
	t.Parallel()

	type testFixtures struct {
		User    *types.User
		Repo    *types.Repo
		Monitor *database.Monitor
	}
	logger := logtest.Scoped(t)
	populateFixtures := func(db database.DB) testFixtures {
		ctx := context.Background()
		u, err := db.Users().Create(ctx, database.NewUser{Email: "test", Username: "test", EmailVerificationCode: "test"})
		require.NoError(t, err)
		err = db.Repos().Create(ctx, &types.Repo{Name: "test"})
		require.NoError(t, err)
		r, err := db.Repos().GetByName(ctx, "test")
		require.NoError(t, err)
		ctx = actor.WithActor(ctx, actor.FromUser(u.ID))
		m, err := db.CodeMonitors().CreateMonitor(ctx, database.MonitorArgs{NamespaceUserID: &u.ID})
		require.NoError(t, err)
		return testFixtures{User: u, Monitor: m, Repo: r}
	}

	db := database.NewDB(logger, dbtest.NewDB(t))
	fixtures := populateFixtures(db)
	ctx := context.Background()

	gs := gitserver.NewMockClient()
	gs.ResolveRevisionFunc.PushReturn("hash1", nil)
	gs.ResolveRevisionFunc.PushReturn("hash2", nil)
	gs.ResolveRevisionFunc.PushReturn("hash3", nil)
	gs.ResolveRevisionFunc.PushReturn("hash4", nil)
	gs.ResolveRevisionFunc.PushReturn("hash5", nil)
	gs.ResolveRevisionFunc.PushReturn("hash6", nil)

	// The first time, doSearch should receive only the resolved hashes
	doSearch := func(args *gitprotocol.SearchRequest) error {
		require.Equal(t, args.Revisions, []string{"hash1", "hash2"})
		return nil
	}
	triggerJobID := int32(1)
	err := hookWithID(ctx, logger, db, gs, fixtures.Monitor.ID, triggerJobID, fixtures.Repo.ID, &gitprotocol.SearchRequest{Revisions: []string{"rev1", "rev2"}}, doSearch)
	require.NoError(t, err)

	// The next time, doSearch should receive the new resolved hashes plus the
	// hashes from last time, but excluded
	doSearch = func(args *gitprotocol.SearchRequest) error {
		require.Equal(t, args.Revisions, []string{"hash3", "hash4", "^hash1", "^hash2"})
		return nil
	}
	err = hookWithID(ctx, logger, db, gs, fixtures.Monitor.ID, triggerJobID, fixtures.Repo.ID, &gitprotocol.SearchRequest{Revisions: []string{"rev1", "rev2"}}, doSearch)
	require.NoError(t, err)

	t.Run("deadline exceeded is propagated", func(t *testing.T) {
		doSearch = func(args *gitprotocol.SearchRequest) error {
			return context.DeadlineExceeded
		}
		err := hookWithID(ctx, logger, db, gs, fixtures.Monitor.ID, triggerJobID, fixtures.Repo.ID, &gitprotocol.SearchRequest{Revisions: []string{"rev1", "rev2"}}, doSearch)
		require.ErrorContains(t, err, "some commits may be skipped")
	})
}
