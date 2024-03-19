package shared

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/redigo"
	"github.com/gomodule/redigo/redis"
	"github.com/slack-go/slack"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/contrib/detectors/gcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/shared/config"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/auth"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/completions"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor/anonymous"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor/dotcomuser"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor/productsubscription"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi"
)

func Main(ctx context.Context, obctx *observation.Context, ready service.ReadyFunc, cfg *config.Config) error {
	shutdownOtel, err := initOpenTelemetry(ctx, obctx.Logger, cfg.OpenTelemetry)
	if err != nil {
		return errors.Wrap(err, "initOpenTelemetry")
	}
	defer shutdownOtel()

	var eventLogger events.Logger
	if cfg.BigQuery.ProjectID != "" {
		eventLogger, err = events.NewBigQueryLogger(cfg.BigQuery.ProjectID, cfg.BigQuery.Dataset, cfg.BigQuery.Table)
		if err != nil {
			return errors.Wrap(err, "create BigQuery event logger")
		}

		// If a buffer is configured, wrap in events.BufferedLogger
		if cfg.BigQuery.EventBufferSize > 0 {
			eventLogger, err = events.NewBufferedLogger(obctx.Logger, eventLogger,
				cfg.BigQuery.EventBufferSize, cfg.BigQuery.EventBufferWorkers)
			if err != nil {
				return errors.Wrap(err, "create buffered logger")
			}
		}
	} else {
		eventLogger = events.NewStdoutLogger(obctx.Logger)

		// Useful for testing event logging in a way that has latency that is
		// somewhat similar to BigQuery.
		if os.Getenv("CODY_GATEWAY_BUFFERED_LAGGY_EVENT_LOGGING_FUN_TIMES_MODE") == "true" {
			eventLogger, err = events.NewBufferedLogger(
				obctx.Logger,
				events.NewDelayedLogger(eventLogger),
				cfg.BigQuery.EventBufferSize,
				cfg.BigQuery.EventBufferWorkers)
			if err != nil {
				return errors.Wrap(err, "create buffered logger")
			}
		}
	}

	// Create an uncached external doer, we never want to cache any responses.
	// Not only is the cache hit rate going to be really low and requests large-ish,
	// but also do we not want to retain any data.
	httpClient, err := httpcli.UncachedExternalClientFactory.Doer()
	if err != nil {
		return errors.Wrap(err, "failed to initialize external http client")
	}

	var meter = otel.GetMeterProvider().Meter("cody-gateway/redis")
	redisLatency, err := meter.Int64Histogram("cody-gateway.redis_latency",
		metric.WithDescription("Cody Gateway Redis client-side latency in milliseconds"))
	if err != nil {
		return errors.Wrap(err, "init metric 'redis_latency'")
	}

	redisCache := redispool.Cache.WithLatencyRecorder(func(call string, latency time.Duration, err error) {
		redisLatency.Record(context.Background(), latency.Milliseconds(), metric.WithAttributeSet(attribute.NewSet(
			attribute.Bool("error", err != nil),
			attribute.String("command", call))))
	})

	// Add a prefix to the store for globally unique keys and simpler pruning.
	rs := limiter.NewPrefixRedisStore("rate_limit:", newRedisStore(redisCache))

	// Ignore the error because it's already validated in the cfg.
	dotcomURL, _ := url.Parse(cfg.Dotcom.URL)
	dotcomURL.Path = ""
	rateLimitNotifier := notify.NewSlackRateLimitNotifier(
		obctx.Logger,
		redisCache,
		dotcomURL.String(),
		notify.Thresholds{
			// Detailed notifications for product subscriptions.
			codygateway.ActorSourceProductSubscription: []int{90, 95, 100},
			// No notifications for individual dotcom users - this can get quite
			// spammy.
			codygateway.ActorSourceDotcomUser: []int{},
		},
		cfg.ActorRateLimitNotify.SlackWebhookURL,
		func(ctx context.Context, url string, msg *slack.WebhookMessage) error {
			return slack.PostWebhookCustomHTTPContext(ctx, url, otelhttp.DefaultClient, msg)
		},
	)

	// Supported actor/auth sources
	sources := actor.NewSources(anonymous.NewSource(cfg.AllowAnonymous, cfg.ActorConcurrencyLimit))
	var dotcomClient graphql.Client
	if cfg.Dotcom.AccessToken != "" {
		// dotcom-based actor sources only if an access token is provided for
		// us to talk with the client
		obctx.Logger.Info("dotcom-based actor sources are enabled")
		dotcomClient = dotcom.NewClient(cfg.Dotcom.URL, cfg.Dotcom.AccessToken)
		sources.Add(
			productsubscription.NewSource(
				obctx.Logger,
				rcache.NewWithTTL(fmt.Sprintf("product-subscriptions:%s", productsubscription.SourceVersion), int(cfg.SourcesCacheTTL.Seconds())),
				dotcomClient,
				cfg.Dotcom.InternalMode,
				cfg.ActorConcurrencyLimit,
			),
			dotcomuser.NewSource(obctx.Logger, rcache.NewWithTTL(fmt.Sprintf("dotcom-users:%s", dotcomuser.SourceVersion), int(cfg.SourcesCacheTTL.Seconds())), dotcomClient, cfg.ActorConcurrencyLimit, rs, cfg.Dotcom.ActorRefreshCoolDownInterval),
		)
	} else {
		obctx.Logger.Warn("CODY_GATEWAY_DOTCOM_ACCESS_TOKEN is not set, dotcom-based actor sources are disabled")
	}

	authr := &auth.Authenticator{
		Logger:      obctx.Logger.Scoped("auth"),
		EventLogger: eventLogger,
		Sources:     sources,
	}

	// Set up our handler chain, which is run from the bottom up. Application handlers
	// come last.
	handler, err := httpapi.NewHandler(obctx.Logger, eventLogger, rs, httpClient, authr,
		&dotcomPromptRecorder{
			// TODO: Make configurable
			ttlSeconds: 60 * // minutes
				60,
			redis: redisCache,
		},
		&httpapi.Config{
			RateLimitNotifier:           rateLimitNotifier,
			Anthropic:                   cfg.Anthropic,
			OpenAI:                      cfg.OpenAI,
			Fireworks:                   cfg.Fireworks,
			EmbeddingsAllowedModels:     cfg.AllowedEmbeddingsModels,
			AutoFlushStreamingResponses: cfg.AutoFlushStreamingResponses,
			EnableAttributionSearch:     cfg.Attribution.Enabled,
			Sourcegraph:                 cfg.Sourcegraph,
		},
		dotcomClient)
	if err != nil {
		return errors.Wrap(err, "httpapi.NewHandler")
	}

	// Diagnostic layers
	handler = httpapi.NewDiagnosticsHandler(obctx.Logger, handler, cfg.DiagnosticsSecret, sources)

	// Collect request client for downstream handlers. Outside of dev, we always set up
	// Cloudflare in from of Cody Gateway. This comes first.
	handler = requestclient.ExternalHTTPMiddleware(handler)
	handler = requestinteraction.HTTPMiddleware(handler)

	// Initialize our server
	address := fmt.Sprintf(":%d", cfg.Port)
	server := httpserver.NewFromAddr(address, &http.Server{
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Handler:      handler,
	})

	// Set up redis-based distributed mutex for the source syncer worker
	p := redispool.Store.Pool()
	sourceWorkerMutex := redsync.New(redigo.NewPool(p)).NewMutex("source-syncer-worker",
		// Do not retry endlessly becuase it's very likely that someone else has
		// a long-standing hold on the mutex. We will try again on the next periodic
		// goroutine run.
		redsync.WithTries(1),
		// Expire locks at Nx sync interval to avoid contention while avoiding
		// the lock getting stuck for too long if something happens and to make
		// sure we can extend the lock after a sync. Instances spinning will
		// explicitly release the lock so this is a fallback measure.
		// Note that syncs can take several minutes.
		redsync.WithExpiry(4*cfg.SourcesSyncInterval))

	// Mark health server as ready and go!
	ready()
	obctx.Logger.Info("service ready", log.String("address", address))

	// Collect background routines
	backgroundRoutines := []goroutine.BackgroundRoutine{
		server,
		sources.Worker(obctx, sourceWorkerMutex, cfg.SourcesSyncInterval),
	}
	if w, ok := eventLogger.(goroutine.BackgroundRoutine); ok {
		// eventLogger is events.BufferedLogger
		backgroundRoutines = append(backgroundRoutines, w)
	}
	// Block until done
	goroutine.MonitorBackgroundRoutines(ctx, backgroundRoutines...)

	return nil
}

