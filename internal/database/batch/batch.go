package batch

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	sglog "github.com/sourcegraph/log"
)

// Inserter allows for bulk updates to a single Postgres table.
type Inserter struct {
	db                   dbutil.DB
	numColumns           int
	maxNumValues         int
	values               []any
	cumulativeValueSizes []int
	queryPrefix          string
	querySuffix          string
	onConflictSuffix     string
	returningSuffix      string
	returningScanner     ReturningScanner
	operations           *operations
	commonAttrs          []attribute.KeyValue
}

type ReturningScanner func(rows dbutil.Scanner) error

// InsertValues creates a new batch inserter using the given database handle, table name, and
// column names, then reads from the given channel as if they specify values for a single row.
// The inserter will be flushed and any error that occurred during insertion or flush will be
// returned.
func InsertValues(ctx context.Context, db dbutil.DB, tableName string, maxNumParameters int, columnNames []string, values <-chan []any) error {
	return WithInserter(ctx, db, tableName, maxNumParameters, columnNames, func(inserter *Inserter) error {
	outer:
		for {
			select {
			case rowValues, ok := <-values:
				if !ok {
					break outer
				}

				if err := inserter.Insert(ctx, rowValues...); err != nil {
					return err
				}

			case <-ctx.Done():
				break outer
			}
		}

		return nil
	})
}

// WithInserter creates a new batch inserter using the given database handle, table name,
// and column names, then calls the given function with the new inserter as a parameter.
// The inserter will be flushed regardless of the error condition of the given function.
// Any error returned from the given function will be decorated with the inserter's flush
// error, if one occurs.
func WithInserter(
	ctx context.Context,
	db dbutil.DB,
	tableName string,
	maxNumParameters int,
	columnNames []string,
	f func(inserter *Inserter) error,
) (err error) {
	inserter := NewInserter(ctx, db, tableName, maxNumParameters, columnNames...)
	return with(ctx, inserter, f)
}

// WithInserterWithReturn creates a new batch inserter using the given database handle,
// table name, column names, returning columns and returning scanner, then calls the given
// function with the new inserter as a parameter. The inserter will be flushed regardless
// of the error condition of the given function. Any error returned from the given function
// will be decorated with the inserter's flush error, if one occurs.
func WithInserterWithReturn(
	ctx context.Context,
	db dbutil.DB,
	tableName string,
	maxNumParameters int,
	columnNames []string,
	onConflictClause string,
	returningColumnNames []string,
	returningScanner ReturningScanner,
	f func(inserter *Inserter) error,
) (err error) {
	inserter := NewInserterWithReturn(ctx, db, tableName, maxNumParameters, columnNames, onConflictClause, returningColumnNames, returningScanner)
	return with(ctx, inserter, f)
}

// WithInserterForIdentifiers creates a new batch inserter using the given database handle, table name,
// column names, and calls the given function with the new inserter as a parameter. The single returning
// column name will be scanned as an integer and collected. The sequence of collected identifiers are
// returned from this function. The inserter will be flushed regardless of the error condition of the given
// function. Any error returned from the given function will be decorated with the inserter's flush error,
// if one occurs.
func WithInserterForIdentifiers(
	ctx context.Context,
	db dbutil.DB,
	tableName string,
	maxNumParameters int,
	columnNames []string,
	onConflictClause string,
	returningColumnName string,
	f func(inserter *Inserter) error,
) (ids []int, err error) {
	inserter := NewInserterWithReturn(ctx, db, tableName, maxNumParameters, columnNames, onConflictClause, []string{returningColumnName}, func(s dbutil.Scanner) error {
		id, err := basestore.ScanInt(s)
		if err != nil {
			return err
		}

		ids = append(ids, id)
		return nil
	})
	if err := with(ctx, inserter, f); err != nil {
		return nil, err
	}

	return ids, nil
}

func with(ctx context.Context, inserter *Inserter, f func(inserter *Inserter) error) (err error) {
	defer func() {
		if flushErr := inserter.Flush(ctx); flushErr != nil {
			err = errors.Append(err, errors.Wrap(flushErr, "inserter.Flush"))
		}
	}()

	return f(inserter)
}

// NewInserter creates a new batch inserter using the given database handle, table name,
// and column names. For performance and atomicity, handle should be a transaction.
func NewInserter(ctx context.Context, db dbutil.DB, tableName string, maxNumParameters int, columnNames ...string) *Inserter {
	return NewInserterWithReturn(ctx, db, tableName, maxNumParameters, columnNames, "", nil, nil)
}

// NewInserterWithConflict creates a new batch inserter using the given database handle, table name, column names,
// and on conflict clause. For performance and atomicity, handle should be a transaction.
func NewInserterWithConflict(ctx context.Context, db dbutil.DB, tableName string, maxNumParameters int, onConflictClause string, columnNames ...string) *Inserter {
	return NewInserterWithReturn(ctx, db, tableName, maxNumParameters, columnNames, onConflictClause, nil, nil)
}

