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

	"github.com/sourcegraph/sourcegraph/internal/metrics"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/accesstoken"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
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
// being cancelled.
const maxRequestDuration = time.Minute

var timeToFirstEventMetrics = metrics.NewREDMetrics(
	prometheus.DefaultRegisterer,
	"completions_stream_first_event",
	metrics.WithLabels("model"),
	metrics.WithDurationBuckets([]float64{0.25, 0.5, 0.75, 1.0, 2.0, 3.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0, 18.0, 20.0, 25.0, 30.0}),
)

func newCompletionsHandler(
	logger log.Logger,
	userStore database.UserStore,
	accessTokenStore database.AccessTokenStore,
	events *telemetry.EventRecorder,
	feature types.CompletionsFeature,
	rl RateLimiter,
	traceFamily string,
	getModel func(context.Context, types.CodyCompletionRequestParameters, *conftypes.CompletionsConfig) (string, error),
) http.Handler {
	responseHandler := newSwitchingResponseHandler(logger, feature)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, fmt.Sprintf("unsupported method %s", r.Method), http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), maxRequestDuration)
		defer cancel()

		if isEnabled := cody.IsCodyEnabled(ctx); !isEnabled {
			http.Error(w, "cody experimental feature flag is not enabled for current user", http.StatusUnauthorized)
			return
		}

		completionsConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
		if completionsConfig == nil {
			http.Error(w, "completions are not configured or disabled", http.StatusInternalServerError)
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
		isCodyProEnabled := featureflag.FromContext(ctx).GetBoolOr("cody-pro", false)
		isDotcom := envvar.SourcegraphDotComMode()
		isProviderCodyGateway := completionsConfig.Provider == conftypes.CompletionsProviderNameSourcegraph
		if isCodyProEnabled && isDotcom && isProviderCodyGateway {
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
				isProUser := user.CodyProEnabledAt != nil
				respondRateLimited(w, unwrap, isDotcom, isCodyProEnabled, isProUser)
				return
			}
			l.Warn("Rate limit error", log.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		responseHandler(ctx, requestParams.CompletionRequestParameters, completionClient, w, userStore)
	})
}

func respondRateLimited(w http.ResponseWriter, err RateLimitExceededError, isDotcom, isCodyProEnabled, isProUser bool) {
	// Rate limit exceeded, write well known headers and return correct status code.
	w.Header().Set("x-ratelimit-limit", strconv.Itoa(err.Limit))
	w.Header().Set("x-ratelimit-remaining", strconv.Itoa(max(err.Limit-err.Used, 0)))
	w.Header().Set("retry-after", err.RetryAfter.Format(time.RFC1123))
	if isDotcom && isCodyProEnabled {
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
func newSwitchingResponseHandler(logger log.Logger, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
	nonStreamer := newNonStreamingResponseHandler(logger, feature)
	streamer := newStreamingResponseHandler(logger, feature)
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore,
	) {
		if requestParams.IsStream(feature) {
			streamer(ctx, requestParams, cc, w, userStore)
		} else {
			nonStreamer(ctx, requestParams, cc, w, userStore)
		}
	}
}

// newStreamingResponseHandler handles streaming requests to an LLM provider,
// It writes events to an SSE stream as they come in.
func newStreamingResponseHandler(logger log.Logger, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
		var eventWriter = sync.OnceValue[*streamhttp.Writer](func() *streamhttp.Writer {
			eventWriter, err := streamhttp.NewWriter(w)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return nil
			}
			return eventWriter
		})

		// Always send a final done event so clients know the stream is shutting down.
		firstEventObserved := false
		defer func() {
			if firstEventObserved {
				if ev := eventWriter(); ev != nil {
					_ = ev.Event("done", map[string]any{})
				}
			}
		}()
		start := time.Now()
		err := cc.Stream(ctx, feature, requestParams,
			func(event types.CompletionResponse) error {
				if !firstEventObserved {
					firstEventObserved = true
					timeToFirstEventMetrics.Observe(time.Since(start).Seconds(), 1, nil, requestParams.Model)
				}
				if ev := eventWriter(); ev != nil {
					return ev.Event("completion", event)
				}
				return nil
			})
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
					isProUser := user.CodyProEnabledAt != nil
					isCodyProEnabled := featureflag.FromContext(ctx).GetBoolOr("cody-pro", false)
					isDotcom := envvar.SourcegraphDotComMode()
					if isDotcom && isCodyProEnabled {
						if isProUser {
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
			if ev := eventWriter(); ev != nil {
				if err := ev.Event("error", map[string]string{"error": err.Error()}); err != nil {
					l.Error("error reporting streaming completion error", log.Error(err))
				}
			}
			return
		}
	}
}

// newNonStreamingResponseHandler handles non-streaming requests to an LLM provider,
// awaiting the complete response before writing it back in a structured JSON response
// to the client.
func newNonStreamingResponseHandler(logger log.Logger, feature types.CompletionsFeature) func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
	return func(ctx context.Context, requestParams types.CompletionRequestParameters, cc types.CompletionsClient, w http.ResponseWriter, userStore database.UserStore) {
		completion, err := cc.Complete(ctx, feature, requestParams)
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
					isProUser := user.CodyProEnabledAt != nil
					isCodyProEnabled := featureflag.FromContext(ctx).GetBoolOr("cody-pro", false)
					isDotcom := envvar.SourcegraphDotComMode()
					if isDotcom && isCodyProEnabled {
						if isProUser {
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
