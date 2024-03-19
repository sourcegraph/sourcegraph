package commitgraph

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
)

func TestCalculateVisibleUploads(t *testing.T) {
	// testGraph has the following layout:
	//
	//       +--- b -------------------------------+-- [j]
	//       |                                     |
	// [a] --+         +-- d             +-- [h] --+--- k -- [m]
	//       |         |                 |
	//       +-- [c] --+       +-- [f] --+
	//                 |       |         |
	//                 +-- e --+         +-- [i] ------ l -- [n]
	//                         |
	//                         +--- g
	//
	// NOTE: The input to ParseCommitGraph must match the order and format
	// of `git log --pretty="%H %P" --topo-order`.
	testGraph := gitdomain.ParseCommitGraph([]string{
		"n l",
		"m k",
		"k h",
		"j b h",
		"h f",
		"l i",
		"i f",
		"f e",
		"g e",
		"e c",
		"d c",
		"c a",
		"b a",
	})

	commitGraphView := NewCommitGraphView()
	commitGraphView.Add(UploadMeta{UploadID: 45}, "n", "sub3/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 50}, "a", "sub1/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 51}, "j", "sub2/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 52}, "c", "sub3/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 53}, "f", "sub3/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 54}, "i", "sub3/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 55}, "h", "sub3/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 56}, "m", "sub3/:lsif-go")

	visibleUploads, links := makeTestGraph(testGraph, commitGraphView)

	expectedVisibleUploads := map[string][]UploadMeta{
		"a": {{UploadID: 50, Distance: 0}},
		"b": {{UploadID: 50, Distance: 1}},
		"c": {{UploadID: 50, Distance: 1}, {UploadID: 52, Distance: 0}},
		"f": {{UploadID: 50, Distance: 3}, {UploadID: 53, Distance: 0}},
		"i": {{UploadID: 50, Distance: 4}, {UploadID: 54, Distance: 0}},
		"h": {{UploadID: 50, Distance: 4}, {UploadID: 55, Distance: 0}},
		"j": {{UploadID: 50, Distance: 2}, {UploadID: 51, Distance: 0}, {UploadID: 55, Distance: 1}},
		"m": {{UploadID: 50, Distance: 6}, {UploadID: 56, Distance: 0}},
		"n": {{UploadID: 45, Distance: 0}, {UploadID: 50, Distance: 6}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, visibleUploads); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]LinkRelationship{
		"d": {Commit: "d", AncestorCommit: "c", Distance: 1},
		"e": {Commit: "e", AncestorCommit: "c", Distance: 1},
		"g": {Commit: "g", AncestorCommit: "c", Distance: 2},
		"k": {Commit: "k", AncestorCommit: "h", Distance: 1},
		"l": {Commit: "l", AncestorCommit: "i", Distance: 1},
	}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected links (-want +got):\n%s", diff)
	}
}

