package httpapi

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/guardrails"
	"github.com/sourcegraph/sourcegraph/internal/metrics"

	"github.com/sourcegraph/sourcegraph/internal/accesstoken"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/hashutil"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/completions/client"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/internal/telemetry"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

// maxRequestDuration is the maximum amount of time a request can take before
// being cancelled as DeadlineExceeded.
const maxRequestDuration = 2 * time.Minute

var timeToFirstEventMetrics = metrics.NewREDMetrics(
	prometheus.DefaultRegisterer,
	"completions_stream_first_event",
	metrics.WithLabels("model"),
	metrics.WithDurationBuckets([]float64{0.25, 0.5, 0.75, 1.0, 2.0, 3.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 25.0, 30.0}),
)

func newCompletionsHandler(
	logger log.Logger,
	db database.DB,
	userStore database.UserStore,
	accessTokenStore database.AccessTokenStore,
	events *telemetry.EventRecorder,
	test guardrails.AttributionTest,
	feature types.CompletionsFeature,
	rl RateLimiter,
	traceFamily string,
	getModel func(context.Context, types.CodyCompletionRequestParameters, *conftypes.CompletionsConfig) (string, error),
) http.Handler {
	responseHandler := newSwitchingResponseHandler(logger, db, feature)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, fmt.Sprintf("unsupported method %s", r.Method), http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), maxRequestDuration)
		defer cancel()

		if isEnabled, reason := cody.IsCodyEnabled(ctx, db); !isEnabled {
			http.Error(w, fmt.Sprintf("cody is not enabled: %s", reason), http.StatusUnauthorized)
			return
		}

		completionsConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
		if completionsConfig == nil {
			http.Error(w, "completions are not configured or disabled", http.StatusInternalServerError)
			return
		}

		var version types.CompletionsVersion
		versionParam := r.URL.Query().Get("api-version")
		if versionParam == "" {
			version = types.CompletionsVersionLegacy
		} else if versionParam == "1" {
			version = types.CompletionsV1
		} else {
			http.Error(w, "Unsupported API Version (Please update your client)", http.StatusNotAcceptable)
			return
		}

		var requestParams types.CodyCompletionRequestParameters
		if err := json.NewDecoder(r.Body).Decode(&requestParams); err != nil {
			http.Error(w, "could not decode request body", http.StatusBadRequest)
			return
		}

		// TODO: Model is not configurable but technically allowed in the request body right now.
		var err error
		requestParams.Model, err = getModel(ctx, requestParams, completionsConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx, done := Trace(ctx, traceFamily, requestParams.Model, requestParams.MaxTokensToSample).
			WithErrorP(&err).
			WithRequest(r).
			Build()
		defer done()

		// Use the user's access token for Cody Gateway on dotcom if PLG is enabled.
		accessToken := completionsConfig.AccessToken
		isDotcom := dotcom.SourcegraphDotComMode()
		isProviderCodyGateway := completionsConfig.Provider == conftypes.CompletionsProviderNameSourcegraph
		if isDotcom && isProviderCodyGateway {
			// Note: if we have no Authorization header, that's fine too, this will return an error
			apiToken, _, err := authz.ParseAuthorizationHeader(r.Header.Get("Authorization"))
			if err != nil {
				// No actor either, so we fail.
				if r.Header.Get("Authorization") != "" || sgactor.FromContext(ctx) == nil {
					trace.Logger(ctx, logger).Info("Error parsing auth header", log.String("Authorization header", r.Header.Get("Authorization")), log.Error(err))
					http.Error(w, "Error parsing auth header", http.StatusUnauthorized)
					return
				}

				// Get or create an internal token to use.
				actor := sgactor.FromContext(ctx)
				apiTokenSha256, err := accessTokenStore.GetOrCreateInternalToken(ctx, actor.UID, []string{"user:all"})
				if err != nil {
					trace.Logger(ctx, logger).Info("Error creating internal access token", log.Error(err))
					http.Error(w, "Missing auth header", http.StatusUnauthorized)
					return
				}
				// Convert the user's sha256-encoded access token to an "sgd_" token for Cody Gateway.
				// Note: we can't use accesstoken.GenerateDotcomUserGatewayAccessToken here because
				// we only need to hash this once, not twice, as this is already an SHA256-encoding
				// of the original token.
				accessToken = accesstoken.DotcomUserGatewayAccessTokenPrefix + hex.EncodeToString(hashutil.ToSHA256Bytes(apiTokenSha256))
			} else {
				accessToken, err = accesstoken.GenerateDotcomUserGatewayAccessToken(apiToken)
				if err != nil {
					trace.Logger(ctx, logger).Info("Access token generation failed", log.String("API token", apiToken), log.Error(err))
					http.Error(w, "Access token generation failed", http.StatusUnauthorized)
					return
				}
			}
		}

		completionClient, err := client.Get(
			logger,
			events,
			completionsConfig.Endpoint,
			completionsConfig.Provider,
			accessToken,
		)
		l := trace.Logger(ctx, logger)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if !isDotcom || !isProviderCodyGateway {
			// Check rate limit.
			err = rl.TryAcquire(ctx)
			if err != nil {
				if unwrap, ok := err.(RateLimitExceededError); ok {
					actor := sgactor.FromContext(ctx)
					user, err := actor.User(ctx, userStore)
					if err != nil {
						l.Error("Error while fetching user", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}
					subscription, err := cody.SubscriptionForUser(ctx, db, *user)
					if err != nil {
						l.Error("Error while fetching user's cody subscription", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}

					respondRateLimited(w, unwrap, isDotcom, subscription.ApplyProRateLimits)
					return
				}
				l.Warn("Rate limit error", log.Error(err))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
		}

		responseHandler(ctx, requestParams.CompletionRequestParameters, version, completionClient, w, userStore, test)
	})
}

func respondRateLimited(w http.ResponseWriter, err RateLimitExceededError, isDotcom, isProUser bool) {
	// Rate limit exceeded, write well known headers and return correct status code.
	w.Header().Set("x-ratelimit-limit", strconv.Itoa(err.Limit))
	w.Header().Set("x-ratelimit-remaining", strconv.Itoa(max(err.Limit-err.Used, 0)))
	w.Header().Set("retry-after", err.RetryAfter.Format(time.RFC1123))
	if isDotcom {
		if isProUser {
			w.Header().Set("x-is-cody-pro-user", "true")
		} else {
			w.Header().Set("x-is-cody-pro-user", "false")
		}
	}
	http.Error(w, err.Error(), http.StatusTooManyRequests)
}

// newSwitchingResponseHandler handles requests to an LLM provider, and wraps the correct
// handler based on the requestParams.Stream flag.
func newSwitchingResponseHandler(logger log.Logger, db database.DB, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore, test guardrails.AttributionTest) {
	nonStreamer := newNonStreamingResponseHandler(logger, db, feature)
	streamer := newStreamingResponseHandler(logger, db, feature)
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore, test guardrails.AttributionTest) {
		if requestParams.IsStream(feature) {
			streamer(ctx, requestParams, version, cc, w, userStore, test)
		} else {
			// TODO(#59832): Add attribution to non-streaming endpoint.
			nonStreamer(ctx, requestParams, version, cc, w, userStore)
		}
	}
}

