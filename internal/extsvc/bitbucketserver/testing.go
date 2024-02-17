package bitbucketserver

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnaeon/go-vcr/cassette"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewTestClient returns a bitbucketserver.Client that records its interactions
// to testdata/vcr/.
func NewTestClient(t testing.TB, name string, update bool) *Client {
	t.Helper()

	cassete := filepath.Join("testdata/vcr/", normalize(name))
	rec, err := httptestutil.NewRecorder(cassete, update)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rec.Stop(); err != nil {
			t.Errorf("failed to update test data: %s", err)
		}
	})
	rec.SetMatcher(ignoreHostMatcher)

	instanceURL := os.Getenv("BITBUCKET_SERVER_URL")
	if instanceURL == "" {
		instanceURL = "https://bitbucket.sgdev.org"
	}

	c := &schema.BitbucketServerConnection{
		Token: os.Getenv("BITBUCKET_SERVER_TOKEN"),
		Url:   instanceURL,
	}

	cli, err := NewClient("urn", c, httpcli.NewFactory(nil, httptestutil.NewRecorderOpt(rec)))
	if err != nil {
		t.Fatal(err)
	}
	cli.rateLimit = ratelimit.NewInstrumentedLimiter("bitbucket", rate.NewLimiter(100, 10))

	return cli
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
