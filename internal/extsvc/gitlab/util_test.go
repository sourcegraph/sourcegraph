package gitlab

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

type mockHTTPResponseBody struct {
	count        int
	header       http.Header
	responseBody string
	statusCode   int
}

func (s *mockHTTPResponseBody) Do(req *http.Request) (*http.Response, error) {
	s.count++
	statusCode := http.StatusOK
	if s.statusCode != 0 {
		statusCode = s.statusCode
	}
	return &http.Response{
		Request:    req,
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(s.responseBody)),
		Header:     s.header,
	}, nil
}

type mockHTTPEmptyResponse struct {
	statusCode int
}

func (s mockHTTPEmptyResponse) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Request:    req,
		StatusCode: s.statusCode,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}, nil
}

func newTestClient(t *testing.T) *Client {
	rcache.SetupForTest(t)
	return &Client{
		baseURL:             &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
		httpClient:          &http.Client{},
		externalRateLimiter: &ratelimit.Monitor{},
		projCache:           rcache.NewWithTTL("__test__gl_proj", 1000, false),
	}
}
