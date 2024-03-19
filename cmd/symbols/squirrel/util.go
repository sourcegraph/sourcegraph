package squirrel

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NodeId is a nominal type for the ID of a tree-sitter node.
type NodeId string

// walk walks every node in the tree-sitter tree, calling f(node) on each node.
func walk(node *sitter.Node, f func(node *sitter.Node)) {
	walkFilter(node, func(n *sitter.Node) bool { f(n); return true })
}

// walkFilter walks every node in the tree-sitter tree, calling f(node) on each node and descending into
// children if it returns true.
func walkFilter(node *sitter.Node, f func(node *sitter.Node) bool) {
	if f(node) {
		for i := range int(node.ChildCount()) {
			walkFilter(node.Child(i), f)
		}
	}
}

// nodeId returns the ID of the node.
func nodeId(node *sitter.Node) NodeId {
	return NodeId(fmt.Sprint(nodeToRange(node)))
}

// getRoot returns the root node of the tree-sitter tree, given any node inside it.
func getRoot(node *sitter.Node) *sitter.Node {
	var top *sitter.Node
	for cur := node; cur != nil; cur = cur.Parent() {
		top = cur
	}
	return top
}

// isLessRange compares ranges.
func isLessRange(a, b types.Range) bool {
	if a.Row == b.Row {
		return a.Column < b.Column
	}
	return a.Row < b.Row
}

// tabsToSpaces converts tabs to spaces.
func tabsToSpaces(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

const tabSize = 4

// lengthInSpaces returns the length of the string in spaces (using tabSize).
func lengthInSpaces(s string) int {
	total := 0
	for i := range len(s) {
		if s[i] == '\t' {
			total += tabSize
		} else {
			total++
		}
	}
	return total
}

// spacesToColumn measures the length in spaces from the start of the string to the given column.
func spacesToColumn(s string, column int) int {
	total := 0
	for i := range len(s) {
		if total >= column {
			return i
		}

		if s[i] == '\t' {
			total += tabSize
		} else {
			total++
		}
	}
	return total
}

// colorSprintfFunc is a color printing function.
type colorSprintfFunc func(a ...any) string

// bracket prefixes all the lines of the given string with pretty brackets.
func bracket(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) == 1 {
		return "- " + text
	}

	for i, line := range lines {
		if i == 0 {
			lines[i] = "┌ " + line
		} else if i < len(lines)-1 {
			lines[i] = "│ " + line
		} else {
			lines[i] = "└ " + line
		}
	}

	return strings.Join(lines, "\n")
}

func withQuery(query string, node Node, f func(query *sitter.Query, cursor *sitter.QueryCursor)) error {
	sitterQuery, err := sitter.NewQuery([]byte(query), node.LangSpec.language)
	if err != nil {
		return errors.Newf("failed to parse query: %s\n%s", err, query)
	}
	defer sitterQuery.Close()
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(sitterQuery, node.Node)

	f(sitterQuery, cursor)

	return nil
}

// forEachCapture runs the given tree-sitter query on the given node and calls f(captureName, node) for
// each capture.
func forEachCapture(query string, node Node, f func(map[string]Node)) {
	withQuery(query, node, func(sitterQuery *sitter.Query, cursor *sitter.QueryCursor) {
		match, _, hasCapture := cursor.NextCapture()
		for hasCapture {
			nameToNode := map[string]Node{}
			for _, capture := range match.Captures {
				captureName := sitterQuery.CaptureNameForId(capture.Index)
				nameToNode[captureName] = Node{
					RepoCommitPath: node.RepoCommitPath,
					Node:           capture.Node,
					Contents:       node.Contents,
					LangSpec:       node.LangSpec,
				}
			}
			f(nameToNode)
			match, _, hasCapture = cursor.NextCapture()
		}
	})
}

