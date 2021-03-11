package saml

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	saml2 "github.com/russellhaering/gosaml2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

type authnResponseInfo struct {
	spec                 extsvc.AccountSpec
	email, displayName   string
	unnormalizedUsername string
	accountData          interface{}
}

func readAuthnResponse(p *provider, encodedResp string) (*authnResponseInfo, error) {
	{
		if raw, err := base64.StdEncoding.DecodeString(encodedResp); err == nil {
			traceLog(fmt.Sprintf("AuthnResponse: %s", p.ConfigID().ID), string(raw))
		}
	}

	assertions, err := p.samlSP.RetrieveAssertionInfo(encodedResp)
	if err != nil {
		return nil, errors.WithMessage(err, "reading AuthnResponse assertions")
	}
	if wi := assertions.WarningInfo; wi.InvalidTime || wi.NotInAudience {
		return nil, fmt.Errorf("invalid SAML AuthnResponse: %+v", wi)
	}

	pi, err := p.getCachedInfoAndError()
	if err != nil {
		return nil, err
	}

	firstNonempty := func(ss ...string) string {
		for _, s := range ss {
			if s := strings.TrimSpace(s); s != "" {
				return s
			}
		}
		return ""
	}
	attr := samlAssertionValues(assertions.Values)
	email := firstNonempty(attr.Get("email"), attr.Get("emailaddress"), attr.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"), attr.Get("http://schemas.xmlsoap.org/claims/EmailAddress"))
	if email == "" && mightBeEmail(assertions.NameID) {
		email = assertions.NameID
	}
	if pn := attr.Get("eduPersonPrincipalName"); email == "" && mightBeEmail(pn) {
		email = pn
	}
	info := authnResponseInfo{
		spec: extsvc.AccountSpec{
			ServiceType: providerType,
			ServiceID:   pi.ServiceID,
			ClientID:    pi.ClientID,
			AccountID:   assertions.NameID,
		},
		email:                email,
		unnormalizedUsername: firstNonempty(attr.Get("login"), attr.Get("uid"), attr.Get("username"), attr.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"), email),
		displayName:          firstNonempty(attr.Get("displayName"), attr.Get("givenName")+" "+attr.Get("surname"), attr.Get("http://schemas.xmlsoap.org/claims/CommonName"), attr.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname")),
		accountData:          assertions,
	}
	if assertions.NameID == "" {
		return nil, errors.New("the SAML response did not contain a valid NameID")
	}
	if info.email == "" {
		return nil, errors.New("the SAML response did not contain an email attribute")
	}
	if info.unnormalizedUsername == "" {
		return nil, errors.New("the SAML response did not contain a username attribute")
	}
	return &info, nil
}

// getOrCreateUser gets or creates a user account based on the SAML claims. It returns the
// authenticated actor if successful; otherwise it returns an friendly error message (safeErrMsg)
// that is safe to display to users, and a non-nil err with lower-level error details.
func getOrCreateUser(ctx context.Context, allowSignup bool, info *authnResponseInfo) (_ *actor.Actor, safeErrMsg string, err error) {
	var data extsvc.AccountData
	data.SetAccountData(info.accountData)

	username, err := auth.NormalizeUsername(info.unnormalizedUsername)
	if err != nil {
		return nil, fmt.Sprintf("Error normalizing the username %q. See https://docs.sourcegraph.com/admin/auth/#username-normalization.", info.unnormalizedUsername), err
	}

	userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, auth.GetAndSaveUserOp{
		UserProps: database.NewUser{
			Username:        username,
			Email:           info.email,
			EmailIsVerified: info.email != "", // SAML emails are assumed to be verified
			DisplayName:     info.displayName,
			// SAML has no standard way of providing an avatar URL.
		},
		ExternalAccount:     info.spec,
		ExternalAccountData: data,
		CreateIfNotExist:    allowSignup,
	})
	if err != nil {
		return nil, safeErrMsg, err
	}
	return actor.FromUser(userID), "", nil
}

func mightBeEmail(s string) bool {
	return strings.Count(s, "@") == 1
}

type samlAssertionValues saml2.Values

func (v samlAssertionValues) Get(key string) string {
	for _, a := range v {
		if a.Name == key || a.FriendlyName == key {
			return a.Values[0].Value
		}
	}
	return ""
}
