package rubygems

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
)

// Run go test ./internal/extsvc/rubygems -update to update snapshots.
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

var updateRecordings = flag.Bool("update", false, "make npm API calls, record and save data")

func newTestHTTPClient(t *testing.T) (client *Client, stop func()) {
	t.Helper()
	recorderFactory, stop := httptestutil.NewRecorderFactory(t, *updateRecordings, t.Name())

	client = NewClient("rubygems_urn", "https://rubygems.org", recorderFactory)
	client.limiter = ratelimit.NewInstrumentedLimiter("rubygems", rate.NewLimiter(100, 10))
	return client, stop
}

func TestGetPackageContents(t *testing.T) {
	ctx := context.Background()
	client, stop := newTestHTTPClient(t)
	defer stop()
	dep := reposource.ParseRubyVersionedPackage("hola@0.1.0")
	readCloser, err := client.GetPackageContents(ctx, dep)
	require.Nil(t, err)
	defer readCloser.Close()

	tmpDir, err := os.MkdirTemp("", "test-rubygems-")
	require.Nil(t, err)
	err = unpack.Tar(readCloser, tmpDir, unpack.Opts{})
	require.Nil(t, err)
	dataTgz, err := os.ReadFile(filepath.Join(tmpDir, "data.tar.gz"))
	require.Nil(t, err)
	dataFiles, err := unpack.ListTgzUnsorted(bytes.NewReader(dataTgz))
	require.Nil(t, err)
	sort.Strings(dataFiles)

	require.Equal(t, dataFiles, []string{
		"Rakefile",
		"bin/hola",
		"lib/hola.rb",
		"lib/hola/translator.rb",
		"test/test_hola.rb",
	})
}
