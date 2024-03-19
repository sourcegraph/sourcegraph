package outboundwebhooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/webhooks/outbound"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestHandler_Handle(t *testing.T) {
	// This isn't a full blown integration test — we're going to mock pretty
	// much all the dependencies. This is just to ensure that the expected knobs
	// are twiddled in the expected scenarios.

	t.Run("success", func(t *testing.T) {
		ctx := context.Background()
		logger := logtest.Scoped(t)

		payload := []byte(`"test payload"`)
		secret := "shared secret"

		happyServer := newMockServer(t, payload, http.StatusOK)
		sadServer := newMockServer(t, payload, http.StatusInternalServerError)

		job := &types.OutboundWebhookJob{
			ID:        1,
			EventType: "event",
			Payload:   encryption.NewUnencrypted(string(payload)),
		}

		happyWebhook := &types.OutboundWebhook{
			ID:     1,
			URL:    encryption.NewUnencrypted(happyServer.URL),
			Secret: encryption.NewUnencrypted(secret),
		}
		sadWebhook := &types.OutboundWebhook{
			ID:     2,
			URL:    encryption.NewUnencrypted(sadServer.URL),
			Secret: encryption.NewUnencrypted(secret),
		}

		store := dbmocks.NewMockOutboundWebhookStore()
		store.ListFunc.SetDefaultReturn([]*types.OutboundWebhook{happyWebhook, sadWebhook}, nil)

		logStore := dbmocks.NewMockOutboundWebhookLogStore()
		webhooksSeen := newSeen[int64]()
		logStore.CreateFunc.SetDefaultHook(func(ctx context.Context, log *types.OutboundWebhookLog) error {
			assert.Equal(t, job.ID, log.JobID)
			webhooksSeen.record(log.OutboundWebhookID)

			// Ensure that the network error field is empty.
			errorMessage, err := log.Error.Decrypt(ctx)
			require.NoError(t, err)
			assert.Empty(t, errorMessage)

			// For the sadWebhook, we'll simulate a log write failure to ensure
			// that the handler still completes.
			if log.OutboundWebhookID == sadWebhook.ID {
				assert.EqualValues(t, http.StatusInternalServerError, log.StatusCode)
				return errors.New("no log for you!")
			}

			assert.EqualValues(t, http.StatusOK, log.StatusCode)
			return nil
		})

		h := &handler{
			client:   http.DefaultClient,
			store:    store,
			logStore: logStore,
		}

		outbound.SetTestDenyList()
		t.Cleanup(outbound.ResetDenyList)

		err := h.Handle(ctx, logger, job)
		// We expect an error here because sadServer returned a 500.
		assert.Error(t, err)

		mockassert.CalledN(t, store.ListFunc, 1)
		mockassert.CalledN(t, logStore.CreateFunc, 2)

		assert.EqualValues(t, 1, happyServer.requestCount)
		assert.EqualValues(t, 1, sadServer.requestCount)

		assert.EqualValues(t, 1, webhooksSeen.count(happyWebhook.ID))
		assert.EqualValues(t, 1, webhooksSeen.count(sadWebhook.ID))
	})

	t.Run("network failure", func(t *testing.T) {
		ctx := context.Background()
		logger := logtest.Scoped(t)

		payload := []byte(`"test payload"`)
		secret := "shared secret"

		job := &types.OutboundWebhookJob{
			ID:        1,
			EventType: "event",
			Payload:   encryption.NewUnencrypted(string(payload)),
		}

		webhook := &types.OutboundWebhook{
			ID:     1,
			URL:    encryption.NewUnencrypted("http://127.0.0.1/webhook-receiver/1234"),
			Secret: encryption.NewUnencrypted(secret),
		}

		store := dbmocks.NewMockOutboundWebhookStore()
		store.ListFunc.SetDefaultReturn([]*types.OutboundWebhook{webhook}, nil)

		want := errors.New("connection error")

		logStore := dbmocks.NewMockOutboundWebhookLogStore()
		logStore.CreateFunc.SetDefaultHook(func(ctx context.Context, log *types.OutboundWebhookLog) error {
			have, err := log.Error.Decrypt(ctx)
			require.NoError(t, err)
			assert.Contains(t, have, want.Error())

			return nil
		})

		h := &handler{
			client:   &http.Client{Transport: &badTransport{Err: want}},
			store:    store,
			logStore: logStore,
		}

		outbound.SetTestDenyList()
		t.Cleanup(outbound.ResetDenyList)

		err := h.Handle(ctx, logger, job)
		assert.ErrorIs(t, err, want)

		mockassert.CalledN(t, store.ListFunc, 1)
		mockassert.CalledN(t, logStore.CreateFunc, 1)
	})
}

type badTransport struct {
	Err error
}

var _ http.RoundTripper = &badTransport{}

func (t *badTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, t.Err
}

type mockServer struct {
	*httptest.Server
	requestCount int32
}

func newMockServer(t *testing.T, expectedPayload []byte, statusCode int) *mockServer {
	t.Helper()

	ms := &mockServer{}
	ms.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ms.requestCount, 1)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, expectedPayload, body)

		w.WriteHeader(statusCode)
	}))
	t.Cleanup(ms.Server.Close)

	return ms
}

type seen[T comparable] struct {
	mu   sync.RWMutex
	seen map[T]int
}

func newSeen[T comparable]() *seen[T] {
	return &seen[T]{
		seen: map[T]int{},
	}
}

func (s *seen[T]) count(value T) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.seen[value]
}

func (s *seen[T]) record(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seen[value] = s.seen[value] + 1
}
