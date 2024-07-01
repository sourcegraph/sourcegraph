package graphqlbackend

import (
	"context"
)

func (r *schemaResolver) Viewer(ctx context.Context) (ViewerResolver, error) {
	user, err := CurrentUser(ctx, r.db)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return visitorResolver{}, nil
}

type ViewerResolver interface {
	AffiliatedNamespaces(ctx context.Context) ([]*NamespaceResolver, error)
}
