package vfsutil

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/sourcegraph/ctxvfs"
)

func testVFS(t *testing.T, fs ctxvfs.FileSystem, want map[string]string) {
	tree, err := ctxvfs.ReadAllFiles(context.Background(), fs, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != len(want) {
		t.Errorf("got %d files, want %d files", len(tree), len(want))
	}
	for wantFile, wantContents := range want {
		contentsBytes, ok := tree[wantFile]
		if !ok {
			t.Errorf("missing file %s", wantFile)
			continue
		}
		contents := string(contentsBytes)

		if strings.HasSuffix(wantContents, "...") {
			// Allow specifying expected contents with "..." at the
			// end for ease of test creation.
			if len(contents) >= len(wantContents)+3 {
				contents = contents[:len(wantContents)-3] + "..."
			}
		}
		if contents != wantContents {
			t.Errorf("%s: got contents %q, want %q", wantFile, contents, wantContents)
		}
	}
	for file := range tree {
		if _, ok := want[file]; !ok {
			t.Errorf("extra file %s", file)
		}
	}
	if fsLister, ok := fs.(interface {
		ListAllFiles(context.Context) ([]string, error)
	}); ok {
		got, err := fsLister.ListAllFiles(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		wantList := make([]string, 0, len(want))
		for file := range want {
			wantList = append(wantList, strings.TrimPrefix(file, "/"))
		}
		sort.Strings(got)
		sort.Strings(wantList)
		if !reflect.DeepEqual(got, wantList) {
			t.Fatalf("ListAllFiles does not match want:\ngot:  %v\nwant: %v", got, wantList)
		}
	}
}
