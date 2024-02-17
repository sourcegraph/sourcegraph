package worker

import (
	"context"
	"net/url"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/github_apps/auth"
	ghtypes "github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewGitHubInstallationWorker returns a goroutine.Handler that will backfill GitHub App
// installation information from the GitHub API into the database.
func NewGitHubInstallationWorker(db database.DB, logger log.Logger) goroutine.Handler {
	return &githubAppInstallationWorker{
		db:     db,
		logger: logger,
	}
}

type githubAppInstallationWorker struct {
	db     database.DB
	logger log.Logger
}

func (g *githubAppInstallationWorker) Handle(ctx context.Context) error {
	store := g.db.GitHubApps()
	apps, err := store.List(ctx, nil)
	if err != nil {
		g.logger.Error("Fetching GitHub Apps", log.Error(err))
		return errors.Wrap(err, "Fetching GitHub Apps")
	}

	var errs errors.MultiError
	for _, app := range apps {
		if app == nil || app.AppID == 0 {
			continue
		}

		g.logger.Info("GitHub App Installation backfill job", log.String("appName", app.Name), log.Int("id", app.ID))

		client, err := newGithubClient(app, g.logger)
		if err != nil {
			g.logger.Error("Creating GitHub client", log.Error(err), log.String("appName", app.Name), log.Int("id", app.ID))
			errs = errors.Append(errs, err)
			continue
		}

		sErrs := store.SyncInstallations(ctx, *app, g.logger, client)
		if sErrs != nil && len(sErrs.Errors()) > 0 {
			errs = errors.Append(errs, sErrs.Errors()...)
		}
	}

	return errs
}

var MockGitHubClient func(app *ghtypes.GitHubApp, logger log.Logger) (ghtypes.GitHubAppClient, error)

func newGithubClient(app *ghtypes.GitHubApp, logger log.Logger) (ghtypes.GitHubAppClient, error) {
	if MockGitHubClient != nil {
		return MockGitHubClient(app, logger)
	}
	auther, err := auth.NewGitHubAppAuthenticator(app.AppID, []byte(app.PrivateKey))
	if err != nil {
		return nil, err
	}

	baseURL, err := url.Parse(app.BaseURL)
	if err != nil {
		return nil, err
	}

	apiURL, _ := github.APIRoot(baseURL)
	return github.NewV3Client(logger, "", apiURL, auther, nil)
}
