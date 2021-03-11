package highlight

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/inconshreveable/log15"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/gosyntect"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var (
	syntectServer = env.Get("SRC_SYNTECT_SERVER", "http://syntect-server:9238", "syntect_server HTTP(s) address")
	client        *gosyntect.Client
)

func init() {
	client = gosyntect.New(syntectServer)
}

// IsBinary is a helper to tell if the content of a file is binary or not.
func IsBinary(content []byte) bool {
	// We first check if the file is valid UTF8, since we always consider that
	// to be non-binary.
	//
	// Secondly, if the file is not valid UTF8, we check if the detected HTTP
	// content type is text, which covers a whole slew of other non-UTF8 text
	// encodings for us.
	return !utf8.Valid(content) && !strings.HasPrefix(http.DetectContentType(content), "text/")
}

// Params defines mandatory and optional parameters to use when highlighting
// code.
type Params struct {
	// Content is the file content.
	Content []byte

	// Filepath is used to detect the language, it must contain at least the
	// file name + extension.
	Filepath string

	// DisableTimeout indicates whether or not a user has requested to wait as
	// long as needed to get highlighted results (this should never be on by
	// default, as some files can take a very long time to highlight).
	DisableTimeout bool

	// Whether or not the light theme should be used to highlight the code.
	IsLightTheme bool

	// HighlightLongLines, if true, highlighting lines which are greater than
	// 2000 bytes is enabled. This may produce a significant amount of HTML
	// which some browsers (such as Chrome, but not Firefox) may have trouble
	// rendering efficiently.
	HighlightLongLines bool

	// Whether or not to simulate the syntax highlighter taking too long to
	// respond.
	SimulateTimeout bool

	// Metadata provides optional metadata about the code we're highlighting.
	Metadata Metadata
}

// Metadata contains metadata about a request to highlight code. It is used to
// ensure that when syntax highlighting takes a long time or errors out, we
// can log enough information to track down what the problematic code we were
// trying to highlight was.
//
// All fields are optional.
type Metadata struct {
	RepoName string
	Revision string
}

// ErrBinary is returned when a binary file was attempted to be highlighted.
var ErrBinary = errors.New("cannot render binary file")

