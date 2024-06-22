package userpasswd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/cookie"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/txemail"
)

func SendResetPasswordURLEmail(ctx context.Context, email, username string, resetURL *url.URL) error {
	// Configure the template
	emailTemplate := defaultResetPasswordEmailTemplates
	if customTemplates := conf.SiteConfig().EmailTemplates; customTemplates != nil {
		emailTemplate = txemail.FromSiteConfigTemplateWithDefault(customTemplates.ResetPassword, emailTemplate)
	}

	return txemail.Send(ctx, "password_reset", txemail.Message{
		To:       []string{email},
		Template: emailTemplate,
		Data: SetPasswordEmailTemplateData{
			Username: username,
			URL:      globals.ExternalURL().ResolveReference(resetURL).String(),
			Host:     globals.ExternalURL().Host,
		},
	})
}

// HandleResetPasswordInit initiates the builtin-auth password reset flow by sending a password-reset email.
func HandleResetPasswordInit(logger log.Logger, db database.DB) http.HandlerFunc {
	logger = logger.Scoped("HandleResetPasswordInit")
	return func(w http.ResponseWriter, r *http.Request) {
		if handleEnabledCheck(logger, w) {
			return
		}
		if handleNotAuthenticatedCheck(w, r) {
			return
		}
		if !conf.CanSendEmail() {
			httpLogError(logger.Error, w, "Unable to reset password because email sending is not configured on this site", http.StatusNotFound)
			return
		}

		ctx := r.Context()
		var formData struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&formData); err != nil {
			httpLogError(logger.Error, w, "Could not decode password reset request body", http.StatusBadRequest, log.Error(err))
			return
		}

		if formData.Email == "" {
			httpLogError(logger.Warn, w, "No email specified in password reset request", http.StatusBadRequest)
			return
		}

		usr, err := db.Users().GetByVerifiedEmail(ctx, formData.Email)
		if err != nil {
			// 🚨 SECURITY: We don't show an error message when the user is not found
			// as to not leak the existence of a given e-mail address in the database.
			if !errcode.IsNotFound(err) {
				httpLogError(logger.Warn, w, "Failed to lookup user", http.StatusInternalServerError)
			}
			return
		}

		resetURL, err := backend.MakePasswordResetURL(ctx, db, usr.ID, formData.Email)
		if err == database.ErrPasswordResetRateLimit {
			httpLogError(logger.Warn, w, "Too many password reset requests. Try again in a few minutes.", http.StatusTooManyRequests, log.Error(err))
			return
		} else if err != nil {
			httpLogError(logger.Error, w, "Could not reset password", http.StatusBadRequest, log.Error(err))
			return
		}

		if err := SendResetPasswordURLEmail(r.Context(), formData.Email, usr.Username, resetURL); err != nil {
			httpLogError(logger.Error, w, "Could not send reset password email", http.StatusInternalServerError, log.Error(err))
			return
		}
		database.LogPasswordEvent(ctx, db, r, database.SecurityEventNamPasswordResetRequested, usr.ID)
	}
}

// HandleResetPasswordCode resets the password if the correct code is provided, and also
// verifies emails if the appropriate parameters are found.
func HandleResetPasswordCode(logger log.Logger, db database.DB) http.HandlerFunc {
	logger = logger.Scoped("HandleResetPasswordCode")

	return func(w http.ResponseWriter, r *http.Request) {
		if handleEnabledCheck(logger, w) {
			return
		}
		if handleNotAuthenticatedCheck(w, r) {
			return
		}

		ctx := r.Context()
		var params struct {
			UserID          int32  `json:"userID"`
			Code            string `json:"code"`
			Email           string `json:"email"`
			EmailVerifyCode string `json:"emailVerifyCode"`
			Password        string `json:"password"` // new password
		}
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			httpLogError(logger.Error, w, "Password reset with code: could not decode request body", http.StatusBadGateway, log.Error(err))
			return
		}
		verifyEmail := params.Email != "" && params.EmailVerifyCode != ""
		logger = logger.With(
			log.Int32("userID", params.UserID),
			log.Bool("verifyEmail", verifyEmail))

		if err := database.CheckPassword(params.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		logger.Info("handling password reset")

		success, err := db.Users().SetPassword(ctx, params.UserID, params.Code, params.Password)
		if err != nil {
			httpLogError(logger.Error, w, "Unexpected error", http.StatusInternalServerError, log.Error(err))
			return
		}

		if !success {
			http.Error(w, "Password reset code was invalid or expired.", http.StatusUnauthorized)
			return
		}

		database.LogPasswordEvent(ctx, db, r, database.SecurityEventNamePasswordChanged, params.UserID)

		if verifyEmail {
			ok, err := db.UserEmails().Verify(ctx, params.UserID, params.Email, params.EmailVerifyCode)
			if err != nil {
				logger.Error("failed to verify email", log.Error(err))
			} else if !ok {
				logger.Warn("got invalid email verification code")
			} else {

				anonymousID, _ := cookie.AnonymousUID(r)
				if err := db.SecurityEventLogs().LogSecurityEvent(r.Context(), database.SecurityEventNameEmailVerified, r.URL.Path, uint32(params.UserID), anonymousID, "BACKEND", params.Email); err != nil {
					logger.Warn("Error logging security event", log.Error(err))
				}

			}
		}

		if conf.CanSendEmail() {
			if err := backend.NewUserEmailsService(db, logger).SendUserEmailOnFieldUpdate(ctx, params.UserID, "reset the password"); err != nil {
				logger.Warn("Failed to send email to inform user of password reset", log.Error(err))
			}
		}
	}
}

func handleNotAuthenticatedCheck(w http.ResponseWriter, r *http.Request) (handled bool) {
	if actor.FromContext(r.Context()).IsAuthenticated() {
		http.Error(w, "Authenticated users may not perform password reset.", http.StatusInternalServerError)
		return true
	}
	return false
}
