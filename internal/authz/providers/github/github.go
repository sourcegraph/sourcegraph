package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/oauthtoken"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Provider implements authz.Provider for GitHub repository permissions.
type Provider struct {
	urn      string
	client   func() (client, error)
	codeHost *extsvc.CodeHost
	// groupsCache may be nil if group caching is disabled (negative TTL)
	groupsCache *cachedGroups

	syncInternalRepoPermissions bool

	// enableGithubInternalRepoVisibility is a feature flag to optionally enable a fix for
	// internal repos on GithHub Enterprise. At the moment we do not handle internal repos
	// explicitly and allow all org members to read it irrespective of repo permissions. We have
	// this as a temporary feature flag here to guard against any regressions. This will go away as
	// soon as we have verified our approach works and is reliable, at which point the fix will
	// become the default behaviour.
	enableGithubInternalRepoVisibility bool

	db database.DB
}

type ProviderOptions struct {
	// If a GitHubClient is not provided, one is constructed from GitHubURL
	GitHubClient *github.V3Client
	GitHubURL    *url.URL

	BaseAuther     auth.Authenticator
	GroupsCacheTTL time.Duration
	IsApp          bool
	DB             database.DB

	SyncInternalRepoPermissions bool
}

func NewProvider(urn string, opts ProviderOptions) (*Provider, error) {
	if opts.GitHubClient == nil {
		apiURL, _ := github.APIRoot(opts.GitHubURL)
		var err error
		opts.GitHubClient, err = github.NewV3Client(log.Scoped("provider.github.v3"),
			urn, apiURL, opts.BaseAuther, nil)
		if err != nil {
			return nil, err
		}
	}

	codeHost := extsvc.NewCodeHost(opts.GitHubURL, extsvc.TypeGitHub)

	var cg *cachedGroups
	if opts.GroupsCacheTTL >= 0 {
		cg = &cachedGroups{
			cache: rcache.NewWithTTL(
				fmt.Sprintf("gh_groups_perms:%s:%s", codeHost.ServiceID, urn), int(opts.GroupsCacheTTL.Seconds()),
			),
		}
	}

	return &Provider{
		urn:         urn,
		codeHost:    codeHost,
		groupsCache: cg,
		client: func() (client, error) {
			return &ClientAdapter{V3Client: opts.GitHubClient}, nil
		},
		db:                          opts.DB,
		syncInternalRepoPermissions: opts.SyncInternalRepoPermissions,
	}, nil
}

var _ authz.Provider = (*Provider)(nil)

// FetchAccount implements the authz.Provider interface. It always returns nil, because the GitHub
// API doesn't currently provide a way to fetch user by external SSO account.
func (p *Provider) FetchAccount(context.Context, *types.User, []*extsvc.Account, []string) (mine *extsvc.Account, err error) {
	return nil, nil
}

func (p *Provider) URN() string {
	return p.urn
}

func (p *Provider) ServiceID() string {
	return p.codeHost.ServiceID
}

func (p *Provider) ServiceType() string {
	return p.codeHost.ServiceType
}

func (p *Provider) ValidateConnection(ctx context.Context) error {
	required, ok := p.requiredAuthScopes()
	if !ok {
		return nil
	}

	client, err := p.client()
	if err != nil {
		return errors.Wrap(err, "unable to get client")
	}

	scopes, err := client.GetAuthenticatedOAuthScopes(ctx)
	if err != nil {
		return errors.Wrap(err, "additional OAuth scopes are required, but failed to get available scopes")
	}

	gotScopes := make(map[string]struct{})
	for _, gotScope := range scopes {
		gotScopes[gotScope] = struct{}{}
	}

	// check if required scopes are satisfied
	satisfiesScope := false
	for _, s := range required.oneOf {
		if _, found := gotScopes[s]; found {
			satisfiesScope = true
			break
		}
	}
	if !satisfiesScope {
		return errors.New(required.message)
	}

	return nil
}

type requiredAuthScope struct {
	// at least one of these scopes is required
	oneOf []string
	// message to display if this required auth scope is not satisfied
	message string
}

