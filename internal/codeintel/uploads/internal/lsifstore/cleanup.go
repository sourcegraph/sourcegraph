package lsifstore

import (
	"context"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func (s *store) IDsWithMeta(ctx context.Context, ids []int) (_ []int, err error) {
	ctx, _, endObservation := s.operations.idsWithMeta.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numIDs", len(ids)),
		attribute.IntSlice("ids", ids),
	}})
	defer endObservation(1, observation.Args{})

	return basestore.ScanInts(s.db.Query(ctx, sqlf.Sprintf(
		idsWithMetaQuery,
		pq.Array(ids),
	)))
}

const idsWithMetaQuery = `
SELECT m.upload_id
FROM codeintel_scip_metadata m
WHERE m.upload_id = ANY(%s)
`

func (s *store) ReconcileCandidates(ctx context.Context, batchSize int) (_ []int, err error) {
	ctx, _, endObservation := s.operations.reconcileCandidates.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("batchSize", batchSize),
	}})
	defer endObservation(1, observation.Args{})

	return s.ReconcileCandidatesWithTime(ctx, batchSize, time.Now().UTC())
}

func (s *store) ReconcileCandidatesWithTime(ctx context.Context, batchSize int, now time.Time) (_ []int, err error) {
	return basestore.ScanInts(s.db.Query(ctx, sqlf.Sprintf(reconcileQuery, batchSize, batchSize, now, now)))
}

const reconcileQuery = `
WITH
unscanned_candidates AS (
	SELECT m.upload_id
	FROM codeintel_scip_metadata m
	WHERE NOT EXISTS (SELECT 1 FROM codeintel_last_reconcile lr WHERE lr.dump_id = m.upload_id)
	ORDER BY m.upload_id
),
scanned_candidates AS (
	SELECT lr.dump_id AS upload_id
	FROM codeintel_last_reconcile lr
	ORDER BY lr.last_reconcile_at, lr.dump_id
),
ordered_candidates AS (
	(
		SELECT upload_id FROM unscanned_candidates
		LIMIT %s
	) UNION ALL (
		SELECT upload_id FROM scanned_candidates
	)
	LIMIT %s
)
INSERT INTO codeintel_last_reconcile
SELECT upload_id, %s FROM ordered_candidates
ON CONFLICT (dump_id) DO UPDATE
SET last_reconcile_at = %s
RETURNING dump_id
`

func (s *store) DeleteLsifDataByUploadIds(ctx context.Context, bundleIDs ...int) (err error) {
	ctx, _, endObservation := s.operations.deleteLsifDataByUploadIds.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numBundleIDs", len(bundleIDs)),
		attribute.IntSlice("bundleIDs", bundleIDs),
	}})
	defer endObservation(1, observation.Args{})

	if len(bundleIDs) == 0 {
		return nil
	}

	return s.withTransaction(ctx, func(tx *store) error {
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPDocumentLookupQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPMetadataQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPSymbolNamesQuery, pq.Array(bundleIDs), pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPDocumentLookupSchemaVersionsQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPSymbolsSchemaVersionsQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPSymbolLookupsQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := tx.db.Exec(ctx, sqlf.Sprintf(deleteSCIPSymbolLookupLeavesQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}
		if err := s.db.Exec(ctx, sqlf.Sprintf(deleteLastReconcileQuery, pq.Array(bundleIDs))); err != nil {
			return err
		}

		return nil
	})
}

const deleteSCIPMetadataQuery = `
 WITH
 locked_metadata AS (
 	SELECT id
 	FROM codeintel_scip_metadata
 	WHERE upload_id = ANY(%s)
 	ORDER BY id
 	FOR UPDATE
 )
DELETE FROM codeintel_scip_metadata
WHERE id IN (SELECT id FROM locked_metadata)
`

const deleteSCIPDocumentLookupQuery = `
WITH
locked_document_lookup AS (
	SELECT id
	FROM codeintel_scip_document_lookup
	WHERE upload_id = ANY(%s)
	ORDER BY id
	FOR UPDATE
)
DELETE FROM codeintel_scip_document_lookup
WHERE id IN (SELECT id FROM locked_document_lookup)
`

const deleteSCIPSymbolNamesQuery = `
WITH
locked_symbol_names AS (
	SELECT id
	FROM codeintel_scip_symbol_names
	WHERE upload_id = ANY(%s)
	ORDER BY id
	FOR UPDATE
)
DELETE FROM codeintel_scip_symbol_names
WHERE upload_id = ANY(%s) AND id IN (SELECT id FROM locked_symbol_names)
`