// newStreamingResponseHandler handles streaming requests to an LLM provider,
// It writes events to an SSE stream as they come in.
func newStreamingResponseHandler(logger log.Logger, db database.DB, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore, test guardrails.AttributionTest) {
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore, test guardrails.AttributionTest) {
		var eventWriter = sync.OnceValue[*streamhttp.Writer](func() *streamhttp.Writer {
			eventWriter, err := streamhttp.NewWriter(w)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return nil
			}
			return eventWriter
		})

		// Isolate writing events.
		var mu sync.Mutex
		writeEvent := func(name string, data any) error {
			mu.Lock()
			defer mu.Unlock()
			if ev := eventWriter(); ev != nil {
				return ev.Event(name, data)
			}
			return nil
		}

		// Always send a final done event so clients know the stream is shutting down.
		firstEventObserved := false
		defer func() {
			if firstEventObserved {
				_ = writeEvent("done", map[string]any{})
			}
		}()
		start := time.Now()
		eventSink := func(e types.CompletionResponse) error {
			return writeEvent("completion", e)
		}
		attributionErrorLog := func(err error) {
			l := trace.Logger(ctx, logger)
			if err := writeEvent("attribution-error", map[string]string{"error": err.Error()}); err != nil {
				l.Error("error reporting attribution error", log.Error(err))
			} else {
				return
			}
			l.Error("attribution error", log.Error(err))
		}
		f := guardrails.NoopCompletionsFilter(eventSink)
		if cf := conf.GetConfigFeatures(conf.SiteConfig()); cf != nil && cf.Attribution &&
			featureflag.FromContext(ctx).GetBoolOr("autocomplete-attribution", true) {
			ff, err := guardrails.NewCompletionsFilter(guardrails.CompletionsFilterConfig{
				Sink:             eventSink,
				Test:             test,
				AttributionError: attributionErrorLog,
			})
			if err != nil {
				attributionErrorLog(err)
			} else {
				f = ff
			}
		}
		err := cc.Stream(ctx, feature, version, requestParams,
			func(event types.CompletionResponse) error {
				if !firstEventObserved {
					firstEventObserved = true
					timeToFirstEventMetrics.Observe(time.Since(start).Seconds(), 1, nil, requestParams.Model)
				}
				return f.Send(ctx, event)
			}, logger)
		if err != nil {
			l := trace.Logger(ctx, logger)

			logFields := []log.Field{log.Error(err)}
			if errNotOK, ok := types.IsErrStatusNotOK(err); ok {
				if !firstEventObserved && errNotOK.StatusCode == http.StatusTooManyRequests {
					actor := sgactor.FromContext(ctx)
					user, err := actor.User(ctx, userStore)
					if err != nil {
						l.Error("Error while fetching user", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}

					subscription, err := cody.SubscriptionForUser(ctx, db, *user)
					if err != nil {
						l.Error("Error while fetching user's cody subscription", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}

					isDotcom := dotcom.SourcegraphDotComMode()
					if isDotcom {
						if subscription.ApplyProRateLimits {
							w.Header().Set("x-is-cody-pro-user", "true")
						} else {
							w.Header().Set("x-is-cody-pro-user", "false")
						}
					}
					errNotOK.WriteHeader(w)
					return

				}
				if tc := errNotOK.SourceTraceContext; tc != nil {
					logFields = append(logFields,
						log.String("sourceTraceContext.traceID", tc.TraceID),
						log.String("sourceTraceContext.spanID", tc.SpanID))
				}
			}
			l.Error("error while streaming completions", logFields...)

			// Note that we do NOT attempt to forward the status code to the
			// client here, since we are using streamhttp.Writer - see
			// streamhttp.NewWriter for more details. Instead, we send an error
			// event, which clients should check as appropriate.
			if !firstEventObserved {
				firstEventObserved = true
				timeToFirstEventMetrics.Observe(time.Since(start).Seconds(), 1, nil, requestParams.Model)
			}
			if err := writeEvent("error", map[string]string{"error": err.Error()}); err != nil {
				l.Error("error reporting streaming completion error", log.Error(err))
			}
			return
		}
		if f != nil { // if autocomplete-attribution enabled
			if err := f.WaitDone(ctx); err != nil {
				l := trace.Logger(ctx, logger)
				if err := writeEvent("error", map[string]string{"error": err.Error()}); err != nil {
					l.Error("error reporting streaming completion error", log.Error(err))
				}
			}
		}
	}
}

