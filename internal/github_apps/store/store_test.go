package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v55/github"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	gh "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghtypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func newTestStore(t *testing.T) *gitHubAppsStore {
	logger := logtest.Scoped(t)
	return &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}

}

func TestCreateGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	store := newTestStore(t)

	app := &ghtypes.GitHubApp{
		AppID:        1,
		Name:         "Test App",
		Domain:       "repos",
		BaseURL:      "https://github.com/",
		Slug:         "test-app",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
		AppURL:       "https://github.com/apps/testapp",
	}

	id, err := store.Create(ctx, app)
	require.NoError(t, err)

	var createdApp ghtypes.GitHubApp
	query := sqlf.Sprintf(`SELECT app_id, name, domain, slug, base_url, app_url, client_id, client_secret, private_key, encryption_key_id, logo FROM github_apps WHERE id=%s`, id)
	err = store.QueryRow(ctx, query).Scan(
		&createdApp.AppID,
		&createdApp.Name,
		&createdApp.Domain,
		&createdApp.Slug,
		&createdApp.BaseURL,
		&createdApp.AppURL,
		&createdApp.ClientID,
		&createdApp.ClientSecret,
		&createdApp.PrivateKey,
		&createdApp.EncryptionKey,
		&createdApp.Logo,
	)
	require.NoError(t, err)
	require.Equal(t, app, &createdApp)
}

func TestDeleteGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		Name:         "Test App",
		Domain:       "repos",
		Slug:         "test-app",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
	}

	id, err := store.Create(ctx, app)
	require.NoError(t, err)

	err = store.Delete(ctx, id)
	require.NoError(t, err)

	query := sqlf.Sprintf(`SELECT * FROM github_apps WHERE id=%s`, id)
	row, err := store.Query(ctx, query)
	require.NoError(t, err)
	// expect false since the query should not return any results
	require.False(t, row.Next())

	// deleting non-existent should not return error
	err = store.Delete(ctx, id)
	require.NoError(t, err)
}

func TestUpdateGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		AppID:        123,
		Name:         "Test App",
		Domain:       "repos",
		Slug:         "test-app",
		BaseURL:      "https://example.com/",
		AppURL:       "https://example.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
	}

	id, err := store.Create(ctx, app)
	require.NoError(t, err)

	app, err = store.GetByID(ctx, id)
	require.NoError(t, err)

	updated := &ghtypes.GitHubApp{
		AppID:        234,
		Name:         "Updated Name",
		Domain:       "repos",
		Slug:         "updated-slug",
		BaseURL:      "https://updated-example.com/",
		AppURL:       "https://updated-example.com/apps/updated-app",
		ClientID:     "def456",
		ClientSecret: "updated-secret",
		PrivateKey:   "updated-private-key",
	}

	fetched, err := store.Update(ctx, 1, updated)
	require.NoError(t, err)

	require.Greater(t, fetched.UpdatedAt, app.UpdatedAt)

	require.Equal(t, updated.AppID, fetched.AppID)
	require.Equal(t, updated.Name, fetched.Name)
	require.Equal(t, updated.Domain, fetched.Domain)
	require.Equal(t, updated.Slug, fetched.Slug)
	require.Equal(t, updated.BaseURL, fetched.BaseURL)
	require.Equal(t, updated.ClientID, fetched.ClientID)
	require.Equal(t, updated.ClientSecret, fetched.ClientSecret)
	require.Equal(t, updated.PrivateKey, fetched.PrivateKey)
	require.Equal(t, updated.Logo, fetched.Logo)

	// updating non-existent should result in error
	_, err = store.Update(ctx, 42, updated)
	require.Error(t, err)
}

