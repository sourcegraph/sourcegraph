// Package bitbucketserver contains an authorization provider for Bitbucket Server.
package bitbucketserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// Provider is an implementation of AuthzProvider that provides repository permissions as
// determined from a Bitbucket Server instance API.
type Provider struct {
	urn      string
	client   *bitbucketserver.Client
	codeHost *extsvc.CodeHost
	pageSize int // Page size to use in paginated requests.

	// pluginPerm enables fetching permissions from the alternative roaring
	// bitmap endpoint provided by the Bitbucket Server Sourcegraph plugin:
	// https://github.com/sourcegraph/bitbucket-server-plugin
	pluginPerm bool
}

var _ authz.Provider = (*Provider)(nil)

// NewProvider returns a new Bitbucket Server authorization provider that uses
// the given bitbucketserver.Client to talk to a Bitbucket Server API that is
// the source of truth for permissions. It assumes usernames of Sourcegraph accounts
// match 1-1 with usernames of Bitbucket Server API users.
func NewProvider(cli *bitbucketserver.Client, urn string, pluginPerm bool) *Provider {
	return &Provider{
		urn:        urn,
		client:     cli,
		codeHost:   extsvc.NewCodeHost(cli.URL, extsvc.TypeBitbucketServer),
		pageSize:   1000,
		pluginPerm: pluginPerm,
	}
}

// Validate validates that the Provider has access to the Bitbucket Server API
// with the OAuth credentials it was configured with.
func (p *Provider) Validate() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	username, err := p.client.Username()
	if err != nil {
		return []string{err.Error()}
	}

	if _, err := p.client.UserPermissions(ctx, username); err != nil {
		return []string{err.Error()}
	}

	return nil
}

func (p *Provider) URN() string {
	return p.urn
}

// ServiceID returns the absolute URL that identifies the Bitbucket Server instance
// this provider is configured with.
func (p *Provider) ServiceID() string { return p.codeHost.ServiceID }

// ServiceType returns the type of this Provider, namely, "bitbucketServer".
func (p *Provider) ServiceType() string { return p.codeHost.ServiceType }

// FetchAccount satisfies the authz.Provider interface.
func (p *Provider) FetchAccount(ctx context.Context, user *types.User, _ []*extsvc.Account) (acct *extsvc.Account, err error) {
	if user == nil {
		return nil, nil
	}

	tr, ctx := trace.New(ctx, "bitbucket.authz.provider.FetchAccount", "")
	defer func() {
		tr.LogFields(
			otlog.String("user.name", user.Username),
			otlog.Int32("user.id", user.ID),
		)

		if err != nil {
			tr.SetError(err)
		}

		tr.Finish()
	}()

	bitbucketUser, err := p.user(ctx, user.Username)
	if err != nil {
		return nil, err
	}

	accountData, err := json.Marshal(bitbucketUser)
	if err != nil {
		return nil, err
	}

	return &extsvc.Account{
		UserID: user.ID,
		AccountSpec: extsvc.AccountSpec{
			ServiceType: p.codeHost.ServiceType,
			ServiceID:   p.codeHost.ServiceID,
			AccountID:   strconv.Itoa(bitbucketUser.ID),
		},
		AccountData: extsvc.AccountData{
			Data: (*json.RawMessage)(&accountData),
		},
	}, nil
}

// FetchUserPerms returns a list of repository IDs (on code host) that the given account
// has read access on the code host. The repository ID has the same value as it would be
// used as api.ExternalRepoSpec.ID. The returned list only includes private repository IDs.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://docs.atlassian.com/bitbucket-server/rest/5.16.0/bitbucket-rest.html#idm8296923984
func (p *Provider) FetchUserPerms(ctx context.Context, account *extsvc.Account) (*authz.ExternalUserPermissions, error) {
	switch {
	case account == nil:
		return nil, errors.New("no account provided")
	case account.Data == nil:
		return nil, errors.New("no account data provided")
	case !extsvc.IsHostOfAccount(p.codeHost, account):
		return nil, fmt.Errorf("not a code host of the account: want %q but have %q",
			p.codeHost.ServiceID, account.AccountSpec.ServiceID)
	}

	var user bitbucketserver.User
	if err := json.Unmarshal(*account.Data, &user); err != nil {
		return nil, errors.Wrap(err, "unmarshaling account data")
	}

	ids, err := p.repoIDs(ctx, user.Name, false)

	extIDs := make([]extsvc.RepoID, 0, len(ids))
	for _, id := range ids {
		extIDs = append(extIDs, extsvc.RepoID(strconv.FormatUint(uint64(id), 10)))
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
func (p *Provider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error) {
	switch {
	case repo == nil:
		return nil, errors.New("no repo provided")
	case !extsvc.IsHostOfRepo(p.codeHost, &repo.ExternalRepoSpec):
		return nil, fmt.Errorf("not a code host of the repo: want %q but have %q",
			p.codeHost.ServiceID, repo.ServiceID)
	}

	ids, err := p.userIDs(ctx, repo.ID)

	extIDs := make([]extsvc.AccountID, 0, len(ids))
	for _, id := range ids {
		extIDs = append(extIDs, extsvc.AccountID(strconv.FormatInt(int64(id), 10)))
	}

	return extIDs, err
}

var errNoResults = errors.New("no results returned by the Bitbucket Server API")

func (p *Provider) repoIDs(ctx context.Context, username string, public bool) ([]uint32, error) {
	if p.pluginPerm {
		return p.repoIDsFromPlugin(ctx, username)
	}
	return p.repoIDsFromAPI(ctx, username, public)
}

// repoIDsFromAPI returns all repositories for which the given user has the permission to read from
// the Bitbucket Server API. when no username is given, only public repos are returned.
func (p *Provider) repoIDsFromAPI(ctx context.Context, username string, public bool) (ids []uint32, err error) {
	t := &bitbucketserver.PageToken{Limit: p.pageSize}
	c := p.client

	var filters []string
	if username == "" {
		filters = append(filters, "?visibility=public")
	} else if c, err = c.Sudo(username); err != nil {
		return nil, err
	} else if !public {
		filters = append(filters, "?visibility=private")
	}

	for t.HasMore() {
		repos, next, err := c.Repos(ctx, t, filters...)
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

func (p *Provider) repoIDsFromPlugin(ctx context.Context, username string) (ids []uint32, err error) {
	c, err := p.client.Sudo(username)
	if err != nil {
		return nil, err
	}
	return c.RepoIDs(ctx, "read")
}

func (p *Provider) user(ctx context.Context, username string, fs ...bitbucketserver.UserFilter) (*bitbucketserver.User, error) {
	t := &bitbucketserver.PageToken{Limit: p.pageSize}
	fs = append(fs, bitbucketserver.UserFilter{Filter: username})

	for t.HasMore() {
		users, next, err := p.client.Users(ctx, t, fs...)
		if err != nil {
			return nil, err
		}

		for _, u := range users {
			if u.Name == username {
				return u, nil
			}
		}

		t = next
	}

	return nil, errNoResults
}

func (p *Provider) userIDs(ctx context.Context, repoID string) (ids []int, err error) {
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
