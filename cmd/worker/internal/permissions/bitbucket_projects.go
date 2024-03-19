package permissions

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// bitbucketProjectPermissionsJob implements the job.Job interface. It is used by the worker service
// to spawn a new background worker.
type bitbucketProjectPermissionsJob struct{}

// NewBitbucketProjectPermissionsJob creates a new job for applying explicit permissions
// to all the repositories of a Bitbucket Project.
func NewBitbucketProjectPermissionsJob() job.Job {
	return &bitbucketProjectPermissionsJob{}
}

func (j *bitbucketProjectPermissionsJob) Description() string {
	return "Applies explicit permissions to all repositories of a Bitbucket Project."
}

func (j *bitbucketProjectPermissionsJob) Config() []env.Config {
	return []env.Config{ConfigInst}
}

// Routines is called by the worker service to start the worker.
// It returns a list of goroutines that the worker service should start and manage.
func (j *bitbucketProjectPermissionsJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, err
	}

	bbProjectMetrics := newMetricsForBitbucketProjectPermissionsQueries(observationCtx.Logger)

	rootContext := actor.WithInternalActor(context.Background())

	return []goroutine.BackgroundRoutine{
		newBitbucketProjectPermissionsWorker(rootContext, observationCtx, db, ConfigInst, bbProjectMetrics),
		newBitbucketProjectPermissionsResetter(observationCtx, db, ConfigInst, bbProjectMetrics),
	}, nil
}

// bitbucketProjectPermissionsHandler handles the execution of a single explicit_permissions_bitbucket_projects_jobs record.
type bitbucketProjectPermissionsHandler struct {
	db     database.DB
	client *bitbucketserver.Client
}

// Handle implements the workerutil.Handler interface.
func (h *bitbucketProjectPermissionsHandler) Handle(ctx context.Context, logger log.Logger, workerJob *types.BitbucketProjectPermissionJob) (err error) {
	logger = logger.Scoped("BitbucketProjectPermissionsHandler")
	defer func() {
		if err != nil {
			logger.Error("Handle", log.Error(err))
		}
	}()

	// get the external service
	svc, err := h.db.ExternalServices().GetByID(ctx, workerJob.ExternalServiceID)
	if err != nil {
		return errcode.MakeNonRetryable(errors.Wrapf(err, "failed to get external service %d", workerJob.ExternalServiceID))
	}

	if svc.Kind != extsvc.KindBitbucketServer {
		return errcode.MakeNonRetryable(errors.Newf("expected Bitbucket Server external service, got: %s", svc.Kind))
	}

	// get repos from the Bitbucket project
	client, err := h.getBitbucketClient(ctx, logger, svc)
	if err != nil {
		return errors.Wrapf(err, "failed to build Bitbucket client for external service %d", svc.ID)
	}

	projectKey := workerJob.ProjectKey

	// These repos are fetched from Bitbucket, therefore their IDs are Bitbucket IDs
	// and we need to search for these repos in frontend DB to get Sourcegraph internal IDs
	bitbucketRepos, err := client.ProjectRepos(ctx, projectKey)
	if err != nil {
		return errors.Wrapf(err, "failed to list repositories of Bitbucket Project %q", projectKey)
	}

	repoIDs, err := h.getRepoIDsByNames(ctx, svc, bitbucketRepos)
	if err != nil {
		return errors.Wrap(err, "failed to get gitserver repos from the database")
	}

	if workerJob.Unrestricted {
		return h.setReposUnrestricted(ctx, logger, repoIDs, projectKey)
	}

	err = h.setPermissionsForUsers(ctx, logger, workerJob.Permissions, repoIDs, projectKey)
	if err != nil {
		return errors.Wrapf(err, "failed to set permissions for Bitbucket Project %q", projectKey)
	}

	return nil
}

