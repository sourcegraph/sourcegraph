package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GetUploads returns a list of uploads and the total count of records matching the given conditions.
func (s *store) GetUploads(ctx context.Context, opts shared.GetUploadsOptions) (uploads []shared.Upload, totalCount int, err error) {
	ctx, trace, endObservation := s.operations.getUploads.With(ctx, &err, observation.Args{Attrs: buildGetUploadsLogFields(opts)})
	defer endObservation(1, observation.Args{})

	tableExpr, conds, cte := buildGetConditionsAndCte(opts)
	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return nil, 0, err
	}
	conds = append(conds, authzConds)

	var orderExpression *sqlf.Query
	if opts.OldestFirst {
		orderExpression = sqlf.Sprintf("uploaded_at, id DESC")
	} else {
		orderExpression = sqlf.Sprintf("uploaded_at DESC, id")
	}

	var a []shared.Upload
	var b int
	err = s.withTransaction(ctx, func(tx *store) error {
		query := sqlf.Sprintf(
			getUploadsSelectQuery,
			buildCTEPrefix(cte),
			tableExpr,
			sqlf.Join(conds, " AND "),
			orderExpression,
			opts.Limit,
			opts.Offset,
		)
		uploads, err = scanUploadComplete(tx.db.Query(ctx, query))
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner",
			attribute.Int("numUploads", len(uploads)))

		countQuery := sqlf.Sprintf(
			getUploadsCountQuery,
			buildCTEPrefix(cte),
			tableExpr,
			sqlf.Join(conds, " AND "),
		)
		totalCount, _, err = basestore.ScanFirstInt(tx.db.Query(ctx, countQuery))
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner",
			attribute.Int("totalCount", totalCount),
		)

		a = uploads
		b = totalCount
		return nil
	})

	return a, b, err
}

const getUploadsSelectQuery = `
%s -- Dynamic CTE definitions for use in the WHERE clause
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	repo.name,
	u.indexer,
	u.indexer_version,
	u.num_parts,
	u.uploaded_parts,
	u.upload_size,
	u.associated_index_id,
	u.content_type,
	u.should_reindex,
	s.rank,
	u.uncompressed_size
FROM %s
LEFT JOIN (` + uploadRankQueryFragment + `) s
ON u.id = s.id
JOIN repo ON repo.id = u.repository_id
WHERE %s
ORDER BY %s
LIMIT %d OFFSET %d
`

const getUploadsCountQuery = `
%s -- Dynamic CTE definitions for use in the WHERE clause
SELECT COUNT(*) AS count
FROM %s
JOIN repo ON repo.id = u.repository_id
WHERE %s
`

func scanCompleteUpload(s dbutil.Scanner) (upload shared.Upload, _ error) {
	var rawUploadedParts []sql.NullInt32
	if err := s.Scan(
		&upload.ID,
		&upload.Commit,
		&upload.Root,
		&upload.VisibleAtTip,
		&upload.UploadedAt,
		&upload.State,
		&upload.FailureMessage,
		&upload.StartedAt,
		&upload.FinishedAt,
		&upload.ProcessAfter,
		&upload.NumResets,
		&upload.NumFailures,
		&upload.RepositoryID,
		&upload.RepositoryName,
		&upload.Indexer,
		&dbutil.NullString{S: &upload.IndexerVersion},
		&upload.NumParts,
		pq.Array(&rawUploadedParts),
		&upload.UploadSize,
		&upload.AssociatedIndexID,
		&upload.ContentType,
		&upload.ShouldReindex,
		&upload.Rank,
		&upload.UncompressedSize,
	); err != nil {
		return upload, err
	}

	upload.UploadedParts = make([]int, 0, len(rawUploadedParts))
	for _, uploadedPart := range rawUploadedParts {
		upload.UploadedParts = append(upload.UploadedParts, int(uploadedPart.Int32))
	}

	return upload, nil
}

var scanUploadComplete = basestore.NewSliceScanner(scanCompleteUpload)

// scanFirstUpload scans a slice of uploads from the return value of `*Store.query` and returns the first.
var scanFirstUpload = basestore.NewFirstScanner(scanCompleteUpload)

// GetUploadByID returns an upload by its identifier and boolean flag indicating its existence.
func (s *store) GetUploadByID(ctx context.Context, id int) (_ shared.Upload, _ bool, err error) {
	ctx, _, endObservation := s.operations.getUploadByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{attribute.Int("id", id)}})
	defer endObservation(1, observation.Args{})

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return shared.Upload{}, false, err
	}

	return scanFirstUpload(s.db.Query(ctx, sqlf.Sprintf(getUploadByIDQuery, id, authzConds)))
}

const getUploadByIDQuery = `
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	repo.name,
	u.indexer,
	u.indexer_version,
	u.num_parts,
	u.uploaded_parts,
	u.upload_size,
	u.associated_index_id,
	u.content_type,
	u.should_reindex,
	s.rank,
	u.uncompressed_size
FROM lsif_uploads u
LEFT JOIN (` + uploadRankQueryFragment + `) s
ON u.id = s.id
JOIN repo ON repo.id = u.repository_id
WHERE repo.deleted_at IS NULL AND u.state != 'deleted' AND u.id = %s AND %s
`

