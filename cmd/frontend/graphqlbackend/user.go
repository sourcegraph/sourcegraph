package graphqlbackend

import (
	"context"
	"net/url"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/ssc"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/suspiciousnames"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *schemaResolver) User(
	ctx context.Context,
	args struct {
		Username   *string
		Email      *string
		DatabaseID *int32
	},
) (*UserResolver, error) {
	var err error
	var user *types.User
	switch {
	case args.Username != nil:
		user, err = r.db.Users().GetByUsername(ctx, *args.Username)

	case args.DatabaseID != nil:
		user, err = r.db.Users().GetByID(ctx, *args.DatabaseID)

	case args.Email != nil:
		if dotcom.IsUserAndOrgProfileDataPrivate() {
			// 🚨 SECURITY: Only site admins are allowed to look up by email address in this mode for user privacy reasons.
			if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
				return nil, err
			}
		}
		user, err = r.db.Users().GetByVerifiedEmail(ctx, *args.Email)
	default:
		return nil, errors.New("must specify either username or email to look up a user")
	}
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return NewUserResolver(ctx, r.db, user), nil
}

// UserResolver implements the GraphQL User type.
type UserResolver struct {
	logger log.Logger
	db     database.DB
	user   *types.User
	// actor containing current user (derived from request context), which lets us
	// skip fetching actor from context in every resolver function and save DB calls,
	// because user is fetched in actor only once.
	actor                      *actor.Actor
	codySubscription           *cody.UserSubscription
	fetchCodySubscriptionError error
	fetchCodySubscriptionOnce  sync.Once
}

// NewUserResolver returns a new UserResolver with given user object.
func NewUserResolver(ctx context.Context, db database.DB, user *types.User) *UserResolver {
	return &UserResolver{
		db:     db,
		user:   user,
		logger: log.Scoped("userResolver").With(log.String("user", user.Username)),
		actor:  actor.FromContext(ctx),
	}
}

// newUserResolverFromActor returns a new UserResolver with given user object.
func newUserResolverFromActor(a *actor.Actor, db database.DB, user *types.User) *UserResolver {
	return &UserResolver{
		db:     db,
		user:   user,
		logger: log.Scoped("userResolver").With(log.String("user", user.Username)),
		actor:  a,
	}
}

// UserByID looks up and returns the user with the given GraphQL ID. If no such user exists, it returns a
// non-nil error.
func UserByID(ctx context.Context, db database.DB, id graphql.ID) (*UserResolver, error) {
	userID, err := UnmarshalUserID(id)
	if err != nil {
		return nil, err
	}
	return UserByIDInt32(ctx, db, userID)
}

// UserByIDInt32 looks up and returns the user with the given database ID. If no such user exists,
// it returns a non-nil error.
func UserByIDInt32(ctx context.Context, db database.DB, id int32) (*UserResolver, error) {
	user, err := db.Users().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewUserResolver(ctx, db, user), nil
}

func (r *UserResolver) ID() graphql.ID { return MarshalUserID(r.user.ID) }

func MarshalUserID(id int32) graphql.ID { return relay.MarshalID("User", id) }

func UnmarshalUserID(id graphql.ID) (userID int32, err error) {
	err = relay.UnmarshalSpec(id, &userID)
	return
}

// DatabaseID returns the numeric ID for the user in the database.
func (r *UserResolver) DatabaseID() int32 { return r.user.ID }

// Email returns the user's oldest email, if one exists.
// Deprecated: use Emails instead.
func (r *UserResolver) Email(ctx context.Context) (string, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the email address.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return "", err
	}

	email, _, err := r.db.UserEmails().GetPrimaryEmail(ctx, r.user.ID)
	if err != nil && !errcode.IsNotFound(err) {
		return "", err
	}

	return email, nil
}

func (r *UserResolver) Username() string { return r.user.Username }

func (r *UserResolver) DisplayName() *string {
	if r.user.DisplayName == "" {
		return nil
	}
	return &r.user.DisplayName
}

func (r *UserResolver) BuiltinAuth() bool {
	return r.user.BuiltinAuth && providers.BuiltinAuthEnabled()
}

func (r *UserResolver) AvatarURL() *string {
	if r.user.AvatarURL == "" {
		return nil
	}
	return &r.user.AvatarURL
}

func (r *UserResolver) URL() string {
	return "/users/" + r.user.Username
}

func (r *UserResolver) SettingsURL() *string { return strptr(r.URL() + "/settings") }

