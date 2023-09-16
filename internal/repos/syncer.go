package repos

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// A Syncer periodically synchronizes available repositories from all its given Sources
// with the stored Repositories in Sourcegraph.
type Syncer struct {
	Sourcer Sourcer
	Store   Store

	// Synced is sent a collection of Repos that were synced by Sync (only if Synced is non-nil)
	Synced chan Diff

	ObsvCtx *observation.Context

	// Now is time.Now. Can be set by tests to get deterministic output.
	Now func() time.Time

	// Ensure that we only run one sync per repo at a time
	syncGroup singleflight.Group
}

func NewSyncer(observationCtx *observation.Context, store Store, sourcer Sourcer) *Syncer {
	return &Syncer{
		Sourcer: sourcer,
		Store:   store,
		Synced:  make(chan Diff),
		Now:     func() time.Time { return time.Now().UTC() },
		ObsvCtx: observation.ContextWithLogger(observationCtx.Logger.Scoped("syncer", "repo syncer"), observationCtx),
	}
}

// RunOptions contains options customizing Run behaviour.
type RunOptions struct {
	EnqueueInterval func() time.Duration // Defaults to 1 minute
	IsDotCom        bool                 // Defaults to false
	MinSyncInterval func() time.Duration // Defaults to 1 minute
	DequeueInterval time.Duration        // Default to 10 seconds
}

// Routines returns the goroutines that run the Sync at the specified interval.
func (s *Syncer) Routines(ctx context.Context, store Store, opts RunOptions) []goroutine.BackgroundRoutine {
	if opts.EnqueueInterval == nil {
		opts.EnqueueInterval = func() time.Duration { return time.Minute }
	}
	if opts.MinSyncInterval == nil {
		opts.MinSyncInterval = func() time.Duration { return time.Minute }
	}
	if opts.DequeueInterval == 0 {
		opts.DequeueInterval = 10 * time.Second
	}

	if !opts.IsDotCom {
		s.initialUnmodifiedDiffFromStore(ctx, store)
	}

	worker, resetter, syncerJanitor := NewSyncWorker(ctx, observation.ContextWithLogger(s.ObsvCtx.Logger.Scoped("syncWorker", ""), s.ObsvCtx),
		store.Handle(),
		&syncHandler{
			syncer:          s,
			store:           store,
			minSyncInterval: opts.MinSyncInterval,
		}, SyncWorkerOptions{
			WorkerInterval: opts.DequeueInterval,
			NumHandlers:    ConfRepoConcurrentExternalServiceSyncers(),
			CleanupOldJobs: true,
		},
	)

	scheduler := goroutine.NewPeriodicGoroutine(
		actor.WithInternalActor(ctx),
		goroutine.HandlerFunc(func(ctx context.Context) error {
			if conf.Get().DisableAutoCodeHostSyncs {
				return nil
			}

			if err := store.EnqueueSyncJobs(ctx, opts.IsDotCom); err != nil {
				return errors.Wrap(err, "enqueueing sync jobs")
			}

			return nil
		}),
		goroutine.WithName("repo-updater.repo-sync-scheduler"),
		goroutine.WithDescription("enqueues sync jobs for external service sync jobs"),
		goroutine.WithIntervalFunc(opts.EnqueueInterval),
	)

	return []goroutine.BackgroundRoutine{worker, resetter, syncerJanitor, scheduler}
}

type syncHandler struct {
	syncer          *Syncer
	store           Store
	minSyncInterval func() time.Duration
}

func (s *syncHandler) Handle(ctx context.Context, _ log.Logger, sj *SyncJob) (err error) {
	// Limit calls to progressRecorder as it will most likely hit the database
	progressLimiter := rate.NewLimiter(rate.Limit(1.0), 1)

	progressRecorder := func(ctx context.Context, progress SyncProgress, final bool) error {
		if final || progressLimiter.Allow() {
			return s.store.ExternalServiceStore().UpdateSyncJobCounters(ctx, &types.ExternalServiceSyncJob{
				ID:              int64(sj.ID),
				ReposSynced:     progress.Synced,
				RepoSyncErrors:  progress.Errors,
				ReposAdded:      progress.Added,
				ReposDeleted:    progress.Deleted,
				ReposModified:   progress.Modified,
				ReposUnmodified: progress.Unmodified,
			})
		}
		return nil
	}

	return s.syncer.SyncExternalService(ctx, sj.ExternalServiceID, s.minSyncInterval(), progressRecorder)
}

