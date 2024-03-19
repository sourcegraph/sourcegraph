package backend

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestReposService_Get(t *testing.T) {
	t.Parallel()

	wantRepo := &types.Repo{ID: 1, Name: "github.com/u/r"}

	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultReturn(wantRepo, nil)
	s := &repos{store: repoStore}

	repo, err := s.Get(context.Background(), 1)
	require.NoError(t, err)
	mockrequire.Called(t, repoStore.GetFunc)
	require.Equal(t, wantRepo, repo)
}

func TestReposService_List(t *testing.T) {
	t.Parallel()

	wantRepos := []*types.Repo{
		{Name: "r1"},
		{Name: "r2"},
	}

	repoStore := dbmocks.NewMockRepoStore()
	repoStore.ListFunc.SetDefaultReturn(wantRepos, nil)
	s := &repos{store: repoStore}

	repos, err := s.List(context.Background(), database.ReposListOptions{})
	require.NoError(t, err)
	mockrequire.Called(t, repoStore.ListFunc)
	require.Equal(t, wantRepos, repos)
}

func TestRepos_AddRepoToSourcegraphDotCom(t *testing.T) {
	var s repos
	ctx := testContext()

	const repoName = "github.com/my/repo"
	const newName = "github.com/my/repo2"

	calledRepoLookup := false
	repoupdater.MockRepoLookup = func(args protocol.RepoLookupArgs) (*protocol.RepoLookupResult, error) {
		calledRepoLookup = true
		if args.Repo != repoName {
			t.Errorf("got %q, want %q", args.Repo, repoName)
		}
		return &protocol.RepoLookupResult{
			Repo: &protocol.RepoInfo{Name: newName, Description: "d"},
		}, nil
	}
	defer func() { repoupdater.MockRepoLookup = nil }()

	// The repoName could change if it has been renamed on the code host
	s = repos{
		logger: logtest.Scoped(t),
		cf: httpcli.DoerFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "https://github.com/my/repo.git/info/refs?service=git-upload-pack", r.URL.String())
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	addedName, err := s.addRepoToSourcegraphDotCom(ctx, repoName)
	if err != nil {
		t.Fatal(err)
	}
	if addedName != newName {
		t.Fatalf("Want %q, got %q", newName, addedName)
	}
	if !calledRepoLookup {
		t.Error("!calledRepoLookup")
	}

	// Verify that non 200 codes return the right error.
	s = repos{
		logger: logtest.Scoped(t),
		cf: httpcli.DoerFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "https://github.com/my/repo.git/info/refs?service=git-upload-pack", r.URL.String())
			return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	_, err = s.addRepoToSourcegraphDotCom(ctx, repoName)
	require.Error(t, err)
	require.IsType(t, &database.RepoNotFoundErr{}, err)
}

func TestRepos_Add_NonPublicCodehosts(t *testing.T) {
	var s repos
	ctx := testContext()

	const repoName = "github.private.corp/my/repo"

	repoupdater.MockRepoLookup = func(args protocol.RepoLookupArgs) (*protocol.RepoLookupResult, error) {
		t.Fatal("unexpected call to repo-updater for non public code host")
		return nil, nil
	}
	defer func() { repoupdater.MockRepoLookup = nil }()

	gitserver.MockIsRepoCloneable = func(name api.RepoName) error {
		t.Fatal("unexpected call to gitserver for non public code host")
		return nil
	}
	defer func() { gitserver.MockIsRepoCloneable = nil }()

	// The repoName could change if it has been renamed on the code host
	_, err := s.addRepoToSourcegraphDotCom(ctx, repoName)
	if !errcode.IsNotFound(err) {
		t.Fatalf("expected a not found error, got: %v", err)
	}
}

type gitObjectInfo string

func (oid gitObjectInfo) OID() gitdomain.OID {
	var v gitdomain.OID
	copy(v[:], oid)
	return v
}

func TestReposGetInventory(t *testing.T) {
	ctx := testContext()

	const (
		wantRepoName = "a"
		wantCommitID = "cccccccccccccccccccccccccccccccccccccccc"
		wantRootOID  = "oid-root"
	)
	gitserverClient := gitserver.NewMockClient()
	repoupdater.MockRepoLookup = func(args protocol.RepoLookupArgs) (*protocol.RepoLookupResult, error) {
		if args.Repo != wantRepoName {
			t.Errorf("got %q, want %q", args.Repo, wantRepoName)
		}
		return &protocol.RepoLookupResult{Repo: &protocol.RepoInfo{Name: wantRepoName}}, nil
	}
	defer func() { repoupdater.MockRepoLookup = nil }()
	gitserverClient.StatFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, path string) (fs.FileInfo, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		return &fileutil.FileInfo{Name_: path, Mode_: os.ModeDir, Sys_: gitObjectInfo(wantRootOID)}, nil
	})
	gitserverClient.ReadDirFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, name string, _ bool) ([]fs.FileInfo, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		switch name {
		case "":
			return []fs.FileInfo{
				&fileutil.FileInfo{Name_: "a", Mode_: os.ModeDir, Sys_: gitObjectInfo("oid-a")},
				&fileutil.FileInfo{Name_: "b.go", Size_: 12},
			}, nil
		case "a":
			return []fs.FileInfo{&fileutil.FileInfo{Name_: "a/c.m", Size_: 24}}, nil
		default:
			panic("unhandled mock ReadDir " + name)
		}
	})
	gitserverClient.NewFileReaderFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, name string) (io.ReadCloser, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		var data []byte
		switch name {
		case "b.go":
			data = []byte("package main")
		case "a/c.m":
			data = []byte("@interface X:NSObject {}")
		default:
			panic("unhandled mock ReadFile " + name)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	s := repos{
		logger:          logtest.Scoped(t),
		gitserverClient: gitserverClient,
	}

	tests := []struct {
		useEnhancedLanguageDetection bool
		want                         *inventory.Inventory
	}{
		{
			useEnhancedLanguageDetection: false,
			want: &inventory.Inventory{
				Languages: []inventory.Lang{
					{Name: "Limbo", TotalBytes: 24, TotalLines: 0}, // obviously incorrect, but this is how the pre-enhanced lang detection worked
					{Name: "Go", TotalBytes: 12, TotalLines: 0},
				},
			},
		},
		{
			useEnhancedLanguageDetection: true,
			want: &inventory.Inventory{
				Languages: []inventory.Lang{
					{Name: "Objective-C", TotalBytes: 24, TotalLines: 1},
					{Name: "Go", TotalBytes: 12, TotalLines: 1},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("useEnhancedLanguageDetection=%v", test.useEnhancedLanguageDetection), func(t *testing.T) {
			rcache.SetupForTest(t)
			orig := useEnhancedLanguageDetection
			useEnhancedLanguageDetection = test.useEnhancedLanguageDetection
			defer func() { useEnhancedLanguageDetection = orig }() // reset

			inv, err := s.GetInventory(ctx, wantRepoName, wantCommitID, false)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(test.want, inv); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		logtest.InitWithLevel(m, log.LevelNone)
	} else {
		logtest.Init(m)
	}
	os.Exit(m.Run())
}
