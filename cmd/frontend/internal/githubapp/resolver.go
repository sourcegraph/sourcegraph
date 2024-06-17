package githubapp

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/repos/webhooks/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	ghauth "github.com/sourcegraph/sourcegraph/internal/github_apps/auth"
	"github.com/sourcegraph/sourcegraph/internal/github_apps/types"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	itypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewResolver returns a new Resolver that uses the given database
func NewResolver(logger log.Logger, db database.DB) graphqlbackend.GitHubAppsResolver {
	return &resolver{logger: logger, db: db}
}

type resolver struct {
	logger log.Logger
	db     database.DB
}

const gitHubAppIDKind = "GitHubApp"

// MarshalGitHubAppID converts a GitHub App ID (database ID) to a GraphQL ID.
func MarshalGitHubAppID(id int64) graphql.ID {
	return relay.MarshalID(gitHubAppIDKind, id)
}

// UnmarshalGitHubAppID converts a GitHub App GraphQL ID into a database ID.
func UnmarshalGitHubAppID(id graphql.ID) (gitHubAppID int64, err error) {
	if kind := relay.UnmarshalKind(id); kind != gitHubAppIDKind {
		err = errors.Errorf("expected graph ID to have kind %q; got %q", gitHubAppIDKind, kind)
		return
	}

	err = relay.UnmarshalSpec(id, &gitHubAppID)
	return
}

// DeleteGitHubApp deletes a GitHub App along with all of its associated
// code host connections and auth providers.
func (r *resolver) DeleteGitHubApp(ctx context.Context, args *graphqlbackend.DeleteGitHubAppArgs) (*graphqlbackend.EmptyResponse, error) {
	// 🚨 SECURITY: Only site admins can delete GitHub Apps.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	appID, err := UnmarshalGitHubAppID(args.GitHubApp)
	if err != nil {
		return nil, err
	}

	if err := r.db.GitHubApps().Delete(ctx, int(appID)); err != nil {
		return nil, err
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *resolver) RefreshGitHubApp(ctx context.Context, args *graphqlbackend.RefreshGitHubAppArgs) (*graphqlbackend.EmptyResponse, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	app, err := r.gitHubAppByID(ctx, args.GitHubApp)
	if err != nil {
		return nil, err
	}

	err = app.syncInstallationsWithError(ctx)
	return nil, err
}

func (r *resolver) GitHubApps(ctx context.Context, args *graphqlbackend.GitHubAppsArgs) (graphqlbackend.GitHubAppConnectionResolver, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	domain, err := parseDomain(args.Domain)
	if err != nil {
		return nil, err
	}
	apps, err := r.db.GitHubApps().List(ctx, domain)
	if err != nil {
		return nil, err
	}

	resolvers := make([]graphqlbackend.GitHubAppResolver, len(apps))
	for i := range apps {
		resolvers[i] = NewGitHubAppResolver(r.db, apps[i], r.logger)
	}

	gitHubAppConnection := &gitHubAppConnectionResolver{
		resolvers:  resolvers,
		totalCount: len(resolvers),
	}

	return gitHubAppConnection, nil
}

func parseDomain(s *string) (*itypes.GitHubAppDomain, error) {
	if s == nil {
		return nil, nil
	}
	switch strings.ToUpper(*s) {
	case "REPOS":
		domain := itypes.ReposGitHubAppDomain
		return &domain, nil
	case "BATCHES":
		domain := itypes.BatchesGitHubAppDomain
		return &domain, nil
	default:
		return nil, errors.Errorf("unknown domain %q", *s)
	}
}

func parseKind(s *string) (*itypes.GitHubAppKind, error) {
	if s == nil {
		return nil, nil
	}
	switch strings.ToUpper(*s) {
	case "USER_CREDENTIAL":
		kind := itypes.UserCredentialGitHubAppKind
		return &kind, nil
	case "COMMIT_SIGNING":
		kind := itypes.CommitSigningGitHubAppKind
		return &kind, nil
	case "REPO_SYNC":
		kind := itypes.RepoSyncGitHubAppKind
		return &kind, nil
	default:
		return nil, errors.Errorf("unknown kind %q", *s)
	}
}

func (r *resolver) GitHubApp(ctx context.Context, args *graphqlbackend.GitHubAppArgs) (graphqlbackend.GitHubAppResolver, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	return r.gitHubAppByID(ctx, args.ID)
}

func (r *resolver) GitHubAppByAppID(ctx context.Context, args *graphqlbackend.GitHubAppByAppIDArgs) (graphqlbackend.GitHubAppResolver, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	return r.gitHubAppByAppID(ctx, int(args.AppID), args.BaseURL)
}

func (r *resolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		gitHubAppIDKind: func(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
			return r.gitHubAppByID(ctx, id)
		},
	}
}

