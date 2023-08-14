package scip

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
)

type inserterFunc func(ctx context.Context, symbolLookupInserter *batch.Inserter) error

func withSymbolLookupSegmentTypeInserter(ctx context.Context, tx *basestore.Store, uploadID int, f inserterFunc) error {
	createTempTableQuery := sqlf.Sprintf(`
		CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_segment_type(
			name text NOT NULL,
			id integer NOT NULL,
			segment_type SymbolNameSegmentType NOT NULL,
			segment_quality SymbolNameSegmentQuality,
			parent_id integer
		) ON COMMIT DROP
	`)

	insertQuery := sqlf.Sprintf(`
		INSERT INTO codeintel_scip_symbols_lookup (id, upload_id, name, segment_type, parent_id)
		SELECT id, %s, name, segment_type, parent_id
		FROM t_codeintel_scip_symbols_lookup_segment_type
	`,
		uploadID,
	)

	return withInserter(ctx, tx, f, createTempTableQuery, insertQuery, batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols_lookup_segment_type",
		batch.MaxNumPostgresParameters,
		"segment_type",
		"name",
		"id",
		"parent_id",
	))
}

func withSymbolLookupSegmentQualityInserter(ctx context.Context, tx *basestore.Store, uploadID int, f inserterFunc) error {
	createTempTableQuery := sqlf.Sprintf(`
		CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_segment_quality(
			name text NOT NULL,
			id integer NOT NULL,
			segment_quality SymbolNameSegmentQuality,
			parent_id integer
		) ON COMMIT DROP
	`)

	insertQuery := sqlf.Sprintf(`
		INSERT INTO codeintel_scip_symbols_lookup (id, upload_id, name, segment_type, segment_quality, parent_id)
		SELECT id, %s, name, 'DESCRIPTOR_SUFFIX', segment_quality, parent_id
		FROM t_codeintel_scip_symbols_lookup_segment_quality
	`,
		uploadID,
	)

	return withInserter(ctx, tx, f, createTempTableQuery, insertQuery, batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols_lookup_segment_quality",
		batch.MaxNumPostgresParameters,
		"segment_quality",
		"name",
		"id",
		"parent_id",
	))
}

func withSymbolLookupCommonSuffixInserter(ctx context.Context, tx *basestore.Store, uploadID int, f inserterFunc) error {
	createTempTableQuery := sqlf.Sprintf(`
		CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_common_suffix(
			name text NOT NULL,
			id integer NOT NULL,
			parent_id integer
		) ON COMMIT DROP
	`)

	insertQuery := sqlf.Sprintf(`
		INSERT INTO codeintel_scip_symbols_lookup (id, upload_id, name, segment_type, segment_quality, parent_id)
		SELECT id, %s, name, 'DESCRIPTOR_SUFFIX', 'BOTH', parent_id
		FROM t_codeintel_scip_symbols_lookup_common_suffix
	`,
		uploadID,
	)

	return withInserter(ctx, tx, f, createTempTableQuery, insertQuery, batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols_lookup_common_suffix",
		batch.MaxNumPostgresParameters,
		"name",
		"id",
		"parent_id",
	))
}

func withSymbolLookupLeavesWithFuzzyInserter(ctx context.Context, tx *basestore.Store, uploadID int, f inserterFunc) error {
	createTempTableQuery := sqlf.Sprintf(`
		CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_leaves_with_fuzzy(
			symbol_id integer NOT NULL,
			descriptor_suffix_id integer NOT NULL,
			fuzzy_descriptor_suffix_id integer NOT NULL
		) ON COMMIT DROP
	`)

	insertQuery := sqlf.Sprintf(`
		INSERT INTO codeintel_scip_symbols_lookup_leaves (upload_id, symbol_id, descriptor_suffix_id, fuzzy_descriptor_suffix_id)
		SELECT %s, symbol_id, descriptor_suffix_id, fuzzy_descriptor_suffix_id
		FROM t_codeintel_scip_symbols_lookup_leaves_with_fuzzy
	`,
		uploadID,
	)

	return withInserter(ctx, tx, f, createTempTableQuery, insertQuery, batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols_lookup_leaves_with_fuzzy",
		batch.MaxNumPostgresParameters,
		"symbol_id",
		"descriptor_suffix_id",
		"fuzzy_descriptor_suffix_id",
	))
}

func withSymbolLookupLeavesWithoutFuzzyInserter(ctx context.Context, tx *basestore.Store, uploadID int, f inserterFunc) error {
	createTempTableQuery := sqlf.Sprintf(`
		CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_leaves_without_fuzzy(
			symbol_id integer NOT NULL,
			descriptor_suffix_id integer NOT NULL
		) ON COMMIT DROP
	`)

	insertQuery := sqlf.Sprintf(`
		INSERT INTO codeintel_scip_symbols_lookup_leaves (upload_id, symbol_id, descriptor_suffix_id)
		SELECT %s, symbol_id, descriptor_suffix_id
		FROM t_codeintel_scip_symbols_lookup_leaves_without_fuzzy
	`,
		uploadID,
	)

	return withInserter(ctx, tx, f, createTempTableQuery, insertQuery, batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols_lookup_leaves_without_fuzzy",
		batch.MaxNumPostgresParameters,
		"symbol_id",
		"descriptor_suffix_id",
	))
}

func withInserter(
	ctx context.Context,
	tx *basestore.Store,
	f inserterFunc,
	createTempTableQuery, insertQuery *sqlf.Query,
	symbolLookupInserter *batch.Inserter,
) error {
	if err := tx.Exec(ctx, createTempTableQuery); err != nil {
		return err
	}

	if err := f(ctx, symbolLookupInserter); err != nil {
		return err
	}

	if err := symbolLookupInserter.Flush(ctx); err != nil {
		return err
	}

	return tx.Exec(ctx, insertQuery)
}
