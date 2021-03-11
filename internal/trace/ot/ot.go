// Package ot wraps github.com/opentracing/opentracing-go and
// github.com./opentracing-contrib/go-stdlib with selective tracing behavior that is toggled on and
// off with the presence of a context item (uses context.Context). This context item is propagated
// across API boundaries through a HTTP header (X-Sourcegraph-Should-Trace).
package ot

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"go.uber.org/atomic"
)

type tracePolicy string

const (
	// TraceNone turns off tracing.
	TraceNone tracePolicy = "none"

	// TraceSelective turns on tracing only for requests with the X-Sourcegraph-Should-Trace header
	// set to a truthy value.
	TraceSelective tracePolicy = "selective"

	// Comprehensive turns on tracing for all requests.
	TraceAll tracePolicy = "all"
)

var trPolicy = atomic.NewString(string(TraceNone))

func SetTracePolicy(newTracePolicy tracePolicy) {
	trPolicy.Store(string(newTracePolicy))
}

func GetTracePolicy() tracePolicy {
	return tracePolicy(trPolicy.Load())
}

// Middleware wraps the handler with the following:
//
// - If the HTTP header, X-Sourcegraph-Should-Trace, is set to a truthy value, set the
//   shouldTraceKey context.Context value to true
// - github.com/opentracing-contrib/go-stdlib/nethttp.Middleware, which creates a new span to track
//   the request handler from the global tracer.
func Middleware(h http.Handler, opts ...nethttp.MWOption) http.Handler {
	return MiddlewareWithTracer(opentracing.GlobalTracer(), h)
}

// MiddlewareWithTracer is like Middleware, but uses the specified tracer instead of the global
// tracer.
func MiddlewareWithTracer(tr opentracing.Tracer, h http.Handler, opts ...nethttp.MWOption) http.Handler {
	nethttpMiddleware := nethttp.Middleware(tr, h, append([]nethttp.MWOption{
		nethttp.MWSpanFilter(func(r *http.Request) bool {
			return ShouldTrace(r.Context())
		}),
	}, opts...)...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var trace bool
		switch GetTracePolicy() {
		case TraceSelective:
			trace = requestWantsTracing(r)
		case TraceAll:
			trace = true
		default:
			trace = false
		}
		nethttpMiddleware.ServeHTTP(w, r.WithContext(WithShouldTrace(r.Context(), trace)))
	})
}

const traceHeader = "X-Sourcegraph-Should-Trace"
const traceQuery = "trace"

// requestWantsTrace returns true if a request is opting into tracing either
// via our HTTP Header or our URL Query.
func requestWantsTracing(r *http.Request) bool {
	// Prefer header over query param.
	if v := r.Header.Get(traceHeader); v != "" {
		b, _ := strconv.ParseBool(v)
		return b
	}
	// PERF: Avoid parsing RawQuery if "trace=" is not present
	if strings.Contains(r.URL.RawQuery, "trace=") {
		v := r.URL.Query().Get(traceQuery)
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

// Transport wraps an underlying HTTP RoundTripper, injecting the X-Sourcegraph-Should-Trace header
// into outgoing requests whenever the shouldTraceKey context value is true.
type Transport struct {
	http.RoundTripper
}

func (r *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(traceHeader, strconv.FormatBool(ShouldTrace(req.Context())))
	t := nethttp.Transport{RoundTripper: r.RoundTripper}
	return t.RoundTrip(req)
}

type key int

const (
	shouldTraceKey key = iota
)

// ShouldTrace returns true if the shouldTraceKey context value is true.
func ShouldTrace(ctx context.Context) bool {
	v, ok := ctx.Value(shouldTraceKey).(bool)
	if !ok {
		return false
	}
	return v
}

// WithShouldTrace sets the shouldTraceKey context value.
func WithShouldTrace(ctx context.Context, shouldTrace bool) context.Context {
	return context.WithValue(ctx, shouldTraceKey, shouldTrace)
}

// GetTracer returns the tracer to use for the given context. If ShouldTrace returns true, it
// returns the global tracer. Otherwise, it returns the NoopTracer.
func GetTracer(ctx context.Context) opentracing.Tracer {
	return getTracer(ctx, opentracing.GlobalTracer())
}

// getTracer is like GetTracer, but accepts a tracer as an argument. If ShouldTrace returns false,
// it returns the NoopTracer. If it returns true and the passed-in tracer is not nil, it returns the
// passed-in tracer. Otherwise, it returns the global tracer.
func getTracer(ctx context.Context, tracer opentracing.Tracer) opentracing.Tracer {
	if !ShouldTrace(ctx) {
		return opentracing.NoopTracer{}
	}
	if tracer == nil {
		return opentracing.GlobalTracer()
	}
	return tracer
}

// StartSpanFromContext starts a span using the tracer returned by GetTracer.
func StartSpanFromContext(ctx context.Context, operationName string, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {
	return StartSpanFromContextWithTracer(ctx, opentracing.GlobalTracer(), operationName, opts...)
}

// StartSpanFromContext starts a span using the tracer returned by invoking getTracer with the
// passed-in tracer.
func StartSpanFromContextWithTracer(ctx context.Context, tracer opentracing.Tracer, operationName string, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {
	return opentracing.StartSpanFromContextWithTracer(ctx, getTracer(ctx, tracer), operationName, opts...)
}