func TestCalculateVisibleUploadsAlternateCommitGraph(t *testing.T) {
	// testGraph has the following layout:
	//
	//       [b] ------+                                          +------ n --- p
	//                 |                                          |
	//             +-- d --+                                  +-- l --+
	//             |       |                                  |       |
	// [a] -- c ---+       +-- f -- g -- h -- [i] -- j -- k --+       +-- o -- [q]
	//             |       |                                  |       |
	//             +-- e --+                                  +-- m --+
	//
	// NOTE: The input to ParseCommitGraph must match the order and format
	// of `git log --topo-sort`.
	testGraph := gitdomain.ParseCommitGraph([]string{
		"q o",
		"p n",
		"o l m",
		"n l",
		"m k",
		"l k",
		"k j",
		"j i",
		"i h",
		"h g",
		"g f",
		"f d e",
		"e c",
		"d b c",
		"c a",
	})

	commitGraphView := NewCommitGraphView()
	commitGraphView.Add(UploadMeta{UploadID: 50}, "a", "sub1/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 51}, "b", "sub1/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 52}, "i", "sub2/:lsif-go")
	commitGraphView.Add(UploadMeta{UploadID: 53}, "q", "sub3/:lsif-go")

	visibleUploads, links := makeTestGraph(testGraph, commitGraphView)

	expectedVisibleUploads := map[string][]UploadMeta{
		"a": {{UploadID: 50, Distance: 0}},
		"b": {{UploadID: 51, Distance: 0}},
		"c": {{UploadID: 50, Distance: 1}},
		"d": {{UploadID: 51, Distance: 1}},
		"e": {{UploadID: 50, Distance: 2}},
		"f": {{UploadID: 51, Distance: 2}},
		"g": {{UploadID: 51, Distance: 3}},
		"h": {{UploadID: 51, Distance: 4}},
		"i": {{UploadID: 51, Distance: 5}, {UploadID: 52, Distance: 0}},
		"l": {{UploadID: 51, Distance: 8}, {UploadID: 52, Distance: 3}},
		"m": {{UploadID: 51, Distance: 8}, {UploadID: 52, Distance: 3}},
		"o": {{UploadID: 51, Distance: 9}, {UploadID: 52, Distance: 4}},
		"q": {{UploadID: 51, Distance: 10}, {UploadID: 52, Distance: 5}, {UploadID: 53, Distance: 0}},
	}
	if diff := cmp.Diff(expectedVisibleUploads, visibleUploads); diff != "" {
		t.Errorf("unexpected visible uploads (-want +got):\n%s", diff)
	}

	expectedLinks := map[string]LinkRelationship{
		"j": {Commit: "j", AncestorCommit: "i", Distance: 1},
		"k": {Commit: "k", AncestorCommit: "i", Distance: 2},
		"n": {Commit: "n", AncestorCommit: "l", Distance: 1},
		"p": {Commit: "p", AncestorCommit: "l", Distance: 2},
	}
	if diff := cmp.Diff(expectedLinks, links); diff != "" {
		t.Errorf("unexpected links (-want +got):\n%s", diff)
	}
}

//
// Benchmarks
//

func BenchmarkCalculateVisibleUploads(b *testing.B) {
	commitGraph, err := readBenchmarkCommitGraph()
	if err != nil {
		b.Fatalf("unexpected error reading benchmark commit graph: %s", err)
	}
	commitGraphView, err := readBenchmarkCommitGraphView()
	if err != nil {
		b.Fatalf("unexpected error reading benchmark commit graph view: %s", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	uploadsByCommit, links := NewGraph(commitGraph, commitGraphView).Gather()

	var numUploads int
	for uploads := range uploadsByCommit {
		numUploads += len(uploads)
	}

	fmt.Printf("\nNum Uploads: %d\nNum Links:   %d\n\n", numUploads, len(links))
}

const customer = "customer1"

func readBenchmarkCommitGraph() (*gitdomain.CommitGraph, error) {
	contents, err := readBenchmarkFile(filepath.Join("testdata", customer, "commits.txt.gz"))
	if err != nil {
		return nil, err
	}

	return gitdomain.ParseCommitGraph(strings.Split(string(contents), "\n")), nil
}

func readBenchmarkCommitGraphView() (*CommitGraphView, error) {
	contents, err := readBenchmarkFile(filepath.Join("testdata", customer, "uploads.csv.gz"))
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(bytes.NewReader(contents))

	commitGraphView := NewCommitGraphView()
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, err
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, err
		}

		commitGraphView.Add(
			UploadMeta{UploadID: id},             // meta
			record[1],                            // commit
			fmt.Sprintf("%s:lsif-go", record[2]), // token = hash({root}:{indexer})
		)
	}

	return commitGraphView, nil
}

func readBenchmarkFile(path string) ([]byte, error) {
	uploadsFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer uploadsFile.Close()

	r, err := gzip.NewReader(uploadsFile)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	contents, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

// makeTestGraph calls Gather on a new graph then sorts the uploads deterministically
// for easier comparison. Order of the upload list is not relevant to production flows.
func makeTestGraph(commitGraph *gitdomain.CommitGraph, commitGraphView *CommitGraphView) (uploads map[string][]UploadMeta, links map[string]LinkRelationship) {
	uploads, links = NewGraph(commitGraph, commitGraphView).Gather()
	for _, us := range uploads {
		sort.Slice(us, func(i, j int) bool {
			return us[i].UploadID-us[j].UploadID < 0
		})
	}

	return uploads, links
}
