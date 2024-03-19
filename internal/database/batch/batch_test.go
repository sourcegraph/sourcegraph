package batch

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	checkBatchInserterInvariants = true
}

func TestBatchInserter(t *testing.T) {
	db := dbtest.NewDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	expectedValues := makeTestValues(tableSizeFactor, 0)
	testInsert(t, db, expectedValues)

	rows, err := db.Query("SELECT col1, col2, col3, col4, col5 from batch_inserter_test")
	if err != nil {
		t.Fatalf("unexpected error querying data: %s", err)
	}
	defer rows.Close()

	var values [][]any
	for rows.Next() {
		var v1, v2, v3, v4 int
		var v5 string
		if err := rows.Scan(&v1, &v2, &v3, &v4, &v5); err != nil {
			t.Fatalf("unexpected error scanning data: %s", err)
		}

		values = append(values, []any{v1, v2, v3, v4, v5})
	}

	if diff := cmp.Diff(expectedValues, values); diff != "" {
		t.Errorf("unexpected table contents (-want +got):\n%s", diff)
	}
}

func TestBatchInserterThin(t *testing.T) {
	db := dbtest.NewDB(t)
	setupTestTableThin(t, db)

	tableSizeFactor := 2
	var expectedValues [][]any
	for i := range MaxNumPostgresParameters * tableSizeFactor {
		expectedValues = append(expectedValues, []any{i})
	}
	testInsertThin(t, db, expectedValues)

	rows, err := db.Query("SELECT col1 from batch_inserter_test_thin")
	if err != nil {
		t.Fatalf("unexpected error querying data: %s", err)
	}
	defer rows.Close()

	var values [][]any
	for rows.Next() {
		var v1 int
		if err := rows.Scan(&v1); err != nil {
			t.Fatalf("unexpected error scanning data: %s", err)
		}

		values = append(values, []any{v1})
	}

	if diff := cmp.Diff(expectedValues, values); diff != "" {
		t.Errorf("unexpected table contents (-want +got):\n%s", diff)
	}
}

func TestBatchInserterWithReturn(t *testing.T) {
	db := dbtest.NewDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	numRows := MaxNumPostgresParameters * tableSizeFactor
	expectedValues := makeTestValues(tableSizeFactor, 0)

	var expectedIDs []int
	for i := range numRows {
		expectedIDs = append(expectedIDs, i+1)
	}

	if diff := cmp.Diff(expectedIDs, testInsertWithReturn(t, db, expectedValues)); diff != "" {
		t.Errorf("unexpected returned ids (-want +got):\n%s", diff)
	}
}

func TestBatchInserterWithReturnWithConflicts(t *testing.T) {
	db := dbtest.NewDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	duplicationFactor := 2
	numRows := MaxNumPostgresParameters * tableSizeFactor
	expectedValues := makeTestValues(tableSizeFactor, 0)

	var expectedIDs []int
	for i := range numRows {
		expectedIDs = append(expectedIDs, i+1)
	}

	if diff := cmp.Diff(expectedIDs, testInsertWithReturnWithConflicts(t, db, duplicationFactor, expectedValues)); diff != "" {
		t.Errorf("unexpected returned ids (-want +got):\n%s", diff)
	}
}

func TestBatchInserterWithConflict(t *testing.T) {
	db := dbtest.NewDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	duplicationFactor := 2
	expectedValues := makeTestValues(tableSizeFactor, 0)
	testInsertWithConflicts(t, db, duplicationFactor, expectedValues)

	rows, err := db.Query("SELECT col1, col2, col3, col4, col5 from batch_inserter_test")
	if err != nil {
		t.Fatalf("unexpected error querying data: %s", err)
	}
	defer rows.Close()

	var values [][]any
	for rows.Next() {
		var v1, v2, v3, v4 int
		var v5 string
		if err := rows.Scan(&v1, &v2, &v3, &v4, &v5); err != nil {
			t.Fatalf("unexpected error scanning data: %s", err)
		}

		values = append(values, []any{v1, v2, v3, v4, v5})
	}

	if diff := cmp.Diff(expectedValues, values); diff != "" {
		t.Errorf("unexpected table contents (-want +got):\n%s", diff)
	}
}

func BenchmarkBatchInserter(b *testing.B) {
	db := dbtest.NewDB(b)
	setupTestTable(b, db)
	expectedValues := makeTestValues(10, 0)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		testInsert(b, db, expectedValues)
	}
}

