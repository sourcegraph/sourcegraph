package lsif

import (
	"context"
	"runtime"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/conc/pool"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// migrator is a code-intelligence-specific out-of-band migration runner. This migrator can
// be configured by supplying a different driver instance, which controls the update behavior
// over every matching row in the migration set.
//
// Code intelligence tables are very large and using a full table scan count is too expensvie
// to use in an out-of-band migration. For each table we need to perform a migration over, we
// introduce a second aggregate table that tracks the minimum and maximum schema version of
// each data record  associated with a particular upload record.
//
// We have the following assumptions about the schema (for a configured table T):
//
//  1. There is an index on T.dump_id
//
//  2. For each distinct dump_id in table T, there is a corresponding row in table
//     T_schema_version. This invariant is kept up to date via triggers on insert.
//
//  3. Table T_schema_version has the following schema:
//
//     CREATE TABLE T_schema_versions (
//     dump_id            integer PRIMARY KEY NOT NULL,
//     min_schema_version integer,
//     max_schema_version integer
//     );
//
// When selecting a set of candidate records to migrate, we first use the each upload record's
// schema version bounds to determine if there are still records associated with that upload
// that may still need migrating. This set allows us to use the dump_id index on the target
// table. These counts can be maintained efficiently within the same transaction as a batch
// of migration updates. This requires counting within a small indexed subset of the original
// table. When checking progress, we can efficiently do a full-table on the much smaller
// aggregate table.
type migrator struct {
	store                    *basestore.Store
	driver                   migrationDriver
	options                  migratorOptions
	selectionExpressions     []*sqlf.Query // expressions used in select query
	temporaryTableFieldNames []string      // names of fields inserted into temporary table
	temporaryTableFieldSpecs []*sqlf.Query // names of fields inserted into temporary table
	updateConditions         []*sqlf.Query // expressions used for the update statement
	updateAssignments        []*sqlf.Query // expressions used to assign to the target table
}

type migratorOptions struct {
	// tableName is the name of the table undergoing migration.
	tableName string

	// targetVersion is the value of the row's schema version after an up migration.
	targetVersion int

	// batchSize limits the number of rows that will be scanned on each call to Up/Down.
	batchSize int

	// numRoutines is the maximum number of routines that can run at once on invocation of the
	// migrator's Up or Down methods. If zero, a number of routines equal to the number of available
	// CPUs will be used.
	numRoutines int

	// fields is an ordered set of fields used to construct temporary tables and update queries.
	fields []fieldSpec
}

type fieldSpec struct {
	// name is the name of the column.
	name string

	// postgresType is the type (with modifiers) of the column.
	postgresType string

	// primaryKey indicates that the field is part of a composite primary key.
	primaryKey bool

	// readOnly indicates that the field should be skipped on updates.
	readOnly bool

	// updateOnly indicates that the field should be skipped on selects.
	updateOnly bool
}

type migrationDriver interface {
	ID() int
	Interval() time.Duration

	// MigrateRowUp determines which fields to update for the given row. The scanner will receive
	// the values of the primary keys plus any additional non-updateOnly fields supplied via the
	// migrator's fields option. Implementations must return the same number of values as the set
	// of primary keys plus any additional non-selectOnly fields supplied via the migrator's fields
	// option.
	MigrateRowUp(scanner dbutil.Scanner) ([]any, error)

	// MigrateRowDown undoes the migration for the given row.  The scanner will receive the values
	// of the primary keys plus any additional non-updateOnly fields supplied via the migrator's
	// fields option. Implementations must return the same number of values as the set  of primary
	// keys plus any additional non-selectOnly fields supplied via the migrator's fields option.
	MigrateRowDown(scanner dbutil.Scanner) ([]any, error)
}

// driverFunc is the type of MigrateRowUp and MigrateRowDown.
type driverFunc func(scanner dbutil.Scanner) ([]any, error)

func newMigrator(store *basestore.Store, driver migrationDriver, options migratorOptions) *migrator {
	selectionExpressions := make([]*sqlf.Query, 0, len(options.fields))
	temporaryTableFieldNames := make([]string, 0, len(options.fields))
	temporaryTableFieldSpecs := make([]*sqlf.Query, 0, len(options.fields))
	updateConditions := make([]*sqlf.Query, 0, len(options.fields))
	updateAssignments := make([]*sqlf.Query, 0, len(options.fields))

	for _, field := range options.fields {
		if field.primaryKey {
			updateConditions = append(updateConditions, sqlf.Sprintf("dest."+field.name+" = src."+field.name))
		}
		if !field.updateOnly {
			selectionExpressions = append(selectionExpressions, sqlf.Sprintf(field.name))
		}
		if !field.readOnly {
			temporaryTableFieldNames = append(temporaryTableFieldNames, field.name)
			temporaryTableFieldSpecs = append(temporaryTableFieldSpecs, sqlf.Sprintf(field.name+" "+field.postgresType))

			if !field.primaryKey {
				updateAssignments = append(updateAssignments, sqlf.Sprintf(field.name+" = src."+field.name))
			}
		}
	}

	if options.numRoutines == 0 {
		options.numRoutines = runtime.GOMAXPROCS(0)
	}

	return &migrator{
		store:                    store,
		driver:                   driver,
		options:                  options,
		selectionExpressions:     selectionExpressions,
		temporaryTableFieldNames: temporaryTableFieldNames,
		temporaryTableFieldSpecs: temporaryTableFieldSpecs,
		updateConditions:         updateConditions,
		updateAssignments:        updateAssignments,
	}
}

func (m *migrator) ID() int                 { return m.driver.ID() }
func (m *migrator) Interval() time.Duration { return m.driver.Interval() }

// Progress returns the ratio between the number of upload records that have been completely
// migrated over the total number of upload records. A record is migrated if its schema version
// is no less than (on upgrades) or no greater than (on downgrades) than the target migration
// version.
func (m *migrator) Progress(ctx context.Context, applyReverse bool) (float64, error) {
	table := "min_schema_version"
	if applyReverse {
		table = "max_schema_version"
	}

	progress, _, err := basestore.ScanFirstFloat(m.store.Query(ctx, sqlf.Sprintf(
		migratorProgressQuery,
		sqlf.Sprintf(m.options.tableName),
		sqlf.Sprintf(table),
		m.options.targetVersion,
		sqlf.Sprintf(m.options.tableName),
	)))
	if err != nil {
		return 0, err
	}

	return progress, nil
}

const migratorProgressQuery = `
SELECT CASE c2.count WHEN 0 THEN 1 ELSE cast(c1.count as float) / cast(c2.count as float) END FROM
	(SELECT COUNT(*) as count FROM %s_schema_versions WHERE %s >= %s) c1,
	(SELECT COUNT(*) as count FROM %s_schema_versions) c2
`

// Up runs a batch of the migration.
//
// Each invocation of the internal method `up` (and symmetrically, `down`) selects an upload identifier
// that still has data in the target range. Records associated with this upload identifier are read and
// transformed, then updated in-place in the database.
//
// Two migrators (of the same concrete type) will not process the same upload identifier concurrently as
// the selection of the upload holds a row lock associated with that upload for the duration of the method's
// enclosing transaction.
func (m *migrator) Up(ctx context.Context) (err error) {
	p := pool.New().WithErrors()
	for range m.options.numRoutines {
		p.Go(func() error { return m.up(ctx) })
	}

	return p.Wait()
}

func (m *migrator) up(ctx context.Context) (err error) {
	return m.run(ctx, m.options.targetVersion-1, m.options.targetVersion, m.driver.MigrateRowUp)
}

// Down runs a batch of the migration in reverse.
//
// For notes on parallelism, see the symmetric `Up` method on this migrator.
func (m *migrator) Down(ctx context.Context) error {
	p := pool.New().WithErrors()
	for range m.options.numRoutines {
		p.Go(func() error { return m.down(ctx) })
	}

	return p.Wait()
}

func (m *migrator) down(ctx context.Context) error {
	return m.run(ctx, m.options.targetVersion, m.options.targetVersion-1, m.driver.MigrateRowDown)
}

// run performs a batch of updates with the given driver function. Records with the given source version
// will be selected for candidacy, and their version will match the given target version after an update.
func (m *migrator) run(ctx context.Context, sourceVersion, targetVersion int, driverFunc driverFunc) (err error) {
	tx, err := m.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	uploadID, ok, err := m.selectAndLockDump(ctx, tx, sourceVersion)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	rowValues, err := m.processRows(ctx, tx, uploadID, sourceVersion, driverFunc)
	if err != nil {
		return err
	}

	if err := m.updateBatch(ctx, tx, uploadID, targetVersion, rowValues); err != nil {
		return err
	}

	// After selecting a dump for migration, update the schema version bounds for that
	// dump. We do this regardless if we actually migrated any rows to catch the case
	// where we would select a missing dump infinitely.

	rows, err := tx.Query(ctx, sqlf.Sprintf(
		runUpdateBoundsQuery,
		uploadID,
		sqlf.Sprintf(m.options.tableName),
		uploadID,
		sqlf.Sprintf(m.options.tableName),
		sqlf.Sprintf(m.options.tableName),
		uploadID,
	))
	if err != nil {
		return err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	if rows.Next() {
		var rowsUpserted, rowsDeleted int
		if err := rows.Scan(&rowsUpserted, &rowsDeleted); err != nil {
			return err
		}

		// do nothing with these values for now
	}

	return nil
}

const runUpdateBoundsQuery = `
WITH
	current_bounds AS (
		-- Find the current bounds by scanning the data rows for the
		-- dump id and tracking the min and max. Note that these values
		-- will be null if there are no data rows.

		SELECT
			%s::integer AS dump_id,
			MIN(schema_version) as min_schema_version,
			MAX(schema_version) as max_schema_version
		FROM %s
		WHERE dump_id = %s
	),
	ups AS (
		-- Upsert the current bounds into the schema versions table. If
		-- the row already exists, we forcibly update the values as we
		-- just calculated the most recent view of row versions.

		INSERT INTO %s_schema_versions
		SELECT dump_id, min_schema_version, max_schema_version
		FROM current_bounds
		WHERE
			-- Do not insert or update if there are no data rows
			min_schema_version IS NOT NULL AND
			min_schema_version IS NOT NULL
		ON CONFLICT (dump_id) DO UPDATE SET
			min_schema_version = EXCLUDED.min_schema_version,
			max_schema_version = EXCLUDED.max_schema_version
		RETURNING 1
	),
	del AS (
		-- If there were no data rows associated with this dump, then
		-- there are no bounds (by definition) and we should remove the
		-- schema version row so we don't re-select it for migration.

		DELETE FROM %s_schema_versions
		WHERE dump_id = %s AND EXISTS (
			SELECT 1
			FROM current_bounds
			WHERE
				min_schema_version IS NULL AND
				max_schema_version IS NULL
			)
		RETURNING 1
	)
SELECT
	(SELECT COUNT(*) FROM ups) AS num_ups,
	(SELECT COUNT(*) FROM del) AS num_del
`

// selectAndLockDump chooses and row-locks a schema version row associated with a particular dump.
// Having each batch of updates touch only rows associated with a single dump reduces contention
// when multiple migrators are running. This method returns the dump identifier and a boolean flag
// indicating that such a dump could be selected.
func (m *migrator) selectAndLockDump(ctx context.Context, tx *basestore.Store, sourceVersion int) (_ int, _ bool, err error) {
	return basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(
		selectAndLockDumpQuery,
		sqlf.Sprintf(m.options.tableName),
		sourceVersion,
		sourceVersion,
	)))
}

