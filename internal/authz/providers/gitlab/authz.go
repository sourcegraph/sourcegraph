package gitlab

import (
	"net/url"

	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	atypes "github.com/sourcegraph/sourcegraph/internal/authz/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewAuthzProviders returns the set of GitLab authz providers derived from the connections.
//
// It also returns any simple validation problems with the config, separating these into "serious problems"
// and "warnings". "Serious problems" are those that should make Sourcegraph set authz.allowAccessByDefault
// to false. "Warnings" are all other validation problems.
//
// This constructor does not and should not directly check connectivity to external services - if
// desired, callers should use `(*Provider).ValidateConnection` directly to get warnings related
// to connection issues.
func NewAuthzProviders(
	db database.DB,
	cfg schema.SiteConfiguration,
	conns []*types.GitLabConnection,
) *atypes.ProviderInitResult {
	initResults := &atypes.ProviderInitResult{}
	// Authorization (i.e., permissions) providers
	for _, c := range conns {
		p, err := newAuthzProvider(db, c, cfg.AuthProviders)
		if err != nil {
			initResults.InvalidConnections = append(initResults.InvalidConnections, extsvc.TypeGitLab)
			initResults.Problems = append(initResults.Problems, err.Error())
		} else if p != nil {
			initResults.Providers = append(initResults.Providers, p)
		}
	}

	return initResults
}

func newAuthzProvider(db database.DB, c *types.GitLabConnection, ps []schema.AuthProviders) (authz.Provider, error) {
	if c.Authorization == nil {
		return nil, nil
	}

	if errLicense := licensing.Check(licensing.FeatureACLs); errLicense != nil {
		return nil, errLicense
	}

	glURL, err := url.Parse(c.Url)
	if err != nil {
		return nil, errors.Errorf("Could not parse URL for GitLab instance %q: %s", c.Url, err)
	}

	switch idp := c.Authorization.IdentityProvider; {
	case idp.Oauth != nil:
		// Check that there is a GitLab authn provider corresponding to this GitLab instance
		foundAuthProvider := false
		syncInternalRepoPermissions := true
		for _, authnProvider := range ps {
			if authnProvider.Gitlab == nil {
				continue
			}
			authnURL := authnProvider.Gitlab.Url
			if authnURL == "" {
				authnURL = "https://gitlab.com"
			}
			authProviderURL, err := url.Parse(authnURL)
			if err != nil {
				// Ignore the error here, because the authn provider is responsible for its own validation
				continue
			}
			if authProviderURL.Hostname() == glURL.Hostname() {
				foundAuthProvider = true
				sirp := authnProvider.Gitlab.SyncInternalRepoPermissions
				syncInternalRepoPermissions = sirp == nil || *sirp
				break
			}
		}
		if !foundAuthProvider {
			return nil, errors.Errorf("Did not find authentication provider matching %q. Check the [**site configuration**](/site-admin/configuration) to verify an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) exists for %s.", c.Url, c.Url)
		}

		return NewOAuthProvider(OAuthProviderOp{
			URN:                         c.URN,
			BaseURL:                     glURL,
			Token:                       c.Token,
			TokenType:                   gitlab.TokenType(c.TokenType),
			DB:                          db,
			SyncInternalRepoPermissions: syncInternalRepoPermissions,
		})
	case idp.Username != nil:
		return NewSudoProvider(SudoProviderOp{
			URN:                         c.URN,
			BaseURL:                     glURL,
			SudoToken:                   c.Token,
			UseNativeUsername:           true,
			SyncInternalRepoPermissions: !c.MarkInternalReposAsPublic,
		})
	case idp.External != nil:
		ext := idp.External
		for _, authProvider := range ps {
			saml := authProvider.Saml
			foundMatchingSAML := saml != nil && saml.ConfigID == ext.AuthProviderID && ext.AuthProviderType == saml.Type
			oidc := authProvider.Openidconnect
			foundMatchingOIDC := oidc != nil && oidc.ConfigID == ext.AuthProviderID && ext.AuthProviderType == oidc.Type
			if foundMatchingSAML || foundMatchingOIDC {
				return NewSudoProvider(SudoProviderOp{
					URN:     c.URN,
					BaseURL: glURL,
					AuthnConfigID: providers.ConfigID{
						Type: ext.AuthProviderType,
						ID:   ext.AuthProviderID,
					},
					GitLabProvider:              ext.GitlabProvider,
					SudoToken:                   c.Token,
					UseNativeUsername:           false,
					SyncInternalRepoPermissions: !c.MarkInternalReposAsPublic,
				})
			}
		}
		return nil, errors.Errorf("Did not find authentication provider matching type %s and configID %s. Check the [**site configuration**](/site-admin/configuration) to verify that an entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) matches the type and configID.", ext.AuthProviderType, ext.AuthProviderID)
	default:
		return nil, errors.Errorf("No identityProvider was specified")
	}
}

// NewOAuthProvider is a mockable constructor for new OAuthProvider instances.
var NewOAuthProvider = func(op OAuthProviderOp) (authz.Provider, error) {
	return newOAuthProvider(op, op.HTTPFactory)
}

// NewSudoProvider is a mockable constructor for new SudoProvider instances.
var NewSudoProvider = func(op SudoProviderOp) (authz.Provider, error) {
	return newSudoProvider(op, nil)
}

// ValidateAuthz validates the authorization fields of the given GitLab external
// service config.
func ValidateAuthz(cfg *schema.GitLabConnection, ps []schema.AuthProviders) error {
	_, err := newAuthzProvider(nil, &types.GitLabConnection{GitLabConnection: cfg}, ps)
	return err
}
