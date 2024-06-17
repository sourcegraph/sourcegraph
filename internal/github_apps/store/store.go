package store

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v55/github"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	encryption "github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	gh "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghtypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ErrNoGitHubAppFound struct {
	Criteria string
}

func (e ErrNoGitHubAppFound) Error() string {
	return fmt.Sprintf("no app exists matching criteria: '%s'", e.Criteria)
}

// GitHubAppsStore handles storing and retrieving GitHub Apps from the database.
type GitHubAppsStore interface {
	// Create inserts a new GitHub App into the database.
	Create(ctx context.Context, app *ghtypes.GitHubApp) (int, error)

	// Delete removes a GitHub App from the database by ID.
	Delete(ctx context.Context, id int) error

	// Update updates a GitHub App in the database and returns the updated struct.
	Update(ctx context.Context, id int, app *ghtypes.GitHubApp) (*ghtypes.GitHubApp, error)

	// Install creates a new GitHub App installation in the database.
	Install(ctx context.Context, ghai ghtypes.GitHubAppInstallation) (*ghtypes.GitHubAppInstallation, error)

	// BulkRemoveInstallations revokes multiple GitHub App installation IDs from the database
	// for the GitHub App with the given App ID.
	BulkRemoveInstallations(ctx context.Context, appID int, installationIDs []int) error

	// GetInstallations retrieves all installations for the GitHub App with the given App ID.
	GetInstallations(ctx context.Context, appID int) ([]*ghtypes.GitHubAppInstallation, error)

	// SyncInstallations retrieves all installations for the GitHub App with the given ID
	// from GitHub and updates the database to match.
	SyncInstallations(ctx context.Context, app ghtypes.GitHubApp, logger log.Logger, client ghtypes.GitHubAppClient) (errs errors.MultiError)

	// GetInstall retrieves the GitHub App installation ID from the database for the
	// GitHub App with the provided appID and account name, if one can be found.
	GetInstallID(ctx context.Context, appID int, account string) (int, error)

	// GetByID retrieves a GitHub App from the database by ID.
	GetByID(ctx context.Context, id int) (*ghtypes.GitHubApp, error)

	// GetByAppID retrieves a GitHub App from the database by appID and base url
	GetByAppID(ctx context.Context, appID int, baseURL string) (*ghtypes.GitHubApp, error)

	// GetBySlug retrieves a GitHub App from the database by slug and base url
	GetBySlug(ctx context.Context, slug string, baseURL string) (*ghtypes.GitHubApp, error)

	// GetByDomainAndKind retrieves a GitHub App from the database by domain and base url
	GetByDomainAndKind(ctx context.Context, domain itypes.GitHubAppDomain, baseURL string, kind itypes.GitHubAppKind) (*ghtypes.GitHubApp, error)

	// WithEncryptionKey sets encryption key on store. Returns a new GitHubAppsStore
	WithEncryptionKey(key encryption.Key) GitHubAppsStore

	// Lists all GitHub Apps in the store and optionally filters by domain
	List(ctx context.Context, domain *itypes.GitHubAppDomain) ([]*ghtypes.GitHubApp, error)
}

// gitHubAppStore handles storing and retrieving GitHub Apps from the database.
type gitHubAppsStore struct {
	*basestore.Store

	key encryption.Key
}

func GitHubAppsWith(other *basestore.Store) GitHubAppsStore {
	return &gitHubAppsStore{
		Store: basestore.NewWithHandle(other.Handle()),
	}
}

// WithEncryptionKey sets encryption key on store. Returns a new GitHubAppsStore
func (s *gitHubAppsStore) WithEncryptionKey(key encryption.Key) GitHubAppsStore {
	return &gitHubAppsStore{Store: s.Store, key: key}
}

func (s *gitHubAppsStore) getEncryptionKey() encryption.Key {
	if s.key != nil {
		return s.key
	}
	return keyring.Default().GitHubAppKey
}