const selectAndLockDumpQuery = `
SELECT dump_id
FROM %s_schema_versions
WHERE
	min_schema_version <= %s AND
	max_schema_version >= %s
ORDER BY dump_id
LIMIT 1

-- Lock the record in the schema_versions table. All concurrent migrators should then
-- be able to process records related to a distinct dump.
FOR UPDATE SKIP LOCKED
`

// processRows selects a batch of rows from the target table associated with the given dump identifier
// to  update and calls the given driver func over each row. The driver func returns the set of values
// that should be used to update that row. These values are fed into a channel usable for batch insert.
func (m *migrator) processRows(ctx context.Context, tx *basestore.Store, uploadID, version int, driverFunc driverFunc) (_ <-chan []any, err error) {
	rows, err := tx.Query(ctx, sqlf.Sprintf(
		processRowsQuery,
		sqlf.Join(m.selectionExpressions, ", "),
		sqlf.Sprintf(m.options.tableName),
		uploadID,
		version,
		m.options.batchSize,
	))
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	rowValues := make(chan []any, m.options.batchSize)
	defer close(rowValues)

	for rows.Next() {
		values, err := driverFunc(rows)
		if err != nil {
			return nil, err
		}

		rowValues <- values
	}

	return rowValues, nil
}

const processRowsQuery = `
SELECT %s FROM %s WHERE dump_id = %s AND schema_version = %s LIMIT %s
`

