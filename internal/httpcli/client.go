package httpcli

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gregjones/httpcache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// A Doer captures the Do method of an http.Client. It facilitates decorating
// an http.Client with orthogonal concerns such as logging, metrics, retries,
// etc.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// DoerFunc is function adapter that implements the http.RoundTripper
// interface by calling itself.
type DoerFunc func(*http.Request) (*http.Response, error)

// Do implements the Doer interface.
func (f DoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// A Middleware function wraps a Doer with a layer of behaviour. It's used
// to decorate an http.Client with orthogonal layers of behaviour such as
// logging, instrumentation, retries, etc.
type Middleware func(Doer) Doer

// NewMiddleware returns a Middleware stack composed of the given Middlewares.
func NewMiddleware(mws ...Middleware) Middleware {
	return func(bottom Doer) (stacked Doer) {
		stacked = bottom
		for _, mw := range mws {
			stacked = mw(stacked)
		}
		return stacked
	}
}

// Opt configures an aspect of a given *http.Client,
// returning an error in case of failure.
type Opt func(*http.Client) error

// A Factory constructs an http.Client with the given functional
// options applied, returning an aggregate error of the errors returned by
// all those options.
type Factory struct {
	stack  Middleware
	common []Opt
}

// redisCache is an HTTP cache backed by Redis. The TTL of a week is a balance
// between caching values for a useful amount of time versus growing the cache
// too large.
var redisCache = rcache.NewWithTTL("http", 604800)

// CachedTransportOpt is the default transport cache - it will return values from
// the cache where possible (avoiding a network request) and will additionally add
// validators (etag/if-modified-since) to repeated requests allowing servers to
// return 304 / Not Modified.
//
// Responses load from cache will have the 'X-From-Cache' header set.
var CachedTransportOpt = NewCachedTransportOpt(redisCache)

var (
	externalTimeout               = env.MustGetDuration("SRC_HTTP_CLI_EXTERNAL_TIMEOUT", 5*time.Minute, "Timeout for external HTTP requests")
	externalRetryDelayBase        = env.MustGetDuration("SRC_HTTP_CLI_EXTERNAL_RETRY_DELAY_BASE", 200*time.Millisecond, "Base retry delay duration for external HTTP requests")
	externalRetryDelayMax         = env.MustGetDuration("SRC_HTTP_CLI_EXTERNAL_RETRY_DELAY_MAX", 3*time.Second, "Max retry delay duration for external HTTP requests")
	externalRetryMaxAttempts      = env.MustGetInt("SRC_HTTP_CLI_EXTERNAL_RETRY_MAX_ATTEMPTS", 20, "Max retry attempts for external HTTP requests")
	externalRetryAfterMaxDuration = env.MustGetDuration("SRC_HTTP_CLI_EXTERNAL_RETRY_AFTER_MAX_DURATION", 3*time.Second, "Max duration to wait in retry-after header before we won't auto-retry")
	codyGatewayDisableHTTP2       = env.MustGetBool("SRC_HTTP_CLI_DISABLE_CODY_GATEWAY_HTTP2", false, "Whether we should disable HTTP2 for Cody Gateway communication")
)

// NewExternalClientFactory returns a httpcli.Factory with common options
// and middleware pre-set for communicating with external services. Additional
// middleware can also be provided to e.g. enable logging with NewLoggingMiddleware.
func NewExternalClientFactory(middleware ...Middleware) *Factory {
	mw := []Middleware{
		ContextErrorMiddleware,
		HeadersMiddleware("User-Agent", "Sourcegraph-Bot"),
		redisLoggerMiddleware(),
	}
	mw = append(mw, middleware...)

	return NewFactory(
		NewMiddleware(mw...),
		NewTimeoutOpt(externalTimeout),
		// externalTransportOpt needs to be before TracedTransportOpt and
		// NewCachedTransportOpt since it wants to extract a http.Transport,
		// not a generic http.RoundTripper.
		// TODO
		ExternalTransportOpt,
		NewErrorResilientTransportOpt(
			newRetryPolicy(maxRetries(externalRetryMaxAttempts), externalRetryAfterMaxDuration),
			expJitterDelayOrRetryAfterDelay(externalRetryDelayBase, externalRetryDelayMax),
		),
		RequestInteractionTransportOpt,
		TracedTransportOpt,
	)
}

// ExternalDoer is a shared client for external communication. This is a
// convenience for existing uses of http.DefaultClient.
var ExternalDoer, _ = NewExternalClientFactory().Doer()

// CodyGatewayDoer is a client for communication with Cody Gateway.
// This client does not cache responses.
var CodyGatewayDoer, _ = NewExternalClientFactory().Doer(NewDisableHTTP2Opt(codyGatewayDisableHTTP2))

var (
	internalTimeout               = env.MustGetDuration("SRC_HTTP_CLI_INTERNAL_TIMEOUT", 0, "Timeout for internal HTTP requests")
	internalRetryDelayBase        = env.MustGetDuration("SRC_HTTP_CLI_INTERNAL_RETRY_DELAY_BASE", 50*time.Millisecond, "Base retry delay duration for internal HTTP requests")
	internalRetryDelayMax         = env.MustGetDuration("SRC_HTTP_CLI_INTERNAL_RETRY_DELAY_MAX", 1*time.Second, "Max retry delay duration for internal HTTP requests")
	internalRetryMaxAttempts      = env.MustGetInt("SRC_HTTP_CLI_INTERNAL_RETRY_MAX_ATTEMPTS", 20, "Max retry attempts for internal HTTP requests")
	internalRetryAfterMaxDuration = env.MustGetDuration("SRC_HTTP_CLI_INTERNAL_RETRY_AFTER_MAX_DURATION", 3*time.Second, "Max duration to wait in retry-after header before we won't auto-retry")
)

// NewInternalClientFactory returns a httpcli.Factory with common options
// and middleware pre-set for communicating with internal services. Additional
// middleware can also be provided to e.g. enable logging with NewLoggingMiddleware.
func NewInternalClientFactory(subsystem string, middleware ...Middleware) *Factory {
	mw := []Middleware{
		ContextErrorMiddleware,
	}
	mw = append(mw, middleware...)

	return NewFactory(
		NewMiddleware(mw...),
		NewTimeoutOpt(internalTimeout),
		NewMaxIdleConnsPerHostOpt(500),
		NewErrorResilientTransportOpt(
			newRetryPolicy(maxRetries(internalRetryMaxAttempts), internalRetryAfterMaxDuration),
			expJitterDelayOrRetryAfterDelay(internalRetryDelayBase, internalRetryDelayMax),
		),
		MeteredTransportOpt(subsystem),
		ActorTransportOpt,
		RequestClientTransportOpt,
		RequestInteractionTransportOpt,
		TracedTransportOpt,
	)
}

// InternalDoer is a shared client for internal communication. This is a
// convenience for existing uses of http.DefaultClient.
var InternalDoer, _ = NewInternalClientFactory("internal").Doer()

// Doer returns a new Doer wrapped with the middleware stack
// provided in the Factory constructor and with the given common
// and base opts applied to it.
func (f Factory) Doer(base ...Opt) (Doer, error) {
	cli, err := f.Client(base...)
	if err != nil {
		return nil, err
	}

	if f.stack != nil {
		return f.stack(cli), nil
	}

	return cli, nil
}

// Client returns a new http.Client configured with the
// given common and base opts, but not wrapped with any
// middleware.
func (f Factory) Client(base ...Opt) (*http.Client, error) {
	opts := make([]Opt, 0, len(f.common)+len(base))
	opts = append(opts, base...)
	opts = append(opts, f.common...)

	var cli http.Client
	var err error

	for _, opt := range opts {
		err = errors.Append(err, opt(&cli))
	}

	return &cli, err
}

// WithOpts returns a new Factory with the given opts added to the common opts.
func (f *Factory) WithOpts(addtl ...Opt) *Factory {
	return &Factory{stack: f.stack, common: append(f.common, addtl...)}
}

// NewFactory returns a Factory that applies the given common
// Opts after the ones provided on each invocation of Client or Doer.
//
// If the given Middleware stack is not nil, the final configured client
// will be wrapped by it before being returned from a call to Doer, but not Client.
func NewFactory(stack Middleware, common ...Opt) *Factory {
	return &Factory{stack: stack, common: common}
}

//
// Common Middleware
//

// HeadersMiddleware returns a middleware that wraps a Doer
// and sets the given headers.
func HeadersMiddleware(headers ...string) Middleware {
	if len(headers)%2 != 0 {
		panic("missing header values")
	}
	return func(cli Doer) Doer {
		return DoerFunc(func(req *http.Request) (*http.Response, error) {
			for i := 0; i < len(headers); i += 2 {
				req.Header.Add(headers[i], headers[i+1])
			}
			return cli.Do(req)
		})
	}
}

// ContextErrorMiddleware wraps a Doer with context.Context error
// handling. It checks if the request context is done, and if so,
// returns its error. Otherwise, it returns the error from the inner
// Doer call.
func ContextErrorMiddleware(cli Doer) Doer {
	return DoerFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := cli.Do(req)
		if err != nil {
			// If we got an error, and the context has been canceled,
			// the context's error is probably more useful.
			if e := req.Context().Err(); e != nil {
				err = e
			}
		}
		return resp, err
	})
}

