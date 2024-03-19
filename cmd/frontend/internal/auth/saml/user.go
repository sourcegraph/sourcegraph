package saml

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	saml2 "github.com/russellhaering/gosaml2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type authnResponseInfo struct {
	spec                 extsvc.AccountSpec
	email, displayName   string
	unnormalizedUsername string
	groups               map[string]bool
	accountData          any
}

func readAuthnResponse(p *provider, encodedResp string) (*authnResponseInfo, error) {
	if raw, err := base64.StdEncoding.DecodeString(encodedResp); err == nil {
		traceLog(fmt.Sprintf("AuthnResponse: %s", p.ConfigID().ID), string(raw))
	}

	assertions, err := p.samlSP.RetrieveAssertionInfo(encodedResp)
	if err != nil {
		return nil, errors.WithMessage(err, "reading AuthnResponse assertions")
	}
	if wi := assertions.WarningInfo; wi.InvalidTime || wi.NotInAudience {
		return nil, errors.Errorf("invalid SAML AuthnResponse: %+v", wi)
	}

	if assertions.NameID == "" {
		return nil, errors.New("the SAML response did not contain a valid NameID")
	}

	attr := samlAssertionValues(assertions.Values)
	email := attr.getEmail(assertions)

	if email == "" {
		return nil, errors.New("the SAML response did not contain an email attribute")
	}

	unnormalizedUsername := attr.getUnnormalizedUsername(email, p.config.UsernameAttributeNames)
	if unnormalizedUsername == "" {
		return nil, errors.New("the SAML response did not contain a username attribute")
	}

	pi, err := p.getCachedInfoAndError()
	if err != nil {
		return nil, err
	}

	groupsAttr := firstNonEmpty(p.config.GroupsAttributeName, "groups")

	info := &authnResponseInfo{
		spec: extsvc.AccountSpec{
			ServiceType: providerType,
			ServiceID:   pi.ServiceID,
			ClientID:    pi.ClientID,
			AccountID:   assertions.NameID,
		},
		email:                email,
		unnormalizedUsername: unnormalizedUsername,
		displayName:          attr.getDisplayName(),
		groups:               attr.GetMap(groupsAttr),
		accountData:          assertions,
	}
	return info, nil
}

// getOrCreateUser gets or creates a user account based on the SAML claims. It returns the
// authenticated actor if successful; otherwise it returns an friendly error message (safeErrMsg)
// that is safe to display to users, and a non-nil err with lower-level error details.
func getOrCreateUser(ctx context.Context, db database.DB, allowSignup bool, info *authnResponseInfo) (newUserCreated bool, _ *actor.Actor, safeErrMsg string, err error) {
	var data extsvc.AccountData
	if err := SetExternalAccountData(&data, info); err != nil {
		return false, nil, "", err
	}

	username, err := auth.NormalizeUsername(info.unnormalizedUsername)
	if err != nil {
		return false, nil, fmt.Sprintf("Error normalizing the username %q. See https://sourcegraph.com/docs/admin/auth/#username-normalization.", info.unnormalizedUsername), err
	}

	newUserCreated, userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, db, auth.GetAndSaveUserOp{
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
		return false, nil, safeErrMsg, err
	}
	return newUserCreated, actor.FromUser(userID), "", nil
}

func mightBeEmail(s string) bool {
	return strings.Count(s, "@") == 1
}

// firstNonEmpty returns the first string in the list that's not
// empty or entirely whitespace.
//
// If no non-empty string is found, an empty string is returned.
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s := strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
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

func (v samlAssertionValues) getFirstNonEmpty(keys []string) string {
	for _, key := range keys {
		if s := v.Get(key); s != "" {
			return s
		}
	}

	return ""
}

func (v samlAssertionValues) GetMap(key string) map[string]bool {
	for _, a := range v {
		if a.Name == key || a.FriendlyName == key {
			output := make(map[string]bool)
			for _, v := range a.Values {
				output[v.Value] = true
			}
			return output
		}
	}
	return nil
}