func (p *Provider) requiredAuthScopes() (requiredAuthScope, bool) {
	if p.groupsCache == nil {
		return requiredAuthScope{}, false
	}

	// Needs extra scope to pull group permissions
	return requiredAuthScope{
		oneOf: []string{"read:org", "write:org", "admin:org"},
		message: "Scope `read:org`, `write:org`, or `admin:org` is required to enable `authorization.groupsCacheTTL` - " +
			"please provide a `token` with the required scopes, or try updating the [**site configuration**](/site-admin/configuration)'s " +
			"corresponding entry in [`auth.providers`](https://sourcegraph.com/docs/admin/auth) to enable `allowGroupsPermissionsSync`.",
	}, true
}

// getAllAuthenticatedUserOrgs gets all the orgs the authenticated user is a member of.
func getAllAuthenticatedUserOrgs(ctx context.Context, cli client) ([]*github.Org, error) {
	var orgs []*github.Org
	for page := 1; true; page++ {
		select {
		case <-ctx.Done():
			return orgs, ctx.Err()
		default:
		}
		pageOrgs, hasNextPage, _, err := cli.GetAuthenticatedUserOrgs(ctx, page)
		if err != nil {
			// We return partial results
			return orgs, errors.Wrap(err, "list orgs for authenticated user")
		}
		orgs = append(orgs, pageOrgs...)

		if !hasNextPage {
			break
		}
	}

	return orgs, nil
}

// getAllInternalRepositoriesForOrg fetches all internal repositories for the given org.
func getAllInternalRepositoriesForOrg(ctx context.Context, cli client, orgLogin string) ([]*github.Repository, error) {
	var repos []*github.Repository
	for page := 1; true; page++ {
		select {
		case <-ctx.Done():
			return repos, ctx.Err()
		default:
		}
		reposPage, hasNextPage, _, err := cli.ListOrgRepositories(ctx, orgLogin, page, "internal")
		if err != nil {
			// We return partial results
			return repos, errors.Wrap(err, "list internal repos for org")
		}
		repos = append(repos, reposPage...)

		if !hasNextPage {
			break
		}
	}

	return repos, nil
}

// getAllAuthenticatedUserInternalRepositories returns all internal repositories that the authenticated
// user has access to across all orgs.
func getAllAuthenticatedUserInternalRepositories(ctx context.Context, cli client) ([]*github.Repository, error) {
	var repos []*github.Repository
	orgs, err := getAllAuthenticatedUserOrgs(ctx, cli)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		select {
		case <-ctx.Done():
			return repos, ctx.Err()
		default:
		}
		orgRepos, err := getAllInternalRepositoriesForOrg(ctx, cli, org.Login)
		if err != nil {
			return repos, err
		}
		repos = append(repos, orgRepos...)
	}
	return repos, nil
}

func getAllAuthenticatedUserAffiliatedRepositories(ctx context.Context, cli client, affiliations ...github.RepositoryAffiliation) ([]*github.Repository, error) {
	var repos []*github.Repository
	// Sync direct affiliations
	for page := 1; true; page++ {
		select {
		case <-ctx.Done():
			return repos, ctx.Err()
		default:
		}
		reposPage, hasNextPage, _, err := cli.ListAffiliatedRepositories(ctx, github.VisibilityPrivate, page, 100, affiliations...)
		if err != nil {
			return repos, errors.Wrap(err, "list affiliated repos for user")
		}

		repos = append(repos, reposPage...)

		if !hasNextPage {
			break
		}
	}

	return repos, nil
}

