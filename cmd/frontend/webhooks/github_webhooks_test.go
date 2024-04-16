package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	gh "github.com/google/go-github/v55/github"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestGithubWebhookDispatchSuccess(t *testing.T) {
	h := GitHubWebhook{Router: &Router{}}
	var called bool
	h.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		called = true
		return nil
	}, extsvc.KindGitHub, "test-event-1")

	ctx := context.Background()
	if err := h.Dispatch(ctx, "test-event-1", extsvc.KindGitHub, extsvc.CodeHostBaseURL{}, nil); err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	if !called {
		t.Errorf("Expected called to be true, was false")
	}
}

func TestGithubWebhookDispatchNoHandler(t *testing.T) {
	logger := logtest.Scoped(t)
	h := GitHubWebhook{Router: &Router{Logger: logger}}
	ctx := context.Background()

	eventType := "test-event-1"
	err := h.Dispatch(ctx, eventType, extsvc.KindGitHub, extsvc.CodeHostBaseURL{}, nil)
	assert.Nil(t, err)
}

func TestGithubWebhookDispatchSuccessMultiple(t *testing.T) {
	var (
		h      = GitHubWebhook{Router: &Router{}}
		called = make(chan struct{}, 2)
	)
	h.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		called <- struct{}{}
		return nil
	}, extsvc.KindGitHub, "test-event-1")
	h.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		called <- struct{}{}
		return nil
	}, extsvc.KindGitHub, "test-event-1")

	ctx := context.Background()
	if err := h.Dispatch(ctx, "test-event-1", extsvc.KindGitHub, extsvc.CodeHostBaseURL{}, nil); err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	if len(called) != 2 {
		t.Errorf("Expected called to be 2, got %v", called)
	}
}