// Create inserts a new GitHub App into the database. The default domain for the App is "repos".
func (s *gitHubAppsStore) Create(ctx context.Context, app *ghtypes.GitHubApp) (int, error) {
	key := s.getEncryptionKey()
	clientSecret, _, err := encryption.MaybeEncrypt(ctx, key, app.ClientSecret)
	if err != nil {
		return -1, err
	}
	privateKey, keyID, err := encryption.MaybeEncrypt(ctx, key, app.PrivateKey)
	if err != nil {
		return -1, err
	}

	baseURL, err := url.Parse(app.BaseURL)
	if err != nil {
		return -1, errors.New(fmt.Sprintf("unable to parse base URL: %s", baseURL.String()))
	}
	baseURL = extsvc.NormalizeBaseURL(baseURL)
	domain := app.Domain
	if domain == "" {
		domain = itypes.ReposGitHubAppDomain
	}

	kind := ghtypes.NormalizeGitHubAppKind(app)

	// We enforce that GitHub Apps created in the "batches" domain and are used for commit signing are for unique instance URLs.
	if domain == itypes.BatchesGitHubAppDomain && kind == itypes.CommitSigningGitHubAppKind {
		existingGHApp, err := s.GetByDomainAndKind(ctx, domain, baseURL.String(), kind)
		// An error is expected if no existing app was found, but we double check that
		// we didn't get a different, unrelated error
		if _, ok := err.(ErrNoGitHubAppFound); !ok {
			return -1, errors.Wrapf(err, "checking for existing batches app (kind: %s)", kind)
		}
		if existingGHApp != nil {
			return -1, errors.Newf("GitHub App already exists for this GitHub instance in the batches domain for %s", kind)
		}
	}

	query := sqlf.Sprintf(`INSERT INTO
	    github_apps (app_id, name, domain, slug, base_url, app_url, client_id, client_secret, private_key, encryption_key_id, logo, kind)
    	VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
		RETURNING id`,
		app.AppID, app.Name, domain, app.Slug, baseURL.String(), app.AppURL, app.ClientID, clientSecret, privateKey, keyID, app.Logo, kind)
	id, _, err := basestore.ScanFirstInt(s.Query(ctx, query))
	return id, err
}

// Delete removes a GitHub App from the database by ID.
func (s *gitHubAppsStore) Delete(ctx context.Context, id int) error {
	query := sqlf.Sprintf(`DELETE FROM github_apps WHERE id = %s`, id)
	return s.Exec(ctx, query)
}

func scanGitHubApp(s dbutil.Scanner) (*ghtypes.GitHubApp, error) {
	var app ghtypes.GitHubApp

	err := s.Scan(
		&app.ID,
		&app.AppID,
		&app.Name,
		&app.Domain,
		&app.Slug,
		&app.BaseURL,
		&app.AppURL,
		&app.ClientID,
		&app.ClientSecret,
		&app.WebhookID,
		&app.PrivateKey,
		&app.EncryptionKey,
		&app.Logo,
		&app.Kind,
		&app.CreatedAt,
		&app.UpdatedAt)
	return &app, err
}

// githubAppInstallColumns are used by the github app install related Store methods to
// insert, update and query.
var githubAppInstallColumns = []*sqlf.Query{
	sqlf.Sprintf("github_app_installs.id"),
	sqlf.Sprintf("github_app_installs.app_id"),
	sqlf.Sprintf("github_app_installs.installation_id"),
	sqlf.Sprintf("github_app_installs.url"),
	sqlf.Sprintf("github_app_installs.account_login"),
	sqlf.Sprintf("github_app_installs.account_avatar_url"),
	sqlf.Sprintf("github_app_installs.account_url"),
	sqlf.Sprintf("github_app_installs.account_type"),
	sqlf.Sprintf("github_app_installs.created_at"),
	sqlf.Sprintf("github_app_installs.updated_at"),
}

// githubAppInstallInsertColumns is the list of github app install columns that are modified in
// Install.
var githubAppInstallInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("app_id"),
	sqlf.Sprintf("installation_id"),
	sqlf.Sprintf("url"),
	sqlf.Sprintf("account_login"),
	sqlf.Sprintf("account_avatar_url"),
	sqlf.Sprintf("account_url"),
	sqlf.Sprintf("account_type"),
}

func scanGitHubAppInstallation(s dbutil.Scanner) (*ghtypes.GitHubAppInstallation, error) {
	var install ghtypes.GitHubAppInstallation

	err := s.Scan(
		&install.ID,
		&install.AppID,
		&install.InstallationID,
		&dbutil.NullString{S: &install.URL},
		&dbutil.NullString{S: &install.AccountLogin},
		&dbutil.NullString{S: &install.AccountAvatarURL},
		&dbutil.NullString{S: &install.AccountURL},
		&dbutil.NullString{S: &install.AccountType},
		&install.CreatedAt,
		&install.UpdatedAt,
	)
	return &install, err
}

var (
	scanGitHubApps     = basestore.NewSliceScanner(scanGitHubApp)
	scanFirstGitHubApp = basestore.NewFirstScanner(scanGitHubApp)

	scanGitHubAppInstallations     = basestore.NewSliceScanner(scanGitHubAppInstallation)
	scanFirstGitHubAppInstallation = basestore.NewFirstScanner(scanGitHubAppInstallation)
)