// fetchCachedAuthenticatedUserPerms fetches permissions for the authenticated user using cached group membership data.
func (p *Provider) fetchCachedAuthenticatedUserPerms(ctx context.Context, logger log.Logger, cli client, accountID extsvc.AccountID, opts authz.FetchPermsOptions) (collections.Set[extsvc.RepoID], error) {
	userRepos := collections.NewSet[extsvc.RepoID]()

	// If groupsCache is disabled, return early
	if p.groupsCache == nil {
		return userRepos, nil
	}

	// Now, we look for groups this user belongs to that give access to additional
	// repositories.
	groups, err := p.getUserAffiliatedGroups(ctx, cli, opts)
	if err != nil {
		return userRepos, errors.Wrap(err, "get groups affiliated with user")
	}

	// Get repos from groups, cached if possible.
	for _, group := range groups {
		// If this is a partial cache, add self to group
		if len(group.Users) > 0 {
			hasUser := false
			for _, user := range group.Users {
				if user == accountID {
					hasUser = true
					break
				}
			}
			if !hasUser {
				group.Users = append(group.Users, accountID)
				if err := p.groupsCache.setGroup(group); err != nil {
					logger.Warn("setting group", log.Error(err))
				}
			}
		}

		// If a valid cached value was found, use it and continue. Check for a nil,
		// because it is possible this cached group does not have any repositories, in
		// which case it should have a non-nil length 0 slice of repositories.
		if group.Repositories != nil {
			userRepos.Add(group.Repositories...)
			continue
		}

		// Perform full sync. Start with instantiating the repos slice.
		group.Repositories = make([]extsvc.RepoID, 0, 100)
		isOrg := group.Team == ""
		hasNextPage := true
		for page := 1; hasNextPage; page++ {
			var repos []*github.Repository
			if isOrg {
				repos, hasNextPage, _, err = cli.ListOrgRepositories(ctx, group.Org, page, "")
			} else {
				repos, hasNextPage, _, err = cli.ListTeamRepositories(ctx, group.Org, group.Team, page)
			}
			if github.IsNotFound(err) || github.HTTPErrorCode(err) == http.StatusForbidden {
				// If we get a 403/404 here, something funky is going on and this is very
				// unexpected. Since this is likely not transient, instead of bailing out and
				// potentially causing unbounded retries later, we let this result proceed to
				// cache. This is safe because the cache will eventually get invalidated, at
				// which point we can retry this group, or a sync can be triggered that marks the
				// cached group as invalidated. GitHub sometimes returns 403 when requesting team
				// or org information when the token is not allowed to see it, so we treat it the
				// same as 404.
				logger.Debug("list repos for group: unexpected 403/404, persisting to cache",
					log.Error(err))
			} else if err != nil {
				// Add and return what we've found on this page but don't persist group
				// to cache
				for _, r := range repos {
					userRepos.Add(extsvc.RepoID(r.ID))
				}
				return userRepos, errors.Wrap(err, "list repos for group")
			}
			// Add results to both group (for persistence) and permissions for user
			for _, r := range repos {
				repoID := extsvc.RepoID(r.ID)
				group.Repositories = append(group.Repositories, repoID)
				userRepos.Add(repoID)
			}
		}

		// Persist repos affiliated with group to cache
		if err := p.groupsCache.setGroup(group); err != nil {
			logger.Warn("setting group", log.Error(err))
		}
	}

	return userRepos, nil
}

// fetchAuthenticatedUserPerms fetches all the private repo ids that the authenticated client can access.
//
// This may return a partial result if an error is encountered, e.g. via rate limits.
func (p *Provider) fetchAuthenticatedUserPerms(ctx context.Context, cli client, accountID extsvc.AccountID, opts authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	// 🚨 SECURITY: Use user token is required to only list repositories the user has access to.
	logger := log.Scoped("fetchUserPermsByToken")

	// Repository affiliations to list for - groupsCache only lists for a subset. Left
	// unset indicates all affiliations should be sync'd.
	affiliations := []github.RepositoryAffiliation{}
	if p.groupsCache != nil {
		// We sync just a subset of direct affiliations - we let other permissions
		// ('organization' affiliation) be sync'd by teams/orgs.
		affiliations = []github.RepositoryAffiliation{github.AffiliationOwner, github.AffiliationCollaborator}
	}

	userRepos := collections.NewSet[extsvc.RepoID]()

	// Sync direct affiliations
	repos, err := getAllAuthenticatedUserAffiliatedRepositories(ctx, cli, affiliations...)
	// May return partial results, so we add possible repos first
	for _, r := range repos {
		userRepos.Add(extsvc.RepoID(r.ID))
	}
	if err != nil {
		return &authz.ExternalUserPermissions{Exacts: userRepos.Values()}, errors.Wrap(err, "list repos for user")
	}

	// If cache is disabled and syncInternalRepoPermissions is true, we also need to fetch a list of
	// internal repositories for each org the user belongs to.
	// If caching is enabled, then the internal repositories will be fetched as part of the group sync.
	if p.groupsCache == nil && p.syncInternalRepoPermissions {
		internalRepos, err := getAllAuthenticatedUserInternalRepositories(ctx, cli)
		for _, r := range internalRepos {
			userRepos.Add(extsvc.RepoID(r.ID))
		}
		if err != nil {
			return &authz.ExternalUserPermissions{Exacts: userRepos.Values()}, err
		}
	}

	if p.groupsCache != nil {
		// If groups caching is enabled, we need to fetch cached repositories as well
		groupsPerms, err := p.fetchCachedAuthenticatedUserPerms(ctx, logger, cli, accountID, opts)
		userRepos = userRepos.Union(groupsPerms)
		if err != nil {
			return &authz.ExternalUserPermissions{Exacts: userRepos.Values()}, err
		}
	}

	return &authz.ExternalUserPermissions{
		Exacts: userRepos.Values(),
	}, nil
}

