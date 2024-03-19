package lsifstore

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestDeleteLsifDataByUploadIds(t *testing.T) {
	logger := logtest.ScopedWith(t, logtest.LoggerOptions{
		Level: log.LevelError,
	})
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)

	t.Run("scip", func(t *testing.T) {
		for i := range 5 {
			query := sqlf.Sprintf("INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tooL_name, tool_version, tool_arguments, protocol_version) VALUES (%s, 'utf8', '', '', '{}', 1)", i+1)

			if _, err := codeIntelDB.ExecContext(context.Background(), query.Query(sqlf.PostgresBindVar), query.Args()...); err != nil {
				t.Fatalf("unexpected error inserting repo: %s", err)
			}
		}

		if err := store.DeleteLsifDataByUploadIds(context.Background(), 2, 4); err != nil {
			t.Fatalf("unexpected error clearing bundle data: %s", err)
		}

		uploadIDs, err := basestore.ScanInts(codeIntelDB.QueryContext(context.Background(), "SELECT upload_id FROM codeintel_scip_metadata"))
		if err != nil {
			t.Fatalf("Unexpected error querying dump identifiers: %s", err)
		}

		if diff := cmp.Diff([]int{1, 3, 5}, uploadIDs); diff != "" {
			t.Errorf("unexpected dump identifiers (-want +got):\n%s", diff)
		}
	})
}

func TestDeleteAbandonedSchemaVersionsRecords(t *testing.T) {
	logger := logtest.ScopedWith(t, logtest.LoggerOptions{
		Level: log.LevelError,
	})
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)
	ctx := context.Background()

	assertCounts := func(expectedNumSymbols, expectedNumDocuments int) {
		numSymbols, _, err := basestore.ScanFirstInt(codeIntelDB.QueryContext(ctx, "SELECT COUNT(*) FROM codeintel_scip_symbols_schema_versions"))
		if err != nil {
			t.Fatalf("unexpected error fetching count: %s", err)
		}
		if numSymbols != expectedNumSymbols {
			t.Errorf("unexpected number of symbols schema version records. want=%d have=%d", expectedNumSymbols, numSymbols)
		}

		numDocuments, _, err := basestore.ScanFirstInt(codeIntelDB.QueryContext(ctx, "SELECT COUNT(*) FROM codeintel_scip_document_lookup_schema_versions"))
		if err != nil {
			t.Fatalf("unexpected error fetching count: %s", err)
		}
		if numDocuments != expectedNumDocuments {
			t.Errorf("unexpected number of documents schema version records. want=%d have=%d", expectedNumDocuments, numDocuments)
		}
	}

	// Insert records backed by a live source
	if _, err := codeIntelDB.ExecContext(ctx, `
		INSERT INTO codeintel_scip_metadata (upload_id, tool_name, tool_version, tool_arguments, text_document_encoding, protocol_version)
		VALUES
			(100, '', '', '{}', '', 1),
			(102, '', '', '{}', '', 1),
			(104, '', '', '{}', '', 1),
			(200, '', '', '{}', '', 1),
			(202, '', '', '{}', '', 1),
			(204, '', '', '{}', '', 1),
			(206, '', '', '{}', '', 1);

		INSERT INTO codeintel_scip_symbols_schema_versions (upload_id, min_schema_version, max_schema_version) VALUES
			(100, 1, 1), -- live
			(101, 1, 1), -- abandoned
			(102, 1, 1), -- live
			(103, 1, 1), -- abandoned
			(104, 1, 1); -- live

		INSERT INTO codeintel_scip_document_lookup_schema_versions (upload_id, min_schema_version, max_schema_version) VALUES
			(200, 1, 1), -- live
			(201, 1, 1), -- abandoned
			(202, 1, 1), -- live
			(203, 1, 1), -- abandoned
			(204, 1, 1), -- live
			(205, 1, 1), -- abandoned
			(206, 1, 1); -- live
	`); err != nil {
		t.Fatalf("failed to prepare data: %s", err)
	}

	// Assert test count
	assertCounts(5, 7)

	// Prune all abandoned records
	count, err := store.DeleteAbandonedSchemaVersionsRecords(ctx)
	if err != nil {
		t.Fatalf("unexpected error deleting abandoned schema version records: %s", err)
	}
	if expected := 5; count != expected {
		t.Errorf("Unexpected count. want=%d have=%d", expected, count)
	}

	// Assert count of records backed by a metadata record
	assertCounts(3, 4)
}

