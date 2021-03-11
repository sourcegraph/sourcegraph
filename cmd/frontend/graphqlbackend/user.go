package graphqlbackend

import (
	"context"
	"errors"
	"fmt"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/suspiciousnames"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func (r *schemaResolver) User(ctx context.Context, args struct {
	Username *string
	Email    *string
}) (*UserResolver, error) {
	switch {
	case args.Username != nil:
		user, err := database.GlobalUsers.GetByUsername(ctx, *args.Username)
		if err != nil {
			return nil, err
		}
		return NewUserResolver(r.db, user), nil

	case args.Email != nil:
		// 🚨 SECURITY: Only site admins are allowed to look up by email address on Sourcegraph.com, for
		// user privacy reasons.
		if envvar.SourcegraphDotComMode() {
			if err := backend.CheckCurrentUserIsSiteAdmin(ctx); err != nil {
				return nil, err
			}
		}
		user, err := database.GlobalUsers.GetByVerifiedEmail(ctx, *args.Email)
		if err != nil {
			return nil, err
		}
		return NewUserResolver(r.db, user), nil

	default:
		return nil, errors.New("must specify either username or email to look up user")
	}
}

// UserResolver implements the GraphQL User type.
type UserResolver struct {
	db   dbutil.DB
	user *types.User
}

// NewUserResolver returns a new UserResolver with given user object.
func NewUserResolver(db dbutil.DB, user *types.User) *UserResolver {
	return &UserResolver{db: db, user: user}
}

// UserByID looks up and returns the user with the given GraphQL ID. If no such user exists, it returns a
// non-nil error.
func UserByID(ctx context.Context, db dbutil.DB, id graphql.ID) (*UserResolver, error) {
	userID, err := UnmarshalUserID(id)
	if err != nil {
		return nil, err
	}
	return UserByIDInt32(ctx, db, userID)
}

// UserByIDInt32 looks up and returns the user with the given database ID. If no such user exists,
// it returns a non-nil error.
func UserByIDInt32(ctx context.Context, db dbutil.DB, id int32) (*UserResolver, error) {
	user, err := database.GlobalUsers.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewUserResolver(db, user), nil
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
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return "", err
	}

	email, _, err := database.GlobalUserEmails.GetPrimaryEmail(ctx, r.user.ID)
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

func (r *UserResolver) CreatedAt() DateTime {
	return DateTime{Time: r.user.CreatedAt}
}

func (r *UserResolver) UpdatedAt() *DateTime {
	return &DateTime{Time: r.user.UpdatedAt}
}

func (r *UserResolver) settingsSubject() api.SettingsSubject {
	return api.SettingsSubject{User: &r.user.ID}
}

func (r *UserResolver) LatestSettings(ctx context.Context) (*settingsResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's settings, because they
	// may contain secrets or other sensitive data.
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return nil, err
	}

	settings, err := database.GlobalSettings.GetLatest(ctx, r.settingsSubject())
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}
	return &settingsResolver{r.db, &settingsSubject{user: r}, settings, nil}, nil
}

func (r *UserResolver) SettingsCascade() *settingsCascade {
	return &settingsCascade{db: r.db, subject: &settingsSubject{user: r}}
}

func (r *UserResolver) ConfigurationCascade() *settingsCascade { return r.SettingsCascade() }

func (r *UserResolver) SiteAdmin(ctx context.Context) (bool, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to determine if the user is a site admin.
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return false, err
	}

	return r.user.SiteAdmin, nil
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
	if err := backend.CheckSiteAdminOrSameUser(ctx, userID); err != nil {
		return nil, err
	}

	if args.Username != nil {
		if err := suspiciousnames.CheckNameAllowedForUserOrOrganization(*args.Username); err != nil {
			return nil, err
		}
	}

	update := database.UserUpdate{
		DisplayName: args.DisplayName,
		AvatarURL:   args.AvatarURL,
	}
	if args.Username != nil && viewerIsChangingUsername(ctx, userID, *args.Username) {
		if !viewerCanChangeUsername(ctx, userID) {
			return nil, fmt.Errorf("unable to change username because auth.enableUsernameChanges is false in site configuration")
		}
		update.Username = *args.Username
	}
	if err := database.GlobalUsers.Update(ctx, userID, update); err != nil {
		return nil, err
	}
	return UserByIDInt32(ctx, r.db, userID)
}