// FetchUserPerms returns a list of repository IDs (on code host) that the given account
// has read access on the code host. The repository ID has the same value as it would be
// used as api.ExternalRepoSpec.ID. The returned list only includes private repository IDs.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://developer.github.com/v3/repos/#list-repositories-for-the-authenticated-user
func (p *Provider) FetchUserPerms(ctx context.Context, account *extsvc.Account, opts authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	if account == nil {
		return nil, errors.New("no account provided")
	} else if !extsvc.IsHostOfAccount(p.codeHost, account) {
		return nil, errors.Errorf("not a code host of the account: want %q but have %q",
			account.AccountSpec.ServiceID, p.codeHost.ServiceID)
	}

	_, tok, err := github.GetExternalAccountData(ctx, &account.AccountData)
	if err != nil {
		return nil, errors.Wrap(err, "get external account data")
	} else if tok == nil {
		return nil, errors.New("no token found in the external account data")
	}

	oauthToken := &auth.OAuthBearerToken{
		Token:              tok.AccessToken,
		RefreshToken:       tok.RefreshToken,
		Expiry:             tok.Expiry,
		RefreshFunc:        oauthtoken.GetAccountRefreshAndStoreOAuthTokenFunc(p.db.UserExternalAccounts(), account.ID, github.GetOAuthContext(strings.TrimSuffix(p.ServiceID(), "/"))),
		NeedsRefreshBuffer: 5,
	}

	client, err := p.client()
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}

	client = client.WithAuthenticator(oauthToken)

	return p.fetchAuthenticatedUserPerms(ctx, client, extsvc.AccountID(account.AccountID), opts)
}

