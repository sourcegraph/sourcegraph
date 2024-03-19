package saml

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	stdlog "log" //nolint:logging // TODO move all logging to sourcegraph/log
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/schema"
)

var mockGetProviderValue *provider

// getProvider looks up the registered saml auth provider with the given ID.
func getProvider(pcID string) *provider {
	if mockGetProviderValue != nil {
		return mockGetProviderValue
	}

	p, _ := providers.GetProviderByConfigID(providers.ConfigID{Type: providerType, ID: pcID}).(*provider)
	if p != nil {
		return p
	}

	// Special case: if there is only a single SAML auth provider, return it regardless of the pcID.
	for _, ap := range providers.Providers() {
		if ap.Config().Saml != nil {
			if p != nil {
				return nil // multiple SAML providers, can't use this special case
			}
			p = ap.(*provider)
		}
	}

	return p
}

func handleGetProvider(ctx context.Context, w http.ResponseWriter, pcID string) (p *provider, handled bool) {
	p = getProvider(pcID)
	if p == nil {
		log15.Error("No SAML auth provider found with ID", "id", pcID)
		http.Error(w, "Misconfigured SAML auth provider", http.StatusInternalServerError)
		return nil, true
	}
	if err := p.Refresh(ctx); err != nil {
		log15.Error("Error getting SAML auth provider", "id", p.ConfigID(), "error", err)
		http.Error(w, "Unexpected error getting SAML authentication provider. This may indicate that the SAML IdP does not exist. Ask a site admin to check the server \"frontend\" logs for \"Error getting SAML auth provider\".", http.StatusInternalServerError)
		return nil, true
	}
	return p, false
}

func Init() {
	conf.ContributeValidator(validateConfig)

	const pkgName = "saml"
	logger := log.Scoped(pkgName)
	go func() {
		conf.Watch(func() {
			ps := getProviders()
			if len(ps) == 0 {
				providers.Update(pkgName, nil)
				return
			}

			if err := licensing.Check(licensing.FeatureSSO); err != nil {
				logger.Error("Check license for SSO (SAML)", log.Error(err))
				providers.Update(pkgName, nil)
				return
			}

			for _, p := range ps {
				go func() {
					if err := p.Refresh(context.Background()); err != nil {
						logger.Error("Error prefetching SAML service provider metadata.", log.Error(err))
					}
				}()
			}
			providers.Update(pkgName, ps)
		})
	}()
}

func getProviders() []providers.Provider {
	var cfgs []*schema.SAMLAuthProvider
	for _, p := range conf.Get().AuthProviders {
		if p.Saml == nil {
			continue
		}
		cfgs = append(cfgs, withConfigDefaults(p.Saml))
	}
	multiple := len(cfgs) >= 2
	ps := make([]providers.Provider, 0, len(cfgs))
	for _, cfg := range cfgs {
		p := &provider{config: *cfg, multiple: multiple, httpClient: httpcli.ExternalClient}
		ps = append(ps, p)
	}
	return ps
}

func validateConfig(c conftypes.SiteConfigQuerier) (problems conf.Problems) {
	var loggedNeedsExternalURL bool
	for _, p := range c.SiteConfig().AuthProviders {
		if p.Saml != nil && c.SiteConfig().ExternalURL == "" && !loggedNeedsExternalURL {
			problems = append(problems, conf.NewSiteProblem("saml auth provider requires `externalURL` to be set to the external URL of your site (example: https://sourcegraph.example.com)"))
			loggedNeedsExternalURL = true
		}
	}

	seen := map[string]int{}
	for i, p := range c.SiteConfig().AuthProviders {
		if p.Saml != nil {
			// we can ignore errors: converting to JSON must work, as we parsed from JSON before
			bytes, _ := json.Marshal(*p.Saml)
			key := string(bytes)
			if j, ok := seen[key]; ok {
				problems = append(problems, conf.NewSiteProblem(fmt.Sprintf("SAML auth provider at index %d is duplicate of index %d, ignoring", i, j)))
			} else {
				seen[key] = i
			}
		}
	}

	return problems
}

func withConfigDefaults(pc *schema.SAMLAuthProvider) *schema.SAMLAuthProvider {
	if pc.ServiceProviderIssuer == "" {
		externalURL := conf.Get().ExternalURL
		if externalURL == "" {
			// An empty issuer will be detected as an error later.
			return pc
		}

		// Derive default issuer from externalURL.
		tmp := *pc
		tmp.ServiceProviderIssuer = strings.TrimSuffix(externalURL, "/") + path.Join(authPrefix, "metadata")
		return &tmp
	}
	return pc
}

func getNameIDFormat(pc *schema.SAMLAuthProvider) string {
	// Persistent is best because users will reuse their user_external_accounts row instead of (as
	// with transient) creating a new one each time they authenticate.
	const defaultNameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	if pc.NameIDFormat != "" {
		return pc.NameIDFormat
	}
	return defaultNameIDFormat
}

// providerConfigID produces a semi-stable identifier for a saml auth provider config object. It is
// used to distinguish between multiple auth providers of the same type when in multi-step auth
// flows. Its value is never persisted, and it must be deterministic.
//
// If there is only a single saml auth provider, it returns the empty string because that satisfies
// the requirements above.
func providerConfigID(pc *schema.SAMLAuthProvider, multiple bool) string {
	if pc.ConfigID != "" {
		return pc.ConfigID
	}
	if !multiple {
		return ""
	}
	data, err := json.Marshal(pc)
	if err != nil {
		panic(err)
	}
	b := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(b[:16])
}

var traceLogEnabled, _ = strconv.ParseBool(env.Get("INSECURE_SAML_LOG_TRACES", "false", "Log all SAML requests and responses. Only use during testing because the log messages will contain sensitive data."))

func traceLog(description, body string) {
	if traceLogEnabled {
		const n = 40
		stdlog.Printf("%s SAML trace: %s\n%s\n%s", strings.Repeat("=", n), description, body, strings.Repeat("=", n+len(description)+1))
	}
}
