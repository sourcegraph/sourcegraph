package dbworker

import (
	"context"
	"fmt"
	"time"

	"github.com/derision-test/glock"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/internal/workerutil/dbworker/store"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Resetter periodically moves all unlocked records that have been in the processing state
// for a while back to queued.
//
// An unlocked record signifies that it is not actively being processed and records in this
// state for more than a few seconds are very likely to be stuck after the worker processing
// them has crashed.
type Resetter[T workerutil.Record] struct {
	store    store.Store[T]
	options  ResetterOptions
	clock    glock.Clock
	ctx      context.Context // root context passed to the database
	cancel   func()          // cancels the root context
	finished chan struct{}   // signals that Start has finished
	logger   log.Logger
}

type ResetterOptions struct {
	Name     string
	Interval time.Duration
	Metrics  ResetterMetrics
}

type ResetterMetrics struct {
	RecordResets        prometheus.Counter
	RecordResetFailures prometheus.Counter
	Errors              prometheus.Counter
}

// NewResetterMetrics returns a metrics object for a resetter that follows
// standard naming convention. The base metric name should be the same metric
// name provided to a `worker` ex. my_job_queue. Do not provide prefix "src" or
// postfix "_record...".
func NewResetterMetrics(observationCtx *observation.Context, metricNameRoot string) ResetterMetrics {
	resets := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_" + metricNameRoot + "_record_resets_total",
		Help: "The number of stalled record resets.",
	})
	observationCtx.Registerer.MustRegister(resets)

	resetFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_" + metricNameRoot + "_record_reset_failures_total",
		Help: "The number of stalled record resets marked as failure.",
	})
	observationCtx.Registerer.MustRegister(resetFailures)

	resetErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_" + metricNameRoot + "_record_reset_errors_total",
		Help: "The number of errors that occur during stalled " +
			"record reset.",
	})
	observationCtx.Registerer.MustRegister(resetErrors)

	return ResetterMetrics{
		RecordResets:        resets,
		RecordResetFailures: resetFailures,
		Errors:              resetErrors,
	}
}

func NewResetter[T workerutil.Record](ctx context.Context, logger log.Logger, store store.Store[T], options ResetterOptions) *Resetter[T] {
	return newResetter(ctx, logger, store, options, glock.NewRealClock())
}

func newResetter[T workerutil.Record](ctx context.Context, logger log.Logger, store store.Store[T], options ResetterOptions, clock glock.Clock) *Resetter[T] {
	if options.Name == "" {
		panic("no name supplied to github.com/sourcegraph/sourcegraph/internal/dbworker/newResetter")
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Resetter[T]{
		store:    store,
		options:  options,
		clock:    clock,
		ctx:      ctx,
		cancel:   cancel,
		finished: make(chan struct{}),
		logger:   logger,
	}
}

func (r *Resetter[T]) Name() string {
	return fmt.Sprintf("dbworker.Resetter[%s]", r.options.Name)
}

// Start begins periodically calling reset stalled on the underlying store.
func (r *Resetter[T]) Start() {
	defer close(r.finished)

loop:
	for {
		resetLastHeartbeatsByIDs, failedLastHeartbeatsByIDs, err := r.store.ResetStalled(r.ctx)
		if err != nil {
			if r.ctx.Err() != nil && errors.Is(err, r.ctx.Err()) {
				// If the error is due to the loop being shut down, just break
				break loop
			}

			r.options.Metrics.Errors.Inc()
			r.logger.Error("Failed to reset stalled records", log.String("name", r.options.Name), log.Error(err))
		}

		for id, lastHeartbeatAge := range resetLastHeartbeatsByIDs {
			r.logger.Warn("Reset stalled record back to 'queued' state", log.String("name", r.options.Name), log.Int("id", id), log.Duration("timeSinceLastHeartbeat", lastHeartbeatAge))
		}
		for id, lastHeartbeatAge := range failedLastHeartbeatsByIDs {
			r.logger.Warn("Reset stalled record to 'failed' state", log.String("name", r.options.Name), log.Int("id", id), log.Duration("timeSinceLastHeartbeat", lastHeartbeatAge))
		}

		r.options.Metrics.RecordResets.Add(float64(len(resetLastHeartbeatsByIDs)))
		r.options.Metrics.RecordResetFailures.Add(float64(len(failedLastHeartbeatsByIDs)))

		select {
		case <-r.clock.After(r.options.Interval):
		case <-r.ctx.Done():
			return
		}
	}
}

// Stop will cause the resetter loop to exit after the current iteration.
func (r *Resetter[T]) Stop(ctx context.Context) error {
	r.cancel()
	select {
	case <-r.finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
