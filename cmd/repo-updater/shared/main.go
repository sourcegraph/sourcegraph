package shared

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/phabricator"
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/purge"
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/cmd/repo-updater/internal/scheduler"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/providers"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/batches/syncer"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/grpc/grpcserver"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	proto "github.com/sourcegraph/sourcegraph/internal/repoupdater/v1"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const port = "3182"

//go:embed state.html.tmpl
var stateHTMLTemplate string

type LazyDebugserverEndpoint struct {
	repoUpdaterStateEndpoint http.HandlerFunc
	manualPurgeEndpoint      http.HandlerFunc
}

func Main(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, debugserverEndpoints *LazyDebugserverEndpoint) error {
	// NOTE: Internal actor is required to have full visibility of the repo table
	// 	(i.e. bypass repository authorization).
	ctx = actor.WithInternalActor(ctx)

	logger := observationCtx.Logger

	if err := keyring.Init(ctx); err != nil {
		return errors.Wrap(err, "initializing encryption keyring")
	}

	db, err := getDB(observationCtx)
	if err != nil {
		return err
	}

	// Generally we'll mark the service as ready sometime after the database has been
	// connected; migrations may take a while and we don't want to start accepting
	// traffic until we've fully constructed the server we'll be exposing. We have a
	// bit more to do in this method, though, and the process will be marked ready
	// further down this function.

	mustRegisterMetrics(log.Scoped("MustRegisterMetrics"), db, dotcom.SourcegraphDotComMode())

	store := repos.NewStore(logger.Scoped("store"), db)
	{
		m := repos.NewStoreMetrics()
		m.MustRegister(prometheus.DefaultRegisterer)
		store.SetMetrics(m)
	}

	sourcerLogger := logger.Scoped("repos.Sourcer")
	cf := httpcli.NewExternalClientFactory(
		httpcli.NewLoggingMiddleware(sourcerLogger),
	)

	sourceMetrics := repos.NewSourceMetrics()
	sourceMetrics.MustRegister(prometheus.DefaultRegisterer)
	src := repos.NewSourcer(
		sourcerLogger,
		db,
		cf,
		gitserver.NewClient("repo-updater.sourcer"),
		repos.WithDependenciesService(dependencies.NewService(observationCtx, db)),
		repos.ObservedSource(sourcerLogger, sourceMetrics),
	)
	syncer := repos.NewSyncer(observationCtx, store, src)
	updateScheduler := scheduler.NewUpdateScheduler(logger, db, gitserver.NewClient("repos.updatescheduler"))
	server := &repoupdater.Server{
		Logger:    logger,
		Store:     store,
		Syncer:    syncer,
		Scheduler: updateScheduler,
	}

	// No Batch Changes on dotcom, so we don't need to spawn the
	// background jobs for this feature.
	if !dotcom.SourcegraphDotComMode() {
		syncRegistry := batches.InitBackgroundJobs(ctx, db, keyring.Default().BatchChangesCredentialKey, cf)
		server.ChangesetSyncRegistry = syncRegistry
	}

	go globals.WatchExternalURL()
	go watchAuthzProviders(ctx, db)
	go watchSyncer(ctx, logger, syncer, updateScheduler, server.ChangesetSyncRegistry)

	routines := []goroutine.BackgroundRoutine{
		makeGRPCServer(logger, server),
		newUnclonedReposManager(ctx, logger, dotcom.SourcegraphDotComMode(), updateScheduler, store),
		phabricator.NewRepositorySyncWorker(ctx, db, log.Scoped("PhabricatorRepositorySyncWorker"), store),
		// Run git fetches scheduler
		updateScheduler,
	}

	routines = append(routines,
		syncer.Routines(ctx, store, repos.RunOptions{
			EnqueueInterval: conf.RepoListUpdateInterval,
			IsDotCom:        dotcom.SourcegraphDotComMode(),
			MinSyncInterval: conf.RepoListUpdateInterval,
		})...,
	)

	if server.ChangesetSyncRegistry != nil {
		routines = append(routines, server.ChangesetSyncRegistry)
	}

	// git-server repos purging thread
	// Temporary escape hatch if this feature proves to be dangerous
	// TODO: Move to config.
	if disabled, _ := strconv.ParseBool(os.Getenv("DISABLE_REPO_PURGE")); disabled {
		logger.Info("repository purger is disabled via env DISABLE_REPO_PURGE")
	} else {
		routines = append(routines, purge.NewRepositoryPurgeWorker(ctx, log.Scoped("repoPurgeWorker"), db, conf.DefaultClient()))
	}

	// Register recorder in all routines that support it.
	recorderCache := recorder.GetCache()
	rec := recorder.New(observationCtx.Logger, env.MyName, recorderCache)
	for _, r := range routines {
		if recordable, ok := r.(recorder.Recordable); ok {
			recordable.SetJobName("repo-updater")
			recordable.RegisterRecorder(rec)
			rec.Register(recordable)
		}
	}
	rec.RegistrationDone()

	debugDumpers := make(map[string]debugserver.Dumper)
	debugDumpers["repos"] = updateScheduler
	debugserverEndpoints.repoUpdaterStateEndpoint = repoUpdaterStatsHandler(debugDumpers)
	debugserverEndpoints.manualPurgeEndpoint = manualPurgeHandler(db)

	// We mark the service as ready now AFTER assigning the additional endpoints in
	// the debugserver constructed at the top of this function. This ensures we don't
	// have a race between becoming ready and a debugserver request failing directly
	// after being unblocked.
	ready()

	goroutine.MonitorBackgroundRoutines(ctx, routines...)

	return nil
}

