package graphqlbackend

import (
	"context"
	"fmt"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/suspiciousnames"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *schemaResolver) Organization(ctx context.Context, args struct{ Name string }) (*OrgResolver, error) {
	org, err := r.db.Orgs().GetByName(ctx, args.Name)
	if err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Only org members can get org details on Cloud
	if dotcom.SourcegraphDotComMode() {
		hasAccess := func() error {
			if auth.CheckOrgAccess(ctx, r.db, org.ID) == nil {
				return nil
			}

			if a := sgactor.FromContext(ctx); a.IsAuthenticated() {
				_, err = r.db.OrgInvitations().GetPending(ctx, org.ID, a.UID)
				if err == nil {
					return nil
				}
			}

			// NOTE: We want to present a unified error to unauthorized users to prevent
			// them from differentiating service states by different error messages.
			return &database.OrgNotFoundError{Message: fmt.Sprintf("name %s", args.Name)}
		}
		if err := hasAccess(); err != nil {
			// site admin can access org ID
			if auth.CheckCurrentUserIsSiteAdmin(ctx, r.db) == nil {
				if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

					// Log action for site admin vieweing an organization's details in dotcom
					if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameDotComOrgViewed, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", args); err != nil {
						r.logger.Warn("Error logging security event", log.Error(err))

					}
				}
				onlyOrgID := &types.Org{ID: org.ID}
				return &OrgResolver{db: r.db, org: onlyOrgID}, nil
			}
			return nil, err
		}
	}

	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

		// Log action for siteadmin viewing an organization's details
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameOrgViewed, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", args); err != nil {
			r.logger.Warn("Error logging security event", log.Error(err))

		}
	}
	return &OrgResolver{db: r.db, org: org}, nil
}

// Deprecated: Org is only in use by sourcegraph/src. Use Node to look up an
// org by its graphql.ID instead.
func (r *schemaResolver) Org(ctx context.Context, args *struct {
	ID graphql.ID
},
) (*OrgResolver, error) {
	return OrgByID(ctx, r.db, args.ID)
}

func OrgByID(ctx context.Context, db database.DB, id graphql.ID) (*OrgResolver, error) {
	orgID, err := UnmarshalOrgID(id)
	if err != nil {
		return nil, err
	}
	return OrgByIDInt32(ctx, db, orgID)
}

func OrgByIDInt32(ctx context.Context, db database.DB, orgID int32) (*OrgResolver, error) {
	return orgByIDInt32WithForcedAccess(ctx, db, orgID, false)
}

func orgByIDInt32WithForcedAccess(ctx context.Context, db database.DB, orgID int32, forceAccess bool) (*OrgResolver, error) {
	// 🚨 SECURITY: Only org members can get org details on Cloud
	//              And all invited users by email
	if !forceAccess && dotcom.SourcegraphDotComMode() {
		err := auth.CheckOrgAccess(ctx, db, orgID)
		if err != nil {
			hasAccess := false
			// allow invited user to view org details
			if a := sgactor.FromContext(ctx); a.IsAuthenticated() {
				_, err := db.OrgInvitations().GetPending(ctx, orgID, a.UID)
				if err == nil {
					hasAccess = true
				}
			}
			if !hasAccess {
				return nil, &database.OrgNotFoundError{Message: fmt.Sprintf("id %d", orgID)}
			}
		}
	}
	org, err := db.Orgs().GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return &OrgResolver{db, org}, nil
}

type OrgResolver struct {
	db  database.DB
	org *types.Org
}

func (o *OrgResolver) ID() graphql.ID { return MarshalOrgID(o.org.ID) }

func MarshalOrgID(id int32) graphql.ID { return relay.MarshalID("Org", id) }

func UnmarshalOrgID(id graphql.ID) (orgID int32, err error) {
	if kind := relay.UnmarshalKind(id); kind != "Org" {
		return 0, errors.Newf("invalid org id of kind %q", kind)
	}
	err = relay.UnmarshalSpec(id, &orgID)
	return
}

func (o *OrgResolver) OrgID() int32 {
	return o.org.ID
}

func (o *OrgResolver) Name() string {
	return o.org.Name
}

func (o *OrgResolver) DisplayName() *string {
	return o.org.DisplayName
}

func (o *OrgResolver) URL() string { return "/organizations/" + o.org.Name }

func (o *OrgResolver) SettingsURL() *string { return strptr(o.URL() + "/settings") }

func (o *OrgResolver) CreatedAt() gqlutil.DateTime { return gqlutil.DateTime{Time: o.org.CreatedAt} }

