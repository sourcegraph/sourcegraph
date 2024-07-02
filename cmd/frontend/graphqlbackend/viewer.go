package graphqlbackend

import (
	"context"
)

func (r *schemaResolver) Viewer(ctx context.Context) (*viewerResolver, error) {
	user, err := CurrentUser(ctx, r.db)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return &viewerResolver{user}, nil
	}
	return &viewerResolver{&visitorResolver{}}, nil
}

// viewer is the interface for the GraphQL viewer interface.
type viewer interface {
	AffiliatedNamespaces(ctx context.Context) ([]*NamespaceResolver, error)
}

// viewerResolver resolves the GraphQL Viewer interface to a type.
type viewerResolver struct {
	viewer
}

func (v viewerResolver) ToUser() (*UserResolver, bool) {
	n, ok := v.viewer.(*UserResolver)
	return n, ok
}

func (v viewerResolver) ToVisitor() (*visitorResolver, bool) {
	n, ok := v.viewer.(*visitorResolver)
	return n, ok
}
