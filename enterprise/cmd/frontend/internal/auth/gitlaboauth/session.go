package gitlaboauth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot/hubspotutil"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type sessionIssuerHelper struct {
	*extsvc.CodeHost
	clientID string
}

func (s *sessionIssuerHelper) GetOrCreateUser(ctx context.Context, token *oauth2.Token, anonymousUserID, firstSourceURL string) (actr *actor.Actor, safeErrMsg string, err error) {
	gUser, err := UserFromContext(ctx)
	if err != nil {
		return nil, "Could not read GitLab user from callback request.", errors.Wrap(err, "could not read user from context")
	}

	login, err := auth.NormalizeUsername(gUser.Username)
	if err != nil {
		return nil, fmt.Sprintf("Error normalizing the username %q. See https://docs.sourcegraph.com/admin/auth/#username-normalization.", login), err
	}

	var data extsvc.AccountData
	gitlab.SetExternalAccountData(&data, gUser, token)

	// Unlike with GitHub, we can *only* use the primary email to resolve the user's identity,
	// because the GitLab API does not return whether an email has been verified. The user's primary
	// email on GitLab is always verified, so we use that.
	userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, auth.GetAndSaveUserOp{
		UserProps: database.NewUser{
			Username:        login,
			Email:           gUser.Email,
			EmailIsVerified: gUser.Email != "",
			DisplayName:     gUser.Name,
			AvatarURL:       gUser.AvatarURL,
		},
		ExternalAccount: extsvc.AccountSpec{
			ServiceType: s.ServiceType,
			ServiceID:   s.ServiceID,
			ClientID:    s.clientID,
			AccountID:   strconv.FormatInt(int64(gUser.ID), 10),
		},
		ExternalAccountData: data,
		CreateIfNotExist:    true,
	})
	if err != nil {
		return nil, safeErrMsg, err
	}

	// There is no need to send record if we know email is empty as it's a primary property
	if gUser.Email != "" {
		go hubspotutil.SyncUser(gUser.Email, hubspotutil.SignupEventID, &hubspot.ContactProperties{
			AnonymousUserID: anonymousUserID,
			FirstSourceURL:  firstSourceURL,
		})
	}
	return actor.FromUser(userID), "", nil
}

func (s *sessionIssuerHelper) CreateCodeHostConnection(ctx context.Context, token *oauth2.Token, providerID string) (safeErrMsg string, err error) {
	actor := actor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return "Must be authenticated to create code host connection from OAuth flow.", errors.New("unauthenticated request")
	}

	p := oauth.GetProvider(extsvc.TypeGitLab, providerID)
	if p == nil {
		return "Could not find OAuth provider for the state.", errors.Errorf("provider not found for %q", providerID)
	}

	gUser, err := UserFromContext(ctx)
	if err != nil {
		return "Could not read GitLab user from callback request.", errors.Wrap(err, "could not read user from context")
	}

	err = database.GlobalExternalServices.Create(ctx, conf.Get,
		&types.ExternalService{
			Kind:        extsvc.KindGitLab,
			DisplayName: fmt.Sprintf("GitLab (%s)", gUser.Username),
			Config: fmt.Sprintf(`
{
  "url": "%s",
  "token": "%s",
  "projectQuery": ["projects?visibility=public"]
}
`, p.ServiceID, token.AccessToken),
			NamespaceUserID: actor.UID,
		},
	)
	if err != nil {
		return "Could not create code host connection.", err
	}
	return "", nil // success
}

func (s *sessionIssuerHelper) DeleteStateCookie(w http.ResponseWriter) {
	stateConfig := getStateConfig()
	stateConfig.MaxAge = -1
	http.SetCookie(w, oauth.NewCookie(stateConfig, ""))
}

func (s *sessionIssuerHelper) SessionData(token *oauth2.Token) oauth.SessionData {
	return oauth.SessionData{
		ID: providers.ConfigID{
			ID:   s.ServiceID,
			Type: s.ServiceType,
		},
		AccessToken: token.AccessToken,
		TokenType:   token.Type(),
		// TODO(beyang): store and use refresh token to auto-refresh sessions
	}
}
