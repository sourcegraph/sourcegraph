package search

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"unicode/utf8"

	"github.com/sourcegraph/go-diff/diff"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// initGitRepository initializes a new Git repository and runs cmds in a new
// temporary directory (returned as dir).
func initGitRepository(t testing.TB, cmds ...string) string {
	t.Helper()
	dir := t.TempDir()
	cmds = append([]string{"git init"}, cmds...)
	for _, cmd := range cmds {
		out, err := gitCommand(dir, "bash", "-c", cmd).CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}
	return dir
}

func gitCommand(dir, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG="+path.Join(dir, ".git", "config"), "GIT_CONFIG_NOSYSTEM=1", "HOME=/dev/null")
	return c
}

func TestSearch(t *testing.T) {
	cmds := []string{
		"echo lorem ipsum dolor sit amet > file1",
		"git add -A",
		"GIT_COMMITTER_NAME=camden1 " +
			"GIT_COMMITTER_EMAIL=camden1@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden1 " +
			"GIT_AUTHOR_EMAIL=camden1@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit1 ",
		"echo consectetur adipiscing elit > file2",
		"echo consectetur adipiscing elit again > file3",
		"git add -A",
		"GIT_COMMITTER_NAME=camden2 " +
			"GIT_COMMITTER_EMAIL=camden2@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden2 " +
			"GIT_AUTHOR_EMAIL=camden2@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit2",
		"mv file1 file1a",
		"git add -A",
		"GIT_COMMITTER_NAME=camden3 " +
			"GIT_COMMITTER_EMAIL=camden3@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden3 " +
			"GIT_AUTHOR_EMAIL=camden3@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit3",
	}
	dir := initGitRepository(t, cmds...)

	t.Run("match one", func(t *testing.T) {
		query := &protocol.MessageMatches{Expr: "commit2"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir: dir,
			Query:   tree,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 1)
		require.Empty(t, matches[0].ModifiedFiles)
	})

	t.Run("match both, in order", func(t *testing.T) {
		query := &protocol.MessageMatches{Expr: "c"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir: dir,
			Query:   tree,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 3)
		require.Equal(t, matches[0].Author.Name, "camden3")
		require.Equal(t, matches[1].Author.Name, "camden2")
		require.Equal(t, matches[2].Author.Name, "camden1")
	})

	t.Run("and with no operands matches all", func(t *testing.T) {
		query := protocol.NewAnd()
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir: dir,
			Query:   tree,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 3)
	})

	t.Run("match diff content", func(t *testing.T) {
		query := &protocol.DiffMatches{Expr: "ipsum"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir: dir,
			Query:   tree,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 2)
		require.Equal(t, matches[0].Author.Name, "camden3")
		require.Equal(t, matches[1].Author.Name, "camden1")
	})

	t.Run("author matches", func(t *testing.T) {
		query := &protocol.AuthorMatches{Expr: "2"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir: dir,
			Query:   tree,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 1)
		require.Equal(t, matches[0].Author.Name, "camden2")
	})

	t.Run("file matches", func(t *testing.T) {
		query := &protocol.DiffModifiesFile{Expr: "file1"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir:              dir,
			Query:                tree,
			IncludeModifiedFiles: true,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 2)
		require.Equal(t, matches[0].Author.Name, "camden3")
		require.Equal(t, matches[1].Author.Name, "camden1")
	})

	t.Run("and match", func(t *testing.T) {
		query := protocol.NewAnd(
			&protocol.DiffMatches{Expr: "lorem"},
			&protocol.DiffMatches{Expr: "ipsum"},
		)
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir:     dir,
			Query:       tree,
			IncludeDiff: true,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 2)
		require.Equal(t, matches[0].Author.Name, "camden3")
		require.Equal(t, matches[1].Author.Name, "camden1")
		require.Len(t, strings.Split(matches[1].Diff.Content, "\n"), 4)
	})

	t.Run("match all, in order with modified files", func(t *testing.T) {
		query := &protocol.MessageMatches{Expr: "c"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir:              dir,
			Query:                tree,
			IncludeModifiedFiles: true,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)
		require.Len(t, matches, 3)
		require.Equal(t, matches[0].Author.Name, "camden3")
		require.Equal(t, matches[1].Author.Name, "camden2")
		require.Equal(t, matches[2].Author.Name, "camden1")
		require.Equal(t, []string{"file1", "file1a"}, matches[0].ModifiedFiles)
		require.Equal(t, []string{"file2", "file3"}, matches[1].ModifiedFiles)
		require.Equal(t, []string{"file1"}, matches[2].ModifiedFiles)
	})

	t.Run("non utf8 elements", func(t *testing.T) {
		cmds := []string{
			"echo lorem ipsum dolor sit amet > file1",
			"git add -A",
			"GIT_COMMITTER_NAME=camden1 " +
				"GIT_COMMITTER_EMAIL=camden1@ccheek.com " +
				"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
				"GIT_AUTHOR_NAME=\xc0mden " +
				"GIT_AUTHOR_EMAIL=\xc0mden1@ccheek.com " +
				"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
				"git commit -m \xc0mmit1 ",
		}
		dir := initGitRepository(t, cmds...)

		query := &protocol.AuthorMatches{Expr: "c"}
		tree, err := ToMatchTree(query)
		require.NoError(t, err)
		searcher := &CommitSearcher{
			RepoDir:              dir,
			Query:                tree,
			IncludeModifiedFiles: true,
		}
		var matches []*protocol.CommitMatch
		err = searcher.Search(context.Background(), func(match *protocol.CommitMatch) {
			matches = append(matches, match)
		})
		require.NoError(t, err)

		require.Len(t, matches, 1)
		match := matches[0]
		require.True(t, utf8.ValidString(match.Author.Name))
		require.True(t, utf8.ValidString(match.Author.Email))
		require.True(t, utf8.ValidString(match.Message.Content))
	})
}

