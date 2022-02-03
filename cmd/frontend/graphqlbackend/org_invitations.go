package graphqlbackend

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"

	"github.com/sourcegraph/sourcegraph/internal/actor"

	"github.com/cockroachdb/errors"
	"github.com/golang-jwt/jwt/v4"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/internal/txemail/txtypes"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func getUserToInviteToOrganization(ctx context.Context, db database.DB, username string, orgID int32) (userToInvite *types.User, userEmailAddress string, err error) {
	userToInvite, err = db.Users().GetByUsername(ctx, username)
	if err != nil {
		return nil, "", err
	}

	if conf.CanSendEmail() {
		// Look up user's email address so we can send them an email (if needed).
		email, verified, err := database.UserEmails(db).GetPrimaryEmail(ctx, userToInvite.ID)
		if err != nil && !errcode.IsNotFound(err) {
			return nil, "", errors.WithMessage(err, "looking up invited user's primary email address")
		}
		if verified {
			// Completely discard unverified emails.
			userEmailAddress = email
		}
	}

	if _, err := db.OrgMembers().GetByOrgIDAndUserID(ctx, orgID, userToInvite.ID); err == nil {
		return nil, "", errors.New("user is already a member of the organization")
	} else if !errors.HasType(err, &database.ErrOrgMemberNotFound{}) {
		return nil, "", err
	}
	return userToInvite, userEmailAddress, nil
}

type inviteUserToOrganizationResult struct {
	sentInvitationEmail bool
	invitationURL       string
}

type orgInvitationClaims struct {
	InvitationID int64 `json:"invite_ID"`
	SenderID     int32 `json:"sender_id"`
	jwt.StandardClaims
}

func (r *inviteUserToOrganizationResult) SentInvitationEmail() bool { return r.sentInvitationEmail }
func (r *inviteUserToOrganizationResult) InvitationURL() string     { return r.invitationURL }

func (r *schemaResolver) InvitationByToken(ctx context.Context, args *struct {
	Token string
}) (*organizationInvitationResolver, error) {
	actor := actor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return nil, errors.New("no current user")
	}
	if !orgInvitationConfigDefined() {
		return nil, errors.Newf("signing key not provided, cannot validate JWT on invitation URL. Please add organizationInvitations signingKey to site configuration.")
	}

	token, err := jwt.ParseWithClaims(args.Token, &orgInvitationClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.Newf("Unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(conf.SiteConfig().OrganizationInvitations.SigningKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*orgInvitationClaims); ok && token.Valid {
		invite, err := r.db.OrgInvitations().GetPendingByID(ctx, claims.InvitationID) //(ctx, r.db, claims.InvitationID)
		if err != nil {
			return nil, err
		}
		if invite.RecipientUserID > 0 && invite.RecipientUserID != actor.UID {
			return nil, database.NewOrgInvitationNotFoundError(claims.InvitationID)
		}

		return NewOrganizationInvitationResolver(r.db, invite), nil
	} else {
		return nil, errors.Newf("Invitation token not valid")
	}
}

