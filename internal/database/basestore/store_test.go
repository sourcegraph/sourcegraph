package basestore

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestTransaction(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	setupStoreTest(t, db)
	store := testStore(t, db)

	// Add record outside of transaction, ensure it's visible
	if err := store.Exec(context.Background(), sqlf.Sprintf(`INSERT INTO store_counts_test VALUES (1, 42)`)); err != nil {
		t.Fatalf("unexpected error inserting count: %s", err)
	}
	assertCounts(t, db, map[int]int{1: 42})

	// Add record inside of a transaction
	tx1, err := store.Transact(context.Background())
	if err != nil {
		t.Fatalf("unexpected error creating transaction: %s", err)
	}
	if err := tx1.Exec(context.Background(), sqlf.Sprintf(`INSERT INTO store_counts_test VALUES (2, 43)`)); err != nil {
		t.Fatalf("unexpected error inserting count: %s", err)
	}

	// Add record inside of another transaction
	tx2, err := store.Transact(context.Background())
	if err != nil {
		t.Fatalf("unexpected error creating transaction: %s", err)
	}
	if err := tx2.Exec(context.Background(), sqlf.Sprintf(`INSERT INTO store_counts_test VALUES (3, 44)`)); err != nil {
		t.Fatalf("unexpected error inserting count: %s", err)
	}

	// Check what's visible pre-commit/rollback
	assertCounts(t, db, map[int]int{1: 42})
	assertCounts(t, tx1.handle, map[int]int{1: 42, 2: 43})
	assertCounts(t, tx2.handle, map[int]int{1: 42, 3: 44})

	// Finalize transactions
	rollbackErr := errors.New("rollback")
	if err := tx1.Done(rollbackErr); !errors.Is(err, rollbackErr) {
		t.Fatalf("unexpected error rolling back transaction. want=%q have=%q", rollbackErr, err)
	}
	if err := tx2.Done(nil); err != nil {
		t.Fatalf("unexpected error committing transaction: %s", err)
	}

	// Check what's visible post-commit/rollback
	assertCounts(t, db, map[int]int{1: 42, 3: 44})
}

func TestConcurrentTransactions(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	setupStoreTest(t, db)
	store := testStore(t, db)
	ctx := context.Background()

	t.Run("creating transactions concurrently does not fail", func(t *testing.T) {
		var g errgroup.Group
		for range 2 {
			g.Go(func() (err error) {
				tx, err := store.Transact(ctx)
				if err != nil {
					return err
				}
				defer func() { err = tx.Done(err) }()

				return tx.Exec(ctx, sqlf.Sprintf(`select pg_sleep(0.1)`))
			})
		}
		require.NoError(t, g.Wait())
	})

	t.Run("parallel insertion on a single transaction does not fail but logs an error", func(t *testing.T) {
		tx, err := store.Transact(ctx)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := tx.Done(err); err != nil {
				t.Fatalf("closing transaction failed: %s", err)
			}
		})

		capturingLogger, export := logtest.Captured(t)
		tx.handle.(*txHandle).logger = capturingLogger

		var g errgroup.Group
		for i := range 2 {
			routine := i
			g.Go(func() (err error) {
				if err := tx.Exec(ctx, sqlf.Sprintf(`SELECT pg_sleep(0.1);`)); err != nil {
					return err
				}
				return tx.Exec(ctx, sqlf.Sprintf(`INSERT INTO store_counts_test VALUES (%s, %s)`, routine, routine))
			})
		}
		err = g.Wait()
		require.NoError(t, err)

		captured := export()
		require.NotEmpty(t, captured)
		require.Equal(t, "transaction used concurrently", captured[0].Message)
	})
}

func TestSavepoints(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	setupStoreTest(t, db)

	NumSavepointTests := 10

	for i := range NumSavepointTests {
		t.Run(fmt.Sprintf("i=%d", i), func(t *testing.T) {
			if _, err := db.Exec(`TRUNCATE store_counts_test`); err != nil {
				t.Fatalf("unexpected error truncating table: %s", err)
			}

			// Make `NumSavepointTests` "nested transactions", where the transaction
			// or savepoint at index `i` will be rolled back. Note that all of the
			// actions in any savepoint after this index will also be rolled back.
			recurSavepoints(t, testStore(t, db), NumSavepointTests, i)

			expected := map[int]int{}
			for j := NumSavepointTests; j > i; j-- {
				expected[j] = j * 2
			}
			assertCounts(t, db, expected)
		})
	}
}