func getDB(observationCtx *observation.Context) (database.DB, error) {
	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.PostgresDSN
	})
	sqlDB, err := connections.EnsureNewFrontendDB(observationCtx, dsn, "repo-updater")
	if err != nil {
		return nil, errors.Wrap(err, "initializing database store")
	}
	return database.NewDB(observationCtx.Logger, sqlDB), nil
}

func makeGRPCServer(logger log.Logger, server *repoupdater.Server) goroutine.BackgroundRoutine {
	host := ""
	if env.InsecureDev {
		host = "127.0.0.1"
	}

	addr := net.JoinHostPort(host, port)
	logger.Info("listening", log.String("addr", addr))

	grpcServer := grpc.NewServer(defaults.ServerOptions(logger)...)
	proto.RegisterRepoUpdaterServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	return grpcserver.NewFromAddr(logger.Scoped("repo-updater.grpcserver"), addr, grpcServer)
}

func manualPurgeHandler(db database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := strconv.Atoi(r.FormValue("limit"))
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid limit: %v", err), http.StatusBadRequest)
			return
		}
		if limit <= 0 {
			http.Error(w, "limit must be greater than 0", http.StatusBadRequest)
			return
		}
		if limit > 10000 {
			http.Error(w, "limit must be less than 10000", http.StatusBadRequest)
			return
		}
		perSecond := 1.0 // Default value
		perSecondParam := r.FormValue("perSecond")
		if perSecondParam != "" {
			perSecond, err = strconv.ParseFloat(perSecondParam, 64)
			if err != nil {
				http.Error(w, fmt.Sprintf("invalid per second rate limit: %v", err), http.StatusBadRequest)
				return
			}
			// Set a sane lower bound
			if perSecond <= 0.1 {
				http.Error(w, fmt.Sprintf("invalid per second rate limit. Must be > 0.1, got %f", perSecond), http.StatusBadRequest)
				return
			}
		}
		err = purge.PurgeOldestRepos(log.Scoped("PurgeOldestRepos"), db, limit, perSecond)
		if err != nil {
			http.Error(w, fmt.Sprintf("starting manual purge: %v", err), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "manual purge started with limit of %d and rate of %f", limit, perSecond)
	}
}

