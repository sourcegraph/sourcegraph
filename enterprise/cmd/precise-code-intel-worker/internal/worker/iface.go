package worker

import (
	"context"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/gitserver"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
)

type DBStore interface {
	basestore.ShareableStore
	gitserver.DBStore

	With(other basestore.ShareableStore) DBStore
	Transact(ctx context.Context) (DBStore, error)
	Done(err error) error

	UpdatePackages(ctx context.Context, dumpID int, packages []semantic.Package) error
	UpdatePackageReferences(ctx context.Context, dumpID int, packageReferences []semantic.PackageReference) error
	MarkRepositoryAsDirty(ctx context.Context, repositoryID int) error
	DeleteOverlappingDumps(ctx context.Context, repositoryID int, commit, root, indexer string) error
}

type DBStoreShim struct {
	*dbstore.Store
}

func (s *DBStoreShim) With(other basestore.ShareableStore) DBStore {
	return &DBStoreShim{s.Store.With(other)}
}

func (s *DBStoreShim) Transact(ctx context.Context) (DBStore, error) {
	tx, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &DBStoreShim{tx}, nil
}

type LSIFStore interface {
	Transact(ctx context.Context) (LSIFStore, error)
	Done(err error) error

	WriteMeta(ctx context.Context, bundleID int, meta semantic.MetaData) error
	WriteDocuments(ctx context.Context, bundleID int, documents chan semantic.KeyedDocumentData) error
	WriteResultChunks(ctx context.Context, bundleID int, resultChunks chan semantic.IndexedResultChunkData) error
	WriteDefinitions(ctx context.Context, bundleID int, monikerLocations chan semantic.MonikerLocations) error
	WriteReferences(ctx context.Context, bundleID int, monikerLocations chan semantic.MonikerLocations) error
}

type LSIFStoreShim struct {
	*lsifstore.Store
}

func (s *LSIFStoreShim) Transact(ctx context.Context) (LSIFStore, error) {
	tx, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &LSIFStoreShim{tx}, nil
}

type GitserverClient interface {
	DirectoryChildren(ctx context.Context, repositoryID int, commit string, dirnames []string) (map[string][]string, error)
}
