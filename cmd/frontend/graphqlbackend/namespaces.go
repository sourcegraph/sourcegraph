package graphqlbackend

import (
	"context"
	"fmt"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Namespace is the interface for the GraphQL Namespace interface.
type Namespace interface {
	ID() graphql.ID
	URL() string
	NamespaceName() string
}

func (r *schemaResolver) Namespace(ctx context.Context, args *struct{ ID graphql.ID }) (*NamespaceResolver, error) {
	n, err := NamespaceByID(ctx, r.db, args.ID)
	if err != nil {
		return nil, err
	}
	return &NamespaceResolver{n}, nil
}

type InvalidNamespaceIDErr struct {
	id graphql.ID
}

func (e InvalidNamespaceIDErr) Error() string {
	return fmt.Sprintf("invalid ID %q for namespace", e.id)
}

// NamespaceByID looks up a GraphQL value of type Namespace by ID.
func NamespaceByID(ctx context.Context, db database.DB, id graphql.ID) (Namespace, error) {
	switch relay.UnmarshalKind(id) {
	case "User":
		return UserByID(ctx, db, id)
	case "Org":
		return OrgByID(ctx, db, id)
	default:
		return nil, InvalidNamespaceIDErr{id: id}
	}
}

// UnmarshalNamespaceID is a helper function that unmarshals a namespace ID into a user or org ID.
//
// DEPRECATED: Prefer UnmarshalNamespaceToIDs.
func UnmarshalNamespaceID(id graphql.ID, userID *int32, orgID *int32) (err error) {
	switch relay.UnmarshalKind(id) {
	case "User":
		err = relay.UnmarshalSpec(id, userID)
	case "Org":
		err = relay.UnmarshalSpec(id, orgID)
	default:
		err = InvalidNamespaceIDErr{id: id}
	}
	return err
}

// UnmarshalNamespaceToIDs is similar to UnmarshalNamespaceID, except instead of unmarshalling into
// existing variables, it creates its own for convenience.
//
// If the err is nil, the returned Namespace will have exactly 1 non-nil field.
func UnmarshalNamespaceToIDs(id graphql.ID) (*types.Namespace, error) {
	var namespace types.Namespace
	var err error
	switch relay.UnmarshalKind(id) {
	case "User":
		err = relay.UnmarshalSpec(id, &namespace.User)
	case "Org":
		err = relay.UnmarshalSpec(id, &namespace.Org)
	}
	if err != nil {
		return nil, err
	}
	if namespace.User == nil && namespace.Org == nil {
		return nil, InvalidNamespaceIDErr{id: id}
	}
	return &namespace, nil
}

func CheckAuthorizedForNamespace(ctx context.Context, db database.DB, namespaceID graphql.ID) (*types.Namespace, error) {
	namespace, err := UnmarshalNamespaceToIDs(namespaceID)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Make sure the current user has permission for the specified namespace.
	if err := CheckAuthorizedForNamespaceByIDs(ctx, db, *namespace); err != nil {
		return nil, err
	}
	return namespace, nil
}

func CheckAuthorizedForNamespaceByIDs(ctx context.Context, db database.DB, namespace types.Namespace) error {
	// 🚨 SECURITY: Make sure the current user has permission for the specified namespace.
	if namespace.User != nil {
		return auth.CheckSiteAdminOrSameUser(ctx, db, *namespace.User)
	}
	if namespace.Org != nil {
		return auth.CheckOrgAccessOrSiteAdmin(ctx, db, *namespace.Org)
	}
	return errors.New("namespace is required (User or Organization ID)")
}

func (r *schemaResolver) NamespaceByName(ctx context.Context, args *struct{ Name string }) (*NamespaceResolver, error) {
	namespace, err := r.db.Namespaces().GetByName(ctx, args.Name)
	if err == database.ErrNamespaceNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var n Namespace
	switch {
	case namespace.User != 0:
		n, err = UserByIDInt32(ctx, r.db, namespace.User)
	case namespace.Organization != 0:
		n, err = OrgByIDInt32(ctx, r.db, namespace.Organization)
	default:
		panic("invalid namespace (neither user nor organization)")
	}
	if err != nil {
		return nil, err
	}
	return &NamespaceResolver{n}, nil
}

// NamespaceResolver resolves the GraphQL Namespace interface to a type.
type NamespaceResolver struct {
	Namespace
}

func (r NamespaceResolver) ToOrg() (*OrgResolver, bool) {
	n, ok := r.Namespace.(*OrgResolver)
	return n, ok
}

func (r NamespaceResolver) ToUser() (*UserResolver, bool) {
	n, ok := r.Namespace.(*UserResolver)
	return n, ok
}