func repoUpdaterStatsHandler(debugDumpers map[string]debugserver.Dumper) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wantDumper := r.URL.Query().Get("dumper")
		wantFormat := r.URL.Query().Get("format")

		// Showing the HTML version of repository syncing schedule as the default,
		// also the only dumper that supports rendering the HTML version.
		if (wantDumper == "" || wantDumper == "repos") && wantFormat != "json" {
			reposDumper, ok := debugDumpers["repos"].(*scheduler.UpdateScheduler)
			if !ok {
				http.Error(w, "No debug dumper for repos found", http.StatusInternalServerError)
				return
			}

			// This case also applies for defaultOffer. Note that this is preferred
			// over e.g. a 406 status code, according to the MDN:
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/406
			tmpl := template.New("state.html").Funcs(template.FuncMap{
				"truncateDuration": func(d time.Duration) time.Duration {
					return d.Truncate(time.Second)
				},
			})
			template.Must(tmpl.Parse(stateHTMLTemplate))
			err := tmpl.Execute(w, reposDumper.DebugDump(r.Context()))
			if err != nil {
				http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}

		var dumps []any
		for name, dumper := range debugDumpers {
			if wantDumper != "" && wantDumper != name {
				continue
			}
			dumps = append(dumps, dumper.DebugDump(r.Context()))
		}

		p, err := json.MarshalIndent(dumps, "", "  ")
		if err != nil {
			http.Error(w, "Failed to marshal dumps: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(p)
	}
}

func watchSyncer(
	ctx context.Context,
	logger log.Logger,
	syncer *repos.Syncer,
	sched *scheduler.UpdateScheduler,
	changesetSyncer syncer.UnarchivedChangesetSyncRegistry,
) {
	logger.Debug("started new repo syncer updates scheduler relay thread")

	for {
		select {
		case <-ctx.Done():
			return
		case diff := <-syncer.Synced:
			if !conf.Get().DisableAutoGitUpdates {
				sched.UpdateFromDiff(diff)
			}

			// Similarly, changesetSyncer is only available in enterprise mode.
			if changesetSyncer != nil {
				repositories := diff.Modified.ReposModified(types.RepoModifiedArchived)
				if len(repositories) > 0 {
					if err := changesetSyncer.EnqueueChangesetSyncsForRepos(ctx, repositories.IDs()); err != nil {
						logger.Warn("error enqueuing changeset syncs for archived and unarchived repos", log.Error(err))
					}
				}
			}
		}
	}
}

// newUnclonedReposManager creates a background routine that will periodically list
// the uncloned repositories on gitserver and update the scheduler with the list.
// It also ensures that if any of our indexable repos are missing from the cloned
// list they will be added for cloning ASAP.
func newUnclonedReposManager(ctx context.Context, logger log.Logger, isSourcegraphDotCom bool, sched *scheduler.UpdateScheduler, store repos.Store) goroutine.BackgroundRoutine {
	return goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(ctx),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			// Don't modify the scheduler if we're not performing auto updates.
			if conf.Get().DisableAutoGitUpdates {
				return nil
			}

			baseRepoStore := database.ReposWith(logger, store)

			if isSourcegraphDotCom {
				// Fetch ALL indexable repos that are NOT cloned so that we can add them to the
				// scheduler.
				opts := database.ListSourcegraphDotComIndexableReposOptions{
					CloneStatus: types.CloneStatusNotCloned,
				}
				indexable, err := baseRepoStore.ListSourcegraphDotComIndexableRepos(ctx, opts)
				if err != nil {
					return errors.Wrap(err, "listing indexable repos")
				}
				// Ensure that uncloned indexable repos are known to the scheduler
				sched.EnsureScheduled(indexable)
			}

			// Next, move any repos managed by the scheduler that are uncloned to the front
			// of the queue.
			managed := sched.ListRepoIDs()

			uncloned, err := baseRepoStore.ListMinimalRepos(ctx, database.ReposListOptions{IDs: managed, NoCloned: true})
			if err != nil {
				return errors.Wrap(err, "failed to fetch list of uncloned repositories")
			}

			sched.PrioritiseUncloned(uncloned)

			return nil
		}),
		goroutine.WithName("repo-updater.uncloned-repo-manager"),
		goroutine.WithDescription("periodically lists uncloned repos and schedules them as high priority in the repo updater update queue"),
		goroutine.WithInterval(30*time.Second),
	)
}

// TODO: This might clash with what osscmd.Main does.
// watchAuthzProviders updates authz providers if config changes.
func watchAuthzProviders(ctx context.Context, db database.DB) {
	go func() {
		t := time.NewTicker(providers.RefreshInterval())
		for range t.C {
			allowAccessByDefault, authzProviders, _, _, _ := providers.ProvidersFromConfig(
				ctx,
				conf.Get(),
				db,
			)
			authz.SetProviders(allowAccessByDefault, authzProviders)
		}
	}()
}

func mustRegisterMetrics(logger log.Logger, db dbutil.DB, sourcegraphDotCom bool) {
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
