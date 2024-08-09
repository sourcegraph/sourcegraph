package tenant

import (
	"context"
	"fmt"
	"net/http"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
)

// TenantHostnameMapper takes a hostname and returns the tenant ID associated with it.
// It is expected to return an error if the tenant is not known that satisfies the
// errcode.IsNotFound check.
type TenantHostnameMapper func(ctx context.Context, host string) (int, error)

// ExternalTenantFromHostnameMiddleware is a middleware that sets the tenant ID
// from the hostname.
//
// 🚨 SECURITY: This must only be called in frontends external facing API, and
// the passed in TenantHostnameMapper must be stable (i.e., not return different
// tenant IDs for the same hostname across invocations).
func ExternalTenantFromHostnameMiddleware(tenantForHostname TenantHostnameMapper, next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		host := extractHost(req.Host)

		tenantID, err := tenantForHostname(ctx, host)
		if err != nil {
			if errcode.IsNotFound(err) {
				http.Error(rw, fmt.Sprintf("tenant %q not known", host), http.StatusBadRequest)
				return
			}
			http.Error(rw, "failed to fetch tenant", http.StatusInternalServerError)
			return
		}

		ctx = withTenant(ctx, tenantID)

		pprof.Do(ctx, pprof.Labels("tenant", strconv.Itoa(tenantID)), func(ctx context.Context) {
			next.ServeHTTP(rw, req.WithContext(ctx))
		})
	})
}

// extractHost extracts the host from a host:port string. This method only works
// for host:port strings, it cannot handle IPv6 addresses.
func extractHost(hostPort string) string {
	colon := strings.LastIndexByte(hostPort, ':')
	if colon != -1 {
		return hostPort[:colon]
	}
	return hostPort
}
