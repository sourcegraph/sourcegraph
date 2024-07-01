package otlpadapter

import (
	"context"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"

	"go.uber.org/atomic"
	"go.uber.org/zap" //nolint:logging // dependencies require direct usage of zap

	"github.com/sourcegraph/sourcegraph/internal/otlpenv"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Register sets up adapter services and registers proxies on the router. enabled can be
// provided to atomically toggle whether the signal endpoints are enabled serverside -
// this is important because clients might not receive updated configuration for quite a
// while, so we need to stop accepting incoming requests.
func Register(ctx context.Context, logger log.Logger, protocol otlpenv.Protocol, endpoint string, r *mux.Router, enabled *atomic.Bool) {
	// Build an OTLP exporter that exports directly to the desired protocol and endpoint
	exporterFactory, signalExporterConfig, err := newExporter(protocol, endpoint)
	if err != nil {
		logger.Fatal("newExporter", log.Error(err))
	}

	// Receive OTLP http/json signals
	receiverURL, err := url.Parse("http://127.0.0.1:4319")
	if err != nil {
		logger.Fatal("unreachable", log.Error(err))
	}
	receiverFactory, signalReceiverConfig := newReceiver(receiverURL)

	// Set up shared configuration for creating signal exporters and receivers. Telemetry
	// settigns are required on all factories, and all fields of this struct are required.
	telemetrySettings := component.TelemetrySettings{
		Logger: zap.NewNop(),

		TracerProvider: otel.GetTracerProvider(),

		MeterProvider: metric.NewMeterProvider(),
		MetricsLevel:  configtelemetry.LevelBasic,

		ReportStatus: func(event *component.StatusEvent) {
			if err := event.Err(); err != nil {
				logger.Warn(event.Status().String(), log.Error(err))
			}
		},
	}
	componentName := "otlpadapter"

	// otelSignals declares the signals we support redirection for.
	var otelSignals = []adaptedSignal{
		{
			PathPrefix: "/v1/traces",
			CreateAdapter: func() (*signalAdapter, error) {
				exporter, err := exporterFactory.CreateTracesExporter(ctx, exporter.Settings{
					ID:                component.NewIDWithName(component.DataTypeTraces, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalExporterConfig)
				if err != nil {
					return nil, errors.Wrap(err, "CreateTracesExporter")
				}
				receiver, err := receiverFactory.CreateTracesReceiver(ctx, receiver.Settings{
					ID:                component.NewIDWithName(component.DataTypeTraces, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalReceiverConfig, exporter)
				if err != nil {
					return nil, errors.Wrap(err, "CreateTracesReceiver")
				}
				return &signalAdapter{Exporter: exporter, Receiver: receiver}, nil
			},
			Enabled: enabled,
		},
		{
			PathPrefix: "/v1/metrics",
			CreateAdapter: func() (*signalAdapter, error) {
				exporter, err := exporterFactory.CreateMetricsExporter(ctx, exporter.Settings{
					ID:                component.NewIDWithName(component.DataTypeMetrics, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalExporterConfig)
				if err != nil {
					return nil, errors.Wrap(err, "CreateMetricsExporter")
				}
				receiver, err := receiverFactory.CreateMetricsReceiver(ctx, receiver.Settings{
					ID:                component.NewIDWithName(component.DataTypeMetrics, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalReceiverConfig, exporter)
				if err != nil {
					return nil, errors.Wrap(err, "CreateMetricsReceiver")
				}
				return &signalAdapter{Exporter: exporter, Receiver: receiver}, nil
			},
			Enabled: enabled,
		},
		{
			PathPrefix: "/v1/logs",
			CreateAdapter: func() (*signalAdapter, error) {
				exporter, err := exporterFactory.CreateLogsExporter(ctx, exporter.Settings{
					ID:                component.NewIDWithName(component.DataTypeLogs, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalExporterConfig)
				if err != nil {
					return nil, errors.Wrap(err, "CreateLogsExporter")
				}
				receiver, err := receiverFactory.CreateLogsReceiver(ctx, receiver.Settings{
					ID:                component.NewIDWithName(component.DataTypeLogs, componentName),
					TelemetrySettings: telemetrySettings,
				}, signalReceiverConfig, exporter)
				if err != nil {
					return nil, errors.Wrap(err, "CreateLogsReceiver")
				}
				return &signalAdapter{Exporter: exporter, Receiver: receiver}, nil
			},
			Enabled: enabled,
		},
	}

	// Finally, spin up redirectors for each signal and set up the appropriate endpoints.
	for _, otelSignal := range otelSignals {
		otelSignal := otelSignal // copy
		otelSignal.Register(ctx, logger, r, receiverURL)
	}
}
