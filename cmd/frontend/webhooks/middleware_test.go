package webhooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mockassert "github.com/derision-test/go-mockgen/v2/testutil/assert"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSetExternalServiceID(t *testing.T) {
	ctx := context.Background()

	// Make sure SetExternalServiceID doesn't crash if there's no setter in the
	// context.
	SetExternalServiceID(ctx, 1)

	// Make sure it can handle an invalid setter.
	invalidCtx := context.WithValue(ctx, extSvcIDSetterContextKey, func() {
		panic("if we get as far as calling this, that's a bug")
	})
	SetExternalServiceID(invalidCtx, 1)

	// Now the real case: a valid setter.
	validCtx := context.WithValue(ctx, extSvcIDSetterContextKey, func(id int64) {
		assert.EqualValues(t, 42, id)
	})
	SetExternalServiceID(validCtx, 42)
}

func TestLogMiddleware(t *testing.T) {
	content := []byte("all systems operational")
	var es int64 = 42

	basicHandler := func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Add("foo", "bar")
		rw.WriteHeader(http.StatusCreated)
		rw.Write(content)
		SetExternalServiceID(r.Context(), es)
	}

	t.Run("logging disabled", func(t *testing.T) {
		conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{
			WebhookLogging: &schema.WebhookLogging{Enabled: pointers.Ptr(false)},
		}})
		defer conf.Mock(nil)

		store := dbmocks.NewMockWebhookLogStore()

		handler := http.HandlerFunc(basicHandler)
		mw := NewLogMiddleware(store)
		server := httptest.NewServer(mw.Logger(handler))
		defer server.Close()

		resp, err := server.Client().Get(server.URL)
		assert.Nil(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, content, body)

		// Check that no record was created.
		mockassert.NotCalled(t, store.CreateFunc)
	})

	t.Run("logging enabled", func(t *testing.T) {
		store := dbmocks.NewMockWebhookLogStore()
		store.CreateFunc.SetDefaultHook(func(c context.Context, log *types.WebhookLog) error {
			logRequest, err := log.Request.Decrypt(c)
			if err != nil {
				return err
			}
			logResponse, err := log.Response.Decrypt(c)
			if err != nil {
				return err
			}

			assert.Equal(t, es, *log.ExternalServiceID)
			assert.Equal(t, http.StatusCreated, log.StatusCode)
			assert.Equal(t, "GET", logRequest.Method)
			assert.Equal(t, "HTTP/1.1", logRequest.Version)
			assert.Equal(t, "bar", logResponse.Header.Get("foo"))
			assert.Equal(t, content, logResponse.Body)
			return nil
		})

		handler := http.HandlerFunc(basicHandler)
		mw := NewLogMiddleware(store)
		server := httptest.NewServer(mw.Logger(handler))
		defer server.Close()

		resp, err := server.Client().Get(server.URL)
		assert.Nil(t, err)
		defer resp.Body.Close()

		// Parse the body to ensure that the middleware didn't change the
		// response.
		body, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Equal(t, content, body)

		// Check the exactly one record was created.
		mockassert.CalledOnce(t, store.CreateFunc)
	})
}

func TestLoggingEnabled(t *testing.T) {
	for name, tc := range map[string]struct {
		c    *conf.Unified
		want bool
	}{
		"empty config": {c: &conf.Unified{}, want: true},
		"encryption; default webhook": {
			c: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				EncryptionKeys: &schema.EncryptionKeys{
					BatchChangesCredentialKey: &schema.EncryptionKey{
						Noop: &schema.NoOpEncryptionKey{
							Type: "noop",
						},
					},
				},
			}},
			want: false,
		},
		"encryption; explicit webhook false": {
			c: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				EncryptionKeys: &schema.EncryptionKeys{
					BatchChangesCredentialKey: &schema.EncryptionKey{
						Noop: &schema.NoOpEncryptionKey{
							Type: "noop",
						},
					},
				},
				WebhookLogging: &schema.WebhookLogging{
					Enabled: pointers.Ptr(false),
				},
			}},
			want: false,
		},
		"encryption; explicit webhook true": {
			c: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				EncryptionKeys: &schema.EncryptionKeys{
					BatchChangesCredentialKey: &schema.EncryptionKey{
						Noop: &schema.NoOpEncryptionKey{
							Type: "noop",
						},
					},
				},
				WebhookLogging: &schema.WebhookLogging{
					Enabled: pointers.Ptr(true),
				},
			}},
			want: true,
		},
		"no encryption; explicit webhook false": {
			c: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				WebhookLogging: &schema.WebhookLogging{
					Enabled: pointers.Ptr(false),
				},
			}},
			want: false,
		},
		"no encryption; explicit webhook true": {
			c: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				WebhookLogging: &schema.WebhookLogging{
					Enabled: pointers.Ptr(true),
				},
			}},
			want: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, LoggingEnabled(tc.c))
		})
	}
}
