package client

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/telemetry"
)

func newObservedClient(logger log.Logger, events *telemetry.EventRecorder, inner types.CompletionsClient) *observedClient {
	observationCtx := observation.NewContext(logger.Scoped("completions"))
	ops := newOperations(observationCtx)
	return &observedClient{
		inner:  inner,
		ops:    ops,
		events: telemetry.NewBestEffortEventRecorder(logger.Scoped("events"), events),
		logger: logger,
	}
}

type observedClient struct {
	inner  types.CompletionsClient
	ops    *operations
	events *telemetry.BestEffortEventRecorder
	logger log.Logger
}

var _ types.CompletionsClient = (*observedClient)(nil)

func (o *observedClient) Stream(ctx context.Context, logger log.Logger, request types.CompletionRequest, responseMetadataCapture *types.ResponseMetadataCapture) (err error) {
	feature := request.Feature
	version := request.Version
	params := request.Parameters

	ctx, tr, endObservation := o.ops.stream.With(ctx, &err, observation.Args{
		Attrs: append(
			params.Attrs(feature),
			attribute.String("feature", string(feature)),
			attribute.Int("version", int(version))),
		MetricLabelValues: []string{params.Model},
	})
	defer endObservation(1, observation.Args{})

	originalCaptureEvent := responseMetadataCapture.SendEvent
	responseMetadataCapture.SendEvent = func(event types.CompletionResponse) error {
		if event.StopReason != "" {
			tr.AddEvent("stopped", attribute.String("reason", event.StopReason))
		} else {
			tr.AddEvent("completion", attribute.Int("len", len(event.Completion)))
		}

		return originalCaptureEvent(event)
	}

	return o.inner.Stream(ctx, logger, request, responseMetadataCapture)
}

func (o *observedClient) Complete(ctx context.Context, logger log.Logger, request types.CompletionRequest) (resp *types.CompletionResponse, err error) {
	feature := request.Feature
	version := request.Version
	params := request.Parameters

	ctx, _, endObservation := o.ops.complete.With(ctx, &err, observation.Args{
		Attrs: append(
			params.Attrs(feature),
			attribute.String("feature", string(feature)),
			attribute.Int("version", int(version))),
		MetricLabelValues: []string{params.Model},
	})
	defer endObservation(1, observation.Args{})

	defer o.events.Record(ctx, "cody.completions", "complete", &telemetry.EventParameters{
		Metadata: telemetry.EventMetadata{
			"feature": float64(feature.ID()),
		},
	})

	return o.inner.Complete(ctx, logger, request)
}

type operations struct {
	stream   *observation.Operation
	complete *observation.Operation
}

var (
	durationBuckets = []float64{0.5, 1.0, 1.5, 2.0, 3.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 25.0, 30.0, 40.0}
	streamMetrics   = metrics.NewREDMetrics(
		prometheus.DefaultRegisterer,
		"completions_stream",
		metrics.WithLabels("model"),
		metrics.WithDurationBuckets(durationBuckets),
	)
	completeMetrics = metrics.NewREDMetrics(
		prometheus.DefaultRegisterer,
		"completions_complete",
		metrics.WithLabels("model"),
		metrics.WithDurationBuckets(durationBuckets),
	)
)

func newOperations(observationCtx *observation.Context) *operations {
	streamOp := observationCtx.Operation(observation.Op{
		Metrics: streamMetrics,
		Name:    "completions.stream",
	})
	completeOp := observationCtx.Operation(observation.Op{
		Metrics: completeMetrics,
		Name:    "completions.complete",
	})
	return &operations{
		stream:   streamOp,
		complete: completeOp,
	}
}