type CodySubscriptionResolver struct {
	subscription *cody.UserSubscription
}

func (r *CodySubscriptionResolver) Status() ssc.SubscriptionStatus {
	return r.subscription.Status
}

func (r *CodySubscriptionResolver) Plan() cody.UserSubscriptionPlan {
	return r.subscription.Plan
}

func (r *CodySubscriptionResolver) ApplyProRateLimits() bool {
	return r.subscription.ApplyProRateLimits
}

func (r *CodySubscriptionResolver) CurrentPeriodStartAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.subscription.CurrentPeriodStartAt}
}

func (r *CodySubscriptionResolver) CurrentPeriodEndAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.subscription.CurrentPeriodEndAt}
}

func (r *CodySubscriptionResolver) CancelAtPeriodEnd() bool {
	return r.subscription.CancelAtPeriodEnd
}

func (r *UserResolver) fetchCodySubscription(ctx context.Context) (*cody.UserSubscription, error) {
	r.fetchCodySubscriptionOnce.Do(func() {
		subscription, err := cody.SubscriptionForUser(ctx, r.db, *r.user)
		if err != nil {
			r.fetchCodySubscriptionError = err
			return
		}

		r.codySubscription = subscription
	})

	if r.fetchCodySubscriptionError != nil {
		return nil, r.fetchCodySubscriptionError
	}

	return r.codySubscription, r.fetchCodySubscriptionError
}

func (r *UserResolver) CodySubscription(ctx context.Context) (*CodySubscriptionResolver, error) {
	if !dotcom.SourcegraphDotComMode() {
		return nil, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return nil, err
	}

	return &CodySubscriptionResolver{subscription: subscription}, nil
}

func (r *UserResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.user.CreatedAt}
}

func (r *UserResolver) CodyProEnabled(ctx context.Context) (bool, error) {
	if !dotcom.SourcegraphDotComMode() {
		return false, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return false, err
	}

	return subscription.Plan == cody.UserSubscriptionPlanPro, nil
}

func (r *UserResolver) CodyCurrentPeriodChatLimit(ctx context.Context) (int32, error) {
	if !dotcom.SourcegraphDotComMode() {
		return 0, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return 0, err
	}

	if subscription.ApplyProRateLimits {
		return 0, nil
	}

	cfg := conf.GetCompletionsConfig(conf.Get().SiteConfig())

	limit := int32(cfg.PerCommunityUserChatMonthlyInteractionLimit)

	return limit, nil
}

func (r *UserResolver) CodyCurrentPeriodCodeLimit(ctx context.Context) (int32, error) {
	if !dotcom.SourcegraphDotComMode() {
		return 0, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return 0, err
	}

	if subscription.ApplyProRateLimits {
		return 0, nil
	}

	cfg := conf.GetCompletionsConfig(conf.Get().SiteConfig())

	limit := int32(cfg.PerCommunityUserCodeCompletionsMonthlyInteractionLimit)

	return limit, nil
}

func (r *UserResolver) CodyCurrentPeriodChatUsage(ctx context.Context) (int32, error) {
	if !dotcom.SourcegraphDotComMode() {
		return 0, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return 0, err
	}

	query := sqlf.Sprintf(`
		WHERE
			user_id = %s
			AND timestamp >= %s
			AND timestamp <= NOW()
			AND name NOT LIKE '%%clicked%%'
			AND name NOT LIKE '%%resetChat%%'
			AND name NOT LIKE '%%menu:opened%%'
			AND name NOT LIKE 'cody.%%'
			AND (
					(
						SOURCE = 'IDEEXTENSION'
						AND (
							name LIKE '%%recipe%%'
							OR name LIKE '%%command%%'
							OR name LIKE '%%chat:executed%%'
							OR name LIKE '%%chat-question:executed%%'
							OR name LIKE '%%inline-chat%%'
							OR name LIKE '%%inline-assist%%'
							OR name LIKE '%%chat:submitted%%'
						)
					)

					OR

					(
						SOURCE = 'WEB'
						AND (
							name = 'web:codyChat:submit'
							OR name LIKE 'web:codyChat:recipe:%%'
						)
					)
			)
	`, r.user.ID, subscription.CurrentPeriodStartAt)

	count, err := r.db.EventLogs().CountBySQL(ctx, query)

	return int32(count), err
}

