package bitbucketcloud

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/regexp"
	"golang.org/x/time/rate"

	bbtest "github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud/testing"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/schema"
)

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

// assertGolden wraps testutil.AssertGolden to ensure that golden fixtures are
// read and written to a consistent location.
//
// Note that assertGolden can only be called once in a single test. (It's safe
// to use from multiple sub-tests at the same level, though, provided they have
// unique names.)
func assertGolden(t testing.TB, expected any) {
	t.Helper()
	testutil.AssertGolden(
		t,
		filepath.Join("testdata/golden/", normalize(t.Name())),
		update(t.Name()),
		expected,
	)
}

// newTestClient returns a bitbucketcloud.Client that records its interactions
// to testdata/vcr/.
func newTestClient(t testing.TB) *client {
	t.Helper()

	cassette := filepath.Join("testdata/vcr/", normalize(t.Name()))
	rec, err := httptestutil.NewRecorder(cassette, update(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rec.Stop(); err != nil {
			t.Errorf("failed to update test data: %s", err)
		}
	})

	cli, err := newClient("urn", &schema.BitbucketCloudConnection{
		ApiURL:      "https://api.bitbucket.org",
		Username:    bbtest.GetenvTestBitbucketCloudUsername(),
		AppPassword: os.Getenv("BITBUCKET_CLOUD_APP_PASSWORD"),
	}, httpcli.NewFactory(nil, httptestutil.NewRecorderOpt(rec)))
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