// NewInserterWithReturn creates a new batch inserter using the given database handle, table
// name, insert column names, and column names to scan on each inserted row. The given scanner
// will be called once for each row inserted into the target table. Beware that this function
// may not be called immediately after a call to Insert as rows are only flushed once the
// current batch is full (or on explicit flush). For performance and atomicity, handle should
// be a transaction.
func NewInserterWithReturn(
	ctx context.Context,
	db dbutil.DB,
	tableName string,
	maxNumParameters int,
	columnNames []string,
	onConflictClause string,
	returningColumnNames []string,
	returningScanner ReturningScanner,
) *Inserter {
	numColumns := len(columnNames)
	maxNumValues := int(maxNumParameters/numColumns) * numColumns
	queryPrefix := makeQueryPrefix(tableName, columnNames)
	querySuffix := makeQuerySuffix(numColumns, maxNumParameters)
	onConflictSuffix := makeOnConflictSuffix(onConflictClause)
	returningSuffix := makeReturningSuffix(returningColumnNames)
	logger := sglog.Scoped("Inserter")

	return &Inserter{
		db:                   db,
		numColumns:           numColumns,
		maxNumValues:         maxNumValues,
		values:               nil,
		cumulativeValueSizes: nil,
		queryPrefix:          queryPrefix,
		querySuffix:          querySuffix,
		onConflictSuffix:     onConflictSuffix,
		returningSuffix:      returningSuffix,
		returningScanner:     returningScanner,
		operations:           getOperations(logger),
		commonAttrs: []attribute.KeyValue{
			attribute.String("tableName", tableName),
			attribute.StringSlice("columnNames", columnNames),
			attribute.Int("numColumns", numColumns),
			attribute.Int("maxNumValues", maxNumValues),
		},
	}
}

// Insert submits a single row of values to be inserted on the next flush.
func (i *Inserter) Insert(ctx context.Context, values ...any) error {
	i.checkInvariants()
	defer i.checkInvariants()

	if len(values) != i.numColumns {
		return errors.Errorf("expected %d values, got %d", i.numColumns, len(values))
	}

	currentCumulativeValueSize := 0
	if n := len(i.cumulativeValueSizes); n != 0 {
		currentCumulativeValueSize = i.cumulativeValueSizes[n-1]
	}

	valueSizes := make([]int, 0, len(values))
	for _, value := range values {
		switch v := value.(type) {
		case string:
			currentCumulativeValueSize += len(v)
		default:
			currentCumulativeValueSize += 1
		}

		valueSizes = append(valueSizes, currentCumulativeValueSize)
	}

	i.values = append(i.values, values...)
	i.cumulativeValueSizes = append(i.cumulativeValueSizes, valueSizes...)

	if len(i.values) >= i.maxNumValues {
		// Flush full batch
		return i.Flush(ctx)
	}

	return nil
}