// getEmail returns an email address from samlAssertionValues and
// saml2.AssertionInfo in the following order of preference:
// 1. "email"
// 2. "emailaddress"
// 3. "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
// 4. "http://schemas.xmlsoap.org/claims/EmailAddress"
// 5. "eduPersonPrincipalName"
// 6. assertions.NameID
func (v samlAssertionValues) getEmail(assertions *saml2.AssertionInfo) string {
	email := firstNonEmpty(
		v.Get("email"),
		v.Get("emailaddress"),
		v.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"),
		v.Get("http://schemas.xmlsoap.org/claims/EmailAddress"),
	)

	if email != "" {
		return email
	}

	principalName := v.Get("eduPersonPrincipalName")
	if mightBeEmail(principalName) {
		return principalName
	}

	if mightBeEmail(assertions.NameID) {
		return assertions.NameID
	}

	return email
}

// getUnnormalizedUsername returns a username from samlAssertionValues.
// If usernameKeys is provided, that list of keys will be checked in order.
// Otherwise, a username is selected in the following order of preference:
// 1. "login"
// 2. "uid"
// 3. "username"
// 4. "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
// 5. email
func (v samlAssertionValues) getUnnormalizedUsername(email string, usernameKeys []string) string {
	if len(usernameKeys) > 0 {
		return v.getFirstNonEmpty(usernameKeys)
	}

	return firstNonEmpty(
		v.Get("login"),
		v.Get("uid"),
		v.Get("username"),
		v.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"),
		email,
	)
}

// getDisplayName returns a username from samlAssertionValues in the following
// order of preference:
// 1. "displayName"
// 2. "givenName" + " " + "surname"
// 3. "http://schemas.xmlsoap.org/claims/CommonName"
// 4. "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
func (v samlAssertionValues) getDisplayName() string {
	return firstNonEmpty(
		v.Get("displayName"),
		v.Get("givenName")+" "+v.Get("surname"),
		v.Get("http://schemas.xmlsoap.org/claims/CommonName"),
		v.Get("http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"),
	)
}

type SAMLValues struct {
	Values map[string]SAMLAttribute `json:"Values,omitempty"`
}

type SAMLAttribute struct {
	Values []SAMLValue `json:"Values"`
}

type SAMLValue struct {
	Value string
}

// GetExternalAccountData returns the deserialized JSON blob from user external accounts table
func GetExternalAccountData(ctx context.Context, data *extsvc.AccountData) (val *SAMLValues, err error) {
	if data.Data != nil {
		val, err = encryption.DecryptJSON[SAMLValues](ctx, data.Data)
		if err != nil {
			return nil, err
		}
	}
	if val == nil {
		return nil, errors.New("could not find data for the external account")
	}

	return val, nil
}

func GetPublicExternalAccountData(ctx context.Context, accountData *extsvc.AccountData) (*extsvc.PublicAccountData, error) {
	data, err := GetExternalAccountData(ctx, accountData)
	if err != nil {
		return nil, err
	}

	values := data.Values
	if values == nil {
		return nil, errors.New("could not find data values for external account")
	}

	// convert keys to lower case for case insensitive matching of candidates
	lowerCaseValues := make(map[string]SAMLAttribute, len(values))
	for k, v := range values {
		lowerCaseValues[strings.ToLower(k)] = v
	}

	var displayName string
	// all candidates are lower case
	candidates := []string{
		"nickname",
		"login",
		"username",
		"name",
		"http://schemas.xmlsoap.org/claims/name",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"email",
		"emailaddress",
		"http://schemas.xmlsoap.org/claims/emailaddress",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	}
	for _, key := range candidates {
		candidate, ok := lowerCaseValues[key]
		if ok && len(candidate.Values) > 0 && candidate.Values[0].Value != "" {
			displayName = candidate.Values[0].Value
			break
		}
	}
	if displayName == "" {
		return nil, nil
	}
	return &extsvc.PublicAccountData{
		DisplayName: displayName,
	}, nil
}

// SetExternalAccountData sets the user and token into the external account data blob.
func SetExternalAccountData(data *extsvc.AccountData, info *authnResponseInfo) error {
	// TODO: leverage the whole info object instead of just storing JSON blob without any structure
	serializedData, err := json.Marshal(info.accountData)
	if err != nil {
		return err
	}

	data.Data = extsvc.NewUnencryptedData(serializedData)
	return nil
}
