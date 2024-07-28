package bitbucketserver

import (
	"net/url"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	atypes "github.com/sourcegraph/sourcegraph/internal/authz/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewAuthzProviders returns the set of Bitbucket Server authz providers derived from the connections.
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
	conns []*types.BitbucketServerConnection,
	authProviders []schema.AuthProviders,
) *atypes.ProviderInitResult {
	initResults := &atypes.ProviderInitResult{}

	oauthProviders := make(map[string]*schema.BitbucketServerAuthProvider)
	for _, p := range authProviders {
		if p.Bitbucketserver != nil {
			var id string
			bbURL, err := url.Parse(p.Bitbucketserver.Url)
			if err != nil {
				// error reporting for this should happen elsewhere, for now just use what is given
				id = p.Bitbucketserver.Url
			} else {
				// use codehost normalized URL as ID
				ch := extsvc.NewCodeHost(bbURL, p.Bitbucketserver.Type)
				id = ch.ServiceID
			}
			oauthProviders[id] = p.Bitbucketserver
		}
	}

	// Authorization (i.e., permissions) providers
	for _, c := range conns {
		pluginPerm := c.Plugin != nil && c.Plugin.Permissions == "enabled"
		p, err := newAuthzProvider(db, c, pluginPerm)
		if err != nil {
			initResults.InvalidConnections = append(initResults.InvalidConnections, extsvc.TypeBitbucketServer)
			initResults.Problems = append(initResults.Problems, err.Error())
		} else if p != nil {
			initResults.Providers = append(initResults.Providers, p)
		}
	}

	return initResults
}

func newAuthzProvider(
	db database.DB,
	c *types.BitbucketServerConnection,
	pluginPerm bool,
) (authz.Provider, error) {
	if c.Authorization == nil {
		return nil, nil
	}

	if err := licensing.Check(licensing.FeatureACLs); err != nil {
		return nil, err
	}

	cli, err := bitbucketserver.NewClient(c.URN, c.BitbucketServerConnection, nil)
	if err != nil {
		return nil, err
	}

	switch idp := c.Authorization.IdentityProvider; {
	case idp.Username != nil:
		return NewProvider(cli, c.URN, pluginPerm), nil
	default:
		// The default is OAuth.
		return NewOAuthProvider(db, c, ProviderOptions{BitbucketServerClient: cli}), nil
	}
}

// ValidateAuthz validates the authorization fields of the given BitbucketServer external
// service config.
func ValidateAuthz(c *schema.BitbucketServerConnection) error {
	_, err := newAuthzProvider(nil, &types.BitbucketServerConnection{BitbucketServerConnection: c}, false)
	return err
}
