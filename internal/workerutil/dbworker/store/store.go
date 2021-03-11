package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/opentracing/opentracing-go/log"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
)

// Store is the persistence layer for the dbworker package that handles worker-side operations backed by a Postgres
// database. See Options for details on the required shape of the database tables (e.g. table column names/types).
type Store interface {
	basestore.ShareableStore

	// Done performs a commit or rollback of the underlying transaction/savepoint depending
	// returned from the Dequeue method. See basestore.Store#Done for additional documentation.
	Done(err error) error

	// QueuedCount returns the number of records in the queued state matching the given conditions.
	QueuedCount(ctx context.Context, conditions []*sqlf.Query) (int, error)

	// Dequeue selects the first unlocked record matching the given conditions and locks it in a new transaction that
	// should be held by the worker process. If there is such a record, it is returned along with a new store instance
	// that wraps the transaction. The resulting transaction must be closed by the caller, and the transaction should
	// include a state transition of the record into a terminal state. If there is no such unlocked record, a nil record
	// and a nil store will be returned along with a false-valued flag. This method must not be called from within a
	// transaction.
	//
	// The supplied conditions may use the alias provided in `ViewName`, if one was supplied.
	Dequeue(ctx context.Context, conditions []*sqlf.Query) (record workerutil.Record, tx Store, exists bool, err error)

	// DequeueWithIndependentTransactionContext is like Dequeue, but will use a context.Background() for the underlying
	// transaction context. This method allows the transaction to lexically outlive the code in which it was created. This
	// is useful if a longer-running transaction is managed explicitly between multiple goroutines.
	DequeueWithIndependentTransactionContext(ctx context.Context, conditions []*sqlf.Query) (workerutil.Record, Store, bool, error)

	// Requeue updates the state of the record with the given identifier to queued and adds a processing delay before
	// the next dequeue of this record can be performed.
	Requeue(ctx context.Context, id int, after time.Time) error

	// AddExecutionLogEntry adds an executor log entry to the record.
	AddExecutionLogEntry(ctx context.Context, id int, entry workerutil.ExecutionLogEntry) error

	// MarkComplete attempts to update the state of the record to complete. If this record has already been moved from
	// the processing state to a terminal state, this method will have no effect. This method returns a boolean flag
	// indicating if the record was updated.
	MarkComplete(ctx context.Context, id int) (bool, error)

	// MarkErrored attempts to update the state of the record to errored. This method will only have an effect
	// if the current state of the record is processing or completed. A requeued record or a record already marked
	// with an error will not be updated. This method returns a boolean flag indicating if the record was updated.
	MarkErrored(ctx context.Context, id int, failureMessage string) (bool, error)

	// MarkFailed attempts to update the state of the record to failed. This method will only have an effect
	// if the current state of the record is processing or completed. A requeued record or a record already marked
	// with an error will not be updated. This method returns a boolean flag indicating if the record was updated.
	MarkFailed(ctx context.Context, id int, failureMessage string) (bool, error)

	// ResetStalled moves all unlocked records in the processing state for more than `StalledMaxAge` back to the queued
	// state. In order to prevent input that continually crashes worker instances, records that have been reset more
	// than `MaxNumResets` times will be marked as errored. This method returns a list of record identifiers that have
	// been reset and a list of record identifiers that have been marked as errored.
	ResetStalled(ctx context.Context) (resetIDs, erroredIDs []int, err error)
}

type ExecutionLogEntry workerutil.ExecutionLogEntry

func (e *ExecutionLogEntry) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("value is not []byte: %T", value)
	}

	return json.Unmarshal(b, &e)
}

func (e ExecutionLogEntry) Value() (driver.Value, error) {
	return json.Marshal(e)
}

func ExecutionLogEntries(raw []workerutil.ExecutionLogEntry) (entries []ExecutionLogEntry) {
	for _, entry := range raw {
		entries = append(entries, ExecutionLogEntry(entry))
	}

	return entries
}

type store struct {
	*basestore.Store
	options        Options
	columnReplacer *strings.Replacer
	operations     *operations
}

