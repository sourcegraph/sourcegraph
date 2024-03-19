package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/derision-test/glock"
	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/executor"
)

func TestStoreQueuedCount(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval),
			(2, 'queued', NOW() - '2 minute'::interval),
			(3, 'state2', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval),
			(5, 'processing', NOW() - '5 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	count, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).QueuedCount(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error getting queued count: %s", err)
	}
	if count != 3 {
		t.Errorf("unexpected count. want=%d have=%d", 3, count)
	}
}

func TestStoreQueuedCountIncludeProcessing(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval),
			(2, 'queued', NOW() - '2 minute'::interval),
			(3, 'state2', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval),
			(5, 'processing', NOW() - '5 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	count, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).QueuedCount(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error getting queued count: %s", err)
	}
	if count != 4 {
		t.Errorf("unexpected count. want=%d have=%d", 4, count)
	}
}

func TestStoreQueuedCountFailed(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at, num_failures)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval, 0),
			(2, 'errored', NOW() - '2 minute'::interval, 2),
			(3, 'state2', NOW() - '3 minute'::interval, 0),
			(4, 'errored', NOW() - '4 minute'::interval, 3),
			(5, 'state2', NOW() - '5 minute'::interval, 0),
			(6, 'failed', NOW() - '6 minute'::interval, 1)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	count, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).QueuedCount(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error getting queued count: %s", err)
	}
	if count != 3 {
		t.Errorf("unexpected count. want=%d have=%d", 3, count)
	}
}

func TestStoreMaxDurationInQueue(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '20 minutes'::interval), -- young
			(2, 'queued', NOW() - '30 minutes'::interval), -- oldest queued
			(3, 'state2', NOW() - '40 minutes'::interval), -- wrong state
			(4, 'queued', NOW() - '10 minutes'::interval), -- young
			(5, 'state3', NOW() - '50 minutes'::interval)  -- wrong state
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	age, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MaxDurationInQueue(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting max duration in queue: %s", err)
	}
	if age.Round(time.Second) != 30*time.Minute {
		t.Fatalf("unexpected max age. want=%s have=%s", 30*time.Minute, age)
	}
}

func TestStoreMaxDurationInQueueProcessAfter(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at, process_after)
		VALUES
			(1, 'queued', NOW() - '90 minutes'::interval, NOW() + '10 minutes'::interval), -- oldest queued, waiting for process_after
			(2, 'queued', NOW() - '70 minutes'::interval, NOW() - '30 minutes'::interval), -- oldest queued
			(3, 'state2', NOW() - '40 minutes'::interval, NULL),                           -- wrong state
			(4, 'queued', NOW() - '10 minutes'::interval, NULL),                           -- young
			(5, 'state3', NOW() - '50 minutes'::interval, NULL)                            -- wrong state
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	age, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MaxDurationInQueue(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting max duration in queue: %s", err)
	}
	if age.Round(time.Second) != 30*time.Minute {
		t.Fatalf("unexpected max age. want=%s have=%s", 30*time.Minute, age)
	}
}

func TestStoreMaxDurationInQueueFailed(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at, finished_at, num_failures)
		VALUES
			(1, 'queued',  NOW() - '10 minutes'::interval, NULL,                           0), -- young
			(2, 'errored', NOW(),                          NOW() - '30 minutes'::interval, 2), -- oldest retryable error'd
			(3, 'errored', NOW(),                          NOW() - '10 minutes'::interval, 2), -- retryable, but too young to be queued
			(4, 'state2',  NOW() - '40 minutes'::interval, NULL,                           0), -- wrong state
			(5, 'queued',  NOW() - '20 minutes'::interval, NULL,                           0), -- oldest queued
			(6, 'failed',  NOW(),                          NOW() - '60 minutes'::interval, 1)  -- wrong state
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	options := defaultTestStoreOptions(nil, testScanRecord)
	options.RetryAfter = 5 * time.Minute

	age, err := testStore(db, options).MaxDurationInQueue(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting max duration in queue: %s", err)
	}
	if age.Round(time.Second) != 25*time.Minute {
		t.Fatalf("unexpected max age. want=%s have=%s", 25*time.Minute, age)
	}
}

