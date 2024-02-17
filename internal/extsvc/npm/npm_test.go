package npm

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log15.Root().SetHandler(log15.DiscardHandler())
	}
	os.Exit(m.Run())
}

var updateRecordings = flag.Bool("update", false, "make npm API calls, record and save data")

func newTestHTTPClient(t *testing.T) (client *HTTPClient, stop func()) {
	t.Helper()
	recorderFactory, stop := httptestutil.NewRecorderFactory(t, *updateRecordings, t.Name())

	client = NewHTTPClient("urn", "https://registry.npmjs.org", "", recorderFactory)
	client.limiter = ratelimit.NewInstrumentedLimiter("npm", rate.NewLimiter(100, 10))
	return client, stop
}

func mockNpmServer(credentials string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if key, ok := req.Header["Authorization"]; ok && key[0] != fmt.Sprintf("Bearer %s", credentials) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "incorrect credentials"}`))
			return
		}
		routes := map[string]struct {
			status int
			body   string
		}{
			"/left-pad/1.3.1": {
				status: http.StatusNotFound,
				body:   `"version not found: 1.3.1"`,
			},
			"/left-pad/1.3.0": {
				status: http.StatusOK,
				body:   `{"name":"left-pad","dist": {"tarball": "https://registry.npmjs.org/left-pad/-/left-pad-1.3.0.tgz"}}`,
			},
		}
		resp, found := routes[req.URL.Path]
		if !found {
			panic(fmt.Sprintf("unexpected request to %s", req.URL.Path))
		}
		w.WriteHeader(resp.status)
		w.Write([]byte(resp.body))
	}))
}

func TestCredentials(t *testing.T) {
	credentials := "top secret access token"
	server := mockNpmServer(credentials)
	defer server.Close()

	ctx := context.Background()
	client := NewHTTPClient("urn", server.URL, credentials, httpcli.NewFactory(nil))
	client.limiter = ratelimit.NewInstrumentedLimiter("npm", rate.NewLimiter(100, 10))

	presentDep, err := reposource.ParseNpmVersionedPackage("left-pad@1.3.0")
	require.NoError(t, err)
	absentDep, err := reposource.ParseNpmVersionedPackage("left-pad@1.3.1")
	require.NoError(t, err)

	info, err := client.GetDependencyInfo(ctx, presentDep)
	require.NoError(t, err)
	require.NotNil(t, info)

	info, err = client.GetDependencyInfo(ctx, absentDep)
	require.Nil(t, info)
	require.ErrorAs(t, err, &npmError{})

	// Check that using the wrong credentials doesn't work
	client.credentials = "incorrect_credentials"

	info, err = client.GetDependencyInfo(ctx, presentDep)
	require.Nil(t, info)
	var npmErr1 npmError
	require.True(t, errors.As(err, &npmErr1) && npmErr1.statusCode == http.StatusUnauthorized)

	info, err = client.GetDependencyInfo(ctx, absentDep)
	require.Nil(t, info)
	var npmErr2 npmError
	require.True(t, errors.As(err, &npmErr2) && npmErr2.statusCode == http.StatusUnauthorized)
}

func TestGetPackage(t *testing.T) {
	ctx := context.Background()
	client, stop := newTestHTTPClient(t)
	defer stop()
	pkg, err := reposource.ParseNpmPackageFromPackageSyntax("is-sorted")
	require.Nil(t, err)
	info, err := client.GetPackageInfo(ctx, pkg)
	require.Nil(t, err)
	require.Equal(t, info.Description, "A small module to check if an Array is sorted")
	versions := []string{}
	for v := range info.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)
	require.Equal(t, versions, []string{"1.0.0", "1.0.1", "1.0.2", "1.0.3", "1.0.4", "1.0.5"})
}

func TestGetDependencyInfo(t *testing.T) {
	ctx := context.Background()
	client, stop := newTestHTTPClient(t)
	defer stop()
	dep, err := reposource.ParseNpmVersionedPackage("left-pad@1.3.0")
	require.NoError(t, err)
	info, err := client.GetDependencyInfo(ctx, dep)
	require.NoError(t, err)
	require.NotNil(t, info)
	dep, err = reposource.ParseNpmVersionedPackage("left-pad@1.3.1")
	require.NoError(t, err)
	info, err = client.GetDependencyInfo(ctx, dep)
	require.Nil(t, info)
	require.ErrorAs(t, err, &npmError{})
}

func TestFetchSources(t *testing.T) {
	ctx := context.Background()
	client, stop := newTestHTTPClient(t)
	defer stop()
	dep, err := reposource.ParseNpmVersionedPackage("is-sorted@1.0.0")
	require.Nil(t, err)
	info, err := client.GetDependencyInfo(ctx, dep)
	require.Nil(t, err)
	dep.TarballURL = info.Dist.TarballURL
	readSeekCloser, err := client.FetchTarball(ctx, dep)
	require.Nil(t, err)
	defer readSeekCloser.Close()
	tarFiles, err := unpack.ListTgzUnsorted(readSeekCloser)
	require.Nil(t, err)
	sort.Strings(tarFiles)
	require.Equal(t, tarFiles, []string{
		"package/.travis.yml",
		"package/LICENSE",
		"package/README.md",
		"package/index.js",
		"package/package.json",
		"package/test/fixtures.json",
		"package/test/index.js",
	})
}

func TestNoPanicOnNonexistentRegistry(t *testing.T) {
	ctx := context.Background()
	client, stop := newTestHTTPClient(t)
	defer stop()
	client.registryURL = "http://not-an-npm-registry.sourcegraph.com"
	dep, err := reposource.ParseNpmVersionedPackage("left-pad@1.3.0")
	require.Nil(t, err)
	info, err := client.GetDependencyInfo(ctx, dep)
	require.Error(t, err)
	require.Nil(t, info)
}