// requestContextKey is used to denote keys to fields that should be logged by the logging
// middleware. They should be set to the request context associated with a response.
type requestContextKey int

const (
	// requestRetryAttemptKey is the key to the Attempt attached to a request, if
	// a request undergoes retries via NewRetryPolicy
	requestRetryAttemptKey requestContextKey = iota

	// redisLoggingMiddlewareErrorKey is the key to any errors that occurred when logging
	// a request to Redis via redisLoggerMiddleware
	redisLoggingMiddlewareErrorKey
)

// NewLoggingMiddleware logs basic diagnostics about requests made through this client at
// debug level. The provided logger is given the 'httpcli' subscope.
//
// It also logs metadata set by request context by other middleware, such as NewRetryPolicy.
func NewLoggingMiddleware(logger log.Logger) Middleware {
	logger = logger.Scoped("httpcli")

	return func(d Doer) Doer {
		return DoerFunc(func(r *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := d.Do(r)

			// Gather fields about this request.
			fields := append(make([]log.Field, 0, 5), // preallocate some space
				log.String("host", r.URL.Host),
				log.String("path", r.URL.Path),
				log.Duration("duration", time.Since(start)))
			if err != nil {
				fields = append(fields, log.Error(err))
			}
			// Check incoming request context, unless a response is available, in which
			// case we check the request associated with the response in case it is not
			// the same as the original request (e.g. due to retries)
			ctx := r.Context()
			if resp != nil {
				ctx = resp.Request.Context()
				fields = append(fields, log.Int("code", resp.StatusCode))
			}
			// Gather fields from request context. When adding fields set into context,
			// make sure to test that the fields get propagated and picked up correctly
			// in TestLoggingMiddleware.
			if attempt, ok := ctx.Value(requestRetryAttemptKey).(Attempt); ok {
				// Get fields from NewRetryPolicy
				fields = append(fields, log.Object("retry",
					log.Int("attempts", attempt.Index),
					log.Error(attempt.Error)))
			}
			if redisErr, ok := ctx.Value(redisLoggingMiddlewareErrorKey).(error); ok {
				// Get fields from redisLoggerMiddleware
				fields = append(fields, log.NamedError("redisLoggerErr", redisErr))
			}

			// Log results with link to trace if present
			trace.Logger(ctx, logger).
				Debug("request", fields...)

			return resp, err
		})
	}
}

