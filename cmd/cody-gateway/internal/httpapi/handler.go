package httpapi

import (
	"context"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/overhead"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/auth"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/attribution"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/completions"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/embeddings"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/featurelimiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/requestlogger"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	RateLimitNotifier           notify.RateLimitNotifier
	Anthropic                   config.AnthropicConfig
	OpenAI                      config.OpenAIConfig
	Fireworks                   config.FireworksConfig
	EmbeddingsAllowedModels     []string
	AutoFlushStreamingResponses bool
	EnableAttributionSearch     bool
	Sourcegraph                 config.SourcegraphConfig
}

var meter = otel.GetMeterProvider().Meter("cody-gateway/internal/httpapi")

var (
	attributesAnthropicCompletions = newMetricAttributes("anthropic", "completions")
	attributesOpenAICompletions    = newMetricAttributes("openai", "completions")
	attributesOpenAIEmbeddings     = newMetricAttributes("openai", "embeddings")
	attributesFireworksCompletions = newMetricAttributes("fireworks", "completions")
)

func NewHandler(
	logger log.Logger,
	eventLogger events.Logger,
	rs limiter.RedisStore,
	httpClient httpcli.Doer,
	authr *auth.Authenticator,
	promptRecorder completions.PromptRecorder,
	config *Config,
	dotcomClient graphql.Client,
) (http.Handler, error) {
	// Initialize metrics
	counter, err := meter.Int64UpDownCounter("cody-gateway.concurrent_upstream_requests",
		metric.WithDescription("number of concurrent active requests for upstream services"))
	if err != nil {
		return nil, errors.Wrap(err, "init metric 'concurrent_upstream_requests'")
	}
	latencyHistogram, err := meter.Int64Histogram("cody-gateway.latency_overhead",
		metric.WithDescription("Cody Gateway response latency overhead in milliseconds"))
	if err != nil {
		return nil, errors.Wrap(err, "init metric 'latency_overhead'")
	}
	r := mux.NewRouter()

	// V1 service routes
	v1router := r.PathPrefix("/v1").Subrouter()
	// registerStandardEndpoint registers an HTTP endpoint with all of the expected middleware
	// for authentication, latency, instrumentationm, etc.
	registerStandardEndpoint := func(name, route string, attributes attribute.Set, handler http.Handler) {
		// Create an HTTP handler that will update the "concurrent_upstream_requests" metric.
		gaugedHandler := gaugeHandler(
			counter,
			attributes,
			authr.Middleware(
				requestlogger.Middleware(
					logger,
					handler,
				),
			))
		// Wrap that in our instrumentation middleware, adding more logging.
		instrumentedHandler := instrumentation.HTTPMiddleware(
			name,
			gaugedHandler,
			otelhttp.WithPublicEndpoint())
		// Finally wrap that again in our overall middleware.
		overheadMiddleware := overhead.HTTPMiddleware(latencyHistogram, instrumentedHandler)

		v1router.Path(route).Methods(http.MethodPost).Handler(overheadMiddleware)
	}

	// registerSimpleGETEndpoint registers a basic HTTP GET endpoint, without the
	// latency and performance counter middle ware that we register for other endpoints.
	registerSimpleGETEndpoint := func(name, route string, handler http.Handler) {
		v1router.Path(route).Methods(http.MethodGet).Handler(
			instrumentation.HTTPMiddleware(name,
				authr.Middleware(
					requestlogger.Middleware(
						logger,
						handler,
					),
				),
				otelhttp.WithPublicEndpoint(),
			),
		)
	}

	if config.Anthropic.AccessToken == "" {
		logger.Error("Anthropic access token not set. Not registering Anthropic-related endpoints.")
	} else {
		anthropicHandler, err := completions.NewAnthropicHandler(
			logger,
			eventLogger,
			rs,
			config.RateLimitNotifier,
			httpClient,
			config.Anthropic,
			promptRecorder,
			config.AutoFlushStreamingResponses)
		if err != nil {
			return nil, errors.Wrap(err, "init Anthropic handler")
		}
		registerStandardEndpoint(
			"v1.completions.anthropic",
			"/completions/anthropic",
			attributesAnthropicCompletions,
			anthropicHandler)

		anthropicMessagesHandler, err := completions.NewAnthropicMessagesHandler(
			logger,
			eventLogger,
			rs,
			config.RateLimitNotifier,
			httpClient,
			config.Anthropic,
			promptRecorder,
			config.AutoFlushStreamingResponses)
		if err != nil {
			return nil, errors.Wrap(err, "init anthropicMessages handler")
		}
		registerStandardEndpoint(
			"v1.completions.anthropicmessages",
			"/completions/anthropic-messages",
			attributesAnthropicCompletions,
			anthropicMessagesHandler)
	}

	if config.OpenAI.AccessToken == "" {
		logger.Error("OpenAI access token not set. Not registering OpenAI-related endpoints.")
	} else {
		openAIHandler := completions.NewOpenAIHandler(
			logger,
			eventLogger,
			rs,
			config.RateLimitNotifier,
			httpClient,
			config.OpenAI,
			config.AutoFlushStreamingResponses)
		registerStandardEndpoint(
			"v1.completions.openai",
			"/completions/openai",
			attributesOpenAICompletions,
			openAIHandler)

		registerSimpleGETEndpoint("v1.embeddings.models", "/embeddings/models", embeddings.NewListHandler())

		factoryMap := embeddings.ModelFactoryMap{
			embeddings.ModelNameOpenAIAda:         embeddings.NewOpenAIClient(httpClient, config.OpenAI.AccessToken),
			embeddings.ModelNameSourcegraphTriton: embeddings.NewSourcegraphClient(httpClient, config.Sourcegraph.TritonURL),
		}
		embeddingsHandler := embeddings.NewHandler(
			logger,
			eventLogger,
			rs,
			config.RateLimitNotifier,
			factoryMap,
			config.EmbeddingsAllowedModels)
		// TODO: If embeddings.ModelFactoryMap includes more than just OpenAI, we might want to
		// revisit how we count concurrent requests into the handler. (Instead of assuming they are
		// all OpenAI-related requests. (i.e. maybe we should use something other than
		// attributesOpenAIEmbeddings here.)
		registerStandardEndpoint(
			"v1.embeddings",
			"/embeddings",
			attributesOpenAIEmbeddings,
			embeddingsHandler)

		if config.Sourcegraph.UnlimitedEmbeddings {
			// Register an endpoint for Triton embeddings that does not implement rate limiting.
			unlimitedHandler := embeddings.NewUnlimitedHandler(logger, eventLogger, factoryMap, []string{string(embeddings.ModelNameSourcegraphTriton)})
			registerStandardEndpoint(
				"v1.embeddings-unlimited",
				"/embeddings-unlimited",
				attributesOpenAIEmbeddings,
				unlimitedHandler)
		}
	}

	if config.Fireworks.AccessToken == "" {
		logger.Error("Fireworks access token not set. Not registering Fireworks-related endpoints.")
	} else {
		fireworksHandler := completions.NewFireworksHandler(
			logger,
			eventLogger,
			rs,
			config.RateLimitNotifier,
			httpClient,
			config.Fireworks,
			config.AutoFlushStreamingResponses)
		registerStandardEndpoint(
			"v1.completions.fireworks",
			"/completions/fireworks",
			attributesFireworksCompletions,
			fireworksHandler)
	}

	// Register a route where actors can retrieve their current rate limit state.
	limitsHandler := featurelimiter.ListLimitsHandler(logger, rs)
	registerSimpleGETEndpoint("v1.limits", "/limits", limitsHandler)

	// Register a route where actors can refresh their rate limit state.
	v1router.Path("/limits/refresh").Methods(http.MethodPost).Handler(
		instrumentation.HTTPMiddleware("v1.limits",
			authr.Middleware(
				requestlogger.Middleware(
					logger,
					featurelimiter.RefreshLimitsHandler(logger),
				),
			),
			otelhttp.WithPublicEndpoint(),
		),
	)

	var attributionClient graphql.Client
	if config.EnableAttributionSearch {
		attributionClient = dotcomClient
	}
	v1router.Path("/attribution").Methods(http.MethodPost).Handler(
		instrumentation.HTTPMiddleware("v1.attribution",
			authr.Middleware(
				attribution.NewHandler(attributionClient, logger),
			),
			otelhttp.WithPublicEndpoint(),
		),
	)
	return r, nil
}

func newMetricAttributes(provider string, feature string) attribute.Set {
	return attribute.NewSet(
		attribute.String("provider", provider),
		attribute.String("feature", feature))
}

// gaugeHandler increments gauge when handling the request and decrements it
// upon completion.
func gaugeHandler(counter metric.Int64UpDownCounter, attrs attribute.Set, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(r.Context(), 1, metric.WithAttributeSet(attrs))
		handler.ServeHTTP(w, r)
		// Background context when done, since request may be cancelled.
		counter.Add(context.Background(), -1, metric.WithAttributeSet(attrs))
	})
}

type CompletionsConfig struct {
	logger      log.Logger
	eventLogger events.Logger
	rs          limiter.RedisStore
}