func allCaptures(query string, node Node) []Node {
	var captures []Node
	withQuery(query, node, func(sitterQuery *sitter.Query, cursor *sitter.QueryCursor) {
		match, _, hasCapture := cursor.NextCapture()
		for hasCapture {
			for _, capture := range match.Captures {
				captures = append(captures, Node{
					RepoCommitPath: node.RepoCommitPath,
					Node:           capture.Node,
					Contents:       node.Contents,
					LangSpec:       node.LangSpec,
				})
			}
			match, _, hasCapture = cursor.NextCapture()
		}
	})

	return captures
}

// nodeToRange returns the range of the node.
func nodeToRange(node *sitter.Node) types.Range {
	length := 1
	if node.StartPoint().Row == node.EndPoint().Row {
		length = int(node.EndPoint().Column - node.StartPoint().Column)
	}
	return types.Range{
		Row:    int(node.StartPoint().Row),
		Column: int(node.StartPoint().Column),
		Length: length,
	}
}

// nodeLength returns the length of the node.
func nodeLength(node *sitter.Node) int {
	length := 1
	if node.StartPoint().Row == node.EndPoint().Row {
		length = int(node.EndPoint().Column - node.StartPoint().Column)
	}
	return length
}

// Node is a sitter.Node plus convenient info.
type Node struct {
	RepoCommitPath types.RepoCommitPath
	*sitter.Node
	Contents []byte
	LangSpec LangSpec
}

func swapNode(other Node, newNode *sitter.Node) Node {
	return Node{
		RepoCommitPath: other.RepoCommitPath,
		Node:           newNode,
		Contents:       other.Contents,
		LangSpec:       other.LangSpec,
	}
}

func swapNodePtr(other Node, newNode *sitter.Node) *Node {
	ret := swapNode(other, newNode)
	return &ret
}

// CAUTION: These error messages are checked by client-side code,
// so make sure to update clients if changing them.
var UnrecognizedFileExtensionError = errors.New("unrecognized file extension")
var UnsupportedLanguageError = errors.New("unsupported language")

// Parses a file and returns info about it.
func (s *SquirrelService) parse(ctx context.Context, repoCommitPath types.RepoCommitPath) (*Node, error) {
	ext := filepath.Base(repoCommitPath.Path)
	if strings.Contains(ext, ".") {
		ext = strings.TrimPrefix(filepath.Ext(repoCommitPath.Path), ".")
	}

	langName, ok := extToLang[ext]
	if !ok {
		// It is not uncommon to have files with upper-case extensions
		// like .C, .H, .CPP etc., especially for code developed on
		// case-insensitive filesystems. So check if lower-casing helps.
		//
		// It might be tempting to refactor this code to store all the
		// extensions in lower-case and do one lookup instead of two,
		// but that would be incorrect as we want to distinguish files
		// named 'build' (a common name for shell scripts) vs BUILD
		//// (file extension for Bazel).
		if langName, ok = extToLang[strings.ToLower(ext)]; !ok {
			return nil, UnrecognizedFileExtensionError
		}
	}

	langSpec, ok := langToLangSpec[langName]
	if !ok {
		return nil, UnsupportedLanguageError
	}

	s.parser.SetLanguage(langSpec.language)

	contents, err := s.readFile(ctx, repoCommitPath)
	if err != nil {
		return nil, err
	}

	tree, err := s.parser.ParseCtx(ctx, nil, contents)
	if err != nil {
		return nil, errors.Newf("failed to parse file contents: %s", err)
	}
	s.closables = append(s.closables, tree.Close)

	root := tree.RootNode()
	if root == nil {
		return nil, errors.New("root is nil")
	}
	if s.errorOnParseFailure && root.HasError() {
		return nil, errors.Newf("parse error in %+v, try pasting it in https://tree-sitter.github.io/tree-sitter/playground to find the ERROR node", repoCommitPath)
	}

	return &Node{RepoCommitPath: repoCommitPath, Node: root, Contents: contents, LangSpec: langSpec}, nil
}

