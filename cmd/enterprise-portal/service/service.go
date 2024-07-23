package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"connectrpc.com/otelconnect"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph-accounts-sdk-go/scopes"

	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/codyaccessservice"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/importer"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/routines/licenseexpiration"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/subscriptionsservice"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/background"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

// Service is the implementation of the Enterprise Portal service.
type Service struct{}

var _ runtime.Service[Config] = (*Service)(nil)

func (Service) Name() string    { return "enterprise-portal" }
func (Service) Version() string { return version.Version() }

func (Service) Initialize(ctx context.Context, logger log.Logger, contract runtime.ServiceContract, config Config) (background.Routine, error) {
	// We use Sourcegraph tracing code, so explicitly configure a trace policy
	policy.SetTracePolicy(policy.TraceAll)

	redisClient, err := newRedisClient(contract.MSP, contract.RedisEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "initialize Redis client")
	}
	redisKVClient := newRedisKVClient(contract.RedisEndpoint)

	dbHandle, err := database.NewHandle(ctx, logger, contract.Contract, redisClient, version.Version())
	if err != nil {
		return nil, errors.Wrap(err, "initialize database handle")
	}

	dotcomDB, err := newDotComDBConn(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "initialize dotcom database handle")
	}

	// Prepare SAMS client, so that we can enforce SAMS-based M2M authz/authn
	logger.Debug("using SAMS client",
		log.String("samsExternalURL", config.SAMS.ExternalURL),
		log.Stringp("samsAPIURL", config.SAMS.APIURL),
		log.String("clientID", config.SAMS.ClientID))
	samsClient, err := sams.NewClientV1(
		sams.ClientV1Config{
			ConnConfig: config.SAMS.ConnConfig,
			TokenSource: sams.ClientCredentialsTokenSource(
				config.SAMS.ConnConfig,
				config.SAMS.ClientID,
				config.SAMS.ClientSecret,
				[]scopes.Scope{scopes.OpenID, scopes.Profile, scopes.Email},
			),
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, "create Sourcegraph Accounts client")
	}

	iamClient, closeIAMClient, err := newIAMClient(ctx, logger, contract.Contract, redisClient)
	if err != nil {
		return nil, errors.Wrap(err, "initialize IAM client")
	}

	httpServer := http.NewServeMux()

	// Register MSP endpoints
	contract.Diagnostics.RegisterDiagnosticsHandlers(httpServer, serviceState{
		dotcomDB: dotcomDB,
	})

	// Prepare instrumentation middleware for ConnectRPC handlers
	otelConnctInterceptor, err := otelconnect.NewInterceptor(
		// Keep data low-cardinality
		otelconnect.WithoutServerPeerAttributes(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "create OTEL interceptor")
	}

	// Register connect endpoints
	codyaccessservice.RegisterV1(
		logger,
		httpServer,
		codyaccessservice.NewStoreV1(
			codyaccessservice.StoreV1Options{
				SAMSClient: samsClient,
				DB:         dbHandle,
				CodyGatewayEvents: newCodyGatewayEventsService(
					logger.Scoped("codygatewayevents"),
					config.CodyGatewayEvents),
			},
		),
		connect.WithInterceptors(otelConnctInterceptor),
	)
	subscriptionsservice.RegisterV1(
		logger,
		httpServer,
		subscriptionsservice.NewStoreV1(
			subscriptionsservice.NewStoreV1Options{
				DB:                     dbHandle,
				SAMSClient:             samsClient,
				IAMClient:              iamClient,
				LicenseKeySigner:       config.LicenseKeys.Signer,
				LicenseKeyRequiredTags: config.LicenseKeys.RequiredTags,
			},
		),
		connect.WithInterceptors(otelConnctInterceptor),
	)

	// Optionally enable reflection handlers and a debug UI
	listenAddr := fmt.Sprintf(":%d", contract.Port)
	if !contract.MSP && debugserver.GRPCWebUIEnabled {
		// Enable reflection for the web UI
		reflector := grpcreflect.NewStaticReflector(
			codyaccessservice.Name,
			subscriptionsservice.Name,
		)
		httpServer.Handle(grpcreflect.NewHandlerV1(reflector))
		httpServer.Handle(grpcreflect.NewHandlerV1Alpha(reflector)) // web UI still requires old API
		// Enable the web UI
		grpcUI := debugserver.NewGRPCWebUIEndpoint("enterprise-portal", listenAddr)
		httpServer.Handle(grpcUI.Path, grpcUI.Handler)
		logger.Warn("gRPC web UI enabled", log.String("url", fmt.Sprintf("%s%s", listenAddr, grpcUI.Path)))
	}

	// Initialize server
	server := httpserver.NewFromAddr(
		listenAddr,
		&http.Server{
			Addr: listenAddr,
			// Cloud Run only supports HTTP/2 if the service accepts HTTP/2 cleartext (h2c),
			// see https://cloud.google.com/run/docs/configuring/http2
			Handler: h2c.NewHandler(
				otelhttp.NewHandler(
					httpServer,
					"handler",
					// Don't trust incoming spans, start our own.
					otelhttp.WithPublicEndpoint(),
					// Generate custom span names from the request, the default is very vague.
					otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
						// Prefix with 'handle' because outgoing HTTP requests can have similar-looking
						// spans.
						return fmt.Sprintf("handle.%s %s", r.Method, r.URL.Path)
					}),
				),
				&http2.Server{},
			),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: time.Minute,
		},
	)
	return background.LIFOStopRoutine{
		// Everything else can be stopped in conjunction
		background.CombinedRoutine{
			background.CallbackRoutine{
				NameFunc: func() string { return "close IAM client" },
				StopFunc: func(context.Context) error {
					closeIAMClient()
					return nil
				},
			},
			background.CallbackRoutine{
				NameFunc: func() string { return "close database handle" },
				StopFunc: func(context.Context) error {
					start := time.Now()
					// NOTE: If we simply shut down, some in-fly requests may be dropped as the
					// service exits, so we attempt to gracefully shutdown first.
					dbHandle.Close()
					logger.Info("database handle closed", log.Duration("elapsed", time.Since(start)))
					return nil
				},
			},
			background.CallbackRoutine{
				NameFunc: func() string { return "close dotcom database connection pool" },
				StopFunc: func(context.Context) error {
					start := time.Now()
					// NOTE: If we simply shut down, some in-fly requests may be dropped as the
					// service exits, so we attempt to gracefully shutdown first.
					dotcomDB.Close()
					logger.Info("dotcom database connection pool closed", log.Duration("elapsed", time.Since(start)))
					return nil
				},
			},
		},
		// Run background routines
		background.CombinedRoutine{
			importer.NewPeriodicImporter(ctx, logger.Scoped("importer"), dotcomDB, dbHandle, redisKVClient, config.DotComDB.ImportInterval),
			licenseexpiration.NewRoutine(ctx, logger.Scoped("licenseexpiration"),
				licenseexpiration.NewStore(
					logger.Scoped("licenseexpiration.store"),
					dbHandle.Subscriptions(),
					redisKVClient,
					config.LicenseExpirationChecker),
			),
		},
		// Stop server first
		server,
	}, nil
}