// newNonStreamingResponseHandler handles non-streaming requests to an LLM provider,
// awaiting the complete response before writing it back in a structured JSON response
// to the client.
func newNonStreamingResponseHandler(logger log.Logger, db database.DB, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, version types.CompletionsVersion, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
		completion, err := cc.Complete(ctx, feature, version, requestParams, logger)
		if err != nil {
			logFields := []log.Field{log.Error(err)}

			// Propagate the upstream headers to the client if available.
			if errNotOK, ok := types.IsErrStatusNotOK(err); ok {
				errNotOK.WriteHeader(w)

				if errNotOK.StatusCode == http.StatusTooManyRequests {
					actor := sgactor.FromContext(ctx)
					user, err := actor.User(ctx, userStore)
					if err != nil {
						logger.Error("Error while fetching user", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}
					subscription, err := cody.SubscriptionForUser(ctx, db, *user)
					if err != nil {
						logger.Error("Error while fetching user's cody subscription", log.Error(err))
						http.Error(w, "Internal server error", http.StatusInternalServerError)
						return
					}

					isDotcom := dotcom.SourcegraphDotComMode()
					if isDotcom {
						if subscription.ApplyProRateLimits {
							w.Header().Set("x-is-cody-pro-user", "true")
						} else {
							w.Header().Set("x-is-cody-pro-user", "false")
						}
					}
					return

				}

				if tc := errNotOK.SourceTraceContext; tc != nil {
					logFields = append(logFields,
						log.String("sourceTraceContext.traceID", tc.TraceID),
						log.String("sourceTraceContext.spanID", tc.SpanID))
				}
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			_, err = w.Write([]byte(err.Error()))
			if err != nil {
				logger.Error("failed to write", log.Error(err))
			}

			trace.Logger(ctx, logger).Error("error on completion", logFields...)
			return
		}

		completionBytes, err := json.Marshal(completion)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(completionBytes)
	}
}
