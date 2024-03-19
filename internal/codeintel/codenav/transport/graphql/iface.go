package graphql

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
)

type CodeNavService interface {
	GetHover(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState) (_ string, _ shared.Range, _ bool, err error)
	GetReferences(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState, cursor codenav.Cursor) (_ []shared.UploadLocation, nextCursor codenav.Cursor, err error)
	GetImplementations(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState, cursor codenav.Cursor) (_ []shared.UploadLocation, nextCursor codenav.Cursor, err error)
	GetPrototypes(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState, cursor codenav.Cursor) (_ []shared.UploadLocation, nextCursor codenav.Cursor, err error)
	GetDefinitions(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState) (_ []shared.UploadLocation, err error)
	GetDiagnostics(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState) (diagnosticsAtUploads []codenav.DiagnosticAtUpload, _ int, err error)
	GetRanges(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState, startLine, endLine int) (adjustedRanges []codenav.AdjustedCodeIntelligenceRange, err error)
	GetStencil(ctx context.Context, args codenav.PositionalRequestArgs, requestState codenav.RequestState) (adjustedRanges []shared.Range, err error)
	GetClosestCompletedUploadsForBlob(ctx context.Context, repositoryID int, commit, path string, exactPath bool, indexer string) (_ []uploadsshared.CompletedUpload, err error)
	VisibleUploadsForPath(ctx context.Context, requestState codenav.RequestState) ([]uploadsshared.CompletedUpload, error)
	SnapshotForDocument(ctx context.Context, repositoryID int, commit, path string, uploadID int) (data []shared.SnapshotData, err error)
}

type AutoIndexingService interface {
	QueueRepoRev(ctx context.Context, repositoryID int, rev string) error
}
