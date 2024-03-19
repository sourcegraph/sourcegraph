package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/transport/http/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/uploadhandler"
	uploadstoremocks "github.com/sourcegraph/sourcegraph/internal/uploadstore/mocks"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const testCommit = "deadbeef01deadbeef02deadbeef03deadbeef04"

func TestHandleEnqueueAuth(t *testing.T) {
	setupRepoMocks(t)

	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	repoStore := backend.NewRepos(logger, db, gitserver.NewMockClient())
	mockDBStore := NewMockDBStore[uploads.UploadMetadata]()
	mockUploadStore := uploadstoremocks.NewMockStore()

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			LsifEnforceAuth: true,
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	mockDBStore.WithTransactionFunc.SetDefaultHook(func(ctx context.Context, f func(tx uploadhandler.DBStore[uploads.UploadMetadata]) error) error {
		return f(mockDBStore)
	})
	mockDBStore.InsertUploadFunc.SetDefaultReturn(42, nil)

	testURL, err := url.Parse("http://test.com/upload")
	if err != nil {
		t.Fatalf("unexpected error constructing url: %s", err)
	}
	testURL.RawQuery = (url.Values{
		"commit":      []string{testCommit},
		"root":        []string{"proj/"},
		"repository":  []string{"github.com/test/test"},
		"indexerName": []string{"lsif-go"},
	}).Encode()

	users := []struct {
		name       string
		siteAdmin  bool
		noUser     bool
		statusCode int
	}{
		{
			name:       "chad",
			siteAdmin:  true,
			statusCode: http.StatusAccepted,
		},
		{
			name:       "owning-user",
			siteAdmin:  false,
			statusCode: http.StatusAccepted,
		},
		{
			name:       "non-owning-user",
			siteAdmin:  false,
			statusCode: http.StatusUnauthorized,
		},
		{
			noUser:     true,
			statusCode: http.StatusUnauthorized,
		},
	}

	for _, user := range users {
		var expectedContents []byte
		for i := range 20000 {
			expectedContents = append(expectedContents, byte(i))
		}

		w := httptest.NewRecorder()
		r, err := http.NewRequest("POST", testURL.String(), bytes.NewReader(expectedContents))
		if err != nil {
			t.Fatalf("unexpected error constructing request: %s", err)
		}

		if !user.noUser {
			userID := insertTestUser(t, db, user.name, user.siteAdmin)
			r = r.WithContext(actor.WithActor(r.Context(), actor.FromUser(userID)))
		}

		authValidators := auth.AuthValidatorMap{
			"github": func(context.Context, url.Values, string) (int, error) {
				if user.name != "owning-user" {
					return http.StatusUnauthorized, errors.New("sample text import cycle")
				}
				return 200, nil
			},
		}

		auth.AuthMiddleware(
			newHandler(
				repoStore,
				mockUploadStore,
				mockDBStore,
				uploadhandler.NewOperations(&observation.TestContext, "test"),
			),
			db.Users(),
			repoStore,
			authValidators,
			newOperations(&observation.TestContext).authMiddleware,
		).ServeHTTP(w, r)

		if w.Code != user.statusCode {
			t.Errorf("unexpected status code for user %s. want=%d have=%d", user.name, user.statusCode, w.Code)
		}
	}
}

func setupRepoMocks(t testing.TB) {
	t.Cleanup(func() {
		backend.Mocks.Repos.GetByName = nil
		backend.Mocks.Repos.ResolveRev = nil
	})

	backend.Mocks.Repos.GetByName = func(ctx context.Context, name api.RepoName) (*types.Repo, error) {
		if name != "github.com/test/test" {
			t.Errorf("unexpected repository name. want=%s have=%s", "github.com/test/test", name)
		}
		return &types.Repo{ID: 50}, nil
	}

	backend.Mocks.Repos.ResolveRev = func(ctx context.Context, repo api.RepoName, rev string) (api.CommitID, error) {
		if rev != testCommit {
			t.Errorf("unexpected commit. want=%s have=%s", testCommit, rev)
		}
		return "", nil
	}
}

func insertTestUser(t *testing.T, db database.DB, name string, isAdmin bool) (userID int32) {
	t.Helper()

	q := sqlf.Sprintf("INSERT INTO users (username, site_admin) VALUES (%s, %t) RETURNING id", name, isAdmin)
	err := db.QueryRowContext(context.Background(), q.Query(sqlf.PostgresBindVar), q.Args()...).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	return userID
}