func (r *resolver) gitHubAppByID(ctx context.Context, id graphql.ID) (*gitHubAppResolver, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	gitHubAppID, err := UnmarshalGitHubAppID(id)
	if err != nil {
		return nil, err
	}
	app, err := r.db.GitHubApps().GetByID(ctx, int(gitHubAppID))
	if err != nil {
		return nil, err
	}

	return &gitHubAppResolver{
		app:    app,
		db:     r.db,
		logger: r.logger,
	}, nil
}

func (r *resolver) gitHubAppByAppID(ctx context.Context, appID int, baseURL string) (*gitHubAppResolver, error) {
	// 🚨 SECURITY: Check whether user is site-admin
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	app, err := r.db.GitHubApps().GetByAppID(ctx, appID, baseURL)
	if err != nil {
		return nil, err
	}

	return &gitHubAppResolver{
		app:    app,
		db:     r.db,
		logger: r.logger,
	}, nil
}

// NewGitHubAppResolver creates a new GitHubAppResolver from a GitHubApp.
func NewGitHubAppResolver(db database.DB, app *types.GitHubApp, logger log.Logger) *gitHubAppResolver {
	return &gitHubAppResolver{app: app, db: db, logger: logger}
}

type gitHubAppConnectionResolver struct {
	resolvers  []graphqlbackend.GitHubAppResolver
	totalCount int
}

func (r *gitHubAppConnectionResolver) Nodes(ctx context.Context) []graphqlbackend.GitHubAppResolver {
	return r.resolvers
}

func (r *gitHubAppConnectionResolver) TotalCount(ctx context.Context) int32 {
	return int32(r.totalCount)
}

// gitHubAppResolver is a GraphQL node resolver for GitHubApps.
type gitHubAppResolver struct {
	logger log.Logger
	app    *types.GitHubApp
	db     database.DB

	once          sync.Once
	installations []graphqlbackend.GitHubAppInstallation
	err           error
}

func (r *gitHubAppResolver) ID() graphql.ID {
	return MarshalGitHubAppID(int64(r.app.ID))
}

func (r *gitHubAppResolver) AppID() int32 {
	return int32(r.app.AppID)
}

func (r *gitHubAppResolver) Name() string {
	return r.app.Name
}

func (r *gitHubAppResolver) Domain() string {
	return r.app.Domain.ToGraphQL()
}

func (r *gitHubAppResolver) Slug() string {
	return r.app.Slug
}

func (r *gitHubAppResolver) BaseURL() string {
	return r.app.BaseURL
}

func (r *gitHubAppResolver) AppURL() string {
	return r.app.AppURL
}

func (r *gitHubAppResolver) ClientID() string {
	return r.app.ClientID
}

func (r *gitHubAppResolver) ClientSecret() string {
	return r.app.ClientSecret
}

func (r *gitHubAppResolver) Logo() string {
	return r.app.Logo
}

func (r *gitHubAppResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.app.CreatedAt}
}

func (r *gitHubAppResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.app.UpdatedAt}
}

func (r *gitHubAppResolver) Installations(ctx context.Context) (installations []graphqlbackend.GitHubAppInstallation, err error) {
	installations, err = r.compute(ctx)
	if err != nil {
		return []graphqlbackend.GitHubAppInstallation{}, err
	}
	return installations, nil
}

