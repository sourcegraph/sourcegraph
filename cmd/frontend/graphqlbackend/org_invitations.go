package graphqlbackend

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/txemail/txtypes"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var SigningKeyMessage = "signing key not provided, cannot create JWT for invitation URL. Please add organizationInvitations signingKey to site configuration."
var DefaultExpiryDuration = 48 * time.Hour

func getUserToInviteToOrganization(ctx context.Context, db database.DB, username string, orgID int32) (userToInvite *types.User, userEmailAddress string, err error) {
	userToInvite, err = db.Users().GetByUsername(ctx, username)
	if err != nil {
		return nil, "", err
	}

	if conf.CanSendEmail() {
		// Look up user's email address so we can send them an email (if needed).
		email, verified, err := db.UserEmails().GetPrimaryEmail(ctx, userToInvite.ID)
		if err != nil && !errcode.IsNotFound(err) {
			return nil, "", errors.WithMessage(err, "looking up invited user's primary email address")
		}
		if !verified {
			return nil, "", errors.New("cannot invite user because their primary email address is not verified")
		}

		userEmailAddress = email
	}

	if _, err := db.OrgMembers().GetByOrgIDAndUserID(ctx, orgID, userToInvite.ID); err == nil {
		return nil, "", errors.New("user is already a member of the organization")
	} else if !errors.HasType[*database.ErrOrgMemberNotFound](err) {
		return nil, "", err
	}
	return userToInvite, userEmailAddress, nil
}

type inviteUserToOrganizationResult struct {
	sentInvitationEmail bool
	invitationURL       string
}

type orgInvitationClaims struct {
	InvitationID int64 `json:"invite_id"`
	SenderID     int32 `json:"sender_id"`
	jwt.RegisteredClaims
}

func (r *inviteUserToOrganizationResult) SentInvitationEmail() bool { return r.sentInvitationEmail }
func (r *inviteUserToOrganizationResult) InvitationURL() string     { return r.invitationURL }

func checkEmail(ctx context.Context, db database.DB, inviteEmail string) (bool, error) {
	user, err := db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return false, err
	}

	emails, err := db.UserEmails().ListByUser(ctx, database.UserEmailsListOptions{
		UserID: user.ID,
	})
	if err != nil {
		return false, err
	}

	containsEmail := func(userEmails []*database.UserEmail, email string) *database.UserEmail {
		for _, userEmail := range userEmails {
			if strings.EqualFold(email, userEmail.Email) {
				return userEmail
			}
		}

		return nil
	}

	emailMatch := containsEmail(emails, inviteEmail)
	if emailMatch == nil {
		var emailAddresses []string
		for _, userEmail := range emails {
			emailAddresses = append(emailAddresses, userEmail.Email)
		}
		return false, errors.Newf("your email addresses %v do not match the email address on the invitation.", emailAddresses)
	} else if emailMatch.VerifiedAt == nil {
		// set email address as verified if not already
		// db.UserEmails().SetVerified(ctx, user.ID, inviteEmail, true)
		return true, nil
	}

	return false, nil
}

func newExpiryDuration() time.Duration {
	expiryDuration := DefaultExpiryDuration
	if orgInvitationConfigDefined() && conf.SiteConfig().OrganizationInvitations.ExpiryTime > 0 {
		expiryDuration = time.Duration(conf.SiteConfig().OrganizationInvitations.ExpiryTime) * time.Hour
	}
	return expiryDuration
}

func newExpiryTime() time.Time {
	return timeNow().Add(newExpiryDuration())
}

