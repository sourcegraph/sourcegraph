package shared

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/auth"
	workerauthz "github.com/sourcegraph/sourcegraph/cmd/worker/internal/authz"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/batches"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/codeintel"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/codemonitors"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/codygateway"
	repoembeddings "github.com/sourcegraph/sourcegraph/cmd/worker/internal/embeddings/repo"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/encryption"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/executormultiqueue"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/executors"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/githubapps"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/gitserver"
	workerinsights "github.com/sourcegraph/sourcegraph/cmd/worker/internal/insights"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/licensecheck"
	workermigrations "github.com/sourcegraph/sourcegraph/cmd/worker/internal/migrations"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/outboundwebhooks"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/own"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/permissions"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/repostatistics"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/search"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/telemetry"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/telemetrygatewayexporter"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/webhooks"
	"github.com/sourcegraph/sourcegraph/cmd/worker/internal/zoektrepos"
	workerjob "github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers"
	srp "github.com/sourcegraph/sourcegraph/internal/authz/subrepoperms"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/versions"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/register"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/symbols"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const addr = ":3189"

type EnterpriseInit = func(db database.DB)

type namedBackgroundRoutine struct {
	Routine goroutine.BackgroundRoutine
	JobName string
}

func LoadConfig(registerEnterpriseMigrators oobmigration.RegisterMigratorsFunc) *Config {
	symbols.LoadConfig()

	registerMigrators := oobmigration.ComposeRegisterMigratorsFuncs(register.RegisterOSSMigrators, registerEnterpriseMigrators)

	builtins := map[string]workerjob.Job{
		"webhook-log-janitor":                   webhooks.NewJanitor(),
		"out-of-band-migrations":                workermigrations.NewMigrator(registerMigrators),
		"gitserver-metrics":                     gitserver.NewMetricsJob(),
		"record-encrypter":                      encryption.NewRecordEncrypterJob(),
		"repo-statistics-compactor":             repostatistics.NewCompactor(),
		"repo-statistics-resetter":              repostatistics.NewResetter(),
		"zoekt-repos-updater":                   zoektrepos.NewUpdater(),
		"outbound-webhook-sender":               outboundwebhooks.NewSender(),
		"license-check":                         licensecheck.NewJob(),
		"cody-gateway-usage-check":              codygateway.NewUsageJob(),
		"rate-limit-config":                     ratelimit.NewRateLimitConfigJob(),
		"codehost-version-syncing":              versions.NewSyncingJob(),
		"insights-job":                          workerinsights.NewInsightsJob(),
		"insights-query-runner-job":             workerinsights.NewInsightsQueryRunnerJob(),
		"insights-data-retention-job":           workerinsights.NewInsightsDataRetentionJob(),
		"batches-janitor":                       batches.NewJanitorJob(),
		"batches-scheduler":                     batches.NewSchedulerJob(),
		"batches-reconciler":                    batches.NewReconcilerJob(),
		"batches-bulk-processor":                batches.NewBulkOperationProcessorJob(),
		"batches-workspace-resolver":            batches.NewWorkspaceResolverJob(),
		"executors-janitor":                     executors.NewJanitorJob(),
		"executors-metricsserver":               executors.NewMetricsServerJob(),
		"executors-multiqueue-metrics-reporter": executormultiqueue.NewMultiqueueMetricsReporterJob(),
		"codemonitors-job":                      codemonitors.NewCodeMonitorJob(),
		"bitbucket-project-permissions":         permissions.NewBitbucketProjectPermissionsJob(),
		"permission-sync-job-cleaner":           permissions.NewPermissionSyncJobCleaner(),
		"permission-sync-job-scheduler":         permissions.NewPermissionSyncJobScheduler(),
		"export-usage-telemetry":                telemetry.NewTelemetryJob(),
		"telemetrygateway-exporter":             telemetrygatewayexporter.NewJob(),

		"codeintel-policies-repository-matcher":       codeintel.NewPoliciesRepositoryMatcherJob(),
		"codeintel-autoindexing-summary-builder":      codeintel.NewAutoindexingSummaryBuilder(),
		"codeintel-autoindexing-dependency-scheduler": codeintel.NewAutoindexingDependencySchedulerJob(),
		"codeintel-autoindexing-scheduler":            codeintel.NewAutoindexingSchedulerJob(),
		"codeintel-commitgraph-updater":               codeintel.NewCommitGraphUpdaterJob(),
		"codeintel-metrics-reporter":                  codeintel.NewMetricsReporterJob(),
		"codeintel-upload-backfiller":                 codeintel.NewUploadBackfillerJob(),
		"codeintel-upload-expirer":                    codeintel.NewUploadExpirerJob(),
		"codeintel-upload-janitor":                    codeintel.NewUploadJanitorJob(),
		"codeintel-ranking-file-reference-counter":    codeintel.NewRankingFileReferenceCounter(),
		"codeintel-uploadstore-expirer":               codeintel.NewPreciseCodeIntelUploadExpirer(),
		"codeintel-sentinel-cve-scanner":              codeintel.NewSentinelCVEScannerJob(),
		"codeintel-package-filter-applicator":         codeintel.NewPackagesFilterApplicatorJob(),

		"auth-sourcegraph-operator-cleaner": auth.NewSourcegraphOperatorCleaner(),

		"repo-embedding-janitor":   repoembeddings.NewRepoEmbeddingJanitorJob(),
		"repo-embedding-job":       repoembeddings.NewRepoEmbeddingJob(),
		"repo-embedding-scheduler": repoembeddings.NewRepoEmbeddingSchedulerJob(),

		"own-repo-indexing-queue": own.NewOwnRepoIndexingQueue(),

		"github-apps-installation-validation-job": githubapps.NewGitHubApsInstallationJob(),

		"exhaustive-search-job": search.NewSearchJob(),

		"repo-perms-syncer": workerauthz.NewPermsSyncerJob(),
	}

	var config Config
	config.Jobs = map[string]workerjob.Job{}

	for name, job := range builtins {
		config.Jobs[name] = job
	}

	// Setup environment variables
	loadConfigs(config.Jobs)

	// Validate environment variables
	if err := validateConfigs(config.Jobs); err != nil {
		config.AddError(err)
	}

	return &config
}