var _ Store = &store{}

// Options configure the behavior of Store over a particular set of tables, columns, and expressions.
type Options struct {
	// Name denotes the name of the store used to distinguish log messages and emitted metrics. The
	// store constructor will fail if this field is not supplied.
	Name string

	// TableName is the name of the table containing work records.
	//
	// The target table (and the target view referenced by `ViewName`) must have the following columns
	// and types:
	//
	//   - id: integer primary key
	//   - state: text (may be updated to `queued`, `processing`, `errored`, or `failed`)
	//   - failure_message: text
	//   - started_at: timestamp with time zone
	//   - finished_at: timestamp with time zone
	//   - process_after: timestamp with time zone
	//   - num_resets: integer not null
	//   - num_failures: integer not null
	//   - execution_logs: json[] (each entry has the form of `ExecutionLogEntry`)
	//
	// The names of these columns may be customized based on the table name by adding a replacement
	// pair in the AlternateColumnNames mapping.
	//
	// It's recommended to put an index or (or partial index) on the state column for more efficient
	// dequeue operations.
	TableName string

	// AlternateColumnNames is a map from expected column names to actual column names in the target
	// table. This allows existing tables to be more easily retrofitted into the expected record
	// shape.
	AlternateColumnNames map[string]string

	// ViewName is an optional name of a view on top of the table containing work records to query when
	// selecting a candidate and when selecting the record after it has been locked. If this value is
	// not supplied, `TableName` will be used. The value supplied may also indicate a table alias, which
	// can be referenced in `OrderByExpression`, `ColumnExpressions`, and the conditions supplied to
	// `Dequeue`.
	//
	// The target of this column must be a view on top of the configured table with the same column
	// requirements as the base table described above.
	//
	// Example use case:
	// The processor for LSIF uploads supplies `lsif_uploads_with_repository_name`, a view on top of the
	// `lsif_uploads` table that joins work records with the `repo` table and adds an additional repository
	// name column. This allows `Dequeue` to return a record with additional data so that a second query
	// is not necessary by the caller.
	ViewName string

	// Scan is the function used to convert a rows object into a record of the expected shape.
	Scan RecordScanFn

	// OrderByExpression is the SQL expression used to order candidate records when selecting the next
	// batch of work to perform. This expression may use the alias provided in `ViewName`, if one was
	// supplied.
	OrderByExpression *sqlf.Query

	// ColumnExpressions are the target columns provided to the query when selecting a locked record.
	// These expressions may use the alias provided in `ViewName`, if one was supplied.
	ColumnExpressions []*sqlf.Query

	// StalledMaxAge is the maximum allow duration between updating the state of a record as "processing"
	// and locking the record row during processing. An unlocked row that is marked as processing likely
	// indicates that the worker that dequeued the record has died. There should be a nearly-zero delay
	// between these states during normal operation.
	StalledMaxAge time.Duration

	// MaxNumResets is the maximum number of times a record can be implicitly reset back to the queued
	// state (via `ResetStalled`). If a record's reset attempts counter reaches this threshold, it will
	// be moved into the errored state rather than queued on its next reset to prevent an infinite retry
	// cycle of the same input.
	MaxNumResets int

	// RetryAfter determines whether the store dequeues jobs that have errored more than RetryAfter ago.
	// Setting this value to zero will disable retries entirely.
	//
	// If RetryAfter is a non-zero duration, the store dequeues records where:
	//
	//   - the state is 'errored'
	//   - the failed attempts counter hasn't reached MaxNumRetries
	//   - the finished_at timestamp was more than RetryAfter ago
	RetryAfter time.Duration

	// MaxNumRetries is the maximum number of times a record can be retried after an explicit failure.
	// Setting this value to zero will disable retries entirely.
	MaxNumRetries int
}

// RecordScanFn is a function that interprets row values as a particular record. This function should
// return a false-valued flag if the given result set was empty. This function must close the rows
// value if the given error value is nil.
//
// See the `CloseRows` function in the store/base package for suggested implementation details.
type RecordScanFn func(rows *sql.Rows, err error) (workerutil.Record, bool, error)

