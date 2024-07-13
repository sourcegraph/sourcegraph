package versions

import (
	"context"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

const syncInterval = 24 * time.Hour

func NewSyncingJob() job.Job {
	return &syncingJob{}
}

type syncingJob struct{}

func (j *syncingJob) Description() string {
	return ""
}

func (j *syncingJob) Config() []env.Config {
	return []env.Config{}
}

func (j *syncingJob) Routines(_ context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, err
	}

	sourcerLogger := observationCtx.Logger.Scoped("repos.Sourcer")
	sourcerCF := httpcli.NewExternalClientFactory(
		httpcli.NewLoggingMiddleware(sourcerLogger),
	)
	sourcer := repos.NewSourcer(sourcerLogger, db, sourcerCF, gitserver.NewClient("extsvc.version-syncer"))

	store := db.ExternalServices()
	handler := goroutine.HandlerFunc(func(ctx context.Context) error {
		versions, err := loadVersions(ctx, observationCtx.Logger, store, sourcer)
		if err != nil {
			return err
		}
		return storeVersions(versions)
	})

	return []goroutine.BackgroundRoutine{
		// Pass a fresh context, see docs for shared.Job
		goroutine.NewPeriodicGoroutine(
			context.Background(),
			handler,
			goroutine.WithName("repomgmt.version-syncer"),
			goroutine.WithDescription("sync versions of external services"),
			goroutine.WithInterval(syncInterval),
		),
	}, nil
}

func loadVersions(ctx context.Context, logger log.Logger, store database.ExternalServiceStore, sourcer repos.Sourcer) ([]*Version, error) {
	var versions []*Version

	es, err := store.List(ctx, database.ExternalServicesListOptions{})
	if err != nil {
		return versions, err
	}

	// Group the external services by the code host instance they point at so
	// we don't send >1 requests to the same instance.
	unique := make(map[string]*types.ExternalService)
	for _, svc := range es {
		ident, err := extsvc.UniqueEncryptableCodeHostIdentifier(ctx, svc.Kind, svc.Config)
		if err != nil {
			return versions, err
		}

		if _, ok := unique[ident]; ok {
			continue
		}
		unique[ident] = svc
	}

	for _, svc := range unique {
		src, err := sourcer(ctx, svc)
		if err != nil {
			return versions, err
		}

		versionSrc, ok := src.(repos.VersionSource)
		if !ok {
			logger.Debug("external service source does not implement VersionSource interface",
				log.String("kind", svc.Kind))
			continue
		}

		v, err := versionSrc.Version(ctx)
		if err != nil {
			logger.Warn("failed to fetch version of code host",
				log.String("version", v),
				log.Error(err))
			continue
		}

		versions = append(versions, &Version{
			ExternalServiceKind: svc.Kind,
			Version:             v,
			Key:                 svc.URN(),
		})
	}

	return versions, nil
}
