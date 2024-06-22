package auth

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/auth/userpasswd"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ResetPasswordURL modifies the DB when it generates reset URLs, which is somewhat
// counterintuitive for a "value" type from an implementation POV. Its behavior is
// justified because it is convenient and intuitive from the POV of the API consumer.
func ResetPasswordURL(ctx context.Context, db database.DB, logger log.Logger, user *types.User, email string, emailVerified bool) (*string, error) {
	if !userpasswd.ResetPasswordEnabled() {
		return nil, nil
	}

	if email != "" && conf.CanSendEmail() {
		// HandleSetPasswordEmail will send a special password reset email that also
		// verifies the primary email address.
		ru, err := userpasswd.HandleSetPasswordEmail(ctx, db, user.ID, user.Username, email, emailVerified)
		if err != nil {
			msg := "failed to send set password email"
			logger.Error(msg, log.Error(err))
			return nil, errors.Wrap(err, msg)
		}
		return &ru, nil
	}

	resetURL, err := backend.MakePasswordResetURL(ctx, db, user.ID, email)
	if err != nil {
		msg := "failed to generate reset URL"
		logger.Error(msg, log.Error(err))
		return nil, errors.Wrap(err, msg)
	}

	ru := globals.ExternalURL().ResolveReference(resetURL).String()
	return &ru, nil
}