// Flush ensures that all queued rows are inserted. This method must be invoked at the end
// of insertion to ensure that all records are flushed to the underlying db connection.
func (i *Inserter) Flush(ctx context.Context) (err error) {
	i.checkInvariants()
	defer i.checkInvariants()

	batch, payloadSize := i.pop()
	if len(batch) == 0 {
		return nil
	}

	operationAttrs := []attribute.KeyValue{
		attribute.Int("batchSize", len(batch)),
		attribute.Int("payloadSize", payloadSize),
	}
	combinedAttrs := append(operationAttrs, i.commonAttrs...)
	ctx, _, endObservation := i.operations.flush.With(ctx, &err, observation.Args{Attrs: combinedAttrs})
	defer endObservation(1, observation.Args{})

	// Create a query with enough placeholders to match the current batch size. This should
	// generally be the full querySuffix string, except for the last call to Flush which
	// may be a partial batch.
	rows, err := i.db.QueryContext(dbconn.WithBulkInsertion(ctx, true), i.makeQuery(len(batch)), batch...)
	if err != nil {
		return err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	for rows.Next() {
		if err := i.returningScanner(rows); err != nil {
			return err
		}
	}

	return nil
}

// checkBatchInserterInvariants is set to true in tests to enable invariant detection
// at the start and end of public methods. This ensures that the batch and payload size
// lists remain equivalent size whenever the caller can initiate an operation.
var checkBatchInserterInvariants = false

func (i *Inserter) checkInvariants() {
	if checkBatchInserterInvariants && len(i.values) != len(i.cumulativeValueSizes) {
		panic(fmt.Sprintf("broken invariant: len(i.batch) != len(i.cumulativeValueSizes): %d != %d", len(i.values), len(i.cumulativeValueSizes)))
	}
}

// pop removes and returns as many values from the current batch that can be attached to a single
// insert statement. The returned values are the oldest values submitted to the batch (in order).
// This method additionally returns the total (approximate) size of the batch being inserted.
func (i *Inserter) pop() (batch []any, payloadSize int) {
	if len(i.values) == 0 {
		return nil, 0
	}

	if len(i.values) < i.maxNumValues {
		// Grab size before overwriting it
		payloadSize = i.cumulativeValueSizes[len(i.cumulativeValueSizes)-1]

		// Use entire batch. This allows us to cleanly reset the sizes we were tracking for value
		// payloads by just cutting the length of the slice back to zero.
		batch, i.values = i.values, i.values[:0]
		i.cumulativeValueSizes = i.cumulativeValueSizes[:0]
		return batch, payloadSize
	}

	// Grab size before altering containing slice
	payloadSize = i.cumulativeValueSizes[i.maxNumValues-1]

	// Extract partial batch along with the size tracking data for each element
	batch, i.values = i.values[:i.maxNumValues], i.values[i.maxNumValues:]
	i.cumulativeValueSizes = i.cumulativeValueSizes[i.maxNumValues:]

	for idx := range i.cumulativeValueSizes {
		// Remove the size of the batch we've just extracted from every value remaining in the slice.
		// This should generally only be a handful of elements and shouldn't be anywhere near a dominating
		// loop.
		i.cumulativeValueSizes[idx] -= payloadSize
	}

	return batch, payloadSize
}

// makeQuery returns a parameterized SQL query that has the given number of values worth of
// placeholder variables. It is assumed that the number of values is non-zero and also is a
// multiple of the number of columns of the target table.
func (i *Inserter) makeQuery(numValues int) string {
	// Determine how many characters a single tuple of the query suffix occupies.
	// The tuples have the form `($xxxxx,$xxxxx,...)`, and all placeholders are
	// exactly five digits for uniformity. This counts 5 digits, `$`, and `,` for
	// each value, then un-counts the trailing comma, then counts the enveloping
	// `(` and `)`.
	sizeOfTuple := 7*i.numColumns - 1 + 2

	// Determine number of tuples being flushed
	numTuples := numValues / i.numColumns

	// Count commas separating tuples, then un-count the trailing comma
	suffixLength := numTuples*sizeOfTuple + numTuples - 1

	// Construct the query
	return i.queryPrefix + i.querySuffix[:suffixLength] + i.onConflictSuffix + i.returningSuffix
}

// MaxNumPostgresParameters is the maximum number of placeholder variables allowed by Postgres
// in a single insert statement.
const MaxNumPostgresParameters = 65535

// MaxNumSQLiteParameters is the maximum number of placeholder variables allowed by SQLite
// in a single insert statement.
const MaxNumSQLiteParameters = 999

// makeQueryPrefix creates the prefix of the batch insert statement (up to `VALUES `) using the
// given table and column names.
func makeQueryPrefix(tableName string, columnNames []string) string {
	quotedColumnNames := make([]string, 0, len(columnNames))
	for _, columnName := range columnNames {
		quotedColumnNames = append(quotedColumnNames, fmt.Sprintf(`"%s"`, columnName))
	}

	return fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES `, tableName, strings.Join(quotedColumnNames, ","))
}

var (
	querySuffixCache      = map[int]string{}
	querySuffixCacheMutex sync.Mutex
)

// makeQuerySuffix creates the suffix of the batch insert statement containing the placeholder
// variables, e.g. `($1,$2,$3),($4,$5,$6),...`. The number of rows will be the maximum number of
// _full_ rows that can be inserted in one insert statement.
//
// If a fewer number of rows should be inserted (due to flushing a partial batch), then the caller
// slice the appropriate number of rows from the beginning of the string. The suffix constructed
// here is done so with this use case in mind (each placeholder is 5 digits), so finding the right
// substring index is efficient.
//
// This method is memoized.
func makeQuerySuffix(numColumns, maxNumParameters int) string {
	querySuffixCacheMutex.Lock()
	defer querySuffixCacheMutex.Unlock()
	if cache, ok := querySuffixCache[numColumns]; ok {
		return cache
	}

	qs := []byte{
		',', // Start with trailing comma for processing uniformity
	}
	for i := range maxNumParameters {
		if i%numColumns == 0 {
			// Replace previous `,` with `),(`
			qs[len(qs)-1] = ')'
			qs = append(qs, ',', '(')
		}
		qs = append(qs, []byte(fmt.Sprintf("$%05d", i+1))...)
		qs = append(qs, ',')
	}
	// Replace trailing `,` with `)`
	qs[len(qs)-1] = ')'

	// Chop off leading `),`
	querySuffix := string(qs[2:])
	querySuffixCache[numColumns] = querySuffix
	return querySuffix
}

// makeOnConflictSuffix creates a ON CONFLICT ... clause of the batch inserter statement, if
// any on conflict command was supplied to the batch inserter.
func makeOnConflictSuffix(command string) string {
	if command == "" {
		return ""
	}

	// Command assumed to be full clause
	return fmt.Sprintf(" %s", command)
}

// makeReturningSuffix creates a RETURNING ... clause of the batch insert statement, if any
// returning column names were supplied to the batch inserter.
func makeReturningSuffix(columnNames []string) string {
	if len(columnNames) == 0 {
		return ""
	}

	return fmt.Sprintf(" RETURNING %s", strings.Join(columnNames, ", "))
}
