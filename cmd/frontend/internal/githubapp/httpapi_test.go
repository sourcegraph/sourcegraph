package githubapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/github_apps/store"
	ghtypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestGenerateRedirectURL(t *testing.T) {
	reposDomain := "repos"
	batchesDomain := "batches"
	invalidDomain := "invalid"
	appName := "my-cool-app"
	creationErr := errors.New("uh oh!")

	testCases := []struct {
		name           string
		domain         *string
		installationID int
		appID          int
		creationErr    error
		expectedURL    string
	}{
		{
			name:           "repos domain",
			domain:         &reposDomain,
			installationID: 1,
			appID:          2,
			expectedURL:    "/site-admin/github-apps/R2l0SHViQXBwOjI=?installation_id=1",
		},
		{
			name:           "batches domain",
			domain:         &batchesDomain,
			installationID: 1,
			appID:          2,
			expectedURL:    "/site-admin/batch-changes?success=true&app_name=my-cool-app",
		},
		{
			name:        "invalid domain",
			domain:      &invalidDomain,
			expectedURL: "/site-admin/github-apps?success=false&error=invalid+domain%3A+invalid",
		},
		{
			name:        "repos creation error",
			domain:      &reposDomain,
			creationErr: creationErr,
			expectedURL: "/site-admin/github-apps?success=false&error=uh+oh%21",
		},
		{
			name:        "batches creation error",
			domain:      &batchesDomain,
			creationErr: creationErr,
			expectedURL: "/site-admin/batch-changes?success=false&error=uh+oh%21",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			url := generateRedirectURL(gitHubAppStateDetails{
				Domain: *tc.domain,
				AppID:  tc.appID,
			}, &tc.installationID, &appName, tc.creationErr)
			require.Equal(t, tc.expectedURL, url)
		})
	}
}

