package saml

import (
	"fmt"
	"net/http"

	"github.com/beevik/etree"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

// SignOut returns the URL where the user can initiate a logout from the SAML IdentityProvider, if
// it has a SingleLogoutService.
func SignOut(w http.ResponseWriter, r *http.Request) (logoutURL string, err error) {
	// TODO(sqs): Only supports a single SAML auth provider.
	pc, multiple := getFirstProviderConfig()
	if pc == nil {
		return "", nil
	}
	p := getProvider(providerConfigID(pc, multiple))
	if p == nil {
		return "", nil
	}

	doc, err := newLogoutRequest(p)
	if err != nil {
		return "", errors.WithMessage(err, "creating SAML LogoutRequest")
	}
	{
		if data, err := doc.WriteToString(); err == nil {
			traceLog(fmt.Sprintf("LogoutRequest: %s", p.ConfigID().ID), data)
		}
	}
	return p.samlSP.BuildAuthURLRedirect("/", doc)
}

// getFirstProviderConfig returns the SAML auth provider config. At most 1 can be specified in site
// config; if there is more than 1, it returns multiple == true (which the caller should handle by
// returning an error and refusing to proceed with auth).
func getFirstProviderConfig() (pc *schema.SAMLAuthProvider, multiple bool) {
	for _, p := range conf.Get().AuthProviders {
		if p.Saml != nil {
			if pc != nil {
				return pc, true // multiple SAML auth providers
			}
			pc = withConfigDefaults(p.Saml)
		}
	}
	return pc, false
}

func newLogoutRequest(p *provider) (*etree.Document, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.samlSP == nil {
		return nil, errors.New("unable to create SAML LogoutRequest because provider is not yet initialized")
	}

	// Start with the doc for AuthnRequest and change a few things to make it into a LogoutRequest
	// doc. This saves us from needing to duplicate a bunch of code.
	doc, err := p.samlSP.BuildAuthRequestDocumentNoSig()
	if err != nil {
		return nil, err
	}
	root := doc.Root()
	root.Tag = "LogoutRequest"
	// TODO(sqs): This assumes SSO URL == SLO URL (i.e., the same endpoint is used for signin and
	// logout). To fix this, use `root.SelectAttr("Destination").Value = "..."`.
	if t := root.FindElement("//samlp:NameIDPolicy"); t != nil {
		root.RemoveChild(t)
	}

	if p.samlSP.SignAuthnRequests {
		signed, err := p.samlSP.SignAuthnRequest(root)
		if err != nil {
			return nil, err
		}
		doc.SetRoot(signed)
	}
	return doc, nil
}