// Start runs the worker.
func Start(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, config *Config) error {
	if err := keyring.Init(ctx); err != nil {
		return errors.Wrap(err, "initializing keyring")
	}

	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return errors.Wrap(err, "failed to create database connection")
	}

	authz.DefaultSubRepoPermsChecker = srp.NewSubRepoPermsClient(db.SubRepoPerms())

	// Emit metrics to help site admins detect instances that accidentally
	// omit a job from from the instance's deployment configuration.
	emitJobCountMetrics(config.Jobs)

	// Create the background routines that the worker will monitor for its
	// lifetime. There may be a non-trivial startup time on this step as we
	// connect to external databases, wait for migrations, etc.
	allRoutinesWithJobNames, err := createBackgroundRoutines(observationCtx, config.Jobs)
	if err != nil {
		return err
	}

	// Initialize health server
	server := httpserver.NewFromAddr(addr, &http.Server{
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Handler:      httpserver.NewHandler(nil),
	})
	serverRoutineWithJobName := namedBackgroundRoutine{Routine: server, JobName: "health-server"}
	allRoutinesWithJobNames = append(allRoutinesWithJobNames, serverRoutineWithJobName)

	// Register recorder in all routines that support it
	recorderCache := recorder.GetCache()
	rec := recorder.New(observationCtx.Logger, env.MyName, recorderCache)
	for _, rj := range allRoutinesWithJobNames {
		if recordable, ok := rj.Routine.(recorder.Recordable); ok {
			recordable.SetJobName(rj.JobName)
			recordable.RegisterRecorder(rec)
			rec.Register(recordable)
		}
	}
	rec.RegistrationDone()

	// We're all set up now
	// Respond positively to ready checks
	ready()

	// This method blocks while the app is live - the following return is only to appease
	// the type checker.
	allRoutines := make([]goroutine.BackgroundRoutine, 0, len(allRoutinesWithJobNames))
	for _, r := range allRoutinesWithJobNames {
		allRoutines = append(allRoutines, r.Routine)
	}

	goroutine.MonitorBackgroundRoutines(ctx, allRoutines...)
	return nil
}

// loadConfigs calls Load on the configs of each of the jobs registered in this binary.
// All configs will be loaded regardless if they would later be validated - this is the
// best place we have to manipulate the environment before the call to env.Lock.
func loadConfigs(jobs map[string]workerjob.Job) {
	// Load the worker config
	config.names = jobNames(jobs)
	config.Load()

	// Load all other registered configs
	for _, j := range jobs {
		for _, c := range j.Config() {
			c.Load()
		}
	}
}

