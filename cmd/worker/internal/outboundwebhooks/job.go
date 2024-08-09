package outboundwebhooks

import (
	"context"
	"net/http"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/worker/job"
	workerdb "github.com/sourcegraph/sourcegraph/cmd/worker/shared/init/db"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type sender struct{}

func NewSender() job.Job {
	return &sender{}
}

func (s *sender) Description() string {
	return "Outbound webhook sender"
}

func (*sender) Config() []env.Config {
	return nil
}

func (s *sender) Routines(ctx context.Context, observationCtx *observation.Context) ([]goroutine.BackgroundRoutine, error) {
	observationCtx = observation.NewContext(observationCtx.Logger.Scoped("sender"))
	ctx = actor.WithInternalActor(ctx)

	db, err := workerdb.InitDB(observationCtx)
	if err != nil {
		return nil, errors.Wrap(err, "initialising database")
	}

	client := httpcli.ExternalClient
	key := keyring.Default().OutboundWebhookKey
	workerStore := makeStore(observationCtx, db.Handle(), key)

	return []goroutine.BackgroundRoutine{
		makeWorker(
			ctx, observationCtx, workerStore, client,
			database.OutboundWebhooksWith(db, key),
			database.OutboundWebhookLogsWith(db, key),
		),
		makeResetter(ctx, observationCtx, workerStore),
		makeJanitor(observationCtx, db.OutboundWebhookJobs(key)),
	}, nil
}

func makeWorker(
	ctx context.Context,
	observationCtx *observation.Context,
	workerStore store.Store[*types.OutboundWebhookJob],
	client *http.Client,
	webhookStore database.OutboundWebhookStore,
	logStore database.OutboundWebhookLogStore,
) *workerutil.Worker[*types.OutboundWebhookJob] {
	handler := &handler{
		client:   client,
		store:    webhookStore,
		logStore: logStore,
	}

	return dbworker.NewWorker[*types.OutboundWebhookJob](
		ctx, workerStore, handler, workerutil.WorkerOptions{
			Name:              "outbound_webhook_job_worker",
			Interval:          time.Second,
			NumHandlers:       1,
			HeartbeatInterval: 10 * time.Second,
			Metrics:           workerutil.NewMetrics(observationCtx, "outbound_webhook_job_worker"),
		},
	)
}

func makeResetter(
	ctx context.Context,
	observationCtx *observation.Context,
	workerStore store.Store[*types.OutboundWebhookJob],
) *dbworker.Resetter[*types.OutboundWebhookJob] {
	return dbworker.NewResetter(
		ctx, observationCtx.Logger, workerStore, dbworker.ResetterOptions{
			Name:     "outbound_webhook_job_resetter",
			Interval: 5 * time.Minute,
			Metrics:  dbworker.NewResetterMetrics(observationCtx, "outbound_webhook_job_resetter"),
		},
	)
}