func TestGithubWebhookDispatchError(t *testing.T) {
	var (
		h      = GitHubWebhook{Router: &Router{}}
		called = make(chan struct{}, 2)
	)
	h.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		called <- struct{}{}
		return errors.Errorf("oh no")
	}, extsvc.KindGitHub, "test-event-1")
	h.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		called <- struct{}{}
		return nil
	}, extsvc.KindGitHub, "test-event-1")

	ctx := context.Background()
	if have, want := h.Dispatch(ctx, "test-event-1", extsvc.KindGitHub, extsvc.CodeHostBaseURL{}, nil), "oh no"; errString(have) != want {
		t.Errorf("Expected %q, got %q", want, have)
	}
	if len(called) != 2 {
		t.Errorf("Expected called to be 2, got %v", called)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestGithubWebhookExternalServices(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	secret := "secret"
	esStore := db.ExternalServices()
	extSvc := &types.ExternalService{
		Kind:        extsvc.KindGitHub,
		DisplayName: "GitHub",
		Config: extsvc.NewUnencryptedConfig(marshalJSON(t, &schema.GitHubConnection{
			Authorization: &schema.GitHubAuthorization{},
			Url:           "https://github.com",
			Token:         "fake",
			Repos:         []string{"sourcegraph/sourcegraph"},
			Webhooks:      []*schema.GitHubWebhook{{Org: "sourcegraph", Secret: secret}},
		})),
	}

	err := esStore.Upsert(ctx, extSvc)
	externalServiceConfig := fmt.Sprintf(`
{
    // Some comment to mess with json decoding
    "url": "https://github.com",
    "token": "fake",
    "repos": ["sourcegraph/sourcegraph"],
    "webhooks": [
        {
            "org": "sourcegraph",
            "secret": %q
        }
    ]
}
`, secret)
	require.NoError(t, esStore.Update(ctx, []schema.AuthProviders{}, 1, &database.ExternalServiceUpdate{Config: &externalServiceConfig}))
	if err != nil {
		t.Fatal(err)
	}

	hook := GitHubWebhook{
		Router: &Router{
			DB: db,
		},
	}

	extURL, err := url.Parse("https://example.com/")
	require.NoError(t, err)

	var called bool
	hook.Register(func(ctx context.Context, db database.DB, urn extsvc.CodeHostBaseURL, payload any) error {
		evt, ok := payload.(*gh.PublicEvent)
		if !ok {
			t.Errorf("Expected *gh.PublicEvent event, got %T", payload)
		}
		if evt.GetRepo().GetFullName() != "sourcegraph/sourcegraph" {
			t.Errorf("Expected 'sourcegraph/sourcegraph', got %s", evt.GetRepo().GetFullName())
		}
		called = true
		return nil
	}, extsvc.KindGitHub, "public")

	u, err := extsvc.WebhookURL(extsvc.TypeGitHub, extSvc.ID, nil, extURL)
	if err != nil {
		t.Fatal(err)
	}

	urls := []string{
		// current webhook URLs, uses fast path for finding external service
		u,
		// old webhook URLs, finds external service by searching all configured external services
		"https://example.com/.api/github-webhook",
	}

	sendRequest := func(u, secret string) *http.Response {
		req, err := http.NewRequest("POST", u, bytes.NewReader(eventPayload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Github-Event", "public")
		if secret != "" {
			req.Header.Set("X-Hub-Signature", sign(t, eventPayload, []byte(secret)))
		}
		rec := httptest.NewRecorder()
		hook.ServeHTTP(rec, req)
		resp := rec.Result()
		return resp
	}

	t.Run("missing service", func(t *testing.T) {
		u, err := extsvc.WebhookURL(extsvc.TypeGitHub, 99, nil, extURL)
		if err != nil {
			t.Fatal(err)
		}
		called = false
		resp := sendRequest(u, secret)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.False(t, called)
	})

	t.Run("valid secret", func(t *testing.T) {
		for _, u := range urls {
			called = false
			resp := sendRequest(u, secret)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.True(t, called)
		}
	})

	t.Run("invalid secret", func(t *testing.T) {
		for _, u := range urls {
			called = false
			resp := sendRequest(u, "not_secret")
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.False(t, called)
		}
	})

	t.Run("no secret", func(t *testing.T) {
		// Secrets are optional and if they're not provided then the payload is not
		// signed and we don't need to validate it on our side
		for _, u := range urls {
			called = false
			resp := sendRequest(u, "")
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.True(t, called)
		}
	})
}

func marshalJSON(t testing.TB, v any) string {
	t.Helper()

	bs, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return string(bs)
}

func sign(t *testing.T, message, secret []byte) string {
	t.Helper()

	mac := hmac.New(sha256.New, secret)

	_, err := mac.Write(message)
	if err != nil {
		t.Fatalf("writing hmac message failed: %s", err)
	}

	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

var eventPayload = []byte(`{
  "repository": {
    "id": 310572870,
    "node_id": "MDEwOlJlcG9zaXRvcnkzMTA1NzI4NzA=",
    "name": "sourcegraph",
    "full_name": "sourcegraph/sourcegraph",
    "private": false,
    "owner": {
      "login": "sourcegraph",
      "id": 74051180,
      "node_id": "MDEyOk9yZ2FuaXphdGlvbjc0MDUxMTgw",
      "type": "Organization",
      "site_admin": false
    },
    "html_url": "https://github.com/sourcegraph",
    "created_at": "2020-11-06T11:02:56Z",
    "updated_at": "2020-11-09T15:06:34Z",
    "pushed_at": "2020-11-06T11:02:58Z",
    "default_branch": "main"
  },
  "organization": {
    "login": "sourcegraph",
    "id": 74051180,
    "node_id": "MDEyOk9yZ2FuaXphdGlvbjc0MDUxMTgw",
    "description": null
  },
  "sender": {
    "login": "sourcegraph",
    "id": 5236823,
    "node_id": "MDQ6VXNlcjUyMzY4MjM=",
    "type": "User",
    "site_admin": false
  }
}`)
