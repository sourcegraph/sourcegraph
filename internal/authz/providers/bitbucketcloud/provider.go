// Package bitbucketcloud contains an authorization provider for Bitbucket Cloud.
package bitbucketcloud

import (
	"context"
	"net/url"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/oauthtoken"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Provider is an implementation of AuthzProvider that provides repository and
// user permissions as determined from Bitbucket Cloud.
type Provider struct {
	urn      string
	codeHost *extsvc.CodeHost
	client   bitbucketcloud.Client
	pageSize int // Page size to use in paginated requests.
	db       database.DB
}

type ProviderOptions struct {
	BitbucketCloudClient bitbucketcloud.Client
}

var _ authz.Provider = (*Provider)(nil)

// NewProvider returns a new Bitbucket Cloud authorization provider that uses
// the given bitbucket.Client to talk to the Bitbucket Cloud API that is
// the source of truth for permissions. Sourcegraph users will need a valid
// Bitbucket Cloud external account for permissions to sync correctly.
func NewProvider(db database.DB, conn *types.BitbucketCloudConnection, opts ProviderOptions) *Provider {
	baseURL, err := url.Parse(conn.Url)
	if err != nil {
		return nil
	}

	if opts.BitbucketCloudClient == nil {
		opts.BitbucketCloudClient, err = bitbucketcloud.NewClient(conn.URN, conn.BitbucketCloudConnection, httpcli.NewExternalClientFactory())
		if err != nil {
			return nil
		}
	}

	return &Provider{
		urn:      conn.URN,
		codeHost: extsvc.NewCodeHost(baseURL, extsvc.TypeBitbucketCloud),
		client:   opts.BitbucketCloudClient,
		pageSize: 1000,
		db:       db,
	}
}

// ValidateConnection validates that the Provider has access to the Bitbucket Cloud API
// with the credentials it was configured with.
//
// Credentials are verified by querying the "/2.0/repositories" endpoint.
// This validates that the credentials have the `repository` scope.
// See: https://developer.atlassian.com/cloud/bitbucket/rest/api-group-repositories/#api-repositories-get
func (p *Provider) ValidateConnection(ctx context.Context) error {
	// We don't care about the contents returned, only whether or not an error occurred
	_, _, err := p.client.Repos(ctx, nil, "", nil)
	return err
}

func (p *Provider) URN() string {
	return p.urn
}

// ServiceID returns the absolute URL that identifies the Bitbucket Server instance
// this provider is configured with.
func (p *Provider) ServiceID() string { return p.codeHost.ServiceID }

// ServiceType returns the type of this Provider, namely, "bitbucketCloud".
func (p *Provider) ServiceType() string { return p.codeHost.ServiceType }

// FetchAccount satisfies the authz.Provider interface.
func (p *Provider) FetchAccount(ctx context.Context, user *types.User, _ []*extsvc.Account, _ []string) (acct *extsvc.Account, err error) {
	return nil, nil
}

// FetchUserPerms returns a list of repository IDs (on code host) that the given account
// has read access on the code host. The repository ID has the same value as it would be
// used as api.ExternalRepoSpec.ID. The returned list only includes private repository IDs.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.atlassian.com/bitbucket-server/rest/5.16.0/bitbucket-rest.html#idm8296923984
func (p *Provider) FetchUserPerms(ctx context.Context, account *extsvc.Account, opts authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	switch {
	case account == nil:
		return nil, errors.New("no account provided")
	case !extsvc.IsHostOfAccount(p.codeHost, account):
		return nil, errors.Errorf("not a code host of the account: want %q but have %q",
			p.codeHost.ServiceID, account.AccountSpec.ServiceID)
	case account.Data == nil:
		return nil, errors.New("no account data provided")
	}

	_, tok, err := bitbucketcloud.GetExternalAccountData(ctx, &account.AccountData)
	if err != nil {
		return nil, err
	}
	oauthToken := &auth.OAuthBearerToken{
		Token:              tok.AccessToken,
		RefreshToken:       tok.RefreshToken,
		Expiry:             tok.Expiry,
		NeedsRefreshBuffer: 5,
	}
	oauthToken.RefreshFunc = oauthtoken.GetAccountRefreshAndStoreOAuthTokenFunc(p.db.UserExternalAccounts(), account.ID, bitbucketcloud.GetOAuthContext(p.codeHost.BaseURL.String()))

	client := p.client.WithAuthenticator(oauthToken)

	repos, _, err := client.Repos(ctx, &bitbucketcloud.PageToken{Pagelen: 100}, "", &bitbucketcloud.ReposOptions{RequestOptions: bitbucketcloud.RequestOptions{FetchAll: true}, Role: "member"})
	if err != nil {
		return nil, err
	}

	extIDs := make([]extsvc.RepoID, 0, len(repos))
	for _, repo := range repos {
		extIDs = append(extIDs, extsvc.RepoID(repo.UUID))
	}

	return &authz.ExternalUserPermissions{
		Exacts: extIDs,
	}, err
}

// FetchRepoPerms returns a list of user IDs (on code host) who have read access to
// the given repo on the code host. The user ID has the same value as it would
// be used as extsvc.Account.AccountID. The returned list includes both direct access
// and inherited from the group membership.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.atlassian.com/bitbucket-server/rest/5.16.0/bitbucket-rest.html#idm8283203728
func (p *Provider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository, opts authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	repoNameParts := strings.Split(repo.URI, "/")
	repoOwner := repoNameParts[1]
	repoName := repoNameParts[2]

	users, _, err := p.client.ListExplicitUserPermsForRepo(ctx, &bitbucketcloud.PageToken{Pagelen: 100}, repoOwner, repoName, &bitbucketcloud.RequestOptions{FetchAll: true})
	if err != nil {
		return nil, err
	}

	// Bitbucket Cloud API does not return the owner of the repository as part
	// of the explicit permissions list, so we need to fetch and add them.
	bbCloudRepo, err := p.client.Repo(ctx, repoOwner, repoName)
	if err != nil {
		return nil, err
	}

	if bbCloudRepo.Owner != nil {
		users = append(users, bbCloudRepo.Owner)
	}

	userIDs := make([]extsvc.AccountID, len(users))
	for i, user := range users {
		userIDs[i] = extsvc.AccountID(user.UUID)
	}

	return userIDs, nil
}
