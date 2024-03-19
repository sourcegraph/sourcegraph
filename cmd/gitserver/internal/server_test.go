package internal

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git/gitcli"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/vcssyncer"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	v1 "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Test struct {
	Name            string
	Request         *v1.ExecRequest
	ExpectedCode    codes.Code
	ExpectedBody    string
	ExpectedError   string
	ExpectedDetails []any
}

func TestExecRequest(t *testing.T) {
	conf.Mock(&conf.Unified{})
	t.Cleanup(func() { conf.Mock(nil) })

	tests := []Test{
		{
			Name: "Command",
			Request: &v1.ExecRequest{
				Repo: "github.com/gorilla/mux",
				Args: [][]byte{[]byte("diff")},
			},
			ExpectedCode:  codes.Unknown,
			ExpectedBody:  "teststdout",
			ExpectedError: "teststderr",
			ExpectedDetails: []any{&v1.ExecStatusPayload{
				StatusCode: 42,
				Stderr:     "teststderr",
			}},
		},
		{
			Name: "NonexistingRepo",
			Request: &v1.ExecRequest{
				Repo: "github.com/gorilla/doesnotexist",
				Args: [][]byte{[]byte("diff")},
			},
			ExpectedCode:  codes.NotFound,
			ExpectedError: "repo not found",
			ExpectedDetails: []any{&v1.RepoNotFoundPayload{
				Repo:            "github.com/gorilla/doesnotexist",
				CloneInProgress: false,
			}},
		},
		{
			Name: "UnclonedRepo",
			Request: &v1.ExecRequest{
				Repo: "github.com/nicksnyder/go-i18n",
				Args: [][]byte{[]byte("diff")},
			},
			ExpectedCode:  codes.NotFound,
			ExpectedError: "repo not found",
			ExpectedDetails: []any{&v1.RepoNotFoundPayload{
				Repo:            "github.com/nicksnyder/go-i18n",
				CloneInProgress: true,
			}},
		},
		{
			Name: "Error",
			Request: &v1.ExecRequest{
				Repo: "github.com/gorilla/mux",
				Args: [][]byte{[]byte("merge-base")},
			},
			ExpectedCode:  codes.Unknown,
			ExpectedError: "testerror",
			ExpectedDetails: []any{&v1.ExecStatusPayload{
				StatusCode: 1,
				Stderr:     "teststderr",
			}},
		},
		{
			Name: "EmptyInput",
			Request: &v1.ExecRequest{
				Repo: "github.com/gorilla/mux",
			},
			ExpectedCode:  codes.InvalidArgument,
			ExpectedError: "invalid command",
		},
		{
			Name: "BadCommand",
			Request: &v1.ExecRequest{
				Repo: "github.com/gorilla/mux",
				Args: [][]byte{[]byte("invalid-command")},
			},
			ExpectedCode:  codes.InvalidArgument,
			ExpectedError: "invalid command",
		},
	}

	db := dbmocks.NewMockDB()
	gr := dbmocks.NewMockGitserverRepoStore()
	db.GitserverReposFunc.SetDefaultReturn(gr)
	reposDir := t.TempDir()
	s := NewServer(&ServerOpts{
		Logger:   logtest.Scoped(t),
		ReposDir: reposDir,
		GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
			backend := git.NewMockGitBackend()
			backend.ExecFunc.SetDefaultHook(func(ctx context.Context, args ...string) (io.ReadCloser, error) {
				if !gitcli.IsAllowedGitCmd(logtest.Scoped(t), args, gitserverfs.RepoDirFromName(reposDir, repoName)) {
					return nil, gitcli.ErrBadGitCommand
				}

				switch args[0] {
				case "diff":
					var stdout bytes.Buffer
					stdout.Write([]byte("teststdout"))
					return &errorReader{
						ReadCloser: io.NopCloser(&stdout),
						err: &gitcli.CommandFailedError{
							Stderr:     []byte("teststderr"),
							ExitStatus: 42,
							Inner:      errors.New("teststderr"),
						},
					}, nil
				case "merge-base":
					return &errorReader{
						ReadCloser: io.NopCloser(&bytes.Buffer{}),
						err: &gitcli.CommandFailedError{
							Stderr:     []byte("teststderr"),
							ExitStatus: 1,
							Inner:      errors.New("testerror"),
						},
					}, nil
				}
				return io.NopCloser(&bytes.Buffer{}), nil
			})
			return backend
		},
		GetRemoteURLFunc: func(ctx context.Context, name api.RepoName) (string, error) {
			return "https://" + string(name) + ".git", nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
			return vcssyncer.NewGitRepoSyncer(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory()), nil
		},
		DB:                      db,
		RecordingCommandFactory: wrexec.NewNoOpRecordingCommandFactory(),
		Locker:                  NewRepositoryLocker(),
		RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(rate.Inf, 10)),
	})

	s.skipCloneForTests = true

	gs := NewGRPCServer(s)

	origRepoCloned := repoCloned
	repoCloned = func(dir common.GitDir) bool {
		return dir == gitserverfs.RepoDirFromName(reposDir, "github.com/gorilla/mux") || dir == gitserverfs.RepoDirFromName(reposDir, "my-mux")
	}
	t.Cleanup(func() { repoCloned = origRepoCloned })

	vcssyncer.TestGitRepoExists = func(ctx context.Context, remoteURL *vcs.URL) error {
		if remoteURL.String() == "https://github.com/nicksnyder/go-i18n.git" {
			return nil
		}
		return errors.New("not cloneable")
	}
	t.Cleanup(func() { vcssyncer.TestGitRepoExists = nil })

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ss := gitserver.NewMockGitserverService_ExecServer()
			ss.ContextFunc.SetDefaultReturn(context.Background())
			var receivedData []byte
			ss.SendFunc.SetDefaultHook(func(er *v1.ExecResponse) error {
				receivedData = append(receivedData, er.GetData()...)
				return nil
			})
			err := gs.Exec(test.Request, ss)

			if test.ExpectedCode == codes.OK && err != nil {
				t.Fatal(err)
			}

			if test.ExpectedCode != codes.OK {
				if err == nil {
					t.Fatal("expected error to be returned")
				}
				s, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, test.ExpectedCode, s.Code(), "wrong error code: expected %v, got %v %v", test.ExpectedCode, s.Code(), err)

				if len(test.ExpectedDetails) > 0 {
					if diff := cmp.Diff(test.ExpectedDetails, s.Details(), cmpopts.IgnoreUnexported(v1.ExecStatusPayload{}, v1.RepoNotFoundPayload{})); diff != "" {
						t.Fatalf("unexpected error details (-want +got):\n%s", diff)
					}
				}

				if strings.TrimSpace(s.Message()) != test.ExpectedError {
					t.Errorf("wrong error body: expected %q, got %q", test.ExpectedError, s.Message())
				}
			}

			if strings.TrimSpace(string(receivedData)) != test.ExpectedBody {
				t.Errorf("wrong body: expected %q, got %q", test.ExpectedBody, string(receivedData))
			}
		})
	}
}

