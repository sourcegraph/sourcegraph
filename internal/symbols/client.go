package symbols

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/neelance/parallel"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context/ctxhttp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var symbolsURL = env.Get("SYMBOLS_URL", "k8s+http://symbols:3184", "symbols service URL")

// DefaultClient is the default Client. Unless overwritten, it is connected to the server specified by the
// SYMBOLS_URL environment variable.
var DefaultClient = &Client{
	URL: symbolsURL,
	HTTPClient: &http.Client{
		// ot.Transport will propagate opentracing spans
		Transport: &ot.Transport{
			RoundTripper: &http.Transport{
				// Default is 2, but we can send many concurrent requests
				MaxIdleConnsPerHost: 500,
			},
		},
	},
	HTTPLimiter: parallel.NewRun(500),
}

// Client is a symbols service client.
type Client struct {
	// URL to symbols service.
	URL string

	// HTTP client to use
	HTTPClient *http.Client

	// Limits concurrency of outstanding HTTP posts
	HTTPLimiter *parallel.Run

	once     sync.Once
	endpoint *endpoint.Map
}

type key struct {
	repo     api.RepoName
	commitID api.CommitID
}

func (c *Client) url(key key) (string, error) {
	c.once.Do(func() {
		if len(strings.Fields(c.URL)) == 0 {
			c.endpoint = endpoint.Empty(errors.New("a symbols service has not been configured"))
		} else {
			c.endpoint = endpoint.New(c.URL)
		}
	})
	return c.endpoint.Get(string(key.repo)+":"+string(key.commitID), nil)
}

// Search performs a symbol search on the symbols service.
func (c *Client) Search(ctx context.Context, args search.SymbolsParameters) (result *result.Symbols, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "symbols.Client.Search")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()
	span.SetTag("Repo", string(args.Repo))
	span.SetTag("CommitID", string(args.CommitID))

	resp, err := c.httpPost(ctx, "search", key{repo: args.Repo, commitID: args.CommitID}, args)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// best-effort inclusion of body in error message
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, errors.Errorf("Symbol.Search http status %d for %+v: %s", resp.StatusCode, resp.StatusCode, string(body))
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func (c *Client) httpPost(ctx context.Context, method string, key key, payload interface{}) (resp *http.Response, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "symbols.Client.httpPost")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()

	url, err := c.url(key)
	if err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	req, err := http.NewRequest("POST", url+method, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	if c.HTTPLimiter != nil {
		span.LogKV("event", "Waiting on HTTP limiter")
		c.HTTPLimiter.Acquire()
		defer c.HTTPLimiter.Release()
		span.LogKV("event", "Acquired HTTP limiter")
	}

	req, ht := nethttp.TraceRequest(span.Tracer(), req,
		nethttp.OperationName("Symbols Client"),
		nethttp.ClientTrace(false))
	defer ht.Finish()

	// Do not lose the context returned by TraceRequest
	ctx = req.Context()

	return ctxhttp.Do(ctx, c.HTTPClient, req)
}
