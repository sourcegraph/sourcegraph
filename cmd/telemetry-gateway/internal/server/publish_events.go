package server

import (
	"context"
	"fmt"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/sourcegraph/sourcegraph/cmd/telemetry-gateway/internal/events"
	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
	sgtrace "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func handlePublishEvents(
	ctx context.Context,
	logger log.Logger,
	payloadMetrics *recordEventsRequestPayloadMetrics,
	publisher *events.Publisher,
	events []*telemetrygatewayv1.Event,
) *telemetrygatewayv1.RecordEventsResponse {
	var tr sgtrace.Trace
	tr, ctx = sgtrace.New(ctx, "handlePublishEvents",
		attribute.Int("events", len(events)))
	defer tr.End()

	logger = sgtrace.Logger(ctx, logger)

	// Send off our events
	results := publisher.Publish(ctx, events)

	// Aggregate failure details
	summary := summarizePublishEventsResults(results)

	// Record the result on the trace and metrics
	resultAttribute := attribute.String("result", summary.result)
	sourceAttribute := attribute.String("source", publisher.GetSourceName())
	tr.SetAttributes(resultAttribute, sourceAttribute)
	payloadMetrics.length.Record(ctx, int64(len(events)),
		metric.WithAttributes(resultAttribute, sourceAttribute))
	payloadMetrics.processedEvents.Add(ctx, int64(len(summary.succeededEvents)),
		metric.WithAttributes(attribute.Bool("succeeded", true), resultAttribute, sourceAttribute))
	payloadMetrics.processedEvents.Add(ctx, int64(len(summary.failedEvents)),
		metric.WithAttributes(attribute.Bool("succeeded", false), resultAttribute, sourceAttribute))

	// Generate a log message for convenience
	summaryFields := []log.Field{
		log.String("result", summary.result),
		log.Int("submitted", len(events)),
		log.Int("succeeded", len(summary.succeededEvents)),
		log.Int("failed", len(summary.failedEvents)),
	}
	if len(summary.failedEvents) > 0 {
		tr.SetError(errors.New(summary.message)) // mark span as failed
		tr.SetAttributes(attribute.Int("failed", len(summary.failedEvents)))
		logger.Error(summary.message, append(summaryFields, summary.errorFields...)...)
	} else {
		logger.Info(summary.message, summaryFields...)
	}

	return &telemetrygatewayv1.RecordEventsResponse{
		SucceededEvents: summary.succeededEvents,
	}
}

type publishEventsSummary struct {
	// message is a human-readable summary summarizing the result
	message string
	// result is a low-cardinality indicator of the result category
	result string

	errorFields     []log.Field
	succeededEvents []string
	failedEvents    []events.PublishEventResult
}

func summarizePublishEventsResults(results []events.PublishEventResult) publishEventsSummary {
	var (
		errFields = make([]log.Field, 0)
		succeeded = make([]string, 0, len(results))
		failed    = make([]events.PublishEventResult, 0)
	)

	for i, result := range results {
		if result.PublishError != nil {
			failed = append(failed, result)
			errFields = append(errFields, log.NamedError(fmt.Sprintf("error.%d", i), result.PublishError))
		} else {
			succeeded = append(succeeded, result.EventID)
		}
	}

	var message, category string
	switch {
	case len(failed) == len(results):
		message = "all events in batch failed to submit"
		category = "complete_failure"
	case len(failed) > 0 && len(failed) < len(results):
		message = "some events in batch failed to submit"
		category = "partial_failure"
	case len(failed) == 0:
		message = "all events in batch submitted successfully"
		category = "success"
	}

	return publishEventsSummary{
		message:         message,
		result:          category,
		errorFields:     errFields,
		succeededEvents: succeeded,
		failedEvents:    failed,
	}
}