func (r *UserResolver) CodyCurrentPeriodCodeUsage(ctx context.Context) (int32, error) {
	if !dotcom.SourcegraphDotComMode() {
		return 0, errors.New("this feature is only available on sourcegraph.com")
	}

	subscription, err := r.fetchCodySubscription(ctx)
	if err != nil {
		return 0, err
	}

	query := sqlf.Sprintf(`
		WHERE
			user_id = %s
			AND timestamp >= %s
			AND timestamp <= NOW()
			AND source = 'IDEEXTENSION'
			AND name LIKE '%%completion:suggested%%'
	`, r.user.ID, subscription.CurrentPeriodStartAt)

	count, err := r.db.EventLogs().CountBySQL(ctx, query)

	return int32(count), err
}

func (r *UserResolver) UpdatedAt() *gqlutil.DateTime {
	return &gqlutil.DateTime{Time: r.user.UpdatedAt}
}

func (r *UserResolver) settingsSubject() api.SettingsSubject {
	return api.SettingsSubject{User: &r.user.ID}
}

func (r *UserResolver) LatestSettings(ctx context.Context) (*settingsResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's
	// settings, because they may contain secrets or other sensitive data.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return nil, err
	}

	settings, err := r.db.Settings().GetLatest(ctx, r.settingsSubject())
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}
	return &settingsResolver{r.db, &settingsSubjectResolver{user: r}, settings, nil}, nil
}

func (r *UserResolver) SettingsCascade() *settingsCascade {
	return &settingsCascade{db: r.db, subject: &settingsSubjectResolver{user: r}}
}

func (r *UserResolver) ConfigurationCascade() *settingsCascade { return r.SettingsCascade() }

func (r *UserResolver) SiteAdmin() (bool, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to determine if the user is a site admin.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return false, err
	}

	return r.user.SiteAdmin, nil
}

func (r *UserResolver) TosAccepted(_ context.Context) bool {
	return r.user.TosAccepted
}

func (r *UserResolver) CompletedPostSignup(ctx context.Context) (bool, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to state of
	// post-signup flow completion.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return false, err
	}

	return r.user.CompletedPostSignup, nil
}

type updateUserArgs struct {
	User        graphql.ID
	Username    *string
	DisplayName *string
	AvatarURL   *string
}

func (r *schemaResolver) UpdateUser(ctx context.Context, args *updateUserArgs) (*UserResolver, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Only the user and site admins are allowed to update the user.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, userID); err != nil {
		return nil, err
	}

	if args.Username != nil {
		if err := suspiciousnames.CheckNameAllowedForUserOrOrganization(*args.Username); err != nil {
			return nil, err
		}
	}

	if args.AvatarURL != nil && len(*args.AvatarURL) > 0 {
		if len(*args.AvatarURL) > 3000 {
			return nil, errors.New("avatar URL exceeded 3000 characters")
		}

		u, err := url.Parse(*args.AvatarURL)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse avatar URL")
		} else if u.Scheme != "http" && u.Scheme != "https" {
			return nil, errors.New("avatar URL must be an HTTP or HTTPS URL")
		}
	}

	update := database.UserUpdate{
		AvatarURL: args.AvatarURL,
	}

	user, err := r.db.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "getting user from the database")
	}

	usernameChanged := args.Username != nil && user.Username != *args.Username

	// Check if the user account is SCIM controlled
	if user.SCIMControlled {
		errUserScimManaged := errors.Errorf("cannot update externally managed user")

		if usernameChanged {
			return nil, errUserScimManaged
		}

		if args.DisplayName != nil && user.DisplayName != *args.DisplayName {
			return nil, errUserScimManaged
		}
	}

	update.DisplayName = args.DisplayName

	// If user is changing their username, we need to verify if this action can be
	// done.
	if usernameChanged {
		if !viewerCanChangeUsername(actor.FromContext(ctx), r.db, userID) {
			return nil, errors.Errorf("unable to change username because auth.enableUsernameChanges is false in site configuration")
		}
		update.Username = *args.Username
	}

	if err := r.db.Users().Update(ctx, userID, update); err != nil {
		return nil, err
	}
	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {
		// Log an event when a user account is modified/updated
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameAccountModified, "", uint32(userID), "", "BACKEND", args); err != nil {
			r.logger.Error("Error logging security event", log.Error(err))
		}
	}
	return UserByIDInt32(ctx, r.db, userID)
}

type changeCodyPlanArgs struct {
	User graphql.ID
	Pro  bool
}

