package githuboauth

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dghubble/gologin/v2/github"
	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot/hubspotutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	esauth "github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	githubsvc "github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github/githubconvert"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type sessionIssuerHelper struct {
	*extsvc.CodeHost
	db           database.DB
	clientID     string
	allowSignup  bool
	allowOrgs    []string
	allowOrgsMap map[string][]string
}

func (s *sessionIssuerHelper) AuthSucceededEventName() database.SecurityEventName {
	return database.SecurityEventGitHubAuthSucceeded
}

func (s *sessionIssuerHelper) AuthFailedEventName() database.SecurityEventName {
	return database.SecurityEventGitHubAuthFailed
}

func (s *sessionIssuerHelper) GetServiceID() string {
	return s.ServiceID
}

func (s *sessionIssuerHelper) GetOrCreateUser(ctx context.Context, token *oauth2.Token, hubSpotProps *hubspot.ContactProperties) (newUserCreated bool, actr *actor.Actor, safeErrMsg string, err error) {
	ghUser, err := github.UserFromContext(ctx)
	if ghUser == nil {
		if err != nil {
			err = errors.Wrap(err, "could not read user from context")
		} else {
			err = errors.New("could not read user from context")
		}
		return false, nil, "Could not read GitHub user from callback request.", err
	}

	login, err := auth.NormalizeUsername(deref(ghUser.Login))
	if err != nil {
		return false, nil, fmt.Sprintf("Error normalizing the username %q. See https://sourcegraph.com/docs/admin/auth/#username-normalization.", login), err
	}

	ghClient, err := s.newClient(token.AccessToken)
	if err != nil {
		return false, nil, "Error creating GitHub client", err
	}

	// 🚨 SECURITY: Ensure that the user email is verified
	verifiedEmails := getVerifiedEmails(ctx, ghClient)
	if len(verifiedEmails) == 0 {
		return false, nil, "Could not get verified email for GitHub user. Check that your GitHub account has a verified email that matches one of your Sourcegraph verified emails.", errors.New("no verified email")
	}

	dc := conf.Get().Dotcom
	if dc != nil && dc.MinimumExternalAccountAge > 0 {
		exempted := false
		for _, exemptedEmail := range dc.MinimumExternalAccountAgeExemptList {
			if slices.Contains(verifiedEmails, exemptedEmail) {
				exempted = true
				break
			}
		}
		earliestValidCreationDate := time.Now().Add(time.Duration(-dc.MinimumExternalAccountAge) * 24 * time.Hour)
		if !exempted && ghUser.CreatedAt.After(earliestValidCreationDate) {
			return false, nil, fmt.Sprintf("User account was created less than %d days ago", dc.MinimumExternalAccountAge), errors.New("user account too new")
		}
	}

	// 🚨 SECURITY: Ensure that the user is part of one of the allow listed orgs or teams, if any.
	userBelongsToAllowedOrgsOrTeams := s.verifyUserOrgsAndTeams(ctx, ghClient)
	if !userBelongsToAllowedOrgsOrTeams {
		message := "user does not belong to allowed GitHub organizations or teams."
		return false, nil, message, errors.New(message)
	}

	// Try every verified email in succession until the first that succeeds
	var data extsvc.AccountData
	if err := githubsvc.SetExternalAccountData(&data, githubconvert.ConvertUserV48ToV55(ghUser), token); err != nil {
		return false, nil, "", err
	}
	var (
		lastSafeErrMsg string
		lastErr        error
	)

	// We will first attempt to connect one of the verified emails with an existing
	// account in Sourcegraph
	type attemptConfig struct {
		email            string
		createIfNotExist bool
	}
	var attempts []attemptConfig
	for i := range verifiedEmails {
		attempts = append(attempts, attemptConfig{
			email:            verifiedEmails[i],
			createIfNotExist: false,
		})
	}
	signupErrorMessage := ""
	// If allowSignup is true, we will create an account using the first verified
	// email address from GitHub which we expect to be their primary address. Note
	// that the order of attempts is important. If we manage to connect with an
	// existing account we return early and don't attempt to create a new account.
	if s.allowSignup {
		attempts = append(attempts, attemptConfig{
			email:            verifiedEmails[0],
			createIfNotExist: true,
		})
		signupErrorMessage = "\n\nOr failed on creating a user account"
	}

	for _, attempt := range attempts {
		newUserCreated, userID, safeErrMsg, err := auth.GetAndSaveUser(ctx, s.db, auth.GetAndSaveUserOp{
			UserProps: database.NewUser{
				Username: login,

				// We always only take verified emails from an external source.
				Email:           attempt.email,
				EmailIsVerified: true,

				DisplayName: deref(ghUser.Name),
				AvatarURL:   deref(ghUser.AvatarURL),
			},
			ExternalAccount: extsvc.AccountSpec{
				ServiceType: s.ServiceType,
				ServiceID:   s.ServiceID,
				ClientID:    s.clientID,
				AccountID:   strconv.FormatInt(derefInt64(ghUser.ID), 10),
			},
			ExternalAccountData: data,
			CreateIfNotExist:    attempt.createIfNotExist,
		})
		if err == nil {
			go hubspotutil.SyncUser(attempt.email, hubspotutil.SignupEventID, hubSpotProps)
			return newUserCreated, actor.FromUser(userID), "", nil // success
		}
		lastSafeErrMsg, lastErr = safeErrMsg, err
	}

	// On failure, return the last error
	return false, nil, fmt.Sprintf("Could not find existing user matching any of the verified emails: %s %s \n\nLast error was: %s", strings.Join(verifiedEmails, ", "), signupErrorMessage, lastSafeErrMsg), lastErr
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

func (s *sessionIssuerHelper) newClient(token string) (*githubsvc.V3Client, error) {
	apiURL, _ := githubsvc.APIRoot(s.BaseURL)
	return githubsvc.NewV3Client(log.Scoped("session.github.v3"),
		extsvc.URNGitHubOAuth, apiURL, &esauth.OAuthBearerToken{Token: token}, nil)
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

// verifyUserOrgs checks whether the authenticated user belongs to one of the GitHub orgs
// listed in auth.provider > allowOrgs configuration
func (s *sessionIssuerHelper) verifyUserOrgs(ctx context.Context, ghClient *githubsvc.V3Client) bool {
	allowed := make(map[string]bool, len(s.allowOrgs))
	for _, org := range s.allowOrgs {
		allowed[org] = true
	}

	hasNextPage := true
	var userOrgs []*githubsvc.Org
	var err error
	page := 1
	for hasNextPage {
		userOrgs, hasNextPage, _, err = ghClient.GetAuthenticatedUserOrgs(ctx, page)

		if err != nil {
			log15.Warn("Could not get GitHub authenticated user organizations", "error", err)
			return false
		}

		for _, org := range userOrgs {
			if allowed[org.Login] {
				return true
			}
		}
		page++
	}

	return false
}

// verifyUserTeams checks whether the authenticated user belongs to one of the GitHub teams listed in the auth.provider > allowOrgsMap configuration
func (s *sessionIssuerHelper) verifyUserTeams(ctx context.Context, ghClient *githubsvc.V3Client) bool {
	var err error
	hasNextPage := true
	allowedTeams := make(map[string]map[string]bool, len(s.allowOrgsMap))

	for org, teams := range s.allowOrgsMap {
		teamsMap := make(map[string]bool)
		for _, team := range teams {
			teamsMap[team] = true
		}

		allowedTeams[org] = teamsMap
	}

	for page := 1; hasNextPage; page++ {
		var githubTeams []*githubsvc.Team

		githubTeams, hasNextPage, _, err = ghClient.GetAuthenticatedUserTeams(ctx, page)
		if err != nil {
			log15.Warn("Could not get GitHub authenticated user teams", "error", err)
			return false
		}

		for _, ghTeam := range githubTeams {
			_, ok := allowedTeams[ghTeam.Organization.Login][ghTeam.Name]
			if ok {
				return true
			}
		}
	}

	return false
}

// verifyUserOrgsAndTeams checks if the user belongs to one of the allowed listed orgs or teams provided in the auth.provider configuration.
func (s *sessionIssuerHelper) verifyUserOrgsAndTeams(ctx context.Context, ghClient *githubsvc.V3Client) bool {
	if len(s.allowOrgs) == 0 && len(s.allowOrgsMap) == 0 {
		return true
	}

	if len(s.allowOrgs) > 0 && s.verifyUserOrgs(ctx, ghClient) {
		return true
	}

	if len(s.allowOrgsMap) > 0 && s.verifyUserTeams(ctx, ghClient) {
		return true
	}

	return false
}
