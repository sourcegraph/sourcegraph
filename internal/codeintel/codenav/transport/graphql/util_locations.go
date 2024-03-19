package graphql

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sourcegraph/go-lsp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
)

func newLocationConnectionResolver(locations []shared.UploadLocation, cursor *string, locationResolver *gitresolvers.CachedLocationResolver) resolverstubs.LocationConnectionResolver {
	return resolverstubs.NewLazyConnectionResolver(func(ctx context.Context) ([]resolverstubs.LocationResolver, error) {
		return resolveLocations(ctx, locationResolver, locations)
	}, encodeCursor(cursor))
}

// resolveLocations creates a slide of LocationResolvers for the given list of adjusted locations. The
// resulting list may be smaller than the input list as any locations with a commit not known by
// gitserver will be skipped.
func resolveLocations(ctx context.Context, locationResolver *gitresolvers.CachedLocationResolver, locations []shared.UploadLocation) ([]resolverstubs.LocationResolver, error) {
	resolvedLocations := make([]resolverstubs.LocationResolver, 0, len(locations))
	for i := range locations {
		resolver, err := resolveLocation(ctx, locationResolver, locations[i])
		if err != nil {
			return nil, err
		}
		if resolver == nil {
			continue
		}

		resolvedLocations = append(resolvedLocations, resolver)
	}

	return resolvedLocations, nil
}

// resolveLocation creates a LocationResolver for the given adjusted location. This function may return a
// nil resolver if the location's commit is not known by gitserver.
func resolveLocation(ctx context.Context, locationResolver *gitresolvers.CachedLocationResolver, location shared.UploadLocation) (resolverstubs.LocationResolver, error) {
	treeResolver, err := locationResolver.Path(ctx, api.RepoID(location.Upload.RepositoryID), location.TargetCommit, location.Path, false)
	if err != nil || treeResolver == nil {
		return nil, err
	}

	lspRange := convertRange(location.TargetRange)
	return newLocationResolver(treeResolver, &lspRange), nil
}

//
//

type locationResolver struct {
	resource resolverstubs.GitTreeEntryResolver
	lspRange *lsp.Range
}

func newLocationResolver(resource resolverstubs.GitTreeEntryResolver, lspRange *lsp.Range) resolverstubs.LocationResolver {
	return &locationResolver{
		resource: resource,
		lspRange: lspRange,
	}
}

func (r *locationResolver) Resource() resolverstubs.GitTreeEntryResolver { return r.resource }

func (r *locationResolver) Range() resolverstubs.RangeResolver {
	return r.rangeInternal()
}

func (r *locationResolver) rangeInternal() *rangeResolver {
	if r.lspRange == nil {
		return nil
	}
	return &rangeResolver{*r.lspRange}
}

func (r *locationResolver) URL(ctx context.Context) (string, error) {
	return r.urlPath(r.resource.URL()), nil
}

func (r *locationResolver) CanonicalURL() string {
	return r.urlPath(r.resource.URL())
}

func (r *locationResolver) urlPath(prefix string) string {
	url := prefix
	if r.lspRange != nil {
		url += "?L" + r.rangeInternal().urlFragment()
	}
	return url
}

//
//

type rangeResolver struct{ lspRange lsp.Range }

func newRangeResolver(lspRange lsp.Range) resolverstubs.RangeResolver {
	return &rangeResolver{
		lspRange: lspRange,
	}
}

func (r *rangeResolver) Start() resolverstubs.PositionResolver { return r.start() }
func (r *rangeResolver) End() resolverstubs.PositionResolver   { return r.end() }

func (r *rangeResolver) start() *positionResolver { return &positionResolver{r.lspRange.Start} }
func (r *rangeResolver) end() *positionResolver   { return &positionResolver{r.lspRange.End} }

func (r *rangeResolver) urlFragment() string {
	if r.lspRange.Start == r.lspRange.End {
		return r.start().urlFragment(false)
	}
	hasCharacter := r.lspRange.Start.Character != 0 || r.lspRange.End.Character != 0
	return r.start().urlFragment(hasCharacter) + "-" + r.end().urlFragment(hasCharacter)
}

//
//

type positionResolver struct{ pos lsp.Position }

// func newPositionResolver(pos lsp.Position) resolverstubs.PositionResolver {
// 	return &positionResolver{pos: pos}
// }

func (r *positionResolver) Line() int32      { return int32(r.pos.Line) }
func (r *positionResolver) Character() int32 { return int32(r.pos.Character) }

func (r *positionResolver) urlFragment(forceIncludeCharacter bool) string {
	if !forceIncludeCharacter && r.pos.Character == 0 {
		return strconv.Itoa(r.pos.Line + 1)
	}
	return fmt.Sprintf("%d:%d", r.pos.Line+1, r.pos.Character+1)
}

//
//

// convertRange creates an LSP range from a bundle range.
func convertRange(r shared.Range) lsp.Range {
	return lsp.Range{Start: convertPosition(r.Start.Line, r.Start.Character), End: convertPosition(r.End.Line, r.End.Character)}
}

func convertPosition(line, character int) lsp.Position {
	return lsp.Position{Line: line, Character: character}
}
