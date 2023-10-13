package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"

	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/service/servegit"
)

func (r *siteResolver) NeedsRepositoryConfiguration(ctx context.Context) (bool, error) {
	if envvar.SourcegraphDotComMode() {
		return false, nil
	}

	// 🚨 SECURITY: The site alerts may contain sensitive data, so only site
	// admins may view them.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		// TODO(dax): This should return err once the site flags query is fixed for users
		return false, nil
	}

	return needsRepositoryConfiguration(ctx, r.db)
}

func needsRepositoryConfiguration(ctx context.Context, db database.DB) (bool, error) {
	kinds := make([]string, 0, len(database.ExternalServiceKinds))
	for kind, config := range database.ExternalServiceKinds {
		if config.CodeHost {
			kinds = append(kinds, kind)
		}
	}

	if deploy.IsApp() {
		// In the Cody app, we need repository configuration iff:
		//
		// 1. The user has not configured extsvc (number of extsvc excluding the autogenerated one is equal to zero.)
		// 2. The autogenerated extsvc did not discover any local repositories.
		//
		services, err := db.ExternalServices().List(ctx, database.ExternalServicesListOptions{
			Kinds: kinds,
		})
		if err != nil {
			return false, err
		}
		count := 0
		for _, svc := range services {
			if svc.ID == servegit.ExtSVCID {
				continue
			}
			count++
		}
		if count != 0 {
			// User has configured extsvc, no configuration needed.
			return false, nil
		}

		// We need configuration if autogenerated extsvc did not find any repos
		numRepos, err := db.ExternalServices().RepoCount(ctx, servegit.ExtSVCID)
		if err != nil {
			// Assume configuration is needed. It's possible the autogenerated extsvc doesn't exist
			// for some reason, or just hasn't been created yet (race condition.)
			return true, nil
		}
		return numRepos == 0, nil
	}

	count, err := db.ExternalServices().Count(ctx, database.ExternalServicesListOptions{
		Kinds: kinds,
	})
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (*siteResolver) SendsEmailVerificationEmails() bool { return conf.EmailVerificationRequired() }

func (r *siteResolver) FreeUsersExceeded(ctx context.Context) (bool, error) {
	if envvar.SourcegraphDotComMode() {
		return false, nil
	}

	// If a license exists, warnings never need to be shown.
	if info, err := GetConfiguredProductLicenseInfo(); info != nil && !IsFreePlan(info) {
		return false, err
	}
	// If OSS, warnings never need to be shown.
	if NoLicenseWarningUserCount == nil {
		return false, nil
	}

	userCount, err := r.db.Users().Count(
		ctx,
		&database.UsersListOptions{
			ExcludeSourcegraphOperators: true,
		},
	)
	if err != nil {
		return false, err
	}

	return *NoLicenseWarningUserCount <= int32(userCount), nil
}

func (r *siteResolver) ExternalServicesFromFile() bool { return envvar.ExtsvcConfigFile() != "" }
func (r *siteResolver) AllowEditExternalServicesWithFile() bool {
	return envvar.ExtsvcConfigAllowEdits()
}