func TestDeleteUnreferencedDocuments(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	internalStore := basestore.NewWithHandle(codeIntelDB.Handle())
	store := New(&observation.TestContext, codeIntelDB)

	for i := range 200 {
		insertDocumentQuery := sqlf.Sprintf(
			`INSERT INTO codeintel_scip_documents(id, schema_version, payload_hash, raw_scip_payload) VALUES (%s, 1, %s, %s)`,
			i+1,
			fmt.Sprintf("hash-%d", i+1),
			fmt.Sprintf("payload-%d", i+1),
		)
		if err := internalStore.Exec(context.Background(), insertDocumentQuery); err != nil {
			t.Fatalf("unexpected error setting up database: %s", err)
		}
	}

	for i := range 200 {
		insertDocumentLookupQuery := sqlf.Sprintf(
			`INSERT INTO codeintel_scip_document_lookup(upload_id, document_path, document_id) VALUES (%s, %s, %s)`,
			42,
			fmt.Sprintf("path-%d", i+1),
			i+1,
		)
		if err := internalStore.Exec(context.Background(), insertDocumentLookupQuery); err != nil {
			t.Fatalf("unexpected error setting up database: %s", err)
		}

		if i%3 == 0 {
			insertDocumentLookupQuery := sqlf.Sprintf(
				`INSERT INTO codeintel_scip_document_lookup(upload_id, document_path, document_id) VALUES (%s, %s, %s)`,
				43,
				fmt.Sprintf("path-%d", i+1),
				i+1,
			)
			if err := internalStore.Exec(context.Background(), insertDocumentLookupQuery); err != nil {
				t.Fatalf("unexpected error setting up database: %s", err)
			}
		}
	}

	deleteReferencesQuery := sqlf.Sprintf(`DELETE FROM codeintel_scip_document_lookup WHERE upload_id = 42`)
	if err := internalStore.Exec(context.Background(), deleteReferencesQuery); err != nil {
		t.Fatalf("unexpected error setting up database: %s", err)
	}

	// Check too soon
	_, count, err := store.DeleteUnreferencedDocuments(context.Background(), 20, time.Minute, time.Now())
	if err != nil {
		t.Fatalf("unexpected error deleting unreferenced documents: %s", err)
	}
	if count != 0 {
		t.Fatalf("did not expect any expired records, have %d", count)
	}

	// Consume actual records. We expect 10 batches (200 records deleted / 20 per batch) to be required to
	// process this workload.

	totalCount := 0
	for range 10 {
		_, count, err = store.DeleteUnreferencedDocuments(context.Background(), 20, time.Minute, time.Now().Add(time.Minute*5))
		if err != nil {
			t.Fatalf("unexpected error deleting unreferenced documents: %s", err)
		}
		totalCount += count
	}
	if expected := 2 * 200 / 3; totalCount != expected {
		t.Fatalf("unexpected number of unreferenced documents deleted. want=%d have=%d", expected, totalCount)
	}

	// Assert no more records should be available for processing
	_, count, err = store.DeleteUnreferencedDocuments(context.Background(), 20, time.Minute, time.Now())
	if err != nil {
		t.Fatalf("unexpected error deleting unreferenced documents: %s", err)
	}
	if count != 0 {
		t.Fatalf("did not expect any unprocessed records, have %d", count)
	}

	documentIDsQuery := sqlf.Sprintf(`SELECT id FROM codeintel_scip_documents ORDER BY id`)
	ids, err := basestore.ScanInts(internalStore.Query(context.Background(), documentIDsQuery))
	if err != nil {
		t.Fatalf("unexpected error querying document ids: %s", err)
	}

	var expectedIDs []int
	for i := range 200 {
		if i%3 == 0 {
			expectedIDs = append(expectedIDs, i+1)
		}
	}
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected remaining document identifiers (-want +got):\n%s", diff)
	}
}

func TestIDsWithMeta(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)
	ctx := context.Background()

	if _, err := codeIntelDB.ExecContext(ctx, `
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (200, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (202, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (204, 'utf8', '', '', '{}', 1);
	`); err != nil {
		t.Fatalf("unexpected error setting up test: %s", err)
	}

	candidates := []int{
		200, // exists
		201,
		203,
		204, // exists
		205,
	}
	ids, err := store.IDsWithMeta(ctx, candidates)
	if err != nil {
		t.Fatalf("failed to find upload IDs with metadata: %s", err)
	}
	expectedIDs := []int{
		200,
		204,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}
}

func TestReconcileCandidates(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)

	ctx := context.Background()
	now := time.Unix(1587396557, 0).UTC()

	if _, err := codeIntelDB.ExecContext(ctx, `
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (200, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (201, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (202, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (203, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (204, 'utf8', '', '', '{}', 1);
		INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version) VALUES (205, 'utf8', '', '', '{}', 1);
	`); err != nil {
		t.Fatalf("unexpected error setting up test: %s", err)
	}

	// Initial batch of records
	ids, err := store.ReconcileCandidatesWithTime(ctx, 4, now)
	if err != nil {
		t.Fatalf("failed to get candidate IDs for reconciliation: %s", err)
	}
	expectedIDs := []int{
		200,
		201,
		202,
		203,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}

	// Wraps around after exhausting first records
	ids, err = store.ReconcileCandidatesWithTime(ctx, 4, now.Add(time.Minute*1))
	if err != nil {
		t.Fatalf("failed to get candidate IDs for reconciliation: %s", err)
	}
	expectedIDs = []int{
		200,
		201,
		204,
		205,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}

	// Continues to wrap around
	ids, err = store.ReconcileCandidatesWithTime(ctx, 2, now.Add(time.Minute*2))
	if err != nil {
		t.Fatalf("failed to get candidate IDs for reconciliation: %s", err)
	}
	expectedIDs = []int{
		202,
		203,
	}
	sort.Ints(ids)
	if diff := cmp.Diff(expectedIDs, ids); diff != "" {
		t.Fatalf("unexpected IDs (-want +got):\n%s", diff)
	}
}