func TestStoreMaxDurationInQueueEmpty(t *testing.T) {
	db := setupStoreTest(t)

	age, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MaxDurationInQueue(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting max duration in queue: %s", err)
	}
	if age.Round(time.Second) != 0*time.Minute {
		t.Fatalf("unexpected max age. want=%s have=%s", 0*time.Minute, age)
	}
}

func TestStoreDequeueState(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval),
			(2, 'queued', NOW() - '2 minute'::interval),
			(3, 'state2', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval),
			(5, 'state2', NOW() - '5 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	record, ok, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Dequeue(context.Background(), "test", nil)
	assertDequeueRecordResult(t, 4, record, ok, err)
}

func TestStoreDequeueOrder(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '2 minute'::interval),
			(2, 'queued', NOW() - '5 minute'::interval),
			(3, 'queued', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '1 minute'::interval),
			(5, 'queued', NOW() - '4 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	record, ok, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Dequeue(context.Background(), "test", nil)
	assertDequeueRecordResult(t, 2, record, ok, err)
}

func TestStoreDequeueConditions(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval),
			(2, 'queued', NOW() - '2 minute'::interval),
			(3, 'queued', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval),
			(5, 'queued', NOW() - '5 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	conditions := []*sqlf.Query{sqlf.Sprintf("workerutil_test.id < 4")}
	record, ok, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Dequeue(context.Background(), "test", conditions)
	assertDequeueRecordResult(t, 3, record, ok, err)
}

func TestStoreDequeueResetExecutionLogs(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, execution_logs, created_at)
		VALUES
			(1, 'queued', E'{"{\\"key\\": \\"test\\"}"}', NOW() - '1 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	record, ok, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Dequeue(context.Background(), "test", nil)
	assertDequeueRecordResult(t, 1, record, ok, err)
	assertDequeueRecordResultLogCount(t, 0, record)
}