// NewCertPoolOpt returns an Opt that sets the RootCAs pool of an http.Client's
// transport.
func NewCertPoolOpt(certs ...string) Opt {
	return func(cli *http.Client) error {
		if len(certs) == 0 {
			return nil
		}

		tr, err := getTransportForMutation(cli)
		if err != nil {
			return errors.Wrap(err, "httpcli.NewCertPoolOpt")
		}

		if tr.TLSClientConfig == nil {
			tr.TLSClientConfig = new(tls.Config)
		}

		pool := x509.NewCertPool()
		tr.TLSClientConfig.RootCAs = pool

		for _, cert := range certs {
			if ok := pool.AppendCertsFromPEM([]byte(cert)); !ok {
				return errors.New("httpcli.NewCertPoolOpt: invalid certificate")
			}
		}

		return nil
	}
}

// NewCachedTransportOpt returns an Opt that wraps the existing http.Transport
// of an http.Client with caching using the given Cache.
//
// Responses returned from the cache will be given an extra header,
// X-From-Cache.
func NewCachedTransportOpt(c httpcache.Cache) Opt {
	return func(cli *http.Client) error {
		if cli.Transport == nil {
			cli.Transport = http.DefaultTransport
		}

		cli.Transport = WrapTransport(
			&httpcache.Transport{
				Transport:           cli.Transport,
				Cache:               c,
				MarkCachedResponses: true,
			},
			cli.Transport,
		)

		return nil
	}
}