const deleteSCIPDocumentLookupSchemaVersionsQuery = `
DELETE FROM codeintel_scip_document_lookup_schema_versions WHERE upload_id = ANY(%s)
`

const deleteSCIPSymbolsSchemaVersionsQuery = `
DELETE FROM codeintel_scip_symbols_schema_versions WHERE upload_id = ANY(%s)
`

const deleteSCIPSymbolLookupsQuery = `
DELETE FROM codeintel_scip_symbols_lookup WHERE upload_id = ANY(%s)
`

const deleteSCIPSymbolLookupLeavesQuery = `
DELETE FROM codeintel_scip_symbols_lookup_leaves WHERE upload_id = ANY(%s)
`

const deleteLastReconcileQuery = `
WITH locked_rows AS (
	SELECT dump_id
	FROM codeintel_last_reconcile
	WHERE dump_id = ANY(%s)
	ORDER BY dump_id
	FOR UPDATE
)
DELETE FROM codeintel_last_reconcile
WHERE dump_id IN (SELECT dump_id FROM locked_rows)
`

func (s *store) DeleteAbandonedSchemaVersionsRecords(ctx context.Context) (_ int, err error) {
	tx, err := s.db.Transact(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { err = tx.Done(err) }()

	count1, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(deleteAbandonedSymbolsSchemaVersionsQuery)))
	if err != nil {
		return 0, err
	}

	count2, _, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(deleteAbandonedDocumentLookupSchemaVersionsQuery)))
	if err != nil {
		return 0, err
	}

	return count1 + count2, nil
}

const deleteAbandonedSymbolsSchemaVersionsQuery = `
WITH del AS (
	DELETE FROM codeintel_scip_symbols_schema_versions sv
	WHERE NOT EXISTS (
		SELECT 1
		FROM codeintel_scip_metadata m
		WHERE m.upload_id = sv.upload_id
	)
	RETURNING 1
)
SELECT COUNT(*) FROM del
`

const deleteAbandonedDocumentLookupSchemaVersionsQuery = `
WITH del AS (
	DELETE FROM codeintel_scip_document_lookup_schema_versions sv
	WHERE NOT EXISTS (
		SELECT 1
		FROM codeintel_scip_metadata m
		WHERE m.upload_id = sv.upload_id
	)
	RETURNING 1
)
SELECT COUNT(*) FROM del
`

func (s *store) DeleteUnreferencedDocuments(ctx context.Context, batchSize int, maxAge time.Duration, now time.Time) (_, _ int, err error) {
	ctx, _, endObservation := s.operations.idsWithMeta.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Stringer("maxAge", maxAge),
	}})
	defer endObservation(1, observation.Args{})

	rows, err := s.db.Query(ctx, sqlf.Sprintf(
		deleteUnreferencedDocumentsQuery,
		now,
		maxAge/time.Second,
		batchSize,
	))
	if err != nil {
		return 0, 0, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	var c1, c2 int
	for rows.Next() {
		if err := rows.Scan(&c1, &c2); err != nil {
			return 0, 0, err
		}
	}

	return c1, c2, nil
}

const deleteUnreferencedDocumentsQuery = `
WITH
candidates AS (
	SELECT id, document_id
	FROM codeintel_scip_documents_dereference_logs log
	WHERE %s - log.last_removal_time > (%s * interval '1 second')
	ORDER BY last_removal_time DESC, document_id
	LIMIT %s
	FOR UPDATE SKIP LOCKED
),
locked_documents AS (
	SELECT sd.id
	FROM candidates d
	JOIN codeintel_scip_documents sd ON sd.id = d.document_id
	WHERE NOT EXISTS (SELECT 1 FROM codeintel_scip_document_lookup sdl WHERE sdl.document_id = sd.id)
	ORDER BY sd.id
	FOR UPDATE OF sd
),
deleted_documents AS (
	DELETE FROM codeintel_scip_documents
	WHERE id IN (SELECT id FROM locked_documents)
	RETURNING id
),
deleted_candidates AS (
	DELETE FROM codeintel_scip_documents_dereference_logs
	WHERE id IN (SELECT id FROM candidates)
	RETURNING id
)
SELECT
	(SELECT COUNT(*) FROM candidates),
	(SELECT COUNT(*) FROM deleted_documents)
`