func TestGetByID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app1 := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app-1",
		BaseURL:      "https://github.com/",
		AppURL:       "https://github.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	app2 := &ghtypes.GitHubApp{
		AppID:        5678,
		Name:         "Test App 2",
		Domain:       "repos",
		Slug:         "test-app-2",
		BaseURL:      "https://enterprise.github.com/",
		AppURL:       "https://enterprise.github.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	id1, err := store.Create(ctx, app1)
	require.NoError(t, err)
	id2, err := store.Create(ctx, app2)
	require.NoError(t, err)

	fetched, err := store.GetByID(ctx, id1)
	require.NoError(t, err)
	require.Equal(t, app1.AppID, fetched.AppID)
	require.Equal(t, app1.Name, fetched.Name)
	require.Equal(t, app1.Domain, fetched.Domain)
	require.Equal(t, app1.Slug, fetched.Slug)
	require.Equal(t, app1.BaseURL, fetched.BaseURL)
	require.Equal(t, app1.ClientID, fetched.ClientID)
	require.Equal(t, app1.ClientSecret, fetched.ClientSecret)
	require.Equal(t, app1.PrivateKey, fetched.PrivateKey)
	require.Equal(t, app1.Logo, fetched.Logo)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)

	fetched, err = store.GetByID(ctx, id2)
	require.NoError(t, err)
	require.Equal(t, app2.AppID, fetched.AppID)

	// does not exist
	_, err = store.GetByID(ctx, 42)
	require.Error(t, err)
}

func TestGetByAppID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app1 := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app-1",
		BaseURL:      "https://github.com/",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	app2 := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 2",
		Domain:       "repos",
		Slug:         "test-app-2",
		BaseURL:      "https://enterprise.github.com/",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	_, err := store.Create(ctx, app1)
	require.NoError(t, err)
	_, err = store.Create(ctx, app2)
	require.NoError(t, err)

	fetched, err := store.GetByAppID(ctx, 1234, "https://github.com/")
	require.NoError(t, err)
	require.Equal(t, app1.AppID, fetched.AppID)
	require.Equal(t, app1.Name, fetched.Name)
	require.Equal(t, app1.Domain, fetched.Domain)
	require.Equal(t, app1.Slug, fetched.Slug)
	require.Equal(t, app1.BaseURL, fetched.BaseURL)
	require.Equal(t, app1.ClientID, fetched.ClientID)
	require.Equal(t, app1.ClientSecret, fetched.ClientSecret)
	require.Equal(t, app1.PrivateKey, fetched.PrivateKey)
	require.Equal(t, app1.Logo, fetched.Logo)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)

	fetched, err = store.GetByAppID(ctx, 1234, "https://enterprise.github.com/")
	require.NoError(t, err)
	require.Equal(t, app2.AppID, fetched.AppID)
	require.Equal(t, app2.Slug, fetched.Slug)

	// does not exist
	_, err = store.GetByAppID(ctx, 3456, "https://github.com/")
	require.Error(t, err)
}

func TestGetBySlug(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app1 := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app",
		BaseURL:      "https://github.com/",
		AppURL:       "https://github.com/apps/testapp1",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	app2 := &ghtypes.GitHubApp{
		AppID:        5678,
		Name:         "Test App",
		Domain:       "repos",
		Slug:         "test-app",
		BaseURL:      "https://enterprise.github.com/",
		AppURL:       "https://enterprise.github.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	_, err := store.Create(ctx, app1)
	require.NoError(t, err)
	_, err = store.Create(ctx, app2)
	require.NoError(t, err)

	fetched, err := store.GetBySlug(ctx, "test-app", "https://github.com/")
	require.NoError(t, err)
	require.Equal(t, app1.AppID, fetched.AppID)
	require.Equal(t, app1.Name, fetched.Name)
	require.Equal(t, app1.Domain, fetched.Domain)
	require.Equal(t, app1.Slug, fetched.Slug)
	require.Equal(t, app1.BaseURL, fetched.BaseURL)
	require.Equal(t, app1.ClientID, fetched.ClientID)
	require.Equal(t, app1.ClientSecret, fetched.ClientSecret)
	require.Equal(t, app1.PrivateKey, fetched.PrivateKey)
	require.Equal(t, app1.Logo, fetched.Logo)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)

	fetched, err = store.GetBySlug(ctx, "test-app", "https://enterprise.github.com/")
	require.NoError(t, err)
	require.Equal(t, app2.AppID, fetched.AppID)

	// does not exist
	_, err = store.GetBySlug(ctx, "foo", "bar")
	require.Error(t, err)
}