func (r *schemaResolver) InvitationByToken(ctx context.Context, args *struct {
	Token string
}) (*organizationInvitationResolver, error) {
	actor := sgactor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return nil, errors.New("no current user")
	}
	if !orgInvitationConfigDefined() {
		return nil, errors.Newf("signing key not provided, cannot validate JWT on invitation URL. Please add organizationInvitations signingKey to site configuration.")
	}

	token, err := jwt.ParseWithClaims(args.Token, &orgInvitationClaims{}, func(token *jwt.Token) (any, error) {
		return base64.StdEncoding.DecodeString(conf.SiteConfig().OrganizationInvitations.SigningKey)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Name}))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*orgInvitationClaims); ok && token.Valid {
		invite, err := r.db.OrgInvitations().GetPendingByID(ctx, claims.InvitationID)
		if err != nil {
			return nil, err
		}
		if invite.RecipientUserID > 0 && invite.RecipientUserID != actor.UID {
			return nil, database.NewOrgInvitationNotFoundError(claims.InvitationID)
		}
		if invite.RecipientEmail != "" {
			willVerify, err := checkEmail(ctx, r.db, invite.RecipientEmail)
			if err != nil {
				return nil, err
			}
			invite.IsVerifiedEmail = !willVerify
		}

		return NewOrganizationInvitationResolver(r.db, invite), nil
	} else {
		return nil, errors.Newf("Invitation token not valid")
	}
}

func (r *schemaResolver) InviteUserToOrganization(ctx context.Context, args *struct {
	Organization graphql.ID
	Username     string
}) (*inviteUserToOrganizationResult, error) {
	var orgID int32
	if err := relay.UnmarshalSpec(args.Organization, &orgID); err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Check that the current user is a member of the org that the user is being
	// invited to.
	if err := auth.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgID); err != nil {
		return nil, err
	}

	// Create the invitation.
	org, err := r.db.Orgs().GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	sender, err := r.db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}

	// sending invitation to user ID or email
	var recipientID int32
	var recipientEmail string
	var userEmail string
	var recipient *types.User
	recipient, userEmail, err = getUserToInviteToOrganization(ctx, r.db, args.Username, orgID)
	if err != nil {
		return nil, err
	}
	recipientID = recipient.ID
	hasConfig := orgInvitationConfigDefined()

	expiryTime := newExpiryTime()
	invitation, err := r.db.OrgInvitations().Create(ctx, orgID, sender.ID, recipientID, recipientEmail, expiryTime)
	if err != nil {
		return nil, err
	}

	// create invitation URL
	var invitationURL string
	if hasConfig {
		invitationURL, err = orgInvitationURL(*invitation, false)
	} else { // TODO: remove this fallback once signing key is enforced for on-prem instances
		invitationURL = orgInvitationURLLegacy(org, false)
	}

	if err != nil {
		return nil, err
	}
	result := &inviteUserToOrganizationResult{
		invitationURL: invitationURL,
	}

	// Send a notification to the recipient. If disabled, the frontend will still show the
	// invitation link.
	if conf.CanSendEmail() && userEmail != "" {
		if err := sendOrgInvitationNotification(ctx, r.db, org, sender, userEmail, invitationURL, *invitation.ExpiresAt); err != nil {
			return nil, errors.WithMessage(err, "sending notification to invitation recipient")
		}
		result.sentInvitationEmail = true
	}
	return result, nil
}

func (r *schemaResolver) RespondToOrganizationInvitation(ctx context.Context, args *struct {
	OrganizationInvitation graphql.ID
	ResponseType           string
}) (*EmptyResponse, error) {
	a := sgactor.FromContext(ctx)
	if !a.IsAuthenticated() {
		return nil, errors.New("no current user")
	}

	id, err := UnmarshalOrgInvitationID(args.OrganizationInvitation)
	if err != nil {
		return nil, err
	}

	// Convert from GraphQL enum to Go bool.
	var accept bool
	switch args.ResponseType {
	case "ACCEPT":
		accept = true
	case "REJECT":
		// noop
	default:
		return nil, errors.Errorf("invalid OrganizationInvitationResponseType value %q", args.ResponseType)
	}

	invitation, err := r.db.OrgInvitations().GetPendingByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Mark the email as verified if needed
	if invitation.RecipientEmail != "" {
		// 🚨 SECURITY: This fails if the org invitation's recipient email is not the one given
		shouldMarkAsVerified, err := checkEmail(ctx, r.db, invitation.RecipientEmail)
		if err != nil {
			return nil, err
		}
		if shouldMarkAsVerified && accept {
			// ignore errors here as this is a best-effort action
			_ = r.db.UserEmails().SetVerified(ctx, a.UID, invitation.RecipientEmail, shouldMarkAsVerified)
		}
	} else if invitation.RecipientUserID > 0 && invitation.RecipientUserID != a.UID {
		// 🚨 SECURITY: Fail if the org invitation's recipient is not the one given
		return nil, database.NewOrgInvitationNotFoundError(id)
	}

	// 🚨 SECURITY: This fails if the invitation is invalid
	orgID, err := r.db.OrgInvitations().Respond(ctx, id, a.UID, accept)
	if err != nil {
		return nil, err
	}

	if accept {
		// The recipient accepted the invitation.
		if _, err := r.db.OrgMembers().Create(ctx, orgID, a.UID); err != nil {
			return nil, err
		}

		// Schedule permission sync for user that accepted the invite. Internally it will log an error if enqueuing fails.
		permssync.SchedulePermsSync(ctx, r.logger, r.db, permssync.ScheduleSyncOpts{UserIDs: []int32{a.UID}, Reason: database.ReasonUserAcceptedOrgInvite})
	}
	return &EmptyResponse{}, nil
}