func (s *SquirrelService) getSymbols(ctx context.Context, repoCommitPath types.RepoCommitPath) (result.Symbols, error) { //nolint:unparam
	root, err := s.parse(context.Background(), repoCommitPath)
	if err != nil {
		return nil, err
	}

	symbols := result.Symbols{}

	query := root.LangSpec.topLevelSymbolsQuery
	if query == "" {
		return nil, nil
	}

	captures := allCaptures(query, *root)
	for _, capture := range captures {
		symbols = append(symbols, result.Symbol{
			Name:        capture.Node.Content(root.Contents),
			Path:        root.RepoCommitPath.Path,
			Line:        int(capture.Node.StartPoint().Row),
			Character:   int(capture.Node.StartPoint().Column),
			Kind:        "",
			Language:    root.LangSpec.name,
			Parent:      "",
			ParentKind:  "",
			Signature:   "",
			FileLimited: false,
		})
	}

	return symbols, nil
}

func fatalIfError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func fatalIfErrorLabel(t *testing.T, err error, label string) {
	if err != nil {
		_, file, no, ok := runtime.Caller(1)
		if !ok {
			t.Fatalf("%s: %s\n", label, err)
		}
		fmt.Printf("%s:%d %s\n", file, no, err)
		t.FailNow()
	}
}

func children(node *sitter.Node) []*sitter.Node {
	if node == nil {
		return nil
	}
	var children []*sitter.Node
	for i := range int(node.NamedChildCount()) {
		children = append(children, node.NamedChild(i))
	}
	return children
}

func snippet(node *Node) string {
	contextChars := 5
	start := int(node.StartByte()) - contextChars
	if start < 0 {
		start = 0
	}
	end := int(node.StartByte()) + contextChars
	if end > len(node.Contents) {
		end = len(node.Contents)
	}
	ret := string(node.Contents[start:end])
	ret = strings.ReplaceAll(ret, "\n", "\\n")
	ret = strings.ReplaceAll(ret, "\t", "\\t")
	return ret
}

type String string

func (f String) String() string {
	return string(f)
}

type Tuple []interface{}

func (t *Tuple) String() string {
	s := []string{}
	for _, v := range *t {
		s = append(s, fmt.Sprintf("%v", v))
	}
	return strings.Join(s, ", ")
}

func lazyNodeStringer(node **Node) func() fmt.Stringer {
	return func() fmt.Stringer {
		if node != nil && *node != nil {
			if (*node).Node != nil {
				return String(fmt.Sprintf("%s ...%s...", (*node).Type(), snippet(*node)))
			} else {
				return String((*node).RepoCommitPath.Path)
			}
		} else {
			return String("<nil>")
		}
	}
}

func (s *SquirrelService) symbolSearchOne(ctx context.Context, repo string, commit string, include []string, ident string) (*Node, error) {
	symbols, err := s.symbolSearch(ctx, search.SymbolsParameters{
		Repo:            api.RepoName(repo),
		CommitID:        api.CommitID(commit),
		Query:           fmt.Sprintf("^%s$", ident),
		IsRegExp:        true,
		IsCaseSensitive: true,
		IncludePatterns: include,
		ExcludePattern:  "",
		First:           1,
	})
	if err != nil {
		return nil, err
	}
	if len(symbols) == 0 {
		return nil, nil
	}
	symbol := symbols[0]
	file, err := s.parse(ctx, types.RepoCommitPath{
		Repo:   repo,
		Commit: commit,
		Path:   symbol.Path,
	})
	if errors.Is(err, UnsupportedLanguageError) || errors.Is(err, UnrecognizedFileExtensionError) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	point := sitter.Point{
		Row:    uint32(symbol.Line),
		Column: uint32(symbol.Character),
	}
	symbolNode := file.NamedDescendantForPointRange(point, point)
	if symbolNode == nil {
		return nil, nil
	}
	ret := swapNode(*file, symbolNode)
	return &ret, nil
}
