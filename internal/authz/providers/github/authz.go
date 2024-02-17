package github

import (
	"context"
	"fmt"
	"net/url"
	"time"

	atypes "github.com/sourcegraph/sourcegraph/internal/authz/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	eauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github/auth"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ExternalConnection is a composite object of a GITHUB kind external service and
// parsed connection information.
type ExternalConnection struct {
	*types.ExternalService
	*types.GitHubConnection
}

// NewAuthzProviders returns the set of GitHub authz providers derived from the connections.
//
// It also returns any simple validation problems with the config, separating these into "serious problems"
// and "warnings". "Serious problems" are those that should make Sourcegraph set authz.allowAccessByDefault
// to false. "Warnings" are all other validation problems.
//
// This constructor does not and should not directly check connectivity to external services - if
// desired, callers should use `(*Provider).ValidateConnection` directly to get warnings related
// to connection issues.
func NewAuthzProviders(
	ctx context.Context,
	db database.DB,
	conns []*ExternalConnection,
	authProviders []schema.AuthProviders,
	enableGithubInternalRepoVisibility bool,
) *atypes.ProviderInitResult {
	initResults := &atypes.ProviderInitResult{}
	// Auth providers (i.e. login mechanisms)
	githubAuthProviders := make(map[string]*schema.GitHubAuthProvider)
	for _, p := range authProviders {
		if p.Github != nil {
			var id string
			ghURL, err := url.Parse(p.Github.GetURL())
			if err != nil {
				// error reporting for this should happen elsewhere, for now just use what is given
				id = p.Github.GetURL()
			} else {
				// use codehost normalized URL as ID
				ch := extsvc.NewCodeHost(ghURL, p.Github.Type)
				id = ch.ServiceID
			}
			githubAuthProviders[id] = p.Github
		}
	}

	for _, c := range conns {
		// Initialize authz (permissions) provider.
		p, err := newAuthzProvider(ctx, db, c)
		if err != nil {
			initResults.InvalidConnections = append(initResults.InvalidConnections, extsvc.TypeGitHub)
			initResults.Problems = append(initResults.Problems, err.Error())
		}
		if p == nil {
			continue
		}

		// We want to make the feature flag available to the GitHub provider, but at the same time
		// also not use the global conf.SiteConfig which is discouraged and could cause race
		// conditions. As a result, we use a temporary hack by setting this on the provider for now.
		p.enableGithubInternalRepoVisibility = enableGithubInternalRepoVisibility

		authProvider, exists := githubAuthProviders[p.ServiceID()]
		if exists && p.groupsCache != nil && !authProvider.AllowGroupsPermissionsSync {
			// Groups permissions requires auth provider to request the correct scopes.
			initResults.Warnings = append(initResults.Warnings,
				fmt.Sprintf("GitHub config for %[1]s has `authorization.groupsCacheTTL` enabled, but "+
					"the authentication provider matching %[1]q does not have `allowGroupsPermissionsSync` enabled. "+
					"Update the [**site configuration**](/site-admin/configuration) in the appropriate entry "+
					"in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) to enable this.",
					p.ServiceID()))
			// Forcibly disable groups cache.
			p.groupsCache = nil
		}

		// Register this provider.
		initResults.Providers = append(initResults.Providers, p)
	}

	return initResults
}

// newAuthzProvider instantiates a provider, or returns nil if authorization is disabled.
// Errors returned are "serious problems".
func newAuthzProvider(
	ctx context.Context,
	db database.DB,
	c *ExternalConnection,
) (*Provider, error) {
	if c.Authorization == nil {
		return nil, nil
	}

	if errLicense := licensing.Check(licensing.FeatureACLs); errLicense != nil {
		return nil, errLicense
	}

	baseURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, errors.Errorf("could not parse URL for GitHub instance %q: %s", c.Url, err)
	}

	// Disable by default for now
	if c.Authorization.GroupsCacheTTL <= 0 {
		c.Authorization.GroupsCacheTTL = -1
	}

	var auther eauth.Authenticator
	if ghaDetails := c.GitHubConnection.GitHubAppDetails; ghaDetails != nil {
		auther, err = auth.FromConnection(ctx, c.GitHubConnection.GitHubConnection, db.GitHubApps(), keyring.Default().GitHubAppKey)
		if err != nil {
			return nil, err
		}
	} else {
		auther = &eauth.OAuthBearerToken{Token: c.Token}
	}

	ttl := time.Duration(c.Authorization.GroupsCacheTTL) * time.Hour
	return NewProvider(c.GitHubConnection.URN, ProviderOptions{
		GitHubURL:                   baseURL,
		BaseAuther:                  auther,
		GroupsCacheTTL:              ttl,
		DB:                          db,
		SyncInternalRepoPermissions: (c.Authorization != nil) && c.Authorization.SyncInternalRepoPermissions,
	})
}

// ValidateAuthz validates the authorization fields of the given GitHub external
// service config.
func ValidateAuthz(db database.DB, c *types.GitHubConnection) error {
	_, err := newAuthzProvider(context.Background(), db, &ExternalConnection{GitHubConnection: c})
	return err
}
