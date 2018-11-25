package openidconnect

import (
	"context"
	"fmt"

	oidc "github.com/coreos/go-oidc"
	"github.com/pkg/errors"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/db"
	"github.com/sourcegraph/sourcegraph/pkg/actor"
	"github.com/sourcegraph/sourcegraph/pkg/extsvc"
)

// getOrCreateUser gets or creates a user account based on the OpenID Connect token. It returns the
// authenticated actor if successful; otherwise it returns an friendly error message (safeErrMsg)
// that is safe to display to users, and a non-nil err with lower-level error details.
func getOrCreateUser(ctx context.Context, p *provider, idToken *oidc.IDToken, userInfo *oidc.UserInfo, claims *userClaims) (_ *actor.Actor, safeErrMsg string, err error) {
	if userInfo.Email == "" {
		return nil, "Only users with an email address may authenticate to Sourcegraph.", errors.New("no email address in claims")
	}
	if unverifiedEmail := claims.EmailVerified != nil && !*claims.EmailVerified; unverifiedEmail {
		// If the OP explicitly reports `"email_verified": false`, then reject the authentication
		// attempt. If undefined or true, then it will be allowed.
		return nil, fmt.Sprintf("Only users with verified email addresses may authenticate to Sourcegraph. The email address %q is not verified on the external authentication provider.", userInfo.Email), fmt.Errorf("refusing unverified user email address %q", userInfo.Email)
	}

	pi, err := p.getCachedInfoAndError()
	if err != nil {
		return nil, "", err
	}

	login := claims.PreferredUsername
	if login == "" {
		login = userInfo.Email
	}
	email := userInfo.Email
	var displayName = claims.GivenName
	if displayName == "" {
		if claims.Name == "" {
			displayName = claims.Name
		} else {
			displayName = login
		}
	}
	login, err = auth.NormalizeUsername(login)
	if err != nil {
		return nil, fmt.Sprintf("Error normalizing the username %q. See https://docs.sourcegraph.com/admin/auth/#username-normalization.", login), err
	}

	var data extsvc.ExternalAccountData
	data.SetAccountData(struct {
		IDToken    *oidc.IDToken  `json:"idToken"`
		UserInfo   *oidc.UserInfo `json:"userInfo"`
		UserClaims *userClaims    `json:"userClaims"`
	}{IDToken: idToken, UserInfo: userInfo, UserClaims: claims})

	userID, safeErrMsg, err := auth.CreateOrUpdateUser(ctx, db.NewUser{
		Username:        login,
		Email:           email,
		EmailIsVerified: email != "", // TODO(sqs): https://github.com/sourcegraph/sourcegraph/issues/10118
		DisplayName:     displayName,
		AvatarURL:       claims.Picture,
	}, extsvc.ExternalAccountSpec{
		ServiceType: providerType,
		ServiceID:   pi.ServiceID,
		ClientID:    pi.ClientID,
		AccountID:   idToken.Subject,
	}, data)
	if err != nil {
		return nil, safeErrMsg, err
	}
	return actor.FromUser(userID), "", nil
}