func TestStoreDequeueDelay(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at, process_after)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval, NULL),
			(2, 'queued', NOW() - '2 minute'::interval, NULL),
			(3, 'queued', NOW() - '3 minute'::interval, NOW() + '2 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval, NULL),
			(5, 'queued', NOW() - '5 minute'::interval, NOW() + '1 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	record, ok, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Dequeue(context.Background(), "test", nil)
	assertDequeueRecordResult(t, 4, record, ok, err)
}

func TestStoreDequeueView(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '1 minute'::interval),
			(2, 'queued', NOW() - '2 minute'::interval),
			(3, 'queued', NOW() - '3 minute'::interval),
			(4, 'queued', NOW() - '4 minute'::interval),
			(5, 'queued', NOW() - '5 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	options := defaultTestStoreOptions(nil, testScanRecordView)
	options.ViewName = "workerutil_test_view v"
	options.OrderByExpression = sqlf.Sprintf("v.created_at")
	options.ColumnExpressions = []*sqlf.Query{
		sqlf.Sprintf("v.id"),
		sqlf.Sprintf("v.state"),
		sqlf.Sprintf("v.new_field"),
	}

	conditions := []*sqlf.Query{sqlf.Sprintf("v.new_field < 15")}
	record, ok, err := testStore(db, options).Dequeue(context.Background(), "test", conditions)
	assertDequeueRecordViewResult(t, 2, 14, record, ok, err)
}

func TestStoreDequeueConcurrent(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, created_at)
		VALUES
			(1, 'queued', NOW() - '2 minute'::interval),
			(2, 'queued', NOW() - '1 minute'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	store := testStore(db, defaultTestStoreOptions(nil, testScanRecord))

	// Worker A
	record1, ok, err := store.Dequeue(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("expected a dequeueable record")
	}

	// Worker B
	record2, ok, err := store.Dequeue(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if !ok {
		t.Fatalf("expected a second dequeueable record")
	}

	if val := record1.ID; val != 1 {
		t.Errorf("unexpected id. want=%d have=%d", 1, val)
	}
	if val := record2.ID; val != 2 {
		t.Errorf("unexpected id. want=%d have=%d", 2, val)
	}

	// Worker C
	_, ok, err = store.Dequeue(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ok {
		t.Fatalf("did not expect a third dequeueable record")
	}
}

func TestStoreDequeueRetryAfter(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, finished_at, failure_message, num_failures, created_at)
		VALUES
			(1, 'errored', NOW() - '6 minute'::interval, 'error', 3, NOW() - '2 minutes'::interval),
			(2, 'errored', NOW() - '4 minute'::interval, 'error', 0, NOW() - '3 minutes'::interval),
			(3, 'failed',  NOW() - '6 minute'::interval, 'error', 5, NOW() - '4 minutes'::interval),
			(4, 'queued',                          NULL,    NULL, 0, NOW() - '1 minutes'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	options := defaultTestStoreOptions(nil, testScanRecordRetry)
	options.MaxNumRetries = 5
	options.RetryAfter = 5 * time.Minute
	options.ColumnExpressions = []*sqlf.Query{
		sqlf.Sprintf("workerutil_test.id"),
		sqlf.Sprintf("workerutil_test.state"),
		sqlf.Sprintf("workerutil_test.num_resets"),
	}
	store := testStore(db, options)

	// Dequeue errored record
	record1, ok, err := store.Dequeue(context.Background(), "test", nil)
	assertDequeueRecordRetryResult(t, 1, record1, ok, err)

	// Dequeue non-errored record
	record2, ok, err := store.Dequeue(context.Background(), "test", nil)
	assertDequeueRecordRetryResult(t, 4, record2, ok, err)

	// Does not dequeue old or max retried errored
	if _, ok, _ := store.Dequeue(context.Background(), "test", nil); ok {
		t.Fatalf("did not expect a third dequeueable record")
	}
}

func TestStoreDequeueRetryAfterDisabled(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, finished_at, failure_message, num_failures, created_at)
		VALUES
			(1, 'errored', NOW() - '6 minute'::interval, 'error', 3, NOW() - '2 minutes'::interval),
			(2, 'errored', NOW() - '4 minute'::interval, 'error', 0, NOW() - '3 minutes'::interval),
			(3, 'errored', NOW() - '6 minute'::interval, 'error', 5, NOW() - '4 minutes'::interval),
			(4, 'queued',                          NULL,    NULL, 0, NOW() - '1 minutes'::interval)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	options := defaultTestStoreOptions(nil, testScanRecordRetry)
	options.MaxNumRetries = 5
	options.RetryAfter = 0
	options.ColumnExpressions = []*sqlf.Query{
		sqlf.Sprintf("workerutil_test.id"),
		sqlf.Sprintf("workerutil_test.state"),
		sqlf.Sprintf("workerutil_test.num_resets"),
	}

	store := testStore(db, options)

	// Dequeue non-errored record only
	record2, ok, err := store.Dequeue(context.Background(), "test", nil)
	assertDequeueRecordRetryResult(t, 4, record2, ok, err)

	// Does not dequeue errored
	if _, ok, _ := store.Dequeue(context.Background(), "test", nil); ok {
		t.Fatalf("did not expect a second dequeueable record")
	}
}

func TestStoreRequeue(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	after := testNow().Add(time.Hour)

	if err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Requeue(context.Background(), 1, after); err != nil {
		t.Fatalf("unexpected error requeueing record: %s", err)
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, process_after FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var processAfter *time.Time

	if err := rows.Scan(&state, &processAfter); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "queued" {
		t.Errorf("unexpected state. want=%q have=%q", "queued", state)
	}
	if processAfter == nil || !processAfter.Equal(after) {
		t.Errorf("unexpected process after. want=%s have=%s", after, processAfter)
	}
}

func TestStoreAddExecutionLogEntry(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	numEntries := 5

	for i := range numEntries {
		command := []string{"ls", "-a", fmt.Sprintf("%d", i+1)}
		payload := fmt.Sprintf("<load payload %d>", i+1)

		entry := executor.ExecutionLogEntry{
			Command: command,
			Out:     payload,
		}

		entryID, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).AddExecutionLogEntry(context.Background(), 1, entry, ExecutionLogEntryOptions{})
		if err != nil {
			t.Fatalf("unexpected error adding executor log entry: %s", err)
		}
		// PostgreSQL's arrays use 1-based indexing, so the first entry is at 1
		if entryID != i+1 {
			t.Fatalf("executor log entry has wrong entry id. want=%d, have=%d", i+1, entryID)
		}
	}

	contents, err := basestore.ScanStrings(db.QueryContext(context.Background(), `SELECT unnest(execution_logs)::text FROM workerutil_test WHERE id = 1`))
	if err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if len(contents) != numEntries {
		t.Fatalf("unexpected number of payloads. want=%d have=%d", numEntries, len(contents))
	}

	for i := range numEntries {
		var entry executor.ExecutionLogEntry
		if err := json.Unmarshal([]byte(contents[i]), &entry); err != nil {
			t.Fatalf("unexpected error decoding entry: %s", err)
		}

		expected := executor.ExecutionLogEntry{
			Command: []string{"ls", "-a", fmt.Sprintf("%d", i+1)},
			Out:     fmt.Sprintf("<load payload %d>", i+1),
		}
		if diff := cmp.Diff(expected, entry); diff != "" {
			t.Errorf("unexpected entry (-want +got):\n%s", diff)
		}
	}
}

func TestStoreAddExecutionLogEntryNoRecord(t *testing.T) {
	db := setupStoreTest(t)

	entry := executor.ExecutionLogEntry{
		Command: []string{"ls", "-a"},
		Out:     "output",
	}

	_, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).AddExecutionLogEntry(context.Background(), 1, entry, ExecutionLogEntryOptions{})
	if err == nil {
		t.Fatalf("expected error but got none")
	}
}

func TestStoreUpdateExecutionLogEntry(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	numEntries := 5
	for i := range numEntries {
		command := []string{"ls", "-a", fmt.Sprintf("%d", i+1)}
		payload := fmt.Sprintf("<load payload %d>", i+1)

		entry := executor.ExecutionLogEntry{
			Command: command,
			Out:     payload,
		}

		entryID, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).AddExecutionLogEntry(context.Background(), 1, entry, ExecutionLogEntryOptions{})
		if err != nil {
			t.Fatalf("unexpected error adding executor log entry: %s", err)
		}
		// PostgreSQL's arrays use 1-based indexing, so the first entry is at 1
		if entryID != i+1 {
			t.Fatalf("executor log entry has wrong entry id. want=%d, have=%d", i+1, entryID)
		}

		entry.Out += fmt.Sprintf("\n<load payload %d again, nobody was at home>", i+1)
		if err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).UpdateExecutionLogEntry(context.Background(), 1, entryID, entry, ExecutionLogEntryOptions{}); err != nil {
			t.Fatalf("unexpected error updating executor log entry: %s", err)
		}
	}

	contents, err := basestore.ScanStrings(db.QueryContext(context.Background(), `SELECT unnest(execution_logs)::text FROM workerutil_test WHERE id = 1`))
	if err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if len(contents) != numEntries {
		t.Fatalf("unexpected number of payloads. want=%d have=%d", numEntries, len(contents))
	}

	for i := range numEntries {
		var entry executor.ExecutionLogEntry
		if err := json.Unmarshal([]byte(contents[i]), &entry); err != nil {
			t.Fatalf("unexpected error decoding entry: %s", err)
		}

		expected := executor.ExecutionLogEntry{
			Command: []string{"ls", "-a", fmt.Sprintf("%d", i+1)},
			Out:     fmt.Sprintf("<load payload %d>\n<load payload %d again, nobody was at home>", i+1, i+1),
		}
		if diff := cmp.Diff(expected, entry); diff != "" {
			t.Errorf("unexpected entry (-want +got):\n%s", diff)
		}
	}
}

