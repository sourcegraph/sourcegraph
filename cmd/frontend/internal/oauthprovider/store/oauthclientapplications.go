package store

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"

	otypes "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/types"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
)

type Store struct {
	*basestore.Store

	key encryption.Key
}

func (s *Store) Create(ctx context.Context, app *otypes.OAuthClientApplication) error {
	r := s.QueryRow(ctx, sqlf.Sprintf(createOAuthAppQueryFmtstr,
		app.Name,
		app.Description,
		app.ClientID,
		app.ClientSecret,
		app.RedirectURL,
		dbutil.NewNullInt32(app.Creator),
		app.CreatedAt,
		app.UpdatedAt,
	))
	return r.Scan(
		&app.ID,
		&app.Name,
		&app.Description,
		&app.ClientID,
		&app.ClientSecret,
		&app.RedirectURL,
		&dbutil.NullInt32{N: &app.Creator},
		&app.CreatedAt,
		&app.UpdatedAt,
	)
}

const createOAuthAppQueryFmtstr = `
INSERT INTO oauth_client_applications (
	client_id,
	client_secret,
	redirect_uris,
	description,
	created_at,
	updated_at,
	deleted_at
) VALUES (
	%s,
	%s,
	%s,
	%s,
	%s,
	%s,
	%s
)
RETURNING
	id,
	name,
	description,
	client_id,
	client_secret,
	redirect_url,
	creator_id,
	created_at,
	updated_at
`

func (s *Store) GetByID(ctx context.Context, id int64) (*otypes.OAuthClientApplication, error) {
	app := new(otypes.OAuthClientApplication)
	err := s.QueryRow(ctx, sqlf.Sprintf(getOAuthAppByIDQueryFmtstr, id)).Scan(
		&app.ID,
		&app.Name,
		&app.Description,
		&app.ClientID,
		&app.ClientSecret,
		&app.RedirectURL,
		&dbutil.NullInt32{N: &app.Creator},
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &NotFoundError{}
		}
		return nil, err
	}
	return app, nil
}

const getOAuthAppByIDQueryFmtstr = `
SELECT
	id,
	name,
	description,
	client_id,
	client_secret,
	redirect_url,
	creator_id,
	created_at,
	updated_at
FROM oauth_client_applications
WHERE
	id = %s
`
