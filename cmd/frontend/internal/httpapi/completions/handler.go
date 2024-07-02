package completions

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/guardrails"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/lib/errors"

	"github.com/sourcegraph/sourcegraph/internal/accesstoken"
	sgactor "github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/hashutil"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
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
const maxRequestDuration = 8 * time.Minute

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
	getModel getModelFn,
) http.Handler {
	responseHandler := newSwitchingResponseHandler(logger, db, feature)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), maxRequestDuration)
		defer cancel()

		if isEnabled, reason := cody.IsCodyEnabled(ctx, db); !isEnabled {
			errResponse := fmt.Sprintf("cody is not enabled: %s", reason)
			http.Error(w, errResponse, http.StatusUnauthorized)
			return
		}

		completionsConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
		if completionsConfig == nil {
			http.Error(w, "completions are not configured or disabled", http.StatusInternalServerError)
			return
		}

		var version types.CompletionsVersion
		switch versionParam := r.URL.Query().Get("api-version"); versionParam {
		case "":
			version = types.CompletionsVersionLegacy
		case "1":
			version = types.CompletionsV1
		default:
			logger.Warn(
				"blocking request because unrecognized CompletionsVersion API param",
				log.String("version", versionParam))
			http.Error(w, "Unsupported API Version (Please update your client)", http.StatusNotAcceptable)
			return
		}

		// Enterprise customers may define instance-wide Cody context filters (aka Cody Ignore) in the site config.
		// To ensure Cody clients respect these restrictions, we enforce the minimum supported client version.
		isDotcom := dotcom.SourcegraphDotComMode()
		if !isDotcom {
			if err := checkClientCodyIgnoreCompatibility(ctx, db, r); err != nil {
				logger.Info("rejecting request due to CodyIngore compat", log.Error(err))
				http.Error(w, err.Error(), err.statusCode)
				return
			}
		}

		// We don't perform any sort of validation. So we would silently accept a totally bogus
		// JSON payload. And just have a zero-value CompletionRequestParameters, e.g. no prompt.
		var requestParams types.CodyCompletionRequestParameters
		if err := json.NewDecoder(r.Body).Decode(&requestParams); err != nil {
			logger.Warn("malformed CodyCompletionRequestParameters", log.Error(err))
			http.Error(w, "could not decode request body", http.StatusBadRequest)
			return
		}

		// TODO: Model is not configurable but technically allowed in the request body right now.
		var err error
		requestParams.Model, err = getModel(ctx, requestParams, completionsConfig)
		requestParams.User = completionsConfig.User
		requestParams.AzureChatModel = completionsConfig.AzureChatModel
		requestParams.AzureCompletionModel = completionsConfig.AzureCompletionModel
		requestParams.AzureUseDeprecatedCompletionsAPIForOldModels = completionsConfig.AzureUseDeprecatedCompletionsAPIForOldModels
		if err != nil {
			// NOTE: We return the raw error to the user assuming that it contains relevant
			// user-facing diagnostic information, and doesn't leak any internal details.
			logger.Info("error fetching model", log.Error(err))
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
			// NOTE: The HTTP response defaults to 200 if not set explicitly before writing the response.
			// This means that this function will never serve a non-OK response. (Which makes sense, since
			// we don't know ahead of time if some later SSE event will fail.
			eventWriter, err := streamhttp.NewWriter(w)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return nil
			}
			return eventWriter
		})

		// Isolate writing events.
		var mu sync.Mutex
		writeEvent := func(name string, data any) (err error) {
			// Attribution search is currently panicing. The main hypothesis
			// is calling writeEvent on a finished request in the case of an
			// error. Rather than panicing the whole process we convert it
			// into an error which will get logged.
			//
			// https://github.com/sourcegraph/sourcegraph/issues/60439
			defer func() {
				if rec := recover(); rec != nil {
					err = errors.WithStack(errors.Errorf("recovered panic in completions writeEvent: %v", rec))
				}
			}()

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
			factory := guardrails.NewCompletionsFilter
			// TODO(#61828) - Validate & cleanup:
			// 1.  If experiments are successful on S2 and we do not see any panics,
			//     please switch the feature flag default value to true.
			// 2.  Afterwards cleanup the implementation and only use V2 completion filter.
			//     Remove v1 implementation completely.
			if featureflag.FromContext(ctx).GetBoolOr("autocomplete-attribution-v2", false) {
				factory = guardrails.NewCompletionsFilter2
			}
			ff, err := factory(guardrails.CompletionsFilterConfig{
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

		// Build and send the completions request.
		compReq := types.CompletionRequest{
			Feature:    feature,
			Version:    version,
			Parameters: requestParams,
		}
		sendEventFn := func(event types.CompletionResponse) error {
			if !firstEventObserved {
				firstEventObserved = true
				timeToFirstEventMetrics.Observe(time.Since(start).Seconds(), 1, nil, requestParams.Model)
			}
			return f.Send(ctx, event)
		}
		err := cc.Stream(ctx, logger, compReq, sendEventFn)
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
		compRequest := types.CompletionRequest{
			Feature:    feature,
			Version:    version,
			Parameters: requestParams,
		}
		completion, err := cc.Complete(ctx, logger, compRequest)
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

type codyIgnoreCompatibilityError struct {
	reason     string
	statusCode int
}

func (e *codyIgnoreCompatibilityError) Error() string {
	// prefix value is used to identify specific errors in the Cody clients codebases.
	// When changing its value be sure to update the clients code.
	const prefix = "ClientCodyIgnoreCompatibilityError"
	return fmt.Sprintf("%s: %s", prefix, e.reason)
}

// checkClientCodyIgnoreCompatibility checks if the client version respects Cody context filters (aka Cody Ignore) defined in the site config.
// A non-nil codyIgnoreCompatibilityError implies that the HTTP request should be rejected with the error text and code from the returned error.
// Error text is safe to be surfaced to end user.
func checkClientCodyIgnoreCompatibility(ctx context.Context, db database.DB, r *http.Request) *codyIgnoreCompatibilityError {
	// If Cody context filters are not defined on the instance, we do not restrict client version.
	// Because the site hasn't configured Cody Ignore, no need to enforce it.
	if conf.SiteConfig().CodyContextFilters == nil {
		return nil
	}

	clientName := types.CodyClientName(r.URL.Query().Get("client-name"))
	if clientName == "" {
		return &codyIgnoreCompatibilityError{
			reason:     "\"client-name\" query param is required.",
			statusCode: http.StatusNotAcceptable,
		}
	}

	// Intended for development use only.
	// TODO: remove after `CodyContextFilters` support is added to the IDE clients.
	flag, err := db.FeatureFlags().GetFeatureFlag(ctx, "cody-context-filters-clients-test-mode")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return &codyIgnoreCompatibilityError{
			reason:     "Failed to get feature flag value.",
			statusCode: http.StatusInternalServerError,
		}
	}
	isClientsTestMode := flag != nil && flag.Bool.Value

	// clientVersionConstraint defines the minimum client version required to support Cody Ignore.
	type clientVersionConstraint struct {
		client     types.CodyClientName
		constraint string // represents client version constraint following the semver spec
	}
	var cvc clientVersionConstraint
	switch clientName {
	case types.CodyClientWeb:
		// Cody Web is of the same version as the Sourcegraph instance, thus no version constraint is needed.
		return nil
	case types.CodyClientVscode:
		cvc = clientVersionConstraint{client: clientName, constraint: ">= 1.20.0"}
		if isClientsTestMode {
			// set the constraint to a lower pre-release version to enable testing
			cvc.constraint = ">= 1.16.0-0"
		}
	case types.CodyClientJetbrains:
		cvc = clientVersionConstraint{client: clientName, constraint: ">= 6.0.0"}
		if isClientsTestMode {
			// set the constraint to a lower pre-release version to enable testing
			cvc.constraint = ">= 5.5.8-0"
		}
	default:
		return &codyIgnoreCompatibilityError{
			reason:     fmt.Sprintf("please use one of the supported clients: %s, %s, %s.", types.CodyClientVscode, types.CodyClientJetbrains, types.CodyClientWeb),
			statusCode: http.StatusNotAcceptable,
		}
	}

	clientVersion := r.URL.Query().Get("client-version")
	if clientVersion == "" {
		return &codyIgnoreCompatibilityError{
			reason:     "\"client-version\" query param is required.",
			statusCode: http.StatusNotAcceptable,
		}
	}

	c, err := semver.NewConstraint(cvc.constraint)
	if err != nil {
		return &codyIgnoreCompatibilityError{
			reason:     fmt.Sprintf("Cody for %s version constraint %q doesn't follow semver spec.", cvc.client, cvc.constraint),
			statusCode: http.StatusInternalServerError,
		}
	}

	v, err := semver.NewVersion(clientVersion)
	if err != nil {
		return &codyIgnoreCompatibilityError{
			reason:     fmt.Sprintf("Cody for %s version %q doesn't follow semver spec.", cvc.client, clientVersion),
			statusCode: http.StatusBadRequest,
		}
	}

	ok := c.Check(v)
	if !ok {
		return &codyIgnoreCompatibilityError{
			reason:     fmt.Sprintf("Cody for %s version %q doesn't match version constraint %q. Please upgrade your client.", cvc.client, clientVersion, cvc.constraint),
			statusCode: http.StatusNotAcceptable,
		}
	}

	return nil
}