// TriggerExternalServiceSync will enqueue a sync job for the supplied external
// service
func (s *Syncer) TriggerExternalServiceSync(ctx context.Context, id int64) error {
	return s.Store.EnqueueSingleSyncJob(ctx, id)
}

const (
	ownerUndefined = ""
	ownerSite      = "site"
)

type ErrUnauthorized struct{}

func (e ErrUnauthorized) Error() string {
	return "bad credentials"
}

func (e ErrUnauthorized) Unauthorized() bool {
	return true
}

type ErrForbidden struct{}

func (e ErrForbidden) Error() string {
	return "forbidden"
}

func (e ErrForbidden) Forbidden() bool {
	return true
}

type ErrAccountSuspended struct{}

func (e ErrAccountSuspended) Error() string {
	return "account suspended"
}

func (e ErrAccountSuspended) AccountSuspended() bool {
	return true
}

// initialUnmodifiedDiffFromStore creates a diff of all repos present in the
// store and sends it to s.Synced. This is used so that on startup the reader
// of s.Synced will receive a list of repos. In particular this is so that the
// git update scheduler can start working straight away on existing
// repositories.
func (s *Syncer) initialUnmodifiedDiffFromStore(ctx context.Context, store Store) {
	if s.Synced == nil {
		return
	}

	stored, err := store.RepoStore().List(ctx, database.ReposListOptions{})
	if err != nil {
		s.ObsvCtx.Logger.Warn("initialUnmodifiedDiffFromStore store.ListRepos", log.Error(err))
		return
	}

	// Assuming sources returns no differences from the last sync, the Diff
	// would be just a list of all stored repos Unmodified. This is the steady
	// state, so is the initial diff we choose.
	select {
	case s.Synced <- Diff{Unmodified: stored}:
	case <-ctx.Done():
	}
}

// Diff is the difference found by a sync between what is in the store and
// what is returned from sources.
type Diff struct {
	Added      types.Repos
	Deleted    types.Repos
	Modified   ReposModified
	Unmodified types.Repos
}

// Sort sorts all Diff elements by Repo.IDs.
func (d *Diff) Sort() {
	for _, ds := range []types.Repos{
		d.Added,
		d.Deleted,
		d.Modified.Repos(),
		d.Unmodified,
	} {
		sort.Sort(ds)
	}
}

// Repos returns all repos in the Diff.
func (d *Diff) Repos() types.Repos {
	all := make(types.Repos, 0, len(d.Added)+
		len(d.Deleted)+
		len(d.Modified)+
		len(d.Unmodified))

	for _, rs := range []types.Repos{
		d.Added,
		d.Deleted,
		d.Modified.Repos(),
		d.Unmodified,
	} {
		all = append(all, rs...)
	}

	return all
}

func (d *Diff) Len() int {
	return len(d.Deleted) + len(d.Modified) + len(d.Added) + len(d.Unmodified)
}

// RepoModified tracks the modifications applied to a single repository after a
// sync.
type RepoModified struct {
	Repo     *types.Repo
	Modified types.RepoModified
}

type ReposModified []RepoModified

// Repos returns all modified repositories.
func (rm ReposModified) Repos() types.Repos {
	repos := make(types.Repos, len(rm))
	for i := range rm {
		repos[i] = rm[i].Repo
	}

	return repos
}

// ReposModified returns only the repositories that had a specific field
// modified in the sync.
func (rm ReposModified) ReposModified(modified types.RepoModified) types.Repos {
	repos := types.Repos{}
	for _, pair := range rm {
		if pair.Modified&modified == modified {
			repos = append(repos, pair.Repo)
		}
	}

	return repos
}