// FetchRepoPerms returns a list of user IDs (on code host) who have read access to
// the given project on the code host. The user ID has the same value as it would
// be used as extsvc.Account.AccountID. The returned list includes both direct access
// and inherited from the organization membership.
//
// This method may return partial but valid results in case of error, and it is up to
// callers to decide whether to discard.
//
// API docs: https://developer.github.com/v4/object/repositorycollaboratorconnection/
func (p *Provider) FetchRepoPerms(ctx context.Context, repo *extsvc.Repository, opts authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	if repo == nil {
		return nil, errors.New("no repository provided")
	} else if !extsvc.IsHostOfRepo(p.codeHost, &repo.ExternalRepoSpec) {
		return nil, errors.Errorf("not a code host of the repository: want %q but have %q",
			repo.ServiceID, p.codeHost.ServiceID)
	}

	// NOTE: We do not store port or scheme in our URI, so stripping the hostname alone is enough.
	nameWithOwner := strings.TrimPrefix(repo.URI, p.codeHost.BaseURL.Hostname())
	nameWithOwner = strings.TrimPrefix(nameWithOwner, "/")

	owner, name, err := github.SplitRepositoryNameWithOwner(nameWithOwner)
	if err != nil {
		return nil, errors.Wrap(err, "split nameWithOwner")
	}

	// 100 matches the maximum page size, thus a good default to avoid multiple allocations
	// when appending the first 100 results to the slice.
	const userPageSize = 100

	var (
		// userIDs tracks users with access to this repo
		userIDs = make([]extsvc.AccountID, 0, userPageSize)
		// seenUsers helps deduplication of userIDs for groupsCache. Left unset indicates
		// it is unused.
		seenUsers map[extsvc.AccountID]struct{}
		// addUserToRepoPerms checks if the given users are already tracked before adding
		// it to perms for groupsCache, otherwise just adds directly
		addUserToRepoPerms func(users ...extsvc.AccountID)
		// affiliations to list for - groupCache only lists for a subset. Left unset indicates
		// all affiliations should be sync'd.
		affiliation github.CollaboratorAffiliation
	)

	// If cache is disabled the code path is simpler, avoid allocating memory
	if p.groupsCache == nil { // groups cache is disabled
		// addUserToRepoPerms just adds to perms.
		addUserToRepoPerms = func(users ...extsvc.AccountID) {
			userIDs = append(userIDs, users...)
		}
	} else { // groups cache is enabled
		// instantiate map to help with deduplication
		seenUsers = make(map[extsvc.AccountID]struct{}, userPageSize)
		// addUserToRepoPerms checks if the given users are already tracked before adding it to perms.
		addUserToRepoPerms = func(users ...extsvc.AccountID) {
			for _, user := range users {
				if _, exists := seenUsers[user]; !exists {
					seenUsers[user] = struct{}{}
					userIDs = append(userIDs, user)
				}
			}
		}
		// If groups caching is enabled, we sync just direct affiliations, and sync org/team
		// collaborators separately from cache
		affiliation = github.AffiliationDirect
	}

	client, err := p.client()
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}

	// Sync collaborators
	hasNextPage := true
	for page := 1; hasNextPage; page++ {
		var err error
		var users []*github.Collaborator
		users, hasNextPage, err = client.ListRepositoryCollaborators(ctx, owner, name, page, affiliation)
		if err != nil {
			return userIDs, errors.Wrap(err, "list users for repo")
		}

		for _, u := range users {
			userID := strconv.FormatInt(u.DatabaseID, 10)

			addUserToRepoPerms(extsvc.AccountID(userID))
		}
	}

	// If groups caching is disabled, we are done.
	if p.groupsCache == nil {
		return userIDs, nil
	}

	// Get groups affiliated with this repo.
	groups, err := p.getRepoAffiliatedGroups(ctx, owner, name, opts)
	if err != nil {
		return userIDs, errors.Wrap(err, "get groups affiliated with repo")
	}

	// Perform a fresh sync with groups that need a sync.
	repoID := extsvc.RepoID(repo.ID)
	for _, group := range groups {
		// If this is a partial cache, add self to group
		if len(group.Repositories) > 0 {
			hasRepo := false
			for _, repo := range group.Repositories {
				if repo == repoID {
					hasRepo = true
					break
				}
			}
			if !hasRepo {
				group.Repositories = append(group.Repositories, repoID)
				p.groupsCache.setGroup(group.cachedGroup)
			}
		}

		// Just use cache if available and not invalidated and continue
		if len(group.Users) > 0 {
			addUserToRepoPerms(group.Users...)
			continue
		}

		// Perform full sync
		hasNextPage := true
		for page := 1; hasNextPage; page++ {
			var members []*github.Collaborator
			if group.Team == "" {
				members, hasNextPage, err = client.ListOrganizationMembers(ctx, owner, page, group.adminsOnly)
			} else {
				members, hasNextPage, err = client.ListTeamMembers(ctx, owner, group.Team, page)
			}
			if err != nil {
				return userIDs, errors.Wrap(err, "list users for group")
			}
			for _, u := range members {
				// Add results to both group (for persistence) and permissions for user
				accountID := extsvc.AccountID(strconv.FormatInt(u.DatabaseID, 10))
				group.Users = append(group.Users, accountID)
				addUserToRepoPerms(accountID)
			}
		}

		// Persist group
		p.groupsCache.setGroup(group.cachedGroup)
	}

	return userIDs, nil
}