// GetCompletedUploadsByIDs returns a set of uploads by identifiers.
func (s *store) GetCompletedUploadsByIDs(ctx context.Context, ids []int) (_ []shared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.getCompletedUploadsByIDs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numIDs", len(ids)),
		attribute.IntSlice("ids", ids),
	}})
	defer endObservation(1, observation.Args{})

	if len(ids) == 0 {
		return nil, nil
	}

	var idx []*sqlf.Query
	for _, id := range ids {
		idx = append(idx, sqlf.Sprintf("%s", id))
	}

	// TODO(id: completed-state-check) Make sure we only return uploads with state = 'completed' here
	uploads, err := scanCompletedUploads(s.db.Query(ctx, sqlf.Sprintf(getCompletedUploadsByIDsQuery, sqlf.Join(idx, ", "))))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numUploads", len(uploads)))

	return uploads, nil
}

const getCompletedUploadsByIDsQuery = `
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	u.repository_name,
	u.indexer,
	u.indexer_version,
	u.associated_index_id
FROM lsif_dumps_with_repository_name u WHERE u.id IN (%s)
`

func (s *store) getUploadsByIDs(ctx context.Context, allowDeleted bool, ids ...int) (_ []shared.Upload, err error) {
	ctx, _, endObservation := s.operations.getUploadsByIDs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.IntSlice("ids", ids),
	}})
	defer endObservation(1, observation.Args{})

	if len(ids) == 0 {
		return nil, nil
	}

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return nil, err
	}

	queries := make([]*sqlf.Query, 0, len(ids))
	for _, id := range ids {
		queries = append(queries, sqlf.Sprintf("%d", id))
	}

	cond := sqlf.Sprintf("TRUE")
	if !allowDeleted {
		cond = sqlf.Sprintf("u.state != 'deleted'")
	}

	return scanUploadComplete(s.db.Query(ctx, sqlf.Sprintf(getUploadsByIDsQuery, cond, sqlf.Join(queries, ", "), authzConds)))
}

// GetUploadsByIDs returns an upload for each of the given identifiers. Not all given ids will necessarily
// have a corresponding element in the returned list.
func (s *store) GetUploadsByIDs(ctx context.Context, ids ...int) (_ []shared.Upload, err error) {
	return s.getUploadsByIDs(ctx, false, ids...)
}

func (s *store) GetUploadsByIDsAllowDeleted(ctx context.Context, ids ...int) (_ []shared.Upload, err error) {
	return s.getUploadsByIDs(ctx, true, ids...)
}

const getUploadsByIDsQuery = `
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	repo.name,
	u.indexer,
	u.indexer_version,
	u.num_parts,
	u.uploaded_parts,
	u.upload_size,
	u.associated_index_id,
	u.content_type,
	u.should_reindex,
	s.rank,
	u.uncompressed_size
FROM lsif_uploads u
LEFT JOIN (` + uploadRankQueryFragment + `) s
ON u.id = s.id
JOIN repo ON repo.id = u.repository_id
WHERE repo.deleted_at IS NULL AND %s AND u.id IN (%s) AND %s
`

// GetUploadIDsWithReferences returns uploads that probably contain an import
// or implementation moniker whose identifier matches any of the given monikers' identifiers. This method
// will not return uploads for commits which are unknown to gitserver, nor will it return uploads which
// are listed in the given ignored identifier slice. This method also returns the number of records
// scanned (but possibly filtered out from the return slice) from the database (the offset for the
// subsequent request) and the total number of records in the database.
func (s *store) GetUploadIDsWithReferences(
	ctx context.Context,
	orderedMonikers []precise.QualifiedMonikerData,
	ignoreIDs []int,
	repositoryID int,
	commit string,
	limit int,
	offset int,
	trace observation.TraceLogger,
) (ids []int, recordsScanned int, totalCount int, err error) {
	scanner, totalCount, err := s.GetVisibleUploadsMatchingMonikers(ctx, repositoryID, commit, orderedMonikers, limit, offset)
	if err != nil {
		return nil, 0, 0, errors.Wrap(err, "dbstore.ReferenceIDs")
	}

	defer func() {
		if closeErr := scanner.Close(); closeErr != nil {
			err = errors.Append(err, errors.Wrap(closeErr, "dbstore.ReferenceIDs.Close"))
		}
	}()

	ignoreIDsMap := map[int]struct{}{}
	for _, id := range ignoreIDs {
		ignoreIDsMap[id] = struct{}{}
	}

	filtered := map[int]struct{}{}

	for len(filtered) < limit {
		packageReference, exists, err := scanner.Next()
		if err != nil {
			return nil, 0, 0, errors.Wrap(err, "dbstore.GetUploadIDsWithReferences.Next")
		}
		if !exists {
			break
		}
		recordsScanned++

		if _, ok := filtered[packageReference.UploadID]; ok {
			// This index includes a definition so we can skip testing the filters here. The index
			// will be included in the moniker search regardless if it contains additional references.
			continue
		}

		if _, ok := ignoreIDsMap[packageReference.UploadID]; ok {
			// Ignore this dump
			continue
		}

		filtered[packageReference.UploadID] = struct{}{}
	}

	if trace != nil {
		trace.AddEvent("TODO Domain Owner",
			attribute.Int("uploadIDsWithReferences.numFiltered", len(filtered)),
			attribute.Int("uploadIDsWithReferences.numRecordsScanned", recordsScanned))
	}

	flattened := make([]int, 0, len(filtered))
	for k := range filtered {
		flattened = append(flattened, k)
	}
	sort.Ints(flattened)

	return flattened, recordsScanned, totalCount, nil
}