// CurrentUser returns the authenticated user if any. If there is no authenticated user, it returns
// (nil, nil). If some other error occurs, then the error is returned.
func CurrentUser(ctx context.Context, db dbutil.DB) (*UserResolver, error) {
	user, err := database.GlobalUsers.GetByCurrentAuthUser(ctx)
	if err != nil {
		if errcode.IsNotFound(err) || err == database.ErrNoCurrentUser {
			return nil, nil
		}
		return nil, err
	}
	return NewUserResolver(db, user), nil
}

func (r *UserResolver) Organizations(ctx context.Context) (*orgConnectionStaticResolver, error) {
	orgs, err := database.GlobalOrgs.GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	c := orgConnectionStaticResolver{nodes: make([]*OrgResolver, len(orgs))}
	for i, org := range orgs {
		c.nodes[i] = &OrgResolver{r.db, org}
	}
	return &c, nil
}

func (r *UserResolver) Tags(ctx context.Context) ([]string, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's tags.
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return nil, err
	}
	return r.user.Tags, nil
}

func (r *UserResolver) SurveyResponses(ctx context.Context) ([]*surveyResponseResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's survey responses.
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return nil, err
	}

	responses, err := database.SurveyResponses(r.db).GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	surveyResponseResolvers := []*surveyResponseResolver{}
	for _, response := range responses {
		surveyResponseResolvers = append(surveyResponseResolvers, &surveyResponseResolver{r.db, response})
	}
	return surveyResponseResolvers, nil
}

func (r *UserResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); errcode.IsUnauthorized(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// UserURLForSiteAdminBilling is called to obtain the GraphQL User.urlForSiteAdminBilling value. It
// is only set if billing is implemented.
var UserURLForSiteAdminBilling func(ctx context.Context, userID int32) (*string, error)

func (r *UserResolver) URLForSiteAdminBilling(ctx context.Context) (*string, error) {
	if UserURLForSiteAdminBilling == nil {
		return nil, nil
	}
	return UserURLForSiteAdminBilling(ctx, r.user.ID)
}

func (r *UserResolver) NamespaceName() string { return r.user.Username }

func (r *UserResolver) PermissionsInfo(ctx context.Context) (PermissionsInfoResolver, error) {
	return EnterpriseResolvers.authzResolver.UserPermissionsInfo(ctx, r.ID())
}

func (r *schemaResolver) UpdatePassword(ctx context.Context, args *struct {
	OldPassword string
	NewPassword string
}) (*EmptyResponse, error) {
	// 🚨 SECURITY: A user can only change their own password.
	user, err := database.GlobalUsers.GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("no authenticated user")
	}

	if err := database.GlobalUsers.UpdatePassword(ctx, user.ID, args.OldPassword, args.NewPassword); err != nil {
		return nil, err
	}

	if conf.CanSendEmail() {
		if err := backend.UserEmails.SendUserEmailOnFieldUpdate(ctx, user.ID, "updated the password"); err != nil {
			log15.Warn("Failed to send email to inform user of password update", "error", err)
		}
	}
	return &EmptyResponse{}, nil
}

func (r *schemaResolver) CreatePassword(ctx context.Context, args *struct {
	NewPassword string
}) (*EmptyResponse, error) {
	// 🚨 SECURITY: A user can only create their own password.
	user, err := database.GlobalUsers.GetByCurrentAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("no authenticated user")
	}
	if err := database.GlobalUsers.CreatePassword(ctx, user.ID, args.NewPassword); err != nil {
		return nil, err
	}

	if conf.CanSendEmail() {
		if err := backend.UserEmails.SendUserEmailOnFieldUpdate(ctx, user.ID, "created a password"); err != nil {
			log15.Warn("Failed to send email to inform user of password creation", "error", err)
		}
	}
	return &EmptyResponse{}, nil
}

// ViewerCanChangeUsername returns if the current user can change the username of the user.
func (r *UserResolver) ViewerCanChangeUsername(ctx context.Context) bool {
	return viewerCanChangeUsername(ctx, r.user.ID)
}

// TODO(campaigns-deprecation):
func (r *UserResolver) Campaigns(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	id := r.ID()
	args.Namespace = &id
	return EnterpriseResolvers.batchChangesResolver.Campaigns(ctx, args)
}

func (r *UserResolver) BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	id := r.ID()
	args.Namespace = &id
	return EnterpriseResolvers.batchChangesResolver.BatchChanges(ctx, args)
}

type ListUserRepositoriesArgs struct {
	First             *int32
	Query             *string
	After             *string
	Cloned            bool
	NotCloned         bool
	Indexed           bool
	NotIndexed        bool
	ExternalServiceID *graphql.ID
	OrderBy           *string
	Descending        bool
}

