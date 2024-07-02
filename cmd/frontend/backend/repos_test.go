package backend

import (
	"archive/tar"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"

	mockrequire "github.com/derision-test/go-mockgen/v2/testutil/require"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
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

type gitObjectInfo string

func (oid gitObjectInfo) OID() gitdomain.OID {
	var v gitdomain.OID
	copy(v[:], oid)
	return v
}

type FileData struct {
	Name    string
	Content string
}

// createInMemoryTarArchive creates a tar archive in memory containing multiple files with their given content.
func createInMemoryTarArchive(files []FileData) ([]byte, error) {
	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)

	for _, file := range files {
		// Create a tar header for each file.
		header := &tar.Header{
			Name: file.Name,
			Size: int64(len(file.Content)),
		}

		// Write the header to the tar archive.
		err := tarWriter.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		// Write the content to the tar archive.
		_, err = io.WriteString(tarWriter, file.Content)
		if err != nil {
			return nil, err
		}
	}

	// Close the tar writer to flush the data to the buffer.
	err := tarWriter.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func TestReposGetInventory(t *testing.T) {
	ctx := testContext()

	const (
		wantRepoName = "a"
		wantCommitID = "cccccccccccccccccccccccccccccccccccccccc"
		wantRootOID  = "oid-root"
	)
	gitserverClient := gitserver.NewMockClient()
	gitserverClient.StatFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, path string) (fs.FileInfo, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		return &fileutil.FileInfo{Name_: path, Mode_: os.ModeDir, Sys_: gitObjectInfo(wantRootOID)}, nil
	})
	gitserverClient.ReadDirFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, name string, _ bool) (gitserver.ReadDirIterator, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		switch name {
		case "":
			return gitserver.NewReadDirIteratorFromSlice([]fs.FileInfo{
				&fileutil.FileInfo{Name_: "a", Mode_: os.ModeDir, Sys_: gitObjectInfo("oid-a")},
				&fileutil.FileInfo{Name_: "b.go", Size_: 12},
			}), nil
		case "a":
			return gitserver.NewReadDirIteratorFromSlice([]fs.FileInfo{&fileutil.FileInfo{Name_: "a/c.m", Size_: 24}}), nil
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
		case "c.m":
			data = []byte("@interface X:NSObject {}")
		default:
			panic("unhandled mock ReadFile " + name)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	gitserverClient.ArchiveReaderFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, archiveOptions gitserver.ArchiveOptions) (io.ReadCloser, error) {
		files := []FileData{
			{Name: "b.go", Content: "package main"},
			{Name: "a/c.m", Content: "@interface X:NSObject {}"},
		}

		// Create the in-memory tar archive.
		archiveData, err := createInMemoryTarArchive(files)
		if err != nil {
			t.Fatalf("Failed to create in-memory tar archive: %v", err)
		}

		return io.NopCloser(bytes.NewReader(archiveData)), nil
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