func TestCommitScanner(t *testing.T) {
	cmds := []string{
		"echo lorem ipsum dolor sit amet > file1",
		"git add -A",
		"GIT_COMMITTER_NAME=camden1 " +
			"GIT_COMMITTER_EMAIL=camden1@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden1 " +
			"GIT_AUTHOR_EMAIL=camden1@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit1 ",
		"echo consectetur adipiscing elit > file2",
		"echo consectetur adipiscing elit again > file3",
		"git add -A",
		"GIT_COMMITTER_NAME=camden2 " +
			"GIT_COMMITTER_EMAIL=camden2@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden2 " +
			"GIT_AUTHOR_EMAIL=camden2@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit2",
		"mv file1 file1a",
		"git add -A",
		"GIT_COMMITTER_NAME=camden3 " +
			"GIT_COMMITTER_EMAIL=camden3@ccheek.com " +
			"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z " +
			"GIT_AUTHOR_NAME=camden3 " +
			"GIT_AUTHOR_EMAIL=camden3@ccheek.com " +
			"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z " +
			"git commit -m commit3",
	}
	dir := initGitRepository(t, cmds...)

	getRaw := func(dir string, includeModifiedFiles bool) []byte {
		var buf bytes.Buffer
		cmd := exec.Command("git", (&CommitSearcher{IncludeModifiedFiles: includeModifiedFiles}).gitArgs()...)
		cmd.Dir = dir
		cmd.Stdout = &buf
		require.NoError(t, cmd.Run())
		return buf.Bytes()
	}

	cases := []struct {
		input    []byte
		expected []*RawCommit
	}{
		{
			input: getRaw(dir, true),
			expected: []*RawCommit{
				{
					Hash:           []byte("f45c8f639eeaaaeecd04e60be5800835382fb879"),
					RefNames:       []byte("HEAD -> refs/heads/master"),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden3"),
					AuthorEmail:    []byte("camden3@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden3"),
					CommitterEmail: []byte("camden3@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit3"),
					ParentHashes:   []byte("fa733abad75875e568c949f54f03a38748435e9b"),
					ModifiedFiles: [][]byte{
						[]byte("R100"),
						[]byte("file1"),
						[]byte("file1a"),
					},
				},
				{
					Hash:           []byte("fa733abad75875e568c949f54f03a38748435e9b"),
					RefNames:       []byte(""),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden2"),
					AuthorEmail:    []byte("camden2@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden2"),
					CommitterEmail: []byte("camden2@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit2"),
					ParentHashes:   []byte("008b80cbf30c8608aec73608becb52168f12d558"),
					ModifiedFiles: [][]byte{
						[]byte("A"),
						[]byte("file2"),
						[]byte("A"),
						[]byte("file3"),
					},
				},
				{
					Hash:           []byte("008b80cbf30c8608aec73608becb52168f12d558"),
					RefNames:       []byte(""),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden1"),
					AuthorEmail:    []byte("camden1@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden1"),
					CommitterEmail: []byte("camden1@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit1"),
					ParentHashes:   []byte(""),
					ModifiedFiles: [][]byte{
						[]byte("A"),
						[]byte("file1"),
					},
				},
			},
		},
		{
			input: getRaw(dir, false),
			expected: []*RawCommit{
				{
					Hash:           []byte("f45c8f639eeaaaeecd04e60be5800835382fb879"),
					RefNames:       []byte("HEAD -> refs/heads/master"),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden3"),
					AuthorEmail:    []byte("camden3@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden3"),
					CommitterEmail: []byte("camden3@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit3"),
					ParentHashes:   []byte("fa733abad75875e568c949f54f03a38748435e9b"),
					ModifiedFiles:  [][]byte{},
				},
				{
					Hash:           []byte("fa733abad75875e568c949f54f03a38748435e9b"),
					RefNames:       []byte(""),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden2"),
					AuthorEmail:    []byte("camden2@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden2"),
					CommitterEmail: []byte("camden2@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit2"),
					ParentHashes:   []byte("008b80cbf30c8608aec73608becb52168f12d558"),
					ModifiedFiles:  [][]byte{},
				},
				{
					Hash:           []byte("008b80cbf30c8608aec73608becb52168f12d558"),
					RefNames:       []byte(""),
					SourceRefs:     []byte("HEAD"),
					AuthorName:     []byte("camden1"),
					AuthorEmail:    []byte("camden1@ccheek.com"),
					AuthorDate:     []byte("1136214245"),
					CommitterName:  []byte("camden1"),
					CommitterEmail: []byte("camden1@ccheek.com"),
					CommitterDate:  []byte("1136214245"),
					Message:        []byte("commit1"),
					ParentHashes:   []byte(""),
					ModifiedFiles:  [][]byte{},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			println(dir)
			scanner := NewCommitScanner(bytes.NewReader(tc.input))
			var output []*RawCommit
			for scanner.Scan() {
				output = append(output, scanner.NextRawCommit())
			}
			require.NoError(t, scanner.Err())
			require.Equal(t, tc.expected, output)
		})
	}
}

func TestHighlights(t *testing.T) {
	rawDiff := `diff --git internal/compute/match.go internal/compute/match.go
new file mode 100644
index 0000000000..fcc91bf673
--- /dev/null
+++ internal/compute/match.go
@@ -0,0 +1,97 @@
+package compute
+
+import (
+       "fmt"
+       "regexp"
+
+       "github.com/sourcegraph/sourcegraph/internal/search/result"
+)
+
+func ofFileMatches(fm *result.FileMatch, r *regexp.Regexp) *Result {
+       matches := make([]Match, 0, len(fm.LineMatches))
+       for _, l := range fm.LineMatches {
+               regexpMatches := r.FindAllStringSubmatchIndex(l.Preview, -1)
+               matches = append(matches, ofRegexpMatches(regexpMatches, l.Preview, int(l.LineNumber)))
+       }
+       return &Result{Matches: matches, Path: fm.Path}
+}
diff --git internal/compute/match_test.go internal/compute/match_test.go
new file mode 100644
index 0000000000..7e54670557
--- /dev/null
+++ internal/compute/match_test.go
@@ -0,0 +1,112 @@
+package compute
+
+import (
+       "encoding/json"
+       "regexp"
+       "testing"
+
+       "github.com/hexops/autogold"
+       "github.com/sourcegraph/sourcegraph/internal/search/result"
+)
+
+func TestOfLineMatches(t *testing.T) {
+       test := func(input string) string {
+               r, _ := regexp.Compile(input)
+               result := ofFileMatches(data, r)
+               v, _ := json.MarshalIndent(result, "", "  ")
+               return string(v)
+       }
+}`

	parsedDiff, err := diff.NewMultiFileDiffReader(strings.NewReader(rawDiff)).ReadAllFiles()
	require.NoError(t, err)

	lc := &LazyCommit{
		RawCommit: &RawCommit{
			AuthorName: []byte("Camden Cheek"),
			ModifiedFiles: [][]byte{
				[]byte("M"),
				[]byte("internal/compute/match.go"),
				[]byte("M"),
				[]byte("internal/compute/match_test.go"),
			},
		},
		diff: parsedDiff,
	}

	mt, err := ToMatchTree(protocol.NewAnd(
		&protocol.AuthorMatches{Expr: "Camden"},
		&protocol.DiffModifiesFile{Expr: "match"},
		protocol.NewOr(
			&protocol.DiffMatches{Expr: "result"},
			&protocol.DiffMatches{Expr: "test"},
		),
	))
	require.NoError(t, err)

	mergedResult, highlights, err := mt.Match(lc)
	require.NoError(t, err)
	require.True(t, mergedResult.Satisfies())

	formatted, ranges := FormatDiff(parsedDiff, highlights.Diff)
	expectedFormatted := "/dev/null internal/compute/match.go\n" +
		"@@ -0,0 +6,6 @@ \n" +
		`+
+       "github.com/sourcegraph/sourcegraph/internal/search/result"
+)
+
+func ofFileMatches(fm *result.FileMatch, r *regexp.Regexp) *Result {
+       matches := make([]Match, 0, len(fm.LineMatches))
/dev/null internal/compute/match_test.go
@@ -0,0 +5,7 @@ ... +4
+       "regexp"
+       "testing"
+
+       "github.com/hexops/autogold"
+       "github.com/sourcegraph/sourcegraph/internal/search/result"
+)
+
`

	require.Equal(t, expectedFormatted, formatted)

	expectedRanges := result.Ranges{{
		Start: result.Location{Offset: 27, Line: 0, Column: 27},
		End:   result.Location{Offset: 32, Line: 0, Column: 32},
	}, {
		Start: result.Location{Offset: 115, Line: 3, Column: 60},
		End:   result.Location{Offset: 121, Line: 3, Column: 66},
	}, {
		Start: result.Location{Offset: 152, Line: 6, Column: 24},
		End:   result.Location{Offset: 158, Line: 6, Column: 30},
	}, {
		Start: result.Location{Offset: 282, Line: 8, Column: 27},
		End:   result.Location{Offset: 287, Line: 8, Column: 32},
	}, {
		Start: result.Location{Offset: 345, Line: 11, Column: 9},
		End:   result.Location{Offset: 349, Line: 11, Column: 13},
	}, {
		Start: result.Location{Offset: 453, Line: 14, Column: 60},
		End:   result.Location{Offset: 459, Line: 14, Column: 66},
	}}

	require.Equal(t, expectedRanges, ranges)

	// check formatting w/ sub-repo perms filtering
	filteredDiff := filterRawDiff(parsedDiff, setupSubRepoFilterFunc())
	formattedWithFiltering, ranges := FormatDiff(filteredDiff, highlights.Diff)
	expectedFormatted = "/dev/null internal/compute/match.go\n" +
		"@@ -0,0 +6,6 @@ \n" +
		`+
+       "github.com/sourcegraph/sourcegraph/internal/search/result"
+)
+
+func ofFileMatches(fm *result.FileMatch, r *regexp.Regexp) *Result {
+       matches := make([]Match, 0, len(fm.LineMatches))
`

	require.Equal(t, expectedFormatted, formattedWithFiltering)

	expectedRanges = expectedRanges[:3]
	require.Equal(t, expectedRanges, ranges)
}

func setupSubRepoFilterFunc() func(string) (bool, error) {
	checker := authz.NewMockSubRepoPermissionChecker()
	ctx := context.Background()
	a := &actor.Actor{
		UID: 1,
	}
	ctx = actor.WithActor(ctx, a)
	checker.EnabledFunc.SetDefaultHook(func() bool {
		return true
	})
	checker.PermissionsFunc.SetDefaultHook(func(ctx context.Context, i int32, content authz.RepoContent) (authz.Perms, error) {
		if strings.Contains(content.Path, "_test.go") {
			return authz.None, nil
		}
		return authz.Read, nil
	})
	return getSubRepoFilterFunc(ctx, checker, "my_repo")
}

func TestFuzzQueryCNF(t *testing.T) {
	matchTreeMatches := func(mt MatchTree, a authorNameGenerator) bool {
		lc := &LazyCommit{
			RawCommit: &RawCommit{
				AuthorName: []byte(a),
			},
		}
		mergedResult, _, err := mt.Match(lc)
		require.NoError(t, err)
		return mergedResult.Satisfies()
	}

	rawQueryMatches := func(q queryGenerator, a authorNameGenerator) bool {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in rawQueryMatches:\n  Query: %s\n  Author: %s\n", q.RawQuery.String(), a)
			}
		}()
		mt, err := ToMatchTree(q.RawQuery)
		require.NoError(t, err)
		return matchTreeMatches(mt, a)
	}

	reducedQueryMatches := func(q queryGenerator, a authorNameGenerator) bool {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in reducedQueryMatches:\n  Query: %s\n  Author: %s\n", q.RawQuery.String(), a)
			}
		}()
		mt, err := ToMatchTree(q.ConstructedQuery())
		require.NoError(t, err)
		return matchTreeMatches(mt, a)
	}

	err := quick.CheckEqual(rawQueryMatches, reducedQueryMatches, nil)
	var e *quick.CheckEqualError
	if err != nil && errors.As(err, &e) {
		t.Fatalf("Different outputs for same inputs\n  RawQuery: %s\n  ReducedQuery: %s\n  AuthorName: %s\n",
			e.In[0].(queryGenerator).RawQuery.String(),
			e.In[0].(queryGenerator).ConstructedQuery().String(),
			string(e.In[1].([]uint8)),
		)
	} else if err != nil {
		t.Fatal(err)
	}
}