func (r *UserResolver) Repositories(ctx context.Context, args *ListUserRepositoriesArgs) (RepositoryConnectionResolver, error) {
	opt := database.ReposListOptions{}
	if args.Query != nil {
		opt.Query = *args.Query
	}
	if args.First != nil {
		opt.LimitOffset = &database.LimitOffset{Limit: int(*args.First)}
	}
	if args.After != nil {
		cursor, err := unmarshalRepositoryCursor(args.After)
		if err != nil {
			return nil, err
		}
		opt.CursorColumn = cursor.Column
		opt.CursorValue = cursor.Value
		opt.CursorDirection = cursor.Direction
	} else {
		opt.CursorValue = ""
		opt.CursorDirection = "next"
	}
	if args.OrderBy != nil {
		opt.OrderBy = database.RepoListOrderBy{{
			Field:      toDBRepoListColumn(*args.OrderBy),
			Descending: args.Descending,
		}}
	}
	extSvcs, err := database.GlobalExternalServices.List(ctx, database.ExternalServicesListOptions{
		NamespaceUserID: r.user.ID,
	})
	if err != nil {
		return nil, err
	}

	if args.ExternalServiceID == nil {
		ids := make([]int64, 0, len(extSvcs))
		for _, svc := range extSvcs {
			ids = append(ids, svc.ID)
		}
		if len(ids) == 0 {
			ids = []int64{-1}
		}
		opt.ExternalServiceIDs = ids
	} else {
		id, err := unmarshalExternalServiceID(*args.ExternalServiceID)
		if err != nil {
			return nil, err
		}
		opt.ExternalServiceIDs = []int64{id}
	}

	return &repositoryConnectionResolver{
		db:         r.db,
		opt:        opt,
		cloned:     args.Cloned,
		notCloned:  args.NotCloned,
		indexed:    args.Indexed,
		notIndexed: args.NotIndexed,
	}, nil
}

func (r *UserResolver) CampaignsCodeHosts(ctx context.Context, args *ListCampaignsCodeHostsArgs) (CampaignsCodeHostConnectionResolver, error) {
	args.UserID = r.user.ID
	return EnterpriseResolvers.batchChangesResolver.CampaignsCodeHosts(ctx, args)
}

func (r *UserResolver) BatchChangesCodeHosts(ctx context.Context, args *ListBatchChangesCodeHostsArgs) (BatchChangesCodeHostConnectionResolver, error) {
	args.UserID = r.user.ID
	return EnterpriseResolvers.batchChangesResolver.BatchChangesCodeHosts(ctx, args)
}

func viewerCanChangeUsername(ctx context.Context, userID int32) bool {
	if err := backend.CheckSiteAdminOrSameUser(ctx, userID); err != nil {
		return false
	}
	if conf.Get().AuthEnableUsernameChanges {
		return true
	}
	// 🚨 SECURITY: Only site admins are allowed to change a user's username when auth.enableUsernameChanges == false.
	return backend.CheckCurrentUserIsSiteAdmin(ctx) == nil
}

// Users may be trying to change their own username, or someone else's.
//
// The subjectUserID value represents the decoded user ID from the incoming
// update request, and the proposedUsername is the value that would be applied
// to that subject's record if all security checks pass.
//
// If that subject's username is different from the proposed one, then a
// change is being attempted and may be rejected by viewerCanChangeUsername.
func viewerIsChangingUsername(ctx context.Context, subjectUserID int32, proposedUsername string) bool {
	subject, err := database.GlobalUsers.GetByID(ctx, subjectUserID)
	if err != nil {
		log15.Warn("viewerIsChangingUsername", "error", err)
		return true
	}
	return subject.Username != proposedUsername
}

func (r *UserResolver) Monitors(ctx context.Context, args *ListMonitorsArgs) (MonitorConnectionResolver, error) {
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return nil, err
	}
	return EnterpriseResolvers.codeMonitorsResolver.Monitors(ctx, r.user.ID, args)
}

func (r *UserResolver) PublicRepositories(ctx context.Context) ([]*RepositoryResolver, error) {
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.user.ID); err != nil {
		return nil, err
	}
	repos, err := database.UserPublicRepos(r.db).ListByUser(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	var out []*RepositoryResolver
	for _, repo := range repos {
		out = append(out, &RepositoryResolver{
			id:   repo.RepoID,
			name: api.RepoName(repo.RepoURI),
			db:   r.db,
			innerRepo: &types.Repo{
				ID: repo.RepoID,
			},
		})
	}
	return out, nil
}
