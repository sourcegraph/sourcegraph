package insights

import (
	"context"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

type recordingTimesMigrator struct {
	store *basestore.Store

	batchSize int
}

func NewRecordingTimesMigrator(store *basestore.Store) *recordingTimesMigrator {
	return &recordingTimesMigrator{
		store:     store,
		batchSize: 500,
	}
}

var _ oobmigration.Migrator = &recordingTimesMigrator{}

func (m *recordingTimesMigrator) ID() int                 { return 17 }
func (m *recordingTimesMigrator) Interval() time.Duration { return time.Second * 10 }

func (m *recordingTimesMigrator) Progress(ctx context.Context, _ bool) (float64, error) {
	progress, _, err := basestore.ScanFirstFloat(m.store.Query(ctx, sqlf.Sprintf(`
		SELECT
			CASE c2.count WHEN 0 THEN 1 ELSE
				cast(c1.count as float) / cast(c2.count as float)
			END
		FROM
			(SELECT count(*) as count FROM insight_series WHERE supports_augmentation IS TRUE) c1,
			(SELECT count(*) as count FROM insight_series) c2
	`)))
	return progress, err
}

func (m *recordingTimesMigrator) Up(ctx context.Context) (err error) {
	tx, err := m.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	rows, err := tx.Query(ctx, sqlf.Sprintf(
		"SELECT id, created_at, last_recorded_at, sample_interval_unit, sample_interval_value FROM insight_series WHERE supports_augmentation IS FALSE LIMIT %s FOR UPDATE SKIP LOCKED",
		m.batchSize,
	))
	if err != nil {
		return err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	series := make(map[int]seriesMetadata) // id -> metadata
	for rows.Next() {
		var id int
		var createdAt, lastRecordedAt time.Time
		var sampleIntervalUnit string
		var sampleIntervalValue int
		if err := rows.Scan(
			&id,
			&createdAt,
			&lastRecordedAt,
			&sampleIntervalUnit,
			&sampleIntervalValue,
		); err != nil {
			return err
		}
		series[id] = seriesMetadata{
			id:             id,
			createdAt:      createdAt,
			lastRecordedAt: lastRecordedAt,
			interval: timeInterval{
				unit:  intervalUnit(sampleIntervalUnit),
				value: sampleIntervalValue,
			},
		}
	}

	for id, metadata := range series {
		recordingTimesRows, err := tx.Query(ctx, sqlf.Sprintf(
			"SELECT DISTINCT recording_time FROM insight_series_recording_times WHERE insight_series_id = %s", id,
		))
		if err != nil {
			return err
		}
		defer func() { err = basestore.CloseRows(recordingTimesRows, err) }()
		var recordingTimes []time.Time
		for rows.Next() {
			var record time.Time
			if err := recordingTimesRows.Scan(&record); err != nil {
				return err
			}
			recordingTimes = append(recordingTimes, record)
		}
		metadata.existingTimes = recordingTimes
	}

	// using the inserter is probably the most efficient way however it creates a dependency so commenting out for now
	// inserter := batch.NewInserterWithConflict(ctx, tx.Handle(), "insight_series_recording_times", batch.MaxNumPostgresParameters, "ON CONFLICT DO NOTHING", "insight_series_id", "recording_time", "snapshot")
	for id, metadata := range series {
		calculatedTimes := calculateRecordingTimes(metadata.createdAt, metadata.lastRecordedAt, metadata.interval, metadata.existingTimes)
		for _, recordTime := range calculatedTimes {
			if err := tx.Exec(ctx, sqlf.Sprintf(
				"INSERT INTO insight_series_recording_times (insight_series_id, recording_time, snapshot) VALUES(%s, %s, false) ON CONFLICT DO NOTHING",
				id,
				recordTime.UTC(),
			)); err != nil {
				return err
			}
		}
		if err := tx.Exec(ctx, sqlf.Sprintf(
			"UPDATE insight_series SET supports_augmentation = TRUE WHERE id = %s",
			id,
		)); err != nil {
			return err
		}
	}

	return nil
}

type seriesMetadata struct {
	id             int
	createdAt      time.Time
	lastRecordedAt time.Time
	interval       timeInterval
	existingTimes  []time.Time
}

func (m *recordingTimesMigrator) Down(ctx context.Context) error {
	// Drop recording times for migrated records
	if err := m.store.Exec(ctx, sqlf.Sprintf(
		"DELETE FROM insight_series_recording_times WHERE insight_series_id IN (SELECT id FROM insight_series WHERE supports_augmentation = TRUE LIMIT %s)",
		m.batchSize,
	)); err != nil {
		return err
	}

	tx, err := m.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()
	// Records no longer support augmentation
	if err := m.store.Exec(ctx, sqlf.Sprintf(
		"UPDATE insight_series SET supports_augmentation = FALSE"),
	); err != nil {
		return err
	}

	return nil
}
