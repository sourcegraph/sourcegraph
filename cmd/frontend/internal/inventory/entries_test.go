package inventory

import (
	"bytes"
	"context"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"io"
	"io/fs"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/fileutil"
)

func TestContext_Entries(t *testing.T) {
	var (
		readTreeCalls      []string
		newFileReaderCalls []string
		cacheGetCalls      []string
		cacheSetCalls      = map[string]Inventory{}
	)
	var mu sync.Mutex
	c := Context{
		ReadTree: func(ctx context.Context, path string) ([]fs.FileInfo, error) {
			mu.Lock()
			defer mu.Unlock()
			readTreeCalls = append(readTreeCalls, path)
			switch path {
			case "d":
				return []fs.FileInfo{
					&fileutil.FileInfo{Name_: "d/a", Mode_: os.ModeDir},
					&fileutil.FileInfo{Name_: "d/b.go", Size_: 12},
				}, nil
			case "d/a":
				return []fs.FileInfo{&fileutil.FileInfo{Name_: "d/a/c.m", Size_: 24}}, nil
			default:
				panic("unhandled mock ReadTree " + path)
			}
		},
		NewFileReader: func(ctx context.Context, path string) (io.ReadCloser, error) {
			mu.Lock()
			defer mu.Unlock()
			newFileReaderCalls = append(newFileReaderCalls, path)
			var data []byte
			switch path {
			case "f.go":
				data = []byte("package f")
			case "d/b.go":
				data = []byte("package main")
			case "d/a/c.m":
				data = []byte("@interface X:NSObject {}")
			default:
				panic("unhandled mock ReadFile " + path)
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
		CacheGet: func(ctx context.Context, cacheKey string, commitID api.CommitID) (Inventory, bool) {
			mu.Lock()
			defer mu.Unlock()
			cacheGetCalls = append(cacheGetCalls, cacheKey)
			return Inventory{}, false
		},
		CacheSet: func(ctx context.Context, cacheKey string, commitID api.CommitID, inv Inventory) {
			mu.Lock()
			defer mu.Unlock()
			if _, ok := cacheSetCalls[cacheKey]; ok {
				t.Fatalf("already stored %q in cache", cacheKey)
			}
			cacheSetCalls[cacheKey] = inv
		},
		CacheKey: func(e fs.FileInfo) string {
			return e.Name()
		},
	}

	inv, err := c.Entries(context.Background(),
		"HEAD",
		&fileutil.FileInfo{Name_: "d", Mode_: os.ModeDir},
		&fileutil.FileInfo{Name_: "f.go", Mode_: 0, Size_: 1 /* HACK to force read */},
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Inventory{
		Languages: []Lang{
			{Name: "Go", TotalBytes: 21, TotalLines: 2},
			{Name: "Objective-C", TotalBytes: 24, TotalLines: 1},
		},
	}); !reflect.DeepEqual(inv, want) {
		t.Fatalf("got  %#v\nwant %#v", inv, want)
	}

	// Check that our mocks were called as expected.
	if want := []string{"d", "d/a"}; !reflect.DeepEqual(readTreeCalls, want) {
		t.Errorf("ReadTree calls: got %q, want %q", readTreeCalls, want)
	}
	if len(newFileReaderCalls) != 3 {
		t.Errorf("GetFileReader calls: got %d, want %d", len(newFileReaderCalls), 3)
	}
	if want := []string{"d", "d/a", "f.go"}; !reflect.DeepEqual(cacheGetCalls, want) {
		t.Errorf("CacheGet calls: got %q, want %q", cacheGetCalls, want)
	}
	want := map[string]Inventory{
		"d": {
			Languages: []Lang{
				{Name: "Objective-C", TotalBytes: 24, TotalLines: 1},
				{Name: "Go", TotalBytes: 12, TotalLines: 1},
			},
		},
		"d/a": {
			Languages: []Lang{
				{Name: "Objective-C", TotalBytes: 24, TotalLines: 1},
			},
		},
		"f.go": {
			Languages: []Lang{
				{Name: "Go", TotalBytes: 9, TotalLines: 1},
			},
		},
	}
	if diff := cmp.Diff(want, cacheSetCalls); diff != "" {
		t.Error(diff)
	}
}