// GetVisibleUploadsMatchingMonikers returns visible uploads that refer (via package information) to any of
// the given monikers' packages.
func (s *store) GetVisibleUploadsMatchingMonikers(ctx context.Context, repositoryID int, commit string, monikers []precise.QualifiedMonikerData, limit, offset int) (_ shared.PackageReferenceScanner, _ int, err error) {
	ctx, trace, endObservation := s.operations.getVisibleUploadsMatchingMonikers.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", repositoryID),
		attribute.String("commit", commit),
		attribute.Int("numMonikers", len(monikers)),
		attribute.String("monikers", monikersToString(monikers)),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	}})
	defer endObservation(1, observation.Args{})

	if len(monikers) == 0 {
		return PackageReferenceScannerFromSlice(), 0, nil
	}

	qs := make([]*sqlf.Query, 0, len(monikers))
	for _, moniker := range monikers {
		qs = append(qs, sqlf.Sprintf("(%s, %s, %s, %s)", moniker.Scheme, moniker.Manager, moniker.Name, moniker.Version))
	}

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return nil, 0, err
	}

	var (
		countExpr            = sqlf.Sprintf("COUNT(distinct r.dump_id)")
		emptyExpr            = sqlf.Sprintf("")
		selectExpr           = sqlf.Sprintf("r.dump_id, r.scheme, r.manager, r.name, r.version")
		orderLimitOffsetExpr = sqlf.Sprintf(`ORDER BY dump_id LIMIT %s OFFSET %s`, limit, offset)
	)

	countQuery := sqlf.Sprintf(
		referenceIDsQuery,
		repositoryID, dbutil.CommitBytea(commit),
		repositoryID, dbutil.CommitBytea(commit),
		countExpr,
		sqlf.Join(qs, ", "),
		authzConds,
		emptyExpr,
	)
	totalCount, _, err := basestore.ScanFirstInt(s.db.Query(ctx, countQuery))
	if err != nil {
		return nil, 0, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("totalCount", totalCount))

	rows, err := s.db.Query(ctx, sqlf.Sprintf(
		referenceIDsQuery,
		repositoryID, dbutil.CommitBytea(commit),
		repositoryID, dbutil.CommitBytea(commit),
		selectExpr,
		sqlf.Join(qs, ", "),
		authzConds,
		orderLimitOffsetExpr,
	))
	if err != nil {
		return nil, 0, err
	}

	return PackageReferenceScannerFromRows(rows), totalCount, nil
}

const referenceIDsQuery = `
WITH
visible_uploads AS (
	SELECT t.upload_id
	FROM (

		-- Select the set of uploads visible from the given commit. This is done by looking
		-- at each commit's row in the lsif_nearest_uploads table, and the (adjusted) set of
		-- uploads from each commit's nearest ancestor according to the data compressed in
		-- the links table.
		--
		-- NB: A commit should be present in at most one of these tables.
		SELECT
			t.upload_id,
			row_number() OVER (PARTITION BY root, indexer ORDER BY distance) AS r
		FROM (
			SELECT
				upload_id::integer,
				u_distance::text::integer as distance
			FROM lsif_nearest_uploads nu
			CROSS JOIN jsonb_each(nu.uploads) as u(upload_id, u_distance)
			WHERE nu.repository_id = %s AND nu.commit_bytea = %s
			UNION (
				SELECT
					upload_id::integer,
					u_distance::text::integer + ul.distance as distance
				FROM lsif_nearest_uploads_links ul
				JOIN lsif_nearest_uploads nu ON nu.repository_id = ul.repository_id AND nu.commit_bytea = ul.ancestor_commit_bytea
				CROSS JOIN jsonb_each(nu.uploads) as u(upload_id, u_distance)
				WHERE nu.repository_id = %s AND ul.commit_bytea = %s
			)
		) t
		JOIN lsif_uploads u ON u.id = upload_id
	) t
	WHERE t.r <= 1
)
SELECT %s
FROM lsif_references r
LEFT JOIN lsif_dumps u ON u.id = r.dump_id
JOIN repo ON repo.id = u.repository_id
WHERE
	-- Source moniker condition
	(r.scheme, r.manager, r.name, r.version) IN (%s) AND

	-- Visibility conditions
	(
		-- Visibility (local case): if the index belongs to the given repository,
		-- it is visible if it can be seen from the given index
		r.dump_id IN (SELECT * FROM visible_uploads) OR

		-- Visibility (remote case): An index is visible if it can be seen from the
		-- tip of the default branch of its own repository.
		EXISTS (
			SELECT 1
			FROM lsif_uploads_visible_at_tip uvt
			WHERE
				uvt.upload_id = r.dump_id AND
				uvt.is_default_branch
		)
	) AND

	-- Authz conditions
	%s
%s
`