// queryGenerator is a type that satisfies the tesing/quick Generator interface,
// generating random, unreduced queries in its RawQuery field. Additionally,
// it exposes a ConstructedQuery() convienence method that allows the caller to get the
// query as if it had been created with the protocol.New* functions.
type queryGenerator struct {
	RawQuery protocol.Node
}

func (queryGenerator) Generate(rand *rand.Rand, size int) reflect.Value {
	// Set max depth to avoid massive trees
	if size > 10 {
		size = 10
	}
	return reflect.ValueOf(queryGenerator{generateQuery(rand, size)})
}

// ConstructedQuery returns the query as if constructted with the protocol.New* functions
func (q queryGenerator) ConstructedQuery() protocol.Node {
	return protocol.Reduce(constructedQuery(q.RawQuery))
}

// constructedQuery takes any query and recursively reduces it with the
// protocol.New* functions. This is not meant to be used outside of fuzz testing
// because any caller should be using the protocol.New* functions directly, which
// reduce the query on construction.
func constructedQuery(q protocol.Node) protocol.Node {
	switch v := q.(type) {
	case *protocol.Operator:
		newOperands := make([]protocol.Node, 0, len(v.Operands))
		for _, operand := range v.Operands {
			newOperands = append(newOperands, constructedQuery(operand))
		}
		switch v.Kind {
		case protocol.And:
			return protocol.NewAnd(newOperands...)
		case protocol.Or:
			return protocol.NewOr(newOperands...)
		case protocol.Not:
			return protocol.NewNot(newOperands[0])
		default:
			panic("unreachable")
		}
	default:
		return v
	}
}

