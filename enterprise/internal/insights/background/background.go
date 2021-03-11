package background

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/store"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
)

// StartBackgroundJobs is the main entrypoint which starts background jobs for code insights. It is
// called from the repo-updater service, currently.
func StartBackgroundJobs(ctx context.Context, mainAppDB *sql.DB) {
	if !insights.IsEnabled() {
		return
	}

	// Create a connection to TimescaleDB, so we can record results.
	timescale, err := insights.InitializeCodeInsightsDB()
	if err != nil {
		// e.g. migration failed, DB unavailable, etc. code insights is non-functional so we do not
		// want to continue.
		//
		// In some situations (i.e. if the frontend is running migrations), this will be expected
		// and we should restart until the frontend finishes - the exact same way repo updater would
		// behave if the frontend had not yet migrated the main app DB.
		log.Fatal("failed to initialize code insights (set DISABLE_CODE_INSIGHTS=true if needed)", err)
	}
	insightsStore := store.New(timescale)

	// Create a base store to be used for storing worker state. We store this in the main app Postgres
	// DB, not the TimescaleDB (which we use only for storing insights data.)
	workerBaseStore := basestore.NewWithDB(mainAppDB, sql.TxOptions{})
	settingStore := database.Settings(mainAppDB)

	// Create basic metrics for recording information about background jobs.
	observationContext := &observation.Context{
		Logger:     log15.Root(),
		Tracer:     &trace.Tracer{Tracer: opentracing.GlobalTracer()},
		Registerer: prometheus.DefaultRegisterer,
	}
	queryRunnerWorkerMetrics, queryRunnerResetterMetrics := newWorkerMetrics(observationContext, "query_runner_worker")

	// Start background goroutines for all of our workers.
	routines := []goroutine.BackgroundRoutine{
		// Register the background goroutine which discovers and enqueues insights work.
		newInsightEnqueuer(ctx, workerBaseStore, settingStore, observationContext),

		// Register the query-runner worker and resetter, which executes search queries and records
		// results to TimescaleDB.
		queryrunner.NewWorker(ctx, workerBaseStore, insightsStore, queryRunnerWorkerMetrics),
		queryrunner.NewResetter(ctx, workerBaseStore, queryRunnerResetterMetrics),

		// TODO(slimsag): future: register another worker here for webhook querying.
	}

	// Register the background goroutine which discovers historical gaps in data and enqueues
	// work to fill them - if not disabled.
	disableHistorical, _ := strconv.ParseBool(os.Getenv("DISABLE_CODE_INSIGHTS_HISTORICAL"))
	if !disableHistorical {
		routines = append(routines, newInsightHistoricalEnqueuer(ctx, workerBaseStore, settingStore, insightsStore, observationContext))
	}

	go goroutine.MonitorBackgroundRoutines(ctx, routines...)
}

// newWorkerMetrics returns a basic set of metrics to be used for a worker and its resetter:
//
// * WorkerMetrics records worker operations & number of jobs.
// * ResetterMetrics records the number of jobs that got reset because workers timed out / took too
//   long.
//
// Individual insights workers may then _also_ want to register their own metrics, if desired, in
// their NewWorker functions.
func newWorkerMetrics(observationContext *observation.Context, workerName string) (workerutil.WorkerMetrics, dbworker.ResetterMetrics) {
	workerResets := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_insights_" + workerName + "_resets_total",
		Help: "The number of times work took too long and was reset for retry later.",
	})
	observationContext.Registerer.MustRegister(workerResets)

	workerResetFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_insights_" + workerName + "_reset_failures_total",
		Help: "The number of times work took too long so many times that retries will no longer happen.",
	})
	observationContext.Registerer.MustRegister(workerResetFailures)

	workerErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_insights_" + workerName + "_reset_errors_total",
		Help: "The number of errors that occurred during a worker job.",
	})
	observationContext.Registerer.MustRegister(workerErrors)

	workerMetrics := workerutil.NewMetrics(observationContext, "insights_"+workerName, nil)
	resetterMetrics := dbworker.ResetterMetrics{
		RecordResets:        workerResets,
		RecordResetFailures: workerResetFailures,
		Errors:              workerErrors,
	}
	return workerMetrics, resetterMetrics
}
