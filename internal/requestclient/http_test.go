package requestclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExternalHTTPMiddleware(t *testing.T) {

	tests := []struct {
		name          string
		wantAddr      string
		remoteAddr    string
		useCloudFlare bool
	}{
		{
			name:          "set RealIp from Cf-Connecting-Ip header",
			wantAddr:      "1.1.1.1",
			remoteAddr:    "192.0.2.1",
			useCloudFlare: true,
		},
		{
			name:          "use RemoteAddr for RealIp when not using Cloudflare",
			remoteAddr:    "192.0.2.1",
			wantAddr:      "192.0.2.1",
			useCloudFlare: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				req := FromContext(r.Context())
				if req.RealIP != tc.wantAddr {
					t.Errorf("want client IP %q, got %q", tc.wantAddr, req.IP)
				}
			})

			useCloudflareHeaders = true
			middleware := ExternalHTTPMiddleware(next)

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Cf-Connecting-Ip", tc.wantAddr)
			req.RemoteAddr = tc.remoteAddr

			middleware.ServeHTTP(httptest.NewRecorder(), req)
		})
	}

}
