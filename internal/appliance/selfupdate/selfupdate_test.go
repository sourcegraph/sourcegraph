package selfupdate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/releaseregistry"
	"github.com/sourcegraph/sourcegraph/internal/releaseregistry/mocks"
)

func TestReplaceTag(t *testing.T) {
	img := "index.docker.io/sourcegraph/appliance:1.2.3"
	updated := replaceTag(img, "4.5.6")
	require.Equal(t, "index.docker.io/sourcegraph/appliance:4.5.6", updated)
}

func TestReplaceTagNeverPanics(t *testing.T) {
	img := "badImageNameFormat"
	updated := replaceTag(img, "4.5.6")
	require.Equal(t, ":4.5.6", updated)
}

func TestGetLatestTag_ReturnsLatestSupportedPublicVersion(t *testing.T) {
	relregClient := mocks.NewMockReleaseRegistryClient()
	selfUpdater := &SelfUpdate{
		Logger:       logtest.Scoped(t),
		RelregClient: relregClient,
	}
	relregClient.ListVersionsFunc.PushReturn([]releaseregistry.ReleaseInfo{
		{Version: "v4.5.6", Public: false},
		{Version: "v4.5.5", Public: true},
		{Version: "v4.5.4", Public: true},
		{Version: "v4.5.3", Public: false},
		{Version: "v3.17.1", Public: true},
	}, nil)

	latest, err := selfUpdater.getLatestTag(context.Background())
	require.NoError(t, err)
	require.Equal(t, "4.5.5", latest)
}
