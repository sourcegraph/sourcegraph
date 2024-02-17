//nolint:bodyclose // Body is closed in Client.Do, but the response is still returned to provide access to the headers
package pagure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
	"github.com/sourcegraph/sourcegraph/schema"
)

// Client access a Pagure via the REST API.
type Client struct {
	// Config is the code host connection config for this client
	Config *schema.PagureConnection

	// URL is the base URL of Pagure.
	URL *url.URL

	// HTTP Client used to communicate with the API
	httpClient httpcli.Doer

	// RateLimit is the self-imposed rate limiter (since Pagure does not have a concept
	// of rate limiting in HTTP response headers).
	rateLimit *ratelimit.InstrumentedLimiter
}

// NewClient returns an authenticated Pagure API client with
// the provided configuration. If a nil httpClient is provided, an external doer
// will be used.
func NewClient(urn string, config *schema.PagureConnection, httpClient httpcli.Doer) (*Client, error) {
	u, err := url.Parse(config.Url)
	if err != nil {
		return nil, err
	}

	if httpClient == nil {
		httpClient = httpcli.ExternalDoer
	}

	return &Client{
		Config:     config,
		URL:        u,
		httpClient: httpClient,
		rateLimit:  ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("PagureClient"), urn)),
	}, nil
}

// ListProjectsArgs defines options to be set on ListProjects method calls.
type ListProjectsArgs struct {
	Cursor    *Pagination
	Tags      []string
	Pattern   string
	Namespace string
	Fork      bool
}

// listProjectsResponse defines a response struct returned from ListProjects method calls.
type listProjectsResponse struct {
	*Pagination `json:"pagination"`
	Projects    []*Project `json:"projects"`
}

func (c *Client) ListProjects(ctx context.Context, opts ListProjectsArgs) *iterator.Iterator[*Project] {
	cursor := opts.Cursor
	if cursor == nil {
		cursor = &Pagination{PerPage: 100, Page: 1}
	}

	return iterator.New(func() ([]*Project, error) {
		if cursor == nil {
			return nil, nil
		}

		qs := make(url.Values)

		cursor.EncodeTo(qs)
		for _, tag := range opts.Tags {
			if tag != "" {
				qs.Add("tags", tag)
			}
		}

		if opts.Pattern != "" {
			qs.Set("pattern", opts.Pattern)
		}

		if opts.Namespace != "" {
			qs.Set("namespace", opts.Namespace)
		}

		qs.Set("fork", strconv.FormatBool(opts.Fork))

		u := url.URL{Path: "api/0/projects", RawQuery: qs.Encode()}

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, err
		}

		var resp listProjectsResponse
		if _, err = c.do(ctx, req, &resp); err != nil {
			return nil, err
		}

		cursor = resp.Pagination
		if cursor.Next == "" {
			cursor = nil
		} else {
			cursor.Page++
		}

		return resp.Projects, nil
	})
}

func (c *Client) do(ctx context.Context, req *http.Request, result any) (*http.Response, error) {
	req.URL = c.URL.ResolveReference(req.URL)
	if req.Header.Get("Content-Type") == "" && req.Method != "GET" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if c.Config.Token != "" {
		req.Header.Add("Authorization", "token "+c.Config.Token)
	}

	if err := c.rateLimit.Wait(ctx); err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, errors.WithStack(&httpError{
			URL:        req.URL,
			StatusCode: resp.StatusCode,
			Body:       bs,
		})
	}

	return resp, json.Unmarshal(bs, result)
}

type Pagination struct {
	First   string `json:"first"`
	Last    string `json:"last"`
	Next    string `json:"next"`
	Page    int    `json:"page"`
	Pages   int    `json:"pages"`
	PerPage int    `json:"per_page"`
	Prev    string `json:"prev"`
}

func (p *Pagination) EncodeTo(qs url.Values) {
	if p == nil {
		return
	}

	qs.Set("per_page", strconv.FormatInt(int64(p.PerPage), 10))
	qs.Set("page", strconv.FormatInt(int64(p.Page), 10))
}

type Project struct {
	Description string   `json:"description"`
	FullURL     string   `json:"full_url"`
	Fullname    string   `json:"fullname"`
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Namespace   string   `json:"namespace"`
	Parent      *Project `json:"parent,omitempty"`
	Tags        []string `json:"tags"`
	URLPath     string   `json:"url_path"`
}

type httpError struct {
	StatusCode int
	URL        *url.URL
	Body       []byte
}

func (e *httpError) Error() string {
	return fmt.Sprintf("Pagure API HTTP error: code=%d url=%q body=%q", e.StatusCode, e.URL, e.Body)
}

func (e *httpError) Unauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

func (e *httpError) NotFound() bool {
	return e.StatusCode == http.StatusNotFound
}