func orgInvitationConfigDefined() bool {
	return conf.SiteConfig().OrganizationInvitations != nil && conf.SiteConfig().OrganizationInvitations.SigningKey != ""
}

func orgInvitationURLLegacy(org *types.Org, relative bool) string {
	path := fmt.Sprintf("/organizations/%s/invitation", org.Name)
	if relative {
		return path
	}
	return globals.ExternalURL().ResolveReference(&url.URL{Path: path}).String()
}

func orgInvitationURL(invitation database.OrgInvitation, relative bool) (string, error) {
	if invitation.ExpiresAt == nil {
		return "", errors.New("invitation does not have expiry time defined")
	}
	token, err := createInvitationJWT(invitation.OrgID, invitation.ID, invitation.SenderUserID, *invitation.ExpiresAt)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/organizations/invitation/%s", token)
	if relative {
		return path, nil
	}
	return globals.ExternalURL().ResolveReference(&url.URL{Path: path}).String(), nil
}

func createInvitationJWT(orgID int32, invitationID int64, senderID int32, expiryTime time.Time) (string, error) {
	if !orgInvitationConfigDefined() {
		return "", errors.New(SigningKeyMessage)
	}
	config := conf.SiteConfig().OrganizationInvitations

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &orgInvitationClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    globals.ExternalURL().String(),
			ExpiresAt: jwt.NewNumericDate(expiryTime),
			Subject:   strconv.FormatInt(int64(orgID), 10),
		},
		InvitationID: invitationID,
		SenderID:     senderID,
	})

	// Sign and get the complete encoded token as a string using the secret
	key, err := base64.StdEncoding.DecodeString(config.SigningKey)
	if err != nil {
		return "", err
	}
	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// sendOrgInvitationNotification sends an email to the recipient of an org invitation with a link to
// respond to the invitation. Callers should check conf.CanSendEmail() if they want to return a nice
// error if sending email is not enabled.
func sendOrgInvitationNotification(ctx context.Context, db database.DB, org *types.Org, sender *types.User, recipientEmail string, invitationURL string, expiryTime time.Time) error {
	if dotcom.IsAbusePreventionEnabled() {
		// Only allow email-verified users to send invites.
		if _, senderEmailVerified, err := db.UserEmails().GetPrimaryEmail(ctx, sender.ID); err != nil {
			return err
		} else if !senderEmailVerified {
			return errors.New("must verify your email address to invite a user to an organization")
		}

		// Check and decrement our invite quota, to prevent abuse (sending too many invites).
		//
		// There is no user invite quota for on-prem instances because we assume they can
		// trust their users to not abuse invites.
		if ok, err := db.Users().CheckAndDecrementInviteQuota(ctx, sender.ID); err != nil {
			return err
		} else if !ok {
			return errors.New("invite quota exceeded (contact support to increase the quota)")
		}
	}

	var fromName string
	if sender.DisplayName != "" {
		fromName = fmt.Sprintf("%s (@%s)", sender.DisplayName, sender.Username)
	} else {
		fromName = fmt.Sprintf("@%s", sender.Username)
	}

	var orgName string
	if org.DisplayName != nil {
		orgName = *org.DisplayName
	} else {
		orgName = org.Name
	}

	return txemail.Send(ctx, "org_invite", txemail.Message{
		To:       []string{recipientEmail},
		Template: emailTemplates,
		Data: struct {
			FromName        string
			FromDisplayName string
			FromUserName    string
			OrgName         string
			InvitationUrl   string
			ExpiryDays      int
		}{
			FromName:        fromName,
			FromDisplayName: sender.DisplayName,
			FromUserName:    sender.Username,
			OrgName:         orgName,
			InvitationUrl:   invitationURL,
			ExpiryDays:      int(math.Round(expiryTime.Sub(timeNow()).Hours() / 24)), // golang does not have `duration.Days` :(
		},
	})
}

