package tenant

import (
	"context"
	"runtime/pprof"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// TenantPropagator implements the (internal/grpc).Propagator interface
// for propagating tenants across RPC calls. This is modeled directly on
// the HTTP middleware in this package, and should work exactly the same.
type TenantPropagator struct{}

func (TenantPropagator) FromContext(ctx context.Context) metadata.MD {
	tenant, err := FromContext(ctx)
	md := make(metadata.MD)

	if err != nil {
		md.Append(headerKeyTenantID, headerValueNoTenant)
	} else {
		md.Append(headerKeyTenantID, strconv.Itoa(tenant.ID()))
	}

	return md
}

func (TenantPropagator) InjectContext(ctx context.Context, md metadata.MD) (context.Context, error) {
	var idStr string
	if vals := md.Get(headerKeyTenantID); len(vals) > 0 {
		idStr = vals[0]
	}

	switch idStr {
	case "", headerValueNoTenant:
		// Nothing to do, empty tenant.
		return ctx, nil
	default:
		uid, err := strconv.Atoi(idStr)
		if err != nil {
			// The tenant value is invalid.
			return ctx, status.New(codes.InvalidArgument, errors.Wrap(err, "bad tenant value in metadata").Error()).Err()
		}
		return withTenant(ctx, uid), nil
	}
}

// UnaryServerInterceptor is a grpc.UnaryServerInterceptor that injects the tenant ID
// from the context into pprof labels.
func UnaryServerInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
	if tnt, err := FromContext(ctx); err == nil {
		defer pprof.SetGoroutineLabels(ctx)
		ctx = pprof.WithLabels(ctx, pprof.Labels("tenant", strconv.Itoa(tnt.ID())))
		pprof.SetGoroutineLabels(ctx)
	}

	return handler(ctx, req)
}

// StreamServerInterceptor is a grpc.StreamServerInterceptor that injects the tenant ID
// from the context into pprof labels.
func StreamServerInterceptor(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if tnt, err := FromContext(ss.Context()); err == nil {
		ctx := ss.Context()
		defer pprof.SetGoroutineLabels(ctx)
		ctx = pprof.WithLabels(ctx, pprof.Labels("tenant", strconv.Itoa(tnt.ID())))
		pprof.SetGoroutineLabels(ctx)

		ss = &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: ctx,
		}
	}

	return handler(srv, ss)
}