// SyncRepo syncs a single repository by name and associates it with an external service.
//
// It works for repos from:
//
//  1. Public "cloud_default" code hosts since we don't sync them in the background
//     (which would delete lazy synced repos).
//  2. Any package hosts (i.e. npm, Maven, etc) since callers are expected to store
//     repos in the `lsif_dependency_repos` table which is used as the source of truth
//     for the next full sync, so lazy added repos don't get wiped.
//
// The "background" boolean flag indicates that we should run this
// sync in the background vs block and call s.syncRepo synchronously.
func (s *Syncer) SyncRepo(ctx context.Context, name api.RepoName, background bool) (repo *types.Repo, err error) {
	logger := s.ObsvCtx.Logger.With(log.String("name", string(name)), log.Bool("background", background))

	logger.Debug("SyncRepo started")

	tr, ctx := trace.New(ctx, "Syncer.SyncRepo", name.Attr())
	defer tr.End()

	repo, err = s.Store.RepoStore().GetByName(ctx, name)
	if err != nil && !errcode.IsNotFound(err) {
		return nil, errors.Wrapf(err, "GetByName failed for %q", name)
	}

	codehost := extsvc.CodeHostOf(name, extsvc.PublicCodeHosts...)
	if codehost == nil {
		if repo != nil {
			return repo, nil
		}

		logger.Debug("no associated code host found, skipping")
		return nil, &database.RepoNotFoundErr{Name: name}
	}

	if repo != nil {
		// Only public repos can be individually synced on sourcegraph.com
		if repo.Private {
			logger.Debug("repo is private, skipping")
			return nil, &database.RepoNotFoundErr{Name: name}
		}
		// Don't sync the repo if it's been updated in the past 1 minute.
		if s.Now().Sub(repo.UpdatedAt) < time.Minute {
			logger.Debug("repo updated recently, skipping")
			return repo, nil
		}
	}

	if background && repo != nil {
		logger.Debug("starting background sync in goroutine")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			// We don't care about the return value here, but we still want to ensure that
			// only one is in flight at a time.
			updatedRepo, err, shared := s.syncGroup.Do(string(name), func() (any, error) {
				return s.syncRepo(ctx, codehost, name, repo)
			})
			logger.Debug("syncGroup completed", log.String("updatedRepo", fmt.Sprintf("%v", updatedRepo.(*types.Repo))))
			if err != nil {
				logger.Error("background.SyncRepo", log.Error(err), log.Bool("shared", shared))
			}
		}()
		return repo, nil
	}

	logger.Debug("starting foreground sync")
	updatedRepo, err, shared := s.syncGroup.Do(string(name), func() (any, error) {
		return s.syncRepo(ctx, codehost, name, repo)
	})
	if err != nil {
		return nil, errors.Wrapf(err, "foreground.SyncRepo (shared=%v)", shared)
	}
	return updatedRepo.(*types.Repo), nil
}

func (s *Syncer) syncRepo(
	ctx context.Context,
	codehost *extsvc.CodeHost,
	name api.RepoName,
	stored *types.Repo,
) (repo *types.Repo, err error) {
	var svc *types.ExternalService
	ctx, save := s.observeSync(ctx, "Syncer.syncRepo", name.Attr())
	defer func() { save(svc, err) }()

	svcs, err := s.Store.ExternalServiceStore().List(ctx, database.ExternalServicesListOptions{
		Kinds: []string{extsvc.TypeToKind(codehost.ServiceType)},
		// Since package host external services have the set of repositories to sync in
		// the lsif_dependency_repos table, we can lazy-sync individual repos without wiping them
		// out in the next full background sync as long as we add them to that table.
		//
		// This permits lazy-syncing of package repos in on-prem instances as well as in cloud.
		OnlyCloudDefault: !codehost.IsPackageHost(),
		LimitOffset:      &database.LimitOffset{Limit: 1},
	})
	if err != nil {
		return nil, errors.Wrap(err, "listing external services")
	}

	if len(svcs) != 1 {
		return nil, errors.Wrapf(
			&database.RepoNotFoundErr{Name: name},
			"cloud default external service of type %q not found", codehost.ServiceType,
		)
	}

	svc = svcs[0]

	src, err := s.Sourcer(ctx, svc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve Sourcer")
	}

	rg, ok := src.(RepoGetter)
	if !ok {
		return nil, errors.Wrapf(
			&database.RepoNotFoundErr{Name: name},
			"can't get repo metadata for service of type %q", codehost.ServiceType,
		)
	}

	path := strings.TrimPrefix(string(name), strings.TrimPrefix(codehost.ServiceID, "https://"))

	if stored != nil {
		defer func() {
			s.ObsvCtx.Logger.Debug("deferred deletable repo check")
			if isDeleteableRepoError(err) {
				err2 := s.Store.DeleteExternalServiceRepo(ctx, svc, stored.ID)
				if err2 != nil {
					s.ObsvCtx.Logger.Error(
						"SyncRepo failed to delete",
						log.Object("svc", log.String("name", svc.DisplayName), log.Int64("id", svc.ID)),
						log.String("repo", string(name)),
						log.NamedError("cause", err),
						log.Error(err2),
					)
				}
				s.ObsvCtx.Logger.Debug("external service repo deleted", log.Int32("deleted ID", int32(stored.ID)))
				s.notifyDeleted(ctx, stored.ID)
			}
		}()
	}

	repo, err = rg.GetRepo(ctx, path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get repo with path: %q", path)
	}

	if repo.Private {
		s.ObsvCtx.Logger.Debug("repo is private, skipping")
		return nil, &database.RepoNotFoundErr{Name: name}
	}

	if _, err = s.sync(ctx, svc, repo); err != nil {
		return nil, err
	}

	return repo, nil
}