func TestStoreUpdateExecutionLogEntryUnknownEntry(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	entry := executor.ExecutionLogEntry{
		Command: []string{"ls", "-a"},
		Out:     "<load payload>",
	}

	for unknownEntryID := range 2 {
		err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).UpdateExecutionLogEntry(context.Background(), 1, unknownEntryID, entry, ExecutionLogEntryOptions{})
		if err == nil {
			t.Fatal("expected error but got none")
		}
	}
}

func TestStoreMarkComplete(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkComplete(context.Background(), 1, MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking record as completed: %s", err)
	}
	if !marked {
		t.Fatalf("expected record to be marked")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "completed" {
		t.Errorf("unexpected state. want=%q have=%q", "completed", state)
	}
	if failureMessage != nil {
		t.Errorf("unexpected failure message. want=%v have=%v", nil, failureMessage)
	}
}

func TestStoreMarkCompleteNotProcessing(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, failure_message)
		VALUES
			(1, 'errored', 'old message')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkComplete(context.Background(), 1, MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking record as completed: %s", err)
	}
	if marked {
		t.Fatalf("expected record not to be marked")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "errored" {
		t.Errorf("unexpected state. want=%q have=%q", "errored", state)
	}
	if failureMessage == nil || *failureMessage != "old message" {
		t.Errorf("unexpected failure message. want=%v have=%v", "old message", failureMessage)
	}
}

func TestStoreMarkErrored(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkErrored(context.Background(), 1, "new message", MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking record as errored: %s", err)
	}
	if !marked {
		t.Fatalf("expected record to be marked")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "errored" {
		t.Errorf("unexpected state. want=%q have=%q", "errored", state)
	}
	if failureMessage == nil || *failureMessage != "new message" {
		t.Errorf("unexpected failure message. want=%v have=%v", "new message", failureMessage)
	}
}

func TestStoreMarkFailed(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'processing')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkFailed(context.Background(), 1, "new message", MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking upload as completed: %s", err)
	}
	if !marked {
		t.Fatalf("expected record to be marked")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "failed" {
		t.Errorf("unexpected state. want=%q have=%q", "failed", state)
	}
	if failureMessage == nil || *failureMessage != "new message" {
		t.Errorf("unexpected failure message. want=%v have=%v", "new message", failureMessage)
	}
}

func TestStoreMarkErroredAlreadyCompleted(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state)
		VALUES
			(1, 'completed')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkErrored(context.Background(), 1, "new message", MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking record as errored: %s", err)
	}
	if marked {
		t.Fatalf("expected record not to be marked errired")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "completed" {
		t.Errorf("unexpected state. want=%q have=%q", "completed", state)
	}
	if failureMessage != nil {
		t.Errorf("unexpected non-empty failure message")
	}
}

func TestStoreMarkErroredAlreadyErrored(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, failure_message)
		VALUES
			(1, 'errored', 'old message')
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	marked, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).MarkErrored(context.Background(), 1, "new message", MarkFinalOptions{})
	if err != nil {
		t.Fatalf("unexpected error marking record as errored: %s", err)
	}
	if marked {
		t.Fatalf("expected record not to be marked")
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, failure_message FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var failureMessage *string
	if err := rows.Scan(&state, &failureMessage); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "errored" {
		t.Errorf("unexpected state. want=%q have=%q", "errored", state)
	}
	if failureMessage == nil || *failureMessage != "old message" {
		t.Errorf("unexpected failure message. want=%v have=%v", "old message", failureMessage)
	}
}

func TestStoreMarkErroredRetriesExhausted(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, num_failures)
		VALUES
			(1, 'processing', 0),
			(2, 'processing', 1)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	options := defaultTestStoreOptions(nil, testScanRecord)
	options.MaxNumRetries = 2
	store := testStore(db, options)

	for i := 1; i < 3; i++ {
		marked, err := store.MarkErrored(context.Background(), i, "new message", MarkFinalOptions{})
		if err != nil {
			t.Fatalf("unexpected error marking record as errored: %s", err)
		}
		if !marked {
			t.Fatalf("expected record to be marked")
		}
	}

	assertState := func(id int, wantState string) {
		q := fmt.Sprintf(`SELECT state FROM workerutil_test WHERE id = %d`, id)
		rows, err := db.QueryContext(context.Background(), q)
		if err != nil {
			t.Fatalf("unexpected error querying record: %s", err)
		}
		defer func() { _ = basestore.CloseRows(rows, nil) }()

		if !rows.Next() {
			t.Fatal("expected record to exist")
		}

		var state string
		if err := rows.Scan(&state); err != nil {
			t.Fatalf("unexpected error scanning record: %s", err)
		}
		if state != wantState {
			t.Errorf("record %d unexpected state. want=%q have=%q", id, wantState, state)
		}
	}

	assertState(1, "errored")
	assertState(2, "failed")
}