func (o *OrgResolver) Members(ctx context.Context, args struct {
	graphqlutil.ConnectionResolverArgs
	Query *string
},
) (*graphqlutil.ConnectionResolver[*UserResolver], error) {
	// 🚨 SECURITY: Verify listing users is allowed.
	if err := checkMembersAccess(ctx, o.db); err != nil {
		return nil, err
	}

	connectionStore := &membersConnectionStore{
		db:    o.db,
		orgID: o.org.ID,
		query: args.Query,
	}

	return graphqlutil.NewConnectionResolver[*UserResolver](connectionStore, &args.ConnectionResolverArgs, &graphqlutil.ConnectionResolverOptions{
		AllowNoLimit: true,
	})
}

type membersConnectionStore struct {
	db    database.DB
	orgID int32
	query *string
}

func (s *membersConnectionStore) ComputeTotal(ctx context.Context) (int32, error) {
	query := ""
	if s.query != nil {
		query = *s.query
	}

	count, err := s.db.Users().Count(ctx, &database.UsersListOptions{OrgID: s.orgID, Query: query})
	if err != nil {
		return 0, err
	}

	return int32(count), nil
}

func (s *membersConnectionStore) ComputeNodes(ctx context.Context, args *database.PaginationArgs) ([]*UserResolver, error) {
	users, err := s.db.Users().ListByOrg(ctx, s.orgID, args, s.query)
	if err != nil {
		return nil, err
	}

	var userResolvers []*UserResolver
	for _, user := range users {
		userResolvers = append(userResolvers, NewUserResolver(ctx, s.db, user))
	}

	return userResolvers, nil
}

func (s *membersConnectionStore) MarshalCursor(node *UserResolver, _ database.OrderBy) (*string, error) {
	if node == nil {
		return nil, errors.New(`node is nil`)
	}

	cursor := string(node.ID())

	return &cursor, nil
}

func (s *membersConnectionStore) UnmarshalCursor(cursor string, _ database.OrderBy) ([]any, error) {
	nodeID, err := UnmarshalUserID(graphql.ID(cursor))
	if err != nil {
		return nil, err
	}

	return []any{nodeID}, nil
}

func (o *OrgResolver) settingsSubject() api.SettingsSubject {
	return api.SettingsSubject{Org: &o.org.ID}
}

func (o *OrgResolver) LatestSettings(ctx context.Context) (*settingsResolver, error) {
	// 🚨 SECURITY: Only organization members and site admins (not on cloud) may access the settings,
	// because they may contain secrets or other sensitive data.
	if err := auth.CheckOrgAccessOrSiteAdmin(ctx, o.db, o.org.ID); err != nil {
		return nil, err
	}

	settings, err := o.db.Settings().GetLatest(ctx, o.settingsSubject())
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}

	return &settingsResolver{o.db, &settingsSubjectResolver{org: o}, settings, nil}, nil
}

func (o *OrgResolver) SettingsCascade() *settingsCascade {
	return &settingsCascade{db: o.db, subject: &settingsSubjectResolver{org: o}}
}

func (o *OrgResolver) ConfigurationCascade() *settingsCascade { return o.SettingsCascade() }

func (o *OrgResolver) ViewerPendingInvitation(ctx context.Context) (*organizationInvitationResolver, error) {
	if actor := sgactor.FromContext(ctx); actor.IsAuthenticated() {
		orgInvitation, err := o.db.OrgInvitations().GetPending(ctx, o.org.ID, actor.UID)
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			// ignore expired invitations, otherwise error is returned
			// for all users who have an expired invitation on record
			if _, ok := err.(database.OrgInvitationExpiredErr); ok {
				return nil, nil
			}
			return nil, err
		}
		return &organizationInvitationResolver{o.db, orgInvitation}, nil
	}
	return nil, nil
}