func BenchmarkBatchInserterLargePayload(b *testing.B) {
	db := dbtest.NewDB(b)
	setupTestTable(b, db)
	expectedValues := makeTestValues(10, 4096)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		testInsert(b, db, expectedValues)
	}
}

func setupTestTable(t testing.TB, db *sql.DB) {
	createTableQuery := `
		CREATE TABLE batch_inserter_test (
			id SERIAL,
			col1 integer NOT NULL UNIQUE,
			col2 integer NOT NULL,
			col3 integer NOT NULL,
			col4 integer NOT NULL,
			col5 text
		)
	`
	if _, err := db.Exec(createTableQuery); err != nil {
		t.Fatalf("unexpected error creating test table: %s", err)
	}
}

func setupTestTableThin(t testing.TB, db *sql.DB) {
	createTableQuery := `
		CREATE TABLE batch_inserter_test_thin (
			id SERIAL,
			col1 integer NOT NULL UNIQUE
		)
	`
	if _, err := db.Exec(createTableQuery); err != nil {
		t.Fatalf("unexpected error creating test table: %s", err)
	}
}

func makeTestValues(tableSizeFactor, payloadSize int) [][]any {
	var expectedValues [][]any
	for i := range MaxNumPostgresParameters * tableSizeFactor {
		expectedValues = append(expectedValues, []any{
			i,
			i + 1,
			i + 2,
			i + 3,
			makePayload(payloadSize),
		})
	}

	return expectedValues
}

func makePayload(size int) string {
	s := make([]byte, 0, size)
	for range size {
		s = append(s, '!')
	}

	return string(s)
}

func testInsert(t testing.TB, db *sql.DB, expectedValues [][]any) {
	ctx := context.Background()

	inserter := NewInserter(ctx, db, "batch_inserter_test", MaxNumPostgresParameters, "col1", "col2", "col3", "col4", "col5")
	for _, values := range expectedValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("unexpected error inserting values: %s", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}
}

func testInsertThin(t testing.TB, db *sql.DB, expectedValues [][]any) {
	ctx := context.Background()

	inserter := NewInserter(ctx, db, "batch_inserter_test_thin", MaxNumPostgresParameters, "col1")
	for _, values := range expectedValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("unexpected error inserting values: %s", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}
}

func testInsertWithReturn(t testing.TB, db *sql.DB, expectedValues [][]any) (insertedIDs []int) {
	ctx := context.Background()

	inserter := NewInserterWithReturn(
		ctx,
		db,
		"batch_inserter_test",
		MaxNumPostgresParameters,
		[]string{"col1", "col2", "col3", "col4", "col5"},
		"",
		[]string{"id"},
		func(rows dbutil.Scanner) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}

			insertedIDs = append(insertedIDs, id)
			return nil
		},
	)

	for _, values := range expectedValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("unexpected error inserting values: %s", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}

	return insertedIDs
}

func testInsertWithReturnWithConflicts(t testing.TB, db *sql.DB, n int, expectedValues [][]any) (insertedIDs []int) {
	ctx := context.Background()

	inserter := NewInserterWithReturn(
		ctx,
		db,
		"batch_inserter_test",
		MaxNumPostgresParameters,
		[]string{"id", "col1", "col2", "col3", "col4", "col5"},
		"ON CONFLICT DO NOTHING",
		[]string{"id"},
		func(rows dbutil.Scanner) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}

			insertedIDs = append(insertedIDs, id)
			return nil
		},
	)

	for range n {
		for j, values := range expectedValues {
			if err := inserter.Insert(ctx, append([]any{j + 1}, values...)...); err != nil {
				t.Fatalf("unexpected error inserting values: %s", err)
			}
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}

	return insertedIDs
}

func testInsertWithConflicts(t testing.TB, db *sql.DB, n int, expectedValues [][]any) {
	ctx := context.Background()

	inserter := NewInserterWithConflict(
		ctx,
		db,
		"batch_inserter_test",
		MaxNumPostgresParameters,
		"ON CONFLICT DO NOTHING",
		"id", "col1", "col2", "col3", "col4", "col5",
	)

	for range n {
		for j, values := range expectedValues {
			if err := inserter.Insert(ctx, append([]any{j + 1}, values...)...); err != nil {
				t.Fatalf("unexpected error inserting values: %s", err)
			}
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}
}