func TestGetByDomain(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	repoApp := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Repo App",
		Domain:       "repos",
		Slug:         "repos-app",
		BaseURL:      "https://github.com/",
		AppURL:       "https://github.com/apps/test-repos-app",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	batchesApp := &ghtypes.GitHubApp{
		AppID:        5678,
		Name:         "Batches App",
		Domain:       "batches",
		Slug:         "batches-app",
		BaseURL:      "https://github.com/",
		AppURL:       "https://github.com/apps/test-batches-app",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	_, err := store.Create(ctx, repoApp)
	require.NoError(t, err)
	_, err = store.Create(ctx, batchesApp)
	require.NoError(t, err)

	domain := types.ReposGitHubAppDomain
	fetched, err := store.GetByDomainAndKind(ctx, domain, "https://github.com/", types.RepoSyncGitHubAppKind)
	require.NoError(t, err)
	require.Equal(t, repoApp.AppID, fetched.AppID)
	require.Equal(t, repoApp.Name, fetched.Name)
	require.Equal(t, repoApp.Domain, fetched.Domain)
	require.Equal(t, repoApp.Slug, fetched.Slug)
	require.Equal(t, repoApp.BaseURL, fetched.BaseURL)
	require.Equal(t, repoApp.ClientID, fetched.ClientID)
	require.Equal(t, repoApp.ClientSecret, fetched.ClientSecret)
	require.Equal(t, repoApp.PrivateKey, fetched.PrivateKey)
	require.Equal(t, repoApp.Logo, fetched.Logo)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)

	// does not exist
	fetched, err = store.GetByDomainAndKind(ctx, domain, "https://myCompany.github.com/", types.RepoSyncGitHubAppKind)
	require.Nil(t, fetched)
	require.Error(t, err)
	notFoundErr, ok := err.(ErrNoGitHubAppFound)
	require.Equal(t, ok, true)
	require.Equal(t, notFoundErr.Error(), "no app exists matching criteria: 'domain = repos AND trim(trailing '/' from base_url) = https://myCompany.github.com'")

	domain = types.BatchesGitHubAppDomain
	fetched, err = store.GetByDomainAndKind(ctx, domain, "https://github.com/", types.CommitSigningGitHubAppKind)
	require.NoError(t, err)
	require.Equal(t, batchesApp.AppID, fetched.AppID)
}

func TestListGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	store := newTestStore(t)
	ctx := context.Background()

	repoApp := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       types.ReposGitHubAppDomain,
		Slug:         "test-app-1",
		BaseURL:      "https://github.com/",
		AppURL:       "https://github.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	batchesApp := &ghtypes.GitHubApp{
		AppID:        5678,
		Name:         "Test App 2",
		Domain:       types.BatchesGitHubAppDomain,
		Slug:         "test-app-2",
		BaseURL:      "https://enterprise.github.com/",
		AppURL:       "https://enterprise.github.com/apps/testapp",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	_, err := store.Create(ctx, repoApp)
	require.NoError(t, err)
	_, err = store.Create(ctx, batchesApp)
	require.NoError(t, err)

	t.Run("all github apps", func(t *testing.T) {
		fetched, err := store.List(ctx, nil)
		require.NoError(t, err)
		require.Len(t, fetched, 2)

		apps := []*ghtypes.GitHubApp{repoApp, batchesApp}
		for index, curr := range fetched {
			app := apps[index]
			require.Equal(t, app.AppID, curr.AppID)
			require.Equal(t, app.Name, curr.Name)
			require.Equal(t, app.Domain, curr.Domain)
			require.Equal(t, app.Slug, curr.Slug)
			require.Equal(t, app.BaseURL, curr.BaseURL)
			require.Equal(t, app.ClientID, curr.ClientID)
			require.Equal(t, app.ClientSecret, curr.ClientSecret)
			require.Equal(t, app.PrivateKey, curr.PrivateKey)
			require.Equal(t, app.Logo, curr.Logo)
			require.NotZero(t, curr.CreatedAt)
			require.NotZero(t, curr.UpdatedAt)
		}
	})

	t.Run("domain-filtered github apps", func(t *testing.T) {
		domain := types.ReposGitHubAppDomain
		fetched, err := store.List(ctx, &domain)
		require.NoError(t, err)
		require.Len(t, fetched, 1)

		curr := fetched[0]
		require.Equal(t, curr.AppID, repoApp.AppID)
		require.Equal(t, curr.Name, repoApp.Name)
		require.Equal(t, curr.Domain, repoApp.Domain)
		require.Equal(t, curr.Slug, repoApp.Slug)
		require.Equal(t, curr.BaseURL, repoApp.BaseURL)
		require.Equal(t, curr.ClientID, repoApp.ClientID)
		require.Equal(t, curr.ClientSecret, repoApp.ClientSecret)
		require.Equal(t, curr.PrivateKey, repoApp.PrivateKey)
		require.Equal(t, curr.Logo, repoApp.Logo)
		require.NotZero(t, curr.CreatedAt)
		require.NotZero(t, curr.UpdatedAt)
	})
}

func TestInstallGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	store := newTestStore(t)

	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		AppID:        1,
		Name:         "Test App",
		Slug:         "test-app",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	id, err := store.Create(ctx, app)
	require.NoError(t, err)

	installationID := 42

	ghai, err := store.Install(ctx, ghtypes.GitHubAppInstallation{
		AppID:            id,
		InstallationID:   installationID,
		URL:              "https://github.com/apps/test-app",
		AccountLogin:     "test-user",
		AccountURL:       "https://github.com/test-user",
		AccountAvatarURL: "https://github.com/test-user.jpg",
		AccountType:      "User",
	})
	require.NoError(t, err)
	require.Equal(t, id, ghai.AppID)
	require.Equal(t, installationID, ghai.InstallationID)
	require.Equal(t, "https://github.com/apps/test-app", ghai.URL)
	require.Equal(t, "test-user", ghai.AccountLogin)
	require.Equal(t, "https://github.com/test-user", ghai.AccountURL)
	require.Equal(t, "https://github.com/test-user.jpg", ghai.AccountAvatarURL)
	require.Equal(t, "User", ghai.AccountType)

	var fetchedID, fetchedInstallID int
	var createdAt time.Time
	query := sqlf.Sprintf(`SELECT app_id, installation_id, created_at FROM github_app_installs WHERE app_id=%s AND installation_id = %s`, id, installationID)
	err = store.QueryRow(ctx, query).Scan(
		&fetchedID,
		&fetchedInstallID,
		&createdAt,
	)
	require.NoError(t, err)
	require.NotZero(t, createdAt)

	// installing with the same ID results in an upsert
	ghai, err = store.Install(ctx, ghtypes.GitHubAppInstallation{
		AppID:            id,
		InstallationID:   installationID,
		URL:              "https://github.com/apps/test-app",
		AccountLogin:     "test-user",
		AccountURL:       "https://github.com/test-user",
		AccountAvatarURL: "https://github.com/test-user-new.jpg",
		AccountType:      "User",
	})
	require.NoError(t, err)
	require.Equal(t, id, ghai.AppID)
	require.Equal(t, installationID, ghai.InstallationID)
	require.Equal(t, "https://github.com/apps/test-app", ghai.URL)
	require.Equal(t, "test-user", ghai.AccountLogin)
	require.Equal(t, "https://github.com/test-user", ghai.AccountURL)
	require.Equal(t, "https://github.com/test-user-new.jpg", ghai.AccountAvatarURL)
	require.Equal(t, "User", ghai.AccountType)

	var createdAt2 time.Time
	err = store.QueryRow(ctx, query).Scan(
		&fetchedID,
		&fetchedInstallID,
		&createdAt2,
	)
	require.NoError(t, err)
	require.Equal(t, createdAt, createdAt2)
}

func TestGetInstallationsForGitHubApp(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	store := newTestStore(t)
	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app-1",
		BaseURL:      "https://github.com/",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	appID, err := store.Create(ctx, app)
	require.NoError(t, err)

	installationIDs := []int{1, 2, 3}
	for _, installationID := range installationIDs {
		_, err := store.Install(ctx, ghtypes.GitHubAppInstallation{
			AppID:          appID,
			InstallationID: installationID,
			AccountLogin:   fmt.Sprintf("test-user-%d", installationID),
		})
		require.NoError(t, err)
	}

	installations, err := store.GetInstallations(ctx, appID)
	require.NoError(t, err)

	require.Len(t, installations, 3, "expected 3 installations, got %d", len(installations))

	for _, installation := range installations {
		require.Equal(t, appID, installation.AppID, "expected AppID %d, got %d", appID, installation.AppID)

		found := false
		for _, installationID := range installationIDs {
			if installation.InstallationID == installationID {
				found = true
				break
			}
		}

		require.True(t, found, "installation with ID %d not found", installation.InstallationID)
	}
}