func (o *OrgResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	if err := auth.CheckOrgAccessOrSiteAdmin(ctx, o.db, o.org.ID); err == auth.ErrNotAuthenticated || err == auth.ErrNotAnOrgMember {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (o *OrgResolver) ViewerIsMember(ctx context.Context) (bool, error) {
	actor := sgactor.FromContext(ctx)
	if !actor.IsAuthenticated() {
		return false, nil
	}
	if _, err := o.db.OrgMembers().GetByOrgIDAndUserID(ctx, o.org.ID, actor.UID); err != nil {
		if errcode.IsNotFound(err) {
			err = nil
		}
		return false, err
	}
	return true, nil
}

func (o *OrgResolver) NamespaceName() string { return o.org.Name }

func (o *OrgResolver) BatchChanges(ctx context.Context, args *ListBatchChangesArgs) (BatchChangesConnectionResolver, error) {
	id := o.ID()
	args.Namespace = &id
	return EnterpriseResolvers.batchChangesResolver.BatchChanges(ctx, args)
}

func (r *schemaResolver) CreateOrganization(ctx context.Context, args *struct {
	Name        string
	DisplayName *string
},
) (*OrgResolver, error) {
	a := sgactor.FromContext(ctx)
	if !a.IsAuthenticated() {
		return nil, errors.New("no current user")
	}

	if err := suspiciousnames.CheckNameAllowedForUserOrOrganization(args.Name); err != nil {
		return nil, err
	}
	newOrg, err := r.db.Orgs().Create(ctx, args.Name, args.DisplayName)
	if err != nil {
		return nil, err
	}

	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {
		// Log an event when a new organization being created
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameOrgCreated, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", args); err != nil {
			r.logger.Warn("Error logging security event", log.Error(err))

		}
	}

	// Add the current user as the first member of the new org.
	_, err = r.db.OrgMembers().Create(ctx, newOrg.ID, a.UID)
	if err != nil {
		return nil, err
	}

	return &OrgResolver{db: r.db, org: newOrg}, nil
}

func (r *schemaResolver) UpdateOrganization(ctx context.Context, args *struct {
	ID          graphql.ID
	DisplayName *string
},
) (*OrgResolver, error) {
	var orgID int32
	if err := relay.UnmarshalSpec(args.ID, &orgID); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the current user is a member
	// of the org that is being modified.
	if err := auth.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgID); err != nil {
		return nil, err
	}

	updatedOrg, err := r.db.Orgs().Update(ctx, orgID, args.DisplayName)
	if err != nil {
		return nil, err
	}

	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {
		// Log an event when organization settings are updated
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameOrgUpdated, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", args); err != nil {
			r.logger.Warn("Error logging security event", log.Error(err))

		}
	}
	return &OrgResolver{db: r.db, org: updatedOrg}, nil
}

func (r *schemaResolver) RemoveUserFromOrganization(ctx context.Context, args *struct {
	User         graphql.ID
	Organization graphql.ID
},
) (*EmptyResponse, error) {
	orgID, err := UnmarshalOrgID(args.Organization)
	if err != nil {
		return nil, err
	}
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Check that the current user is a member of the org that is being modified, or a
	// site admin.
	if err := auth.CheckOrgAccessOrSiteAdmin(ctx, r.db, orgID); err != nil {
		return nil, err
	}
	memberCount, err := r.db.OrgMembers().MemberCount(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if memberCount == 1 && !r.siteAdminSelfRemoving(ctx, userID) {
		return nil, errors.New("you can’t remove the only member of an organization")
	}
	r.logger.Info("removing user from org", log.Int32("userID", userID), log.Int32("orgID", orgID))
	if err := r.db.OrgMembers().Remove(ctx, orgID, userID); err != nil {
		return nil, err
	}

	// Enqueue a sync job. Internally this will log an error if enqueuing failed.
	permssync.SchedulePermsSync(ctx, r.logger, r.db, permssync.ScheduleSyncOpts{UserIDs: []int32{userID}, Reason: database.ReasonUserRemovedFromOrg})

	return nil, nil
}

func (r *schemaResolver) siteAdminSelfRemoving(ctx context.Context, userID int32) bool {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return false
	}
	if err := auth.CheckSameUser(ctx, userID); err != nil {
		return false
	}
	return true
}

func (r *schemaResolver) AddUserToOrganization(ctx context.Context, args *struct {
	Organization graphql.ID
	Username     string
},
) (*EmptyResponse, error) {
	// get the organization ID as an integer first
	var orgID int32
	if err := relay.UnmarshalSpec(args.Organization, &orgID); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Must be a site admin to immediately add a user to an organization (bypassing the
	// invitation step).
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	userToInvite, err := r.db.Users().GetByUsername(ctx, args.Username)
	if err != nil {
		return nil, err
	}
	if _, err := r.db.OrgMembers().Create(ctx, orgID, userToInvite.ID); err != nil {
		return nil, err
	}

	// Schedule permission sync for newly added user. Internally it will log an error if enqueuing failed.
	permssync.SchedulePermsSync(ctx, r.logger, r.db, permssync.ScheduleSyncOpts{UserIDs: []int32{userToInvite.ID}, Reason: database.ReasonUserAddedToOrg})

	return &EmptyResponse{}, nil
}
