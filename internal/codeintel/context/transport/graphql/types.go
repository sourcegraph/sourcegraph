package graphql

import (
	"context"

	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
)

type ContextService interface {
	GetPreciseContext(ctx context.Context, args *resolverstubs.GetPreciseContextInput) ([]*types.PreciseContext, string, error)
}