type errorReader struct {
	io.ReadCloser

	err error
}

func (ec *errorReader) Read(p []byte) (int, error) {
	n, err := ec.ReadCloser.Read(p)
	if err == nil {
		return n, nil
	}
	if err == io.EOF {
		return n, ec.err
	}
	return n, err
}

// makeSingleCommitRepo make create a new repo with a single commit and returns
// the HEAD SHA
func makeSingleCommitRepo(cmd func(string, ...string) string) string {
	// Setup a repo with a commit so we can see if we can clone it.
	cmd("git", "init", ".")
	cmd("sh", "-c", "echo hello world > hello.txt")
	return addCommitToRepo(cmd)
}

// addCommitToRepo adds a commit to the repo at the current path.
func addCommitToRepo(cmd func(string, ...string) string) string {
	// Setup a repo with a commit so we can see if we can clone it.
	cmd("git", "add", "hello.txt")
	cmd("git", "commit", "-m", "hello")
	return cmd("git", "rev-parse", "HEAD")
}

func makeTestServer(ctx context.Context, t *testing.T, repoDir, remote string, db database.DB) *Server {
	t.Helper()

	if db == nil {
		mDB := dbmocks.NewMockDB()
		mDB.GitserverReposFunc.SetDefaultReturn(dbmocks.NewMockGitserverRepoStore())
		mDB.FeatureFlagsFunc.SetDefaultReturn(dbmocks.NewMockFeatureFlagStore())

		repoStore := dbmocks.NewMockRepoStore()
		repoStore.GetByNameFunc.SetDefaultReturn(nil, &database.RepoNotFoundErr{})

		mDB.ReposFunc.SetDefaultReturn(repoStore)

		db = mDB
	}

	logger := logtest.Scoped(t)
	obctx := observation.TestContextTB(t)

	cloneQueue := NewCloneQueue(obctx, list.New())
	s := NewServer(&ServerOpts{
		Logger:   logger,
		ReposDir: repoDir,
		GetBackendFunc: func(dir common.GitDir, repoName api.RepoName) git.GitBackend {
			return gitcli.NewBackend(logtest.Scoped(t), wrexec.NewNoOpRecordingCommandFactory(), dir, repoName)
		},
		GetRemoteURLFunc: func(context.Context, api.RepoName) (string, error) {
			return remote, nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
			return vcssyncer.NewGitRepoSyncer(logtest.Scoped(t), wrexec.
				NewNoOpRecordingCommandFactory()), nil
		},
		DB:                      db,
		CloneQueue:              cloneQueue,
		Locker:                  NewRepositoryLocker(),
		RPSLimiter:              ratelimit.NewInstrumentedLimiter("GitserverTest", rate.NewLimiter(rate.Inf, 10)),
		RecordingCommandFactory: wrexec.NewRecordingCommandFactory(nil, 0),
		Perforce:                perforce.NewService(ctx, obctx, logger, db, list.New()),
	})

	s.ctx = ctx
	s.cloneLimiter = limiter.NewMutable(1)

	p := s.NewClonePipeline(logtest.Scoped(t), cloneQueue)
	p.Start()
	t.Cleanup(p.Stop)
	return s
}