// getBitbucketClient creates a Bitbucket client for the given external service.
func (h *bitbucketProjectPermissionsHandler) getBitbucketClient(ctx context.Context, logger log.Logger, svc *types.ExternalService) (*bitbucketserver.Client, error) {
	// for testing purpose
	if h.client != nil {
		return h.client, nil
	}

	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var c schema.BitbucketServerConnection
	if err := jsonc.Unmarshal(rawConfig, &c); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	var opts []httpcli.Opt
	if c.Certificate != "" {
		opts = append(opts, httpcli.NewCertPoolOpt(c.Certificate))
	}

	cli, err := httpcli.NewExternalClientFactory(httpcli.NewLoggingMiddleware(logger)).Doer(opts...)
	if err != nil {
		return nil, err
	}

	return bitbucketserver.NewClient(svc.URN(), &c, cli)
}

func (h *bitbucketProjectPermissionsHandler) setReposUnrestricted(ctx context.Context, logger log.Logger, repoIDs []api.RepoID, projectKey string) error {
	sort.Slice(repoIDs, func(i, j int) bool {
		return repoIDs[i] < repoIDs[j]
	})

	// converting api.RepoID to int32
	repoIntIDs := make([]int32, len(repoIDs))
	for i, id := range repoIDs {
		repoIntIDs[i] = int32(id)
	}

	logger.Info("Setting bitbucket repositories to unrestricted",
		log.String("project_key", projectKey),
		log.Int("repo_ids_len", len(repoIDs)),
	)

	err := h.db.Perms().SetRepoPermissionsUnrestricted(ctx, repoIntIDs, true)
	if err != nil {
		return errors.Wrapf(err, "failed to set permissions to unrestricted for Bitbucket Project %q", projectKey)
	}

	return nil
}

// getRepoIDsByNames queries repo IDs from frontend database using external repo IDs fetched from
// Bitbucket code host.
func (h *bitbucketProjectPermissionsHandler) getRepoIDsByNames(ctx context.Context, svc *types.ExternalService, repos []*bitbucketserver.Repo) ([]api.RepoID, error) {
	count := len(repos)
	IDs := make([]api.RepoID, 0, count)
	if count == 0 {
		return IDs, nil
	}

	// unmarshalling external service config
	rawConfig, err := svc.Config.Decrypt(ctx)
	if err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}
	var cfg schema.BitbucketServerConnection
	if err := jsonc.Unmarshal(rawConfig, &cfg); err != nil {
		return nil, errors.Errorf("external service id=%d config error: %s", svc.ID, err)
	}

	// parsing the hostname from the URL
	parsedURL, err := url.Parse(cfg.Url)
	if err != nil {
		return nil, errors.Errorf("error during parsing external service URL", err)
	}

	extSvcType := extsvc.KindToType(svc.Kind)
	extSvcID := extsvc.NormalizeBaseURL(parsedURL).String()
	specs := make([]api.ExternalRepoSpec, 0, count)
	for _, repo := range repos {
		// using external ID, external service type and external service ID of the repo to find it
		spec := api.ExternalRepoSpec{
			ID:          strconv.Itoa(repo.ID),
			ServiceType: extSvcType,
			ServiceID:   extSvcID,
		}

		specs = append(specs, spec)
	}

	foundRepos, err := h.db.Repos().List(ctx, database.ReposListOptions{ExternalRepos: specs})
	if err != nil {
		return nil, err
	}

	// mapping repos to repo IDs
	for _, foundRepo := range foundRepos {
		IDs = append(IDs, foundRepo.ID)
	}

	return IDs, nil
}