func (r *schemaResolver) ChangeCodyPlan(ctx context.Context, args *changeCodyPlanArgs) (*UserResolver, error) {
	if !dotcom.SourcegraphDotComMode() {
		return nil, errors.New("this feature is only available on sourcegraph.com")
	}

	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only the authenticated user can update their properties.
	if err := auth.CheckSameUser(ctx, userID); err != nil {
		return nil, err
	}

	if err := r.db.Users().ChangeCodyPlan(ctx, userID, args.Pro); err != nil {
		return nil, err
	}

	if err, _ := cody.RefreshGatewayRateLimits(ctx, userID, r.db); err != nil {
		// We intentionally don't fail the upgrade flow here, Gateway will pickup
		// the new limits automatically. (Just later than we'd like.)
		r.logger.Warn("refresh gateway limits", log.Error(err))
	}

	return UserByIDInt32(ctx, r.db, userID)
}

// CurrentUser returns the authenticated user if any. If there is no authenticated user, it returns
// (nil, nil). If some other error occurs, then the error is returned.
func CurrentUser(ctx context.Context, db database.DB) (*UserResolver, error) {
	user, err := db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		if errcode.IsNotFound(err) || err == database.ErrNoCurrentUser {
			return nil, nil
		}
		return nil, err
	}
	return newUserResolverFromActor(actor.FromActualUser(user), db, user), nil
}

func (r *UserResolver) Organizations(ctx context.Context) (*orgConnectionStaticResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's
	// organisations.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return nil, err
	}
	orgs, err := r.db.Orgs().GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	c := orgConnectionStaticResolver{nodes: make([]*OrgResolver, len(orgs))}
	for i, org := range orgs {
		c.nodes[i] = &OrgResolver{r.db, org}
	}
	return &c, nil
}

func (r *UserResolver) SurveyResponses(ctx context.Context) ([]*surveyResponseResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's survey responses.
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID); err != nil {
		return nil, err
	}

	responses, err := database.SurveyResponses(r.db).GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	surveyResponseResolvers := make([]*surveyResponseResolver, 0, len(responses))
	for _, response := range responses {
		surveyResponseResolvers = append(surveyResponseResolvers, &surveyResponseResolver{r.db, response})
	}
	return surveyResponseResolvers, nil
}

func (r *UserResolver) ViewerCanAdminister() (bool, error) {
	err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID)
	if errcode.IsUnauthorized(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (r *UserResolver) viewerCanAdministerSettings() (bool, error) {
	err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, r.user.ID)

	if errcode.IsUnauthorized(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (r *UserResolver) NamespaceName() string { return r.user.Username }

func (r *UserResolver) SCIMControlled() bool { return r.user.SCIMControlled }

func (r *UserResolver) PermissionsInfo(ctx context.Context) (PermissionsInfoResolver, error) {
	return EnterpriseResolvers.authzResolver.UserPermissionsInfo(ctx, r.ID())
}

func (r *schemaResolver) UpdatePassword(ctx context.Context, args *struct {
	OldPassword string
	NewPassword string
},
) (*EmptyResponse, error) {
	// 🚨 SECURITY: Only the authenticated user can change their password.
	user, err := r.db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("no authenticated user")
	}

	if err := r.db.Users().UpdatePassword(ctx, user.ID, args.OldPassword, args.NewPassword); err != nil {
		return nil, err
	}

	logger := r.logger.Scoped("UpdatePassword").
		With(log.Int32("userID", user.ID))

	if conf.CanSendEmail() {
		if err := backend.NewUserEmailsService(r.db, logger).SendUserEmailOnFieldUpdate(ctx, user.ID, "updated the password"); err != nil {
			logger.Warn("Failed to send email to inform user of password update", log.Error(err))
		}
	}
	return &EmptyResponse{}, nil
}

func (r *schemaResolver) CreatePassword(ctx context.Context, args *struct {
	NewPassword string
},
) (*EmptyResponse, error) {
	// 🚨 SECURITY: Only the authenticated user can create their password.
	if !actor.FromContext(ctx).FromSessionCookie {
		return nil, errors.New("only allowed from user session")
	}

	user, err := r.db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("no authenticated user")
	}

	if err := r.db.Users().CreatePassword(ctx, user.ID, args.NewPassword); err != nil {
		return nil, err
	}

	logger := r.logger.Scoped("CreatePassword").
		With(log.Int32("userID", user.ID))

	if conf.CanSendEmail() {
		if err := backend.NewUserEmailsService(r.db, logger).SendUserEmailOnFieldUpdate(ctx, user.ID, "created a password"); err != nil {
			logger.Warn("Failed to send email to inform user of password creation", log.Error(err))
		}
	}
	return &EmptyResponse{}, nil
}