// definitionDumpsLimit is the maximum number of records that can be returned from DefinitionDumps.
var definitionDumpsLimit, _ = strconv.ParseInt(env.Get("PRECISE_CODE_INTEL_DEFINITION_DUMPS_LIMIT", "100", "The maximum number of dumps that can define the same package."), 10, 64)

// GetCompletedUploadsWithDefinitionsForMonikers returns the set of uploads that define at least one of the given monikers.
func (s *store) GetCompletedUploadsWithDefinitionsForMonikers(ctx context.Context, monikers []precise.QualifiedMonikerData) (_ []shared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.getCompletedUploadsWithDefinitionsForMonikers.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numMonikers", len(monikers)),
		attribute.String("monikers", monikersToString(monikers)),
	}})
	defer endObservation(1, observation.Args{})

	if len(monikers) == 0 {
		return nil, nil
	}

	qs := make([]*sqlf.Query, 0, len(monikers))
	for _, moniker := range monikers {
		qs = append(qs, sqlf.Sprintf("(%s, %s, %s, %s)", moniker.Scheme, moniker.Manager, moniker.Name, moniker.Version))
	}

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return nil, err
	}

	// TODO(id: completed-state-check) Make sure we only return uploads with state = 'completed' here
	query := sqlf.Sprintf(getCompletedUploadsWithDefinitionsForMonikersQuery, sqlf.Join(qs, ", "), authzConds, definitionDumpsLimit)
	uploads, err := scanCompletedUploads(s.db.Query(ctx, query))
	if err != nil {
		return nil, err
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numUploads", len(uploads)))

	return uploads, nil
}

const getCompletedUploadsWithDefinitionsForMonikersQuery = `
WITH
ranked_uploads AS (
	SELECT
		u.id,
		-- Rank each upload providing the same package from the same directory
		-- within a repository by commit date. We'll choose the oldest commit
		-- date as the canonical choice used to resolve the current definitions
		-- request.
		` + packageRankingQueryFragment + ` AS rank
	FROM lsif_uploads u
	JOIN lsif_packages p ON p.dump_id = u.id
	JOIN repo ON repo.id = u.repository_id
	WHERE
		-- Don't match deleted uploads
		u.state = 'completed' AND
		(p.scheme, p.manager, p.name, p.version) IN (%s) AND
		%s -- authz conds
),
canonical_uploads AS (
	SELECT ru.id
	FROM ranked_uploads ru
	WHERE ru.rank = 1
	ORDER BY ru.id
	LIMIT %s
)
SELECT
	u.id,
	u.commit,
	u.root,
	EXISTS (` + visibleAtTipSubselectQuery + `) AS visible_at_tip,
	u.uploaded_at,
	u.state,
	u.failure_message,
	u.started_at,
	u.finished_at,
	u.process_after,
	u.num_resets,
	u.num_failures,
	u.repository_id,
	u.repository_name,
	u.indexer,
	u.indexer_version,
	u.associated_index_id
FROM lsif_dumps_with_repository_name u
WHERE u.id IN (SELECT id FROM canonical_uploads)
`

// scanCompletedUploads scans a slice of dumps from the return value of `*Store.query`.
func scanCompletedUpload(s dbutil.Scanner) (upload shared.CompletedUpload, err error) {
	err = s.Scan(
		&upload.ID,
		&upload.Commit,
		&upload.Root,
		&upload.VisibleAtTip,
		&upload.UploadedAt,
		&upload.State,
		&upload.FailureMessage,
		&upload.StartedAt,
		&upload.FinishedAt,
		&upload.ProcessAfter,
		&upload.NumResets,
		&upload.NumFailures,
		&upload.RepositoryID,
		&upload.RepositoryName,
		&upload.Indexer,
		&dbutil.NullString{S: &upload.IndexerVersion},
		&upload.AssociatedIndexID,
	)
	return upload, err
}

var scanCompletedUploads = basestore.NewSliceScanner(scanCompletedUpload)

// GetAuditLogsForUpload returns all the audit logs for the given upload ID in order of entry
// from oldest to newest, according to the auto-incremented internal sequence field.
func (s *store) GetAuditLogsForUpload(ctx context.Context, uploadID int) (_ []shared.UploadLog, err error) {
	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return nil, err
	}

	return scanUploadAuditLogs(s.db.Query(ctx, sqlf.Sprintf(getAuditLogsForUploadQuery, uploadID, authzConds)))
}

const getAuditLogsForUploadQuery = `
SELECT
	u.log_timestamp,
	u.record_deleted_at,
	u.upload_id,
	u.commit,
	u.root,
	u.repository_id,
	u.uploaded_at,
	u.indexer,
	u.indexer_version,
	u.upload_size,
	u.associated_index_id,
	u.transition_columns,
	u.reason,
	u.operation
FROM lsif_uploads_audit_logs u
JOIN repo ON repo.id = u.repository_id
WHERE u.upload_id = %s AND %s
ORDER BY u.sequence
`

