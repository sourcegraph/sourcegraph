package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/auth"
)

func (r *UserResolver) AffiliatedNamespaces(ctx context.Context) ([]*NamespaceResolver, error) {
	// 🚨 SECURITY: Only the user and admins are allowed to access the user's
	// organization memberships.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}

	// Start with the user's own account (which is a namespace they are always affiliated with).
	namespaces := []*NamespaceResolver{
		{Namespace: r},
	}

	// Add all organizations the user is a member of.
	orgs, err := r.db.Orgs().GetByUserID(ctx, r.user.ID)
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		namespaces = append(namespaces, &NamespaceResolver{Namespace: &OrgResolver{db: r.db, org: org}})
	}

	return namespaces, nil
}

func (visitorResolver) AffiliatedNamespaces(context.Context) ([]*NamespaceResolver, error) {
	return []*NamespaceResolver{}, nil
}
