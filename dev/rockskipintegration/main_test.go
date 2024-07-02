package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/sourcegraph/go-ctags"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/fetcher"
	symbolsgitserver "github.com/sourcegraph/sourcegraph/cmd/symbols/gitserver"
	symbolsParser "github.com/sourcegraph/sourcegraph/cmd/symbols/parser"
	symbolstypes "github.com/sourcegraph/sourcegraph/cmd/symbols/types"
	"github.com/sourcegraph/sourcegraph/dev/gitserverintegration"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/ctags_config"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/rockskip"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestRockskipIntegration(t *testing.T) {
	gs, _ := gitserverintegration.NewTestGitserverWithRepos(t, map[api.RepoName]string{
		"github.com/sourcegraph/rockskiptest": gitserverintegration.RepoWithCommands(t,
			"echo '# Title' > README.md",
			"git add README.md",
			"git commit -m commit --author='Foo Author <foo@sourcegraph.com>'",
		),
	})

	ctx := context.Background()
	observationCtx := observation.TestContextTB(t)

	// Verify gitserver cloned correctly:
	head, headSHA, err := gs.GetDefaultBranch(ctx, "github.com/sourcegraph/rockskiptest", false)
	require.NoError(t, err)
	require.Equal(t, "refs/heads/master", head)

	db := dbtest.NewDB(t)
	require.NoError(t, database.NewDB(logtest.Scoped(t), db).Repos().Create(ctx, &types.Repo{Name: "github.com/sourcegraph/rockskiptest"}))
	_, err = db.ExecContext(ctx, "INSERT INTO rockskip_repos (repo, last_accessed_at) VALUES ($1, NOW())", "github.com/sourcegraph/rockskiptest")
	require.NoError(t, err)

	sgs := symbolsgitserver.NewClient(observationCtx, gs)
	ctagsConfig := symbolstypes.LoadCtagsConfig(env.BaseConfig{})
	// Try to find the universal ctags binary. In bazel, it will be provided by bazel.
	// Outside of bazel, we rely on the system.
	if os.Getenv("BAZEL_TEST") != "" {
		ctagsConfig.UniversalCommand, _ = runfiles.Rlocation(os.Getenv("CTAGS_RLOCATIONPATH"))
	} else {
		_, err = exec.LookPath(ctagsConfig.UniversalCommand)
		if err != nil {
			// universal-ctags installed with brew is called ctags, try that next:
			_, err = exec.LookPath("ctags")
			if err == nil {
				ctagsConfig.UniversalCommand = "ctags"
				// In bazel, we expose the path to ctags via an environment variable.
			}
		}
	}
	svc, err := rockskip.NewService(
		observationCtx,
		db,
		sgs,
		fetcher.NewRepositoryFetcher(observationCtx, sgs, 100000, 1000),
		func() (ctags.Parser, error) {
			return symbolsParser.SpawnCtags(log.Scoped("parser"), ctagsConfig, ctags_config.UniversalCtags)
		},
		// TODO: Adjust these numbers as needed:
		1, 1, true, 1, 1024, 1024, true,
	)
	require.NoError(t, err)

	require.NoError(t, svc.Index(ctx, "github.com/sourcegraph/rockskiptest", string(headSHA)))

	// TODO: Properly validate rockskip data here:
	res, err := svc.Search(ctx, search.SymbolsParameters{
		Repo:     "github.com/sourcegraph/rockskiptest",
		CommitID: api.CommitID(headSHA),
	})
	require.NoError(t, err)
	require.Equal(t, []result.Symbol{
		{
			Name:      "Title",
			Path:      "README.md",
			Line:      0,
			Character: 2,
			Kind:      "chapter",
		},
	}, res)
}
