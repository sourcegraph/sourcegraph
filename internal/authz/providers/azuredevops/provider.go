package azuredevops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	authztypes "github.com/sourcegraph/sourcegraph/internal/authz/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/oauthtoken"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var mockServerURL string

func NewAuthzProviders(db database.DB, conns []*types.AzureDevOpsConnection, cf *httpcli.Factory) *authztypes.ProviderInitResult {
	orgs, projects := map[string]struct{}{}, map[string]struct{}{}

	authorizedConnections := []*types.AzureDevOpsConnection{}

	// Iterate over all Azure Dev Ops code host connections to make sure we sync permissions for all
	// orgs and projects in every permissions sync iteration.
	for _, c := range conns {
		if !c.EnforcePermissions {
			continue
		}

		// The list of orgs and projects may have duplicates if there are multiple Azure DevOps code
		// host connections that have the same project in their config.
		//
		// Add them to a map so that we may filter out duplicates before passing them over to the
		// provider.
		for _, name := range c.Orgs {
			orgs[name] = struct{}{}
		}

		for _, name := range c.Projects {
			projects[name] = struct{}{}
		}

		c := c
		authorizedConnections = append(authorizedConnections, c)
	}

	initResults := &authztypes.ProviderInitResult{}
	if len(authorizedConnections) == 0 {
		return initResults
	}

	p, err := newAuthzProvider(db, authorizedConnections, orgs, projects, cf)
	if err != nil {
		initResults.InvalidConnections = append(initResults.InvalidConnections, extsvc.TypeAzureDevOps)
		initResults.Problems = append(initResults.Problems, err.Error())
	} else if p != nil {
		initResults.Providers = append(initResults.Providers, p)
	}

	return initResults
}

func newAuthzProvider(db database.DB, conns []*types.AzureDevOpsConnection, orgs, projects map[string]struct{}, cf *httpcli.Factory) (*Provider, error) {
	if err := licensing.Check(licensing.FeatureACLs); err != nil {
		return nil, err
	}

	u, err := url.Parse(azuredevops.AzureDevOpsAPIURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse url: %q, this is likely a misconfigured URL in the constant azuredevops.AzureDevOpsAPIURL", azuredevops.AzureDevOpsAPIURL)
	}

	return &Provider{
		db:       db,
		urn:      "azuredevops:authzprovider",
		conns:    conns,
		codeHost: extsvc.NewCodeHost(u, extsvc.TypeAzureDevOps),
		orgs:     orgs,
		projects: projects,
		cf:       cf,
	}, nil
}

type Provider struct {
	db database.DB

	urn      string
	codeHost *extsvc.CodeHost

	conns []*types.AzureDevOpsConnection

	// orgs is the set of orgs as configured across all the code host connections.
	orgs map[string]struct{}
	// projects is the set of projects as configured across all the code host connections.
	projects map[string]struct{}
	cf       *httpcli.Factory
}

func (p *Provider) FetchAccount(_ context.Context, _ *types.User, _ []*extsvc.Account, _ []string) (*extsvc.Account, error) {
	return nil, nil
}

