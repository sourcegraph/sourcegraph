package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/gerrit"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/github"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers/perforce"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ProvidersFromConfig returns the set of permission-related providers derived from the site config
// based on `NewAuthzProviders` constructors provided by each provider type's package.
//
// It also returns any simple validation problems with the config, separating these into "serious problems"
// and "warnings". "Serious problems" are those that should make Sourcegraph set authz.allowAccessByDefault
// to false. "Warnings" are all other validation problems.
//
// This constructor does not and should not directly check connectivity to external services - if
// desired, callers should use `(*Provider).ValidateConnection` directly to get warnings related
// to connection issues.
func ProvidersFromConfig(
	ctx context.Context,
	cfg conftypes.SiteConfigQuerier,
	db database.DB,
) (
	allowAccessByDefault bool,
	providers []authz.Provider,
	seriousProblems []string,
	warnings []string,
	invalidConnections []string,
) {
	logger := log.Scoped("authz")

	allowAccessByDefault = true
	defer func() {
		if len(seriousProblems) > 0 {
			logger.Error("Repository authz config was invalid (errors are visible in the UI as an admin user, you should fix ASAP). Restricting access to repositories by default for now to be safe.", log.Strings("seriousProblems", seriousProblems))
			allowAccessByDefault = false
		}
	}()

	opt := database.ExternalServicesListOptions{
		Kinds: []string{
			extsvc.VariantAzureDevOps.AsKind(),
			extsvc.VariantBitbucketCloud.AsKind(),
			extsvc.VariantBitbucketServer.AsKind(),
			extsvc.VariantGerrit.AsKind(),
			extsvc.VariantGitHub.AsKind(),
			extsvc.VariantGitLab.AsKind(),
			extsvc.VariantPerforce.AsKind(),
		},
		LimitOffset: &database.LimitOffset{
			Limit: 500, // The number is randomly chosen
		},
	}

	var (
		gitHubConns          []*github.ExternalConnection
		gitLabConns          []*types.GitLabConnection
		bitbucketServerConns []*types.BitbucketServerConnection
		perforceConns        []*types.PerforceConnection
		bitbucketCloudConns  []*types.BitbucketCloudConnection
		gerritConns          []*types.GerritConnection
		azuredevopsConns     []*types.AzureDevOpsConnection
	)
	for {
		svcs, err := db.ExternalServices().List(ctx, opt)
		if err != nil {
			seriousProblems = append(seriousProblems, fmt.Sprintf("Could not list external services: %v", err))
			break
		}
		if len(svcs) == 0 {
			break // No more results, exiting
		}
		opt.AfterID = svcs[len(svcs)-1].ID // Advance the cursor

		for _, svc := range svcs {
			if svc.CloudDefault { // Only public repos in CloudDefault services
				continue
			}

			cfg, err := extsvc.ParseEncryptableConfig(ctx, svc.Kind, svc.Config)
			if err != nil {
				seriousProblems = append(seriousProblems, fmt.Sprintf("Could not parse config of external service %d: %v", svc.ID, err))
				continue
			}

			switch c := cfg.(type) {
			case *schema.AzureDevOpsConnection:
				azuredevopsConns = append(azuredevopsConns, &types.AzureDevOpsConnection{
					URN:                   svc.URN(),
					AzureDevOpsConnection: c,
				})
			case *schema.BitbucketCloudConnection:
				bitbucketCloudConns = append(bitbucketCloudConns, &types.BitbucketCloudConnection{
					URN:                      svc.URN(),
					BitbucketCloudConnection: c,
				})
			case *schema.BitbucketServerConnection:
				bitbucketServerConns = append(bitbucketServerConns, &types.BitbucketServerConnection{
					URN:                       svc.URN(),
					BitbucketServerConnection: c,
				})
			case *schema.GerritConnection:
				gerritConns = append(gerritConns, &types.GerritConnection{
					URN:              svc.URN(),
					GerritConnection: c,
				})
			case *schema.GitHubConnection:
				gitHubConns = append(gitHubConns,
					&github.ExternalConnection{
						ExternalService: svc,
						GitHubConnection: &types.GitHubConnection{
							URN:              svc.URN(),
							GitHubConnection: c,
						},
					},
				)
			case *schema.GitLabConnection:
				gitLabConns = append(gitLabConns, &types.GitLabConnection{
					URN:              svc.URN(),
					GitLabConnection: c,
				})
			case *schema.PerforceConnection:
				perforceConns = append(perforceConns, &types.PerforceConnection{
					URN:                svc.URN(),
					PerforceConnection: c,
				})
			default:
				logger.Error("ProvidersFromConfig", log.Error(errors.Errorf("unexpected connection type: %T", cfg)))
				continue
			}
		}

		if len(svcs) < opt.Limit {
			break // Less results than limit means we've reached end
		}
	}

	enableGithubInternalRepoVisibility := false
	ef := cfg.SiteConfig().ExperimentalFeatures
	if ef != nil {
		enableGithubInternalRepoVisibility = ef.EnableGithubInternalRepoVisibility
	}

	initResult := github.NewAuthzProviders(ctx, db, gitHubConns, cfg.SiteConfig().AuthProviders, enableGithubInternalRepoVisibility)
	initResult.Append(gitlab.NewAuthzProviders(db, cfg.SiteConfig(), gitLabConns))
	initResult.Append(bitbucketserver.NewAuthzProviders(bitbucketServerConns))
	initResult.Append(perforce.NewAuthzProviders(perforceConns))
	initResult.Append(bitbucketcloud.NewAuthzProviders(db, bitbucketCloudConns, cfg.SiteConfig().AuthProviders))
	initResult.Append(gerrit.NewAuthzProviders(gerritConns, cfg.SiteConfig().AuthProviders))
	initResult.Append(azuredevops.NewAuthzProviders(db, azuredevopsConns, httpcli.NewExternalClientFactory()))

	return allowAccessByDefault, initResult.Providers, initResult.Problems, initResult.Warnings, initResult.InvalidConnections
}

func RefreshInterval() time.Duration {
	interval := conf.Get().AuthzRefreshInterval
	if interval <= 0 {
		return 5 * time.Second
	}
	return time.Duration(interval) * time.Second
}

// PermissionSyncingDisabled returns true if the background permissions syncing is not enabled.
// It is not enabled if:
//   - There are no code host connections with authorization or enforcePermissions enabled
//   - Not purchased with the current license
//   - `disableAutoCodeHostSyncs` site setting is set to true
func PermissionSyncingDisabled() bool {
	_, p := authz.GetProviders()
	return len(p) == 0 ||
		licensing.Check(licensing.FeatureACLs) != nil ||
		conf.Get().DisableAutoCodeHostSyncs
}

var ValidateExternalServiceConfig = database.MakeValidateExternalServiceConfigFunc(
	[]database.GitHubValidatorFunc{github.ValidateAuthz},
	[]database.GitLabValidatorFunc{gitlab.ValidateAuthz},
	[]database.BitbucketServerValidatorFunc{bitbucketserver.ValidateAuthz},
	[]database.PerforceValidatorFunc{perforce.ValidateAuthz},
	[]database.AzureDevOpsValidatorFunc{func(_ *schema.AzureDevOpsConnection) error { return nil }},
) // TODO: @varsanojidan switch this with actual authz once its implemented.
