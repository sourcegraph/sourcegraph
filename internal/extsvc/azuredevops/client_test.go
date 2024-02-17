package azuredevops

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

var update = flag.Bool("update", false, "update testdata")

// NewTestClient returns an azuredevops.Client that records its interactions
// to testdata/vcr/.
func NewTestClient(t testing.TB, name string, update bool) (Client, func()) {
	t.Helper()

	cassete := filepath.Join("testdata/vcr/", normalize(name))
	rec, err := httptestutil.NewRecorder(cassete, update)
	if err != nil {
		t.Fatal(err)
	}
	rec.SetMatcher(ignoreHostMatcher)

	cli, err := NewClient(
		"urn",
		AzureDevOpsAPIURL,
		&auth.BasicAuth{
			Username: os.Getenv("AZURE_DEV_OPS_USERNAME"),
			Password: os.Getenv("AZURE_DEV_OPS_TOKEN"),
		},
		httpcli.NewFactory(nil, httptestutil.NewRecorderOpt(rec)),
	)
	if err != nil {
		t.Fatal(err)
	}

	cli.(*client).internalRateLimiter = ratelimit.NewInstrumentedLimiter("azuredevops", rate.NewLimiter(100, 10))

	return cli, func() {
		if err := rec.Stop(); err != nil {
			t.Errorf("failed to update test data: %s", err)
		}
	}
}

func TestRateLimitRetry(t *testing.T) {
	rcache.SetupForTest(t)
	ctx := context.Background()

	tests := map[string]struct {
		useRateLimit     bool
		useRetryAfter    bool
		succeeded        bool
		waitForRateLimit bool
		wantNumRequests  int
	}{
		"retry-after hit": {
			useRetryAfter:    true,
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  2,
		},
		"rate limit hit": {
			useRateLimit:     true,
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  2,
		},
		"no rate limit hit": {
			succeeded:        true,
			waitForRateLimit: true,
			wantNumRequests:  1,
		},
		"error if rate limit hit but no waitForRateLimit": {
			useRateLimit:    true,
			wantNumRequests: 1,
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			numRequests := 0
			succeeded := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				numRequests++
				if tt.useRetryAfter {
					w.Header().Add("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("Try again later"))

					tt.useRetryAfter = false
					return
				}

				if tt.useRateLimit {
					w.Header().Add("X-RateLimit-Remaining", "0")
					w.Header().Add("X-RateLimit-Limit", "60")
					resetTime := time.Now().Add(time.Second)
					w.Header().Add("X-RateLimit-Reset", strconv.Itoa(int(resetTime.Unix())))
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("Try again later"))

					tt.useRateLimit = false
					return
				}

				succeeded = true
				w.Write([]byte(`{"some": "response"}`))
			}))

			t.Cleanup(srv.Close)

			MockVisualStudioAppURL = srv.URL
			t.Cleanup(func() {
				MockVisualStudioAppURL = ""
			})
			a := &auth.BasicAuth{Username: "test", Password: "test"}
			c, err := NewClient("test", srv.URL, a, httpcli.NewFactory(nil))
			c.(*client).internalRateLimiter = ratelimit.NewInstrumentedLimiter("azuredevops", rate.NewLimiter(100, 10))
			require.NoError(t, err)
			c.SetWaitForRateLimit(tt.waitForRateLimit)

			// We don't care about the result or if it errors, we monitor the server variables
			_, _ = c.GetAuthorizedProfile(ctx)

			assert.Equal(t, tt.succeeded, succeeded)
			assert.Equal(t, tt.wantNumRequests, numRequests)
		})
	}
}

var normalizer = lazyregexp.New("[^A-Za-z0-9-]+")

func normalize(path string) string {
	return normalizer.ReplaceAllLiteralString(path, "-")
}

func ignoreHostMatcher(r *http.Request, i cassette.Request) bool {
	if r.Method != i.Method {
		return false
	}
	u, err := url.Parse(i.URL)
	if err != nil {
		return false
	}
	u.Host = r.URL.Host
	u.Scheme = r.URL.Scheme
	return r.URL.String() == u.String()
}