func scanUploadAuditLog(s dbutil.Scanner) (log shared.UploadLog, _ error) {
	hstores := pgtype.HstoreArray{}
	err := s.Scan(
		&log.LogTimestamp,
		&log.RecordDeletedAt,
		&log.UploadID,
		&log.Commit,
		&log.Root,
		&log.RepositoryID,
		&log.UploadedAt,
		&log.Indexer,
		&log.IndexerVersion,
		&log.UploadSize,
		&log.AssociatedIndexID,
		&hstores,
		&log.Reason,
		&log.Operation,
	)

	for _, hstore := range hstores.Elements {
		m := make(map[string]*string)
		if err := hstore.AssignTo(&m); err != nil {
			return log, err
		}
		log.TransitionColumns = append(log.TransitionColumns, m)
	}

	return log, err
}

var scanUploadAuditLogs = basestore.NewSliceScanner(scanUploadAuditLog)

// DeleteUploads deletes uploads by filter criteria. The associated repositories will be marked as dirty
// so that their commit graphs will be updated in the background.
func (s *store) DeleteUploads(ctx context.Context, opts shared.DeleteUploadsOptions) (err error) {
	ctx, _, endObservation := s.operations.deleteUploads.With(ctx, &err, observation.Args{Attrs: buildDeleteUploadsLogFields(opts)})
	defer endObservation(1, observation.Args{})

	conds := buildDeleteConditions(opts)
	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return err
	}
	conds = append(conds, authzConds)

	return s.withTransaction(ctx, func(tx *store) error {
		unset, _ := tx.db.SetLocal(ctx, "codeintel.lsif_uploads_audit.reason", "direct delete by filter criteria request")
		defer unset(ctx)

		query := sqlf.Sprintf(
			deleteUploadsQuery,
			sqlf.Join(conds, " AND "),
		)
		repoIDs, err := basestore.ScanInts(s.db.Query(ctx, query))
		if err != nil {
			return err
		}

		var dirtyErr error
		for _, repoID := range repoIDs {
			if err := tx.SetRepositoryAsDirty(ctx, repoID); err != nil {
				dirtyErr = err
			}
		}
		if dirtyErr != nil {
			err = dirtyErr
		}

		return err
	})
}

const deleteUploadsQuery = `
UPDATE lsif_uploads u
SET state = CASE WHEN u.state = 'completed' THEN 'deleting' ELSE 'deleted' END
FROM repo
WHERE repo.id = u.repository_id AND %s
RETURNING repository_id
`

// DeleteUploadByID deletes an upload by its identifier. This method returns a true-valued flag if a record
// was deleted. The associated repository will be marked as dirty so that its commit graph will be updated in
// the background.
func (s *store) DeleteUploadByID(ctx context.Context, id int) (_ bool, err error) {
	ctx, _, endObservation := s.operations.deleteUploadByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	var a bool
	err = s.withTransaction(ctx, func(tx *store) error {
		unset, _ := tx.db.SetLocal(ctx, "codeintel.lsif_uploads_audit.reason", "direct delete by ID request")
		defer unset(ctx)

		repositoryID, deleted, err := basestore.ScanFirstInt(tx.db.Query(ctx, sqlf.Sprintf(deleteUploadByIDQuery, id)))
		if err != nil {
			return err
		}

		if deleted {
			if err := tx.SetRepositoryAsDirty(ctx, repositoryID); err != nil {
				return err
			}
			a = true
		}

		return nil
	})
	return a, err
}

const deleteUploadByIDQuery = `
UPDATE lsif_uploads u
SET
	state = CASE
		WHEN u.state = 'completed' THEN 'deleting'
		ELSE 'deleted'
	END
WHERE id = %s
RETURNING repository_id
`

// ReindexUploads reindexes uploads matching the given filter criteria.
func (s *store) ReindexUploads(ctx context.Context, opts shared.ReindexUploadsOptions) (err error) {
	ctx, _, endObservation := s.operations.reindexUploads.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", opts.RepositoryID),
		attribute.StringSlice("states", opts.States),
		attribute.String("term", opts.Term),
		attribute.Bool("visibleAtTip", opts.VisibleAtTip),
	}})
	defer endObservation(1, observation.Args{})

	var conds []*sqlf.Query

	if opts.RepositoryID != 0 {
		conds = append(conds, sqlf.Sprintf("u.repository_id = %s", opts.RepositoryID))
	}
	if opts.Term != "" {
		conds = append(conds, makeSearchCondition(opts.Term))
	}
	if len(opts.States) > 0 {
		conds = append(conds, makeStateCondition(opts.States))
	}
	if opts.VisibleAtTip {
		conds = append(conds, sqlf.Sprintf("EXISTS ("+visibleAtTipSubselectQuery+")"))
	}
	if len(opts.IndexerNames) != 0 {
		var indexerConds []*sqlf.Query
		for _, indexerName := range opts.IndexerNames {
			indexerConds = append(indexerConds, sqlf.Sprintf("u.indexer ILIKE %s", "%"+indexerName+"%"))
		}

		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(indexerConds, " OR ")))
	}

	authzConds, err := database.AuthzQueryConds(ctx, database.NewDBWith(s.logger, s.db))
	if err != nil {
		return err
	}
	conds = append(conds, authzConds)

	return s.withTransaction(ctx, func(tx *store) error {
		unset, _ := tx.db.SetLocal(ctx, "codeintel.lsif_uploads_audit.reason", "direct reindex by filter criteria request")
		defer unset(ctx)

		return tx.db.Exec(ctx, sqlf.Sprintf(reindexUploadsQuery, sqlf.Join(conds, " AND ")))
	})
}