func (s *gitHubAppsStore) decrypt(ctx context.Context, apps ...*ghtypes.GitHubApp) ([]*ghtypes.GitHubApp, error) {
	key := s.getEncryptionKey()

	for _, app := range apps {
		cs, err := encryption.MaybeDecrypt(ctx, key, app.ClientSecret, app.EncryptionKey)
		if err != nil {
			return nil, err
		}
		app.ClientSecret = cs
		pk, err := encryption.MaybeDecrypt(ctx, key, app.PrivateKey, app.EncryptionKey)
		if err != nil {
			return nil, err
		}
		app.PrivateKey = pk
	}

	return apps, nil
}

// Update updates a GitHub App in the database and returns the updated struct.
func (s *gitHubAppsStore) Update(ctx context.Context, id int, app *ghtypes.GitHubApp) (*ghtypes.GitHubApp, error) {
	key := s.getEncryptionKey()
	clientSecret, _, err := encryption.MaybeEncrypt(ctx, key, app.ClientSecret)
	if err != nil {
		return nil, err
	}
	privateKey, keyID, err := encryption.MaybeEncrypt(ctx, key, app.PrivateKey)
	if err != nil {
		return nil, err
	}

	kind := ghtypes.NormalizeGitHubAppKind(app)

	query := sqlf.Sprintf(`UPDATE github_apps
             SET app_id = %s, name = %s, domain = %s, slug = %s, base_url = %s, app_url = %s, client_id = %s, client_secret = %s, webhook_id = %d, private_key = %s, encryption_key_id = %s, logo = %s, kind = %s, updated_at = NOW()
             WHERE id = %s
			 RETURNING id, app_id, name, domain, slug, base_url, app_url, client_id, client_secret, webhook_id, private_key, encryption_key_id, logo, kind, created_at, updated_at`,
		app.AppID, app.Name, app.Domain, app.Slug, app.BaseURL, app.AppURL, app.ClientID, clientSecret, app.WebhookID, privateKey, keyID, app.Logo, kind, id)
	app, ok, err := scanFirstGitHubApp(s.Query(ctx, query))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.Newf("cannot update app with id: %d because no such app exists", id)
	}
	apps, err := s.decrypt(ctx, app)
	if err != nil {
		return nil, err
	}
	return apps[0], nil
}

// Install creates a new GitHub App installation in the database.
func (s *gitHubAppsStore) Install(ctx context.Context, ghai ghtypes.GitHubAppInstallation) (*ghtypes.GitHubAppInstallation, error) {
	query := sqlf.Sprintf(`
		INSERT INTO github_app_installs (%s)
    	VALUES (%s, %s, %s, %s, %s, %s, %s)
		ON CONFLICT (app_id, installation_id)
		DO UPDATE SET
		(%s) = (%s, %s, %s, %s, %s, %s, %s)
		WHERE github_app_installs.app_id = excluded.app_id AND github_app_installs.installation_id = excluded.installation_id
		RETURNING %s`,
		sqlf.Join(githubAppInstallInsertColumns, ", "),
		ghai.AppID,
		ghai.InstallationID,
		ghai.URL,
		ghai.AccountLogin,
		ghai.AccountAvatarURL,
		ghai.AccountURL,
		ghai.AccountType,
		sqlf.Join(githubAppInstallInsertColumns, ", "),
		ghai.AppID,
		ghai.InstallationID,
		ghai.URL,
		ghai.AccountLogin,
		ghai.AccountAvatarURL,
		ghai.AccountURL,
		ghai.AccountType,
		sqlf.Join(githubAppInstallColumns, ", "),
	)
	in, _, err := scanFirstGitHubAppInstallation(s.Query(ctx, query))
	return in, err
}

func (s *gitHubAppsStore) GetInstallID(ctx context.Context, appID int, account string) (int, error) {
	query := sqlf.Sprintf(`
		SELECT installation_id
		FROM github_app_installs
		JOIN github_apps ON github_app_installs.app_id = github_apps.id
		WHERE github_apps.app_id = %s
		AND LOWER(github_app_installs.account_login) = %s
		-- We get the most recent installation, in case it's recently been removed and readded and the old ones aren't cleaned up yet.
		ORDER BY github_app_installs.id DESC LIMIT 1
		`, appID, strings.ToLower(account))
	installID, _, err := basestore.ScanFirstInt(s.Query(ctx, query))
	return installID, err
}