// setPermissionsForUsers applies user permissions to a list of repos.
// It updates the repo_permissions, user_permissions, repo_pending_permissions and user_pending_permissions table.
// Each repo is processed atomically. In case of error, the task fails but doesn't rollback the committed changes
// done on previous repos. This is fine because when the task is retried, previous repos won't incur any
// additional writes.
func (h *bitbucketProjectPermissionsHandler) setPermissionsForUsers(ctx context.Context, logger log.Logger, perms []types.UserPermission, repoIDs []api.RepoID, projectKey string) error {
	sort.Slice(perms, func(i, j int) bool {
		return perms[i].BindID < perms[j].BindID
	})
	sort.Slice(repoIDs, func(i, j int) bool {
		return repoIDs[i] < repoIDs[j]
	})

	bindIDs := make([]string, 0, len(perms))
	for _, up := range perms {
		bindIDs = append(bindIDs, up.BindID)
	}

	// bind the bindIDs to actual user IDs
	mapping, err := h.db.Perms().MapUsers(ctx, bindIDs, conf.PermissionsUserMapping())
	if err != nil {
		return errors.Wrap(err, "failed to map bind IDs to user IDs")
	}

	userIDs := make(map[int32]struct{}, len(mapping))
	for _, id := range mapping {
		userIDs[id] = struct{}{}
	}

	// determine which users don't exist yet
	pendingBindIDs := make([]string, 0, len(bindIDs))
	for _, bindID := range bindIDs {
		if _, ok := mapping[bindID]; !ok {
			pendingBindIDs = append(pendingBindIDs, bindID)
		}
	}

	logger.Info("Applying permissions to Bitbucket project repositories",
		log.String("project_key", projectKey),
		log.Int("repo_ids_len", len(repoIDs)),
		log.Int("user_ids_len", len(userIDs)),
		log.Int("pending_bind_ids_len", len(pendingBindIDs)),
	)

	// apply the permissions for each repo
	for _, repoID := range repoIDs {
		err = h.setRepoPermissions(ctx, repoID, perms, userIDs, pendingBindIDs)
		if err != nil {
			return errors.Wrapf(err, "failed to set permissions for repo %d", repoID)
		}
	}

	return nil
}

func (h *bitbucketProjectPermissionsHandler) setRepoPermissions(ctx context.Context, repoID api.RepoID, _ []types.UserPermission, userIDs map[int32]struct{}, pendingBindIDs []string) (err error) {
	// Make sure the repo ID is valid.
	if err := h.repoExists(ctx, repoID); err != nil {
		return errcode.MakeNonRetryable(errors.Wrapf(err, "failed to query repo %d", repoID))
	}

	p := authz.RepoPermissions{
		RepoID:  int32(repoID),
		Perm:    authz.Read, // Note: We currently only support read for repository permissions.
		UserIDs: userIDs,
	}

	perms := make([]authz.UserIDWithExternalAccountID, 0, len(userIDs))
	for userID := range userIDs {
		perms = append(perms, authz.UserIDWithExternalAccountID{UserID: userID})
	}

	txs, err := h.db.Perms().Transact(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}
	defer func() { err = txs.Done(err) }()

	accounts := &extsvc.Accounts{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountIDs:  pendingBindIDs,
	}

	// make sure the repo is not unrestricted
	err = txs.SetRepoPermissionsUnrestricted(ctx, []int32{int32(repoID)}, false)
	if err != nil {
		return errors.Wrapf(err, "failed to set repo %d to restricted", repoID)
	}

	// set repo permissions (and user permissions)
	if _, err = txs.SetRepoPerms(ctx, int32(repoID), perms, authz.SourceAPI); err != nil {
		return errors.Wrapf(err, "failed to set user repo permissions for repo %d and users %v", repoID, perms)
	}

	// set pending permissions
	err = txs.SetRepoPendingPermissions(ctx, accounts, &p)
	if err != nil {
		return errors.Wrapf(err, "failed to set pending permissions for repo %d", repoID)
	}

	return nil
}

func (h *bitbucketProjectPermissionsHandler) repoExists(ctx context.Context, repoID api.RepoID) (err error) {
	var id int
	if err := h.db.QueryRowContext(ctx, fmt.Sprintf("SELECT id FROM repo WHERE id = %d", repoID)).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("repo not found")
		}
		return err
	}
	return nil
}

// newBitbucketProjectPermissionsWorker creates a worker that reads the explicit_permissions_bitbucket_projects_jobs table and
// executes the jobs.
func newBitbucketProjectPermissionsWorker(ctx context.Context, observationCtx *observation.Context, db database.DB, cfg *config, metrics bitbucketProjectPermissionsMetrics) *workerutil.Worker[*types.BitbucketProjectPermissionJob] {
	observationCtx = observation.ContextWithLogger(observationCtx.Logger.Scoped("BitbucketProjectPermissionsWorker"), observationCtx)

	options := workerutil.WorkerOptions{
		Name:              "explicit_permissions_bitbucket_projects_jobs_worker",
		Description:       "syncs Bitbucket Projects via Explicit Permissions API",
		NumHandlers:       cfg.WorkerConcurrency,
		Interval:          cfg.WorkerPollInterval,
		HeartbeatInterval: 15 * time.Second,
		Metrics:           metrics.workerMetrics,
	}

	store := createBitbucketProjectPermissionsStore(observationCtx, db, cfg)

	return dbworker.NewWorker[*types.BitbucketProjectPermissionJob](ctx, store, &bitbucketProjectPermissionsHandler{db: db}, options)
}

