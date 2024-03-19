package dependencies

import (
	"context"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/dependencies"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/packagefilters"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	dbworkerstore "github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewDependencySyncScheduler returns a new worker instance that processes
// records from lsif_dependency_syncing_jobs.
func NewDependencySyncScheduler(
	dependencySyncStore dbworkerstore.Store[dependencySyncingJob],
	uploadSvc UploadService,
	depsSvc DependenciesService,
	store store.Store,
	externalServiceStore ExternalServiceStore,
	metrics workerutil.WorkerObservability,
	config *Config,
) *workerutil.Worker[dependencySyncingJob] {
	rootContext := actor.WithInternalActor(context.Background())
	handler := &dependencySyncSchedulerHandler{
		uploadsSvc:  uploadSvc,
		depsSvc:     depsSvc,
		store:       store,
		workerStore: dependencySyncStore,
		extsvcStore: externalServiceStore,
	}

	return dbworker.NewWorker[dependencySyncingJob](rootContext, dependencySyncStore, handler, workerutil.WorkerOptions{
		Name:              "precise_code_intel_dependency_sync_scheduler_worker",
		Description:       "reads dependency package references from code-intel uploads to be synced to the instance",
		NumHandlers:       1,
		Interval:          config.DependencySyncSchedulerPollInterval,
		HeartbeatInterval: 1 * time.Second,
		Metrics:           metrics,
	})
}

type dependencySyncSchedulerHandler struct {
	uploadsSvc  UploadService
	depsSvc     DependenciesService
	store       store.Store
	workerStore dbworkerstore.Store[dependencySyncingJob]
	extsvcStore ExternalServiceStore
}

// For mocking in tests
var autoIndexingEnabled = conf.CodeIntelAutoIndexingEnabled

var schemeToExternalService = map[string]string{
	dependencies.JVMPackagesScheme:    extsvc.KindJVMPackages,
	dependencies.NpmPackagesScheme:    extsvc.KindNpmPackages,
	dependencies.PythonPackagesScheme: extsvc.KindPythonPackages,
	dependencies.RustPackagesScheme:   extsvc.KindRustPackages,
	dependencies.RubyPackagesScheme:   extsvc.KindRubyPackages,
}

func (h *dependencySyncSchedulerHandler) Handle(ctx context.Context, logger log.Logger, job dependencySyncingJob) error {
	if !autoIndexingEnabled() {
		return nil
	}

	scanner, err := h.uploadsSvc.ReferencesForUpload(ctx, job.UploadID)
	if err != nil {
		return errors.Wrap(err, "dbstore.ReferencesForUpload")
	}
	defer func() {
		if closeErr := scanner.Close(); closeErr != nil {
			err = errors.Append(err, errors.Wrap(closeErr, "dbstore.ReferencesForUpload.Close"))
		}
	}()

	var (
		instant             = time.Now()
		kinds               = map[string]struct{}{}
		oldDepReposInserted int
		newDepReposInserted int
		newVersionsInserted int
		oldVersionsInserted int
		errs                []error
	)

	pkgFilters, _, err := h.depsSvc.ListPackageRepoFilters(ctx, dependencies.ListPackageRepoRefFiltersOpts{})
	if err != nil {
		return errors.Wrap(err, "error listing package repo filters")
	}

	packageFilters, err := packagefilters.NewFilterLists(pkgFilters)
	if err != nil {
		return err
	}

	for {
		packageReference, exists, err := scanner.Next()
		if err != nil {
			return errors.Wrap(err, "dbstore.ReferencesForUpload.Next")
		}
		if !exists {
			break
		}

		pkgRef, err := newPackage(packageReference.Package)
		if err != nil {
			// Indexers can potentially create package references with bad names,
			// which are no longer recognized by the package manager. In such a
			// case, it doesn't make sense to add a bad package as a dependency repo.
			logger.Warn("package referenced by upload was invalid",
				log.Error(err),
				log.String("name", packageReference.Name),
				log.String("version", packageReference.Version),
				log.Int("uploadID", packageReference.UploadID))
			continue
		}
		pkg := *pkgRef

		extsvcKind, ok := schemeToExternalService[pkg.Scheme]
		// add entry for empty string/kind here so dependencies such as lsif-go ones still get
		// an associated dependency indexing job
		kinds[extsvcKind] = struct{}{}
		if !ok {
			continue
		}

		newRepo, newVersion, err := h.insertPackageRepoRef(ctx, pkg, packageFilters, instant)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if newRepo {
			newDepReposInserted++
		} else {
			oldDepReposInserted++
		}
		if newVersion {
			newVersionsInserted++
		} else {
			oldVersionsInserted++
		}
	}

	var nextSync time.Time
	kindsArray := kindsToArray(kinds)
	// If len == 0, it will return all external services, which we definitely don't want.
	if len(kindsArray) > 0 {
		nextSync = time.Now()
		externalServices, err := h.extsvcStore.List(ctx, database.ExternalServicesListOptions{
			Kinds: kindsArray,
		})
		if err != nil {
			if len(errs) == 0 {
				return errors.Wrap(err, "dbstore.List")
			} else {
				return errors.Append(err, errs...)
			}
		}

		logger.Info("syncing external services",
			log.Int("upload", job.UploadID),
			log.Int("numExtSvc", len(externalServices)),
			log.Strings("schemaKinds", kindsArray),
			log.Int("newRepos", newDepReposInserted),
			log.Int("existingRepos", oldDepReposInserted),
			log.Int("newVersions", newVersionsInserted),
			log.Int("existingVersions", oldVersionsInserted),
		)

		for _, externalService := range externalServices {
			externalService.NextSyncAt = nextSync
			err := h.extsvcStore.Upsert(ctx, externalService)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "extsvcStore.Upsert: error setting next_sync_at for external service %d - %s", externalService.ID, externalService.DisplayName))
			}
		}
	} else {
		logger.Info("no package schema kinds to sync external services for", log.Int("upload", job.UploadID), log.Int("job", job.ID))
	}

	shouldIndex, err := h.shouldIndexDependencies(ctx, h.uploadsSvc, job.UploadID)
	if err != nil {
		return err
	}

	if shouldIndex {
		// If we saw a kind that's not in schemeToExternalService, then kinds contains an empty string key
		for kind := range kinds {
			if _, err := h.store.InsertDependencyIndexingJob(ctx, job.UploadID, kind, nextSync); err != nil {
				errs = append(errs, errors.Wrap(err, "dbstore.InsertDependencyIndexingJob"))
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	if len(errs) == 1 {
		return errs[0]
	}

	return errors.Append(nil, errs...)
}

// newPackage constructs a precise.Package from the given shared.Package,
// applying any normalization or necessary transformations that LSIF/SCIP uploads
// require for internal consistency.
func newPackage(pkg uploadsshared.Package) (*precise.Package, error) {
	p := precise.Package{
		Scheme:  pkg.Scheme,
		Manager: pkg.Manager,
		Name:    pkg.Name,
		Version: pkg.Version,
	}

	switch pkg.Scheme {
	case dependencies.JVMPackagesScheme:
		p.Name = strings.TrimPrefix(p.Name, "maven/")
		p.Name = strings.ReplaceAll(p.Name, "/", ":")
	case dependencies.NpmPackagesScheme, "scip-typescript":
		if _, err := reposource.ParseNpmPackageFromPackageSyntax(reposource.PackageName(p.Name)); err != nil {
			return nil, err
		}
		p.Scheme = dependencies.NpmPackagesScheme
	case "scip-python":
		// Override scip-python scheme so that we are able to autoindex
		// index.scip created by scip-python
		p.Scheme = dependencies.PythonPackagesScheme
	}

	return &p, nil
}

func (h *dependencySyncSchedulerHandler) insertPackageRepoRef(ctx context.Context, pkg precise.Package, filters packagefilters.PackageFilters, instant time.Time) (newRepos, newVersions bool, err error) {
	insertedRepos, insertedVersions, err := h.depsSvc.InsertPackageRepoRefs(ctx, []dependencies.MinimalPackageRepoRef{
		{
			Name:          reposource.PackageName(pkg.Name),
			Scheme:        pkg.Scheme,
			Blocked:       !packagefilters.IsPackageAllowed(pkg.Scheme, reposource.PackageName(pkg.Name), filters),
			LastCheckedAt: &instant,
			Versions: []dependencies.MinimalPackageRepoRefVersion{{
				Version:       pkg.Version,
				Blocked:       !packagefilters.IsVersionedPackageAllowed(pkg.Scheme, reposource.PackageName(pkg.Name), pkg.Version, filters),
				LastCheckedAt: &instant,
			}},
		},
	})
	if err != nil {
		return false, false, errors.Wrap(err, "dbstore.InsertCloneableDependencyRepos")
	}
	return len(insertedRepos) != 0, len(insertedVersions) != 0, nil
}

// shouldIndexDependencies returns true if the given upload should undergo dependency
// indexing. Currently, we're only enabling dependency indexing for a repositories that
// were indexed via lsif-go, scip-java, lsif-tsc and scip-typescript.
func (h *dependencySyncSchedulerHandler) shouldIndexDependencies(ctx context.Context, store UploadService, uploadID int) (bool, error) {
	upload, _, err := store.GetUploadByID(ctx, uploadID)
	if err != nil {
		return false, errors.Wrap(err, "dbstore.GetUploadByID")
	}

	return upload.Indexer == "lsif-go" ||
		upload.Indexer == "scip-java" ||
		upload.Indexer == "lsif-java" ||
		upload.Indexer == "lsif-tsc" ||
		upload.Indexer == "scip-typescript" ||
		upload.Indexer == "lsif-typescript" ||
		upload.Indexer == "scip-python" ||
		upload.Indexer == "scip-ruby" ||
		upload.Indexer == "rust-analyzer", nil
}

func kindsToArray(k map[string]struct{}) (s []string) {
	for kind := range k {
		if kind != "" {
			s = append(s, kind)
		}
	}
	return
}
