package bitbucketserveroauth

import (
	"fmt"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/schema"
)

func Init(logger log.Logger, db database.DB) {
	const pkgName = "bitbucketserveroauth"
	logger = logger.Scoped(pkgName)
	conf.ContributeValidator(func(cfg conftypes.SiteConfigQuerier) conf.Problems {
		_, problems := parseConfig(logger, cfg, db)
		return problems
	})

	go conf.Watch(func() {
		newProviders, _ := parseConfig(logger, conf.Get(), db)
		if len(newProviders) == 0 {
			providers.Update(pkgName, nil)
			return
		}

		if err := licensing.Check(licensing.FeatureSSO); err != nil {
			logger.Error("Check license for SSO (Bitbucket Server OAuth)", log.Error(err))
			providers.Update(pkgName, nil)
			return
		}

		newProvidersList := make([]providers.Provider, 0, len(newProviders))
		for _, p := range newProviders {
			newProvidersList = append(newProvidersList, p.Provider)
		}
		providers.Update(pkgName, newProvidersList)
	})
}

type Provider struct {
	*schema.BitbucketServerAuthProvider
	providers.Provider
}

func parseConfig(logger log.Logger, cfg conftypes.SiteConfigQuerier, db database.DB) (ps []Provider, problems conf.Problems) {
	existingProviders := make(collections.Set[string])

	for _, pr := range cfg.SiteConfig().AuthProviders {
		if pr.Bitbucketserver == nil {
			continue
		}

		provider, providerProblems := parseProvider(logger, pr.Bitbucketserver, db, pr)
		problems = append(problems, conf.NewSiteProblems(providerProblems...)...)
		if provider == nil {
			continue
		}

		if existingProviders.Has(provider.CachedInfo().UniqueID()) {
			problems = append(problems, conf.NewSiteProblems(fmt.Sprintf(`Cannot have more than one Bitbucket Server auth provider with url %q and client ID %q, only the first one will be used`, provider.ServiceID, provider.CachedInfo().ClientID))...)
			continue
		}

		ps = append(ps, Provider{
			BitbucketServerAuthProvider: pr.Bitbucketserver,
			Provider:                    provider,
		})
		existingProviders.Add(provider.CachedInfo().UniqueID())
	}
	return ps, problems
}
