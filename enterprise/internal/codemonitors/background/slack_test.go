package background

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/hexops/autogold"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/search/result"
)

func TestSlackWebhook(t *testing.T) {
	t.Parallel()
	eu, err := url.Parse("https://sourcegraph.com")
	require.NoError(t, err)

	err = os.Setenv("WEBHOOK_ALLOWLIST", "loopback")
	require.NoError(t, err)

	action := actionArgs{
		MonitorDescription: "My test monitor",
		MonitorOwnerName:   "Camden Cheek",
		ExternalURL:        eu,
		Query:              "repo:camdentest -file:id_rsa.pub BEGIN",
		Results:            []*result.CommitMatch{&diffResultMock, &commitResultMock},
		IncludeResults:     false,
	}

	jsonSlackPayload := func(a actionArgs) autogold.Raw {
		b, err := json.MarshalIndent(slackPayload(a), " ", " ")
		require.NoError(t, err)
		return autogold.Raw(b)
	}

	t.Run("no error", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			autogold.Equal(t, autogold.Raw(b))
			w.WriteHeader(200)
		}))
		defer s.Close()

		err := postSlackWebhook(context.Background(), s.URL, slackPayload(action))
		require.NoError(t, err)
	})

	t.Run("error is returned", func(t *testing.T) {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			autogold.Equal(t, autogold.Raw(b))
			w.WriteHeader(500)
		}))
		defer s.Close()

		err := postSlackWebhook(context.Background(), s.URL, slackPayload(action))
		require.Error(t, err)
	})

	// If these tests fail, be sure to check that the changes are correct here:
	// https://app.slack.com/block-kit-builder/T02FSM7DL#%7B%22blocks%22:%5B%5D%7D
	t.Run("golden with results", func(t *testing.T) {
		actionCopy := action
		actionCopy.IncludeResults = true
		autogold.Equal(t, jsonSlackPayload(actionCopy))
	})

	t.Run("golden with truncated results", func(t *testing.T) {
		actionCopy := action
		actionCopy.IncludeResults = true
		// quadruple the number of results
		actionCopy.Results = append(actionCopy.Results, actionCopy.Results...)
		actionCopy.Results = append(actionCopy.Results, actionCopy.Results...)
		autogold.Equal(t, jsonSlackPayload(actionCopy))
	})

	t.Run("golden without results", func(t *testing.T) {
		autogold.Equal(t, jsonSlackPayload(action))
	})
}

func TestTriggerTestSlackWebhookAction(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		autogold.Equal(t, autogold.Raw(b))
		w.WriteHeader(200)
	}))
	defer s.Close()

	err := os.Setenv("WEBHOOK_ALLOWLIST", "loopback")
	require.NoError(t, err)

	err = SendTestSlackWebhook(context.Background(), "My test monitor", s.URL)
	require.NoError(t, err)
}