// getUserAffiliatedGroups retrieves affiliated organizations and teams for the given client
// with token. Returned groups are populated from cache if a valid value is available.
//
// 🚨 SECURITY: clientWithToken must be authenticated with a user token.
func (p *Provider) getUserAffiliatedGroups(ctx context.Context, clientWithToken client, opts authz.FetchPermsOptions) ([]cachedGroup, error) {
	groups := make([]cachedGroup, 0)
	seenGroups := make(map[string]struct{})

	// syncGroup adds the given group to the list of groups to cache, pulling values from
	// cache where available.
	syncGroup := func(org, team string) {
		if team != "" {
			// If a team's repos is a subset of an organization's, don't sync. Because when an organization
			// has at least default read permissions, a team's repos will always be a strict subset
			// of the organization's.
			if _, exists := seenGroups[team]; exists {
				return
			}
		}
		cachedPerms, exists := p.groupsCache.getGroup(org, team)
		if exists && opts.InvalidateCaches {
			// invalidate this cache
			p.groupsCache.invalidateGroup(&cachedPerms)
		}
		seenGroups[cachedPerms.key()] = struct{}{}
		groups = append(groups, cachedPerms)
	}
	var err error

	// Get orgs
	hasNextPage := true
	for page := 1; hasNextPage; page++ {
		var orgs []github.OrgDetailsAndMembership
		orgs, hasNextPage, _, err = clientWithToken.GetAuthenticatedUserOrgsDetailsAndMembership(ctx, page)
		if err != nil {
			return groups, err
		}
		for _, org := range orgs {
			// 🚨 SECURITY: Iff THIS USER can view this org's repos, we add the entire org to the sync list
			if canViewOrgRepos(&org) {
				syncGroup(org.Login, "")
			}
		}
	}

	// Get teams
	hasNextPage = true
	for page := 1; hasNextPage; page++ {
		var teams []*github.Team
		teams, hasNextPage, _, err = clientWithToken.GetAuthenticatedUserTeams(ctx, page)
		if err != nil {
			return groups, err
		}
		for _, team := range teams {
			// only sync teams with repos
			if team.ReposCount > 0 && team.Organization != nil {
				syncGroup(team.Organization.Login, team.Slug)
			}
		}
	}

	return groups, nil
}

type repoAffiliatedGroup struct {
	cachedGroup
	// Whether this affiliation is an admin-only affiliation rather than a group-wide
	// affiliation - affects how a sync is conducted.
	adminsOnly bool
}

// getRepoAffiliatedGroups retrieves affiliated organizations and teams for the given repository.
// Returned groups are populated from cache if a valid value is available.
func (p *Provider) getRepoAffiliatedGroups(ctx context.Context, owner, name string, opts authz.FetchPermsOptions) (groups []repoAffiliatedGroup, err error) {
	client, err := p.client()
	if err != nil {
		return nil, errors.Wrap(err, "get client")
	}

	// Check if repo belongs in an org
	org, err := client.GetOrganization(ctx, owner)
	if err != nil {
		if github.IsNotFound(err) {
			// Owner is most likely not an org. User repos don't have teams or org permissions,
			// so we are done - this is fine, so don't propagate error.
			return groups, nil
		}
		return
	}

	// indicate if a group should be sync'd
	syncGroup := func(owner, team string, adminsOnly bool) {
		group, exists := p.groupsCache.getGroup(owner, team)
		if exists && opts.InvalidateCaches {
			// invalidate this cache
			p.groupsCache.invalidateGroup(&group)
		}
		groups = append(groups, repoAffiliatedGroup{cachedGroup: group, adminsOnly: adminsOnly})
	}

	// If this repo is an internal repo, we want to allow everyone in the org to read this repo
	// (provided the temporary feature flag is set) irrespective of the user being an admin or not.
	isRepoInternallyVisible := false

	// The visibility field on a repo is only returned if this feature flag is set. As a result
	// there's no point in making an extra API call if this feature flag is not set explicitly.
	if p.enableGithubInternalRepoVisibility {
		var r *github.Repository
		r, err = client.GetRepository(ctx, owner, name)
		if err != nil {
			// Maybe the repo doesn't belong to this org? Or Another error occurred in trying to get the
			// repo. Either way, we are not going to syncGroup for this repo.
			return
		}

		if org != nil && r.Visibility == github.VisibilityInternal {
			isRepoInternallyVisible = true
		}
	}

	allOrgMembersCanRead := isRepoInternallyVisible || canViewOrgRepos(&github.OrgDetailsAndMembership{OrgDetails: org})
	if allOrgMembersCanRead {
		// 🚨 SECURITY: Iff all members of this org can view this repo, indicate that all members should
		// be sync'd.
		syncGroup(owner, "", false)
	} else {
		// 🚨 SECURITY: Sync *only admins* of this org
		syncGroup(owner, "", true)

		// Also check for teams involved in repo, and indicate all groups should be sync'd.
		hasNextPage := true
		for page := 1; hasNextPage; page++ {
			var teams []*github.Team
			teams, hasNextPage, err = client.ListRepositoryTeams(ctx, owner, name, page)
			if err != nil {
				return
			}
			for _, t := range teams {
				syncGroup(owner, t.Slug, false)
			}
		}
	}

	return groups, nil
}