func (r *schemaResolver) InviteUserToOrganization(ctx context.Context, args *struct {
	Organization graphql.ID
	Username     *string
	Email        *string
}) (*inviteUserToOrganizationResult, error) {
	if (args.Email != nil && *args.Email != "") || args.Username == nil {
		return nil, errors.New("inviting by email is not implemented yet")
	}
	var orgID int32
	if err := relay.UnmarshalSpec(args.Organization, &orgID); err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Check that the current user is a member of the org that the user is being
	// invited to.
	if err := backend.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgID); err != nil {
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
	recipient, recipientEmail, err := getUserToInviteToOrganization(ctx, r.db, *args.Username, orgID)
	if err != nil {
		return nil, err
	}
	invitation, err := r.db.OrgInvitations().Create(ctx, orgID, sender.ID, recipient.ID)
	if err != nil {
		return nil, err
	}
	var invitationURL string
	if orgInvitationConfigDefined() {
		invitationURL, err = orgInvitationURL(org.ID, invitation.ID, sender.ID, recipient.ID, recipientEmail, false)
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
	if conf.CanSendEmail() && recipientEmail != "" {
		if err := sendOrgInvitationNotification(ctx, r.db, org, sender, recipientEmail, invitationURL); err != nil {
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
	a := actor.FromContext(ctx)
	if !a.IsAuthenticated() {
		return nil, errors.New("no current user")
	}

	id, err := unmarshalOrgInvitationID(args.OrganizationInvitation)
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

	// 🚨 SECURITY: This fails if the org invitation's recipient is not the one given (or if the
	// invitation is otherwise invalid), so we do not need to separately perform that check.
	orgID, err := r.db.OrgInvitations().Respond(ctx, id, a.UID, accept)
	if err != nil {
		return nil, err
	}

	if accept {
		// The recipient accepted the invitation.
		if _, err := r.db.OrgMembers().Create(ctx, orgID, a.UID); err != nil {
			return nil, err
		}

		// Schedule permission sync for user that accepted the invite
		err = r.repoupdaterClient.SchedulePermsSync(ctx, protocol.PermsSyncRequest{UserIDs: []int32{a.UID}})
		if err != nil {
			log15.Warn("schemaResolver.RespondToOrganizationInvitation.SchedulePermsSync",
				"userID", a.UID,
				"error", err,
			)
		}
	}
	return &EmptyResponse{}, nil
}

func (r *schemaResolver) ResendOrganizationInvitationNotification(ctx context.Context, args *struct {
	OrganizationInvitation graphql.ID
}) (*EmptyResponse, error) {
	orgInvitation, err := orgInvitationByID(ctx, r.db, args.OrganizationInvitation)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the current user is a member of the org that the invite is for.
	if err := backend.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgInvitation.v.OrgID); err != nil {
		return nil, err
	}

	// Prevent reuse. This just prevents annoyance (abuse is prevented by the quota check in the
	// call to sendOrgInvitationNotification).
	if orgInvitation.v.RevokedAt != nil {
		return nil, errors.New("refusing to send notification for revoked invitation")
	}
	if orgInvitation.v.RespondedAt != nil {
		return nil, errors.New("refusing to send notification for invitation that was already responded to")
	}

	if !conf.CanSendEmail() {
		return nil, errors.New("unable to send notification for invitation because sending emails is not enabled")
	}

	org, err := r.db.Orgs().GetByID(ctx, orgInvitation.v.OrgID)
	if err != nil {
		return nil, err
	}
	sender, err := r.db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	recipientEmail, recipientEmailVerified, err := r.db.UserEmails().GetPrimaryEmail(ctx, orgInvitation.v.RecipientUserID)
	if err != nil {
		return nil, err
	}
	if !recipientEmailVerified {
		return nil, errors.New("refusing to send notification because recipient has no verified email address")
	}
	var invitationURL string
	if orgInvitationConfigDefined() {
		invitationURL, err = orgInvitationURL(org.ID, orgInvitation.v.ID, sender.ID, orgInvitation.v.RecipientUserID, recipientEmail, false)
	} else { // TODO: remove this fallback once signing key is enforced for on-prem instances
		invitationURL = orgInvitationURLLegacy(org, false)
	}
	if err != nil {
		return nil, err
	}
	if err := sendOrgInvitationNotification(ctx, r.db, org, sender, recipientEmail, invitationURL); err != nil {
		return nil, err
	}
	return &EmptyResponse{}, nil
}

func (r *schemaResolver) RevokeOrganizationInvitation(ctx context.Context, args *struct {
	OrganizationInvitation graphql.ID
}) (*EmptyResponse, error) {
	orgInvitation, err := orgInvitationByID(ctx, r.db, args.OrganizationInvitation)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the current user is a member of the org that the invite is for.
	if err := backend.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgInvitation.v.OrgID); err != nil {
		return nil, err
	}

	if err := r.db.OrgInvitations().Revoke(ctx, orgInvitation.v.ID); err != nil {
		return nil, err
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

func orgInvitationURL(orgID int32, invitationID int64, senderID int32, recipientID int32, recipientEmail string, relative bool) (string, error) {
	token, err := createInvitationJWT(orgID, invitationID, senderID, recipientID, recipientEmail)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/organizations/invitation/%s", token)
	if relative {
		return path, nil
	}
	return globals.ExternalURL().ResolveReference(&url.URL{Path: path}).String(), nil
}

func createInvitationJWT(orgID int32, invitationID int64, senderID int32, recipientID int32, recipientEmail string) (string, error) {
	aud := recipientEmail
	if aud == "" {
		aud = strconv.FormatInt(int64(recipientID), 10)
	}
	if !orgInvitationConfigDefined() {
		return "", errors.New("signing key not provided, cannot create JWT for invitation URL. Please add organizationInvitations signingKey to site configuration.")
	}
	config := conf.SiteConfig().OrganizationInvitations

	expiryTime := time.Duration(config.ExpiryTime)
	if expiryTime == 0 {
		expiryTime = 48 // default expiry time is 2 days
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, &orgInvitationClaims{
		StandardClaims: jwt.StandardClaims{
			Audience:  aud,
			Issuer:    globals.ExternalURL().String(),
			ExpiresAt: time.Now().Add(expiryTime * time.Hour).Unix(), // TODO: store expiry in DB
			Subject:   strconv.FormatInt(int64(orgID), 10),
		},
		InvitationID: invitationID,
		SenderID:     senderID,
	})

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(config.SigningKey))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// sendOrgInvitationNotification sends an email to the recipient of an org invitation with a link to
// respond to the invitation. Callers should check conf.CanSendEmail() if they want to return a nice
// error if sending email is not enabled.
func sendOrgInvitationNotification(ctx context.Context, db database.DB, org *types.Org, sender *types.User, recipientEmail string, invitationURL string) error {
	if envvar.SourcegraphDotComMode() {
		// Basic abuse prevention for Sourcegraph.com.

		// Only allow email-verified users to send invites.
		if _, senderEmailVerified, err := database.UserEmails(db).GetPrimaryEmail(ctx, sender.ID); err != nil {
			return err
		} else if !senderEmailVerified {
			return errors.New("must verify your email address to invite a user to an organization")
		}

		// Check and decrement our invite quota, to prevent abuse (sending too many invites).
		//
		// There is no user invite quota for on-prem instances because we assume they can
		// trust their users to not abuse invites.
		if ok, err := database.Users(db).CheckAndDecrementInviteQuota(ctx, sender.ID); err != nil {
			return err
		} else if !ok {
			return errors.New("invite quota exceeded (contact support to increase the quota)")
		}
	}

	var fromName string
	if sender.DisplayName != "" {
		fromName = sender.DisplayName
	} else {
		fromName = sender.Username
	}

	return txemail.Send(ctx, txemail.Message{
		To:       []string{recipientEmail},
		Template: emailTemplates,
		Data: struct {
			FromName string
			OrgName  string
			URL      string
		}{
			FromName: fromName,
			OrgName:  org.Name,
			URL:      invitationURL,
		},
	})
}

var emailTemplates = txemail.MustValidate(txtypes.Templates{
	Subject: `{{.FromName}} invited you to join {{.OrgName}} on Sourcegraph`,
	Text: `
{{.FromName}} invited you to join the {{.OrgName}} organization on Sourcegraph.

To accept the invitation, follow this link:

  {{.URL}}
`,
	HTML: `
<p>
  <strong>{{.FromName}}</strong> invited you to join the
  <strong>{{.OrgName}}</strong> organization on Sourcegraph.
</p>

<p><strong><a href="{{.URL}}">Join {{.OrgName}}</a></strong></p>
`,
})
