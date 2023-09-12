package repos

import (
	"context"
	"database/sql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

const (
	tagFamily  = "family"
	tagOwner   = "owner"
	tagID      = "id"
	tagSuccess = "success"
	tagState   = "state"
	tagReason  = "reason"
)

var (
	phabricatorUpdateTime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_repoupdater_time_last_phabricator_sync",
		Help: "The last time a comprehensive Phabricator sync finished",
	}, []string{tagID})

	lastSync = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_repoupdater_syncer_sync_last_time",
		Help: "The last time a sync finished",
	}, []string{tagFamily})

	syncStarted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_syncer_start_sync",
		Help: "A sync was started",
	}, []string{tagFamily, tagOwner})

	syncErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_syncer_sync_errors_total",
		Help: "Total number of sync errors",
	}, []string{tagFamily, tagOwner, tagReason})

	syncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "src_repoupdater_syncer_sync_duration_seconds",
		Help: "Time spent syncing",
	}, []string{tagSuccess, tagFamily})

	syncedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_syncer_synced_repos_total",
		Help: "Total number of synced repositories",
	}, []string{tagState})

	schedError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_error",
		Help: "Incremented each time we encounter an error updating a repository.",
	}, []string{"type"})

	schedLoops = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_loops",
		Help: "Incremented each time the scheduler loops.",
	})

	schedAutoFetch = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_auto_fetch",
		Help: "Incremented each time the scheduler updates a managed repository due to hitting a deadline.",
	})

	schedManualFetch = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_manual_fetch",
		Help: "Incremented each time the scheduler updates a repository due to user traffic.",
	})

	schedKnownRepos = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repoupdater_sched_known_repos",
		Help: "The number of repositories that are managed by the scheduler.",
	})

	schedUpdateQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repoupdater_sched_update_queue_length",
		Help: "The number of repositories that are currently queued for update",
	})
)

func MustRegisterMetrics(logger log.Logger, db dbutil.DB, sourcegraphDotCom bool) {
	scanCount := func(sql string) (float64, error) {
		row := db.QueryRowContext(context.Background(), sql)
		var count int64
		err := row.Scan(&count)
		if err != nil {
			return 0, err
		}
		return float64(count), nil
	}

	scanNullFloat := func(q string) (sql.NullFloat64, error) {
		row := db.QueryRowContext(context.Background(), q)
		var v sql.NullFloat64
		err := row.Scan(&v)
		return v, err
	}

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_external_services_total",
		Help: "The total number of external services added",
	}, func() float64 {
		count, err := scanCount(`
SELECT COUNT(*) FROM external_services
WHERE deleted_at IS NULL
`)
		if err != nil {
			logger.Error("Failed to get total external services", log.Error(err))
			return 0
		}
		return count
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_queued_sync_jobs_total",
		Help: "The total number of queued sync jobs",
	}, func() float64 {
		count, err := scanCount(`
SELECT COUNT(*) FROM external_service_sync_jobs WHERE state = 'queued'
`)
		if err != nil {
			logger.Error("Failed to get total queued sync jobs", log.Error(err))
			return 0
		}
		return count
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_completed_sync_jobs_total",
		Help: "The total number of completed sync jobs",
	}, func() float64 {
		count, err := scanCount(`
SELECT COUNT(*) FROM external_service_sync_jobs WHERE state = 'completed'
`)
		if err != nil {
			logger.Error("Failed to get total completed sync jobs", log.Error(err))
			return 0
		}
		return count
	})

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_errored_sync_jobs_percentage",
		Help: "The percentage of external services that have failed their most recent sync",
	}, func() float64 {
		percentage, err := scanNullFloat(`
with latest_state as (
    -- Get the most recent state per external service
    select distinct on (external_service_id) external_service_id, state
    from external_service_sync_jobs
    order by external_service_id, finished_at desc
)
select round((select cast(count(*) as float) from latest_state where state = 'errored') /
             nullif((select cast(count(*) as float) from latest_state), 0) * 100)
`)
		if err != nil {
			logger.Error("Failed to get total errored sync jobs", log.Error(err))
			return 0
		}
		if !percentage.Valid {
			return 0
		}
		return percentage.Float64
	})

	backoffQuery := `
SELECT extract(epoch from max(now() - last_sync_at))
FROM external_services AS es
WHERE deleted_at IS NULL
AND NOT cloud_default
AND last_sync_at IS NOT NULL
-- Exclude any external services that are currently syncing since it's possible they may sync for more
-- than our max backoff time.
AND NOT EXISTS(SELECT FROM external_service_sync_jobs WHERE external_service_id = es.id AND finished_at IS NULL)
`

	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_max_sync_backoff",
		Help: "The maximum number of seconds since any external service synced",
	}, func() float64 {
		seconds, err := scanNullFloat(backoffQuery)
		if err != nil {
			logger.Error("Failed to get max sync backoff", log.Error(err))
			return 0
		}
		if !seconds.Valid {
			// This can happen when no external services have been synced and they all
			// have last_sync_at as null.
			return 0
		}
		return seconds.Float64
	})

	// Count the number of repos owned by site level external services that haven't
	// been fetched in 8 hours.
	//
	// We always return zero for Sourcegraph.com because we currently have a lot of
	// repos owned by the Starburst service in this state and until that's resolved
	// it would just be noise.
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_stale_repos",
		Help: "The number of repos that haven't been fetched in at least 8 hours",
	}, func() float64 {
		if sourcegraphDotCom {
			return 0
		}

		count, err := scanCount(`
select count(*)
from gitserver_repos
where last_fetched < now() - interval '8 hours'
  and last_error != ''
  and exists(select
             from external_service_repos
                      join external_services es on external_service_repos.external_service_id = es.id
                      join repo r on external_service_repos.repo_id = r.id
             where not es.cloud_default
               and gitserver_repos.repo_id = repo_id
               and external_service_repos.user_id is null
               and external_service_repos.org_id is null
               and es.deleted_at is null
               and r.deleted_at is null
    )
`)
		if err != nil {
			logger.Error("Failed to count stale repos", log.Error(err))
			return 0
		}
		return count
	})

	// Count the number of repos that are deleted but still cloned on disk. These
	// repos are eligible to be purged.
	promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "src_repoupdater_purgeable_repos",
		Help: "The number of deleted repos that are still cloned on disk",
	}, func() float64 {
		count, err := scanCount(`
SELECT
	COALESCE(SUM(cloned), 0)
FROM
	repo_statistics
`)
		if err != nil {
			logger.Error("Failed to count purgeable repos", log.Error(err))
			return 0
		}
		return count
	})
}
