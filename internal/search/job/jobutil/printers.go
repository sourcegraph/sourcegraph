package jobutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/run"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/structural"
	"github.com/sourcegraph/sourcegraph/internal/search/symbol"
	"github.com/sourcegraph/sourcegraph/internal/search/zoekt"
)

func writeSep(b *bytes.Buffer, sep, indent string, depth int) {
	b.WriteString(sep)
	if indent == "" {
		return
	}
	for i := 0; i < depth; i++ {
		b.WriteString(indent)
	}
}

// SexpFormat controls the s-expression format that represents a Job. `sep`
// specifies the separator between terms. If `indent` is not empty, `indent` is
// prefixed the number of times corresponding to depth of the term in the tree.
// See the `Sexp` and `PrettySexp` convenience functions to see how these
// options are used.
func SexpFormat(j job.Job, sep, indent string) string {
	b := new(bytes.Buffer)
	depth := 0
	var writeSexp func(job.Job)
	writeSexp = func(j job.Job) {
		if j == nil {
			return
		}
		switch j := j.(type) {
		case
			*zoekt.ZoektRepoSubsetSearchJob,
			*zoekt.ZoektSymbolSearchJob,
			*searcher.SearcherJob,
			*searcher.SymbolSearcherJob,
			*run.RepoSearchJob,
			*zoekt.ZoektGlobalSearchJob,
			*structural.StructuralSearchJob,
			*commit.CommitSearchJob,
			*symbol.ReferenceSearcherJob,
			*symbol.RepoUniverseSymbolSearchJob,
			*repos.ComputeExcludedReposJob,
			*NoopJob:
			b.WriteString(j.Name())

		case *repoPagerJob:
			b.WriteString("REPOPAGER")
			depth++
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--

		case *AndJob:
			b.WriteString("(AND")
			depth++
			for _, child := range j.children {
				writeSep(b, sep, indent, depth)
				writeSexp(child)
			}
			b.WriteString(")")
			depth--
		case *OrJob:
			b.WriteString("(OR")
			depth++
			for _, child := range j.children {
				writeSep(b, sep, indent, depth)
				writeSexp(child)
			}
			b.WriteString(")")
			depth--
		case *ParallelJob:
			b.WriteString("(PARALLEL")
			depth++
			for _, child := range j.children {
				writeSep(b, sep, indent, depth)
				writeSexp(child)
			}
			depth--
			b.WriteString(")")
		case *SequentialJob:
			b.WriteString("(SEQUENTIAL")
			depth++
			for _, child := range j.children {
				writeSep(b, sep, indent, depth)
				writeSexp(child)
			}
			depth--
			b.WriteString(")")
		case *TimeoutJob:
			b.WriteString("(TIMEOUT")
			depth++
			writeSep(b, sep, indent, depth)
			b.WriteString(j.timeout.String())
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--
		case *LimitJob:
			b.WriteString("(LIMIT")
			depth++
			writeSep(b, sep, indent, depth)
			b.WriteString(strconv.Itoa(j.limit))
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--
		case *subRepoPermsFilterJob:
			b.WriteString("(FILTER")
			depth++
			writeSep(b, sep, indent, depth)
			b.WriteString("SubRepoPermissions")
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--
		case *selectJob:
			b.WriteString("(SELECT")
			depth++
			writeSep(b, sep, indent, depth)
			b.WriteString(j.path.String())
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--
		case *alertJob:
			b.WriteString("(ALERT")
			depth++
			writeSep(b, sep, indent, depth)
			writeSexp(j.child)
			b.WriteString(")")
			depth--
		default:
			panic(fmt.Sprintf("unsupported job %T for SexpFormat printer", j))
		}
	}
	writeSexp(j)
	return b.String()
}

// Sexp outputs the s-expression on a single line.
func Sexp(j job.Job) string {
	return SexpFormat(j, " ", "")
}

// PrettySexp outputs a formatted s-expression with two spaces of indentation, potentially spanning multiple lines.
func PrettySexp(j job.Job) string {
	return SexpFormat(j, "\n", "  ")
}

type NodeStyle int

const (
	DefaultStyle NodeStyle = iota
	RoundedStyle
)

func writeEdge(b *bytes.Buffer, depth, src, dst int) {
	b.WriteString(strconv.Itoa(src))
	b.WriteString("---")
	b.WriteString(strconv.Itoa(dst))
	writeSep(b, "\n", "  ", depth)
}

func writeNode(b *bytes.Buffer, depth int, style NodeStyle, id *int, label string) {
	open := "["
	close := "]"
	if style == RoundedStyle {
		open = "(["
		close = "])"
	}
	b.WriteString(strconv.Itoa(*id))
	b.WriteString(open)
	b.WriteString(label)
	b.WriteString(close)
	writeSep(b, "\n", "  ", depth)
	*id++
}