// newBitbucketProjectPermissionsResetter implements resetter for the explicit_permissions_bitbucket_projects_jobs table.
// See resetter documentation for more details. https://sourcegraph.com/docs/dev/background-information/workers#dequeueing-and-resetting-jobs
func newBitbucketProjectPermissionsResetter(observationCtx *observation.Context, db database.DB, cfg *config, metrics bitbucketProjectPermissionsMetrics) *dbworker.Resetter[*types.BitbucketProjectPermissionJob] {
	observationCtx = observation.ContextWithLogger(observationCtx.Logger.Scoped("BitbucketProjectPermissionsResetter"), observationCtx)

	workerStore := createBitbucketProjectPermissionsStore(observationCtx, db, cfg)

	options := dbworker.ResetterOptions{
		Name:     "explicit_permissions_bitbucket_projects_jobs_worker_resetter",
		Interval: 1 * time.Minute,
		Metrics: dbworker.ResetterMetrics{
			Errors:              metrics.errors,
			RecordResetFailures: metrics.resetFailures,
			RecordResets:        metrics.resets,
		},
	}
	return dbworker.NewResetter(observationCtx.Logger, workerStore, options)
}

// createBitbucketProjectPermissionsStore creates a store that reads and writes to the explicit_permissions_bitbucket_projects_jobs table.
// It is used by the worker and resetter.
func createBitbucketProjectPermissionsStore(observationCtx *observation.Context, s basestore.ShareableStore, cfg *config) dbworkerstore.Store[*types.BitbucketProjectPermissionJob] {
	observationCtx = observation.ContextWithLogger(observationCtx.Logger.Scoped("BitbucketProjectPermission.Store"), observationCtx)

	return dbworkerstore.New(observationCtx, s.Handle(), dbworkerstore.Options[*types.BitbucketProjectPermissionJob]{
		Name:              "explicit_permissions_bitbucket_projects_jobs_store",
		TableName:         "explicit_permissions_bitbucket_projects_jobs",
		ColumnExpressions: database.BitbucketProjectPermissionsColumnExpressions,
		Scan:              dbworkerstore.BuildWorkerScan(database.ScanBitbucketProjectPermissionJob),
		StalledMaxAge:     60 * time.Second,
		RetryAfter:        cfg.WorkerRetryInterval,
		MaxNumRetries:     5,
		OrderByExpression: sqlf.Sprintf("explicit_permissions_bitbucket_projects_jobs.id"),
	})
}

// These are the metrics that are used by the worker and resetter.
// They are required by the workerutil package for automatic metrics collection.
type bitbucketProjectPermissionsMetrics struct {
	workerMetrics workerutil.WorkerObservability
	resets        prometheus.Counter
	resetFailures prometheus.Counter
	errors        prometheus.Counter
}

func newMetricsForBitbucketProjectPermissionsQueries(logger log.Logger) bitbucketProjectPermissionsMetrics {
	observationCtx := observation.NewContext(logger.Scoped("routines"))

	resetFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_explicit_permissions_bitbucket_project_query_reset_failures_total",
		Help: "The number of reset failures.",
	})
	observationCtx.Registerer.MustRegister(resetFailures)

	resets := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_explicit_permissions_bitbucket_project_query_resets_total",
		Help: "The number of records reset.",
	})
	observationCtx.Registerer.MustRegister(resets)

	errorCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_explicit_permissions_bitbucket_project_query_errors_total",
		Help: "The number of errors that occur during job.",
	})
	observationCtx.Registerer.MustRegister(errorCounter)

	return bitbucketProjectPermissionsMetrics{
		workerMetrics: workerutil.NewMetrics(observationCtx, "explicit_permissions_bitbucket_project_queries"),
		resets:        resets,
		resetFailures: resetFailures,
		errors:        errorCounter,
	}
}