// Code highlights the given file content with the given filepath (must contain
// at least the file name + extension) and returns the properly escaped HTML
// table representing the highlighted code.
//
// The returned boolean represents whether or not highlighting was aborted due
// to timeout. In this scenario, a plain text table is returned.
//
// In the event the input content is binary, ErrBinary is returned.
func Code(ctx context.Context, p Params) (h template.HTML, aborted bool, err error) {
	if Mocks.Code != nil {
		return Mocks.Code(p)
	}
	var prometheusStatus string
	requestTime := prometheus.NewTimer(metricRequestHistogram)
	tr, ctx := trace.New(ctx, "highlight.Code", "")
	defer func() {
		if prometheusStatus != "" {
			requestCounter.WithLabelValues(prometheusStatus).Inc()
		} else if err != nil {
			requestCounter.WithLabelValues("error").Inc()
		} else {
			requestCounter.WithLabelValues("success").Inc()
		}
		tr.SetError(err)
		tr.Finish()
		requestTime.ObserveDuration()
	}()

	if !p.DisableTimeout {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}
	if p.SimulateTimeout {
		time.Sleep(4 * time.Second)
	}

	// Never pass binary files to the syntax highlighter.
	if IsBinary(p.Content) {
		return "", false, ErrBinary
	}
	code := string(p.Content)

	themechoice := "Sourcegraph"
	if p.IsLightTheme {
		themechoice = "Sourcegraph (light)"
	}

	// Trim a single newline from the end of the file. This means that a file
	// "a\n\n\n\n" will show line numbers 1-4 rather than 1-5, i.e. no blank
	// line will be shown at the end of the file corresponding to the last
	// newline.
	//
	// This matches other online code reading tools such as e.g. GitHub; see
	// https://github.com/sourcegraph/sourcegraph/issues/8024 for more
	// background.
	code = strings.TrimSuffix(code, "\n")

	// Tracing so we can identify problematic syntax highlighting requests.
	tr.LogFields(
		otlog.String("filepath", p.Filepath),
		otlog.String("repo_name", p.Metadata.RepoName),
		otlog.String("revision", p.Metadata.Revision),
		otlog.String("snippet", fmt.Sprintf("%q…", firstCharacters(code, 10))),
	)

	var stabilizeTimeout time.Duration
	if p.DisableTimeout {
		// The user wants to wait longer for results, so the default 10s worker
		// timeout is too aggressive. We will let it try to highlight the file
		// for 30s and will then terminate the process. Note this means in the
		// worst case one of syntect_server's threads could be stuck at 100%
		// CPU for 30s.
		stabilizeTimeout = 30 * time.Second
	}

	p.Filepath = normalizeFilepath(p.Filepath)

	resp, err := client.Highlight(ctx, &gosyntect.Query{
		Code:             code,
		Filepath:         p.Filepath,
		Theme:            themechoice,
		StabilizeTimeout: stabilizeTimeout,
		Tracer:           ot.GetTracer(ctx),
	})

	if ctx.Err() == context.DeadlineExceeded {
		log15.Warn(
			"syntax highlighting took longer than 3s, this *could* indicate a bug in Sourcegraph",
			"filepath", p.Filepath,
			"repo_name", p.Metadata.RepoName,
			"revision", p.Metadata.Revision,
			"snippet", fmt.Sprintf("%q…", firstCharacters(code, 80)),
		)
		tr.LogFields(otlog.Bool("timeout", true))
		prometheusStatus = "timeout"

		// Timeout, so render plain table.
		table, err2 := generatePlainTable(code)
		return table, true, err2
	} else if err != nil {
		log15.Error(
			"syntax highlighting failed (this is a bug, please report it)",
			"filepath", p.Filepath,
			"repo_name", p.Metadata.RepoName,
			"revision", p.Metadata.Revision,
			"snippet", fmt.Sprintf("%q…", firstCharacters(code, 80)),
			"error", err,
		)
		var problem string
		switch errors.Cause(err) {
		case gosyntect.ErrRequestTooLarge:
			problem = "request_too_large"
		case gosyntect.ErrPanic:
			problem = "panic"
		case gosyntect.ErrHSSWorkerTimeout:
			problem = "hss_worker_timeout"
		}
		if problem != "" {
			// A problem that can sometimes be expected has occurred. We will
			// identify such problems through metrics/logs and resolve them on
			// a case-by-case basis, but they are frequent enough that we want
			// to fallback to plaintext rendering instead of just giving the
			// user an error.
			tr.LogFields(otlog.Bool(problem, true))
			prometheusStatus = problem
			table, err2 := generatePlainTable(code)
			return table, false, err2
		}
		return "", false, err
	}
	// Note: resp.Data is properly HTML escaped by syntect_server
	table, err := preSpansToTable(resp.Data)
	if err != nil {
		return "", false, err
	}
	if !p.HighlightLongLines {
		// This number was arbitrarily chosen. We don't want long lines in general to be unhighlighted,
		// but if there are super long lines OR many lines of near this length we don't want it to slow
		// down the browser's rendering.
		maxLineLength := 2000
		table, err = unhighlightLongLines(table, maxLineLength)
		if err != nil {
			return "", false, err
		}
	}
	return template.HTML(table), false, nil
}

// TODO (Dax): Determine if Histogram provides value and either use only histogram or counter, not both
var requestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "src_syntax_highlighting_requests",
	Help: "Counts syntax highlighting requests and their success vs. failure rate.",
}, []string{"status"})

var metricRequestHistogram = promauto.NewHistogram(
	prometheus.HistogramOpts{
		Name: "src_syntax_highlighting_duration_seconds",
		Help: "time for a request to have syntax highlight",
	})

