package graphqlbackend

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/inventory"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

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

func TestSearchResultsStatsLanguages(t *testing.T) {
	logger := logtest.Scoped(t)
	wantCommitID := api.CommitID(strings.Repeat("a", 40))
	rcache.SetupForTest(t)

	gsClient := gitserver.NewMockClient()
	gsClient.NewFileReaderFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, commit api.CommitID, name string) (io.ReadCloser, error) {
		if commit != wantCommitID {
			t.Errorf("got commit %q, want %q", commit, wantCommitID)
		}
		var data []byte
		switch name {
		case "two.go":
			data = []byte("a\nb\n")
		case "three.go":
			data = []byte("a\nb\nc\n")
		default:
			panic("unhandled mock NewFileReader " + name)
		}
		return io.NopCloser(bytes.NewReader(data)), nil
	})
	const wantDefaultBranchRef = "refs/heads/foo"
	gsClient.GetDefaultBranchFunc.SetDefaultHook(func(context.Context, api.RepoName, bool) (string, api.CommitID, error) {
		// Mock default branch lookup in (*RepositoryResolver).DefaultBranch.
		return wantDefaultBranchRef, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil
	})
	gsClient.ResolveRevisionFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, spec string, _ gitserver.ResolveRevisionOptions) (api.CommitID, error) {
		if want := "HEAD"; spec != want {
			t.Errorf("got spec %q, want %q", spec, want)
		}
		return wantCommitID, nil
	})

	gsClient.StatFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, _ api.CommitID, path string) (fs.FileInfo, error) {
		return &fileutil.FileInfo{Name_: path, Mode_: os.ModeDir}, nil
	})
	gsClient.ArchiveReaderFunc.SetDefaultHook(func(_ context.Context, _ api.RepoName, archiveOptions gitserver.ArchiveOptions) (io.ReadCloser, error) {
		files := []FileData{
			{Name: "two.go", Content: "a\nb\n"},
			{Name: "three.go", Content: "a\nb\nc\n"},
		}

		// Create the in-memory tar archive.
		archiveData, err := createInMemoryTarArchive(files)
		if err != nil {
			t.Fatalf("Failed to create in-memory tar archive: %v", err)
		}

		return io.NopCloser(bytes.NewReader(archiveData)), nil
	})

	mkResult := func(path string, lineNumbers ...int) *result.FileMatch {
		rn := types.MinimalRepo{
			Name: "r",
		}
		fm := mkFileMatch(rn, path, lineNumbers...)
		fm.CommitID = wantCommitID
		return fm
	}

	tests := map[string]struct {
		results  []result.Match
		getFiles []fs.FileInfo
		want     []inventory.Lang // TotalBytes values are incorrect (known issue doc'd in GraphQL schema)
	}{
		"empty": {
			results: nil,
			want:    []inventory.Lang{},
		},
		"1 entire file": {
			results: []result.Match{
				mkResult("three.go"),
			},
			want: []inventory.Lang{{Name: "Go", TotalBytes: 6, TotalLines: 3}},
		},
		"line matches in 1 file": {
			results: []result.Match{
				mkResult("three.go", 1),
			},
			want: []inventory.Lang{{Name: "Go", TotalBytes: 6, TotalLines: 1}},
		},
		"line matches in 2 files": {
			results: []result.Match{
				mkResult("two.go", 1, 2),
				mkResult("three.go", 1),
			},
			want: []inventory.Lang{{Name: "Go", TotalBytes: 10, TotalLines: 3}},
		},
		"1 entire repo": {
			results: []result.Match{
				&result.RepoMatch{Name: "r"},
			},
			getFiles: []fs.FileInfo{
				fileInfo{path: "two.go"},
				fileInfo{path: "three.go"},
			},
			want: []inventory.Lang{{Name: "Go", TotalBytes: 10, TotalLines: 5}},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			gsClient.ReadDirFunc.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, string, bool) (gitserver.ReadDirIterator, error) {
				return gitserver.NewReadDirIteratorFromSlice(test.getFiles), nil
			})

			langs, err := searchResultsStatsLanguages(context.Background(), logger, dbmocks.NewMockDB(), gsClient, test.results)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(langs, test.want) {
				t.Errorf("got %+v, want %+v", langs, test.want)
			}
		})
	}
}
