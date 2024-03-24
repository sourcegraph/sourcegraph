package partitions

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lib/pq"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
)

func TestPartitionList(t *testing.T) {
	createTableQuery := `
		CREATE TABLE partition_test (
			id       SERIAL,
			type     TEXT NOT NULL,
			payload  TEXT NOT NULL,
			created  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		) PARTITION BY LIST (type)
	`

	testPartition(t, NewListPartitionStrategy[String](), createTableQuery, []KeyValues[ListPartitionKey[String]]{
		{key: ListPartitionKey[String]{"foo"}, payloads: []Payload{{"foo", "PAYLOAD_23904802938409"}, {"foo", "PAYLOAD_23094903902304"}, {"foo", "PAYLOAD_90949020984820"}}},
		{key: ListPartitionKey[String]{"bar"}, payloads: []Payload{{"bar", "PAYLOAD_98092389028904"}, {"bar", "PAYLOAD_29302808329809"}, {"bar", "PAYLOAD_24804209840932"}}},
		{key: ListPartitionKey[String]{"baz"}, payloads: []Payload{{"baz", "PAYLOAD_09820398920948"}, {"baz", "PAYLOAD_09832482309490"}, {"baz", "PAYLOAD_23094802374072"}}},
	})
}

func TestPartitionRange(t *testing.T) {
	createTableQuery := `
		CREATE TABLE partition_test (
			id       SERIAL,
			type     TEXT NOT NULL,
			payload  TEXT NOT NULL,
			created  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		) PARTITION BY RANGE (type)
	`

	testPartition(t, NewRangePartitionStrategy[String](), createTableQuery, []KeyValues[RangePartitionKey[String]]{
		{key: RangePartitionKey[String]{"faa", "gzz"}, payloads: []Payload{{"foo-1", "PAYLOAD_239048029384091"}, {"foo-2", "PAYLOAD_23094903902304"}, {"foo-3", "PAYLOAD_90949020984820"}}},
		{key: RangePartitionKey[String]{"baa", "bat"}, payloads: []Payload{{"bar-1", "PAYLOAD_980923890289041"}, {"bar-2", "PAYLOAD_29302808329809"}, {"bar-3", "PAYLOAD_24804209840932"}}},
		{key: RangePartitionKey[String]{"bav", "caa"}, payloads: []Payload{{"baz-1", "PAYLOAD_098203989209481"}, {"baz-2", "PAYLOAD_09832482309490"}, {"baz-3", "PAYLOAD_23094802374072"}}},
	})
}

func TestPartitionHash(t *testing.T) {
	// createTableQuery := `
	// 	CREATE TABLE partition_test (
	// 		id       SERIAL,
	// 		type     TEXT NOT NULL,
	// 		payload  TEXT NOT NULL,
	// 		created  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	// 	) PARTITION BY HASH (type)
	// `

	// TODO - non-deterministic :(
}

//
//

type KeyValues[PK PartitionKey] struct {
	key      PK
	payloads []Payload
}

type Payload struct {
	Type    string
	Payload string
}