func (p *Provider) FetchUserPerms(ctx context.Context, account *extsvc.Account, _ authz.FetchPermsOptions) (*authz.ExternalUserPermissions, error) {
	logger := log.Scoped("azuredevops.FetchuserPerms")
	logger.Debug("starting FetchUserPerms", log.String("user ID", fmt.Sprintf("%#v", account.UserID)))

	profile, token, err := azuredevops.GetExternalAccountData(ctx, &account.AccountData)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to load external account data from database with external account with ID: %d",
			account.ID,
		)
	}

	oauthToken := &auth.OAuthBearerToken{
		Token:              token.AccessToken,
		RefreshToken:       token.RefreshToken,
		Expiry:             token.Expiry,
		NeedsRefreshBuffer: 5,
	}

	oauthContext, err := azuredevops.GetOAuthContext(token.RefreshToken)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate oauth context, this is likely a misconfiguration with the Azure OAuth provider (bad URL?), please check the auth.providers configuration in your site config")
	}

	oauthToken.RefreshFunc = oauthtoken.GetAccountRefreshAndStoreOAuthTokenFunc(p.db.UserExternalAccounts(), account.ID, oauthContext)

	var apiURL string
	if mockServerURL != "" {
		apiURL = mockServerURL
	} else {
		apiURL = azuredevops.AzureDevOpsAPIURL
	}

	client, err := azuredevops.NewClient(
		p.ServiceID(),
		apiURL,
		oauthToken,
		p.cf,
	)
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"failed to create client for service ID: %q, account ID: %q", p.ServiceID(), account.AccountID,
		)
	}

	var repos []azuredevops.Repository
	var orgs []azuredevops.Org

	var userProfile azuredevops.Profile
	if profile == nil {
		userProfile, err = client.GetAuthorizedProfile(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		userProfile = *profile
	}

	// Always list the orgs accessible by this user, even if no orgs are specified in the Azure
	// DevOps code host connection. The code host connection may have have only projects, but
	// listing the user's orgs will help us with discovery of all accessible repos of this user.
	orgs, err = client.ListAuthorizedUserOrganizations(ctx, userProfile)
	if err != nil {
		return nil, err
	}

	allOrgs := map[string]struct{}{}
	for name := range p.orgs {
		allOrgs[name] = struct{}{}
	}

	for project := range p.projects {
		// A project here is a in the format <org-name>/<project-name>. An org or project name
		// itself cannot contain any `/` so we are guaranteed that splitting the string with `/`
		// will return two strings.
		parts := strings.Split(project, "/")

		// Consequently, this should never happen but just in case, log it as a warning instead
		// of a hard failure.
		if len(parts) != 2 {
			logger.Warn(
				"Unexpected project name found in Azure DevOps authorization provider (this likely means a misconfigured item in the `projects` key of one of the Azure DevOps code host connections, please check the code host config). Permissions syncing for this user will not be 100% complete and they may not have access to some repos on Sourcegraph that they can access on Azure DevOps.",
				log.String("project", project), log.String("user", profile.EmailAddress),
			)
			continue
		}

		// Add the org name. If the org is already listed in the `orgs` key of the code host
		// connection, then this is safe. If it is not, then we will now have tracked it for the
		// next step of the user's permissions sync.
		allOrgs[parts[0]] = struct{}{}
	}

	for _, org := range orgs {
		// The user may have access to more orgs than those listed in an Azure DevOps code host
		// connection through the `orgs` or `projects` keys.
		// Do not sync this org.
		if _, ok := allOrgs[org.Name]; !ok {
			logger.Debug("skipping org as it is not set in code host configuration", log.String("org", org.Name))
			continue
		}

		logger.Debug("listing repos", log.String("org", org.Name))

		foundRepos, err := client.ListRepositoriesByProjectOrOrg(ctx, azuredevops.ListRepositoriesByProjectOrOrgArgs{
			ProjectOrOrgName: org.Name,
		})
		if err != nil {
			if httpErr, ok := err.(*azuredevops.HTTPError); ok {
				// If the HTTPError is 401 / 403 / 404, this user does not have access to this org.
				// Skip and continue to the next.
				//
				// For orgs that don't exist, the API returns 404. For orgs that the user does not
				// have access to the API returns 401. We're not sure if the API might return 403
				// for some use case but we don't want to hard fail on that either.
				if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden || httpErr.StatusCode == http.StatusNotFound {

					logger.Debug("user does not have access to this org",
						log.String("org", org.Name),
						log.Int("http status code", httpErr.StatusCode),
					)

					continue
				}
			}

			// For any other errors, we want to hard fail so that the issue can be identified.
			return nil, errors.Newf("failed to list repositories for org: %q with error: %q", org, err.Error())
		}

		logger.Debug("adding repos", log.Int("count", len(foundRepos)))
		repos = append(repos, foundRepos...)
	}

	extIDs := make([]extsvc.RepoID, 0, len(repos))
	for _, repo := range repos {
		extIDs = append(extIDs, extsvc.RepoID(repo.ID))
	}

	return &authz.ExternalUserPermissions{
		Exacts: extIDs,
	}, err
}

// FetchRepoPerms remains unimplemented for Azure DevOps.
//
// Repo permissions syncing is a three step process with the Azure DevOps API:
// 1. Trigger a permissions report for a specific repo
// 2. Check the status of the permissions report (and maybe backoff and check again until the report is generated)
// 3. Download the report and parse it to generate the permissions table
//
// This makes the entire process per repo fragile and cumbersome. Repo syncing could be unreliable and may not scale very well in terms of rate limits if we have to make at least three API requests per repo.
//
// As a result, we prefer incremental user permissions sync instead.
func (p *Provider) FetchRepoPerms(_ context.Context, _ *extsvc.Repository, _ authz.FetchPermsOptions) ([]extsvc.AccountID, error) {
	return nil, authz.ErrUnimplemented{Feature: "azuredevops.FetchRepoPerms"}
}

func (p *Provider) ServiceType() string {
	return p.codeHost.ServiceType
}

func (p *Provider) ServiceID() string {
	return p.codeHost.ServiceID
}

func (p *Provider) URN() string {
	return p.urn
}

func (p *Provider) ValidateConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	allErrors := []string{}
	for _, conn := range p.conns {
		client, err := azuredevops.NewClient(
			p.ServiceID(),
			azuredevops.AzureDevOpsAPIURL,
			&auth.BasicAuth{
				Username: conn.Username,
				Password: conn.Token,
			},
			p.cf,
		)
		if err != nil {
			allErrors = append(allErrors, fmt.Sprintf("%s:%s", conn.URN, err.Error()))
			continue
		}

		_, err = client.GetAuthorizedProfile(ctx)
		if err != nil {
			allErrors = append(allErrors, err.Error())
		}
	}

	if len(allErrors) > 0 {
		msg := strings.Join(allErrors, "\n")
		return errors.Newf("ValidateConnection failed for Azure DevOps with the following errors: %s", msg)
	}

	return nil
}
