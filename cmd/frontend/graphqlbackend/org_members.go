package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func (r *UserResolver) OrganizationMemberships(ctx context.Context) (*organizationMembershipConnectionResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's
	// organization memberships.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}
	memberships, err := r.db.OrgMembers().GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	c := organizationMembershipConnectionResolver{nodes: make([]*organizationMembershipResolver, len(memberships))}
	for i, member := range memberships {
		c.nodes[i] = &organizationMembershipResolver{r.db, member}
	}
	return &c, nil
}

type organizationMembershipConnectionResolver struct {
	nodes []*organizationMembershipResolver
}

func (r *organizationMembershipConnectionResolver) Nodes() []*organizationMembershipResolver {
	return r.nodes
}
func (r *organizationMembershipConnectionResolver) TotalCount() int32 { return int32(len(r.nodes)) }
func (r *organizationMembershipConnectionResolver) PageInfo() *graphqlutil.PageInfo {
	return graphqlutil.HasNextPage(false)
}

type organizationMembershipResolver struct {
	db         database.DB
	membership *types.OrgMembership
}

func (r *organizationMembershipResolver) Organization(ctx context.Context) (*OrgResolver, error) {
	return OrgByIDInt32(ctx, r.db, r.membership.OrgID)
}

func (r *organizationMembershipResolver) User(ctx context.Context) (*UserResolver, error) {
	return UserByIDInt32(ctx, r.db, r.membership.UserID)
}

func (r *organizationMembershipResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.membership.CreatedAt}
}

func (r *organizationMembershipResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.membership.UpdatedAt}
}
