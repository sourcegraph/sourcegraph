package gosyntect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	client *Client
	once   sync.Once
)

func init() {
	syntectServer := env.Get("SRC_SYNTECT_SERVER", "http://syntect-server:9238", "syntect_server HTTP(s) address")
	once.Do(func() {
		client = New(syntectServer)
	})
}

func GetSyntectClient() *Client {
	return client
}

// Keep in sync with 'enum SyntaxEngine' in Rust code
const (
	SyntaxEngineSyntect    = "syntect"
	SyntaxEngineTreesitter = "tree-sitter"
	SyntaxEngineScipSyntax = "scip-syntax"

	SyntaxEngineInvalid = "invalid"
)

func isTreesitterBased(engine string) bool {
	switch engine {
	case SyntaxEngineTreesitter, SyntaxEngineScipSyntax:
		return true
	default:
		return false
	}
}

type HighlightResponseType string

// The different response formats supported by the syntax highlighter.
const (
	FormatHTMLPlaintext HighlightResponseType = "HTML_PLAINTEXT"
	FormatHTMLHighlight HighlightResponseType = "HTML_HIGHLIGHT"
	FormatJSONSCIP      HighlightResponseType = "JSON_SCIP"
)

// Returns corresponding format type for the request format. Defaults to
// FormatHTMLHighlight
func GetResponseFormat(format string) HighlightResponseType {
	if format == string(FormatHTMLPlaintext) {
		return FormatHTMLPlaintext
	}
	if format == string(FormatJSONSCIP) {
		return FormatJSONSCIP
	}
	return FormatHTMLHighlight
}

// Query represents a code highlighting query to the syntect_server.
type Query struct {
	// Filepath is the file path of the code. It can be the full file path, or
	// just the name and extension.
	//
	// See: https://github.com/sourcegraph/syntect_server#supported-file-extensions
	Filepath string `json:"filepath"`

	// Filetype is the language name.
	Filetype string `json:"filetype"`

	// Code is the literal code to highlight.
	Code string `json:"code"`

	// LineLengthLimit is the maximum length of line that will be highlighted if set.
	// Defaults to no max if zero.
	// If CSS is false, LineLengthLimit is ignored.
	LineLengthLimit int `json:"line_length_limit,omitempty"`

	// StabilizeTimeout, if non-zero, overrides the default syntect_server
	// http-server-stabilizer timeout of 10s. This is most useful when a user
	// is requesting to highlight a very large file and is willing to wait
	// longer, but it is important this not _always_ be a long duration because
	// the worker's threads could get stuck at 100% CPU for this amount of
	// time if the user's request ends up being a problematic one.
	StabilizeTimeout time.Duration `json:"-"`

	// Which highlighting engine to use
	Engine string `json:"engine"`
}

// Response represents a response to a code highlighting query.
type Response struct {
	// Data is the actual highlighted HTML version of Query.Code.
	Data string

	// Plaintext indicates whether or not a syntax could not be found for the
	// file and instead it was rendered as plain text.
	Plaintext bool
}

var (
	// ErrRequestTooLarge is returned when the request is too large for syntect_server to handle (e.g. file is too large to highlight).
	ErrRequestTooLarge = errors.New("request too large")

	// ErrPanic occurs when syntect_server panics while highlighting code. This
	// most often occurs when Syntect does not support e.g. an obscure or
	// relatively unused sublime-syntax feature and as a result panics.
	ErrPanic = errors.New("syntect panic while highlighting")

	// ErrHSSWorkerTimeout occurs when syntect_server's wrapper,
	// http-server-stabilizer notices syntect_server is taking too long to
	// serve a request, has most likely gotten stuck, and as such has been
	// restarted. This occurs rarely on certain files syntect_server cannot yet
	// handle for some reason.
	ErrHSSWorkerTimeout = errors.New("HSS worker timeout while serving request")
)

type response struct {
	// Successful response fields.
	Data string `json:"data"`
	// Used by the /scip endpoint
	Scip      string `json:"scip"`
	Plaintext bool   `json:"plaintext"`

	// Error response fields.
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Client represents a client connection to a syntect_server.
type Client struct {
	syntectServer string
	cf            *httpcli.Factory
}

func IsTreesitterSupported(filetype string) bool {
	_, contained := treesitterSupportedFiletypes[languages.NormalizeLanguage(filetype)]
	return contained
}

// Highlight performs a query to highlight some code.
func (c *Client) Highlight(ctx context.Context, q *Query, format HighlightResponseType) (_ *Response, err error) {
	// Normalize filetype
	q.Filetype = languages.NormalizeLanguage(q.Filetype)

	tr, ctx := trace.New(ctx, "gosyntect.Highlight",
		attribute.String("filepath", q.Filepath))
	defer tr.EndWithErr(&err)

	if isTreesitterBased(q.Engine) && !IsTreesitterSupported(q.Filetype) {
		return nil, errors.New("Not a valid treesitter filetype")
	}

	// Build the request.
	jsonQuery, err := json.Marshal(q)
	if err != nil {
		return nil, errors.Wrap(err, "encoding query")
	}

	var url string
	if format == FormatJSONSCIP {
		url = "/scip"
	} else if isTreesitterBased(q.Engine) {
		// "Legacy SCIP mode" for the HTML blob view and languages configured to
		// be processed with tree sitter.
		url = "/lsif"
	} else {
		url = "/"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url(url), bytes.NewReader(jsonQuery))
	if err != nil {
		return nil, errors.Wrap(err, "building request")
	}
	req.Header.Set("Content-Type", "application/json")
	if q.StabilizeTimeout != 0 {
		req.Header.Set("X-Stabilize-Timeout", q.StabilizeTimeout.String())
	}

	cli, err := c.cf.Doer()
	if err != nil {
		return nil, err
	}

	// Perform the request.
	resp, err := cli.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("making request to %s", c.url("/")))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, ErrRequestTooLarge
	}

	// Decode the response.
	var r response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("decoding JSON response from %s", c.url("/")))
	}
	if r.Error != "" {
		var err error
		switch r.Code {
		case "resource_not_found":
			// resource_not_found is returned in the event of a 404, indicating a bug
			// in gosyntect.
			err = errors.New("gosyntect internal error: resource_not_found")
		case "panic":
			err = ErrPanic
		case "hss_worker_timeout":
			err = ErrHSSWorkerTimeout
		default:
			err = errors.Errorf("unknown error=%q code=%q", r.Error, r.Code)
		}
		return nil, errors.Wrap(err, c.syntectServer)
	}
	response := &Response{
		Data:      r.Data,
		Plaintext: r.Plaintext,
	}

	// If SCIP is set, prefer it over HTML
	if r.Scip != "" {
		response.Data = r.Scip
	}

	return response, nil
}

func (c *Client) url(path string) string {
	return c.syntectServer + path
}

// New returns a client connection to a syntect_server.
func New(syntectServer string) *Client {
	return &Client{
		syntectServer: strings.TrimSuffix(syntectServer, "/"),
		cf:            httpcli.NewInternalClientFactory("syntect"),
	}
}
