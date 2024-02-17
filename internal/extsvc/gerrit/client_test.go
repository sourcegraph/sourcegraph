package gerrit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
)

func TestClient_do(t *testing.T) {
	// Setup test server with two routes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/unauthorized" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}
		w.Write([]byte(`)]}'{"key":"value"}`))
	}))
	srvURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	doer, err := httpcli.NewFactory(nil).Doer()
	require.NoError(t, err)

	c := &client{
		URL:        srvURL,
		rateLimit:  &ratelimit.InstrumentedLimiter{Limiter: rate.NewLimiter(10, 10)},
		httpClient: doer,
	}

	t.Run("prefix does not get trimmed if not present", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/unauthorized", nil)
		require.NoError(t, err)

		resp, err := c.do(context.Background(), req, nil)
		assert.Nil(t, resp)
		assert.Equal(t, fmt.Sprintf("Gerrit API HTTP error: code=401 url=\"%s/unauthorized\" body=\"Unauthorized\"", srvURL), err.Error())
	})

	t.Run("prefix gets trimmed if present", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "/anything", nil)
		require.NoError(t, err)

		respStruct := struct {
			Key string `json:"key"`
		}{}

		resp, err := c.do(context.Background(), req, &respStruct)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "value", respStruct.Key)
	})
}