// validateConfigs calls Validate on the configs of each of the jobs that will be run
// by this instance of the worker. If any config has a validation error, an error is
// returned.
func validateConfigs(jobs map[string]workerjob.Job) error {
	validationErrors := map[string][]error{}
	if err := config.Validate(); err != nil {
		return errors.Wrap(err, "Failed to load configuration")
	}

	if len(validationErrors) == 0 {
		// If the worker config is valid, validate the children configs. We guard this
		// in the case of worker config errors because we don't want to spew validation
		// errors for things that should be disabled.
		for name, job := range jobs {
			if !shouldRunJob(name) {
				continue
			}

			for _, c := range job.Config() {
				if err := c.Validate(); err != nil {
					validationErrors[name] = append(validationErrors[name], err)
				}
			}
		}
	}

	if len(validationErrors) != 0 {
		var descriptions []string
		for name, errs := range validationErrors {
			for _, err := range errs {
				descriptions = append(descriptions, fmt.Sprintf("  - %s: %s ", name, err))
			}
		}
		sort.Strings(descriptions)

		return errors.Newf("Failed to load configuration:\n%s", strings.Join(descriptions, "\n"))
	}

	return nil
}

// emitJobCountMetrics registers and emits an initial value for gauges referencing each of
// the jobs that will be run by this instance of the worker. Since these metrics are summed
// over all instances (and we don't change the jobs that are registered to a running worker),
// we only need to emit an initial count once.
func emitJobCountMetrics(jobs map[string]workerjob.Job) {
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_worker_jobs",
		Help: "Total number of jobs running in the worker.",
	}, []string{"job_name"})

	prometheus.DefaultRegisterer.MustRegister(gauge)

	for name := range jobs {
		if !shouldRunJob(name) {
			continue
		}

		gauge.WithLabelValues(name).Set(1)
	}
}

// createBackgroundRoutines runs the Routines function of each of the given jobs concurrently.
// If an error occurs from any of them, a fatal log message will be emitted. Otherwise, the set
// of background routines from each job will be returned.
func createBackgroundRoutines(observationCtx *observation.Context, jobs map[string]workerjob.Job) ([]namedBackgroundRoutine, error) {
	var (
		allRoutinesWithJobNames []namedBackgroundRoutine
		descriptions            []string
	)

	for result := range runRoutinesConcurrently(observationCtx, jobs) {
		if result.err == nil {
			allRoutinesWithJobNames = append(allRoutinesWithJobNames, result.routinesWithJobNames...)
		} else {
			descriptions = append(descriptions, fmt.Sprintf("  - %s: %s", result.name, result.err))
		}
	}
	sort.Strings(descriptions)

	if len(descriptions) != 0 {
		return nil, errors.Newf("Failed to initialize worker:\n%s", strings.Join(descriptions, "\n"))
	}

	return allRoutinesWithJobNames, nil
}

type routinesResult struct {
	name                 string
	routinesWithJobNames []namedBackgroundRoutine
	err                  error
}

// runRoutinesConcurrently returns a channel that will be populated with the return value of
// the Routines function from each given job. Each function is called concurrently. If an
// error occurs in one function, the context passed to all its siblings will be canceled.
func runRoutinesConcurrently(observationCtx *observation.Context, jobs map[string]workerjob.Job) chan routinesResult {
	results := make(chan routinesResult, len(jobs))
	defer close(results)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, name := range jobNames(jobs) {
		jobLogger := observationCtx.Logger.Scoped(name)
		observationCtx := observation.ContextWithLogger(jobLogger, observationCtx)

		if !shouldRunJob(name) {
			jobLogger.Debug("Skipping job")
			continue
		}

		wg.Add(1)
		jobLogger.Debug("Running job")

		go func() {
			defer wg.Done()

			routines, err := jobs[name].Routines(ctx, observationCtx)
			routinesWithJobNames := make([]namedBackgroundRoutine, 0, len(routines))
			for _, r := range routines {
				routinesWithJobNames = append(routinesWithJobNames, namedBackgroundRoutine{
					Routine: r,
					JobName: name,
				})
			}
			results <- routinesResult{name, routinesWithJobNames, err}

			if err == nil {
				jobLogger.Debug("Finished initializing job")
			} else {
				cancel()
			}
		}()
	}

	wg.Wait()
	return results
}

// jobNames returns an ordered slice of keys from the given map.
func jobNames(jobs map[string]workerjob.Job) []string {
	names := make([]string, 0, len(jobs))
	for name := range jobs {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// SetAuthzProviders waits for the database to be initialized, then periodically refreshes the
// global authz providers. This changes the repositories that are visible for reads based on the
// current actor stored in an operation's context, which is likely an internal actor for many of
// the jobs configured in this service. This also enables repository update operations to fetch
// permissions from code hosts.
func setAuthzProviders(ctx context.Context, observationCtx *observation.Context) {
	observationCtx = observation.ContextWithLogger(observationCtx.Logger.Scoped("authz-provider"), observationCtx)
	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return
	}

	for range time.NewTicker(providers.RefreshInterval()).C {
		allowAccessByDefault, authzProviders, _, _, _ := providers.ProvidersFromConfig(ctx, conf.Get(), db)
		authz.SetProviders(allowAccessByDefault, authzProviders)
	}
}
