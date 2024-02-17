package gomodproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"golang.org/x/mod/module"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// A Client to Go module proxies.
type Client struct {
	urls    []string // list of proxy URLs
	cf      *httpcli.Factory
	limiter *ratelimit.InstrumentedLimiter
}

// NewClient returns a new Client for the given urls. urn represents the
// unique urn of the external service this client's config is from.
func NewClient(urn string, urls []string, httpfactory *httpcli.Factory) *Client {
	return &Client{
		urls:    urls,
		cf:      httpfactory,
		limiter: ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("GoModClient"), urn)),
	}
}

// GetVersion gets a single version of the given module if it exists.
func (c *Client) GetVersion(ctx context.Context, mod reposource.PackageName, version string) (*module.Version, error) {
	var paths []string
	if version != "" {
		escapedVersion, err := module.EscapeVersion(version)
		if err != nil {
			return nil, errors.Wrap(err, "failed to escape version")
		}
		paths = []string{"@v", escapedVersion + ".info"}
	} else {
		paths = []string{"@latest"}
	}

	doer, err := c.cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}
	respBody, err := c.get(ctx, doer, mod, paths...)
	if err != nil {
		return nil, err
	}

	var v struct{ Version string }
	if err = json.NewDecoder(respBody).Decode(&v); err != nil {
		return nil, err
	}

	return &module.Version{Path: string(mod), Version: v.Version}, nil
}

func (c *Client) GetZip(ctx context.Context, mod reposource.PackageName, version string) (io.ReadCloser, error) {
	escapedVersion, err := module.EscapeVersion(version)
	if err != nil {
		return nil, errors.Wrap(err, "failed to escape version")
	}

	doer, err := c.cf.Doer()
	if err != nil {
		return nil, err
	}

	zip, err := c.get(ctx, doer, mod, "@v", escapedVersion+".zip")
	if err != nil {
		return nil, err
	}

	return zip, nil
}

func (c *Client) get(ctx context.Context, doer httpcli.Doer, mod reposource.PackageName, paths ...string) (respBody io.ReadCloser, err error) {
	escapedMod, err := module.EscapePath(string(mod))
	if err != nil {
		return nil, errors.Wrap(err, "failed to escape module path")
	}

	// so err isnt shadowed below
	var (
		reqURL *url.URL
		req    *http.Request
	)

	for _, baseURL := range c.urls {
		if err = c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		reqURL, err = url.Parse(baseURL)
		if err != nil {
			return nil, errors.Errorf("invalid go modules proxy URL %q", baseURL)
		}
		reqURL.Path = path.Join(escapedMod, path.Join(paths...))

		req, err = http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
		if err != nil {
			return nil, err
		}

		respBody, err = c.do(doer, req)
		if err == nil || !errcode.IsNotFound(err) {
			break
		} else if respBody != nil {
			respBody.Close()
		}
	}

	return respBody, err
}

func (c *Client) do(doer httpcli.Doer, req *http.Request) (io.ReadCloser, error) {
	resp, err := doer.Do(req)
	if err != nil {
		return nil, err
	}

	// https://go.dev/ref/mod#goproxy-protocol
	// Successful HTTP responses must have the status code 200 (OK).
	// Redirects (3xx) are followed. Responses with status codes 4xx and 5xx are treated as errors.
	// The error codes 404 (Not Found) and 410 (Gone) indicate that the requested module or version is not available
	// on the proxy, but it may be found elsewhere.
	// Error responses should have content type text/plain with charset either utf-8 or us-ascii.

	if resp.StatusCode != http.StatusOK {
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			bs = []byte(errors.Wrap(err, "failed to read body").Error())
		}
		resp.Body.Close()
		return nil, &Error{Path: req.URL.Path, Code: resp.StatusCode, Message: string(bs)}
	}

	return resp.Body, nil
}

// Error returned from an HTTP request to a Go module proxy.
type Error struct {
	Path    string
	Code    int
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("bad go module proxy response with status code %d for %s: %s", e.Code, e.Path, e.Message)
}

func (e *Error) NotFound() bool {
	return e.Code == http.StatusNotFound || e.Code == http.StatusGone
}
