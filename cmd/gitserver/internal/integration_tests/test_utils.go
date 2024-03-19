package inttests

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/time/rate"

	sglog "github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	server "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal"
	common "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

var root string

// This is a default gitserver test client currently used for RequestRepoUpdate
// gitserver calls during invocation of MakeGitRepository function
var (
	testGitserverClient gitserver.Client
	GitserverAddresses  []string
)

func InitGitserver() {
	var t testing.T
	// Ignore users configuration in tests
	os.Setenv("GIT_CONFIG_NOSYSTEM", "true")
	os.Setenv("HOME", "/dev/null")
	logger := sglog.Scoped("gitserver_integration_tests")

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Fatal("listen failed", sglog.Error(err))
	}

	root, err = os.MkdirTemp("", "test")
	if err != nil {
		logger.Fatal(err.Error())
	}

	db := dbmocks.NewMockDB()
	db.GitserverReposFunc.SetDefaultReturn(dbmocks.NewMockGitserverRepoStore())
	db.FeatureFlagsFunc.SetDefaultReturn(dbmocks.NewMockFeatureFlagStore())

	r := dbmocks.NewMockRepoStore()
	r.GetByNameFunc.SetDefaultHook(func(ctx context.Context, repoName api.RepoName) (*types.Repo, error) {
		return &types.Repo{
			Name: repoName,
			ExternalRepo: api.ExternalRepoSpec{
				ServiceType: extsvc.TypeGitHub,
			},
		}, nil
	})
	db.ReposFunc.SetDefaultReturn(r)

	s := server.NewServer(&server.ServerOpts{
		Logger:   sglog.Scoped("server"),
		ReposDir: filepath.Join(root, "repos"),
		GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
			return gitcli.NewBackend(logtest.Scoped(&t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName)
		},
		GetRemoteURLFunc: func(ctx context.Context, name api.RepoName) (string, error) {
			return filepath.Join(root, "remotes", string(name)), nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
			return vcssyncer.NewGitRepoSyncer(logger, wrexec.NewNoOpRecordingCommandFactory()), nil
		},
		DB:                      db,
		RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
		Locker:                  server.NewRepositoryLocker(),
		RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(100, 10)),
	})

	grpcServer := defaults.NewServer(logger)
	proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s))
	handler := internalgrpc.MultiplexHandlers(grpcServer, s.Handler())

	srv := &http.Server{
		Handler: handler,
	}
	go func() {
		if err := srv.Serve(l); err != nil {
			logger.Fatal(err.Error())
		}
	}()

	serverAddress := l.Addr().String()
	source := gitserver.NewTestClientSource(&t, []string{serverAddress})
	testGitserverClient = gitserver.NewTestClient(&t).WithClientSource(source)
	GitserverAddresses = []string{serverAddress}
}

// MakeGitRepository calls initGitRepository to create a new Git repository and returns a handle to
// it.
func MakeGitRepository(t testing.TB, cmds ...string) api.RepoName {
	t.Helper()
	dir := InitGitRepository(t, cmds...)
	repo := api.RepoName(filepath.Base(dir))
	if resp, err := testGitserverClient.RequestRepoUpdate(context.Background(), repo); err != nil {
		t.Fatal(err)
	} else if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	return repo
}

// InitGitRepository initializes a new Git repository and runs cmds in a new
// temporary directory (returned as dir).
func InitGitRepository(t testing.TB, cmds ...string) string {
	t.Helper()
	remotes := filepath.Join(root, "remotes")
	if err := os.MkdirAll(remotes, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(remotes, strings.ReplaceAll(t.Name(), "/", "__"))
	if err != nil {
		t.Fatal(err)
	}
	cmds = append([]string{"git init"}, cmds...)
	for _, cmd := range cmds {
		out, err := GitCommand(dir, "bash", "-c", cmd).CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}
	return dir
}

func GitCommand(dir, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_CONFIG="+path.Join(dir, ".git", "config"),
		"GIT_COMMITTER_NAME=a",
		"GIT_COMMITTER_EMAIL=a@a.com",
		"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z",
		"GIT_AUTHOR_NAME=a",
		"GIT_AUTHOR_EMAIL=a@a.com",
		"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z",
	)
	return c
}

func createSimpleGitRepo(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "remotes", "simple")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range []string{
		"git init",
		"mkdir dir1",
		"echo -n infile1 > dir1/file1",
		"touch --date=2006-01-02T15:04:05Z dir1 dir1/file1 || touch -t 200601021704.05 dir1 dir1/file1",
		"git add dir1/file1",
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_AUTHOR_DATE=2006-01-02T15:04:05Z GIT_COMMITTER_DATE=2006-01-02T15:04:05Z git commit -m commit1 --author='a <a@a.com>' --date 2006-01-02T15:04:05Z",
		"echo -n infile2 > 'file 2'",
		"touch --date=2014-05-06T19:20:21Z 'file 2' || touch -t 201405062120.21 'file 2'",
		"git add 'file 2'",
		"GIT_COMMITTER_NAME=a GIT_COMMITTER_EMAIL=a@a.com GIT_AUTHOR_DATE=2006-01-02T15:04:05Z GIT_COMMITTER_DATE=2014-05-06T19:20:21Z git commit -m commit2 --author='a <a@a.com>' --date 2014-05-06T19:20:21Z",
		"git branch test-ref HEAD~1",
		"git branch test-nested-ref test-ref",
	} {
		c := exec.Command("bash", "-c", `GIT_CONFIG_GLOBAL="" GIT_CONFIG_SYSTEM="" `+cmd)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}

	return dir
}
