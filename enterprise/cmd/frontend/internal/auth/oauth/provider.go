package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"

	"github.com/dghubble/gologin"
	goauth2 "github.com/dghubble/gologin/oauth2"
	"github.com/inconshreveable/log15"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/schema"
)

type Provider struct {
	ProviderOp

	Login    http.Handler
	Callback http.Handler
}

var _ providers.Provider = (*Provider)(nil)

// GetProvider returns a provider with given serviceType and ID. It returns nil
// if no such provider.
func GetProvider(serviceType, id string) *Provider {
	p, ok := providers.GetProviderByConfigID(providers.ConfigID{Type: serviceType, ID: id}).(*Provider)
	if !ok {
		return nil
	}
	return p
}

func (p *Provider) ConfigID() providers.ConfigID {
	return providers.ConfigID{
		ID:   p.ServiceID + "::" + p.OAuth2Config.ClientID,
		Type: p.ServiceType,
	}
}

func (p *Provider) Config() schema.AuthProviders {
	return p.SourceConfig
}

func (p *Provider) CachedInfo() *providers.Info {
	displayName := p.ServiceID
	switch {
	case p.SourceConfig.Github != nil && p.SourceConfig.Github.DisplayName != "":
		displayName = p.SourceConfig.Github.DisplayName
	case p.SourceConfig.Gitlab != nil && p.SourceConfig.Gitlab.DisplayName != "":
		displayName = p.SourceConfig.Gitlab.DisplayName
	}
	return &providers.Info{
		ServiceID:   p.ServiceID,
		ClientID:    p.OAuth2Config.ClientID,
		DisplayName: displayName,
		AuthenticationURL: (&url.URL{
			Path:     path.Join(p.AuthPrefix, "login"),
			RawQuery: (url.Values{"pc": []string{p.ConfigID().ID}}).Encode(),
		}).String(),
	}
}

func (p *Provider) Refresh(ctx context.Context) error {
	return nil
}

type ProviderOp struct {
	AuthPrefix   string
	OAuth2Config oauth2.Config
	SourceConfig schema.AuthProviders
	StateConfig  gologin.CookieConfig
	ServiceID    string
	ServiceType  string
	Login        http.Handler
	Callback     http.Handler
}

func NewProvider(op ProviderOp) *Provider {
	providerID := op.ServiceID + "::" + op.OAuth2Config.ClientID
	return &Provider{
		ProviderOp: op,
		Login:      stateHandler(true, providerID, op.StateConfig, op.Login),
		Callback:   stateHandler(false, providerID, op.StateConfig, op.Callback),
	}
}

// stateHandler decodes the state from the gologin cookie and sets it in the context. It checked by
// some downstream handler to ensure equality with the value of the state URL param.
//
// This is very similar to gologin's default StateHandler function, but we define our own, because
// we encode the returnTo URL in the state. We could use the `redirect_uri` parameter to do this,
// but doing so would require using Sourcegraph's external hostname and making sure it is consistent
// with what is specified in the OAuth app config as the "callback URL."
func stateHandler(isLogin bool, providerID string, config gologin.CookieConfig, success http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		csrf, err := randomState()
		if err != nil {
			log15.Error("Failed to generated random state", "error", err)
			http.Error(w, "Failed to generate random state", http.StatusInternalServerError)
			return
		}
		if isLogin {
			redirect, err := getRedirect(req)
			if err != nil {
				log15.Error("Failed to parse URL from Referrer header", "error", err)
				http.Error(w, "Failed to parse URL from Referrer header.", http.StatusInternalServerError)
				return
			}
			// add Cookie with a random state + redirect
			stateVal, err := LoginState{
				Redirect:   redirect,
				CSRF:       csrf,
				ProviderID: providerID,
				Op:         LoginStateOp(req.URL.Query().Get("op")),
			}.Encode()
			if err != nil {
				log15.Error("Could not encode OAuth state", "error", err)
				http.Error(w, "Could not encode OAuth state.", http.StatusInternalServerError)
				return
			}
			http.SetCookie(w, NewCookie(config, stateVal))
			ctx = goauth2.WithState(ctx, stateVal)
		} else if cookie, err := req.Cookie(config.Name); err == nil { // not login and cookie exists
			// add the cookie state to the ctx
			ctx = goauth2.WithState(ctx, cookie.Value)
		}
		success.ServeHTTP(w, req.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

type LoginStateOp string

const (
	// NOTE: OAuth is almost always used for creating new accounts, therefore we don't need a special name for it.
	LoginStateOpCreateAccount            LoginStateOp = ""
	LoginStateOpCreateCodeHostConnection LoginStateOp = "createCodeHostConnection"
)

type LoginState struct {
	// Redirect is the URL path to redirect to after login.
	Redirect string

	// ProviderID is the service ID of the provider that is handling the auth flow.
	ProviderID string

	// CSRF is the random string that ensures the encoded state is sufficiently random to be checked
	// for CSRF purposes.
	CSRF string

	// Op is the operation to be done after OAuth flow. The default operation is to create a new account.
	Op LoginStateOp
}

func (s LoginState) Encode() (string, error) {
	sb, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sb), nil
}

func DecodeState(encoded string) (*LoginState, error) {
	var s LoginState
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(decoded, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Returns a base64 encoded random 32 byte string.
func randomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// if we have a redirect param use that, otherwise we'll try and pull
// the 'returnTo' param from the referrer URL, this is usually the login
// page where the user has been dumped to after following a link.
func getRedirect(req *http.Request) (string, error) {
	redirect := req.URL.Query().Get("redirect")
	if redirect != "" {
		return redirect, nil
	}
	referer := req.Referer()
	if referer == "" {
		return "", nil
	}
	referrerURL, err := url.Parse(referer)
	if err != nil {
		return "", err
	}
	returnTo := referrerURL.Query().Get("returnTo")
	// to prevent open redirect vulnerabilities used for phishing
	// we limit the redirect URL to only permit certain urls
	if !canRedirect(returnTo) {
		return "", fmt.Errorf("invalid URL in returnTo parameter: %s", returnTo)
	}
	return returnTo, nil
}

// canRedirect is used to limit the set of URLs we will redirect to
// after login to prevent open redirect exploits for things like phishing
func canRedirect(redirect string) bool {
	unescaped, err := url.QueryUnescape(redirect)
	if err != nil {
		return false
	}
	redirectURL, err := url.Parse(unescaped)
	if err != nil {
		return false
	}
	// if we have a non-relative url, make sure it's the same host as the sourcegraph instance
	if redirectURL.Host != "" && redirectURL.Host != globals.ExternalURL().Host {
		return false
	}
	// TODO: do we want to exclude any internal paths here?
	return true
}