func (s *gitHubAppsStore) get(ctx context.Context, where *sqlf.Query) (*ghtypes.GitHubApp, error) {
	selectQuery := `SELECT
		id,
		app_id,
		name,
		domain,
		slug,
		base_url,
		app_url,
		client_id,
		client_secret,
		webhook_id,
		private_key,
		encryption_key_id,
		logo,
		kind,
		created_at,
		updated_at
	FROM github_apps
	WHERE %s`

	query := sqlf.Sprintf(selectQuery, where)
	app, ok, err := scanFirstGitHubApp(s.Query(ctx, query))
	if err != nil {
		return nil, err
	}
	if !ok {
		swhere := where.Query(sqlf.PostgresBindVar)
		for i, arg := range where.Args() {
			swhere = strings.Replace(swhere, fmt.Sprintf("$%d", i+1), fmt.Sprintf("%v", arg), 1)
		}
		return nil, ErrNoGitHubAppFound{Criteria: swhere}
	}

	apps, err := s.decrypt(ctx, app)
	if err != nil {
		return nil, err
	}
	return apps[0], nil
}

func (s *gitHubAppsStore) list(ctx context.Context, where *sqlf.Query) ([]*ghtypes.GitHubApp, error) {
	selectQuery := `SELECT
		id,
		app_id,
		name,
		domain,
		slug,
		base_url,
		app_url,
		client_id,
		client_secret,
		webhook_id,
		private_key,
		encryption_key_id,
		logo,
		kind,
		created_at,
		updated_at
	FROM github_apps
	WHERE %s`

	query := sqlf.Sprintf(selectQuery, where)
	apps, err := scanGitHubApps(s.Query(ctx, query))
	if err != nil {
		return nil, err
	}

	return s.decrypt(ctx, apps...)
}

func baseURLWhere(baseURL string) *sqlf.Query {
	return sqlf.Sprintf(`trim(trailing '/' from base_url) = %s`, strings.TrimRight(baseURL, "/"))
}

// GetByID retrieves a GitHub App from the database by ID.
func (s *gitHubAppsStore) GetByID(ctx context.Context, id int) (*ghtypes.GitHubApp, error) {
	return s.get(ctx, sqlf.Sprintf(`id = %s`, id))
}

// GetByAppID retrieves a GitHub App from the database by appID and base url
func (s *gitHubAppsStore) GetByAppID(ctx context.Context, appID int, baseURL string) (*ghtypes.GitHubApp, error) {
	return s.get(ctx, sqlf.Sprintf(`app_id = %s AND %s`, appID, baseURLWhere(baseURL)))
}

// GetBySlug retrieves a GitHub App from the database by slug and base url
func (s *gitHubAppsStore) GetBySlug(ctx context.Context, slug string, baseURL string) (*ghtypes.GitHubApp, error) {
	return s.get(ctx, sqlf.Sprintf(`slug = %s AND %s`, slug, baseURLWhere(baseURL)))
}

// GetByDomain retrieves a GitHub App from the database by domain and base url
func (s *gitHubAppsStore) GetByDomainAndKind(ctx context.Context, domain itypes.GitHubAppDomain, baseURL string, kind itypes.GitHubAppKind) (*ghtypes.GitHubApp, error) {
	return s.get(ctx, sqlf.Sprintf(`domain = %s AND %s AND kind = %s`, domain, baseURLWhere(baseURL), kind))
}

// List lists all GitHub Apps in the store
func (s *gitHubAppsStore) List(ctx context.Context, domain *itypes.GitHubAppDomain) ([]*ghtypes.GitHubApp, error) {
	where := sqlf.Sprintf(`true`)
	if domain != nil {
		where = sqlf.Sprintf("domain = %s", *domain)
	}
	return s.list(ctx, where)
}

// GetInstallations retrieves all installations for the GitHub App with the given App ID.
func (s *gitHubAppsStore) GetInstallations(ctx context.Context, appID int) ([]*ghtypes.GitHubAppInstallation, error) {
	query := sqlf.Sprintf(
		`SELECT %s FROM github_app_installs WHERE app_id = %s`,
		sqlf.Join(githubAppInstallColumns, ", "),
		appID,
	)
	return scanGitHubAppInstallations(s.Query(ctx, query))
}

