package tenant

import (
	"context"
	"fmt"
	"net/http"
	"runtime/pprof"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/trace"
)

const (
	// headerKeyTenantID is the header key for the tenant ID.
	headerKeyTenantID = "X-Sourcegraph-Tenant-ID"
)

const (
	// headerValueNoTenant indicates the request has no tenant.
	headerValueNoTenant = "none"
)

// InternalHTTPTransport is a roundtripper that sets tenants within request context
// as headers on outgoing requests. The attached headers can be picked up and attached
// to incoming request contexts with tenant.InternalHTTPMiddleware.
type InternalHTTPTransport struct {
	RoundTripper http.RoundTripper
}

var _ http.RoundTripper = &InternalHTTPTransport{}

func (t *InternalHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.RoundTripper == nil {
		t.RoundTripper = http.DefaultTransport
	}

	// RoundTripper should not modify original request. All the code paths
	// below set a header, so we clone the request immediately.
	req = req.Clone(req.Context())

	tenant, err := FromContext(req.Context())

	if err != nil {
		// No tenant set
		req.Header.Set(headerKeyTenantID, headerValueNoTenant)
	} else {
		req.Header.Set(headerKeyTenantID, strconv.Itoa(tenant.ID()))
	}

	return t.RoundTripper.RoundTrip(req)
}

// InternalHTTPMiddleware wraps the given handle func and attaches the tenant indicated
// in incoming requests to the request header. This should only be used to wrap internal
// handlers for communication between Sourcegraph services.
// The client side has to use the InternalHTTPTransport to set the tenant header.
//
// 🚨 SECURITY: This should *never* be called to wrap externally accessible handlers (i.e.
// only use for internal endpoints), because header values allow to impersonate a tenant.
func InternalHTTPMiddleware(logger log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		idStr := req.Header.Get(headerKeyTenantID)
		switch idStr {
		case "", headerValueNoTenant:
			// Request not associated with a tenant, continue with request
			next.ServeHTTP(rw, req)
			return
		default:
			// Request associated with a tenant - add it to the context:
			tntID, err := strconv.Atoi(idStr)
			if err != nil {
				trace.Logger(ctx, logger).
					Warn("invalid tenant ID in request",
						log.Error(err),
						log.String("tenantID", idStr))

				// Do not proceed with request
				rw.WriteHeader(http.StatusForbidden)
				_, _ = rw.Write([]byte(fmt.Sprintf("%s was provided for tenant, but the value was invalid", headerKeyTenantID)))
				return
			}

			// Valid tenant
			ctx = withTenant(ctx, tntID)

			pprof.Do(ctx, pprof.Labels("tenant", strconv.Itoa(tntID)), func(ctx context.Context) {
				next.ServeHTTP(rw, req.WithContext(ctx))
			})
		}
	})
}