// isDeleteableRepoError checks whether the error returned from a repo sync
// signals that we can safely delete the repo
func isDeleteableRepoError(err error) bool {
	return errcode.IsNotFound(err) || errcode.IsUnauthorized(err) ||
		errcode.IsForbidden(err) || errcode.IsAccountSuspended(err) || errcode.IsUnavailableForLegalReasons(err)
}

func (s *Syncer) notifyDeleted(ctx context.Context, deleted ...api.RepoID) {
	var d Diff
	for _, id := range deleted {
		d.Deleted = append(d.Deleted, &types.Repo{ID: id})
	}
	observeDiff(d)

	if s.Synced != nil && d.Len() > 0 {
		select {
		case <-ctx.Done():
		case s.Synced <- d:
		}
	}
}

// ErrCloudDefaultSync is returned by SyncExternalService if an attempt to
// sync a cloud default external service is done. We can't sync these external services
// because their repos are added via the lazy-syncing mechanism on sourcegraph.com
// instead of config (which is empty), so attempting to sync them would delete all of
// the lazy-added repos.
var ErrCloudDefaultSync = errors.New("cloud default external services can't be synced")

// SyncProgress represents running counts for an external service sync.
type SyncProgress struct {
	Synced int32 `json:"synced,omitempty"`
	Errors int32 `json:"errors,omitempty"`

	// Diff stats
	Added      int32 `json:"added,omitempty"`
	Removed    int32 `json:"removed,omitempty"`
	Modified   int32 `json:"modified,omitempty"`
	Unmodified int32 `json:"unmodified,omitempty"`

	Deleted int32 `json:"deleted,omitempty"`
}

type LicenseError struct {
	error
}

// progressRecorderFunc is a function that implements persisting sync progress.
// The final param represents whether this is the final call. This allows the
// function to decide whether to drop some intermediate calls.
type progressRecorderFunc func(ctx context.Context, progress SyncProgress, final bool) error