// userMutationArgs hold an optional user ID for mutations that take in a user ID
// or assume the current user when no ID is given.
type userMutationArgs struct {
	UserID *graphql.ID
}

func (r *schemaResolver) affectedUserID(ctx context.Context, args *userMutationArgs) (affectedUserID int32, err error) {
	if args.UserID != nil {
		return UnmarshalUserID(*args.UserID)
	}

	user, err := r.db.Users().GetByCurrentAuthUser(ctx)
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (r *schemaResolver) SetTosAccepted(ctx context.Context, args *userMutationArgs) (*EmptyResponse, error) {
	affectedUserID, err := r.affectedUserID(ctx, args)
	if err != nil {
		return nil, err
	}

	tosAccepted := true
	update := database.UserUpdate{TosAccepted: &tosAccepted}
	return r.updateAffectedUser(ctx, affectedUserID, update)
}

func (r *schemaResolver) SetCompletedPostSignup(ctx context.Context, args *userMutationArgs) (*EmptyResponse, error) {
	affectedUserID, err := r.affectedUserID(ctx, args)
	if err != nil {
		return nil, err
	}

	has, err := backend.NewUserEmailsService(r.db, r.logger).HasVerifiedEmail(ctx, affectedUserID)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, errors.New("must have a verified email to complete post-signup flow")
	}

	completedPostSignup := true
	update := database.UserUpdate{CompletedPostSignup: &completedPostSignup}
	return r.updateAffectedUser(ctx, affectedUserID, update)
}

func (r *schemaResolver) updateAffectedUser(ctx context.Context, affectedUserID int32, update database.UserUpdate) (*EmptyResponse, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to set the Terms of Service accepted flag.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, affectedUserID); err != nil {
		return nil, err
	}

	if err := r.db.Users().Update(ctx, affectedUserID, update); err != nil {
		return nil, err
	}

	return &EmptyResponse{}, nil
}

// ViewerCanChangeUsername returns if the current user can change the username of the user.
func (r *UserResolver) ViewerCanChangeUsername() bool {
	return viewerCanChangeUsername(r.actor, r.db, r.user.ID)
}

func (r *UserResolver) CompletionsQuotaOverride(ctx context.Context) (*int32, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to see quotas.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}

	v, err := r.db.Users().GetChatCompletionsQuota(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	iv := int32(*v)
	return &iv, nil
}

func (r *UserResolver) CodeCompletionsQuotaOverride(ctx context.Context) (*int32, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to see quotas.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}

	v, err := r.db.Users().GetCodeCompletionsQuota(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}

	if v == nil {
		return nil, nil
	}

	iv := int32(*v)
	return &iv, nil
}

func (r *UserResolver) BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	id := r.ID()
	args.Namespace = &id
	return EnterpriseResolvers.batchChangesResolver.BatchChanges(ctx, args)
}

func (r *UserResolver) BatchChangesCodeHosts(ctx context.Context, args *ListBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error) {
	args.UserID = &r.user.ID
	return EnterpriseResolvers.batchChangesResolver.BatchChangesCodeHosts(ctx, args)
}

func (r *UserResolver) Roles(ctx context.Context, args *ListRoleArgs) (*graphqlutil.ConnectionResolver[RoleResolver], error) {
	// 🚨 SECURITY: Only allow site admins or the user to check the user's roles.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}
	userID := r.user.ID
	connectionStore := &roleConnectionStore{
		db:     r.db,
		userID: userID,
	}
	return graphqlutil.NewConnectionResolver[RoleResolver](
		connectionStore,
		&args.ConnectionResolverArgs,
		&graphqlutil.ConnectionResolverOptions{
			AllowNoLimit: true,
		},
	)
}

func (r *UserResolver) Permissions() (*graphqlutil.ConnectionResolver[PermissionResolver], error) {
	userID := r.user.ID
	if err := auth.CheckSiteAdminOrSameUserFromActor(r.actor, r.db, userID); err != nil {
		return nil, err
	}
	connectionStore := &permissionConnectionStore{
		db:     r.db,
		userID: userID,
	}
	return graphqlutil.NewConnectionResolver[PermissionResolver](
		connectionStore,
		&graphqlutil.ConnectionResolverArgs{},
		&graphqlutil.ConnectionResolverOptions{
			AllowNoLimit: true,
		},
	)
}