func TestBulkRemoveGitHubAppInstallations(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	store := newTestStore(t)
	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app-1",
		BaseURL:      "https://github.com/",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	appID, err := store.Create(ctx, app)
	require.NoError(t, err)

	installationIDs := []int{1, 2, 3}
	for _, installationID := range installationIDs {
		_, err := store.Install(ctx, ghtypes.GitHubAppInstallation{
			AppID:          appID,
			InstallationID: installationID,
			AccountLogin:   fmt.Sprintf("test-user-%d", installationID),
		})
		require.NoError(t, err)
	}

	installations, err := store.GetInstallations(ctx, appID)
	require.NoError(t, err)

	require.Len(t, installations, 3, "expected 3 installations, got %d", len(installations))

	err = store.BulkRemoveInstallations(ctx, appID, installationIDs)
	require.NoError(t, err)

	installations, err = store.GetInstallations(ctx, appID)
	require.NoError(t, err)

	require.Len(t, installations, 0, "expected 0 installations, got %d", len(installations))
}

type mockGitHubClient struct {
	mock.Mock
}

func (m *mockGitHubClient) GetAppInstallations(ctx context.Context, page int) ([]*github.Installation, bool, error) {
	args := m.Called(ctx, page)
	if args.Get(0) != nil {
		return args.Get(0).([]*github.Installation), args.Get(1).(bool), args.Error(2)
	}
	return nil, false, args.Error(1)
}

