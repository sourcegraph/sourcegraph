package loader

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type contextKey string

const key = contextKey("dataloaders")

type Loader interface{}

// Loaders holds references to the individual dataloaders.
type loader struct {
	// individual loaders will be defined here
}

func newLoaders(ctx context.Context, db database.DB) Loader {
	// l := dataloader.NewBatchedLoader[string, []types.User](func(keys []string) ([]types.User, []error) {})
	return &loader{
		// individual loaders will be initialized here
	}
}
