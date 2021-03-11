package gitlab

import (
	"context"
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var _ authz.Provider = (*OAuthProvider)(nil)

type OAuthProvider struct {
	// The token is the access token used for syncing repositories from the code host,
	// but it may or may not be a sudo-scoped.
	token string

	urn            string
	clientProvider *gitlab.ClientProvider
	clientURL      *url.URL
	codeHost       *extsvc.CodeHost
}

type OAuthProviderOp struct {
	// The unique resource identifier of the external service where the provider is defined.
	URN string

	// BaseURL is the URL of the GitLab instance.
	BaseURL *url.URL

	// Token is an access token with api scope, it may or may not have sudo scope.
	//
	// 🚨 SECURITY: This value contains secret information that must not be shown to non-site-admins.
	Token string
}

func newOAuthProvider(op OAuthProviderOp, cli httpcli.Doer) *OAuthProvider {
	return &OAuthProvider{
		token: op.Token,

		urn:            op.URN,
		clientProvider: gitlab.NewClientProvider(op.BaseURL, cli),
		clientURL:      op.BaseURL,
		codeHost:       extsvc.NewCodeHost(op.BaseURL, extsvc.TypeGitLab),
	}
}

func (p *OAuthProvider) Validate() (problems []string) {
	return nil
}

func (p *OAuthProvider) URN() string {
	return p.urn
}

func (p *OAuthProvider) ServiceID() string {
	return p.codeHost.ServiceID
}

func (p *OAuthProvider) ServiceType() string {
	return p.codeHost.ServiceType
}

func (p *OAuthProvider) FetchAccount(ctx context.Context, user *types.User, current []*extsvc.Account) (mine *extsvc.Account, err error) {
	return nil, nil
}

// FetchUserPerms returns a list of private project IDs (on code host) that the given account
// has read access to. The project ID has the same value as it would be
// used as api.ExternalRepoSpec.ID. The returned list only includes private project IDs.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.gitlab.com/ee/api/projects.html#list-all-projects
func (p *OAuthProvider) FetchUserPerms(ctx context.Context, account *extsvc.Account) (*authz.ExternalUserPermissions, error) {
	if account == nil {
		return nil, errors.New("no account provided")
	} else if !extsvc.IsHostOfAccount(p.codeHost, account) {
		return nil, fmt.Errorf("not a code host of the account: want %q but have %q",
			account.AccountSpec.ServiceID, p.codeHost.ServiceID)
	}

	_, tok, err := gitlab.GetExternalAccountData(&account.AccountData)
	if err != nil {
		return nil, errors.Wrap(err, "get external account data")
	} else if tok == nil {
		return nil, errors.New("no token found in the external account data")
	}

	client := p.clientProvider.GetOAuthClient(tok.AccessToken)
	return listProjects(ctx, client)
}

// FetchRepoPerms returns a list of user IDs (on code host) who have read access to
// the given project on the code host. The user ID has the same value as it would
// be used as extsvc.Account.AccountID. The returned list includes both direct access
// and inherited from the group membership.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.gitlab.com/ee/api/members.html#list-all-members-of-a-group-or-project-including-inherited-members
func (p *OAuthProvider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error) {
	if repo == nil {
		return nil, errors.New("no repository provided")
	} else if !extsvc.IsHostOfRepo(p.codeHost, &repo.ExternalRepoSpec) {
		return nil, fmt.Errorf("not a code host of the repository: want %q but have %q",
			repo.ServiceID, p.codeHost.ServiceID)
	}

	client := p.clientProvider.GetPATClient(p.token, "")
	return listMembers(ctx, client, repo.ID)
}
