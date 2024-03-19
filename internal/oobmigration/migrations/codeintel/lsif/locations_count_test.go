package lsif

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
)

func TestLocationsCountMigrator(t *testing.T) {
	logger := logtest.Scoped(t)
	rawDB := lastDBWithLSIF(logger, t)
	db := database.NewDB(logger, rawDB)
	store := basestore.NewWithHandle(db.Handle())
	migrator := newLocationsCountMigrator(store, 10, time.Second, "lsif_data_definitions", 250, 1)
	serializer := newSerializer()

	assertProgress := func(expectedProgress float64, applyReverse bool) {
		if progress, err := migrator.Progress(context.Background(), applyReverse); err != nil {
			t.Fatalf("unexpected error querying progress: %s", err)
		} else if progress != expectedProgress {
			t.Errorf("unexpected progress. want=%.2f have=%.2f", expectedProgress, progress)
		}
	}

	assertCounts := func(expectedCounts []int) {
		query := sqlf.Sprintf(`SELECT num_locations FROM lsif_data_definitions ORDER BY scheme, identifier`)

		if counts, err := basestore.ScanInts(store.Query(context.Background(), query)); err != nil {
			t.Fatalf("unexpected error querying num diagnostics: %s", err)
		} else if diff := cmp.Diff(expectedCounts, counts); diff != "" {
			t.Errorf("unexpected counts (-want +got):\n%s", diff)
		}
	}

	n := 500
	expectedCounts := make([]int, 0, n)
	locations := make([]LocationData, 0, n)

	for i := range n {
		expectedCounts = append(expectedCounts, i+1)
		locations = append(locations, LocationData{URI: fmt.Sprintf("file://%d", i)})

		data, err := serializer.MarshalLocations(locations)
		if err != nil {
			t.Fatalf("unexpected error serializing locations: %s", err)
		}

		if err := store.Exec(context.Background(), sqlf.Sprintf(
			"INSERT INTO lsif_data_definitions (dump_id, scheme, identifier, data, schema_version, num_locations) VALUES (%s, %s, %s, %s, 1, 0)",
			42+i/(n/2), // 50% id=42, 50% id=43
			fmt.Sprintf("s%04d", i),
			fmt.Sprintf("i%04d", i),
			data,
		)); err != nil {
			t.Fatalf("unexpected error inserting row: %s", err)
		}
	}

	assertProgress(0, false)

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("unexpected error performing up migration: %s", err)
	}
	assertProgress(0.5, false)

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("unexpected error performing up migration: %s", err)
	}
	assertProgress(1, false)

	assertCounts(expectedCounts)

	if err := migrator.Down(context.Background()); err != nil {
		t.Fatalf("unexpected error performing down migration: %s", err)
	}
	assertProgress(0.5, true)

	if err := migrator.Down(context.Background()); err != nil {
		t.Fatalf("unexpected error performing down migration: %s", err)
	}
	assertProgress(0, true)
}
