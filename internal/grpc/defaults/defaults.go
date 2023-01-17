// package defaults exports a set of default options for gRPC servers
// and clients in Sourcegraph. It is a separate subpackage so that all
// packages that depend on the internal/grpc package do not need to
// depend on the large dependency tree of this package.
package defaults

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
)

func DialOptions() []grpc.DialOption {
	// Generate the options dynamically rather than using a static slice
	// because these options depend on some globals (tracer, trace sampling)
	// that are not initialized during init time.
	return []grpc.DialOption{
		grpc.WithChainStreamInterceptor(
			internalgrpc.StreamClientPropagator(policy.ShouldTracePropagator{}),
			otelgrpc.StreamClientInterceptor(),
		),
		grpc.WithChainUnaryInterceptor(
			internalgrpc.UnaryClientPropagator(policy.ShouldTracePropagator{}),
			otelgrpc.UnaryClientInterceptor(),
		),
	}
}

func ServerOptions() []grpc.ServerOption {
	// Generate the options dynamically rather than using a static slice
	// because these options depend on some globals (tracer, trace sampling)
	// that are not initialized during init time.
	return []grpc.ServerOption{
		grpc.ChainStreamInterceptor(
			internalgrpc.StreamServerPropagator(policy.ShouldTracePropagator{}),
			otelgrpc.StreamServerInterceptor(),
		),
		grpc.ChainUnaryInterceptor(
			internalgrpc.UnaryServerPropagator(policy.ShouldTracePropagator{}),
			otelgrpc.UnaryServerInterceptor(),
		),
	}
}