func TestGithubAppHTTPAPI(t *testing.T) {
	t.Cleanup(func() {
		MockCreateGitHubApp = nil
	})

	webhookUUID := uuid.New()

	mockUserStore := dbmocks.NewMockUserStore()
	mockUserStore.GetByCurrentAuthUserFunc.SetDefaultHook(func(ctx context.Context) (*types.User, error) {
		a := actor.FromContext(ctx)
		return &types.User{
			ID:        a.UID,
			SiteAdmin: a.UID == 2,
		}, nil
	})

	mockWebhookStore := dbmocks.NewMockWebhookStore()
	mockWebhookStore.CreateFunc.SetDefaultHook(func(ctx context.Context, name, kind, urn string, actorUID int32, e *encryption.Encryptable) (*types.Webhook, error) {
		return &types.Webhook{
			ID:              1,
			UUID:            webhookUUID,
			Name:            name,
			CodeHostKind:    kind,
			CreatedByUserID: actorUID,
			UpdatedByUserID: actorUID,
		}, nil
	})
	mockWebhookStore.GetByUUIDFunc.SetDefaultReturn(&types.Webhook{
		ID:   1,
		UUID: webhookUUID,
		Name: "test-github-app",
	}, nil)
	mockWebhookStore.UpdateFunc.SetDefaultHook(func(ctx context.Context, w *types.Webhook) (*types.Webhook, error) {
		return w, nil
	})

	mockGitHubAppsStore := store.NewMockGitHubAppsStore()
	mockGitHubAppsStore.CreateFunc.SetDefaultReturn(1, nil)
	mockGitHubAppsStore.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int) (*ghtypes.GitHubApp, error) {
		return &ghtypes.GitHubApp{
			ID: id,
		}, nil
	})

	db := dbmocks.NewMockDB()

	db.UsersFunc.SetDefaultReturn(mockUserStore)
	db.WebhooksFunc.SetDefaultReturn(mockWebhookStore)
	db.GitHubAppsFunc.SetDefaultReturn(mockGitHubAppsStore)

	rcache.SetupForTest(t)
	cache := rcache.NewWithTTL("test_cache", 200)

	mux := mux.NewRouter()
	subrouter := mux.PathPrefix("/githubapp/").Subrouter()

	setupGitHubAppRoutesWithCache(subrouter, db, cache)

	t.Run("/state", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/githubapp/state", nil)

		t.Run("regular user", func(t *testing.T) {
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 1,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected status code %d but got %d", http.StatusForbidden, w.Code)
			}
		})

		t.Run("site-admin", func(t *testing.T) {
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status code %d but got %d", http.StatusOK, w.Code)
			}

			state := w.Body.String()
			if state == "" {
				t.Fatal("expected non-empty state in response")
			}

			cachedState, ok := cache.Get(state)
			if !ok {
				t.Fatal("expected state to be cached")
			}

			var stateDetails gitHubAppStateDetails
			if err := json.Unmarshal(cachedState, &stateDetails); err != nil {
				t.Fatalf("unexpected error unmarshalling cached state: %s", err.Error())
			}

			if stateDetails.AppID != 0 {
				t.Fatal("expected AppID to be 0 for empty state")
			}
		})
	})

	t.Run("/new-app-state", func(t *testing.T) {
		webhookURN := "https://example.com"
		appName := "TestApp"
		domain := "batches"
		baseURL := "https://ghe.example.org"
		kind := types.CommitSigningGitHubAppKind.ToGraphQL()
		req := httptest.NewRequest("GET", fmt.Sprintf("/githubapp/new-app-state?webhookURN=%s&appName=%s&domain=%s&baseURL=%s&kind=%s", webhookURN, appName, domain, baseURL, kind), nil)

		t.Run("normal user", func(t *testing.T) {
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 1,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected status code %d but got %d", http.StatusForbidden, w.Code)
			}
		})

		t.Run("site-admin", func(t *testing.T) {
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected status code %d but got %d", http.StatusOK, w.Code)
			}

			var resp struct {
				State       string `json:"state"`
				WebhookUUID string `json:"webhookUUID,omitempty"`
			}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("unexpected error decoding response: %s", err.Error())
			}

			if resp.State == "" {
				t.Fatal("expected non-empty state in response")
			}
			if resp.WebhookUUID == "" {
				t.Fatal("expected non-empty webhookUUID in response")
			}

			cachedState, ok := cache.Get(resp.State)
			if !ok {
				t.Fatal("expected state to be cached")
			}

			var stateDetails gitHubAppStateDetails
			if err := json.Unmarshal(cachedState, &stateDetails); err != nil {
				t.Fatalf("unexpected error unmarshalling cached state: %s", err.Error())
			}

			if stateDetails.WebhookUUID != resp.WebhookUUID {
				t.Fatal("expected webhookUUID in state details to match response")
			}
			if stateDetails.Domain != domain {
				t.Fatal("expected domain in state details to match request param")
			}
			if stateDetails.BaseURL != baseURL {
				t.Fatal("expected baseURL in state details to match request param")
			}
		})
	})

	t.Run("/redirect", func(t *testing.T) {
		baseURL := "/githubapp/redirect"
		code := "2644896245sasdsf6dsd"
		state, err := randomState()
		if err != nil {
			t.Fatalf("unexpected error generating random state: %s", err.Error())
		}
		domain := types.BatchesGitHubAppDomain
		stateBaseURL := "https://github.com"

		t.Run("normal user", func(t *testing.T) {
			req := httptest.NewRequest("GET", baseURL, nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 1,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected status code %d but got %d", http.StatusForbidden, w.Code)
			}
		})

		t.Run("without state", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?code=%s", baseURL, code), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status code %d but got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("without code", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?state=%s", baseURL, state), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status code %d but got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("success", func(t *testing.T) {
			MockCreateGitHubApp = func(conversionURL string, domain types.GitHubAppDomain) (*ghtypes.GitHubApp, error) {
				return &ghtypes.GitHubApp{}, nil
			}
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?state=%s&code=%s", baseURL, state, code), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			stateDeets, err := json.Marshal(gitHubAppStateDetails{
				WebhookUUID: webhookUUID.String(),
				Domain:      string(domain),
				BaseURL:     stateBaseURL,
			})
			require.NoError(t, err)
			cache.Set(state, stateDeets)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusSeeOther {
				t.Fatalf("expected status code %d but got %d", http.StatusOK, w.Code)
			}
		})
	})

	t.Run("/setup", func(t *testing.T) {
		baseURL := "/githubapp/setup"
		state, err := randomState()
		if err != nil {
			t.Fatalf("unexpected error generating random state: %s", err.Error())
		}
		installationID := 232034243
		domain := types.BatchesGitHubAppDomain

		t.Run("normal user", func(t *testing.T) {
			req := httptest.NewRequest("GET", baseURL, nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 1,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Fatalf("expected status code %d but got %d", http.StatusForbidden, w.Code)
			}
		})

		t.Run("without state", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?installation_id=%d", baseURL, installationID), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusFound {
				t.Fatalf("expected status code %d but got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("without installation_id", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?state=%s", baseURL, state), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusFound {
				t.Fatalf("expected status code %d but got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("without setup_action", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?installation_id=%d&state=%s", baseURL, installationID, state), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			stateDeets, err := json.Marshal(gitHubAppStateDetails{})
			require.NoError(t, err)
			cache.Set(state, stateDeets)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected status code %d but got %d", http.StatusBadRequest, w.Code)
			}
		})

		t.Run("success", func(t *testing.T) {
			req := httptest.NewRequest("GET", fmt.Sprintf("%s?installation_id=%d&state=%s&setup_action=install", baseURL, installationID, state), nil)
			req = req.WithContext(actor.WithActor(req.Context(), &actor.Actor{
				UID: 2,
			}))

			stateDeets, err := json.Marshal(gitHubAppStateDetails{
				Domain: string(domain),
			})
			require.NoError(t, err)
			cache.Set(state, stateDeets)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusFound {
				t.Fatalf("expected status code %d but got %d", http.StatusOK, w.Code)
			}
		})
	})
}

