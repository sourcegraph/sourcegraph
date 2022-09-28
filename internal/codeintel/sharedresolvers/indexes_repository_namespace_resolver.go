package sharedresolvers

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
)

type LSIFIndexesWithRepositoryNamespaceResolver interface {
	Root() string
	Indexer() types.CodeIntelIndexerResolver
	Indexes() []LSIFIndexResolver
}

type lsifIndexesWithRepositoryNamespaceResolver struct {
	indexesSummary dbstore.IndexesWithRepositoryNamespace
	indexResolvers []LSIFIndexResolver
}

func NewLSIFIndexesWithRepositoryNamespaceResolver(indexesSummary dbstore.IndexesWithRepositoryNamespace, indexResolvers []LSIFIndexResolver) LSIFIndexesWithRepositoryNamespaceResolver {
	return &lsifIndexesWithRepositoryNamespaceResolver{
		indexesSummary: indexesSummary,
		indexResolvers: indexResolvers,
	}
}

func (r *lsifIndexesWithRepositoryNamespaceResolver) Root() string {
	return r.indexesSummary.Root
}

func (r *lsifIndexesWithRepositoryNamespaceResolver) Indexer() types.CodeIntelIndexerResolver {
	// drop the tag if it exists
	if idx, ok := types.ImageToIndexer[strings.Split(r.indexesSummary.Indexer, ":")[0]]; ok {
		return idx
	}

	return types.NewCodeIntelIndexerResolver(r.indexesSummary.Indexer)
}

func (r *lsifIndexesWithRepositoryNamespaceResolver) Indexes() []LSIFIndexResolver {
	return r.indexResolvers
}
