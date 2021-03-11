package githuboauth

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/dghubble/gologin/github"
	"github.com/inconshreveable/log15"
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
	esauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	githubsvc "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type sessionIssuerHelper struct {
	*extsvc.CodeHost
	clientID    string
	allowSignup bool
	allowOrgs   []string
}

func (s *sessionIssuerHelper) GetOrCreateUser(ctx context.Context, token *oauth2.Token, anonymousUserID, firstSourceURL string) (actr *actor.Actor, safeErrMsg string, err error) {
	ghUser, err := github.UserFromContext(ctx)
	if ghUser == nil {
		if err != nil {
			err = errors.Wrap(err, "could not read user from context")
		} else {
			err = errors.New("could not read user from context")
		}
		return nil, "Could not read GitHub user from callback request.", err
	}

	login, err := auth.NormalizeUsername(deref(ghUser.Login))
	if err != nil {
		return nil, fmt.Sprintf("Error normalizing the username %q. See https://docs.sourcegraph.com/admin/auth/#username-normalization.", login), err
	}

	ghClient := s.newClient(token.AccessToken)

	// 🚨 SECURITY: Ensure that the user email is verified
	verifiedEmails := getVerifiedEmails(ctx, ghClient)
	if len(verifiedEmails) == 0 {
		return nil, "Could not get verified email for GitHub user. Check that your GitHub account has a verified email that matches one of your Sourcegraph verified emails.", errors.New("no verified email")
	}

	// 🚨 SECURITY: Ensure that the user is part of one of the white listed orgs, if any.
	if !s.verifyUserOrgs(ctx, ghClient) {
		return nil, "Could not verify user is part of the allowed GitHub organizations.", errors.New("couldn't verify user is part of allowed GitHub organizations")
	}

	// Try every verified email in succession until the first that succeeds
	var data extsvc.AccountData
	githubsvc.SetExternalAccountData(&data, ghUser, token)
	var (
		firstSafeErrMsg string
		firstErr        error
	)
	for i, verifiedEmail := range verifiedEmails {
		userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, auth.GetAndSaveUserOp{
			UserProps: database.NewUser{
				Username:        login,
				Email:           verifiedEmail,
				EmailIsVerified: true,
				DisplayName:     deref(ghUser.Name),
				AvatarURL:       deref(ghUser.AvatarURL),
			},
			ExternalAccount: extsvc.AccountSpec{
				ServiceType: s.ServiceType,
				ServiceID:   s.ServiceID,
				ClientID:    s.clientID,
				AccountID:   strconv.FormatInt(derefInt64(ghUser.ID), 10),
			},
			ExternalAccountData: data,
			CreateIfNotExist:    s.allowSignup,
		})
		if err == nil {
			go hubspotutil.SyncUser(verifiedEmail, hubspotutil.SignupEventID, &hubspot.ContactProperties{
				AnonymousUserID: anonymousUserID,
				FirstSourceURL:  firstSourceURL,
			})
			return actor.FromUser(userID), "", nil // success
		}
		if i == 0 {
			firstSafeErrMsg, firstErr = safeErrMsg, err
		}
	}
	// On failure, return the first error
	return nil, fmt.Sprintf("No user exists matching any of the verified emails: %s.\n\nFirst error was: %s", strings.Join(verifiedEmails, ", "), firstSafeErrMsg), firstErr
}

func (s *sessionIssuerHelper) CreateCodeHostConnection(ctx context.Context, token *oauth2.Token, providerID string) (safeErrMsg string, err error) {
	actor := actor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return "Must be authenticated to create code host connection from OAuth flow.", errors.New("unauthenticated request")
	}

	p := oauth.GetProvider(extsvc.TypeGitHub, providerID)
	if p == nil {
		return "Could not find OAuth provider for the state.", errors.Errorf("provider not found for %q", providerID)
	}

	ghUser, err := github.UserFromContext(ctx)
	if ghUser == nil {
		if err != nil {
			err = errors.Wrap(err, "could not read user from context")
		} else {
			err = errors.New("could not read user from context")
		}
		return "Could not read GitHub user from callback request.", err
	}

	err = database.GlobalExternalServices.Create(ctx, conf.Get,
		&types.ExternalService{
			Kind:        extsvc.KindGitHub,
			DisplayName: fmt.Sprintf("GitHub (%s)", deref(ghUser.Login)),
			Config: fmt.Sprintf(`
{
  "url": "%s",
  "token": "%s",
  "orgs": []
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func (s *sessionIssuerHelper) newClient(token string) *githubsvc.V3Client {
	apiURL, _ := githubsvc.APIRoot(s.BaseURL)
	return githubsvc.NewV3Client(apiURL, &esauth.OAuthBearerToken{Token: token}, nil)
}

// getVerifiedEmails returns the list of user emails that are verified. If the primary email is verified,
// it will be the first email in the returned list. It only checks the first 100 user emails.
func getVerifiedEmails(ctx context.Context, ghClient *githubsvc.V3Client) (verifiedEmails []string) {
	emails, err := ghClient.GetAuthenticatedUserEmails(ctx)
	if err != nil {
		log15.Warn("Could not get GitHub authenticated user emails", "error", err)
		return nil
	}

	for _, email := range emails {
		if !email.Verified {
			continue
		}
		if email.Primary {
			verifiedEmails = append([]string{email.Email}, verifiedEmails...)
			continue
		}
		verifiedEmails = append(verifiedEmails, email.Email)
	}
	return verifiedEmails
}

func (s *sessionIssuerHelper) verifyUserOrgs(ctx context.Context, ghClient *githubsvc.V3Client) bool {
	if len(s.allowOrgs) == 0 {
		return true
	}

	userOrgs, err := ghClient.GetAuthenticatedUserOrgs(ctx)
	if err != nil {
		log15.Warn("Could not get GitHub authenticated user organizations", "error", err)
		return false
	}

	allowed := make(map[string]bool, len(s.allowOrgs))
	for _, org := range s.allowOrgs {
		allowed[org] = true
	}

	for _, org := range userOrgs {
		if allowed[org.Login] {
			return true
		}
	}

	return false
}