func (r *gitHubAppResolver) Webhook(ctx context.Context) graphqlbackend.WebhookResolver {
	if r.app.WebhookID == nil {
		return nil
	}
	hook, err := r.db.Webhooks(keyring.Default().WebhookKey).GetByID(ctx, int32(*r.app.WebhookID))
	if err != nil {
		return nil
	}
	return resolvers.NewWebhookResolver(r.db, hook)
}

func (r *gitHubAppResolver) compute(ctx context.Context) ([]graphqlbackend.GitHubAppInstallation, error) {
	r.once.Do(func() {
		installs, err := r.db.GitHubApps().GetInstallations(ctx, r.app.ID)
		if err != nil {
			r.err = err
			return
		}

		// We use this opportunity to sync installations in our database. This is done in
		// a goroutine so that we don't block the request completion.
		go r.syncInstallations(ctx)

		extsvcs, err := r.db.ExternalServices().List(ctx, database.ExternalServicesListOptions{
			Kinds: []string{extsvc.KindGitHub},
		})
		if err != nil {
			r.err = err
			return
		}

		for _, install := range installs {
			var installationExtsvcs []*itypes.ExternalService
			for _, es := range extsvcs {
				parsed, err := extsvc.ParseEncryptableConfig(ctx, extsvc.KindGitHub, es.Config)
				if err != nil {
					continue
				}
				c := parsed.(*schema.GitHubConnection)
				if c.GitHubAppDetails == nil || c.GitHubAppDetails.AppID != r.app.AppID || c.Url != r.app.BaseURL || c.GitHubAppDetails.InstallationID != int(install.InstallationID) {
					continue
				}
				installationExtsvcs = append(installationExtsvcs, es)
			}

			r.installations = append(r.installations, graphqlbackend.GitHubAppInstallation{
				DB:         r.db,
				InstallID:  int32(install.InstallationID),
				InstallURL: install.URL,
				InstallAccount: graphqlbackend.GitHubAppInstallationAccount{
					AccountLogin:     install.AccountLogin,
					AccountAvatarURL: install.AccountAvatarURL,
					AccountURL:       install.AccountURL,
					AccountType:      install.AccountType,
				},
				InstallExternalServices: installationExtsvcs,
			})
		}
	})
	return r.installations, r.err
}

// syncInstallations syncs the GitHub App Installations in our database with those
// found on GitHub.
func (r *gitHubAppResolver) syncInstallationsWithError(ctx context.Context) error {
	r.logger.Info("Performing opportunistic GitHub App Installations sync", log.String("app_name", r.app.Name))

	auther, err := ghauth.NewGitHubAppAuthenticator(int(r.AppID()), []byte(r.app.PrivateKey))
	if err != nil {
		r.logger.Warn("Error creating GitHub App authenticator", log.Error(err))
		return err
	}

	baseURL, err := url.Parse(r.app.BaseURL)
	if err != nil {
		r.logger.Warn("Error parsing GitHub App base URL", log.Error(err))
		return err
	}
	apiURL, _ := github.APIRoot(baseURL)

	client := github.NewV3Client(r.logger, "", apiURL, auther, nil)

	errs := r.db.GitHubApps().SyncInstallations(ctx, *r.app, r.logger, client)
	if errs != nil && len(errs.Errors()) > 0 {
		r.logger.Warn("Error syncing GitHub App Installations", log.Error(errs))
		return errs
	}
	return nil
}

// This method only logs errors rather than assigning them to
// the resolver because they should not block the request from completing.
func (r *gitHubAppResolver) syncInstallations(ctx context.Context) {
	// We reuse the context that was passed from the request here so that
	// we retain all the other information in the context (like tracing
	// information, auth information, etc.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 1*time.Minute)
	defer cancel()
	if err := r.syncInstallationsWithError(ctx); err != nil {
		r.logger.Error("Error syncing GitHub App Installations", log.Error(err))
	}
}