func TestStoreResetStalled(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, last_heartbeat_at, num_resets)
		VALUES
			(1, 'processing', NOW() - '6 second'::interval, 1),
			(2, 'processing', NOW() - '2 second'::interval, 0),
			(3, 'processing', NOW() - '3 second'::interval, 0),
			(4, 'processing', NOW() - '8 second'::interval, 0),
			(5, 'processing', NOW() - '8 second'::interval, 0),
			(6, 'processing', NOW() - '6 second'::interval, 5),
			(7, 'processing', NOW() - '8 second'::interval, 5)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	// Row lock record 5 in a transaction which should be skipped by ResetStalled
	if _, err := tx.Exec(`SELECT * FROM workerutil_test WHERE id = 5 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}

	resetLastHeartbeatsByIDs, erroredLastHeartbeatsByIDs, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).ResetStalled(context.Background())
	if err != nil {
		t.Fatalf("unexpected error resetting stalled records: %s", err)
	}

	var resetIDs []int
	for id := range resetLastHeartbeatsByIDs {
		resetIDs = append(resetIDs, id)
	}
	sort.Ints(resetIDs)

	var erroredIDs []int
	for id := range erroredLastHeartbeatsByIDs {
		erroredIDs = append(erroredIDs, id)
	}
	sort.Ints(erroredIDs)

	if diff := cmp.Diff([]int{1, 4}, resetIDs); diff != "" {
		t.Errorf("unexpected reset ids (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]int{6, 7}, erroredIDs); diff != "" {
		t.Errorf("unexpected errored ids (-want +got):\n%s", diff)
	}

	rows, err := db.QueryContext(context.Background(), `SELECT state, num_resets FROM workerutil_test WHERE id = 1`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	var state string
	var numResets int
	if err := rows.Scan(&state, &numResets); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "queued" {
		t.Errorf("unexpected state. want=%q have=%q", "queued", state)
	}
	if numResets != 2 {
		t.Errorf("unexpected num resets. want=%d have=%d", 2, numResets)
	}

	rows, err = db.QueryContext(context.Background(), `SELECT state FROM workerutil_test WHERE id = 6`)
	if err != nil {
		t.Fatalf("unexpected error querying record: %s", err)
	}
	defer func() { _ = basestore.CloseRows(rows, nil) }()

	if !rows.Next() {
		t.Fatal("expected record to exist")
	}

	if err := rows.Scan(&state); err != nil {
		t.Fatalf("unexpected error scanning record: %s", err)
	}
	if state != "failed" {
		t.Errorf("unexpected state. want=%q have=%q", "failed", state)
	}
}

func TestStoreHeartbeat(t *testing.T) {
	db := setupStoreTest(t)

	now := time.Unix(1587396557, 0).UTC()
	clock := glock.NewMockClockAt(now)
	store := testStore(db, defaultTestStoreOptions(clock, testScanRecord))

	if err := store.Exec(context.Background(), sqlf.Sprintf(`
		INSERT INTO workerutil_test (id, state, worker_hostname, last_heartbeat_at)
		VALUES
			(1, 'queued', 'worker1', %s),
			(2, 'queued', 'worker1', %s),
			(3, 'queued', 'worker2', %s)
	`, now, now, now)); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	readAndCompareTimes := func(expected map[int]time.Duration) {
		times, err := scanLastHeartbeatTimestampsFrom(clock.Now())(store.Query(context.Background(), sqlf.Sprintf(`
			SELECT id, last_heartbeat_at FROM workerutil_test
		`)))
		if err != nil {
			t.Fatalf("unexpected error scanning heartbeats: %s", err)
		}

		if diff := cmp.Diff(expected, times); diff != "" {
			t.Errorf("unexpected times (-want +got):\n%s", diff)
		}
	}

	clock.Advance(5 * time.Second)

	if _, _, err := store.Heartbeat(context.Background(), []string{"1", "2", "3"}, HeartbeatOptions{}); err != nil {
		t.Fatalf("unexpected error updating heartbeat: %s", err)
	}
	readAndCompareTimes(map[int]time.Duration{
		1: 5 * time.Second, // not updated, clock advanced 5s from start; note state='queued'
		2: 5 * time.Second, // not updated, clock advanced 5s from start; note state='queued'
		3: 5 * time.Second, // not updated, clock advanced 5s from start; note state='queued'
	})

	// Now update state to processing and expect it to update properly.
	if _, err := db.ExecContext(context.Background(), `UPDATE workerutil_test SET state = 'processing'`); err != nil {
		t.Fatalf("unexpected error updating records: %s", err)
	}

	clock.Advance(5 * time.Second)

	// Only one worker
	if _, _, err := store.Heartbeat(context.Background(), []string{"1", "2", "3"}, HeartbeatOptions{WorkerHostname: "worker1"}); err != nil {
		t.Fatalf("unexpected error updating heartbeat: %s", err)
	}
	readAndCompareTimes(map[int]time.Duration{
		1: 0,                // updated
		2: 0,                // updated
		3: 10 * time.Second, // not updated, clock advanced 10s from start; note worker_hostname=worker2
	})

	clock.Advance(5 * time.Second)

	// Multiple workers
	if _, _, err := store.Heartbeat(context.Background(), []string{"1", "3"}, HeartbeatOptions{}); err != nil {
		t.Fatalf("unexpected error updating heartbeat: %s", err)
	}
	readAndCompareTimes(map[int]time.Duration{
		1: 0,               // updated
		2: 5 * time.Second, // not in known ID list
		3: 0,               // updated
	})
}

func TestStoreCanceledJobs(t *testing.T) {
	db := setupStoreTest(t)

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO workerutil_test (id, state, worker_hostname, cancel)
		VALUES
			-- not processing
			(1, 'queued', 'worker1', false),
			-- not canceling
			(2, 'processing', 'worker1', false),
			-- this one should be returned
			(3, 'processing', 'worker1', true),
			-- other worker
			(4, 'processing', 'worker2', true)
	`); err != nil {
		t.Fatalf("unexpected error inserting records: %s", err)
	}

	_, toCancel, err := testStore(db, defaultTestStoreOptions(nil, testScanRecord)).Heartbeat(context.Background(), []string{"1", "2", "3"}, HeartbeatOptions{WorkerHostname: "worker1"})
	if err != nil {
		t.Fatalf("unexpected error fetching canceled jobs: %s", err)
	}

	require.ElementsMatch(t, toCancel, []string{"3"}, "invalid set of jobs returned")
}