// PrettyMermaid outputs a Mermaid flowchart. See https://mermaid-js.github.io.
func PrettyMermaid(j job.Job) string {
	depth := 0
	id := 0
	b := new(bytes.Buffer)
	b.WriteString("flowchart TB\n")
	var writeMermaid func(job.Job)
	writeMermaid = func(j job.Job) {
		if j == nil {
			return
		}
		switch j := j.(type) {
		case
			*zoekt.ZoektRepoSubsetSearchJob,
			*zoekt.ZoektSymbolSearchJob,
			*searcher.SearcherJob,
			*searcher.SymbolSearcherJob,
			*run.RepoSearchJob,
			*zoekt.ZoektGlobalSearchJob,
			*structural.StructuralSearchJob,
			*commit.CommitSearchJob,
			*symbol.ReferenceSearcherJob,
			*symbol.RepoUniverseSymbolSearchJob,
			*repos.ComputeExcludedReposJob,
			*NoopJob:
			writeNode(b, depth, RoundedStyle, &id, j.Name())

		case *repoPagerJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "REPOPAGER")
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		case *AndJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "AND")
			for _, child := range j.children {
				writeEdge(b, depth, srcId, id)
				writeMermaid(child)
			}
			depth--
		case *OrJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "OR")
			for _, child := range j.children {
				writeEdge(b, depth, srcId, id)
				writeMermaid(child)
			}
			depth--
		case *ParallelJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "PARALLEL")
			for _, child := range j.children {
				writeEdge(b, depth, srcId, id)
				writeMermaid(child)
			}
			depth--
		case *SequentialJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "SEQUENTIAL")
			for _, child := range j.children {
				writeEdge(b, depth, srcId, id)
				writeMermaid(child)
			}
			depth--
		case *TimeoutJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "TIMEOUT")
			writeEdge(b, depth, srcId, id)
			writeNode(b, depth, DefaultStyle, &id, j.timeout.String())
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		case *LimitJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "LIMIT")
			writeEdge(b, depth, srcId, id)
			writeNode(b, depth, DefaultStyle, &id, strconv.Itoa(j.limit))
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		case *subRepoPermsFilterJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "FILTER")
			writeEdge(b, depth, srcId, id)
			writeNode(b, depth, DefaultStyle, &id, "SubRepoPermissions")
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		case *selectJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "SELECT")
			writeEdge(b, depth, srcId, id)
			writeNode(b, depth, DefaultStyle, &id, j.path.String())
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		case *alertJob:
			srcId := id
			depth++
			writeNode(b, depth, RoundedStyle, &id, "ALERT")
			writeEdge(b, depth, srcId, id)
			writeMermaid(j.child)
			depth--
		default:
			panic(fmt.Sprintf("unsupported job %T for PrettyMermaid printer", j))
		}
	}
	writeMermaid(j)
	return b.String()
}

// toJSON returns a JSON object representing a job. If `verbose` is true, values
// for all leaf jobs are emitted; if false, only the names of leaf nodes are
// emitted.
func toJSON(j job.Job, verbose bool) any {
	var emitJSON func(job.Job) any
	emitJSON = func(j job.Job) any {
		if j == nil {
			return struct{}{}
		}
		switch j := j.(type) {
		case
			*zoekt.ZoektRepoSubsetSearchJob,
			*zoekt.ZoektSymbolSearchJob,
			*searcher.SearcherJob,
			*searcher.SymbolSearcherJob,
			*run.RepoSearchJob,
			*zoekt.ZoektGlobalSearchJob,
			*structural.StructuralSearchJob,
			*commit.CommitSearchJob,
			*symbol.ReferenceSearcherJob,
			*symbol.RepoUniverseSymbolSearchJob,
			*repos.ComputeExcludedReposJob,
			*NoopJob:
			if verbose {
				return map[string]any{j.Name(): j}
			}
			return j.Name()

		case *repoPagerJob:
			return struct {
				Repopager any `json:"REPOPAGER"`
			}{
				Repopager: emitJSON(j.child),
			}

		case *AndJob:
			children := make([]any, 0, len(j.children))
			for _, child := range j.children {
				children = append(children, emitJSON(child))
			}
			return struct {
				And []any `json:"AND"`
			}{
				And: children,
			}

		case *OrJob:
			children := make([]any, 0, len(j.children))
			for _, child := range j.children {
				children = append(children, emitJSON(child))
			}
			return struct {
				Or []any `json:"OR"`
			}{
				Or: children,
			}

		case *ParallelJob:
			children := make([]any, 0, len(j.children))
			for _, child := range j.children {
				children = append(children, emitJSON(child))
			}
			return struct {
				Parallel any `json:"PARALLEL"`
			}{
				Parallel: children,
			}

		case *SequentialJob:
			children := make([]any, 0, len(j.children))
			for _, child := range j.children {
				children = append(children, emitJSON(child))
			}
			return struct {
				Sequential any `json:"SEQUENTIAL"`
			}{
				Sequential: children,
			}

		case *TimeoutJob:
			return struct {
				Timeout any    `json:"TIMEOUT"`
				Value   string `json:"value"`
			}{
				Timeout: emitJSON(j.child),
				Value:   j.timeout.String(),
			}

		case *LimitJob:
			return struct {
				Limit any `json:"LIMIT"`
				Value int `json:"value"`
			}{
				Limit: emitJSON(j.child),
				Value: j.limit,
			}

		case *subRepoPermsFilterJob:
			return struct {
				Filter any    `json:"FILTER"`
				Value  string `json:"value"`
			}{
				Filter: emitJSON(j.child),
				Value:  "SubRepoPermissions",
			}
		case *selectJob:
			return struct {
				Select any    `json:"SELECT"`
				Value  string `json:"value"`
			}{
				Select: emitJSON(j.child),
				Value:  j.path.String(),
			}
		case *alertJob:
			return struct {
				Alert any `json:"ALERT"`
			}{
				Alert: emitJSON(j.child),
			}
		default:
			panic(fmt.Sprintf("unsupported job %T for toJSON converter", j))
		}
	}
	return emitJSON(j)
}

// PrettyJSON returns a summary of a job in formatted JSON.
func PrettyJSON(j job.Job) string {
	result, _ := json.MarshalIndent(toJSON(j, false), "", "  ")
	return string(result)
}

// PrettyJSON returns the full fidelity of values that comprise a job in formatted JSON.
func PrettyJSONVerbose(j job.Job) string {
	result, _ := json.MarshalIndent(toJSON(j, true), "", "  ")
	return string(result)
}