// New creates a new store with the given database handle and options.
func New(handle *basestore.TransactableHandle, options Options) Store {
	return NewWithMetrics(handle, options, &observation.TestContext)
}

func NewWithMetrics(handle *basestore.TransactableHandle, options Options, observationContext *observation.Context) Store {
	return newStore(handle, options, observationContext)
}

func newStore(handle *basestore.TransactableHandle, options Options, observationContext *observation.Context) *store {
	if options.Name == "" {
		panic("no name supplied to github.com/sourcegraph/sourcegraph/internal/dbworker/store:newStore")
	}

	if options.ViewName == "" {
		options.ViewName = options.TableName
	}

	alternateColumnNames := map[string]string{}
	for _, name := range columnNames {
		alternateColumnNames[name] = name
	}
	for k, v := range options.AlternateColumnNames {
		alternateColumnNames[k] = v
	}

	var replacements []string
	for k, v := range alternateColumnNames {
		replacements = append(replacements, fmt.Sprintf("{%s}", k), v)
	}

	return &store{
		Store:          basestore.NewWithHandle(handle),
		options:        options,
		columnReplacer: strings.NewReplacer(replacements...),
		operations:     newOperations(options.Name, observationContext),
	}
}

// ColumnNames are the names of the columns expected to be defined by the target table.
var columnNames = []string{
	"id",
	"state",
	"failure_message",
	"started_at",
	"finished_at",
	"process_after",
	"num_resets",
	"num_failures",
	"execution_logs",
}

// DefaultColumnExpressions returns a slice of expressions for the default column name we expect.
func DefaultColumnExpressions() []*sqlf.Query {
	expressions := make([]*sqlf.Query, len(columnNames))
	for i := range columnNames {
		expressions[i] = sqlf.Sprintf(columnNames[i])
	}
	return expressions
}

func (s *store) Transact(ctx context.Context) (*store, error) {
	txBase, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &store{
		Store:          txBase,
		options:        s.options,
		columnReplacer: s.columnReplacer,
		operations:     s.operations,
	}, nil
}

// QueuedCount returns the number of records in the queued state matching the given conditions.
func (s *store) QueuedCount(ctx context.Context, conditions []*sqlf.Query) (_ int, err error) {
	ctx, endObservation := s.operations.queuedCount.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	count, _, err := basestore.ScanFirstInt(s.Query(ctx, s.formatQuery(
		queuedCountQuery,
		quote(s.options.ViewName),
		s.options.MaxNumRetries,
		makeConditionSuffix(conditions),
	)))

	return count, err
}

const queuedCountQuery = `
-- source: internal/workerutil/store.go:QueuedCount
SELECT COUNT(*) FROM %s WHERE (
	{state} = 'queued' OR
	({state} = 'errored' AND {num_failures} < %s)
) %s
`

// Dequeue selects the first unlocked record matching the given conditions and locks it in a new transaction that
// should be held by the worker process. If there is such a record, it is returned along with a new store instance
// that wraps the transaction. The resulting transaction must be closed by the caller, and the transaction should
// include a state transition of the record into a terminal state. If there is no such unlocked record, a nil record
// and a nil store will be returned along with a false-valued flag. This method must not be called from within a
// transaction.
//
// The supplied conditions may use the alias provided in `ViewName`, if one was supplied.
func (s *store) Dequeue(ctx context.Context, conditions []*sqlf.Query) (record workerutil.Record, _ Store, exists bool, err error) {
	return s.dequeue(ctx, conditions, false)
}

// DequeueWithIndependentTransactionContext is like Dequeue, but will use a context.Background() for the underlying
// transaction context. This method allows the transaction to lexically outlive the code in which it was created. This
// is useful if a longer-running transaction is managed explicitly between multiple goroutines.
func (s *store) DequeueWithIndependentTransactionContext(ctx context.Context, conditions []*sqlf.Query) (workerutil.Record, Store, bool, error) {
	return s.dequeue(ctx, conditions, true)
}