func TestSetLocal(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	setupStoreTest(t, db)
	store := testStore(t, db)

	_, err := store.SetLocal(context.Background(), "sourcegraph.banana", "phone")
	if err == nil {
		t.Fatalf("unexpected nil error")
	}
	if !errors.Is(err, ErrNotInTransaction) {
		t.Fatalf("unexpected error. want=%q have=%q", ErrNotInTransaction, err)
	}

	store, _ = store.Transact(context.Background())
	defer store.Done(err)
	func() {
		unset, err := store.SetLocal(context.Background(), "sourcegraph.banana", "phone")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		defer unset(context.Background())

		str, _, err := ScanFirstString(store.Query(context.Background(), sqlf.Sprintf("SELECT current_setting('sourcegraph.banana')")))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if str != "phone" {
			t.Fatalf("unexpected value. want=%q got=%q", "phone", str)
		}
	}()

	str, _, err := ScanFirstString(store.Query(context.Background(), sqlf.Sprintf("SELECT current_setting('sourcegraph.banana', true)")))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if str != "" {
		t.Fatalf("unexpected value. want=%q got=%q", "", str)
	}
}

func TestScanFirstString(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	store := testStore(t, db)

	cases := []struct {
		name        string
		query       string
		expected    string
		called      bool
		shouldError bool
	}{
		{
			name:        "multiple rows returned",
			query:       "SELECT 'A' UNION ALL SELECT 'B'",
			expected:    "A",
			called:      true,
			shouldError: false,
		},
		{
			name:        "single row returned",
			query:       "SELECT 'A'",
			expected:    "A",
			called:      true,
			shouldError: false,
		},
		{
			name:        "no rows returned",
			query:       "SELECT 'A' where 1=0",
			expected:    "",
			called:      false,
			shouldError: false,
		},
		{
			name:        "null return",
			query:       "SELECT null",
			expected:    "",
			called:      true,
			shouldError: true,
		},
		{
			name:        "multiple rows error first row",
			query:       "SELECT null UNION ALL select 'A'",
			expected:    "",
			called:      true,
			shouldError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, called, err := ScanFirstString(store.Query(context.Background(), sqlf.Sprintf(tc.query)))
			if got != tc.expected {
				t.Fatalf("unexpected value. want=%s got=%s", tc.expected, got)
			}
			if called != tc.called {
				t.Fatalf("unexpected called value. want=%t got=%t", tc.called, called)
			}
			if err != nil && !tc.shouldError {
				t.Fatalf("unexpected error: %s", err)
			}
			if err == nil && tc.shouldError {
				t.Fatal("expected error")
			}
		})
	}
}

func recurSavepoints(t *testing.T, store *Store, index, rollbackAt int) {
	if index == 0 {
		return
	}

	tx, err := store.Transact(context.Background())
	if err != nil {
		t.Fatalf("unexpected error creating transaction: %s", err)
	}
	defer func() {
		var doneErr error
		if index == rollbackAt {
			doneErr = errors.New("rollback")
		}

		if err := tx.Done(doneErr); !errors.Is(err, doneErr) {
			t.Fatalf("unexpected error closing transaction. want=%q have=%q", doneErr, err)
		}
	}()

	if err := tx.Exec(context.Background(), sqlf.Sprintf(`INSERT INTO store_counts_test VALUES (%s, %s)`, index, index*2)); err != nil {
		t.Fatalf("unexpected error inserting count: %s", err)
	}

	recurSavepoints(t, tx, index-1, rollbackAt)
}

func testStore(t testing.TB, db *sql.DB) *Store {
	return NewWithHandle(NewHandleWithDB(logtest.Scoped(t), db, sql.TxOptions{}))
}

func assertCounts(t *testing.T, db dbutil.DB, expectedCounts map[int]int) {
	rows, err := db.QueryContext(context.Background(), `SELECT id, value FROM store_counts_test`)
	if err != nil {
		t.Fatalf("unexpected error querying counts: %s", err)
	}
	defer func() { _ = CloseRows(rows, nil) }()

	counts := map[int]int{}
	for rows.Next() {
		var id, count int
		if err := rows.Scan(&id, &count); err != nil {
			t.Fatalf("unexpected error scanning row: %s", err)
		}

		counts[id] = count
	}

	if diff := cmp.Diff(expectedCounts, counts); diff != "" {
		t.Errorf("unexpected counts args (-want +got):\n%s", diff)
	}
}

// setupStoreTest creates a table used only for testing. This table does not need to be truncated
// between tests as all tables in the test database are truncated by SetupGlobalTestDB.
func setupStoreTest(t *testing.T, db dbutil.DB) {
	if testing.Short() {
		t.Skip()
	}

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS store_counts_test (
			id    integer NOT NULL,
			value integer NOT NULL
		)
	`); err != nil {
		t.Fatalf("unexpected error creating test table: %s", err)
	}
}
