// Package defaults exports a set of default options for gRPC servers
// and clients in Sourcegraph. It is a separate subpackage so that all
// packages that depend on the internal/grpc package do not need to
// depend on the large dependency tree of this package.
package defaults

import (
	"context"
	"crypto/tls"
	"sync"

	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/contextconv"
	"github.com/sourcegraph/sourcegraph/internal/grpc/internalerrs"
	"github.com/sourcegraph/sourcegraph/internal/grpc/messagesize"
	"github.com/sourcegraph/sourcegraph/internal/grpc/propagator"
	"github.com/sourcegraph/sourcegraph/internal/grpc/retry"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/tenant"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
)

// Dial creates a client connection to the given target with the default options.
func Dial(addr string, logger log.Logger, additionalOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return DialContext(context.Background(), addr, logger, additionalOpts...)
}

// DialContext creates a client connection to the given target with the default options.
func DialContext(ctx context.Context, addr string, logger log.Logger, additionalOpts ...grpc.DialOption) (*grpc.ClientConn, error) {
	//lint:ignore SA1019 DialContext will be supported throughout 1.x
	return grpc.DialContext(ctx, addr, DialOptions(logger, additionalOpts...)...)
}

// defaultGRPCMessageReceiveSizeBytes is the default message size that gRPCs servers and clients are allowed to process.
// This can be overridden by providing custom Server/Dial options.
const defaultGRPCMessageReceiveSizeBytes = 90 * 1024 * 1024 // 90 MB

// DialOptions is a set of default dial options that should be used for all
// gRPC clients in Sourcegraph, along with any additional client-specific options.
//
// **Note**: Do not append to this slice directly, instead provide extra options
// via "additionalOptions".
func DialOptions(logger log.Logger, additionalOptions ...grpc.DialOption) []grpc.DialOption {
	return defaultDialOptions(logger, insecure.NewCredentials(), additionalOptions...)
}

// ExternalDialOptions is a set of default dial options that should be used for
// gRPC clients external to a Sourcegraph deployment, e.g. Telemetry Gateway,
// along with any additional client-specific options. In particular, these
// options enforce TLS.
//
// Traffic within a Sourcegraph deployment should use DialOptions instead.
//
// **Note**: Do not append to this slice directly, instead provide extra options
// via "additionalOptions".
func ExternalDialOptions(logger log.Logger, additionalOptions ...grpc.DialOption) []grpc.DialOption {
	return defaultDialOptions(logger, credentials.NewTLS(&tls.Config{}), additionalOptions...)
}

func defaultDialOptions(logger log.Logger, creds credentials.TransportCredentials, additionalOptions ...grpc.DialOption) []grpc.DialOption {
	// Generate the options dynamically rather than using a static slice
	// because these options depend on some globals (tracer, trace sampling)
	// that are not initialized during init time.

	metrics := mustGetClientMetrics()

	out := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithChainStreamInterceptor(
			propagator.StreamClientPropagator(tenant.TenantPropagator{}),
			propagator.StreamClientPropagator(actor.ActorPropagator{}),
			propagator.StreamClientPropagator(policy.ShouldTracePropagator{}),
			propagator.StreamClientPropagator(requestclient.Propagator{}),
			propagator.StreamClientPropagator(requestinteraction.Propagator{}),
			otelgrpc.StreamClientInterceptor(), //lint:ignore SA1019 the advertised replacement doesn't seem to be a drop-in replacement, use deprecated mechanism for now
			retry.StreamClientInterceptor(logger),
			metrics.StreamClientInterceptor(),
			messagesize.StreamClientInterceptor,
			internalerrs.PrometheusStreamClientInterceptor,
			internalerrs.LoggingStreamClientInterceptor(logger),
			contextconv.StreamClientInterceptor,
		),
		grpc.WithChainUnaryInterceptor(
			propagator.UnaryClientPropagator(tenant.TenantPropagator{}),
			propagator.UnaryClientPropagator(actor.ActorPropagator{}),
			propagator.UnaryClientPropagator(policy.ShouldTracePropagator{}),
			propagator.UnaryClientPropagator(requestclient.Propagator{}),
			propagator.UnaryClientPropagator(requestinteraction.Propagator{}),
			otelgrpc.UnaryClientInterceptor(), //lint:ignore SA1019 the advertised replacement doesn't seem to be a drop-in replacement, use deprecated mechanism for now
			retry.UnaryClientInterceptor(logger),
			metrics.UnaryClientInterceptor(),
			messagesize.UnaryClientInterceptor,
			internalerrs.PrometheusUnaryClientInterceptor,
			internalerrs.LoggingUnaryClientInterceptor(logger),
			contextconv.UnaryClientInterceptor,
		),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(defaultGRPCMessageReceiveSizeBytes)),
	}

	out = append(out, additionalOptions...)

	// Ensure that the message size options are set last, so they override any other
	// client-specific options that tweak the message size.
	//
	// The message size options are only provided if the environment variable is set. These options serve as an escape hatch, so they
	// take precedence over everything else with a uniform size setting that's easy to reason about.
	out = append(out, messagesize.MustGetClientMessageSizeFromEnv()...)

	return out
}

// NewServer creates a new *grpc.Server with the default options
func NewServer(logger log.Logger, additionalOpts ...grpc.ServerOption) *grpc.Server {
	s := grpc.NewServer(ServerOptions(logger, additionalOpts...)...)
	reflection.Register(s)
	return s
}