func (s *store) dequeue(ctx context.Context, conditions []*sqlf.Query, independentTxCtx bool) (record workerutil.Record, _ Store, exists bool, err error) {
	ctx, traceLog, endObservation := s.operations.dequeue.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if s.InTransaction() {
		return nil, nil, false, ErrDequeueTransaction
	}

	txCtx := ctx
	if independentTxCtx {
		txCtx = context.Background()
	}

	query := s.formatQuery(
		selectCandidateQuery,
		quote(s.options.ViewName),
		int(s.options.RetryAfter/time.Second),
		int(s.options.RetryAfter/time.Second),
		s.options.MaxNumRetries,
		makeConditionSuffix(conditions),
		s.options.OrderByExpression,
		quote(s.options.TableName),
	)

	for {
		// First, we try to select an eligible record outside of a transaction. This will skip
		// any rows that are currently locked inside of a transaction of another dequeue process.
		id, ok, err := basestore.ScanFirstInt(s.Query(ctx, query))
		if err != nil {
			return nil, nil, false, err
		}
		if !ok {
			return nil, nil, false, nil
		}
		traceLog(log.Int("id", id))

		// Once we have an eligible identifier, we try to create a transaction and select the
		// record in a way that takes a row lock for the duration of the transaction.
		tx, err := s.Transact(txCtx)
		if err != nil {
			return nil, nil, false, err
		}

		// Select the candidate record within the transaction to lock it from other processes. Note
		// that SKIP LOCKED here is necessary, otherwise this query would block on race conditions
		// until the other process has finished with the record.
		_, exists, err = basestore.ScanFirstInt(tx.Query(ctx, s.formatQuery(
			lockQuery,
			quote(s.options.TableName),
			id,
		)))
		if err != nil {
			return nil, nil, false, tx.Done(err)
		}
		if !exists {
			// Due to SKIP LOCKED, This query will return a sql.ErrNoRows error if the record has
			// already been locked in another process's transaction. We'll return a special error
			// that is checked by the caller to try to select a different record.
			if err := tx.Done(ErrDequeueRace); err != ErrDequeueRace {
				return nil, nil, false, err
			}

			// This will occur if we selected a candidate record that raced with another dequeue
			// process. If both dequeue processes select the same record and the other process
			// begins its transaction first, this condition will occur. We'll re-try the process
			// by selecting another identifier - this one will be skipped on a second attempt as
			// it is now locked.
			continue
		}

		// The record is now locked in this transaction. As `TableName` and `ViewName` may have distinct
		// values, we need to perform a second select in order to pass the correct data to the scan
		// function.
		record, exists, err = s.options.Scan(tx.Query(ctx, s.formatQuery(
			selectRecordQuery,
			sqlf.Join(s.options.ColumnExpressions, ", "),
			quote(s.options.ViewName),
			id,
		)))
		if err != nil {
			return nil, nil, false, tx.Done(err)
		}
		if !exists {
			// This only happens on a programming error (mismatch between `TableName` and `ViewName`).
			return nil, nil, false, tx.Done(ErrNoRecord)
		}

		return record, tx, true, nil
	}
}

const selectCandidateQuery = `
-- source: internal/workerutil/store.go:Dequeue
WITH candidate AS (
	SELECT {id} FROM %s
	WHERE
		(
			(
				{state} = 'queued' AND
				({process_after} IS NULL OR {process_after} <= NOW())
			) OR (
				%s > 0 AND
				{state} = 'errored' AND
				NOW() - {finished_at} > (%s * '1 second'::interval) AND
				{num_failures} < %s
			)
		)
		%s
	ORDER BY %s
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE %s
SET
	{state} = 'processing',
	{started_at} = NOW(),
	{finished_at} = NULL,
	{failure_message} = NULL
WHERE {id} IN (SELECT {id} FROM candidate)
RETURNING {id}
`

const lockQuery = `
-- source: internal/workerutil/store.go:Dequeue
SELECT 1 FROM %s
WHERE {id} = %s
FOR UPDATE SKIP LOCKED
LIMIT 1
`

