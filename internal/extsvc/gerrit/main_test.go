package gerrit

import (
	"flag"
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
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

var update = flag.Bool("update", false, "update testdata")

// NewTestClient returns a gerrit.Client that records its interactions
// to testdata/vcr/.
func NewTestClient(t testing.TB, name string, update bool) (Client, func()) {
	t.Helper()

	cassete := filepath.Join("testdata/vcr/", normalize(name))
	rec, err := httptestutil.NewRecorder(cassete, update)
	if err != nil {
		t.Fatal(err)
	}
	rec.SetMatcher(ignoreHostMatcher)

	u, err := url.Parse("https://gerrit.sgdev.org")
	if err != nil {
		t.Fatal(err)
	}

	cli, err := NewClient("urn", u, &AccountCredentials{
		Username: os.Getenv("GERRIT_USERNAME"),
		Password: os.Getenv("GERRIT_PASSWORD"),
	}, httpcli.NewFactory(nil, httptestutil.NewRecorderOpt(rec)))
	if err != nil {
		t.Fatal(err)
	}

	cli.(*client).rateLimit = ratelimit.NewInstrumentedLimiter("gerrit", rate.NewLimiter(100, 10))

	return cli, func() {
		if err := rec.Stop(); err != nil {
			t.Errorf("failed to update test data: %s", err)
		}
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
