package codenav

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	getReferences                     *observation.Operation
	getImplementations                *observation.Operation
	getPrototypes                     *observation.Operation
	getDiagnostics                    *observation.Operation
	getHover                          *observation.Operation
	getDefinitions                    *observation.Operation
	getRanges                         *observation.Operation
	getStencil                        *observation.Operation
	getClosestCompletedUploadsForBlob *observation.Operation
	snapshotForDocument               *observation.Operation
	visibleUploadsForPath             *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_codenav",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.codenav.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		getReferences:                     op("getReferences"),
		getImplementations:                op("getImplementations"),
		getPrototypes:                     op("getPrototypes"),
		getDiagnostics:                    op("getDiagnostics"),
		getHover:                          op("getHover"),
		getDefinitions:                    op("getDefinitions"),
		getRanges:                         op("getRanges"),
		getStencil:                        op("getStencil"),
		getClosestCompletedUploadsForBlob: op("GetClosestCompletedUploadsForBlob"),
		snapshotForDocument:               op("SnapshotForDocument"),
		visibleUploadsForPath:             op("VisibleUploadsForPath"),
	}
}

var serviceObserverThreshold = time.Second

func observeResolver(ctx context.Context, err *error, operation *observation.Operation, threshold time.Duration, observationArgs observation.Args) (context.Context, observation.TraceLogger, func()) {
	start := time.Now()
	ctx, trace, endObservation := operation.With(ctx, err, observationArgs)

	return ctx, trace, func() {
		duration := time.Since(start)
		endObservation(1, observation.Args{})

		if duration >= threshold {
			// use trace logger which includes all relevant fields
			lowSlowRequest(trace, duration, err)
		}
	}
}

func lowSlowRequest(logger log.Logger, duration time.Duration, err *error) {
	fields := []log.Field{log.Duration("duration", duration)}
	if err != nil && *err != nil {
		fields = append(fields, log.Error(*err))
	}

	logger.Warn("Slow codeintel request", fields...)
}