func (s *gitHubAppsStore) BulkRemoveInstallations(ctx context.Context, appID int, installationIDs []int) error {
	var pred []*sqlf.Query
	pred = append(pred, sqlf.Sprintf("app_id = %d", appID))

	var installIDQuery []*sqlf.Query
	for _, id := range installationIDs {
		installIDQuery = append(installIDQuery, sqlf.Sprintf("%d", id))
	}
	pred = append(pred, sqlf.Sprintf("installation_id IN (%s)", sqlf.Join(installIDQuery, ", ")))

	query := sqlf.Sprintf(`
		DELETE FROM github_app_installs
		WHERE %s
	`, sqlf.Join(pred, " AND "))
	return s.Exec(ctx, query)
}

func (s *gitHubAppsStore) SyncInstallations(ctx context.Context, app ghtypes.GitHubApp, logger log.Logger, client ghtypes.GitHubAppClient) (errs errors.MultiError) {
	dbInstallations, err := s.GetInstallations(ctx, app.ID)
	if err != nil {
		logger.Error("Fetching App Installations from database", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
		return errors.Append(errs, err)
	}

	getAllAppInstallations := func() (allInstallations []*github.Installation, err error) {
		hasNextPage := true
		for page := 1; hasNextPage; page++ {
			var installations []*github.Installation
			installations, hasNextPage, err = client.GetAppInstallations(ctx, page)
			if err != nil {
				return nil, err
			}
			allInstallations = append(allInstallations, installations...)
		}
		return allInstallations, nil
	}

	remoteInstallations, err := getAllAppInstallations()
	if err != nil {
		logger.Error("Fetching App Installations from GitHub", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
		errs = errors.Append(errs, err)

		// if the error from github is 404 then it means the GitHub app has been deleted. If not, we return
		// the error and preserve whatever installations we have in the database.
		var e *gh.APIError
		if errors.As(err, &e) && e.Code == http.StatusNotFound {
			var toBeDeleted []int
			for _, install := range dbInstallations {
				toBeDeleted = append(toBeDeleted, install.InstallationID)
			}
			if len(toBeDeleted) > 0 {
				logger.Info("Deleting GitHub App Installations", log.String("appName", app.Name), log.Ints("installationIDs", toBeDeleted))
				err = s.BulkRemoveInstallations(ctx, app.ID, toBeDeleted)
				if err != nil {
					logger.Error("Failed to remove GitHub App Installations", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
					return errors.Append(errs, err)
				}
			}
		}

		return errs
	}

	var dbInstallsMap = make(map[int]struct{}, len(dbInstallations))
	for _, in := range dbInstallations {
		dbInstallsMap[in.InstallationID] = struct{}{}
	}

	var toBeAdded []ghtypes.GitHubAppInstallation

	for _, install := range remoteInstallations {
		if install == nil || install.ID == nil {
			continue
		}
		// We add any installation that exists on GitHub regardless of whether or not it
		// already exists in our database, because we will upsert it to ensure that we
		// have the latest metadata for the installation.
		toBeAdded = append(toBeAdded, ghtypes.GitHubAppInstallation{
			InstallationID:   int(install.GetID()),
			AppID:            app.ID,
			URL:              install.GetHTMLURL(),
			AccountLogin:     install.Account.GetLogin(),
			AccountAvatarURL: install.Account.GetAvatarURL(),
			AccountURL:       install.Account.GetHTMLURL(),
			AccountType:      install.Account.GetType(),
		})
		_, exists := dbInstallsMap[int(install.GetID())]
		// If the installation already existed in the DB, we remove it from the map of
		// database installations so that we can determine later which installations need
		// to be removed from the database. Any installations that remain in the map after
		// this loop will be removed.
		if exists {
			delete(dbInstallsMap, int(install.GetID()))
		}
	}

	if len(toBeAdded) > 0 {
		for _, install := range toBeAdded {
			logger.Info("Upserting GitHub App Installation", log.String("appName", app.Name), log.Int("installationID", install.InstallationID))
			_, err = s.Install(ctx, install)
			if err != nil {
				logger.Error("Failed to save new GitHub App Installation", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
				errs = errors.Append(errs, err)
				continue
			}
		}
	}

	// If there are any installations left in the map, it means they were not present in
	// the remote installations, so we should remove them from the database.
	if len(dbInstallsMap) > 0 {
		var toBeDeleted []int
		for id := range dbInstallsMap {
			toBeDeleted = append(toBeDeleted, id)
		}
		logger.Info("Deleting GitHub App Installations", log.String("appName", app.Name), log.Ints("installationIDs", toBeDeleted))
		err = s.BulkRemoveInstallations(ctx, app.ID, toBeDeleted)
		if err != nil {
			logger.Error("Failed to remove GitHub App Installations", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
			return errors.Append(errs, err)
		}
	}

	return errs
}
