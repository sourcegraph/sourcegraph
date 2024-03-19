package openidconnect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

var mockGetProviderValue *Provider

// GetProvider looks up the registered OpenID Connect authentication provider
// with the given ID. It returns nil if no such provider exists.
func GetProvider(id string) *Provider {
	if mockGetProviderValue != nil {
		return mockGetProviderValue
	}
	p, ok := providers.GetProviderByConfigID(
		providers.ConfigID{
			Type: providerType,
			ID:   id,
		},
	).(*Provider)
	if ok {
		p.callbackUrl = path.Join(auth.AuthURLPrefix, "callback")
		return p
	}
	return nil
}

var errNoSuchProvider = errors.New("no such authentication provider")

// GetProviderAndRefresh retrieves the authentication provider with the given
// type and ID, and refreshes the token used by the provider.
func GetProviderAndRefresh(ctx context.Context, id string, getProvider func(id string) *Provider) (p *Provider, safeErrMsg string, err error) {
	p = getProvider(id)
	if p == nil {
		return nil,
			"Misconfigured authentication provider.",
			errNoSuchProvider
	}
	if p.config.Issuer == "" {
		return nil,
			"Misconfigured authentication provider.",
			errors.Errorf("No issuer set for authentication provider with ID %q (set the authentication provider's issuer property).", p.ConfigID())
	}
	if err = p.Refresh(ctx); err != nil {
		return nil,
			"Unexpected error refreshing authentication provider. This may be due to an incorrect issuer URL. Check the logs for more details.",
			errors.Wrapf(err, "refreshing authentication provider with ID %q", p.ConfigID())
	}
	return p, "", nil
}

func Init() {
	conf.ContributeValidator(validateConfig)

	const pkgName = "openidconnect"
	logger := log.Scoped(pkgName)
	go func() {
		conf.Watch(func() {
			ps := getProviders()
			if len(ps) == 0 {
				providers.Update(pkgName, nil)
				return
			}

			if err := licensing.Check(licensing.FeatureSSO); err != nil {
				logger.Error("Check license for SSO (OpenID Connect)", log.Error(err))
				providers.Update(pkgName, nil)
				return
			}

			for _, p := range ps {
				go func() {
					if err := p.Refresh(context.Background()); err != nil {
						logger.Error("Error prefetching OpenID Connect service provider metadata.", log.Error(err))
					}
				}()
			}
			providers.Update(pkgName, ps)
		})
	}()
}

func getProviders() []providers.Provider {
	var cfgs []*schema.OpenIDConnectAuthProvider
	for _, p := range conf.Get().AuthProviders {
		if p.Openidconnect == nil {
			continue
		}
		cfgs = append(cfgs, p.Openidconnect)
	}
	ps := make([]providers.Provider, 0, len(cfgs))
	for _, cfg := range cfgs {
		ps = append(ps, NewProvider(*cfg, authPrefix, path.Join(auth.AuthURLPrefix, "callback"), httpcli.ExternalClient))
	}
	return ps
}

func validateConfig(c conftypes.SiteConfigQuerier) (problems conf.Problems) {
	var loggedNeedsExternalURL bool
	for _, p := range c.SiteConfig().AuthProviders {
		if p.Openidconnect != nil && c.SiteConfig().ExternalURL == "" && !loggedNeedsExternalURL {
			problems = append(problems, conf.NewSiteProblem("openidconnect auth provider requires `externalURL` to be set to the external URL of your site (example: https://sourcegraph.example.com)"))
			loggedNeedsExternalURL = true
		}
	}

	seen := map[schema.OpenIDConnectAuthProvider]int{}
	for i, p := range c.SiteConfig().AuthProviders {
		if p.Openidconnect != nil {
			if j, ok := seen[*p.Openidconnect]; ok {
				problems = append(problems, conf.NewSiteProblem(fmt.Sprintf("OpenID Connect auth provider at index %d is duplicate of index %d, ignoring", i, j)))
			} else {
				seen[*p.Openidconnect] = i
			}
		}
	}

	return problems
}

// providerConfigID produces a semi-stable identifier for an openidconnect auth provider config
// object. It is used to distinguish between multiple auth providers of the same type when in
// multi-step auth flows. Its value is never persisted, and it must be deterministic.
func providerConfigID(pc *schema.OpenIDConnectAuthProvider) string {
	if pc.ConfigID != "" {
		return pc.ConfigID
	}
	data, err := json.Marshal(pc)
	if err != nil {
		panic(err)
	}
	b := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(b[:16])
}