const selectRecordQuery = `
-- source: internal/workerutil/store.go:Dequeue
SELECT %s FROM %s
WHERE {id} = %s
LIMIT 1
`

// Requeue updates the state of the record with the given identifier to queued and adds a processing delay before
// the next dequeue of this record can be performed.
func (s *store) Requeue(ctx context.Context, id int, after time.Time) (err error) {
	ctx, endObservation := s.operations.requeue.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("id", id),
		log.String("after", after.String()),
	}})
	defer endObservation(1, observation.Args{})

	return s.Exec(ctx, s.formatQuery(
		requeueQuery,
		quote(s.options.TableName),
		after,
		id,
	))
}

const requeueQuery = `
-- source: internal/workerutil/store.go:Requeue
UPDATE %s
SET {state} = 'queued', {process_after} = %s
WHERE {id} = %s
`

// AddExecutionLogEntry adds an executor log entry to the record.
func (s *store) AddExecutionLogEntry(ctx context.Context, id int, entry workerutil.ExecutionLogEntry) (err error) {
	ctx, endObservation := s.operations.addExecutionLogEntry.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	return s.Exec(ctx, s.formatQuery(
		addExecutionLogEntryQuery,
		quote(s.options.TableName),
		ExecutionLogEntry(entry),
		id,
	))
}

const addExecutionLogEntryQuery = `
-- source: internal/workerutil/store.go:AddExecutionLogEntry
UPDATE %s
SET {execution_logs} = {execution_logs} || %s::json
WHERE {id} = %s
`

// MarkComplete attempts to update the state of the record to complete. If this record has already been moved from
// the processing state to a terminal state, this method will have no effect. This method returns a boolean flag
// indicating if the record was updated.
func (s *store) MarkComplete(ctx context.Context, id int) (_ bool, err error) {
	ctx, endObservation := s.operations.markComplete.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	_, ok, err := basestore.ScanFirstInt(s.Query(ctx, s.formatQuery(markCompleteQuery, quote(s.options.TableName), id)))
	return ok, err
}

const markCompleteQuery = `
-- source: internal/workerutil/store.go:MarkComplete
UPDATE %s
SET {state} = 'completed', {finished_at} = clock_timestamp()
WHERE {id} = %s AND {state} = 'processing'
RETURNING {id}
`

// MarkErrored attempts to update the state of the record to errored. This method will only have an effect
// if the current state of the record is processing or completed. A requeued record or a record already marked
// with an error will not be updated. This method returns a boolean flag indicating if the record was updated.
func (s *store) MarkErrored(ctx context.Context, id int, failureMessage string) (_ bool, err error) {
	ctx, endObservation := s.operations.markErrored.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	q := s.formatQuery(markErroredQuery, quote(s.options.TableName), s.options.MaxNumRetries, failureMessage, id)
	_, ok, err := basestore.ScanFirstInt(s.Query(ctx, q))
	return ok, err
}

const markErroredQuery = `
-- source: internal/workerutil/store.go:MarkErrored
UPDATE %s
SET {state} = CASE WHEN {num_failures} + 1 = %d THEN 'failed' ELSE 'errored' END,
	{finished_at} = clock_timestamp(),
	{failure_message} = %s,
	{num_failures} = {num_failures} + 1
WHERE {id} = %s AND ({state} = 'processing' OR {state} = 'completed')
RETURNING {id}
`

// MarkFailed attempts to update the state of the record to failed. This method will only have an effect
// if the current state of the record is processing or completed. A requeued record or a record already marked
// with an error will not be updated. This method returns a boolean flag indicating if the record was updated.
func (s *store) MarkFailed(ctx context.Context, id int, failureMessage string) (_ bool, err error) {
	ctx, endObservation := s.operations.markFailed.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("id", id),
	}})
	defer endObservation(1, observation.Args{})

	q := s.formatQuery(markFailedQuery, quote(s.options.TableName), failureMessage, id)
	_, ok, err := basestore.ScanFirstInt(s.Query(ctx, q))
	return ok, err
}

