package backend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	symbolsclient "github.com/sourcegraph/sourcegraph/internal/symbols"
)

// Symbols backend.
var Symbols = &symbols{}

type symbols struct{}

// ListTags returns symbols in a repository from ctags.
func (symbols) ListTags(ctx context.Context, args search.SymbolsParameters) (result.Symbols, error) {
	result, err := symbolsclient.DefaultClient.Search(ctx, args)
	if result == nil {
		return nil, err
	}
	return *result, err
}