var (
	temporaryTableName       = "t_migration_payload"
	temporaryTableExpression = sqlf.Sprintf(temporaryTableName)
)

// updateBatch creates a temporary table symmetric to the target table but without any of the read-only
// fields. Then, the given row values are bulk inserted into the temporary table. Finally, the rows in
// the temporary table are used to update the target table.
func (m *migrator) updateBatch(ctx context.Context, tx *basestore.Store, uploadID, targetVersion int, rowValues <-chan []any) error {
	if err := tx.Exec(ctx, sqlf.Sprintf(
		updateBatchTemporaryTableQuery,
		temporaryTableExpression,
		sqlf.Join(m.temporaryTableFieldSpecs, ", "),
	)); err != nil {
		return err
	}

	if err := batch.InsertValues(
		ctx,
		tx.Handle(),
		temporaryTableName,
		batch.MaxNumPostgresParameters,
		m.temporaryTableFieldNames,
		rowValues,
	); err != nil {
		return err
	}

	// Note that we assign a parameterized dump identifier and schema version here since
	// both values are the same for all rows in this operation.
	if err := tx.Exec(ctx, sqlf.Sprintf(
		updateBatchUpdateQuery,
		sqlf.Sprintf(m.options.tableName),
		sqlf.Join(m.updateAssignments, ", "),
		targetVersion,
		temporaryTableExpression,
		uploadID,
		sqlf.Join(m.updateConditions, " AND "),
	)); err != nil {
		return err
	}

	return nil
}

const updateBatchTemporaryTableQuery = `
CREATE TEMPORARY TABLE %s (%s) ON COMMIT DROP
`

const updateBatchUpdateQuery = `
UPDATE %s dest SET %s, schema_version = %s FROM %s src WHERE dump_id = %s AND %s
`