func TestCreateGitHubApp(t *testing.T) {
	tests := []struct {
		name          string
		domain        types.GitHubAppDomain
		handlerAssert func(t *testing.T) http.HandlerFunc
		expected      *ghtypes.GitHubApp
		expectedErr   error
		kind          types.GitHubAppKind
	}{
		{
			name:   "success",
			domain: types.BatchesGitHubAppDomain,
			handlerAssert: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, http.MethodPost, r.Method)

					w.WriteHeader(http.StatusCreated)

					resp := GitHubAppResponse{
						AppID:         1,
						Slug:          "test/github-app",
						Name:          "test",
						HtmlURL:       "http://my-github-app.com/app",
						ClientID:      "abc",
						ClientSecret:  "password",
						PEM:           "some-pem",
						WebhookSecret: "secret",
						Permissions: map[string]string{
							"checks": "write",
						},
						Events: []string{
							"check_run",
						},
					}
					err := json.NewEncoder(w).Encode(resp)
					require.NoError(t, err)
				}
			},
			kind: types.CommitSigningGitHubAppKind,
			expected: &ghtypes.GitHubApp{
				AppID:         1,
				Name:          "test",
				Slug:          "test/github-app",
				ClientID:      "abc",
				ClientSecret:  "password",
				WebhookSecret: "secret",
				PrivateKey:    "some-pem",
				BaseURL:       "http://my-github-app.com",
				AppURL:        "http://my-github-app.com/app",
				Domain:        types.BatchesGitHubAppDomain,
				Logo:          "http://my-github-app.com/identicons/app/app/test/github-app",
				Kind:          types.CommitSigningGitHubAppKind,
			},
		},
		{
			name:   "unexpected status code",
			domain: types.BatchesGitHubAppDomain,
			kind:   types.CommitSigningGitHubAppKind,
			handlerAssert: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}
			},
			expectedErr: errors.New("expected 201 statusCode, got: 200"),
		},
		{
			name:   "server error",
			domain: types.BatchesGitHubAppDomain,
			kind:   types.CommitSigningGitHubAppKind,
			handlerAssert: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}
			},
			expectedErr: errors.New("expected 201 statusCode, got: 500"),
		},
		{
			name:   "invalid html url",
			domain: types.BatchesGitHubAppDomain,
			kind:   types.CommitSigningGitHubAppKind,
			handlerAssert: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)

					resp := GitHubAppResponse{HtmlURL: ":"}
					err := json.NewEncoder(w).Encode(resp)
					require.NoError(t, err)
				}
			},
			expectedErr: errors.New("parse \":\": missing protocol scheme"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			srv := httptest.NewServer(test.handlerAssert(t))
			defer srv.Close()

			app, err := createGitHubApp(srv.URL, test.domain, httpcli.TestExternalClient, test.kind)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
				assert.Nil(t, app)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, app)
			}
		})
	}
}
