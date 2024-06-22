package graphqlbackend

import (
	"context"
	"net/url"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type randomizeUserPasswordResult struct {
	resetURL  *url.URL
	emailSent bool
}

func (r *randomizeUserPasswordResult) ResetPasswordURL() *string {
	if r.resetURL == nil {
		return nil
	}
	urlStr := globals.ExternalURL().ResolveReference(r.resetURL).String()
	return &urlStr
}

func (r *randomizeUserPasswordResult) EmailSent() bool { return r.emailSent }

func sendPasswordResetURLToPrimaryEmail(ctx context.Context, db database.DB, userID int32, resetURL *url.URL) error {
	user, err := db.Users().GetByID(ctx, userID)
	if err != nil {
		return err
	}

	email, verified, err := db.UserEmails().GetPrimaryEmail(ctx, userID)
	if err != nil {
		return err
	}

	if !verified {
		resetURL, err = userpasswd.AttachEmailVerificationToPasswordReset(ctx, db.UserEmails(), *resetURL, userID, email)
		if err != nil {
			return errors.Wrap(err, "attach email verification")
		}
	}

	if err = userpasswd.SendResetPasswordURLEmail(ctx, email, user.Username, resetURL); err != nil {
		return err
	}

	return nil
}

func (r *schemaResolver) RandomizeUserPassword(ctx context.Context, args *struct {
	User graphql.ID
},
) (*randomizeUserPasswordResult, error) {
	if !userpasswd.ResetPasswordEnabled() {
		return nil, errors.New("resetting passwords is not enabled")
	}

	// 🚨 SECURITY: On dotcom, we MUST send password reset links via email.
	if dotcom.SourcegraphDotComMode() && !conf.CanSendEmail() {
		return nil, errors.New("unable to reset password because email sending is not configured")
	}

	// 🚨 SECURITY: Only site admins can randomize user passwords.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse user ID")
	}

	logger := r.logger.Scoped("randomizeUserPassword").
		With(log.Int32("userID", userID))

	logger.Info("resetting user password")
	if err := r.db.Users().RandomizePasswordAndClearPasswordResetRateLimit(ctx, userID); err != nil {
		return nil, err
	}

	// This method modifies the DB, which is somewhat counterintuitive for a "value" type from an
	// implementation POV. Its behavior is justified because it is convenient and intuitive from the
	// POV of the API consumer.
	resetURL, err := backend.MakePasswordResetURL(ctx, r.db, userID, "")
	if err != nil {
		return nil, err
	}

	// If email is enabled, we also send this reset URL to the user via email.
	var emailSent bool
	var emailSendErr error
	if conf.CanSendEmail() {
		logger.Debug("sending password reset URL in email")
		if emailSendErr = sendPasswordResetURLToPrimaryEmail(ctx, r.db, userID, resetURL); emailSendErr != nil {
			// This is not a hard error - if the email send fails, we still want to
			// provide the reset URL to the caller, so we just log it here.
			logger.Error("failed to send password reset URL", log.Error(emailSendErr))
		} else {
			// Email was sent to an email address associated with the user.
			emailSent = true
		}
	}

	if dotcom.SourcegraphDotComMode() {
		// 🚨 SECURITY: Do not return reset URL on dotcom - we must have send it via an email.
		// We already validate that email is enabled earlier in this endpoint for dotcom.
		resetURL = nil
		// Since we don't provide the reset URL, however, if the email fails to send then
		// this error should be surfaced to the caller.
		if emailSendErr != nil {
			return nil, errors.Wrap(emailSendErr, "failed to send password reset URL")
		}
	}

	return &randomizeUserPasswordResult{
		resetURL:  resetURL,
		emailSent: emailSent,
	}, nil
}
