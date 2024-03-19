package httpcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/strings/slices"

	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestRedisLoggerMiddleware(t *testing.T) {
	rcache.SetupForTest(t)

	normalReq, _ := http.NewRequest("GET", "http://dev/null", strings.NewReader("horse"))
	complexReq, _ := http.NewRequest("PATCH", "http://test.aa?a=2", strings.NewReader("graph"))
	complexReq.Header.Set("Cache-Control", "no-cache")
	postReqEmptyBody, _ := http.NewRequest("POST", "http://dev/null", io.NopCloser(bytes.NewBuffer([]byte{})))

	testCases := []struct {
		req  *http.Request
		name string
		cli  Doer
		err  string
		want *types.OutboundRequestLogItem
	}{
		{
			req:  normalReq,
			name: "normal response",
			cli:  newFakeClientWithHeaders(map[string][]string{"X-Test-Header": {"value"}}, http.StatusOK, []byte(`{"responseBody":true}`), nil),
			err:  "<nil>",
			want: &types.OutboundRequestLogItem{
				Method:          normalReq.Method,
				URL:             normalReq.URL.String(),
				RequestHeaders:  map[string][]string{},
				RequestBody:     "horse",
				StatusCode:      http.StatusOK,
				ResponseHeaders: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}, "X-Test-Header": {"value"}},
			},
		},
		{
			req:  complexReq,
			name: "complex request",
			cli:  newFakeClientWithHeaders(map[string][]string{"X-Test-Header": {"value1", "value2"}}, http.StatusForbidden, []byte(`{"permission":false}`), nil),
			err:  "<nil>",
			want: &types.OutboundRequestLogItem{
				Method:          complexReq.Method,
				URL:             complexReq.URL.String(),
				RequestHeaders:  map[string][]string{"Cache-Control": {"no-cache"}},
				RequestBody:     "graph",
				StatusCode:      http.StatusForbidden,
				ResponseHeaders: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}, "X-Test-Header": {"value1", "value2"}},
			},
		},
		{
			req:  normalReq,
			name: "no response",
			cli: DoerFunc(func(r *http.Request) (*http.Response, error) {
				return nil, errors.New("oh no")
			}),
			err: "oh no",
		},
		{
			req:  postReqEmptyBody,
			name: "post request with empty body",
			cli:  newFakeClientWithHeaders(map[string][]string{"X-Test-Header": {"value1", "value2"}}, http.StatusOK, []byte(`{"permission":false}`), nil),
			err:  "<nil>",
			want: &types.OutboundRequestLogItem{
				Method:          postReqEmptyBody.Method,
				URL:             postReqEmptyBody.URL.String(),
				RequestHeaders:  map[string][]string{},
				RequestBody:     "",
				StatusCode:      http.StatusOK,
				ResponseHeaders: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}, "X-Test-Header": {"value1", "value2"}},
			},
		},
	}

	// Enable feature
	setOutboundRequestLogLimit(t, 1)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Build client with middleware
			cli := redisLoggerMiddleware()(tc.cli)

			// Send request
			_, err := cli.Do(tc.req)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("have error: %q\nwant error: %q", have, want)
			}

			assert.Eventually(t, func() bool {
				// Check logged request
				logged, err := GetOutboundRequestLogItems(context.Background(), "")
				if err != nil {
					t.Fatalf("couldnt get logged requests: %s", err)
				}

				return len(logged) == 1 && equal(tc.want, logged[0])
			}, 5*time.Second, 100*time.Millisecond)
		})
	}
}

func equal(a, b *types.OutboundRequestLogItem) bool {
	if a == nil || b == nil {
		return true
	}
	return cmp.Diff(a, b, cmpopts.IgnoreFields(
		types.OutboundRequestLogItem{},
		"ID",
		"StartedAt",
		"Duration",
		"CreationStackFrame",
		"CallStackFrame",
	)) == ""
}