func firstCharacters(s string, n int) string {
	v := []rune(s)
	if len(v) < n {
		return string(v)
	}
	return string(v[:n])
}

// preSpansToTable takes the syntect data structure, which looks like:
//
// 	<pre>
// 	<span style="color:#foobar">thecode.line1</span>
// 	<span style="color:#foobar">thecode.line2</span>
// 	</pre>
//
// And turns it into a table in the format which the frontend expects:
//
// 	<table>
// 	<tr>
// 		<td class="line" data-line="1"></td>
// 		<td class="code"><span style="color:#foobar">thecode.line1</span></td>
// 	</tr>
// 	<tr>
// 		<td class="line" data-line="2"></td>
// 		<td class="code"><span style="color:#foobar">thecode.line2</span></td>
// 	</tr>
// 	</table>
//
func preSpansToTable(h string) (string, error) {
	doc, err := html.Parse(strings.NewReader(h))
	if err != nil {
		return "", err
	}

	body := doc.FirstChild.LastChild // html->body
	pre := body.FirstChild
	if pre == nil || pre.Type != html.ElementNode || pre.DataAtom != atom.Pre {
		return "", fmt.Errorf("expected html->body->pre, found %+v", pre)
	}

	// We will walk over all of the <span> elements and add them to an existing
	// code cell td, creating a new code cell td each time a newline is
	// encountered.
	var (
		table    = &html.Node{Type: html.ElementNode, DataAtom: atom.Table, Data: atom.Table.String()}
		next     = pre.FirstChild // span or TextNode
		rows     int
		codeCell *html.Node
	)
	newRow := func() {
		// If the previous row did not have any children, then it was a blank
		// line. Blank lines always need a span with a newline character for
		// proper whitespace copy+paste support.
		if codeCell != nil && codeCell.FirstChild == nil {
			span := &html.Node{Type: html.ElementNode, DataAtom: atom.Span, Data: atom.Span.String()}
			codeCell.AppendChild(span)
			spanText := &html.Node{Type: html.TextNode, Data: "\n"}
			span.AppendChild(spanText)
		}

		rows++
		tr := &html.Node{Type: html.ElementNode, DataAtom: atom.Tr, Data: atom.Tr.String()}
		table.AppendChild(tr)

		tdLineNumber := &html.Node{Type: html.ElementNode, DataAtom: atom.Td, Data: atom.Td.String()}
		tdLineNumber.Attr = append(tdLineNumber.Attr, html.Attribute{Key: "class", Val: "line"})
		tdLineNumber.Attr = append(tdLineNumber.Attr, html.Attribute{Key: "data-line", Val: fmt.Sprint(rows)})
		tr.AppendChild(tdLineNumber)
		codeTd := &html.Node{Type: html.ElementNode, DataAtom: atom.Td, Data: atom.Td.String()}
		tr.AppendChild(codeTd)
		codeCell = &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: atom.Div.String()}
		codeTd.AppendChild(codeCell)
		codeTd.Attr = append(codeCell.Attr, html.Attribute{Key: "class", Val: "code"})
	}
	addNewRows := func(textNode *html.Node) {
		// Text node, create a new table row for each newline at the end.
		nodeData := textNode.Data
		// Trim the preceding newlines and check if the entire node was *not* made up of newlines.
		// This prevents us from counting the preceding newlines and appending them as rows at the end.
		trimmedNodeData := strings.TrimLeft(nodeData, "\n")
		if len(trimmedNodeData) > 0 {
			nodeData = trimmedNodeData
		}
		newlines := strings.Count(nodeData, "\n")
		for i := 0; i < newlines; i++ {
			newRow()
		}
	}
	newRow()
	for next != nil {
		nextSibling := next.NextSibling
		switch {
		case next.Type == html.ElementNode && next.DataAtom == atom.Span:
			// Found a span, so add it to our current code cell td.
			next.Parent = nil
			next.PrevSibling = nil
			next.NextSibling = nil
			codeCell.AppendChild(next)

			// Scan the children for text nodes containing new lines so that we
			// can create new table rows.
			if next.FirstChild != nil {
				nextChild := next.FirstChild
				for nextChild != nil {
					switch {
					case nextChild.Type == html.TextNode:
						addNewRows(nextChild)
					default:
						return "", fmt.Errorf("unexpected HTML child structure (encountered %+v)", nextChild)
					}
					nextChild = nextChild.NextSibling
				}
			}
		case next.Type == html.TextNode:
			addNewRows(next)
		default:
			return "", fmt.Errorf("unexpected HTML structure (encountered %+v)", next)
		}
		next = nextSibling
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, table); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func generatePlainTable(code string) (template.HTML, error) {
	table := &html.Node{Type: html.ElementNode, DataAtom: atom.Table, Data: atom.Table.String()}
	for row, line := range strings.Split(code, "\n") {
		line = strings.TrimSuffix(line, "\r") // CRLF files
		if line == "" {
			line = "\n" // important for e.g. selecting whitespace in the produced table
		}
		tr := &html.Node{Type: html.ElementNode, DataAtom: atom.Tr, Data: atom.Tr.String()}
		table.AppendChild(tr)

		tdLineNumber := &html.Node{Type: html.ElementNode, DataAtom: atom.Td, Data: atom.Td.String()}
		tdLineNumber.Attr = append(tdLineNumber.Attr, html.Attribute{Key: "class", Val: "line"})
		tdLineNumber.Attr = append(tdLineNumber.Attr, html.Attribute{Key: "data-line", Val: fmt.Sprint(row + 1)})
		tr.AppendChild(tdLineNumber)

		codeCell := &html.Node{Type: html.ElementNode, DataAtom: atom.Td, Data: atom.Td.String()}
		codeCell.Attr = append(codeCell.Attr, html.Attribute{Key: "class", Val: "code"})
		tr.AppendChild(codeCell)

		// Span to match same structure as what highlighting would usually generate.
		span := &html.Node{Type: html.ElementNode, DataAtom: atom.Span, Data: atom.Span.String()}
		codeCell.AppendChild(span)
		spanText := &html.Node{Type: html.TextNode, Data: line}
		span.AppendChild(spanText)
	}

	var buf bytes.Buffer
	if err := html.Render(&buf, table); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// unhighlightLongLines takes highlighted HTML and unhighlights lines which are
// longer than N bytes in (plaintext) length, making them easier for some
// browsers such as Chrome to render.
//
// And the returned HTML has all <span> tags removed from lines whose length
// are > N bytes.
//
// See https://github.com/sourcegraph/sourcegraph/issues/6489
func unhighlightLongLines(h string, n int) (string, error) {
	doc, err := html.Parse(strings.NewReader(h))
	if err != nil {
		return "", err
	}

	table := doc.FirstChild.LastChild.FirstChild // html > body > table
	if table == nil || table.Type != html.ElementNode || table.DataAtom != atom.Table {
		return "", fmt.Errorf("expected html->body->table, found %+v", table)
	}

	// Iterate over each table row and check length
	var buf bytes.Buffer
	tr := table.FirstChild.FirstChild // table > tbody > tr
	for tr != nil {
		div := tr.LastChild.FirstChild // tr > td > div
		span := div.FirstChild         // div > span
		for span != nil {
			node := span.FirstChild
			for node != nil {
				buf.WriteString(node.Data)
				node = node.NextSibling
			}
			span = span.NextSibling
		}

		// Length exceeds the limit, replace existing child with plain text
		if buf.Len() > n {
			span := &html.Node{
				Type:     html.ElementNode,
				DataAtom: atom.Span,
				Data:     atom.Span.String(),
			}
			span.AppendChild(&html.Node{
				Type: html.TextNode,
				Data: buf.String(),
			})
			div.FirstChild = span
		}

		buf.Reset()
		tr = tr.NextSibling
	}

	buf.Reset()
	// NOTE: The result of html.Parse has parent nodes like "<html><head><body><table>..."
	// to be a valid HTML, but what we want to return is just the <table> section.
	if err := html.Render(&buf, table); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// CodeAsLines highlights the file and returns a list of highlighted lines.
// The returned boolean represents whether or not highlighting was aborted due
// to timeout.
//
// In the event the input content is binary, ErrBinary is returned.
func CodeAsLines(ctx context.Context, p Params) ([]template.HTML, bool, error) {
	html, aborted, err := Code(ctx, p)
	if err != nil {
		return nil, aborted, err
	}
	lines, err := splitHighlightedLines(html, false)
	return lines, aborted, err
}

// splitHighlightedLines takes the highlighted HTML table and returns a slice
// of highlighted strings, where each string corresponds a single line in the
// original, highlighted file.
func splitHighlightedLines(input template.HTML, wholeRow bool) ([]template.HTML, error) {
	doc, err := html.Parse(strings.NewReader(string(input)))
	if err != nil {
		return nil, err
	}

	lines := make([]template.HTML, 0)

	table := doc.FirstChild.LastChild.FirstChild // html > body > table
	if table == nil || table.Type != html.ElementNode || table.DataAtom != atom.Table {
		return nil, fmt.Errorf("expected html->body->table, found %+v", table)
	}

	// Iterate over each table row and extract content
	var buf bytes.Buffer
	tr := table.FirstChild.FirstChild // table > tbody > tr
	for tr != nil {
		var render *html.Node
		if wholeRow {
			render = tr
		} else {
			render = tr.LastChild.FirstChild // tr > td > div
		}
		err = html.Render(&buf, render)
		if err != nil {
			return nil, err
		}
		lines = append(lines, template.HTML(buf.String()))
		buf.Reset()
		tr = tr.NextSibling
	}

	return lines, nil
}

// normalizeFilepath ensures that the filepath p has a lowercase extension, i.e. it applies the
// following transformations:
//
// 	a/b/c/FOO.TXT → a/b/c/FOO.txt
// 	FOO.Sh → FOO.sh
//
// The following are left unmodified, as they already have lowercase extensions:
//
// 	a/b/c/FOO.txt
// 	a/b/c/Makefile
// 	Makefile.am
// 	FOO.txt
//
// It expects the filepath uses forward slashes always.
func normalizeFilepath(p string) string {
	ext := path.Ext(p)
	ext = strings.ToLower(ext)
	return p[:len(p)-len(ext)] + ext
}

// LineRange describes a line range.
//
// It uses int32 for GraphQL compatability.
type LineRange struct {
	// StartLine is the 0-based inclusive start line of the range.
	StartLine int32

	// EndLine is the 0-based exclusive end line of the range.
	EndLine int32
}

// SplitLineRanges takes a syntax highlighted HTML table (returned by highlight.Code) and splits out
// the specified line ranges, returning HTML table rows `<tr>...</tr>` for each line range.
//
// Input line ranges will automatically be clamped within the bounds of the file.
func SplitLineRanges(html template.HTML, ranges []LineRange) ([][]string, error) {
	lines, err := splitHighlightedLines(html, true)
	if err != nil {
		return nil, err
	}
	var lineRanges [][]string
	for _, r := range ranges {
		if r.StartLine < 0 {
			r.StartLine = 0
		}
		if r.EndLine > int32(len(lines)) {
			r.EndLine = int32(len(lines))
		}
		if r.StartLine > r.EndLine {
			r.StartLine = 0
			r.EndLine = 0
		}
		tableRows := make([]string, 0, r.EndLine-r.StartLine)
		for _, line := range lines[r.StartLine:r.EndLine] {
			tableRows = append(tableRows, string(line))
		}
		lineRanges = append(lineRanges, tableRows)
	}
	return lineRanges, nil
}
