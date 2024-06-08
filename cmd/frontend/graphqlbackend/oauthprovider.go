package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
)

type ListOAuthClientApplicationsArgs struct {
	First int32
	After *string
}

type CreateOAuthClientApplicationArgs struct {
	Name        string
	Description string
	RedirectURL string
}

type UpdateOAuthClientApplicationArgs struct {
	ID          graphql.ID
	Name        *string
	Description *string
	RedirectURL *string
}

type DeleteOAuthClientApplicationArgs struct {
	ID graphql.ID
}

type OAuthProviderResolver interface {
	// Mutations
	CreateOAuthClientApplication(ctx context.Context, args *CreateOAuthClientApplicationArgs) (OAuthClientApplicationResolver, error)
	UpdateOAuthClientApplication(ctx context.Context, args *UpdateOAuthClientApplicationArgs) (OAuthClientApplicationResolver, error)
	DeleteOAuthClientApplication(ctx context.Context, args *DeleteOAuthClientApplicationArgs) (*EmptyResponse, error)

	// Queries
	OAuthClientApplications(ctx context.Context, args *ListOAuthClientApplicationsArgs) (OAuthClientApplicationConnectionResolver, error)

	NodeResolvers() map[string]NodeByIDFunc
}

type OAuthClientApplicationConnectionResolver interface {
	Nodes(ctx context.Context) ([]OAuthClientApplicationResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

type OAuthClientApplicationResolver interface {
	ID() graphql.ID
	Name() string
	Description() string
	RedirectURL() string
	ClientID() string
	ClientSecret() string
}
