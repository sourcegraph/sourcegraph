package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/pubsub"
	"github.com/sourcegraph/sourcegraph/internal/sams"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/internal/version"

	"github.com/sourcegraph/sourcegraph/lib/background"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"

	"github.com/sourcegraph/sourcegraph/cmd/telemetry-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/telemetry-gateway/internal/server"
	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
)

var meter = otel.GetMeterProvider().Meter("cmd/telemetry-gateway/service")

type Service struct{}

var _ runtime.Service[Config] = (*Service)(nil)

func (Service) Name() string    { return "telemetry-gateway" }
func (Service) Version() string { return version.Version() }

func (Service) Initialize(ctx context.Context, logger log.Logger, contract runtime.Contract, config Config) (background.Routine, error) {
	// We use Sourcegraph tracing code, so explicitly configure a trace policy
	policy.SetTracePolicy(policy.TraceAll)

	// Prepare pubsub client
	var err error
	var eventsTopic pubsub.TopicClient
	if !config.Events.PubSub.Enabled {
		logger.Warn("pub/sub events publishing disabled, logging messages instead")
		eventsTopic = pubsub.NewLoggingTopicClient(logger)
	} else {
		eventsTopic, err = pubsub.NewTopicClient(*config.Events.PubSub.ProjectID, *config.Events.PubSub.TopicID)
		if err != nil {
			return nil, errors.Errorf("create Events Pub/Sub client: %v", err)
		}
	}
	publishMessageBytes, err := meter.Int64Histogram(
		"telemetry-gateway.pubsub.published_message_size",
		metric.WithUnit("By"), // UCUM for "bytes": https://github.com/open-telemetry/opentelemetry-specification/issues/2973#issuecomment-1430035419
		metric.WithDescription("Size of published messages in bytes"))
	if err != nil {
		return nil, errors.Wrap(err, "create pubsub.published_message_size metric")
	}

	// Prepare SAMS client, so that we can enforce SAMS-based M2M authz/authn
	logger.Debug("using SAMS client",
		log.String("samsServer", config.SAMS.ServerURL),
		log.String("clientID", config.SAMS.ClientID))
	samsClient := sams.NewClient(config.SAMS.ServerURL, clientcredentials.Config{
		ClientID:     config.SAMS.ClientID,
		ClientSecret: config.SAMS.ClientSecret,
		TokenURL:     fmt.Sprintf("%s/oauth/token", config.SAMS.ServerURL),
		Scopes:       []string{"openid", "profile", "email"},
	})

	// Initialize our gRPC server
	grpcServer := defaults.NewPublicServer(logger)
	telemetryGatewayServer, err := server.New(
		logger,
		eventsTopic,
		samsClient,
		events.PublishStreamOptions{
			ConcurrencyLimit:     config.Events.StreamPublishConcurrency,
			MessageSizeHistogram: publishMessageBytes,
		})
	if err != nil {
		return nil, errors.Wrap(err, "init telemetry gateway server")
	}
	telemetrygatewayv1.RegisterTelemeteryGatewayServiceServer(grpcServer, telemetryGatewayServer)

	listenAddr := fmt.Sprintf(":%d", contract.Port)

	// Set up diagnostics endpoints
	diagnosticsServer := http.NewServeMux()
	contract.RegisterDiagnosticsHandlers(diagnosticsServer, &serviceStatus{
		eventsTopic: eventsTopic,
	})
	if !contract.MSP {
		// Requires GRPC_WEB_UI_ENABLED to be set to enable - only use in local
		// development!
		grpcUI := debugserver.NewGRPCWebUIEndpoint("telemetry-gateway", listenAddr)
		diagnosticsServer.Handle(grpcUI.Path, grpcUI.Handler)
		logger.Warn("gRPC web UI enabled", log.String("url", fmt.Sprintf("%s%s", listenAddr, grpcUI.Path)))
	}

	return background.LIFOStopRoutine{
		httpserver.NewFromAddr(
			listenAddr,
			&http.Server{
				ReadTimeout:  2 * time.Minute,
				WriteTimeout: 2 * time.Minute,
				Handler: internalgrpc.MultiplexHandlers(
					grpcServer,
					diagnosticsServer,
				),
			},
		),
		background.CallbackRoutine{
			// No Start - serving is handled by httpserver
			StopFunc: func() {
				grpcServer.GracefulStop()
				eventsTopic.Stop()
			},
		},
	}, nil
}

type serviceStatus struct {
	eventsTopic pubsub.TopicClient
}

var _ runtime.ServiceState = (*serviceStatus)(nil)

func (s *serviceStatus) Healthy(ctx context.Context, _ url.Values) error {
	if err := s.eventsTopic.Ping(ctx); err != nil {
		return errors.Wrap(err, "eventsPubSubClient.Ping")
	}
	return nil
}