func TestCloneRepo(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reposDir := t.TempDir()

	repoName := api.RepoName("example.com/foo/bar")
	db := database.NewDB(logger, dbtest.NewDB(t))
	if _, err := db.FeatureFlags().CreateBool(ctx, "clone-progress-logging", true); err != nil {
		t.Fatal(err)
	}
	dbRepo := &types.Repo{
		Name:        repoName,
		Description: "Test",
	}
	// Insert the repo into our database
	if err := db.Repos().Create(ctx, dbRepo); err != nil {
		t.Fatal(err)
	}

	assertRepoState := func(status types.CloneStatus, size int64, wantErr error) {
		t.Helper()
		fromDB, err := db.GitserverRepos().GetByID(ctx, dbRepo.ID)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, status, fromDB.CloneStatus)
		assert.Equal(t, size, fromDB.RepoSizeBytes)
		var errString string
		if wantErr != nil {
			errString = wantErr.Error()
		}
		assert.Equal(t, errString, fromDB.LastError)
	}

	// Verify the gitserver repo entry exists.
	assertRepoState(types.CloneStatusNotCloned, 0, nil)

	repoDir := gitserverfs.RepoDirFromName(reposDir, repoName)
	remoteDir := filepath.Join(reposDir, "remote")
	if err := os.Mkdir(remoteDir, os.ModePerm); err != nil {
		t.Fatal(err)
	}
	cmdExecDir := remoteDir
	cmd := func(name string, arg ...string) string {
		t.Helper()
		return runCmd(t, cmdExecDir, name, arg...)
	}
	wantCommit := makeSingleCommitRepo(cmd)
	// Add a bad tag
	cmd("git", "tag", "HEAD")

	s := makeTestServer(ctx, t, reposDir, remoteDir, db)

	// Enqueue repo clone.
	_, err := s.CloneRepo(ctx, repoName, CloneOptions{})
	require.NoError(t, err)

	// Wait until the clone is done. Please do not use this code snippet
	// outside of a test. We only know this works since our test only starts
	// one clone and will have nothing else attempt to lock.
	for range 1000 {
		_, cloning := s.locker.Status(repoDir)
		if !cloning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	wantRepoSize := gitserverfs.DirSize(repoDir.Path("."))
	assertRepoState(types.CloneStatusCloned, wantRepoSize, err)

	cmdExecDir = repoDir.Path(".")
	gotCommit := cmd("git", "rev-parse", "HEAD")
	if wantCommit != gotCommit {
		t.Fatal("failed to clone:", gotCommit)
	}

	// Test blocking with a failure (already exists since we didn't specify overwrite)
	_, err = s.CloneRepo(context.Background(), repoName, CloneOptions{Block: true})
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected clone repo to fail with already exists: %s", err)
	}
	assertRepoState(types.CloneStatusCloned, wantRepoSize, err)

	// Test blocking with overwrite. First add random file to GIT_DIR. If the
	// file is missing after cloning we know the directory was replaced
	mkFiles(t, repoDir.Path("."), "HELLO")
	_, err = s.CloneRepo(context.Background(), repoName, CloneOptions{Block: true, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	assertRepoState(types.CloneStatusCloned, wantRepoSize, err)

	if _, err := os.Stat(repoDir.Path("HELLO")); !os.IsNotExist(err) {
		t.Fatalf("expected clone to be overwritten: %s", err)
	}

	gotCommit = cmd("git", "rev-parse", "HEAD")
	if wantCommit != gotCommit {
		t.Fatal("failed to clone:", gotCommit)
	}
	gitserverRepo, err := db.GitserverRepos().GetByName(ctx, repoName)
	if err != nil {
		t.Fatal(err)
	}
	if gitserverRepo.CloningProgress == "" {
		t.Error("want non-empty CloningProgress")
	}
}

func TestCloneRepoRecordsFailures(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := logtest.Scoped(t)
	remote := t.TempDir()
	repoName := api.RepoName("example.com/foo/bar")
	db := database.NewDB(logger, dbtest.NewDB(t))

	dbRepo := &types.Repo{
		Name:        repoName,
		Description: "Test",
	}
	// Insert the repo into our database
	if err := db.Repos().Create(ctx, dbRepo); err != nil {
		t.Fatal(err)
	}

	assertRepoState := func(status types.CloneStatus, size int64, wantErr string) {
		t.Helper()
		fromDB, err := db.GitserverRepos().GetByID(ctx, dbRepo.ID)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, status, fromDB.CloneStatus)
		assert.Equal(t, size, fromDB.RepoSizeBytes)
		assert.Equal(t, wantErr, fromDB.LastError)
	}

	// Verify the gitserver repo entry exists.
	assertRepoState(types.CloneStatusNotCloned, 0, "")

	reposDir := t.TempDir()
	s := makeTestServer(ctx, t, reposDir, remote, db)

	for _, tc := range []struct {
		name         string
		getVCSSyncer func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error)
		wantErr      string
	}{
		{
			name: "Not cloneable",
			getVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
				m := vcssyncer.NewMockVCSSyncer()
				m.IsCloneableFunc.SetDefaultHook(func(context.Context, api.RepoName, *vcs.URL) error {
					return errors.New("not_cloneable")
				})
				return m, nil
			},
			wantErr: "error cloning repo: repo example.com/foo/bar not cloneable: not_cloneable",
		},
		{
			name: "Failing clone",
			getVCSSyncer: func(ctx context.Context, name api.RepoName) (vcssyncer.VCSSyncer, error) {
				m := vcssyncer.NewMockVCSSyncer()
				m.CloneFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, _ *vcs.URL, _ common.GitDir, _ string, w io.Writer) error {
					_, err := fmt.Fprint(w, "fatal: repository '/dev/null' does not exist")
					require.NoError(t, err)
					return &exec.ExitError{ProcessState: &os.ProcessState{}}
				})
				return m, nil
			},
			wantErr: "failed to clone example.com/foo/bar: clone failed. Output: fatal: repository '/dev/null' does not exist: exit status 0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.getVCSSyncer = tc.getVCSSyncer
			_, _ = s.CloneRepo(ctx, repoName, CloneOptions{
				Block: true,
			})
			assertRepoState(types.CloneStatusNotCloned, 0, tc.wantErr)
		})
	}
}