func testPartition[PK PartitionKey](t *testing.T, strategy PartitionStrategy[PK], createTableQuery string, keyValues []KeyValues[PK]) {
	ctx := context.Background()
	logger := logtest.Scoped(t)
	db := dbtest.NewRawDB(logger, t)
	manager := NewPartitionManager(db, "partition_test", strategy)

	if _, err := db.ExecContext(ctx, createTableQuery); err != nil {
		t.Fatalf("failed to create root table: %s", err)
	}

	//
	// Helper insert/select functions

	insertValue := func(key, payload string) error {
		insertQuery := `INSERT INTO partition_test(type, payload) VALUES ($1, $2)`
		_, err := db.ExecContext(ctx, insertQuery, key, payload)
		return err
	}
	queryRoot := func(key PK, types []string) []string {
		query := `SELECT payload FROM partition_test WHERE type = ANY($1) ORDER BY id`
		keyPayloads, err := basestore.ScanStrings(db.QueryContext(ctx, query, pq.Array(types)))
		if err != nil {
			t.Fatalf("unexpected error query values from base table: %s", err)
		}
		return keyPayloads
	}
	queryPartition := func(key PK, types []string) []string {
		query := fmt.Sprintf(`SELECT payload FROM %s WHERE type = ANY($1) ORDER BY id`, manager.PartitionTableNameFor(key))
		partitionPayloads, err := basestore.ScanStrings(db.QueryContext(ctx, query, pq.Array(types)))
		if err != nil {
			t.Fatalf("unexpected error query values from partition table: %s", err)
		}
		return partitionPayloads
	}

	//
	// Helper assertion functions

	types := func(keyValue KeyValues[PK]) (types []string) {
		for _, payload := range keyValue.payloads {
			types = append(types, payload.Type)
		}
		return types
	}
	expected := func(keyValue KeyValues[PK], expectEmpty bool) (payloads []string) {
		if expectEmpty {
			return nil
		}

		for _, payload := range keyValue.payloads {
			payloads = append(payloads, payload.Payload)
		}
		return payloads
	}

	assertRootContents := func(expectEmpty bool) {
		for _, keyValue := range keyValues {
			if diff := cmp.Diff(expected(keyValue, expectEmpty), queryRoot(keyValue.key, types(keyValue))); diff != "" {
				t.Errorf("unexpected payloads for key %s (-want +got):\n%s", keyValue.key.Name(), diff)
			}
		}
	}
	assertPartitionContents := func(expectEmpty bool) {
		for _, keyValue := range keyValues {
			if diff := cmp.Diff(expected(keyValue, expectEmpty), queryPartition(keyValue.key, types(keyValue))); diff != "" {
				t.Errorf("unexpected payloads for partition %s (-want +got):\n%s", keyValue.key.Name(), diff)
			}
		}
	}

	//
	// Actual tests :comfy:

	t.Run("insertions fail without attached partitions", func(t *testing.T) {
		for _, keyValue := range keyValues {
			payload := keyValue.payloads[0]
			if err := insertValue(payload.Type, payload.Payload); !IsMissingPartition(err) {
				t.Fatalf("unexpected error inserting into table without partition: %s", err)
			}
		}
	})

	t.Run("insertions succeed with attached partitions", func(t *testing.T) {
		for _, keyValue := range keyValues {
			if err := manager.EnsurePartition(ctx, keyValue.key); err != nil {
				t.Fatalf("failed to ensure partition: %s", err)
			}

			for _, payload := range keyValue.payloads {
				if err := insertValue(payload.Type, payload.Payload); err != nil {
					t.Fatalf("unexpected error inserting into partition: %s", err)
				}
			}
		}

		_ = assertRootContents
		// _ = assertPartitionContents
		// t.Run("data visible in root", func(t *testing.T) { assertRootContents(false) })
		t.Run("data visible in partitions", func(t *testing.T) { assertPartitionContents(false) })
	})

	// t.Run("detach partition", func(t *testing.T) {
	// 	for _, keyValue := range keyValues {
	// 		if err := manager.DetachPartition(ctx, keyValue.key); err != nil {
	// 			t.Fatalf("failed to detach partition: %s", err)
	// 		}
	// 	}

	// 	// gone
	// 	t.Run("no data visible in root", func(t *testing.T) { assertRootContents(true) })
	// 	t.Run("data visible in partitions", func(t *testing.T) { assertPartitionContents(false) })
	// })

	// t.Run("re-attach partition", func(t *testing.T) {
	// 	for _, keyValue := range keyValues {
	// 		if err := manager.AttachPartition(ctx, keyValue.key); err != nil {
	// 			t.Fatalf("failed to attach partition: %s", err)
	// 		}
	// 	}

	// 	// restored
	// 	t.Run("data visible in root", func(t *testing.T) { assertRootContents(false) })
	// 	t.Run("data visible in partitions", func(t *testing.T) { assertPartitionContents(false) })
	// })

	// t.Run("delete partition", func(t *testing.T) {
	// 	for _, keyValue := range keyValues {
	// 		if err := manager.DeletePartition(ctx, keyValue.key); err != nil {
	// 			t.Fatalf("failed to delete partition: %s", err)
	// 		}
	// 	}

	// 	// gone
	// 	t.Run("data visible in root", func(t *testing.T) { assertRootContents(true) })

	// 	// assert tables are dropped
	// 	for _, keyValue := range keyValues {
	// 		if err := manager.AttachPartition(ctx, keyValue.key); err == nil {
	// 			t.Fatalf("expected an error when attaching missing partition")
	// 		}
	// 	}
	// })
}
