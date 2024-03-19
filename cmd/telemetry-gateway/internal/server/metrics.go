package server

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.GetMeterProvider().Meter("cmd/telemetry-gateway/internal/server")

type recordEventsMetrics struct {
	// Record total event lengths of requests
	totalLength metric.Int64Histogram
	// Record per-payload metrics
	payload recordEventsRequestPayloadMetrics
}

type recordEventsRequestPayloadMetrics struct {
	// Record event length of individual payloads
	length metric.Int64Histogram
	// Count of processed events
	processedEvents metric.Int64Counter
}

func newRecordEventsMetrics() (m recordEventsMetrics, err error) {
	m.totalLength, err = meter.Int64Histogram(
		"telemetry-gateway.record_events.total_length",
		metric.WithDescription("Total number of events in record_events requests"))
	if err != nil {
		return m, err
	}

	m.payload.length, err = meter.Int64Histogram(
		"telemetry-gateway.record_events.payload_length",
		metric.WithDescription("Number of events in indvidiual record_events request payloads"))
	if err != nil {
		return m, err
	}
	m.payload.processedEvents, err = meter.Int64Counter(
		"telemetry-gateway.record_events.processed_events",
		metric.WithDescription("Number of events processed"))
	if err != nil {
		return m, err
	}

	return m, err
}

type recordEventMetrics struct {
	// Count of processed events
	processedEvents metric.Int64Counter
}

func newRecordEventMetrics() (m recordEventMetrics, err error) {
	m.processedEvents, err = meter.Int64Counter(
		"telemetry-gateway.record_event.processed_events",
		metric.WithDescription("Number of events processed"))
	if err != nil {
		return m, err
	}

	return m, nil
}