const markFailedQuery = `
-- source: internal/workerutil/store.go:MarkFailed
UPDATE %s
SET {state} = 'failed',
	{finished_at} = clock_timestamp(),
	{failure_message} = %s,
	{num_failures} = {num_failures} + 1
WHERE {id} = %s AND ({state} = 'processing' OR {state} = 'completed')
RETURNING {id}
`

// ResetStalled moves all unlocked records in the processing state for more than `StalledMaxAge` back to the queued
// state. In order to prevent input that continually crashes worker instances, records that have been reset more
// than `MaxNumResets` times will be marked as errored. This method returns a list of record identifiers that have
// been reset and a list of record identifiers that have been marked as errored.
func (s *store) ResetStalled(ctx context.Context) (resetIDs, erroredIDs []int, err error) {
	ctx, traceLog, endObservation := s.operations.resetStalled.WithAndLogger(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	resetIDs, err = s.resetStalled(ctx, resetStalledQuery)
	if err != nil {
		return resetIDs, erroredIDs, err
	}
	traceLog(log.Int("numResetIDs", len(resetIDs)))

	erroredIDs, err = s.resetStalled(ctx, resetStalledMaxResetsQuery)
	if err != nil {
		return resetIDs, erroredIDs, err
	}
	traceLog(log.Int("numErroredIDs", len(erroredIDs)))

	return resetIDs, erroredIDs, nil
}

func (s *store) resetStalled(ctx context.Context, q string) ([]int, error) {
	return basestore.ScanInts(s.Query(
		ctx,
		s.formatQuery(
			q,
			quote(s.options.TableName),
			int(s.options.StalledMaxAge/time.Second),
			s.options.MaxNumResets,
			quote(s.options.TableName),
		),
	))
}

const resetStalledQuery = `
-- source: internal/workerutil/store.go:ResetStalled
WITH stalled AS (
	SELECT {id} FROM %s
	WHERE
		{state} = 'processing' AND
		NOW() - {started_at} > (%s * '1 second'::interval) AND
		{num_resets} < %s
	FOR UPDATE SKIP LOCKED
)
UPDATE %s
SET
	{state} = 'queued',
	{started_at} = null,
	{num_resets} = {num_resets} + 1
WHERE {id} IN (SELECT {id} FROM stalled)
RETURNING {id}
`

const resetStalledMaxResetsQuery = `
-- source: internal/workerutil/store.go:ResetStalled
WITH stalled AS (
	SELECT {id} FROM %s
	WHERE
		{state} = 'processing' AND
		NOW() - {started_at} > (%s * '1 second'::interval) AND
		{num_resets} >= %s
	FOR UPDATE SKIP LOCKED
)
UPDATE %s
SET
	{state} = 'errored',
	{finished_at} = clock_timestamp(),
	{failure_message} = 'failed to process'
WHERE {id} IN (SELECT {id} FROM stalled)
RETURNING {id}
`

func (s *store) formatQuery(query string, args ...interface{}) *sqlf.Query {
	return sqlf.Sprintf(s.columnReplacer.Replace(query), args...)
}

// quote wraps the given string in a *sqlf.Query so that it is not passed to the database
// as a parameter. It is necessary to quote things such as table names, columns, and other
// expressions that are not simple values.
func quote(s string) *sqlf.Query {
	return sqlf.Sprintf(s)
}

// makeConditionSuffix returns a *sqlf.Query containing "AND {c1 AND c2 AND ...}" when the
// given set of conditions is non-empty, and an empty string otherwise.
func makeConditionSuffix(conditions []*sqlf.Query) *sqlf.Query {
	if len(conditions) == 0 {
		return sqlf.Sprintf("")
	}

	var quotedConditions []*sqlf.Query
	for _, condition := range conditions {
		// Ensure everything is quoted in case the condition has an OR
		quotedConditions = append(quotedConditions, sqlf.Sprintf("(%s)", condition))
	}

	return sqlf.Sprintf("AND %s", sqlf.Join(quotedConditions, " AND "))
}
