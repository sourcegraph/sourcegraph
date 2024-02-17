package releasecache

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
)

func TestReleaseCache_Current(t *testing.T) {
	rc := &releaseCache{
		branches: map[string]string{
			"3.43": "3.43.4",
		},
	}

	t.Run("not found", func(t *testing.T) {
		_, err := rc.Current("4.0")
		assert.True(t, errcode.IsNotFound(err))
		var berr branchNotFoundError
		assert.ErrorAs(t, err, &berr)
		assert.Equal(t, "4.0", string(berr))
	})

	t.Run("found", func(t *testing.T) {
		version, err := rc.Current("3.43")
		assert.NoError(t, err)
		assert.Equal(t, "3.43.4", version)
	})
}

func TestReleaseCache_Fetch(t *testing.T) {
	// We'll just test the happy path in here; the error handling is
	// straightforward, and the mechanics of parsing the versions is unit tested
	// in TestProcessReleases.

	ctx := context.Background()
	logger, _ := logtest.Captured(t)
	ratelimit.SetupForTest(t)
	rc := &releaseCache{
		client: newTestClient(t),
		logger: logger,
		owner:  "sourcegraph",
		name:   "src-cli",
	}

	err := rc.fetch(ctx)
	assert.NoError(t, err)
	testutil.AssertGolden(t, "testdata/golden/"+t.Name(), update(t.Name()), rc.branches)
}

func TestProcessReleases(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		branches := map[string]string{}
		processReleases(nil, branches, []github.Release{})
		assert.Empty(t, branches)
	})

	t.Run("full", func(t *testing.T) {
		branches := map[string]string{}
		releases := []github.Release{
			{TagName: "4.0.0-rc.1", IsPrerelease: true, IsDraft: true},
			{TagName: "4.0.0-rc.0", IsPrerelease: true},
			{TagName: "3.43.4", IsDraft: true},
			{TagName: "3.43.4-rc.0", IsPrerelease: true},
			{TagName: "3.43.3"},
			{TagName: "3.43.2"},
			{TagName: "3.43.1"},
			{TagName: "3.43.0"},
		}
		processReleases(nil, branches, releases)
		assert.Equal(t, map[string]string{
			"3.43": "3.43.3",
		}, branches)
	})

	t.Run("multiple invocations", func(t *testing.T) {
		branches := map[string]string{}
		releases := []github.Release{
			{TagName: "4.0.0-rc.1", IsPrerelease: true, IsDraft: true},
			{TagName: "4.0.0-rc.0", IsPrerelease: true},
		}
		processReleases(nil, branches, releases)
		assert.Empty(t, branches)

		releases = []github.Release{
			{TagName: "3.43.4", IsDraft: true},
			{TagName: "3.43.4-rc.0", IsPrerelease: true},
			{TagName: "3.43.3"},
			{TagName: "3.43.2"},
			{TagName: "3.43.1"},
			{TagName: "3.43.0"},
		}
		processReleases(nil, branches, releases)
		assert.Equal(t, map[string]string{
			"3.43": "3.43.3",
		}, branches)

		releases = []github.Release{
			{TagName: "3.42.9"},
			{TagName: "3.42.8"},
		}
		processReleases(nil, branches, releases)
		assert.Equal(t, map[string]string{
			"3.42": "3.42.9",
			"3.43": "3.43.3",
		}, branches)
	})

	t.Run("malformed release", func(t *testing.T) {
		logger, exportLogs := logtest.Captured(t)
		branches := map[string]string{}
		releases := []github.Release{
			{TagName: "foobar"},
		}
		processReleases(logger, branches, releases)
		assert.Empty(t, branches)
		assert.Len(t, exportLogs(), 1)
	})
}

func newTestClient(t *testing.T) *github.V4Client {
	t.Helper()

	cf, save := newClientFactory(t, t.Name())
	t.Cleanup(func() { save(t) })

	u, err := url.Parse("https://api.github.com")
	require.NoError(t, err)

	a := auth.OAuthBearerToken{Token: os.Getenv("GITHUB_TOKEN")}

	c, err := github.NewV4Client("https://github.com", u, &a, cf)
	require.NoError(t, err)
	return c
}