func TestRedisLoggerMiddleware_multiple(t *testing.T) {
	// This test ensures that we correctly apply limits bigger than 1, as well
	// as ensuring GetOutboundRequestLogItem works.
	requests := 10
	limit := requests / 2

	rcache.SetupForTest(t)

	// Enable the feature
	setOutboundRequestLogLimit(t, int32(limit))

	// Build client with middleware
	cli := redisLoggerMiddleware()(newFakeClient(http.StatusOK, []byte(`{"responseBody":true}`), nil))

	// Send requests and track the URLs we send so we can compare later to
	// what was stored.
	var wantURLs []string
	for i := range requests {
		u := fmt.Sprintf("http://dev/%d", i)
		wantURLs = append(wantURLs, u)

		req, _ := http.NewRequest("GET", u, strings.NewReader("horse"))
		_, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		// Our keys are based on time, so we add a tiny sleep to ensure we
		// don't duplicate keys.
		time.Sleep(10 * time.Millisecond)
	}

	// Updated want by what is actually kept
	wantURLs = wantURLs[len(wantURLs)-limit:]

	gotURLs := func(items []*types.OutboundRequestLogItem) []string {
		var got []string
		for _, item := range items {
			got = append(got, item.URL)
		}
		return got
	}

	// Check logged request
	logged, err := GetOutboundRequestLogItems(context.Background(), "")
	if err != nil {
		t.Fatalf("couldnt get logged requests: %s", err)
	}
	if diff := cmp.Diff(wantURLs, gotURLs(logged)); diff != "" {
		t.Fatalf("unexpected logged URLs (-want, +got):\n%s", diff)
	}

	// Check that after works
	after := logged[limit/2-1].ID
	wantURLs = wantURLs[limit/2:]
	afterLogged, err := GetOutboundRequestLogItems(context.Background(), after)
	if err != nil {
		t.Fatalf("couldnt get logged requests: %s", err)
	}
	if diff := cmp.Diff(wantURLs, gotURLs(afterLogged)); diff != "" {
		t.Fatalf("unexpected logged with after URLs (-want, +got):\n%s", diff)
	}

	// Check that GetOutboundRequestLogItem works
	for _, want := range logged {
		got, err := GetOutboundRequestLogItem(want.ID)
		if err != nil {
			t.Fatalf("failed to find log item %+v", want)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected item returned via GetOutboundRequestLogItem (-want, +got):\n%s", diff)
		}
	}

	// Finally check we return an error if the item key doesn't exist.
	_, err = GetOutboundRequestLogItem("does not exist")
	if got, want := fmt.Sprintf("%s", err), "item not found"; got != want {
		t.Fatalf("unexpected error for GetOutboundRequestLogItem(\"does not exist\") got=%q want=%q", got, want)
	}
}

func TestRedisLoggerMiddleware_redactSensitiveHeaders(t *testing.T) {
	input := http.Header{
		"Authorization":   []string{"all values", "should be", "removed"},
		"Bearer":          []string{"this should be kept as the risky value is only in the name"},
		"GHP_XXXX":        []string{"this should be kept"},
		"GLPAT-XXXX":      []string{"this should also be kept"},
		"GitHub-PAT":      []string{"this should be removed: ghp_XXXX"},
		"GitLab-PAT":      []string{"this should be removed", "glpat-XXXX"},
		"Innocent-Header": []string{"this should be removed as it includes", "the word bearer"},
		"Set-Cookie":      []string{"this is verboten"},
		"Token":           []string{"a token should be removed"},
		"X-Powered-By":    []string{"PHP"},
		"X-Token":         []string{"something that smells like a token should also be removed"},
	}

	// Build the expected output.
	want := make(http.Header)
	riskyKeys := []string{"Bearer", "GHP_XXXX", "GLPAT-XXXX", "X-Powered-By"}
	for key, value := range input {
		if slices.Contains(riskyKeys, key) {
			want[key] = value
		} else {
			want[key] = []string{"REDACTED"}
		}
	}

	cleanHeaders := redactSensitiveHeaders(input)

	if diff := cmp.Diff(cleanHeaders, want); diff != "" {
		t.Errorf("unexpected request headers (-have +want):\n%s", diff)
	}
}

func TestRedisLoggerMiddleware_formatStackFrame(t *testing.T) {
	tests := []struct {
		name     string
		function string
		file     string
		line     int
		want     string
	}{
		{
			name:     "Sourcegraph internal package",
			function: "github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend.(*requestTracer).TraceQuery",
			file:     "/Users/x/github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlbackend.go",
			line:     51,
			want:     "cmd/frontend/graphqlbackend/graphqlbackend.go:51 (Function: (*requestTracer).TraceQuery)",
		},
		{
			name:     "third-party package",
			function: "third-party/library.f",
			file:     "/Users/x/github.com/third-party/library/file.go",
			line:     11,
			want:     "third-party/library/file.go:11 (Function: f)",
		},
		{
			name:     "main package",
			function: "main.f",
			file:     "/Users/x/file.go",
			line:     11,
			want:     "file.go:11 (Function: f)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := formatStackFrame(test.function, test.file, test.line)
			if got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func setOutboundRequestLogLimit(t *testing.T, limit int32) {
	old := OutboundRequestLogLimit()
	SetOutboundRequestLogLimit(limit)
	t.Cleanup(func() { SetOutboundRequestLogLimit(old) })
}