// NewPublicServer creates a new *grpc.Server with the options tailored for
// public-facing gRPC services. Most in-Sourcegraph services should use
// NewServer instead.
func NewPublicServer(logger log.Logger, additionalOpts ...grpc.ServerOption) *grpc.Server {
	s := grpc.NewServer(buildServerOptions(logger, serverOptions{
		additionalOptions: additionalOpts,
		// Public-facing service should not trust remote spans, as we generally
		// won't have access to remote spans anwyay.
		trustRemoteSpans: false,
	})...)
	reflection.Register(s)
	return s
}

// ServerOptions is a set of default server options that should be used for all
// gRPC servers in Sourcegraph. along with any additional service-specific options.
//
// **Note**: Do not append to this slice directly, instead provide extra options
// via "additionalOptions".
func ServerOptions(logger log.Logger, additionalOptions ...grpc.ServerOption) []grpc.ServerOption {
	return buildServerOptions(logger, serverOptions{
		additionalOptions: additionalOptions,
		trustRemoteSpans:  true,
	})
}

type serverOptions struct {
	additionalOptions []grpc.ServerOption
	// trustRemoteSpans, if false, will not accept incoming spans as the parent,
	// and instead create new root spans for each request.
	trustRemoteSpans bool
}

func buildServerOptions(logger log.Logger, opts serverOptions) []grpc.ServerOption {
	// Generate the options dynamically rather than using a static slice
	// because these options depend on some globals (tracer, trace sampling)
	// that are not initialized during init time.

	metrics := mustGetServerMetrics()

	otelOpts := []otelgrpc.Option{}
	if !opts.trustRemoteSpans {
		otelOpts = append(otelOpts,
			otelgrpc.WithSpanOptions(trace.WithNewRoot()))
	}

	out := []grpc.ServerOption{
		grpc.ChainStreamInterceptor(
			internalgrpc.NewStreamPanicCatcher(logger),
			internalerrs.LoggingStreamServerInterceptor(logger),
			metrics.StreamServerInterceptor(),
			messagesize.StreamServerInterceptor,
			propagator.StreamServerPropagator(requestclient.Propagator{}),
			propagator.StreamServerPropagator(requestinteraction.Propagator{}),
			propagator.StreamServerPropagator(tenant.TenantPropagator{}),
			propagator.StreamServerPropagator(actor.ActorPropagator{}),
			propagator.StreamServerPropagator(policy.ShouldTracePropagator{}),
			tenant.StreamServerInterceptor,
			otelgrpc.StreamServerInterceptor(otelOpts...), //lint:ignore SA1019 the advertised replacement doesn't seem to be a drop-in replacement, use deprecated mechanism for now
			contextconv.StreamServerInterceptor,
		),
		grpc.ChainUnaryInterceptor(
			internalgrpc.NewUnaryPanicCatcher(logger),
			internalerrs.LoggingUnaryServerInterceptor(logger),
			metrics.UnaryServerInterceptor(),
			messagesize.UnaryServerInterceptor,
			propagator.UnaryServerPropagator(requestclient.Propagator{}),
			propagator.UnaryServerPropagator(requestinteraction.Propagator{}),
			propagator.UnaryServerPropagator(tenant.TenantPropagator{}),
			propagator.UnaryServerPropagator(actor.ActorPropagator{}),
			propagator.UnaryServerPropagator(policy.ShouldTracePropagator{}),
			tenant.UnaryServerInterceptor,
			otelgrpc.UnaryServerInterceptor(otelOpts...), //lint:ignore SA1019 the advertised replacement doesn't seem to be a drop-in replacement, use deprecated mechanism for now
			contextconv.UnaryServerInterceptor,
		),
		grpc.MaxRecvMsgSize(defaultGRPCMessageReceiveSizeBytes),
	}

	out = append(out, opts.additionalOptions...)

	// Ensure that the message size options are set last, so they override any other
	// server-specific options that tweak the message size.
	//
	// The message size options are only provided if the environment variable is set. These options serve as an escape hatch, so they
	// take precedence over everything else with a uniform size setting that's easy to reason about.
	out = append(out, messagesize.MustGetServerMessageSizeFromEnv()...)

	return out
}

var (
	clientMetricsOnce sync.Once
	clientMetrics     *grpcprom.ClientMetrics

	serverMetricsOnce sync.Once
	serverMetrics     *grpcprom.ServerMetrics
)

// mustGetClientMetrics returns a singleton instance of the client metrics
// that are shared across all gRPC clients that this process creates.
//
// This function panics if the metrics cannot be registered with the default
// Prometheus registry.
func mustGetClientMetrics() *grpcprom.ClientMetrics {
	clientMetricsOnce.Do(func() {
		clientMetrics = grpcprom.NewClientMetrics(
			grpcprom.WithClientCounterOptions(),
			grpcprom.WithClientHandlingTimeHistogram(), // record the overall request latency for a gRPC request
			grpcprom.WithClientStreamRecvHistogram(),   // record how long it takes for a client to receive a message during a streaming RPC
			grpcprom.WithClientStreamSendHistogram(),   // record how long it takes for a client to send a message during a streaming RPC
		)
		prometheus.MustRegister(clientMetrics)
	})

	return clientMetrics
}

// mustGetServerMetrics returns a singleton instance of the server metrics
// that are shared across all gRPC servers that this process creates.
//
// This function panics if the metrics cannot be registered with the default
// Prometheus registry.
func mustGetServerMetrics() *grpcprom.ServerMetrics {
	serverMetricsOnce.Do(func() {
		serverMetrics = grpcprom.NewServerMetrics(
			grpcprom.WithServerCounterOptions(),
			grpcprom.WithServerHandlingTimeHistogram(), // record the overall response latency for a gRPC request)
		)
		prometheus.MustRegister(serverMetrics)
	})

	return serverMetrics
}
