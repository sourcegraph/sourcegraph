package background

import (
	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
)

type batchChangesMetrics struct {
	workerMetrics workerutil.WorkerMetrics
	resets        prometheus.Counter
	resetFailures prometheus.Counter
	errors        prometheus.Counter
}

func newMetrics() batchChangesMetrics {
	observationContext := &observation.Context{
		Logger:     log15.Root(),
		Tracer:     &trace.Tracer{Tracer: opentracing.GlobalTracer()},
		Registerer: prometheus.DefaultRegisterer,
	}

	resetFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_batch_changes_background_reconciler_reset_failures_total",
		Help: "The number of reconciler reset failures.",
	})
	observationContext.Registerer.MustRegister(resetFailures)

	resets := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_batch_changes_background_reconciler_resets_total",
		Help: "The number of reconciler records reset.",
	})
	observationContext.Registerer.MustRegister(resets)

	errors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "src_batch_changes_background_errors_total",
		Help: "The number of errors that occur during a batch changes background job.",
	})
	observationContext.Registerer.MustRegister(errors)

	return batchChangesMetrics{
		workerMetrics: workerutil.NewMetrics(observationContext, "batch_changes_reconciler", nil),
		resets:        resets,
		resetFailures: resetFailures,
		errors:        errors,
	}
}