func newRedisStore(store redispool.KeyValue) limiter.RedisStore {
	return &redisStore{
		store: store,
	}
}

type redisStore struct {
	store redispool.KeyValue
}

func (s *redisStore) Incrby(key string, val int) (int, error) {
	return s.store.Incrby(key, val)
}

func (s *redisStore) GetInt(key string) (int, error) {
	i, err := s.store.Get(key).Int()
	if err != nil && err != redis.ErrNil {
		return 0, err
	}
	return i, nil
}

func (s *redisStore) TTL(key string) (int, error) {
	return s.store.TTL(key)
}

func (s *redisStore) Expire(key string, ttlSeconds int) error {
	return s.store.Expire(key, ttlSeconds)
}

func (s *redisStore) Del(key string) error {
	return s.store.Del(key)
}

func initOpenTelemetry(ctx context.Context, logger log.Logger, config config.OpenTelemetryConfig) (func(), error) {
	res, err := getOpenTelemetryResource(ctx)
	if err != nil {
		return nil, err
	}

	// Enable tracing, at this point tracing wouldn't have been enabled yet because
	// we run Cody Gateway without conf which means Sourcegraph tracing is not enabled.
	shutdownTracing, err := maybeEnableTracing(ctx,
		logger.Scoped("tracing"),
		config, res)
	if err != nil {
		return nil, errors.Wrap(err, "maybeEnableTracing")
	}

	shutdownMetrics, err := maybeEnableMetrics(ctx,
		logger.Scoped("metrics"),
		config, res)
	if err != nil {
		return nil, errors.Wrap(err, "maybeEnableMetrics")
	}

	return func() {
		var wg conc.WaitGroup
		wg.Go(shutdownTracing)
		wg.Go(shutdownMetrics)
		wg.Wait()
	}, nil
}

