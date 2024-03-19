package inttests

import (
	"container/list"
	"context"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	server "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal"
	common "github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
)

func TestClone(t *testing.T) {
	root := t.TempDir()
	reposDir := filepath.Join(root, "repos")
	remote := createSimpleGitRepo(t, root)

	logger := logtest.Scoped(t)
	db := newMockDB()
	gsStore := dbmocks.NewMockGitserverRepoStore()
	db.GitserverReposFunc.SetDefaultReturn(gsStore)
	ctx := context.Background()
	repo := api.RepoName("github.com/test/repo")

	locker := NewMockRepositoryLocker()
	lock := NewMockRepositoryLock()
	locker.TryAcquireFunc.SetDefaultReturn(lock, true)

	s := server.NewServer(&server.ServerOpts{
		Logger:   logger,
		ReposDir: reposDir,
		GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
			return gitcli.NewBackend(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName)
		},
		GetRemoteURLFunc: func(_ context.Context, name api.RepoName) (string, error) {
			require.Equal(t, repo, name)
			return remote, nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
			require.Equal(t, repo, name)
			return vcssyncer.NewGitRepoSyncer(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory()), nil
		},
		DB:                      db,
		Perforce:                perforce.NewService(ctx, observation.TestContextTB(t), logger, db, list.New()),
		RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
		Locker:                  locker,
		RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(100, 10)),
		Hostname:                "test-shard",
	})

	grpcServer := defaults.NewServer(logtest.Scoped(t))
	proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s))

	handler := internalgrpc.MultiplexHandlers(grpcServer, s.Handler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	addrs := []string{u.Host}
	source := gitserver.NewTestClientSource(t, addrs)

	cli := gitserver.NewTestClient(t).WithClientSource(source)

	// Requesting a repo update should figure out that the repo is not yet
	// cloned and call clone. We expect that clone to succeed.
	_, err := cli.RequestRepoUpdate(ctx, repo)
	require.NoError(t, err)

	// Should have acquired a lock.
	mockassert.CalledOnce(t, locker.TryAcquireFunc)
	// Should have reported status. 21 lines is the output git currently produces.
	// This number might need to be adjusted over time, but before doing so please
	// check that the calls actually use the args you would expect them to use.
	mockassert.CalledN(t, lock.SetStatusFunc, 21)
	// Should have released the lock.
	mockassert.CalledOnce(t, lock.ReleaseFunc)

	// Check it was set to cloning first, then cloned.
	mockassert.CalledN(t, gsStore.SetCloneStatusFunc, 2)
	mockassert.CalledWith(t, gsStore.SetCloneStatusFunc, mockassert.Values(mockassert.Skip, repo, types.CloneStatusCloning, "test-shard"))
	mockassert.CalledWith(t, gsStore.SetCloneStatusFunc, mockassert.Values(mockassert.Skip, repo, types.CloneStatusCloned, "test-shard"))

	// Last output should have been stored for the repo.
	mockrequire.CalledOnce(t, gsStore.SetLastOutputFunc)
	haveLastOutput := gsStore.SetLastOutputFunc.History()[0].Arg2
	require.Contains(t, haveLastOutput, "Creating bare repo\n")
	require.Contains(t, haveLastOutput, "Created bare repo at ")
	require.Contains(t, haveLastOutput, "Fetching remote contents\n")
	// Ensure the path is properly redacted. The redactor just takes the whole
	// remote URL as redacted so this is expected.
	require.Contains(t, haveLastOutput, "From <redacted>")
	// Double newlines should not be part of our standard output. They are not
	// forbidden, but we currently don't use them. So this will guard against
	// regressions in the log processing to make sure we don't accidentally
	// introduce blank newlines for CRLF parsing. (Yes this took a while to get
	// right).
	require.NotContains(t, haveLastOutput, "\n\n")

	// Check that it was called exactly once total.
	mockassert.CalledOnce(t, gsStore.SetLastErrorFunc)
	// And that it was called for the right repo, setting the last error to empty.
	mockassert.CalledWith(t, gsStore.SetLastErrorFunc, mockassert.Values(mockassert.Skip, repo, "", "test-shard"))

	// Check that the repo is in the expected location on disk.
	_, err = os.Stat(gitserverfs.RepoDirFromName(reposDir, repo).Path())
	require.NoError(t, err)
}

