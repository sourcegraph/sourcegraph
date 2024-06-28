package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
)

func (s *schemaResolver) SavedSearches(ctx context.Context, args SavedSearchesArgs) (*SavedSearchConnectionResolver, error) {
	return s.SavedSearchesResolver.SavedSearches(ctx, args)
}

type SavedSearchesResolver interface {
	// Query
	SavedSearches(ctx context.Context, args SavedSearchesArgs) (*SavedSearchConnectionResolver, error)
	SavedSearchByID(ctx context.Context, id graphql.ID) (SavedSearchResolver, error)

	// Mutations
	CreateSavedSearch(ctx context.Context, args *CreateSavedSearchArgs) (SavedSearchResolver, error)
	UpdateSavedSearch(ctx context.Context, args *UpdateSavedSearchArgs) (SavedSearchResolver, error)
	DeleteSavedSearch(ctx context.Context, args *DeleteSavedSearchArgs) (*EmptyResponse, error)
	TransferSavedSearchOwnership(ctx context.Context, args *TransferSavedSearchOwnershipArgs) (SavedSearchResolver, error)

	NodeResolvers() map[string]NodeByIDFunc
}

type SavedSearchesOrderBy string

const (
	SavedSearchesOrderByDescription SavedSearchesOrderBy = "SAVED_SEARCH_DESCRIPTION"
	SavedSearchesOrderByUpdatedAt   SavedSearchesOrderBy = "SAVED_SEARCH_UPDATED_AT"
)

type SavedSearchConnectionResolver = graphqlutil.ConnectionResolver[SavedSearchResolver]

type SavedSearchResolver interface {
	ID() graphql.ID
	Description() string
	Query() string
	Owner(context.Context) (*NamespaceResolver, error)
	URL(context.Context) (string, error)
	ViewerCanAdminister(context.Context) (bool, error)
}

type SavedSearchesArgs struct {
	graphqlutil.ConnectionResolverArgs
	Query              *string
	Owner              *graphql.ID
	ViewerIsAffiliated *bool
	OrderBy            SavedSearchesOrderBy
}

type CreateSavedSearchArgs struct {
	Input SavedSearchInput
}

type SavedSearchInput struct {
	Owner       graphql.ID
	Description string
	Query       string
}

type UpdateSavedSearchArgs struct {
	ID    graphql.ID
	Input SavedSearchUpdateInput
}

type SavedSearchUpdateInput struct {
	Description string
	Query       string
}

type DeleteSavedSearchArgs struct {
	ID graphql.ID
}

type TransferSavedSearchOwnershipArgs struct {
	ID       graphql.ID
	NewOwner graphql.ID
}
