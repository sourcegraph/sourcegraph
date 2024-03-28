package requestclient

import (
	"net/http"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/env"
)

var useCloudflareHeaders = env.MustGetBool("SRC_USE_CLOUDFLARE_HEADERS",
	dotcom.SourcegraphDotComMode(), "Use Cloudflare headers for request metadata")

const (
	// Sourcegraph-specific client IP header key
	headerKeyClientIP = "X-Sourcegraph-Client-IP"
	// De-facto standard for identifying original IP address of a client:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For
	headerKeyForwardedFor = "X-Forwarded-For"
	// Standard for identifyying the application, operating system, vendor,
	// and/or version of the requesting user agent.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/User-Agent
	headerKeyUserAgent = "User-Agent"
)

// HTTPTransport is a roundtripper that sets client IP information within request context as
// headers on outgoing requests. The attached headers can be picked up and attached to
// incoming request contexts with client.HTTPMiddleware.
//
// TODO(@bobheadxi): Migrate to httpcli.Doer and httpcli.Middleware
type HTTPTransport struct {
	RoundTripper http.RoundTripper
}

var _ http.RoundTripper = &HTTPTransport{}

func (t *HTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.RoundTripper == nil {
		t.RoundTripper = http.DefaultTransport
	}

	client := FromContext(req.Context())
	if client != nil {
		req = req.Clone(req.Context()) // RoundTripper should not modify original request
		req.Header.Set(headerKeyClientIP, client.IP)
		req.Header.Set(headerKeyForwardedFor, client.ForwardedFor)
	}

	return t.RoundTripper.RoundTrip(req)
}

// ExternalHTTPMiddleware wraps the given handle func and attaches client IP
// data indicated in incoming requests to the request header.
//
// This is meant to be used by http handlers which sit behind a reverse proxy
// receiving user traffic. IE sourcegraph-frontend.
func ExternalHTTPMiddleware(next http.Handler) http.Handler {
	return httpMiddleware(next, true)
}

// InternalHTTPMiddleware wraps the given handle func and attaches client IP
// data indicated in incoming requests to the request header.
//
// This is meant to be used by http handlers which receive internal traffic.
// EG gitserver.
func InternalHTTPMiddleware(next http.Handler) http.Handler {
	return httpMiddleware(next, false)
}

// httpMiddleware wraps the given handle func and attaches client IP data indicated in
// incoming requests to the request header.
//
// In Sourcegraph.com and Sourcegraph Cloud, we have a more reliable headers from
// Cloudflare via Cloudflare WAF, so environments that use Cloudflare can opt-in
// to Cloudflare-provided headers on external handlers. We have a debug endpoint
// that lets you confirm the presence of various headers:
//
//	curl --silent https://sourcegraph.com/-/debug/headers | grep Cf-
//
// Documentation for available Cloudflare headers is available at
// https://developers.cloudflare.com/fundamentals/reference/http-request-headers
func httpMiddleware(next http.Handler, external bool) http.Handler {
	forwardedForHeaders := []string{headerKeyForwardedFor}
	if external && useCloudflareHeaders {
		// Try to find trusted Cloudflare-provided connecting client IP.
		// https://developers.cloudflare.com/fundamentals/reference/http-request-headers/#x-forwarded-for
		forwardedForHeaders = []string{"Cf-Connecting-Ip", headerKeyForwardedFor}
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		forwardedFor := ""
		for _, k := range forwardedForHeaders {
			forwardedFor = req.Header.Get(k)
			if forwardedFor != "" {
				break
			}
		}

		remoteAddr := strings.Split(req.RemoteAddr, ":")[0]
		// Default value if not using Cloudflare
		realIp := remoteAddr

		var wafIPCountryCode string
		if external && useCloudflareHeaders {
			// Try to find trusted Cloudflare-provided country code of the request.
			// https://developers.cloudflare.com/fundamentals/reference/http-request-headers/#cf-ipcountry
			//
			// Cloudflare uses the "XX" code to indicate that the country info
			// is unknown.
			if cfIPCountry := req.Header.Get("CF-IPCountry"); cfIPCountry != "" && cfIPCountry != "XX" {
				wafIPCountryCode = cfIPCountry
			}

			// Set the real IP of the client to the connecting IP, if using Cloudflare.
			if forwardedFor != "" {
				realIp = forwardedFor
			}
		}

		ctxWithClient := WithClient(req.Context(), &Client{
			IP:           remoteAddr,
			RealIP:       realIp,
			ForwardedFor: req.Header.Get(headerKeyForwardedFor),
			UserAgent:    req.Header.Get(headerKeyUserAgent),

			wafIPCountryCode: wafIPCountryCode,
		})
		next.ServeHTTP(rw, req.WithContext(ctxWithClient))
	})
}