var emailTemplates = txemail.MustValidate(txtypes.Templates{
	Subject: `{{.FromName}} invited you to join {{.OrgName}} on Sourcegraph`,
	Text: `
{{.FromName}} invited you to join the {{.OrgName}} organization on Sourcegraph.

New to Sourcegraph? Sourcegraph helps your team to learn and understand your codebase quickly, and share code via links, speeding up team collaboration even while apart.

Visit this link in your browser to accept the invite: {{.InvitationUrl}}

This link will expire in {{.ExpiryDays}} days. You are receiving this email because @{{.FromUserName}} invited you to an organization on Sourcegraph Cloud.


To see our Terms of Service, please visit this link: https://sourcegraph.com/terms
To see our Privacy Policy, please visit this link: https://sourcegraph.com/privacy

Sourcegraph, 981 Mission St, San Francisco, CA 94103, USA
`,
	HTML: `
<html>
<head>
  <meta name="color-scheme" content="light">
  <meta name="supported-color-schemes" content="light">
  <style>
    body { color: #343a4d; background: #fff; padding: 20px; font-size: 16px; font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica Neue,Arial,Noto Sans,sans-serif,Apple Color Emoji,Segoe UI Emoji,Segoe UI Symbol,Noto Color Emoji; }
    .logo { height: 34px; margin-bottom: 15px; }
    a { color: #0b70db; text-decoration: none; background-color: transparent; }
    a:hover { color: #0c7bf0; text-decoration: underline; }
    a.btn { display: inline-block; color: #fff; background-color: #0b70db; padding: 8px 16px; border-radius: 3px; font-weight: 600; }
    a.btn:hover { color: #fff; background-color: #0864c6; text-decoration:none; }
    .smaller { font-size: 14px; }
    small { color: #5e6e8c; font-size: 12px; }
    .mtm { margin-top: 10px; }
    .mtl { margin-top: 20px; }
    .mtxl { margin-top: 30px; }
  </style>
</head>
<body style="font-family: -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica Neue,Arial,Noto Sans,sans-serif,Apple Color Emoji,Segoe UI Emoji,Segoe UI Symbol,Noto Color Emoji;">
  <img class="logo" src="https://storage.googleapis.com/sourcegraph-assets/sourcegraph-logo-light-small.png" alt="Sourcegraph logo">
  <p>
    <strong>{{.FromDisplayName}}</strong> (@{{.FromUserName}}) invited you to join the <strong>{{.OrgName}}</strong> organization on Sourcegraph.
  </p>
  <p class="mtxl">
    <strong>New to Sourcegraph?</strong> Sourcegraph helps your team to learn and understand your codebase quickly, and share code via links, speeding up team collaboration even while apart.
  </p>
  <p>
    <a class="btn mtm" href="{{.InvitationUrl}}">Accept invite</a>
  </p>
  <p class="smaller">Or visit this link in your browser: <a href="{{.InvitationUrl}}">{{.InvitationUrl}}</a></p>
  <small>
  <p class="mtl">
    This link will expire in {{.ExpiryDays}} days. You are receiving this email because @{{.FromUserName}} invited you to an organization on Sourcegraph Cloud.
  </p>
  <p class="mtl">
    <a href="https://sourcegraph.com/terms">Terms</a>&nbsp;&#8226;&nbsp;
    <a href="https://sourcegraph.com/privacy">Privacy</a>
  </p>
  <p>Sourcegraph, 981 Mission St, San Francisco, CA 94103, USA</p>
  </small>
</body>
</html>
`,
})