const reindexUploadsQuery = `
WITH
upload_candidates AS (
    SELECT u.id, u.associated_index_id
	FROM lsif_uploads u
	JOIN repo ON repo.id = u.repository_id
	WHERE %s
    ORDER BY u.id
    FOR UPDATE
),
update_uploads AS (
	UPDATE lsif_uploads u
	SET should_reindex = true
	WHERE u.id IN (SELECT id FROM upload_candidates)
),
index_candidates AS (
	SELECT u.id
	FROM lsif_indexes u
	WHERE u.id IN (SELECT associated_index_id FROM upload_candidates)
	ORDER BY u.id
	FOR UPDATE
)
UPDATE lsif_indexes u
SET should_reindex = true
WHERE u.id IN (SELECT id FROM index_candidates)
`

// ReindexUploadByID reindexes an upload by its identifier.
func (s *store) ReindexUploadByID(ctx context.Context, id int) (err error) {
	ctx, _, endObservation := s.operations.reindexUploadByID.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	return s.db.Exec(ctx, sqlf.Sprintf(reindexUploadByIDQuery, id, id))
}

const reindexUploadByIDQuery = `
WITH
update_uploads AS (
	UPDATE lsif_uploads u
	SET should_reindex = true
	WHERE id = %s
)
UPDATE lsif_indexes u
SET should_reindex = true
WHERE id IN (SELECT associated_index_id FROM lsif_uploads WHERE id = %s)
`

//
//

// makeStateCondition returns a disjunction of clauses comparing the upload against the target state.
func makeStateCondition(states []string) *sqlf.Query {
	stateMap := make(map[string]struct{}, 2)
	for _, state := range states {
		// Treat errored and failed states as equivalent
		if state == "errored" || state == "failed" {
			stateMap["errored"] = struct{}{}
			stateMap["failed"] = struct{}{}
		} else {
			stateMap[state] = struct{}{}
		}
	}

	orderedStates := make([]string, 0, len(stateMap))
	for state := range stateMap {
		orderedStates = append(orderedStates, state)
	}
	sort.Strings(orderedStates)

	if len(orderedStates) == 1 {
		return sqlf.Sprintf("u.state = %s", orderedStates[0])
	}

	return sqlf.Sprintf("u.state = ANY(%s)", pq.Array(orderedStates))
}

// makeSearchCondition returns a disjunction of LIKE clauses against all searchable columns of an upload.
func makeSearchCondition(term string) *sqlf.Query {
	searchableColumns := []string{
		"u.commit",
		"u.root",
		"(u.state)::text",
		"u.failure_message",
		"repo.name",
		"u.indexer",
		"u.indexer_version",
	}

	var termConds []*sqlf.Query
	for _, column := range searchableColumns {
		termConds = append(termConds, sqlf.Sprintf(column+" ILIKE %s", "%"+term+"%"))
	}

	return sqlf.Sprintf("(%s)", sqlf.Join(termConds, " OR "))
}

func buildDeleteConditions(opts shared.DeleteUploadsOptions) []*sqlf.Query {
	conds := []*sqlf.Query{}
	if opts.RepositoryID != 0 {
		conds = append(conds, sqlf.Sprintf("u.repository_id = %s", opts.RepositoryID))
	}
	conds = append(conds, sqlf.Sprintf("repo.deleted_at IS NULL"))
	conds = append(conds, sqlf.Sprintf("u.state != 'deleted'"))
	if opts.Term != "" {
		conds = append(conds, makeSearchCondition(opts.Term))
	}
	if len(opts.States) > 0 {
		conds = append(conds, makeStateCondition(opts.States))
	}
	if opts.VisibleAtTip {
		conds = append(conds, sqlf.Sprintf("EXISTS ("+visibleAtTipSubselectQuery+")"))
	}
	if len(opts.IndexerNames) != 0 {
		var indexerConds []*sqlf.Query
		for _, indexerName := range opts.IndexerNames {
			indexerConds = append(indexerConds, sqlf.Sprintf("u.indexer ILIKE %s", "%"+indexerName+"%"))
		}

		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(indexerConds, " OR ")))
	}

	return conds
}

type cteDefinition struct {
	name       string
	definition *sqlf.Query
}

