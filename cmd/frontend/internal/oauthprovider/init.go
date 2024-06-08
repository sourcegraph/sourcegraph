package oauthprovider

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/resolvers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/oauthprovider/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// Init initializes the given enterpriseServices to include the required
// resolvers for the OAuthProviders module.
func Init(
	ctx context.Context,
	observationCtx *observation.Context,
	db database.DB,
	_ codeintel.Services,
	_ conftypes.UnifiedWatchable,
	enterpriseServices *enterprise.Services,
) error {
	logger := observationCtx.Logger.Scoped("OAuthProvider")
	store := store.New(db, observationCtx, keyring.Default().OAuthProviderKey)

	enterpriseServices.OAuthProviderResolver = resolvers.New(db, store, logger)

	return nil
}
