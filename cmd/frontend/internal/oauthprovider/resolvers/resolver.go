package resolvers

import (
	"context"
	"errors"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/store"
	otypes "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/types"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

const OAuthClientApplicationKind = "OAuthClientApplication"

func UnmarshalOAuthClientApplicationID(id graphql.ID) (appID int64, err error) {
	err = relay.UnmarshalSpec(id, &appID)
	return
}

func MarshalOAuthClientApplicationID(appID int64) graphql.ID {
	return relay.MarshalID(OAuthClientApplicationKind, appID)
}

func New(db database.DB, store *store.Store, logger log.Logger) graphqlbackend.OAuthProviderResolver {
	return &Resolver{db: db, store: store, logger: logger}
}

type Resolver struct {
	db     database.DB
	store  *store.Store
	logger log.Logger
}

func (r *Resolver) CreateOAuthClientApplication(ctx context.Context, args *graphqlbackend.CreateOAuthClientApplicationArgs) (graphqlbackend.OAuthClientApplicationResolver, error) {
	// 🚨 SECURITY: Only site admins can access OAuth applications.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	if args.Name == "" {
		return nil, errors.New("name is required")
	}

	if args.Description == "" {
		return nil, errors.New("description is required")
	}

	if args.RedirectURL == "" {
		return nil, errors.New("redirectURL is required")
	}

	app := &otypes.OAuthClientApplication{
		Name:        args.Name,
		Description: args.Description,
		RedirectURL: args.RedirectURL,
	}

	if err := r.store.Create(ctx, app); err != nil {
		return nil, err
	}

	return &oAuthClientApplicationResolver{db: r.db, store: r.store, app: app}, nil
}

func (r *Resolver) UpdateOAuthClientApplication(ctx context.Context, args *graphqlbackend.UpdateOAuthClientApplicationArgs) (graphqlbackend.OAuthClientApplicationResolver, error) {
	// 🚨 SECURITY: Only site admins can access OAuth applications.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *Resolver) DeleteOAuthClientApplication(ctx context.Context, args *graphqlbackend.DeleteOAuthClientApplicationArgs) (*graphqlbackend.EmptyResponse, error) {
	// 🚨 SECURITY: Only site admins can access OAuth applications.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *Resolver) OAuthClientApplications(ctx context.Context, args *graphqlbackend.ListOAuthClientApplicationsArgs) (graphqlbackend.OAuthClientApplicationConnectionResolver, error) {
	// 🚨 SECURITY: Only site admins can access OAuth applications.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *Resolver) NodeResolvers() map[string]graphqlbackend.NodeByIDFunc {
	return map[string]graphqlbackend.NodeByIDFunc{
		OAuthClientApplicationKind: r.oAuthClientApplicationByID,
	}
}
