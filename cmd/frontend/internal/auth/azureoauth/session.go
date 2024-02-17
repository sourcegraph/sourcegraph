package azureoauth

import (
	"context"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	extsvcauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/azuredevops"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const urnAzureDevOpsOAuth = "AzureDevOpsOAuth"

type sessionIssuerHelper struct {
	*extsvc.CodeHost
	db          database.DB
	clientID    string
	allowOrgs   map[string]struct{}
	allowSignup *bool
}

func (s *sessionIssuerHelper) GetOrCreateUser(ctx context.Context, token *oauth2.Token, _ *hubspot.ContactProperties) (newUserCreated bool, actr *actor.Actor, safeErrMsg string, err error) {
	user, err := userFromContext(ctx)
	if err != nil {
		return false, nil, "failed to read Azure DevOps Profile from oauth2 callback request", errors.Wrap(err, "azureoauth.GetOrCreateUser: failed to read user from context of callback request")
	}

	if allow, err := s.verifyAllowOrgs(ctx, user, token, httpcli.NewExternalClientFactory()); err != nil {
		return false, nil, "error in verifying authorized user organizations", err
	} else if !allow {
		msg := "User does not belong to any org from the allowed list of organizations. Please contact your site admin."
		return false, nil, msg, errors.Newf("%s Must be in one of %v", msg, s.allowOrgs)
	}

	// allowSignup is true by default in the config schema. If it's not set in the provider config,
	// then default to true. Otherwise defer to the value that's set in the config.
	signupAllowed := s.allowSignup == nil || *s.allowSignup

	var data extsvc.AccountData
	if err := azuredevops.SetExternalAccountData(&data, user, token); err != nil {
		return false, nil, "", errors.Wrapf(err, "failed to set external account data for azure devops user with email %q", user.EmailAddress)
	}

	// The API returned an email address with the first character capitalized during development.
	// Not taking any chances.
	email := strings.ToLower(user.EmailAddress)
	username, err := auth.NormalizeUsername(email)
	if err != nil {
		return false, nil, "failed to normalize username from email of azure dev ops account", errors.Wrapf(err, "failed to normalize username from email: %q", email)
	}

	newUserCreated, userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, s.db, auth.GetAndSaveUserOp{
		UserProps: database.NewUser{
			Username:        username,
			Email:           email,
			EmailIsVerified: email != "",
			DisplayName:     user.DisplayName,
		},
		ExternalAccount: extsvc.AccountSpec{
			ServiceType: s.ServiceType,
			ServiceID:   azuredevops.AzureDevOpsAPIURL,
			ClientID:    s.clientID,
			AccountID:   user.ID,
		},
		ExternalAccountData: data,
		CreateIfNotExist:    signupAllowed,
	})
	if err != nil {
		return false, nil, safeErrMsg, err
	}

	return newUserCreated, actor.FromUser(userID), "", nil
}

func (s *sessionIssuerHelper) DeleteStateCookie(w http.ResponseWriter, r *http.Request) {
	session.SetData(w, r, "oauthState", "")
}

func (s *sessionIssuerHelper) SessionData(token *oauth2.Token) oauth.SessionData {
	return oauth.SessionData{
		ID: providers.ConfigID{
			ID:   s.ServiceID,
			Type: s.ServiceType,
		},
		AccessToken: token.AccessToken,
		TokenType:   token.Type(),
	}
}

func (s *sessionIssuerHelper) AuthSucceededEventName() database.SecurityEventName {
	return database.SecurityEventAzureDevOpsAuthSucceeded
}

func (s *sessionIssuerHelper) AuthFailedEventName() database.SecurityEventName {
	return database.SecurityEventAzureDevOpsAuthFailed
}

func (s *sessionIssuerHelper) GetServiceID() string {
	return s.ServiceID
}

func (s *sessionIssuerHelper) verifyAllowOrgs(ctx context.Context, profile *azuredevops.Profile, token *oauth2.Token, cf *httpcli.Factory) (bool, error) {
	if len(s.allowOrgs) == 0 {
		return true, nil
	}

	client, err := azuredevops.NewClient(
		urnAzureDevOpsOAuth,
		azuredevops.AzureDevOpsAPIURL,
		&extsvcauth.OAuthBearerToken{
			Token: token.AccessToken,
		},
		cf,
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to create client for listing organizations of user")
	}

	authorizedOrgs, err := client.ListAuthorizedUserOrganizations(ctx, *profile)
	if err != nil {
		return false, errors.Wrap(err, "failed to list organizations of user")
	}

	for _, org := range authorizedOrgs {
		if _, ok := s.allowOrgs[org.Name]; ok {
			return true, nil
		}
	}

	return false, nil
}

type key int

const userKey key = iota

func withUser(ctx context.Context, user azuredevops.Profile) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (*azuredevops.Profile, error) {
	user, ok := ctx.Value(userKey).(azuredevops.Profile)
	if !ok {
		return nil, errors.Errorf("azuredevops: Context missing Azure DevOps user")
	}
	return &user, nil
}
