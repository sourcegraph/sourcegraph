package resolvers

import (
	"context"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/store"
	otypes "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/types"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
)

func (r *Resolver) oAuthClientApplicationByID(ctx context.Context, id graphql.ID) (graphqlbackend.Node, error) {
	// 🚨 SECURITY: Only site admins can access OAuth applications.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	dbID, err := UnmarshalOAuthClientApplicationID(id)
	if err != nil {
		return nil, err
	}

	app, err := r.store.GetByID(ctx, dbID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &oAuthClientApplicationResolver{db: r.db, store: r.store, app: app}, nil
}

type oAuthClientApplicationResolver struct {
	db    database.DB
	store *store.Store
	app   *otypes.OAuthClientApplication
}

func (r *oAuthClientApplicationResolver) ID() graphql.ID {
	return MarshalOAuthClientApplicationID(r.app.ID)
}

func (r *oAuthClientApplicationResolver) Name() string {
	return r.app.Name
}

func (r *oAuthClientApplicationResolver) Description() string {
	return r.app.Description
}

func (r *oAuthClientApplicationResolver) RedirectURL() string {
	return r.app.RedirectURL
}

func (r *oAuthClientApplicationResolver) ClientID() string {
	return r.app.ClientID
}

func (r *oAuthClientApplicationResolver) ClientSecret() string {
	return r.app.ClientSecret
}

func (r *oAuthClientApplicationResolver) Creator(ctx context.Context) (*graphqlbackend.UserResolver, error) {
	u, err := graphqlbackend.UserByIDInt32(ctx, r.db, r.app.Creator)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *oAuthClientApplicationResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.app.CreatedAt}
}

func (r *oAuthClientApplicationResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.app.UpdatedAt}
}