// TracedTransportOpt wraps an existing http.Transport of an http.Client with
// tracing functionality.
func TracedTransportOpt(cli *http.Client) error {
	if cli.Transport == nil {
		cli.Transport = http.DefaultTransport
	}

	// Propagate trace policy
	cli.Transport = WrapTransport(&policy.Transport{RoundTripper: cli.Transport}, cli.Transport)

	// Collect and propagate OpenTelemetry trace (among other formats initialized
	// in internal/tracer)
	cli.Transport = WrapTransport(instrumentation.NewHTTPTransport(cli.Transport), cli.Transport)

	return nil
}

// MeteredTransportOpt returns an opt that wraps an existing http.Transport of a http.Client with
// metrics collection.
func MeteredTransportOpt(subsystem string) Opt {
	// This will generate a metric of the following format:
	// src_$subsystem_requests_total
	//
	// For example, if the subsystem is set to "internal", the metric being generated will be named
	// src_internal_requests_total
	meter := metrics.NewRequestMeter(
		subsystem,
		"Total number of requests sent to "+subsystem,
	)

	return func(cli *http.Client) error {
		if cli.Transport == nil {
			cli.Transport = http.DefaultTransport
		}

		cli.Transport = WrapTransport(meter.Transport(cli.Transport, func(u *url.URL) string {
			// We don't have a way to return a low cardinality label here (for
			// the prometheus label "category"). Previously we returned u.Path
			// but that blew up prometheus. So we just return unknown.
			return "unknown"
		}), cli.Transport)

		return nil
	}
}

var metricRetry = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_httpcli_retry_total",
	Help: "Total number of times we retry HTTP requests.",
})

// A regular expression to match the error returned by net/http when the
// configured number of redirects is exhausted. This error isn't typed
// specifically so we resort to matching on the error string.
var redirectsErrorRe = lazyregexp.New(`stopped after \d+ redirects\z`)

// A regular expression to match the error returned by net/http when the
// scheme specified in the URL is invalid. This error isn't typed
// specifically so we resort to matching on the error string.
var schemeErrorRe = lazyregexp.New(`unsupported protocol scheme`)

// NewIdleConnTimeoutOpt returns a Opt that sets the IdleConnTimeout of an
// http.Client's transport.
func NewIdleConnTimeoutOpt(timeout time.Duration) Opt {
	return func(cli *http.Client) error {
		tr, err := getTransportForMutation(cli)
		if err != nil {
			return errors.Wrap(err, "httpcli.NewIdleConnTimeoutOpt")
		}

		tr.IdleConnTimeout = timeout

		return nil
	}
}

// NewMaxIdleConnsPerHostOpt returns a Opt that sets the MaxIdleConnsPerHost field of an
// http.Client's transport.
func NewMaxIdleConnsPerHostOpt(max int) Opt {
	return func(cli *http.Client) error {
		tr, err := getTransportForMutation(cli)
		if err != nil {
			return errors.Wrap(err, "httpcli.NewMaxIdleConnsOpt")
		}

		tr.MaxIdleConnsPerHost = max

		return nil
	}
}

// NewDisableHTTP2Opt returns an Opt that makes the http.Client use HTTP/1.1 (instead of defaulting to HTTP/2).
func NewDisableHTTP2Opt(disable bool) Opt {
	return func(cli *http.Client) error {
		tr, err := getTransportForMutation(cli)
		if err != nil {
			return errors.Wrap(err, "httpcli.NewDisableHTTP2Opt")
		}
		if disable {
			tr.ForceAttemptHTTP2 = false
			tr.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
			tr.TLSClientConfig = &tls.Config{}
		}
		return nil
	}
}

