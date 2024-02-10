package userpasswd

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// HandleSetPasswordEmail sends the password reset email directly to the user for users
// created by site admins.
//
// If the primary user's email is not verified, a special version of the reset link is
// emailed that also verifies the email.
func HandleSetPasswordEmail(ctx context.Context, db database.DB, id int32, username, email string, emailVerified bool) (string, error) {
	resetURL, err := backend.MakePasswordResetURL(ctx, db, id, email)
	if err == database.ErrPasswordResetRateLimit {
		return "", err
	} else if err != nil {
		return "", errors.Wrap(err, "make password reset URL")
	}

	shareableResetURL := globals.ExternalURL().ResolveReference(resetURL).String()
	emailedResetURL := shareableResetURL

	if !emailVerified {
		newURL, err := AttachEmailVerificationToPasswordReset(ctx, db.UserEmails(), *resetURL, id, email)
		if err != nil {
			return shareableResetURL, errors.Wrap(err, "attach email verification")
		}
		emailedResetURL = globals.ExternalURL().ResolveReference(newURL).String()
	}

	// Configure the template
	emailTemplate := defaultSetPasswordEmailTemplate
	if customTemplates := conf.SiteConfig().EmailTemplates; customTemplates != nil {
		emailTemplate = txemail.FromSiteConfigTemplateWithDefault(customTemplates.SetPassword, emailTemplate)
	}

	if err := txemail.Send(ctx, "password_set", txemail.Message{
		To:       []string{email},
		Template: emailTemplate,
		Data: SetPasswordEmailTemplateData{
			Username: username,
			URL:      emailedResetURL,
			Host:     globals.ExternalURL().Host,
		},
	}); err != nil {
		return shareableResetURL, err
	}

	return shareableResetURL, nil
}