const randomChars = `abcdefghijkl`

// generateAtom generates a random AuthorMatches atom.
// The AuthorMatches node will match a single, random character from `randomChars`.
// 50% of the generated nodes will also be negated. We negate in the atom step
// rather than in the generateQuery step because we only want to generate negated
// nodes if they are wrapping leaf nodes. Negating non-leaf nodes works correctly,
// but can lead to multiple-exponential behavior.
func generateAtom(rand *rand.Rand) protocol.Node {
	a := &protocol.AuthorMatches{
		Expr: string(randomChars[rand.Int()%len(randomChars)]),
	}
	if rand.Int()%2 == 0 {
		return a
	}
	return &protocol.Operator{Kind: protocol.Not, Operands: []protocol.Node{a}}
}

// generateQuery generates a random query with configurable depth. Atom,
// And, and Or nodes will occur with a 1:1:1 ratio on average.
func generateQuery(rand *rand.Rand, depth int) protocol.Node {
	if depth == 0 {
		return generateAtom(rand)
	}

	switch rand.Int() % 3 {
	case 0:
		var operands []protocol.Node
		for range rand.Int() % 4 {
			operands = append(operands, generateQuery(rand, depth-1))
		}
		return &protocol.Operator{Kind: protocol.And, Operands: operands}
	case 1:
		var operands []protocol.Node
		for range rand.Int() % 4 {
			operands = append(operands, generateQuery(rand, depth-1))
		}
		return &protocol.Operator{Kind: protocol.Or, Operands: operands}
	case 2:
		return generateAtom(rand)
	default:
		panic("unreachable")
	}
}

// authorNameGenerator is a type that implements the testing/quick Generator interface
// so it can be randomly generated using the same characters that the AuthorMatches
// nodes are generated with using generateAtom.
type authorNameGenerator []byte

func (authorNameGenerator) Generate(rand *rand.Rand, size int) reflect.Value {
	if size > 10 {
		size = 10
	}
	buf := make([]byte, size)
	for i := range len(buf) {
		buf[i] = randomChars[rand.Int()%len(randomChars)]
	}
	return reflect.ValueOf(buf)
}

func Test_revsToGitArgs(t *testing.T) {
	cases := []struct {
		name     string
		revSpecs []string
		expected []string
	}{
		{
			name:     "explicit HEAD",
			revSpecs: []string{"HEAD"},
			expected: []string{"HEAD"},
		},
		{
			name:     "implicit HEAD",
			revSpecs: []string{""},
			expected: []string{"HEAD"},
		},
	}

	for _, tc := range cases {
		got := revsToGitArgs(tc.revSpecs)
		require.Equal(t, tc.expected, got)
	}
}
