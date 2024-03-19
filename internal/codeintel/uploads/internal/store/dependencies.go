package store

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

// ReferencesForUpload returns the set of import monikers attached to the given upload identifier.
func (s *store) ReferencesForUpload(ctx context.Context, uploadID int) (_ shared.PackageReferenceScanner, err error) {
	ctx, _, endObservation := s.operations.referencesForUpload.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("uploadID", uploadID),
	}})
	defer endObservation(1, observation.Args{})

	rows, err := s.db.Query(ctx, sqlf.Sprintf(referencesForUploadQuery, uploadID))
	if err != nil {
		return nil, err
	}

	return PackageReferenceScannerFromRows(rows), nil
}

const referencesForUploadQuery = `
SELECT r.dump_id, r.scheme, r.manager, r.name, r.version
FROM lsif_references r
WHERE dump_id = %s
ORDER BY r.scheme, r.manager, r.name, r.version
`

// UpdatePackages upserts package data tied to the given upload.
func (s *store) UpdatePackages(ctx context.Context, uploadID int, packages []precise.Package) (err error) {
	ctx, _, endObservation := s.operations.updatePackages.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numPackages", len(packages)),
	}})
	defer endObservation(1, observation.Args{})

	if len(packages) == 0 {
		return nil
	}

	return s.withTransaction(ctx, func(tx *store) error {
		// Create temporary table symmetric to lsif_packages without the dump id
		if err := tx.db.Exec(ctx, sqlf.Sprintf(updatePackagesTemporaryTableQuery)); err != nil {
			return err
		}

		// Bulk insert all the unique column values into the temporary table
		if err := batch.InsertValues(
			ctx,
			tx.db.Handle(),
			"t_lsif_packages",
			batch.MaxNumPostgresParameters,
			[]string{"scheme", "manager", "name", "version"},
			loadPackagesChannel(packages),
		); err != nil {
			return err
		}

		// Insert the values from the temporary table into the target table. We select a
		// parameterized dump id here since it is the same for all rows in this operation.
		return tx.db.Exec(ctx, sqlf.Sprintf(updatePackagesInsertQuery, uploadID))
	})
}

const updatePackagesTemporaryTableQuery = `
CREATE TEMPORARY TABLE t_lsif_packages (
	scheme text NOT NULL,
	manager text NOT NULL,
	name text NOT NULL,
	version text NOT NULL
) ON COMMIT DROP
`

const updatePackagesInsertQuery = `
INSERT INTO lsif_packages (dump_id, scheme, manager, name, version)
SELECT %s, source.scheme, source.manager, source.name, source.version
FROM t_lsif_packages source
`

func loadPackagesChannel(packages []precise.Package) <-chan []any {
	ch := make(chan []any, len(packages))

	go func() {
		defer close(ch)

		for _, p := range packages {
			ch <- []any{p.Scheme, p.Manager, p.Name, p.Version}
		}
	}()

	return ch
}

// UpdatePackageReferences inserts reference data tied to the given upload.
func (s *store) UpdatePackageReferences(ctx context.Context, uploadID int, references []precise.PackageReference) (err error) {
	ctx, _, endObservation := s.operations.updatePackageReferences.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numReferences", len(references)),
	}})
	defer endObservation(1, observation.Args{})

	if len(references) == 0 {
		return nil
	}

	return s.withTransaction(ctx, func(tx *store) error {
		// Create temporary table symmetric to lsif_references without the dump id
		if err := tx.db.Exec(ctx, sqlf.Sprintf(updateReferencesTemporaryTableQuery)); err != nil {
			return err
		}

		// Bulk insert all the unique column values into the temporary table
		if err := batch.InsertValues(
			ctx,
			tx.db.Handle(),
			"t_lsif_references",
			batch.MaxNumPostgresParameters,
			[]string{"scheme", "manager", "name", "version"},
			loadReferencesChannel(references),
		); err != nil {
			return err
		}

		// Insert the values from the temporary table into the target table. We select a
		// parameterized dump id here since it is the same for all rows in this operation.
		return tx.db.Exec(ctx, sqlf.Sprintf(updateReferencesInsertQuery, uploadID))
	})
}

const updateReferencesTemporaryTableQuery = `
CREATE TEMPORARY TABLE t_lsif_references (
	scheme text NOT NULL,
	manager text NOT NULL,
	name text NOT NULL,
	version text NOT NULL
) ON COMMIT DROP
`

const updateReferencesInsertQuery = `
INSERT INTO lsif_references (dump_id, scheme, manager, name, version)
SELECT %s, source.scheme, source.manager, source.name, source.version
FROM t_lsif_references source
`

func loadReferencesChannel(references []precise.PackageReference) <-chan []any {
	ch := make(chan []any, len(references))

	go func() {
		defer close(ch)

		for _, r := range references {
			ch <- []any{r.Scheme, r.Manager, r.Name, r.Version}
		}
	}()

	return ch
}

//
//

type rowScanner struct {
	rows *sql.Rows
}

// packageReferenceScannerFromRows creates a PackageReferenceScanner that feeds the given values.
func PackageReferenceScannerFromRows(rows *sql.Rows) shared.PackageReferenceScanner {
	return &rowScanner{
		rows: rows,
	}
}

// Next reads the next package reference value from the database cursor.
func (s *rowScanner) Next() (reference shared.PackageReference, _ bool, _ error) {
	if !s.rows.Next() {
		return shared.PackageReference{}, false, nil
	}

	if err := s.rows.Scan(
		&reference.UploadID,
		&reference.Scheme,
		&reference.Manager,
		&reference.Name,
		&reference.Version,
	); err != nil {
		return shared.PackageReference{}, false, err
	}

	return reference, true, nil
}

// Close the underlying row object.
func (s *rowScanner) Close() error {
	return basestore.CloseRows(s.rows, nil)
}

type sliceScanner struct {
	references []shared.PackageReference
}

// PackageReferenceScannerFromSlice creates a PackageReferenceScanner that feeds the given values.
func PackageReferenceScannerFromSlice(references ...shared.PackageReference) shared.PackageReferenceScanner {
	return &sliceScanner{
		references: references,
	}
}

func (s *sliceScanner) Next() (shared.PackageReference, bool, error) {
	if len(s.references) == 0 {
		return shared.PackageReference{}, false, nil
	}

	next := s.references[0]
	s.references = s.references[1:]
	return next, true, nil
}

func (s *sliceScanner) Close() error {
	return nil
}
