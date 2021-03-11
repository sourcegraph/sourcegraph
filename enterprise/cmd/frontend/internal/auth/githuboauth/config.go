package githuboauth

import (
	"github.com/dghubble/gologin"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

func init() {
	const pkgName = "githuboauth"
	conf.ContributeValidator(func(cfg conf.Unified) conf.Problems {
		_, problems := parseConfig(&cfg)
		return problems
	})
	go func() {
		conf.Watch(func() {
			newProviders, _ := parseConfig(conf.Get())
			if len(newProviders) == 0 {
				providers.Update(pkgName, nil)
			} else {
				newProvidersList := make([]providers.Provider, 0, len(newProviders))
				for _, p := range newProviders {
					newProvidersList = append(newProvidersList, p.Provider)
				}
				providers.Update(pkgName, newProvidersList)
			}
		})
	}()
}

type Provider struct {
	*schema.GitHubAuthProvider
	providers.Provider
}

func parseConfig(cfg *conf.Unified) (ps []Provider, problems conf.Problems) {
	for _, pr := range cfg.AuthProviders {
		if pr.Github == nil {
			continue
		}

		provider, providerProblems := parseProvider(pr.Github, pr)
		problems = append(problems, conf.NewSiteProblems(providerProblems...)...)
		if provider != nil {
			ps = append(ps, Provider{
				GitHubAuthProvider: pr.Github,
				Provider:           provider,
			})
		}
	}
	return ps, problems
}

func getStateConfig() gologin.CookieConfig {
	cfg := gologin.CookieConfig{
		Name:     "github-state-cookie",
		Path:     "/",
		MaxAge:   120, // 120 seconds
		HTTPOnly: true,
		Secure:   conf.IsExternalURLSecure(),
	}
	return cfg
}
