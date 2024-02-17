package openidconnect

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"github.com/coreos/go-oidc"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/globals"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const providerType = "openidconnect"

// Provider is an implementation of providers.Provider for the OpenID Connect
// authentication.
type Provider struct {
	config      schema.OpenIDConnectAuthProvider
	authPrefix  string
	callbackUrl string
	httpClient  httpcli.Doer

	mu         sync.Mutex
	oidc       *oidcProvider
	refreshErr error
}

// NewProvider creates and returns a new OpenID Connect authentication provider
// using the given config.
func NewProvider(config schema.OpenIDConnectAuthProvider, authPrefix string, callbackUrl string, httpClient httpcli.Doer) providers.Provider {
	return &Provider{
		config:      config,
		authPrefix:  authPrefix,
		callbackUrl: callbackUrl,
		httpClient:  httpClient,
	}
}

// ConfigID implements providers.Provider.
func (p *Provider) ConfigID() providers.ConfigID {
	return providers.ConfigID{
		Type: p.config.Type,
		ID:   providerConfigID(&p.config),
	}
}

// Config implements providers.Provider.
func (p *Provider) Config() schema.AuthProviders {
	return schema.AuthProviders{Openidconnect: &p.config}
}

// Refresh implements providers.Provider.
func (p *Provider) Refresh(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.oidc, p.refreshErr = newOIDCProvider(p.config.Issuer, p.httpClient)
	return p.refreshErr
}

func (p *Provider) ExternalAccountInfo(ctx context.Context, account extsvc.Account) (*extsvc.PublicAccountData, error) {
	return GetPublicExternalAccountData(ctx, &account.AccountData)
}

// oidcVerifier returns the token verifier of the underlying OIDC provider.
func (p *Provider) oidcVerifier() *oidc.IDTokenVerifier {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.oidc.Verifier(
		&oidc.Config{
			ClientID: p.config.ClientID,
		},
	)
}

// oidcUserInfo returns the user info using the given token source from the
// underlying OIDC provider.
func (p *Provider) oidcUserInfo(ctx context.Context, tokenSource oauth2.TokenSource) (*oidc.UserInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.oidc.UserInfo(ctx, tokenSource)
}

func (p *Provider) getCachedInfoAndError() (*providers.Info, error) {
	info := providers.Info{
		ServiceID:   p.config.Issuer,
		ClientID:    p.config.ClientID,
		DisplayName: p.config.DisplayName,
		AuthenticationURL: (&url.URL{
			Path:     path.Join(p.authPrefix, "login"),
			RawQuery: (url.Values{"pc": []string{providerConfigID(&p.config)}}).Encode(),
		}).String(),
	}
	if info.DisplayName == "" {
		info.DisplayName = "OpenID Connect"
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.refreshErr
	if err != nil {
		err = errors.WithMessage(err, "failed to initialize OpenID Connect auth provider")
	} else if p.oidc == nil {
		err = errors.New("OpenID Connect auth provider is not yet initialized")
	}
	return &info, err
}

// CachedInfo implements providers.Provider.
func (p *Provider) CachedInfo() *providers.Info {
	info, _ := p.getCachedInfoAndError()
	return info
}

// oauth2Config constructs and returns an *oauth2.Config from the provider.
func (p *Provider) oauth2Config() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,

		// It would be nice if this was "/.auth/openidconnect/callback" not "/.auth/callback", but
		// many instances have the "/.auth/callback" value hardcoded in their external auth
		// provider, so we can't change it easily
		RedirectURL: globals.ExternalURL().
			ResolveReference(&url.URL{Path: p.callbackUrl}).
			String(),

		Endpoint: p.oidc.Endpoint(),
		Scopes:   []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

// oidcProvider is an OpenID Connect oidcProvider with additional claims parsed from the service oidcProvider
// discovery response (beyond what github.com/coreos/go-oidc parses by default).
type oidcProvider struct {
	oidc.Provider
	providerExtraClaims
}

type providerExtraClaims struct {
	// EndSessionEndpoint is the URL of the OP's endpoint that logs the user out of the OP (provided
	// in the "end_session_endpoint" field of the OP's service discovery response). See
	// https://openid.net/specs/openid-connect-session-1_0.html#OPMetadata.
	EndSessionEndpoint string `json:"end_session_endpoint,omitempty"`

	// RevocationEndpoint is the URL of the OP's revocation endpoint (provided in the
	// "revocation_endpoint" field of the OP's service discovery response). See
	// https://openid.net/specs/openid-heart-openid-connect-1_0.html#rfc.section.3.5 and
	// https://tools.ietf.org/html/rfc7009.
	RevocationEndpoint string `json:"revocation_endpoint,omitempty"`
}

var mockNewProvider func(issuerURL string) (*oidcProvider, error)

func newOIDCProvider(issuerURL string, httpClient httpcli.Doer) (*oidcProvider, error) {
	if mockNewProvider != nil {
		return mockNewProvider(issuerURL)
	}

	cli, err := httpcli.NewExternalClientFactory().Client()
	if err != nil {
		return nil, err
	}

	bp, err := oidc.NewProvider(oidc.ClientContext(context.Background(), cli), issuerURL)
	if err != nil {
		return nil, err
	}

	p := &oidcProvider{Provider: *bp}
	if err := bp.Claims(&p.providerExtraClaims); err != nil {
		return nil, err
	}
	return p, nil
}

// revokeToken implements Token Revocation. See https://tools.ietf.org/html/rfc7009.
func revokeToken(ctx context.Context, p *Provider, accessToken, tokenType string) error {
	postData := url.Values{}
	postData.Set("token", accessToken)
	if tokenType != "" {
		postData.Set("token_type_hint", tokenType)
	}
	req, err := http.NewRequest(
		"POST",
		p.oidc.RevocationEndpoint,
		strings.NewReader(postData.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(p.config.ClientID, p.config.ClientSecret)

	resp, err := p.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf(
			"non-200 HTTP response from token revocation endpoint %s: HTTP %d",
			p.oidc.RevocationEndpoint,
			resp.StatusCode,
		)
	}
	return nil
}