// SyncExternalService syncs repos using the supplied external service in a streaming fashion, rather than batch.
// This allows very large sync jobs (i.e. that source potentially millions of repos) to incrementally persist changes.
// Deletes of repositories that were not sourced are done at the end.
func (s *Syncer) SyncExternalService(
	ctx context.Context,
	externalServiceID int64,
	minSyncInterval time.Duration,
	progressRecorder progressRecorderFunc,
) (err error) {
	logger := s.ObsvCtx.Logger.With(log.Int64("externalServiceID", externalServiceID))
	logger.Info("syncing external service")

	// Ensure the job field is recorded when monitoring external API calls
	ctx = metrics.ContextWithTask(ctx, "SyncExternalService")

	var svc *types.ExternalService
	ctx, save := s.observeSync(ctx, "Syncer.SyncExternalService")
	defer func() { save(svc, err) }()

	// We don't use tx here as the sourcing process below can be slow and we don't
	// want to hold a lock on the external_services table for too long.
	svc, err = s.Store.ExternalServiceStore().GetByID(ctx, externalServiceID)
	if err != nil {
		return errors.Wrap(err, "fetching external services")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// From this point we always want to make a best effort attempt to update the
	// service timestamps
	var modified bool
	defer func() {
		now := s.Now()
		interval := calcSyncInterval(now, svc.LastSyncAt, minSyncInterval, modified, err)

		nextSyncAt := now.Add(interval)
		lastSyncAt := now

		// We call Update here instead of Upsert, because upsert stores all fields of the external
		// service, and syncs take a while so changes to the external service made while this sync
		// was running would be overwritten again.
		if err := s.Store.ExternalServiceStore().Update(ctx, nil, svc.ID, &database.ExternalServiceUpdate{
			LastSyncAt: &lastSyncAt,
			NextSyncAt: &nextSyncAt,
		}); err != nil {
			// We only want to log this error, not return it
			logger.Error("upserting external service", log.Error(err))
		}

		logger.Debug("synced external service", log.Duration("backoff duration", interval))
	}()

	// We have fail-safes in place to prevent enqueuing sync jobs for cloud default
	// external services, but in case those fail to prevent a sync for any reason,
	// we have this additional check here. Cloud default external services have their
	// repos added via the lazy-syncing mechanism on sourcegraph.com instead of config
	// (which is empty), so attempting to sync them would delete all of the lazy-added repos.
	if svc.CloudDefault {
		return ErrCloudDefaultSync
	}

	src, err := s.Sourcer(ctx, svc)
	if err != nil {
		return err
	}

	if err := src.CheckConnection(ctx); err != nil {
		logger.Warn("connection check failed. syncing repositories might still succeed.", log.Error(err))
	}

	results := make(chan SourceResult)
	go func() {
		src.ListRepos(ctx, results)
		close(results)
	}()

	seen := make(map[api.RepoID]struct{})
	var errs error

	logger = s.ObsvCtx.Logger.With(log.Object("svc", log.String("name", svc.DisplayName), log.Int64("id", svc.ID)))

	var syncProgress SyncProgress
	// Record the final progress state
	defer func() {
		// Use a different context here so that we make sure to record progress
		// even if context has been canceled
		if err := progressRecorder(context.Background(), syncProgress, true); err != nil {
			logger.Error("recording final sync progress", log.Error(err))
		}
	}()

	// Insert or update repos as they are sourced. Keep track of what was seen so we
	// can remove anything else at the end.
	for res := range results {
		logger.Debug("received result", log.String("repo", fmt.Sprintf("%v", res)))

		if err := progressRecorder(ctx, syncProgress, false); err != nil {
			logger.Warn("recording sync progress", log.Error(err))
		}

		if err := res.Err; err != nil {
			syncProgress.Errors++
			logger.Error("error from codehost", log.Int("seen", len(seen)), log.Error(err))

			errs = errors.Append(errs, errors.Wrapf(err, "fetching from code host %s", svc.DisplayName))
			if IsFatalSyncError(err) {
				// Delete all external service repos of this external service
				logger.Error("stopping external service sync due to fatal error from codehost", log.Error(err))
				seen = map[api.RepoID]struct{}{}
				break
			}

			continue
		}

		sourced := res.Repo

		if envvar.SourcegraphDotComMode() && sourced.Private {
			err := errors.Newf("%s is private, but dotcom does not support private repositories.", sourced.Name)
			syncProgress.Errors++
			logger.Error("failed to sync private repo", log.String("repo", string(sourced.Name)), log.Error(err))
			errs = errors.Append(errs, err)
			continue
		}

		var diff Diff
		if diff, err = s.sync(ctx, svc, sourced); err != nil {
			syncProgress.Errors++
			logger.Error("failed to sync, skipping", log.String("repo", string(sourced.Name)), log.Error(err))
			errs = errors.Append(errs, err)
			continue
		}

		syncProgress.Added += int32(diff.Added.Len())
		syncProgress.Removed += int32(diff.Deleted.Len())
		syncProgress.Modified += int32(diff.Modified.Repos().Len())
		syncProgress.Unmodified += int32(diff.Unmodified.Len())

		for _, r := range diff.Repos() {
			seen[r.ID] = struct{}{}
		}
		syncProgress.Synced = int32(len(seen))

		modified = modified || len(diff.Modified)+len(diff.Added) > 0
	}

	// We don't delete any repos of site-level external services if there were any
	// non-warning errors during a sync.
	//
	// Only user or organization external services will delete
	// repos in a sync run with fatal errors.
	//
	// Site-level external services can own lots of repos and are managed by site admins.
	// It's preferable to have them fix any invalidated token manually rather than deleting the repos automatically.
	deleted := 0

	// If all of our errors are warnings and either Forbidden or Unauthorized,
	// we want to proceed with the deletion. This is to be able to properly sync
	// repos (by removing ones if code-host permissions have changed).
	abortDeletion := false
	if errs != nil {
		var ref errors.MultiError
		if errors.As(errs, &ref) {
			for _, e := range ref.Errors() {
				if errors.IsWarning(e) {
					baseError := errors.Unwrap(e)
					if !errcode.IsForbidden(baseError) && !errcode.IsUnauthorized(baseError) {
						abortDeletion = true
						break
					}
					continue
				}
				if errors.As(e, &LicenseError{}) {
					continue
				}
				abortDeletion = true
				break
			}
		}
	}

	if !abortDeletion {
		// Remove associations and any repos that are no longer associated with any
		// external service.
		//
		// We don't want to delete all repos that weren't seen if we had a lot of
		// spurious errors since that could cause lots of repos to be deleted, only to be
		// added the next sync. We delete only if we had no errors,
		// or all of our errors are warnings and either Forbidden or Unauthorized,
		// or we had one of the fatal errors and the service is not site owned.
		var deletedErr error
		deleted, deletedErr = s.delete(ctx, svc, seen)
		if deletedErr != nil {
			logger.Warn("failed to delete some repos",
				log.Int("seen", len(seen)),
				log.Int("deleted", deleted),
				log.Error(deletedErr),
			)

			errs = errors.Append(errs, errors.Wrap(deletedErr, "some repos couldn't be deleted"))
		}

		if deleted > 0 {
			syncProgress.Deleted += int32(deleted)
			logger.Warn("deleted not seen repos",
				log.Int("seen", len(seen)),
				log.Int("deleted", deleted),
				log.Error(err),
			)
		}
	}

	modified = modified || deleted > 0

	return errs
}

// syncs a sourced repo of a given external service, returning a diff with a single repo.
func (s *Syncer) sync(ctx context.Context, svc *types.ExternalService, sourced *types.Repo) (d Diff, err error) {
	tx, err := s.Store.Transact(ctx)
	if err != nil {
		return Diff{}, errors.Wrap(err, "syncer: opening transaction")
	}

	defer func() {
		observeDiff(d)
		// We must commit the transaction before publishing to s.Synced
		// so that gitserver finds the repo in the database.

		s.ObsvCtx.Logger.Debug("committing transaction")
		err = tx.Done(err)
		if err != nil {
			s.ObsvCtx.Logger.Warn("failed to commit transaction", log.Error(err))
			return
		}

		if s.Synced != nil && d.Len() > 0 {
			select {
			case <-ctx.Done():
				s.ObsvCtx.Logger.Debug("sync context done")
			case s.Synced <- d:
				s.ObsvCtx.Logger.Debug("diff synced")
			}
		}
	}()

	stored, err := tx.RepoStore().List(ctx, database.ReposListOptions{
		Names:          []string{string(sourced.Name)},
		ExternalRepos:  []api.ExternalRepoSpec{sourced.ExternalRepo},
		IncludeBlocked: true,
		IncludeDeleted: true,
		UseOr:          true,
	})
	if err != nil {
		return Diff{}, errors.Wrap(err, "syncer: getting repo from the database")
	}

	switch len(stored) {
	case 2: // Existing repo with a naming conflict
		// Scenario where this can happen:
		// 1. Repo `owner/repo1` with external_id 1 exists
		// 2. Repo `owner/repo2` with external_id 2 exists
		// 3. The owner deletes repo1, and renames repo2 to repo1
		// 4. We sync and we receive `owner/repo1` with external_id 2
		//
		// Then the above query will return two results: one matching the name owner/repo1, and one matching the external_service_id 2
		// The original owner/repo1 should be deleted, and then owner/repo2 with the matching external_service_id should be updated
		s.ObsvCtx.Logger.Debug("naming conflict")

		// Pick this sourced repo to own the name by deleting the other repo. If it still exists, it'll have a different
		// name when we source it from the same code host, and it will be re-created.
		var conflicting, existing *types.Repo
		for _, r := range stored {
			if r.ExternalRepo.Equal(&sourced.ExternalRepo) {
				existing = r
			} else {
				conflicting = r
			}
		}

		// invariant: conflicting can't be nil due to our database constraints
		if err = tx.RepoStore().Delete(ctx, conflicting.ID); err != nil {
			return Diff{}, errors.Wrap(err, "syncer: failed to delete conflicting repo")
		}

		// We fallthrough to the next case after removing the conflicting repo in order to update
		// the winner (i.e. existing). This works because we mutate stored to contain it, which the case expects.
		stored = types.Repos{existing}
		s.ObsvCtx.Logger.Debug("retrieved stored repo, falling through", log.String("stored", fmt.Sprintf("%v", stored)))
		fallthrough
	case 1: // Existing repo, update.
		s.ObsvCtx.Logger.Debug("existing repo")
		if err := UpdateRepoLicenseHook(ctx, tx, stored[0], sourced); err != nil {
			return Diff{}, LicenseError{errors.Wrapf(err, "syncer: failed to update repo %s", sourced.Name)}
		}
		modified := stored[0].Update(sourced)
		if modified == types.RepoUnmodified {
			d.Unmodified = append(d.Unmodified, stored[0])
			break
		}

		if err = tx.UpdateExternalServiceRepo(ctx, svc, stored[0]); err != nil {
			return Diff{}, errors.Wrap(err, "syncer: failed to update external service repo")
		}

		*sourced = *stored[0]
		d.Modified = append(d.Modified, RepoModified{Repo: stored[0], Modified: modified})
		s.ObsvCtx.Logger.Debug("appended to modified repos")
	case 0: // New repo, create.
		s.ObsvCtx.Logger.Debug("new repo")

		if err := CreateRepoLicenseHook(ctx, tx, sourced); err != nil {
			return Diff{}, LicenseError{errors.Wrapf(err, "syncer: failed to update repo %s", sourced.Name)}
		}

		if err = tx.CreateExternalServiceRepo(ctx, svc, sourced); err != nil {
			return Diff{}, errors.Wrapf(err, "syncer: failed to create external service repo: %s", sourced.Name)
		}

		d.Added = append(d.Added, sourced)
		s.ObsvCtx.Logger.Debug("appended to added repos")
	default: // Impossible since we have two separate unique constraints on name and external repo spec
		panic("unreachable")
	}

	s.ObsvCtx.Logger.Debug("completed")
	return d, nil
}

// CreateRepoLicenseHook checks if there is still room for private repositories
// available in the applied license before creating a new private repository.
func CreateRepoLicenseHook(ctx context.Context, s Store, repo *types.Repo) error {
	// If the repository is public, we don't have to check anything
	if !repo.Private {
		return nil
	}

	if prFeature := (&licensing.FeaturePrivateRepositories{}); licensing.Check(prFeature) == nil {
		if prFeature.Unrestricted {
			return nil
		}

		numPrivateRepos, err := s.RepoStore().Count(ctx, database.ReposListOptions{OnlyPrivate: true})
		if err != nil {
			return err
		}

		if numPrivateRepos >= prFeature.MaxNumPrivateRepos {
			return errors.Newf("maximum number of private repositories included in license (%d) reached", prFeature.MaxNumPrivateRepos)
		}

		return nil
	}

	return licensing.NewFeatureNotActivatedError("The private repositories feature is not activated for this license. Please upgrade your license to use this feature.")
}

// UpdateRepoLicenseHook checks if there is still room for private repositories
// available in the applied license before updating a repository from public to private,
// or undeleting a private repository.
func UpdateRepoLicenseHook(ctx context.Context, s Store, existingRepo *types.Repo, newRepo *types.Repo) error {
	// If it is being updated to a public repository, or if a repository is being deleted, we don't have to check anything
	if !newRepo.Private || !newRepo.DeletedAt.IsZero() {
		return nil
	}

	if prFeature := (&licensing.FeaturePrivateRepositories{}); licensing.Check(prFeature) == nil {
		if prFeature.Unrestricted {
			return nil
		}

		numPrivateRepos, err := s.RepoStore().Count(ctx, database.ReposListOptions{OnlyPrivate: true})
		if err != nil {
			return err
		}

		if numPrivateRepos > prFeature.MaxNumPrivateRepos {
			return errors.Newf("maximum number of private repositories included in license (%d) reached", prFeature.MaxNumPrivateRepos)
		} else if numPrivateRepos == prFeature.MaxNumPrivateRepos {
			// If the repository is already private, we don't have to check anything
			newPrivateRepo := (!existingRepo.DeletedAt.IsZero() || !existingRepo.Private) && newRepo.Private // If restoring a deleted repository, or if it was a public repository, and is now private
			if newPrivateRepo {
				return errors.Newf("maximum number of private repositories included in license (%d) reached", prFeature.MaxNumPrivateRepos)
			}
		}

		return nil
	}

	return licensing.NewFeatureNotActivatedError("The private repositories feature is not activated for this license. Please upgrade your license to use this feature.")
}

func (s *Syncer) delete(ctx context.Context, svc *types.ExternalService, seen map[api.RepoID]struct{}) (int, error) {
	// We do deletion in a best effort manner, returning any errors for individual repos that failed to be deleted.
	deleted, err := s.Store.DeleteExternalServiceReposNotIn(ctx, svc, seen)

	s.notifyDeleted(ctx, deleted...)

	return len(deleted), err
}

func observeDiff(diff Diff) {
	for state, repos := range map[string]types.Repos{
		"added":      diff.Added,
		"modified":   diff.Modified.Repos(),
		"deleted":    diff.Deleted,
		"unmodified": diff.Unmodified,
	} {
		syncedTotal.WithLabelValues(state).Add(float64(len(repos)))
	}
}

func calcSyncInterval(
	now time.Time,
	lastSync time.Time,
	minSyncInterval time.Duration,
	modified bool,
	err error,
) time.Duration {
	const maxSyncInterval = 8 * time.Hour

	// Special case, we've never synced
	if err == nil && (lastSync.IsZero() || modified) {
		return minSyncInterval
	}

	// No change or there were errors, back off
	interval := now.Sub(lastSync) * 2
	if interval < minSyncInterval {
		return minSyncInterval
	}

	if interval > maxSyncInterval {
		return maxSyncInterval
	}

	return interval
}

func (s *Syncer) observeSync(
	ctx context.Context,
	name string,
	attributes ...attribute.KeyValue,
) (context.Context, func(*types.ExternalService, error)) {
	began := s.Now()
	tr, ctx := trace.New(ctx, name, attributes...)

	return ctx, func(svc *types.ExternalService, err error) {
		var owner string
		if svc == nil {
			owner = ownerUndefined
		} else {
			owner = ownerSite
		}

		syncStarted.WithLabelValues(name, owner).Inc()

		now := s.Now()
		took := now.Sub(began).Seconds()

		lastSync.WithLabelValues(name).Set(float64(now.Unix()))

		success := err == nil
		syncDuration.WithLabelValues(strconv.FormatBool(success), name).Observe(took)

		if !success {
			tr.SetError(err)
			syncErrors.WithLabelValues(name, owner, syncErrorReason(err)).Inc()
		}

		tr.End()
	}
}

func syncErrorReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errcode.IsNotFound(err):
		return "not_found"
	case errcode.IsUnauthorized(err):
		return "unauthorized"
	case errcode.IsForbidden(err):
		return "forbidden"
	case errcode.IsTemporary(err):
		return "temporary"
	case strings.Contains(err.Error(), "expected path in npm/(scope/)?name"):
		// This is a known issue which we can filter out for now
		return "invalid_npm_path"
	case strings.Contains(err.Error(), "internal rate limit exceeded"):
		// We want to identify these as it's not an issue communicating with the code
		// host and is most likely caused by temporary traffic spikes.
		return "internal_rate_limit"
	default:
		return "unknown"
	}
}

func IsFatalSyncError(err error) bool {
	// If the error is just a warning, then it is not fatal.
	if errors.IsWarning(err) && !errcode.IsAccountSuspended(err) {
		return false
	}

	return errcode.IsUnauthorized(err) ||
		errcode.IsForbidden(err) ||
		errcode.IsAccountSuspended(err)
}