func TestSyncInstallations(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	logger := logtest.Scoped(t)
	store := newTestStore(t)

	tcs := []struct {
		name               string
		githubClient       *mockGitHubClient
		expectedInstallIDs []int
		app                ghtypes.GitHubApp
		expectedErr        error
	}{
		{
			name: "no installations",
			githubClient: func() *mockGitHubClient {
				client := &mockGitHubClient{}
				client.On("GetAppInstallations", ctx, 1).Return([]*github.Installation{}, false, nil)
				return client
			}(),
			expectedInstallIDs: []int{},
			app: ghtypes.GitHubApp{
				AppID:      1,
				Name:       "Test App With No Installs",
				BaseURL:    "https://example.com",
				PrivateKey: "private-key",
			},
		},
		{
			name: "one installation",
			githubClient: func() *mockGitHubClient {
				client := &mockGitHubClient{}
				client.On("GetAppInstallations", ctx, 1).Return([]*github.Installation{
					{ID: github.Int64(1)},
				}, false, nil)
				return client
			}(),
			expectedInstallIDs: []int{1},
			app: ghtypes.GitHubApp{
				AppID:      2,
				Name:       "Test App With One Install",
				BaseURL:    "https://example.com",
				PrivateKey: "private-key",
			},
		},
		{
			name: "multiple installations",
			githubClient: func() *mockGitHubClient {
				client := &mockGitHubClient{}
				client.On("GetAppInstallations", ctx, 1).Return([]*github.Installation{
					{ID: github.Int64(2)},
					{ID: github.Int64(3)},
					{ID: github.Int64(4)},
				}, false, nil)
				return client
			}(),
			expectedInstallIDs: []int{2, 3, 4},
			app: ghtypes.GitHubApp{
				AppID:      3,
				Name:       "Test App With Multiple Installs",
				BaseURL:    "https://example.com",
				PrivateKey: "private-key",
			},
		},
		{
			name: "paged installations",
			githubClient: func() *mockGitHubClient {
				client := &mockGitHubClient{}
				client.On("GetAppInstallations", ctx, 1).Return([]*github.Installation{
					{ID: github.Int64(1)},
					{ID: github.Int64(2)},
				}, true, nil)
				client.On("GetAppInstallations", ctx, 2).Return([]*github.Installation{
					{ID: github.Int64(3)},
					{ID: github.Int64(4)},
				}, false, nil)
				return client
			}(),
			expectedInstallIDs: []int{1, 2, 3, 4},
			app: ghtypes.GitHubApp{
				AppID:      4,
				Name:       "Test App With Paged Installs",
				BaseURL:    "https://example.com",
				PrivateKey: "private-key",
			},
		},
		{
			name: "deleted github app",
			githubClient: func() *mockGitHubClient {
				client := &mockGitHubClient{}
				client.On("GetAppInstallations", ctx, 1).Return([]*github.Installation{}, false, errors.New("request to https://ghe.sgdev.org/api/v3/app/installations?page=1 returned status 404: Integration not found"))
				return client
			}(),
			expectedInstallIDs: []int{},
			expectedErr: &gh.APIError{
				URL:     "https://ghe.sgdev.org/api/v3/app/installations?page=1",
				Code:    http.StatusNotFound,
				Message: "request to https://ghe.sgdev.org/api/v3/app/installations?page=1 returned status 404: Integration not found",
			},
			app: ghtypes.GitHubApp{
				AppID:      5,
				Name:       "Test Deleted GitHub App",
				BaseURL:    "https://example.com",
				PrivateKey: "private-key",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dbAppID, err := store.Create(ctx, &tc.app)
			require.NoError(t, err)

			// For the app with multiple installations, we start with a couple to also
			// test updating and deleting
			if tc.name == "multiple installations" {
				_, err := store.Install(ctx, ghtypes.GitHubAppInstallation{
					AppID:          dbAppID,
					InstallationID: 2,
				})
				require.NoError(t, err)
				_, err = store.Install(ctx, ghtypes.GitHubAppInstallation{
					AppID:          dbAppID,
					InstallationID: 9999,
				})
				require.NoError(t, err)
			}

			app := ghtypes.GitHubApp{
				ID:         dbAppID,
				AppID:      tc.app.AppID,
				Name:       tc.app.Name,
				BaseURL:    tc.app.BaseURL,
				PrivateKey: tc.app.PrivateKey,
			}

			errs := store.SyncInstallations(ctx, app, logger, tc.githubClient)

			if tc.expectedErr != nil {
				var e *gh.APIError
				require.ErrorAs(t, tc.expectedErr, &e)
			} else {
				require.NoError(t, errs)
			}

			installations, err := store.GetInstallations(ctx, tc.app.AppID)
			require.NoError(t, err)

			require.Len(t, installations, len(tc.expectedInstallIDs), "expected %d installations, got %d", len(tc.expectedInstallIDs), len(installations))

			for _, expectedInstallID := range tc.expectedInstallIDs {
				found := false
				for _, installation := range installations {
					if installation.InstallationID == expectedInstallID {
						found = true
						break
					}
				}
				require.True(t, found, "expected to find installation with ID %d", expectedInstallID)
			}
		})
	}
}

func TestTrailingSlashesInBaseURL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	logger := logtest.Scoped(t)
	store := &gitHubAppsStore{Store: basestore.NewWithHandle(basestore.NewHandleWithDB(logger, dbtest.NewDB(t), sql.TxOptions{}))}
	ctx := context.Background()

	app := &ghtypes.GitHubApp{
		AppID:        1234,
		Name:         "Test App 1",
		Domain:       "repos",
		Slug:         "test-app-1",
		BaseURL:      "https://github.com",
		ClientID:     "abc123",
		ClientSecret: "secret",
		PrivateKey:   "private-key",
		Logo:         "logo.png",
	}

	id, err := store.Create(ctx, app)
	require.NoError(t, err)

	fetched, err := store.GetByAppID(ctx, 1234, "https://github.com")
	require.NoError(t, err)
	require.Equal(t, app.AppID, fetched.AppID)

	fetched, err = store.GetByAppID(ctx, 1234, "https://github.com/")
	require.NoError(t, err)
	require.Equal(t, app.AppID, fetched.AppID)

	fetched, err = store.GetByAppID(ctx, 1234, "https://github.com////")
	require.NoError(t, err)
	require.Equal(t, app.AppID, fetched.AppID)

	// works the other way around as well
	app.BaseURL = "https://github.com///"
	_, err = store.Update(ctx, id, app)
	require.NoError(t, err)

	fetched, err = store.GetByAppID(ctx, 1234, "https://github.com")
	require.NoError(t, err)
	require.Equal(t, app.AppID, fetched.AppID)
}
