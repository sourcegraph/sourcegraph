package sourcegraphoperator

import (
	"path"

	feAuth "github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/openidconnect"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/schema"
)

// provider is an implementation of providers.Provider for the Sourcegraph
// Operator authentication, also referred to as "SOAP". There can only ever be
// one provider of this type, and it can only be provisioned through Cloud site
// configuration (see github.com/sourcegraph/sourcegraph/internal/cloud)
//
// SOAP is used to provision accounts for Sourcegraph teammates in Sourcegraph
// Cloud - for more details, refer to
// https://handbook.sourcegraph.com/departments/cloud/technical-docs/oidc_site_admin/#faq
type provider struct {
	config cloud.SchemaAuthProviderSourcegraphOperator
	*openidconnect.Provider
}

// NewProvider creates and returns a new Sourcegraph Operator authentication
// provider using the given config.
func NewProvider(config cloud.SchemaAuthProviderSourcegraphOperator, cli httpcli.Doer) providers.Provider {
	allowSignUp := true
	return &provider{
		config: config,
		Provider: openidconnect.NewProvider(
			schema.OpenIDConnectAuthProvider{
				AllowSignup:        &allowSignUp,
				ClientID:           config.ClientID,
				ClientSecret:       config.ClientSecret,
				ConfigID:           auth.SourcegraphOperatorProviderType,
				DisplayName:        "Sourcegraph Operators",
				Issuer:             config.Issuer,
				RequireEmailDomain: "sourcegraph.com",
				Type:               auth.SourcegraphOperatorProviderType,
			},
			authPrefix,
			path.Join(feAuth.AuthURLPrefix, "sourcegraph-operator", "callback"),
			cli,
		).(*openidconnect.Provider),
	}
}

// Config implements providers.Provider.
func (p *provider) Config() schema.AuthProviders {
	// NOTE: Intentionally omitting rest of the information unless absolutely
	// necessary because this provider is configured at the infrastructure level, and
	// those fields may expose sensitive information should not be visible to
	// non-Sourcegraph employees.
	return schema.AuthProviders{
		Openidconnect: &schema.OpenIDConnectAuthProvider{
			ConfigID:    auth.SourcegraphOperatorProviderType,
			DisplayName: "Sourcegraph Operators",
		},
	}
}