func buildGetConditionsAndCte(opts shared.GetUploadsOptions) (*sqlf.Query, []*sqlf.Query, []cteDefinition) {
	conds := make([]*sqlf.Query, 0, 13)

	allowDeletedUploads := opts.AllowDeletedUpload && (opts.State == "" || opts.State == "deleted")

	if opts.RepositoryID != 0 {
		conds = append(conds, sqlf.Sprintf("u.repository_id = %s", opts.RepositoryID))
	}
	if opts.Term != "" {
		conds = append(conds, makeSearchCondition(opts.Term))
	}
	if opts.State != "" {
		opts.States = append(opts.States, opts.State)
	}
	if len(opts.States) > 0 {
		conds = append(conds, makeStateCondition(opts.States))
	} else if !allowDeletedUploads {
		conds = append(conds, sqlf.Sprintf("u.state != 'deleted'"))
	}
	if opts.VisibleAtTip {
		conds = append(conds, sqlf.Sprintf("EXISTS ("+visibleAtTipSubselectQuery+")"))
	}

	cteDefinitions := make([]cteDefinition, 0, 2)
	if opts.DependencyOf != 0 {
		cteDefinitions = append(cteDefinitions, cteDefinition{
			name:       "ranked_dependencies",
			definition: sqlf.Sprintf(rankedDependencyCandidateCTEQuery, sqlf.Sprintf("r.dump_id = %s", opts.DependencyOf)),
		})

		// Limit results to the set of uploads canonically providing packages referenced by the given upload identifier
		// (opts.DependencyOf). We do this by selecting the top ranked values in the CTE defined above, which are the
		// referenced package providers grouped by package name, version, repository, and root.
		conds = append(conds, sqlf.Sprintf(`u.id IN (SELECT rd.pkg_id FROM ranked_dependencies rd WHERE rd.rank = 1)`))
	}
	if opts.DependentOf != 0 {
		cteCondition := sqlf.Sprintf(`(p.scheme, p.manager, p.name, p.version) IN (
			SELECT p.scheme, p.manager, p.name, p.version
			FROM lsif_packages p
			WHERE p.dump_id = %s
		)`, opts.DependentOf)

		cteDefinitions = append(cteDefinitions, cteDefinition{
			name:       "ranked_dependents",
			definition: sqlf.Sprintf(rankedDependentCandidateCTEQuery, cteCondition),
		})

		// Limit results to the set of uploads that reference the target upload if it canonically provides the
		// matching package. If the target upload does not canonically provide a package, the results will contain
		// no dependent uploads.
		conds = append(conds, sqlf.Sprintf(`u.id IN (
			SELECT r.dump_id
			FROM ranked_dependents rd
			JOIN lsif_references r ON
				r.scheme = rd.scheme AND
				r.manager = rd.manager AND
				r.name = rd.name AND
				r.version = rd.version AND
				r.dump_id != rd.pkg_id
			WHERE rd.pkg_id = %s AND rd.rank = 1
		)`, opts.DependentOf))
	}

	if len(opts.IndexerNames) != 0 {
		var indexerConds []*sqlf.Query
		for _, indexerName := range opts.IndexerNames {
			indexerConds = append(indexerConds, sqlf.Sprintf("u.indexer ILIKE %s", "%"+indexerName+"%"))
		}

		conds = append(conds, sqlf.Sprintf("(%s)", sqlf.Join(indexerConds, " OR ")))
	}

	sourceTableExpr := sqlf.Sprintf("lsif_uploads u")
	if allowDeletedUploads {
		cteDefinitions = append(cteDefinitions, cteDefinition{
			name:       "deleted_uploads",
			definition: sqlf.Sprintf(deletedUploadsFromAuditLogsCTEQuery),
		})

		sourceTableExpr = sqlf.Sprintf(`(
			SELECT
				id,
				commit,
				root,
				uploaded_at,
				state,
				failure_message,
				started_at,
				finished_at,
				process_after,
				num_resets,
				num_failures,
				repository_id,
				indexer,
				indexer_version,
				num_parts,
				uploaded_parts,
				upload_size,
				associated_index_id,
				content_type,
				should_reindex,
				expired,
				uncompressed_size
			FROM lsif_uploads
			UNION ALL
			SELECT *
			FROM deleted_uploads
		) AS u`)
	}

	if opts.UploadedBefore != nil {
		conds = append(conds, sqlf.Sprintf("u.uploaded_at < %s", *opts.UploadedBefore))
	}
	if opts.UploadedAfter != nil {
		conds = append(conds, sqlf.Sprintf("u.uploaded_at > %s", *opts.UploadedAfter))
	}
	if opts.InCommitGraph {
		conds = append(conds, sqlf.Sprintf("u.finished_at < (SELECT updated_at FROM lsif_dirty_repositories ldr WHERE ldr.repository_id = u.repository_id)"))
	}
	if opts.LastRetentionScanBefore != nil {
		conds = append(conds, sqlf.Sprintf("(u.last_retention_scan_at IS NULL OR u.last_retention_scan_at < %s)", *opts.LastRetentionScanBefore))
	}
	if !opts.AllowExpired {
		conds = append(conds, sqlf.Sprintf("NOT u.expired"))
	}
	if !opts.AllowDeletedRepo {
		conds = append(conds, sqlf.Sprintf("repo.deleted_at IS NULL"))
	}
	// Never show uploads for deleted repos
	conds = append(conds, sqlf.Sprintf("repo.blocked IS NULL"))

	return sourceTableExpr, conds, cteDefinitions
}

