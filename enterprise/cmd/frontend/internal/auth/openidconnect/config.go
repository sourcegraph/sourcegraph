package openidconnect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

var mockGetProviderValue *provider

// getProvider looks up the registered openidconnect auth provider with the given ID.
func getProvider(id string) *provider {
	if mockGetProviderValue != nil {
		return mockGetProviderValue
	}
	p, _ := providers.GetProviderByConfigID(providers.ConfigID{Type: providerType, ID: id}).(*provider)
	return p
}

func handleGetProvider(ctx context.Context, w http.ResponseWriter, id string) (p *provider, handled bool) {
	handled = true // safer default

	p = getProvider(id)
	if p == nil {
		log15.Error("No OpenID Connect auth provider found with ID.", "id", id)
		http.Error(w, "Misconfigured OpenID Connect auth provider.", http.StatusInternalServerError)
		return nil, true
	}
	if p.config.Issuer == "" {
		log15.Error("No issuer set for OpenID Connect auth provider (set the openidconnect auth provider's issuer property).", "id", p.ConfigID())
		http.Error(w, "Misconfigured OpenID Connect auth provider.", http.StatusInternalServerError)
		return nil, true
	}
	if err := p.Refresh(ctx); err != nil {
		log15.Error("Error refreshing OpenID Connect auth provider.", "id", p.ConfigID(), "error", err)
		http.Error(w, "Unexpected error refreshing OpenID Connect authentication provider. This may be due to an incorrect issuer URL. Check the logs for more details", http.StatusInternalServerError)
		return nil, true
	}
	return p, false
}

func init() {
	conf.ContributeValidator(validateConfig)
}

func validateConfig(c conf.Unified) (problems conf.Problems) {
	var loggedNeedsExternalURL bool
	for _, p := range c.AuthProviders {
		if p.Openidconnect != nil && c.ExternalURL == "" && !loggedNeedsExternalURL {
			problems = append(problems, conf.NewSiteProblem("openidconnect auth provider requires `externalURL` to be set to the external URL of your site (example: https://sourcegraph.example.com)"))
			loggedNeedsExternalURL = true
		}
	}

	seen := map[schema.OpenIDConnectAuthProvider]int{}
	for i, p := range c.AuthProviders {
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