var ignoreVolatileGitserverRepoFields = cmpopts.IgnoreFields(
	types.GitserverRepo{},
	"LastFetched",
	"LastChanged",
	"RepoSizeBytes",
	"UpdatedAt",
	"CorruptionLogs",
	"CloningProgress",
)

func TestHandleRepoUpdate(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	remote := t.TempDir()
	repoName := api.RepoName("example.com/foo/bar")
	db := database.NewDB(logger, dbtest.NewDB(t))

	dbRepo := &types.Repo{
		Name:        repoName,
		Description: "Test",
	}
	// Insert the repo into our database
	if err := db.Repos().Create(ctx, dbRepo); err != nil {
		t.Fatal(err)
	}

	repo := remote
	cmd := func(name string, arg ...string) string {
		t.Helper()
		return runCmd(t, repo, name, arg...)
	}
	_ = makeSingleCommitRepo(cmd)
	// Add a bad tag
	cmd("git", "tag", "HEAD")

	reposDir := t.TempDir()

	s := makeTestServer(ctx, t, reposDir, remote, db)

	// Confirm that failing to clone the repo stores the error
	oldRemoveURLFunc := s.getRemoteURLFunc
	s.getRemoteURLFunc = func(ctx context.Context, name api.RepoName) (string, error) {
		return "https://invalid.example.com/", nil
	}
	s.RepoUpdate(ctx, &protocol.RepoUpdateRequest{
		Repo: repoName,
	})

	size := gitserverfs.DirSize(gitserverfs.RepoDirFromName(s.reposDir, repoName).Path("."))
	want := &types.GitserverRepo{
		RepoID:        dbRepo.ID,
		ShardID:       "",
		CloneStatus:   types.CloneStatusNotCloned,
		RepoSizeBytes: size,
		LastError:     "",
	}
	fromDB, err := db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// We don't care exactly what the error is here
	cmpIgnored := cmpopts.IgnoreFields(types.GitserverRepo{}, "LastFetched", "LastChanged", "RepoSizeBytes", "UpdatedAt", "LastError", "CorruptionLogs")
	// But we do care that it exists
	if fromDB.LastError == "" {
		t.Errorf("Expected an error when trying to clone from an invalid URL")
	}

	// We don't expect an error
	if diff := cmp.Diff(want, fromDB, cmpIgnored); diff != "" {
		t.Fatal(diff)
	}

	// This will perform an initial clone
	s.getRemoteURLFunc = oldRemoveURLFunc
	s.RepoUpdate(ctx, &protocol.RepoUpdateRequest{
		Repo: repoName,
	})

	size = gitserverfs.DirSize(gitserverfs.RepoDirFromName(s.reposDir, repoName).Path("."))
	want = &types.GitserverRepo{
		RepoID:        dbRepo.ID,
		ShardID:       "",
		CloneStatus:   types.CloneStatusCloned,
		RepoSizeBytes: size,
		LastError:     "",
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// We don't expect an error
	if diff := cmp.Diff(want, fromDB, ignoreVolatileGitserverRepoFields); diff != "" {
		t.Fatal(diff)
	}

	// Now we'll call again and with an update that fails
	doBackgroundRepoUpdateMock = func(name api.RepoName) error {
		return errors.New("fail")
	}
	t.Cleanup(func() { doBackgroundRepoUpdateMock = nil })

	// This will trigger an update since the repo is already cloned
	s.RepoUpdate(ctx, &protocol.RepoUpdateRequest{
		Repo: repoName,
	})

	want = &types.GitserverRepo{
		RepoID:        dbRepo.ID,
		ShardID:       "",
		CloneStatus:   types.CloneStatusCloned,
		LastError:     "fail",
		RepoSizeBytes: size,
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// We expect an error
	if diff := cmp.Diff(want, fromDB, ignoreVolatileGitserverRepoFields); diff != "" {
		t.Fatal(diff)
	}

	// Now we'll call again and with an update that succeeds
	doBackgroundRepoUpdateMock = nil

	// This will trigger an update since the repo is already cloned
	s.RepoUpdate(ctx, &protocol.RepoUpdateRequest{
		Repo: repoName,
	})

	want = &types.GitserverRepo{
		RepoID:        dbRepo.ID,
		ShardID:       "",
		CloneStatus:   types.CloneStatusCloned,
		RepoSizeBytes: gitserverfs.DirSize(gitserverfs.RepoDirFromName(s.reposDir, repoName).Path(".")), // we compute the new size
	}
	fromDB, err = db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	// We expect an update
	if diff := cmp.Diff(want, fromDB, ignoreVolatileGitserverRepoFields); diff != "" {
		t.Fatal(diff)
	}
}

func TestCloneRepo_EnsureValidity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("with no remote HEAD file", func(t *testing.T) {
		var (
			remote   = t.TempDir()
			reposDir = t.TempDir()
			cmd      = func(name string, arg ...string) {
				t.Helper()
				runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("rm", ".git/HEAD")

		s := makeTestServer(ctx, t, reposDir, remote, nil)
		if _, err := s.CloneRepo(ctx, "example.com/foo/bar", CloneOptions{}); err == nil {
			t.Fatal("expected an error, got none")
		}
	})
	t.Run("with an empty remote HEAD file", func(t *testing.T) {
		var (
			remote   = t.TempDir()
			reposDir = t.TempDir()
			cmd      = func(name string, arg ...string) {
				t.Helper()
				runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("sh", "-c", ": > .git/HEAD")

		s := makeTestServer(ctx, t, reposDir, remote, nil)
		if _, err := s.CloneRepo(ctx, "example.com/foo/bar", CloneOptions{}); err == nil {
			t.Fatal("expected an error, got none")
		}
	})
	t.Run("with no local HEAD file", func(t *testing.T) {
		var (
			reposDir = t.TempDir()
			remote   = filepath.Join(reposDir, "remote")
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
			repoName = api.RepoName("example.com/foo/bar")
		)

		if err := os.Mkdir(remote, os.ModePerm); err != nil {
			t.Fatal(err)
		}

		_ = makeSingleCommitRepo(cmd)
		s := makeTestServer(ctx, t, reposDir, remote, nil)

		testRepoCorrupter = func(_ context.Context, tmpDir common.GitDir) {
			if err := os.Remove(tmpDir.Path("HEAD")); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() { testRepoCorrupter = nil })
		// Use block so we get clone errors right here and don't have to rely on the
		// clone queue. There's no other reason for blocking here, just convenience/simplicity.
		_, err := s.CloneRepo(ctx, repoName, CloneOptions{Block: true})
		require.NoError(t, err)

		dst := gitserverfs.RepoDirFromName(s.reposDir, repoName)
		head, err := os.ReadFile(fmt.Sprintf("%s/HEAD", dst))
		if os.IsNotExist(err) {
			t.Fatal("expected a reconstituted HEAD, but no file exists")
		}
		if head == nil {
			t.Fatal("expected a reconstituted HEAD, but the file is empty")
		}
	})
	t.Run("with an empty local HEAD file", func(t *testing.T) {
		var (
			remote   = t.TempDir()
			reposDir = t.TempDir()
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
		)

		_ = makeSingleCommitRepo(cmd)
		s := makeTestServer(ctx, t, reposDir, remote, nil)

		testRepoCorrupter = func(_ context.Context, tmpDir common.GitDir) {
			cmd("sh", "-c", fmt.Sprintf(": > %s/HEAD", tmpDir))
		}
		t.Cleanup(func() { testRepoCorrupter = nil })
		if _, err := s.CloneRepo(ctx, "example.com/foo/bar", CloneOptions{Block: true}); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		dst := gitserverfs.RepoDirFromName(s.reposDir, "example.com/foo/bar")

		head, err := os.ReadFile(fmt.Sprintf("%s/HEAD", dst))
		if os.IsNotExist(err) {
			t.Fatal("expected a reconstituted HEAD, but no file exists")
		}
		if head == nil {
			t.Fatal("expected a reconstituted HEAD, but the file is empty")
		}
	})
}

func TestHostnameMatch(t *testing.T) {
	testCases := []struct {
		hostname    string
		addr        string
		shouldMatch bool
	}{
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-1",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-1.gitserver:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-10.gitserver:3178",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-10",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-10",
			addr:        "",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-10",
			addr:        "gitserver-10:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-10",
			addr:        "gitserver-10:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-0.prod",
			addr:        "gitserver-0.prod.default.namespace",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			have := hostnameMatch(tc.hostname, tc.addr)
			if have != tc.shouldMatch {
				t.Fatalf("Want %v, got %v", tc.shouldMatch, have)
			}
		})
	}
}

func TestSyncRepoState(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := database.NewDB(logger, dbtest.NewDB(t))
	remoteDir := t.TempDir()

	cmd := func(name string, arg ...string) {
		t.Helper()
		runCmd(t, remoteDir, name, arg...)
	}

	// Setup a repo with a commit so we can see if we can clone it.
	cmd("git", "init", ".")
	cmd("sh", "-c", "echo hello world > hello.txt")
	cmd("git", "add", "hello.txt")
	cmd("git", "commit", "-m", "hello")

	reposDir := t.TempDir()
	repoName := api.RepoName("example.com/foo/bar")
	hostname := "test"

	s := makeTestServer(ctx, t, reposDir, remoteDir, db)
	s.hostname = hostname

	dbRepo := &types.Repo{
		Name:        repoName,
		URI:         string(repoName),
		Description: "Test",
	}

	// Insert the repo into our database
	err := db.Repos().Create(ctx, dbRepo)
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.CloneRepo(ctx, repoName, CloneOptions{Block: true})
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		// GitserverRepo should exist after updating the lastFetched time
		t.Fatal(err)
	}

	err = syncRepoState(ctx, logger, db, s.locker, hostname, reposDir, gitserver.GitserverAddresses{Addresses: []string{hostname}}, 10, 10, true)
	if err != nil {
		t.Fatal(err)
	}

	gr, err := db.GitserverRepos().GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	if gr.CloneStatus != types.CloneStatusCloned {
		t.Fatalf("Want %v, got %v", types.CloneStatusCloned, gr.CloneStatus)
	}

	t.Run("sync deleted repo", func(t *testing.T) {
		// Fake setting an incorrect status
		if err := db.GitserverRepos().SetCloneStatus(ctx, dbRepo.Name, types.CloneStatusUnknown, hostname); err != nil {
			t.Fatal(err)
		}

		// We should continue to sync deleted repos
		if err := db.Repos().Delete(ctx, dbRepo.ID); err != nil {
			t.Fatal(err)
		}

		err = syncRepoState(ctx, logger, db, s.locker, hostname, reposDir, gitserver.GitserverAddresses{Addresses: []string{hostname}}, 10, 10, true)
		if err != nil {
			t.Fatal(err)
		}

		gr, err := db.GitserverRepos().GetByID(ctx, dbRepo.ID)
		if err != nil {
			t.Fatal(err)
		}

		if gr.CloneStatus != types.CloneStatusCloned {
			t.Fatalf("Want %v, got %v", types.CloneStatusCloned, gr.CloneStatus)
		}
	})
}

func TestLogIfCorrupt(t *testing.T) {
	logger := logtest.Scoped(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db := database.NewDB(logger, dbtest.NewDB(t))
	remoteDir := t.TempDir()

	reposDir := t.TempDir()
	hostname := "test"

	repoName := api.RepoName("example.com/bar/foo")
	s := makeTestServer(ctx, t, reposDir, remoteDir, db)
	s.hostname = hostname

	t.Run("git corruption output creates corruption log", func(t *testing.T) {
		dbRepo := &types.Repo{
			Name:        repoName,
			URI:         string(repoName),
			Description: "Test",
		}

		// Insert the repo into our database
		err := db.Repos().Create(ctx, dbRepo)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = db.Repos().Delete(ctx, dbRepo.ID)
		})

		stdErr := "error: packfile .git/objects/pack/pack-e26c1fc0add58b7649a95f3e901e30f29395e174.pack does not match index"

		s.LogIfCorrupt(ctx, repoName, common.ErrRepoCorrupted{
			Reason: stdErr,
		})

		fromDB, err := s.db.GitserverRepos().GetByName(ctx, repoName)
		assert.NoError(t, err)
		assert.Len(t, fromDB.CorruptionLogs, 1)
		assert.Contains(t, fromDB.CorruptionLogs[0].Reason, stdErr)
	})

	t.Run("non corruption output does not create corruption log", func(t *testing.T) {
		dbRepo := &types.Repo{
			Name:        repoName,
			URI:         string(repoName),
			Description: "Test",
		}

		// Insert the repo into our database
		err := db.Repos().Create(ctx, dbRepo)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = db.Repos().Delete(ctx, dbRepo.ID)
		})

		s.LogIfCorrupt(ctx, repoName, errors.New("Brought to you by Horsegraph"))

		fromDB, err := s.db.GitserverRepos().GetByName(ctx, repoName)
		assert.NoError(t, err)
		assert.Len(t, fromDB.CorruptionLogs, 0)
	})
}