const rankedDependencyCandidateCTEQuery = `
SELECT
	p.dump_id as pkg_id,
	r.dump_id as ref_id,
	-- Rank each upload providing the same package from the same directory
	-- within a repository by commit date. We'll choose the oldest commit
	-- date as the canonical choice and ignore the uploads for younger
	-- commits providing the same package.
	` + packageRankingQueryFragment + ` AS rank
FROM lsif_uploads u
JOIN lsif_packages p ON p.dump_id = u.id
JOIN lsif_references r ON
	r.scheme = p.scheme AND
	r.manager = p.manager AND
	r.name = p.name AND
	r.version = p.version AND
	r.dump_id != p.dump_id
WHERE
	-- Don't match deleted uploads
	u.state = 'completed' AND
	%s
`

const rankedDependentCandidateCTEQuery = `
SELECT
	p.dump_id AS pkg_id,
	p.scheme AS scheme,
	p.manager AS manager,
	p.name AS name,
	p.version AS version,
	-- Rank each upload providing the same package from the same directory
	-- within a repository by commit date. We'll choose the oldest commit
	-- date as the canonical choice and ignore the uploads for younger
	-- commits providing the same package.
	` + packageRankingQueryFragment + ` AS rank
FROM lsif_uploads u
JOIN lsif_packages p ON p.dump_id = u.id
WHERE
	-- Don't match deleted uploads
	u.state = 'completed' AND
	%s
`

const deletedUploadsFromAuditLogsCTEQuery = `
SELECT
	DISTINCT ON(s.upload_id) s.upload_id AS id, au.commit, au.root,
	au.uploaded_at, 'deleted' AS state,
	snapshot->'failure_message' AS failure_message,
	(snapshot->'started_at')::timestamptz AS started_at,
	(snapshot->'finished_at')::timestamptz AS finished_at,
	(snapshot->'process_after')::timestamptz AS process_after,
	COALESCE((snapshot->'num_resets')::integer, -1) AS num_resets,
	COALESCE((snapshot->'num_failures')::integer, -1) AS num_failures,
	au.repository_id,
	au.indexer, au.indexer_version,
	COALESCE((snapshot->'num_parts')::integer, -1) AS num_parts,
	NULL::integer[] as uploaded_parts,
	au.upload_size, au.associated_index_id, au.content_type,
	false AS should_reindex, -- TODO
	COALESCE((snapshot->'expired')::boolean, false) AS expired,
	NULL::bigint AS uncompressed_size
FROM (
	SELECT upload_id, snapshot_transition_columns(transition_columns ORDER BY sequence ASC) AS snapshot
	FROM lsif_uploads_audit_logs
	WHERE record_deleted_at IS NOT NULL
	GROUP BY upload_id
) AS s
JOIN lsif_uploads_audit_logs au ON au.upload_id = s.upload_id
`

func buildGetUploadsLogFields(opts shared.GetUploadsOptions) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("repositoryID", opts.RepositoryID),
		attribute.String("state", opts.State),
		attribute.String("term", opts.Term),
		attribute.Bool("visibleAtTip", opts.VisibleAtTip),
		attribute.Int("dependencyOf", opts.DependencyOf),
		attribute.Int("dependentOf", opts.DependentOf),
		attribute.String("uploadedBefore", nilTimeToString(opts.UploadedBefore)),
		attribute.String("uploadedAfter", nilTimeToString(opts.UploadedAfter)),
		attribute.String("lastRetentionScanBefore", nilTimeToString(opts.LastRetentionScanBefore)),
		attribute.Bool("inCommitGraph", opts.InCommitGraph),
		attribute.Bool("allowExpired", opts.AllowExpired),
		attribute.Bool("oldestFirst", opts.OldestFirst),
		attribute.Int("limit", opts.Limit),
		attribute.Int("offset", opts.Offset),
	}
}

func buildDeleteUploadsLogFields(opts shared.DeleteUploadsOptions) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.StringSlice("states", opts.States),
		attribute.String("term", opts.Term),
		attribute.Bool("visibleAtTip", opts.VisibleAtTip),
	}
}

func buildCTEPrefix(cteDefinitions []cteDefinition) *sqlf.Query {
	if len(cteDefinitions) == 0 {
		return sqlf.Sprintf("")
	}

	cteQueries := make([]*sqlf.Query, 0, len(cteDefinitions))
	for _, cte := range cteDefinitions {
		cteQueries = append(cteQueries, sqlf.Sprintf("%s AS (%s)", sqlf.Sprintf(cte.name), cte.definition))
	}

	return sqlf.Sprintf("WITH\n%s", sqlf.Join(cteQueries, ",\n"))
}

//
//

func monikersToString(vs []precise.QualifiedMonikerData) string {
	strs := make([]string, 0, len(vs))
	for _, v := range vs {
		strs = append(strs, fmt.Sprintf("%s:%s:%s:%s:%s", v.Kind, v.Scheme, v.Manager, v.Identifier, v.Version))
	}

	return strings.Join(strs, ", ")
}

func nilTimeToString(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.String()
}