// NewTimeoutOpt returns a Opt that sets the Timeout field of an http.Client.
func NewTimeoutOpt(timeout time.Duration) Opt {
	return func(cli *http.Client) error {
		if timeout > 0 {
			cli.Timeout = timeout
		}
		return nil
	}
}

// getTransport returns the http.Transport for cli. If Transport is nil, it is
// set to a copy of the DefaultTransport. If it is the DefaultTransport, it is
// updated to a copy of the DefaultTransport.
//
// Use this function when you intend on mutating the transport.
func getTransportForMutation(cli *http.Client) (*http.Transport, error) {
	if cli.Transport == nil {
		cli.Transport = http.DefaultTransport
	}

	// Try to get the underlying, concrete *http.Transport implementation, copy it, and
	// replace it.
	var transport *http.Transport
	switch v := cli.Transport.(type) {
	case *http.Transport:
		transport = v.Clone()
		// Replace underlying implementation
		cli.Transport = transport

	case WrappedTransport:
		wrapped := unwrapAll(v)
		t, ok := (*wrapped).(*http.Transport)
		if !ok {
			return nil, errors.Errorf("http.Client.Transport cannot be unwrapped as *http.Transport: %T", *wrapped)
		}
		transport = t.Clone()
		// Replace underlying implementation
		*wrapped = transport

	default:
		return nil, errors.Errorf("http.Client.Transport cannot be cast as a *http.Transport: %T", cli.Transport)
	}

	return transport, nil
}

// ActorTransportOpt wraps an existing http.Transport of an http.Client to pull the actor
// from the context and add it to each request's HTTP headers.
//
// Servers can use actor.HTTPMiddleware to populate actor context from incoming requests.
func ActorTransportOpt(cli *http.Client) error {
	if cli.Transport == nil {
		cli.Transport = http.DefaultTransport
	}

	cli.Transport = WrapTransport(
		&actor.HTTPTransport{RoundTripper: cli.Transport},
		cli.Transport,
	)

	return nil
}

// RequestClientTransportOpt wraps an existing http.Transport of an http.Client to pull
// the original client's IP from the context and add it to each request's HTTP headers.
//
// Servers can use requestclient.HTTPMiddleware to populate client context from incoming requests.
func RequestClientTransportOpt(cli *http.Client) error {
	if cli.Transport == nil {
		cli.Transport = http.DefaultTransport
	}

	cli.Transport = WrapTransport(
		&requestclient.HTTPTransport{RoundTripper: cli.Transport},
		cli.Transport,
	)

	return nil
}

func RequestInteractionTransportOpt(cli *http.Client) error {
	if cli.Transport == nil {
		cli.Transport = http.DefaultTransport
	}

	cli.Transport = &wrappedTransport{
		RoundTripper: &requestinteraction.HTTPTransport{RoundTripper: cli.Transport},
		Wrapped:      cli.Transport,
	}

	return nil
}

// IsRiskyHeader returns true if the request or response header is likely to contain private data.
func IsRiskyHeader(name string, values []string) bool {
	return isRiskyHeaderName(name) || containsRiskyHeaderValue(values)
}

// isRiskyHeaderName returns true if the request or response header is likely to contain private data
// based on its name.
func isRiskyHeaderName(name string) bool {
	riskyHeaderKeys := []string{"auth", "cookie", "token"}
	for _, riskyKey := range riskyHeaderKeys {
		if strings.Contains(strings.ToLower(name), riskyKey) {
			return true
		}
	}
	return false
}

// ContainsRiskyHeaderValue returns true if the values array of a request or response header
// looks like it's likely to contain private data.
func containsRiskyHeaderValue(values []string) bool {
	riskyHeaderValues := []string{"bearer", "ghp_", "glpat-"}
	for _, value := range values {
		for _, riskyValue := range riskyHeaderValues {
			if strings.Contains(strings.ToLower(value), riskyValue) {
				return true
			}
		}
	}
	return false
}
