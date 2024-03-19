package backfillv2

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/derision-test/glock"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type backfillerv2Migrator struct {
	store     *basestore.Store
	clock     glock.Clock
	batchSize int
}

func NewMigrator(store *basestore.Store, clock glock.Clock, batchSize int) *backfillerv2Migrator {
	return &backfillerv2Migrator{
		store:     store,
		batchSize: batchSize,
		clock:     clock,
	}
}

var _ oobmigration.Migrator = &backfillerv2Migrator{}

func (m *backfillerv2Migrator) ID() int                 { return 18 }
func (m *backfillerv2Migrator) Interval() time.Duration { return time.Second * 10 }

func (m *backfillerv2Migrator) Progress(ctx context.Context, _ bool) (float64, error) {
	if !insightsIsEnabled() {
		return 1, nil
	}
	progress, _, err := basestore.ScanFirstFloat(m.store.Query(ctx, sqlf.Sprintf(`
		SELECT
			CASE c2.count WHEN 0 THEN 1 ELSE
				cast(c1.count as float) / cast(c2.count as float)
			END
		FROM
			(SELECT count(*) as count FROM insight_series s LEFT JOIN insight_series_backfill isb on s.id = isb.series_id WHERE isb.id IS NOT NULL AND generation_method NOT IN ('language-stats', 'mapping-compute')) c1,
			(SELECT count(*) as count FROM insight_series WHERE generation_method NOT IN ('language-stats', 'mapping-compute')) c2
	`)))
	return progress, err
}

// backfillSeries contains only the fields of insight_series_backfill we care about.
type backfillSeries struct {
	id               int
	seriesID         string
	interval         timeInterval
	justInTime       bool
	backfillQueuedAt *time.Time
}

func (m *backfillerv2Migrator) Up(ctx context.Context) (err error) {
	if !insightsIsEnabled() {
		return nil
	}
	tx, err := m.store.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	toMigrate, err := selectBackfillMigrationSeries(ctx, tx, m.batchSize)
	if err != nil {
		return errors.Wrap(err, "selectBackfillMigrationSeries")
	}

	for i := range len(toMigrate) {
		series := toMigrate[i]
		err = m.migrateSeries(ctx, tx, series)
		if err != nil {
			return err
		}
	}
	err = nil
	return err
}

func (m *backfillerv2Migrator) migrateSeries(ctx context.Context, tx *basestore.Store, series *backfillSeries) (err error) {
	if series.justInTime {
		return m.migrateJIT(ctx, tx, series)
	} else if series.backfillQueuedAt != nil {
		return m.migrateBackfilledQueued(ctx, tx, series)
	} else {
		return m.migrateNotBackfillQueued(ctx, tx, series)
	}
}

func (m *backfillerv2Migrator) migrateJIT(ctx context.Context, tx *basestore.Store, series *backfillSeries) (err error) {
	if err := tx.Exec(ctx, sqlf.Sprintf(
		`with new_backfill as (
			INSERT INTO insight_series_backfill (series_id, state) VALUES(%d, 'new') returning id
		)
		INSERT INTO insights_background_jobs(backfill_id)
			SELECT id
			FROM new_backfill`,
		series.id,
	)); err != nil {
		return err
	}
	now := m.clock.Now().UTC()
	nextRecording := timeInterval.StepForwards(series.interval, now)
	nextSnapshotAfter := nextSnapshot(now)
	if err := tx.Exec(ctx, sqlf.Sprintf(`
		UPDATE insight_series set
			backfill_queued_at = %s,
			created_at=%s,
			next_recording_after = %s,
			next_snapshot_after = %s,
			just_in_time = false,
			needs_migration = false
		WHERE id = %d`,
		now,
		now,
		nextRecording.UTC(),
		nextSnapshotAfter.UTC(),
		series.id,
	)); err != nil {
		return err
	}
	return nil
}

func (m *backfillerv2Migrator) migrateNotBackfillQueued(ctx context.Context, tx *basestore.Store, series *backfillSeries) (err error) {
	if err := tx.Exec(ctx, sqlf.Sprintf(
		`with new_backfill as (
			INSERT INTO insight_series_backfill (series_id, state) VALUES(%d, 'new') returning id
		)
		INSERT INTO insights_background_jobs(backfill_id)
			SELECT id
			FROM new_backfill`,
		series.id,
	)); err != nil {
		return err
	}
	now := m.clock.Now().UTC()
	if err := tx.Exec(ctx, sqlf.Sprintf(`
		UPDATE insight_series set backfill_queued_at = %s
		WHERE id = %d`,
		now,
		series.id,
	)); err != nil {
		return err
	}
	return nil
}

func (m *backfillerv2Migrator) migrateBackfilledQueued(ctx context.Context, tx *basestore.Store, series *backfillSeries) (err error) {
	if err := tx.Exec(ctx, sqlf.Sprintf(
		"INSERT INTO insight_series_backfill (series_id, state) VALUES(%d,'completed')",
		series.id,
	)); err != nil {
		return err
	}
	return nil
}

func selectBackfillMigrationSeries(ctx context.Context, tx *basestore.Store, batchSize int) (toMigrate []*backfillSeries, err error) {
	rows, err := tx.Query(ctx, sqlf.Sprintf(`
		SELECT s.id, s.series_id, s.sample_interval_unit, s.sample_interval_value, s.just_in_time, s.backfill_queued_at
		FROM insight_series s
		LEFT JOIN insight_series_backfill isb on s.id = isb.series_id
		WHERE s.generation_method NOT IN ('language-stats', 'mapping-compute')
			AND isb.id IS NULL
		ORDER BY s.id
		LIMIT %s
		FOR UPDATE OF s SKIP LOCKED`,
		batchSize,
	))
	if err != nil {
		return nil, err
	}
	defer func() { err = basestore.CloseRows(rows, err) }()

	toMigrate = make([]*backfillSeries, 0, batchSize)
	for rows.Next() {
		var id int
		var seriesID string
		var backfillQueuedAt *time.Time
		var sampleIntervalUnit string
		var sampleIntervalValue int
		var justInTime bool
		if err := rows.Scan(
			&id,
			&seriesID,
			&sampleIntervalUnit,
			&sampleIntervalValue,
			&justInTime,
			&backfillQueuedAt,
		); err != nil {
			return nil, err
		}
		series := &backfillSeries{
			id:       id,
			seriesID: seriesID,
			interval: timeInterval{
				Unit:  intervalUnit(sampleIntervalUnit),
				Value: sampleIntervalValue,
			},
			justInTime:       justInTime,
			backfillQueuedAt: backfillQueuedAt,
		}
		toMigrate = append(toMigrate, series)
	}

	return
}

func (m *backfillerv2Migrator) Down(ctx context.Context) error {
	return nil
}

func insightsIsEnabled() bool {
	if v, _ := strconv.ParseBool(os.Getenv("DISABLE_CODE_INSIGHTS")); v {
		// Code insights can always be disabled. This can be a helpful escape hatch if e.g. there
		// are issues with (or connecting to) the codeinsights-db deployment and it is preventing
		// the Sourcegraph frontend or repo-updater from starting.
		//
		// It is also useful in dev environments if you do not wish to spend resources running Code
		// Insights.
		return false
	}
	if deploy.IsDeployTypeSingleDockerContainer(deploy.Type()) {
		// Code insights is not supported in single-container Docker demo deployments unless
		// explicity allowed, (for example by backend integration tests.)
		if v, _ := strconv.ParseBool(os.Getenv("ALLOW_SINGLE_DOCKER_CODE_INSIGHTS")); v {
			return true
		}
		return false
	}
	return true
}