func viewerCanChangeUsername(a *actor.Actor, db database.DB, userID int32) bool {
	if err := auth.CheckSiteAdminOrSameUserFromActor(a, db, userID); err != nil {
		return false
	}
	if conf.Get().AuthEnableUsernameChanges {
		return true
	}

	// 🚨 SECURITY: Only site admins are allowed to change a user's username when auth.enableUsernameChanges == false.
	return auth.CheckCurrentActorIsSiteAdmin(a, db) == nil
}

func (r *UserResolver) Monitors(ctx context.Context, args *ListMonitorsArgs) (MonitorConnectionResolver, error) {
	if err := auth.CheckSameUserFromActor(r.actor, r.user.ID); err != nil {
		return nil, err
	}
	return EnterpriseResolvers.codeMonitorsResolver.Monitors(ctx, &r.user.ID, args)
}

func (r *UserResolver) ToUser() (*UserResolver, bool) {
	return r, true
}

func (r *UserResolver) OwnerField() string {
	return EnterpriseResolvers.ownResolver.UserOwnerField(r)
}

type SetUserCompletionsQuotaArgs struct {
	User  graphql.ID
	Quota *int32
}

func (r *schemaResolver) SetUserCompletionsQuota(ctx context.Context, args SetUserCompletionsQuotaArgs) (*UserResolver, error) {
	// 🚨 SECURITY: Only site admins are allowed to change a users quota.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	requestingActor := actor.FromContext(ctx)

	if args.Quota != nil && *args.Quota <= 0 {
		return nil, errors.New("quota must be 1 or greater")
	}

	id, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	// Verify the ID is valid.
	user, err := r.db.Users().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Lookup the current quota, so we can log the delta.
	oldQuota, err := r.db.Users().GetChatCompletionsQuota(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	var newQuota *int
	if args.Quota != nil {
		i := int(*args.Quota)
		newQuota = &i
	}
	if err := r.db.Users().SetChatCompletionsQuota(ctx, user.ID, newQuota); err != nil {
		return nil, err
	}

	// Log that the user's completions quota was updated.
	r.logger.Info("setting user completions quota",
		log.Int("requestingUserID", int(requestingActor.UID)),
		log.Int("targetUserID", int(user.ID)),
		log.Intp("oldQuota", oldQuota),
		log.Intp("newQuota", newQuota))
	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameUserCompletionQuotaUpdated, "", uint32(id), "", "BACKEND", args); err != nil {
			r.logger.Error("Error logging security event", log.Error(err))
		}
	}
	return UserByIDInt32(ctx, r.db, user.ID)
}

type SetUserCodeCompletionsQuotaArgs struct {
	User  graphql.ID
	Quota *int32
}

func (r *schemaResolver) SetUserCodeCompletionsQuota(ctx context.Context, args SetUserCodeCompletionsQuotaArgs) (*UserResolver, error) {
	// 🚨 SECURITY: Only site admins are allowed to change a users quota.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	if args.Quota != nil && *args.Quota <= 0 {
		return nil, errors.New("quota must be 1 or greater")
	}

	id, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	// Verify the ID is valid.
	user, err := r.db.Users().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var quota *int
	if args.Quota != nil {
		i := int(*args.Quota)
		quota = &i
	}
	if err := r.db.Users().SetCodeCompletionsQuota(ctx, user.ID, quota); err != nil {
		return nil, err
	}
	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

		// Log an event when user's code completions quota is updated
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameUserCodeCompletionQuotaUpdated, "", uint32(id), "", "BACKEND", args); err != nil {
			r.logger.Error("Error logging security event", log.Error(err))
		}
	}
	return UserByIDInt32(ctx, r.db, user.ID)
}

func (r *UserResolver) EvaluateFeatureFlag(ctx context.Context, args *struct {
	FlagName string
}) (*bool, error) {
	ffs, err := r.db.FeatureFlags().GetUserFlags(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	if v, ok := ffs[args.FlagName]; ok {
		return &v, nil
	}
	// If there is no value for this feature flag, then we return nil. This follows the existing behaviour from the root level evaluateFeatureFlag function.
	return nil, nil
}