func TestClone_Fail(t *testing.T) {
	root := t.TempDir()
	reposDir := filepath.Join(root, "repos")
	remote := filepath.Join(root, "non-existing")

	logger := logtest.Scoped(t)
	db := newMockDB()
	gsStore := dbmocks.NewMockGitserverRepoStore()
	db.GitserverReposFunc.SetDefaultReturn(gsStore)
	ctx := context.Background()
	repo := api.RepoName("github.com/test/repo")

	locker := NewMockRepositoryLocker()
	lock := NewMockRepositoryLock()
	locker.TryAcquireFunc.SetDefaultReturn(lock, true)

	s := server.NewServer(&server.ServerOpts{
		Logger:   logger,
		ReposDir: reposDir,
		GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
			return gitcli.NewBackend(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName)
		},
		GetRemoteURLFunc: func(_ context.Context, name api.RepoName) (string, error) {
			require.Equal(t, repo, name)
			return remote, nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
			require.Equal(t, repo, name)
			return vcssyncer.NewGitRepoSyncer(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory()), nil
		},
		DB:                      db,
		Perforce:                perforce.NewService(ctx, observation.TestContextTB(t), logger, db, list.New()),
		RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
		Locker:                  locker,
		RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(100, 10)),
		Hostname:                "test-shard",
	})

	grpcServer := defaults.NewServer(logtest.Scoped(t))
	proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s))

	handler := internalgrpc.MultiplexHandlers(grpcServer, s.Handler())
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	addrs := []string{u.Host}
	source := gitserver.NewTestClientSource(t, addrs)

	cli := gitserver.NewTestClient(t).WithClientSource(source)

	// Requesting a repo update should figure out that the repo is not yet
	// cloned and call clone. We expect that clone to fail, because vcssyncer.IsCloneable
	// fails here.
	resp, err := cli.RequestRepoUpdate(ctx, repo)
	require.NoError(t, err)
	// Note that this error is from IsCloneable(), not from Clone().
	require.Contains(t, resp.Error, "error cloning repo: repo github.com/test/repo not cloneable:")
	require.Contains(t, resp.Error, "exit status 128")

	// No lock should have been acquired.
	mockassert.NotCalled(t, locker.TryAcquireFunc)

	// Check we reported an error.
	// Check that it was called exactly once total.
	mockrequire.CalledOnce(t, gsStore.SetLastErrorFunc)
	// And that it was called for the right repo, setting the last error value.
	mockassert.CalledWith(t, gsStore.SetLastErrorFunc, mockassert.Values(mockassert.Skip, repo, mockassert.Skip, "test-shard"))
	require.Contains(t, gsStore.SetLastErrorFunc.History()[0].Arg2, `error cloning repo: repo github.com/test/repo not cloneable:`)
	require.Contains(t, gsStore.SetLastErrorFunc.History()[0].Arg2, "exit status 128")

	// And no other DB activity has happened.
	mockassert.NotCalled(t, gsStore.SetCloneStatusFunc)
	mockassert.NotCalled(t, gsStore.SetLastOutputFunc)

	// ===================

	// Now, fake that the IsCloneable check passes, then Clone will be called
	// and is expected to fail.
	vcssyncer.TestGitRepoExists = func(ctx context.Context, remoteURL *vcs.URL) error {
		return nil
	}
	t.Cleanup(func() {
		vcssyncer.TestGitRepoExists = nil
	})
	// Reset mock counters.
	gsStore = dbmocks.NewMockGitserverRepoStore()
	db.GitserverReposFunc.SetDefaultReturn(gsStore)

	// Requesting another repo update should figure out that the repo is not yet
	// cloned and call clone. We expect that clone to fail, but in the vcssyncer.Clone
	// stage this time, not vcssyncer.IsCloneable.
	resp, err = cli.RequestRepoUpdate(ctx, repo)
	require.NoError(t, err)
	require.Contains(t, resp.Error, "failed to clone github.com/test/repo: clone failed. Output: Creating bare repo\nCreated bare repo at")

	// Should have acquired a lock.
	mockassert.CalledOnce(t, locker.TryAcquireFunc)
	// Should have reported status. 7 lines is the output git currently produces.
	// This number might need to be adjusted over time, but before doing so please
	// check that the calls actually use the args you would expect them to use.
	mockassert.CalledN(t, lock.SetStatusFunc, 7)
	// Should have released the lock.
	mockassert.CalledOnce(t, lock.ReleaseFunc)

	// Check it was set to cloning first, then uncloned again (since clone failed).
	mockassert.CalledN(t, gsStore.SetCloneStatusFunc, 2)
	mockassert.CalledWith(t, gsStore.SetCloneStatusFunc, mockassert.Values(mockassert.Skip, repo, types.CloneStatusCloning, "test-shard"))
	mockassert.CalledWith(t, gsStore.SetCloneStatusFunc, mockassert.Values(mockassert.Skip, repo, types.CloneStatusNotCloned, "test-shard"))

	// Last output should have been stored for the repo.
	mockrequire.CalledOnce(t, gsStore.SetLastOutputFunc)
	haveLastOutput := gsStore.SetLastOutputFunc.History()[0].Arg2
	require.Contains(t, haveLastOutput, "Creating bare repo\n")
	require.Contains(t, haveLastOutput, "Created bare repo at ")
	require.Contains(t, haveLastOutput, "Fetching remote contents\n")
	// Check that also git output made it here.
	require.Contains(t, haveLastOutput, "does not appear to be a git repository\n")

	// Check that it was called exactly once total.
	mockrequire.CalledOnce(t, gsStore.SetLastErrorFunc)
	// And that it was called for the right repo, setting the last error to empty.
	mockassert.CalledWith(t, gsStore.SetLastErrorFunc, mockassert.Values(mockassert.Skip, repo, mockassert.Skip, "test-shard"))
	require.Contains(t, gsStore.SetLastErrorFunc.History()[0].Arg2, "Creating bare repo\n")
	require.Contains(t, gsStore.SetLastErrorFunc.History()[0].Arg2, "failed to fetch: exit status 128")

	// Check that no repo is in the expected location on disk.
	_, err = os.Stat(gitserverfs.RepoDirFromName(reposDir, repo).Path())
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}
