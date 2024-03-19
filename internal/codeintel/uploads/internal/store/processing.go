package store

import (
	"context"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
)

// InsertUpload inserts a new upload and returns its identifier.
func (s *store) InsertUpload(ctx context.Context, upload shared.Upload) (id int, err error) {
	ctx, _, endObservation := s.operations.insertUpload.With(ctx, &err, observation.Args{})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("id", id),
		}})
	}()

	if upload.UploadedParts == nil {
		upload.UploadedParts = []int{}
	}

	id, _, err = basestore.ScanFirstInt(s.db.Query(
		ctx,
		sqlf.Sprintf(
			insertUploadQuery,
			upload.Commit,
			upload.Root,
			upload.RepositoryID,
			upload.Indexer,
			upload.IndexerVersion,
			upload.State,
			upload.NumParts,
			pq.Array(upload.UploadedParts),
			upload.UploadSize,
			upload.AssociatedIndexID,
			upload.ContentType,
			upload.UncompressedSize,
		),
	))

	return id, err
}

const insertUploadQuery = `
INSERT INTO lsif_uploads (
	commit,
	root,
	repository_id,
	indexer,
	indexer_version,
	state,
	num_parts,
	uploaded_parts,
	upload_size,
	associated_index_id,
	content_type,
	uncompressed_size
) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING id
`

// AddUploadPart adds the part index to the given upload's uploaded parts array. This method is idempotent
// (the resulting array is deduplicated on update).
func (s *store) AddUploadPart(ctx context.Context, uploadID, partIndex int) (err error) {
	ctx, _, endObservation := s.operations.addUploadPart.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("uploadID", uploadID),
		attribute.Int("partIndex", partIndex),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.Exec(ctx, sqlf.Sprintf(addUploadPartQuery, partIndex, uploadID))
}

const addUploadPartQuery = `
UPDATE lsif_uploads SET uploaded_parts = array(SELECT DISTINCT * FROM unnest(array_append(uploaded_parts, %s))) WHERE id = %s
`

// MarkQueued updates the state of the upload to queued and updates the upload size.
func (s *store) MarkQueued(ctx context.Context, id int, uploadSize *int64) (err error) {
	ctx, _, endObservation := s.operations.markQueued.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.Exec(ctx, sqlf.Sprintf(markQueuedQuery, dbutil.NullInt64{N: uploadSize}, id))
}

const markQueuedQuery = `
UPDATE lsif_uploads
SET
	state = 'queued',
	queued_at = clock_timestamp(),
	upload_size = %s
WHERE id = %s
`

// MarkFailed updates the state of the upload to failed, increments the num_failures column and sets the finished_at time
func (s *store) MarkFailed(ctx context.Context, id int, reason string) (err error) {
	ctx, _, endObservation := s.operations.markFailed.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.Exec(ctx, sqlf.Sprintf(markFailedQuery, reason, id))
}

const markFailedQuery = `
UPDATE
	lsif_uploads
SET
	state = 'failed',
	finished_at = clock_timestamp(),
	failure_message = %s,
	num_failures = num_failures + 1
WHERE
	id = %s
`

// DeleteOverlapapingCompletedUploads deletes all completed uploads for the given repository with the same
// commit, root, and indexer. This is necessary to perform during conversions before changing
// the state of a processing upload to completed as there is a unique index on these four columns.
func (s *store) DeleteOverlappingCompletedUploads(ctx context.Context, repositoryID int, commit, root, indexer string) (err error) {
	ctx, trace, endObservation := s.operations.deleteOverlappingCompletedUploads.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
		attribute.String("commit", commit),
		attribute.String("root", root),
		attribute.String("indexer", indexer),
	}})
	defer endObservation(1, observation.Args{})

	unset, _ := s.db.SetLocal(ctx, "codeintel.lsif_uploads_audit.reason", "upload overlapping with a newer upload")
	defer unset(ctx)
	count, _, err := basestore.ScanFirstInt(s.db.Query(ctx, sqlf.Sprintf(deleteOverlappingCompletedUploadsQuery, repositoryID, commit, root, indexer)))
	if err != nil {
		return err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("count", count))

	return nil
}

const deleteOverlappingCompletedUploadsQuery = `
WITH
candidates AS (
	SELECT u.id
	FROM lsif_uploads u
	WHERE
		u.state = 'completed' AND
		u.repository_id = %s AND
		u.commit = %s AND
		u.root = %s AND
		u.indexer = %s

	-- Lock these rows in a deterministic order so that we don't
	-- deadlock with other processes updating the lsif_uploads table.
	ORDER BY u.id FOR UPDATE
),
updated AS (
	UPDATE lsif_uploads
	SET state = 'deleting'
	WHERE id IN (SELECT id FROM candidates)
	RETURNING 1
)
SELECT COUNT(*) FROM updated
`

func (s *store) WorkerutilStore(observationCtx *observation.Context) dbworkerstore.Store[shared.Upload] {
	return dbworkerstore.New(observationCtx, s.db.Handle(), UploadWorkerStoreOptions)
}

//
//

// stalledUploadMaxAge is the maximum allowable duration between updating the state of an
// upload as "processing" and locking the upload row during processing. An unlocked row that
// is marked as processing likely indicates that the worker that dequeued the upload has died.
// There should be a nearly-zero delay between these states during normal operation.
const stalledUploadMaxAge = time.Second * 25

// uploadMaxNumResets is the maximum number of times an upload can be reset. If an upload's
// failed attempts counter reaches this threshold, it will be moved into "errored" rather than
// "queued" on its next reset.
const uploadMaxNumResets = 3

var uploadColumnsWithNullRank = []*sqlf.Query{
	sqlf.Sprintf("u.id"),
	sqlf.Sprintf("u.commit"),
	sqlf.Sprintf("u.root"),
	sqlf.Sprintf("EXISTS (" + visibleAtTipSubselectQuery + ") AS visible_at_tip"),
	sqlf.Sprintf("u.uploaded_at"),
	sqlf.Sprintf("u.state"),
	sqlf.Sprintf("u.failure_message"),
	sqlf.Sprintf("u.started_at"),
	sqlf.Sprintf("u.finished_at"),
	sqlf.Sprintf("u.process_after"),
	sqlf.Sprintf("u.num_resets"),
	sqlf.Sprintf("u.num_failures"),
	sqlf.Sprintf("u.repository_id"),
	sqlf.Sprintf("u.repository_name"),
	sqlf.Sprintf("u.indexer"),
	sqlf.Sprintf("u.indexer_version"),
	sqlf.Sprintf("u.num_parts"),
	sqlf.Sprintf("u.uploaded_parts"),
	sqlf.Sprintf("u.upload_size"),
	sqlf.Sprintf("u.associated_index_id"),
	sqlf.Sprintf("u.content_type"),
	sqlf.Sprintf("u.should_reindex"),
	sqlf.Sprintf("NULL"),
	sqlf.Sprintf("u.uncompressed_size"),
}

var UploadWorkerStoreOptions = dbworkerstore.Options[shared.Upload]{
	Name:              "codeintel_upload",
	TableName:         "lsif_uploads",
	ViewName:          "lsif_uploads_with_repository_name u",
	ColumnExpressions: uploadColumnsWithNullRank,
	Scan:              dbworkerstore.BuildWorkerScan(scanCompleteUpload),
	OrderByExpression: sqlf.Sprintf(`
		u.associated_index_id IS NULL DESC,
		COALESCE(u.process_after, u.uploaded_at),
		u.id
	`),
	StalledMaxAge: stalledUploadMaxAge,
	MaxNumResets:  uploadMaxNumResets,
}