func TestLinebasedBufferedWriter(t *testing.T) {
	testCases := []struct {
		name   string
		writes []string
		text   string
	}{
		{
			name:   "identity",
			writes: []string{"hello"},
			text:   "hello",
		},
		{
			name:   "single write begin newline",
			writes: []string{"\nhelloworld"},
			text:   "\nhelloworld",
		},
		{
			name:   "single write contains newline",
			writes: []string{"hello\nworld"},
			text:   "hello\nworld",
		},
		{
			name:   "single write end newline",
			writes: []string{"helloworld\n"},
			text:   "helloworld\n",
		},
		{
			name:   "first write end newline",
			writes: []string{"hello\n", "world"},
			text:   "hello\nworld",
		},
		{
			name:   "second write begin newline",
			writes: []string{"hello", "\nworld"},
			text:   "hello\nworld",
		},
		{
			name:   "single write begin return",
			writes: []string{"\rhelloworld"},
			text:   "helloworld",
		},
		{
			name:   "single write contains return",
			writes: []string{"hello\rworld"},
			text:   "world",
		},
		{
			name:   "single write end return",
			writes: []string{"helloworld\r"},
			text:   "helloworld\r",
		},
		{
			name:   "first write contains return",
			writes: []string{"hel\rlo", "world"},
			text:   "loworld",
		},
		{
			name:   "first write end return",
			writes: []string{"hello\r", "world"},
			text:   "world",
		},
		{
			name:   "second write begin return",
			writes: []string{"hello", "\rworld"},
			text:   "world",
		},
		{
			name:   "second write contains return",
			writes: []string{"hello", "wor\rld"},
			text:   "ld",
		},
		{
			name:   "second write ends return",
			writes: []string{"hello", "world\r"},
			text:   "helloworld\r",
		},
		{
			name:   "third write",
			writes: []string{"hello", "world\r", "hola"},
			text:   "hola",
		},
		{
			name:   "progress one write",
			writes: []string{"progress\n1%\r20%\r100%\n"},
			text:   "progress\n100%\n",
		},
		{
			name:   "progress multiple writes",
			writes: []string{"progress\n", "1%\r", "2%\r", "100%"},
			text:   "progress\n100%",
		},
		{
			name:   "one two three four",
			writes: []string{"one\ntwotwo\nthreethreethree\rfourfourfourfour\n"},
			text:   "one\ntwotwo\nfourfourfourfour\n",
		},
		{
			name:   "real git",
			writes: []string{"Cloning into bare repository '/Users/nick/.sourcegraph/repos/github.com/nicksnyder/go-i18n/.git'...\nremote: Counting objects: 2148, done.        \nReceiving objects:   0% (1/2148)   \rReceiving objects: 100% (2148/2148), 473.65 KiB | 366.00 KiB/s, done.\nResolving deltas:   0% (0/1263)   \rResolving deltas: 100% (1263/1263), done.\n"},
			text:   "Cloning into bare repository '/Users/nick/.sourcegraph/repos/github.com/nicksnyder/go-i18n/.git'...\nremote: Counting objects: 2148, done.        \nReceiving objects: 100% (2148/2148), 473.65 KiB | 366.00 KiB/s, done.\nResolving deltas: 100% (1263/1263), done.\n",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var w linebasedBufferedWriter
			for _, write := range testCase.writes {
				_, _ = w.Write([]byte(write))
			}
			assert.Equal(t, testCase.text, w.String())
		})
	}
}