func getOpenTelemetryResource(ctx context.Context) (*resource.Resource, error) {
	// Identify your application using resource detection
	return resource.New(ctx,
		// Use the GCP resource detector to detect information about the GCP platform
		resource.WithDetectors(gcp.NewDetector()),
		// Keep the default detectors
		resource.WithTelemetrySDK(),
		// Add your own custom attributes to identify your application
		resource.WithAttributes(
			semconv.ServiceNameKey.String("cody-gateway"),
			semconv.ServiceVersionKey.String(version.Version()),
		),
	)
}

type dotcomPromptRecorder struct {
	ttlSeconds int
	redis      redispool.KeyValue
}

var _ completions.PromptRecorder = (*dotcomPromptRecorder)(nil)

func (p *dotcomPromptRecorder) Record(ctx context.Context, prompt string) error {
	// Only log prompts from Sourcegraph.com
	if !actor.FromContext(ctx).IsDotComActor() {
		return errors.New("attempted to record prompt from non-dotcom actor")
	}
	// Must expire entries
	if p.ttlSeconds == 0 {
		return errors.New("prompt recorder must have TTL")
	}
	// Always use trace ID as traceID - each trace = 1 request, and we always record
	// it in our entries.
	traceID := trace.FromContext(ctx).SpanContext().TraceID().String()
	if traceID == "" {
		return errors.New("prompt recorder requires a trace context")
	}
	return p.redis.SetEx(fmt.Sprintf("prompt:%s", traceID), p.ttlSeconds, prompt)
}
