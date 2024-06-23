package bitbucketserver

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/oauthtoken"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type OAuthProvider struct {
	urn      string
	codeHost *extsvc.CodeHost
	client   *bitbucketserver.Client
	pageSize int // Page size to use in paginated requests.
	db       database.DB
}

type ProviderOptions struct {
	BitbucketServerClient *bitbucketserver.Client
}

var _ authz.Provider = (*OAuthProvider)(nil)

// NewProvider returns a new Bitbucket Server authorization provider that uses
// the given bitbucket.Client to talk to the Bitbucket Server API that is
// the source of truth for permissions. Sourcegraph users will need a valid
// Bitbucket Server external account for permissions to sync correctly.
func NewOAuthProvider(db database.DB, conn *types.BitbucketServerConnection, opts ProviderOptions) *OAuthProvider {
	baseURL, err := url.Parse(conn.Url)
	if err != nil {
		return nil
	}

	if opts.BitbucketServerClient == nil {
		opts.BitbucketServerClient, err = bitbucketserver.NewClient(conn.URN, conn.BitbucketServerConnection, httpcli.ExternalClient)
		if err != nil {
			fmt.Printf("Error creating Bitbucket Server client: %s\n", err)
			return nil
		}
	}

	return &OAuthProvider{
		urn:      conn.URN,
		codeHost: extsvc.NewCodeHost(baseURL, extsvc.TypeBitbucketServer),
		client:   opts.BitbucketServerClient,
		pageSize: 1000,
		db:       db,
	}
}

// ValidateConnection validates that the Provider has access to the Bitbucket Server API
// with the credentials it was configured with.
//
// Credentials are verified by querying the "/2.0/repositories" endpoint.
// This validates that the credentials have the `REPO_READ` scope.
func (p *OAuthProvider) ValidateConnection(ctx context.Context) error {
	// We don't care about the contents returned, only whether or not an error occurred
	_, _, err := p.client.Repos(ctx, nil)
	return err
}

func (p *OAuthProvider) URN() string {
	return p.urn
}

// ServiceID returns the absolute URL that identifies the Bitbucket Server instance
// this provider is configured with.
func (p *OAuthProvider) ServiceID() string { return p.codeHost.ServiceID }

// ServiceType returns the type of this Provider, namely, "bitbucketServer".
func (p *OAuthProvider) ServiceType() string { return p.codeHost.ServiceType }

// FetchAccount satisfies the authz.Provider interface.
func (p *OAuthProvider) FetchAccount(ctx context.Context, user *types.User, _ []*extsvc.Account, _ []string) (acct *extsvc.Account, err error) {
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
func (p *OAuthProvider) FetchUserPerms(ctx context.Context, account *extsvc.Account, opts authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	switch {
	case account == nil:
		return nil, errors.New("no account provided")
	case !extsvc.IsHostOfAccount(p.codeHost, account):
		return nil, errors.Errorf("not a code host of the account: want %q but have %q",
			p.codeHost.ServiceID, account.AccountSpec.ServiceID)
	case account.Data == nil:
		return nil, errors.New("no account data provided")
	}

	_, tok, err := bitbucketserver.GetExternalAccountData(ctx, &account.AccountData)
	if err != nil {
		return nil, err
	}
	oauthToken := &auth.OAuthBearerToken{
		Token:              tok.AccessToken,
		RefreshToken:       tok.RefreshToken,
		Expiry:             tok.Expiry,
		NeedsRefreshBuffer: 5,
	}
	oauthToken.RefreshFunc = oauthtoken.GetAccountRefreshAndStoreOAuthTokenFunc(p.db.UserExternalAccounts(), account.ID, bitbucketserver.GetOAuthContext(p.codeHost.BaseURL.String()))

	client := p.client.WithAuthenticator(oauthToken)

	ids, err := p.repoIDs(ctx, client)

	extIDs := make([]extsvc.RepoID, 0, len(ids))
	for _, id := range ids {
		extIDs = append(extIDs, extsvc.RepoID(strconv.FormatUint(uint64(id), 10)))
	}

	return &authz.ExternalUserPermissions{
		Exacts: extIDs,
	}, err
}

func (p *OAuthProvider) repoIDs(ctx context.Context, client *bitbucketserver.Client) ([]uint32, error) {
	// TODO: reimplement or not?
	// if p.pluginPerm {
	// 	return p.repoIDsFromPlugin(ctx, username)
	// }
	return repoIDsFromAPI(ctx, p.pageSize, client)
}

// repoIDsFromAPI returns all repositories for which the given user has the permission to read from
// the Bitbucket Server API. when no username is given, only public repos are returned.
func repoIDsFromAPI(ctx context.Context, pageSize int, client *bitbucketserver.Client) (ids []uint32, err error) {
	t := &bitbucketserver.PageToken{Limit: pageSize}

	var filters []string
	filters = append(filters, "?visibility=private")

	for t.HasMore() {
		repos, next, err := client.Repos(ctx, t, filters...)
		if err != nil {
			return ids, err
		}

		for _, r := range repos {
			ids = append(ids, uint32(r.ID))
		}

		t = next
	}

	if len(ids) == 0 {
		return nil, errNoResults
	}

	return ids, nil
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
func (p *OAuthProvider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository, opts authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	switch {
	case repo == nil:
		return nil, errors.New("no repo provided")
	case !extsvc.IsHostOfRepo(p.codeHost, &repo.ExternalRepoSpec):
		return nil, errors.Errorf("not a code host of the repo: want %q but have %q",
			p.codeHost.ServiceID, repo.ServiceID)
	}

	ids, err := p.userIDs(ctx, repo.ID)

	extIDs := make([]extsvc.AccountID, 0, len(ids))
	for _, id := range ids {
		extIDs = append(extIDs, extsvc.AccountID(strconv.FormatInt(int64(id), 10)))
	}

	return extIDs, err
}

func (p *OAuthProvider) userIDs(ctx context.Context, repoID string) (ids []int, err error) {
	t := &bitbucketserver.PageToken{Limit: p.pageSize}
	f := bitbucketserver.UserFilter{Permission: bitbucketserver.PermissionFilter{
		Root:         "REPO_READ",
		RepositoryID: repoID,
	}}

	for t.HasMore() {
		users, next, err := p.client.Users(ctx, t, f)
		if err != nil {
			return ids, err
		}

		for _, u := range users {
			ids = append(ids, u.ID)
		}

		t = next
	}

	return ids, nil
}
