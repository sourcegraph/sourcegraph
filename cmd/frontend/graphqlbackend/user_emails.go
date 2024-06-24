package graphqlbackend

import (
	"context"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
)

var timeNow = time.Now

func (r *UserResolver) HasVerifiedEmail(ctx context.Context) (bool, error) {
	// 🚨 SECURITY: In the UserEmailsService we check that only the
	// authenticated user and site admins can check
	// whether the user has a verified email.
	return backend.NewUserEmailsService(r.db, r.logger).HasVerifiedEmail(ctx, r.user.ID)
}

func (r *UserResolver) Emails(ctx context.Context) ([]*userEmailResolver, error) {
	// 🚨 SECURITY: Only the authenticated user and site admins can list user's
	// emails on multi-tenant instances.
	if !dotcom.IsUserAndOrgProfileDataPrivate() {
		if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
			return nil, err
		}
	}

	userEmails, err := r.db.UserEmails().ListByUser(ctx, database.UserEmailsListOptions{
		UserID: r.user.ID,
	})
	if err != nil {
		return nil, err
	}

	rs := make([]*userEmailResolver, len(userEmails))
	for i, userEmail := range userEmails {
		rs[i] = &userEmailResolver{
			db:        r.db,
			userEmail: *userEmail,
			user:      r,
		}
	}
	return rs, nil
}

func (r *UserResolver) PrimaryEmail(ctx context.Context) (*userEmailResolver, error) {
	// 🚨 SECURITY: Only the authenticated user and site admins can list the user's emails on
	// multi-tenant instances. We don't return an error, but we don't show the email either.
	if !dotcom.IsUserAndOrgProfileDataPrivate() {
		if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
			return nil, nil
		}
	}
	ms, err := r.db.UserEmails().ListByUser(ctx, database.UserEmailsListOptions{
		UserID:       r.user.ID,
		OnlyVerified: true,
	})
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		if m.Primary {
			return &userEmailResolver{
				db:        r.db,
				userEmail: *m,
				user:      r,
			}, nil
		}
	}
	return nil, nil
}

type userEmailResolver struct {
	db        database.DB
	userEmail database.UserEmail
	user      *UserResolver
}

func (r *userEmailResolver) Email() string { return r.userEmail.Email }

func (r *userEmailResolver) IsPrimary() bool { return r.userEmail.Primary }

func (r *userEmailResolver) Verified() bool { return r.userEmail.VerifiedAt != nil }
func (r *userEmailResolver) VerificationPending() bool {
	return !r.Verified() && conf.EmailVerificationRequired()
}
func (r *userEmailResolver) User() *UserResolver { return r.user }

func (r *userEmailResolver) ViewerCanManuallyVerify(ctx context.Context) (bool, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err == auth.ErrNotAuthenticated || err == auth.ErrMustBeSiteAdmin {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

type addUserEmailArgs struct {
	User  graphql.ID
	Email string
}

func (r *schemaResolver) AddUserEmail(ctx context.Context, args *addUserEmailArgs) (*EmptyResponse, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	logger := r.logger.Scoped("AddUserEmail").
		With(log.Int32("userID", userID))

	userEmails := backend.NewUserEmailsService(r.db, logger)
	if err := userEmails.Add(ctx, userID, args.Email); err != nil {
		return nil, err
	}

	if conf.CanSendEmail() {
		if err := userEmails.SendUserEmailOnFieldUpdate(ctx, userID, "added an email"); err != nil {
			logger.Warn("Failed to send email to inform user of email addition", log.Error(err))
		}
	}

	return &EmptyResponse{}, nil
}

type removeUserEmailArgs struct {
	User  graphql.ID
	Email string
}

func (r *schemaResolver) RemoveUserEmail(ctx context.Context, args *removeUserEmailArgs) (*EmptyResponse, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	userEmails := backend.NewUserEmailsService(r.db, r.logger)
	if err := userEmails.Remove(ctx, userID, args.Email); err != nil {
		return nil, err
	}

	return &EmptyResponse{}, nil
}

type setUserEmailPrimaryArgs struct {
	User  graphql.ID
	Email string
}

func (r *schemaResolver) SetUserEmailPrimary(ctx context.Context, args *setUserEmailPrimaryArgs) (*EmptyResponse, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	userEmails := backend.NewUserEmailsService(r.db, r.logger)
	if err := userEmails.SetPrimaryEmail(ctx, userID, args.Email); err != nil {
		return nil, err
	}

	return &EmptyResponse{}, nil
}

type setUserEmailVerifiedArgs struct {
	User     graphql.ID
	Email    string
	Verified bool
}

func (r *schemaResolver) SetUserEmailVerified(ctx context.Context, args *setUserEmailVerifiedArgs) (*EmptyResponse, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	userEmails := backend.NewUserEmailsService(r.db, r.logger)
	if err := userEmails.SetVerified(ctx, userID, args.Email, args.Verified); err != nil {
		return nil, err
	}

	return &EmptyResponse{}, nil
}

type resendVerificationEmailArgs struct {
	User  graphql.ID
	Email string
}

func (r *schemaResolver) ResendVerificationEmail(ctx context.Context, args *resendVerificationEmailArgs) (*EmptyResponse, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	userEmails := backend.NewUserEmailsService(r.db, r.logger)
	if err := userEmails.ResendVerificationEmail(ctx, userID, args.Email, timeNow()); err != nil {
		return nil, err
	}

	return &EmptyResponse{}, nil
}
