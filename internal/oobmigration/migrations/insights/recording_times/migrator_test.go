package recording_times

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	edb "github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
)

func TestRecordingTimesMigrator(t *testing.T) {
	t.Setenv("DISABLE_CODE_INSIGHTS", "")

	logger := logtest.Scoped(t)
	insightsDB := edb.NewInsightsDB(dbtest.NewInsightsDB(logger, t), logger)

	insightsStore := basestore.NewWithHandle(insightsDB.Handle())

	migrator := NewRecordingTimesMigrator(insightsStore, 500)

	assertProgress := func(expectedProgress float64) {
		if progress, err := migrator.Progress(context.Background(), false); err != nil {
			t.Fatalf("unexpected error querying progress: %s", err)
		} else if progress != expectedProgress {
			t.Errorf("unexpected progress. want=%.2f have=%.2f", expectedProgress, progress)
		}
	}

	assertNumberOfRecordingTimes := func(expectedCount int) {
		query := sqlf.Sprintf(`SELECT count(*) FROM insight_series_recording_times;`)

		numberOfRecordings, _, err := basestore.ScanFirstInt(migrator.store.Query(context.Background(), query))
		if err != nil {
			t.Fatalf("encountered error fetching recording times count: %v", err)
		} else if expectedCount != numberOfRecordings {
			t.Errorf("unexpected counts, want %v got %v", expectedCount, numberOfRecordings)
		}
	}

	numSeries := 1000
	for i := range numSeries {
		if err := migrator.store.Exec(context.Background(), sqlf.Sprintf(
			`INSERT INTO insight_series (series_id, query, generation_method, supports_augmentation, created_at, last_recorded_at, sample_interval_unit, sample_interval_value)
             VALUES (%s, 'query', 'search', FALSE, %s, %s, %s, %s)`,
			fmt.Sprintf("series-%d", i),
			time.Date(2022, 11, 9, 12, 1, 0, 0, time.UTC),
			time.Time{},
			hour,
			2,
		)); err != nil {
			t.Fatalf("unexpected error inserting series data: %s", err)
		}
	}

	assertProgress(0)

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("unexpected error performing up migration: %s", err)
	}
	assertProgress(0.5)

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("unexpected error performing up migration: %s", err)
	}
	assertProgress(1)

	assertNumberOfRecordingTimes(numSeries * 12)

	if err := migrator.Down(context.Background()); err != nil {
		t.Fatalf("unexpected error performing down migration: %s", err)
	}
	assertProgress(0.5)

	if err := migrator.Down(context.Background()); err != nil {
		t.Fatalf("unexpected error performing down migration: %s", err)
	}
	assertProgress(0)
}
