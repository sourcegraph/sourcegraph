package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/overhead"

	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/actor"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/events"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/httpapi/featurelimiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/limiter"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/notify"
	"github.com/sourcegraph/sourcegraph/cmd/cody-gateway/internal/response"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	sgtrace "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const usageHeaderName = "X-Token-Usage"
const baseLoggerScope = "embeddingshandler"

func NewHandler(
	baseLogger log.Logger,
	eventLogger events.Logger,
	rs limiter.RedisStore,
	rateLimitNotifier notify.RateLimitNotifier,
	mf ModelFactory,
	allowedModels []string,
) http.Handler {
	baseLogger = baseLogger.Scoped(baseLoggerScope)

	return featurelimiter.HandleFeature(
		baseLogger,
		eventLogger,
		rs,
		rateLimitNotifier,
		codygateway.FeatureEmbeddings,
		embeddingsHandler(baseLogger, eventLogger, allowedModels, mf),
		func(responseHeaders http.Header) (int, error) {
			uh := responseHeaders.Get(usageHeaderName)
			if uh == "" {
				return 0, errors.New("no usage header set on response")
			}
			usage, err := strconv.Atoi(uh)
			if err != nil {
				return 0, errors.Wrap(err, "failed to parse usage header as number")
			}
			return usage, nil
		})
}

func NewUnlimitedHandler(
	baseLogger log.Logger,
	eventLogger events.Logger,
	mf ModelFactory,
	allowedModels []string,
) http.Handler {

	baseLogger = baseLogger.Scoped(baseLoggerScope)

	return embeddingsHandler(baseLogger, eventLogger, allowedModels, mf)
}

func embeddingsHandler(baseLogger log.Logger, eventLogger events.Logger, allowedModels []string, mf ModelFactory) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		act := actor.FromContext(r.Context())
		logger := act.Logger(sgtrace.Logger(r.Context(), baseLogger))
		// This will never be nil as the rate limiter middleware checks this before.
		// TODO: Should we read the rate limit from context, and store it in the rate
		// limiter to make this less dependent on these two logics to remain the same?
		rateLimit, ok := act.RateLimits[codygateway.FeatureEmbeddings]
		if !ok {
			response.JSONError(logger, w, http.StatusInternalServerError, errors.Newf("rate limit for %q not found", string(codygateway.FeatureEmbeddings)))
			return
		}

		// Parse the request body.
		var body codygateway.EmbeddingsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.JSONError(logger, w, http.StatusBadRequest, errors.Wrap(err, "failed to parse request body"))
			return
		}

		if !isAllowedModel(intersection(allowedModels, rateLimit.AllowedModels), body.Model) {
			response.JSONError(logger, w, http.StatusBadRequest, errors.Newf("model %q is not allowed", body.Model))
			return
		}

		c, ok := mf.ForModel(body.Model)
		if !ok {
			response.JSONError(logger, w, http.StatusBadRequest, errors.Newf("model %q not known", body.Model))
			return
		}

		// Add the client type to the logger fields.
		logger = logger.With(log.String("client", c.ProviderName()))

		var (
			upstreamStarted    = time.Now()
			upstreamFinished   time.Duration
			upstreamStatusCode = -1
			// resolvedStatusCode is the status code that we returned to the
			// client - in most case it is the same as upstreamStatusCode,
			// but sometimes we write something different.
			resolvedStatusCode int = -1
			usedTokens         int = -1
		)
		defer func() {
			o := overhead.FromContext(r.Context())
			o.Feature = codygateway.FeatureEmbeddings
			o.UpstreamLatency = upstreamFinished
			o.Provider = c.ProviderName()
			o.Stream = false

			if span := oteltrace.SpanFromContext(r.Context()); span.IsRecording() {
				span.SetAttributes(
					attribute.Int("upstreamStatusCode", upstreamStatusCode),
					attribute.Int("resolvedStatusCode", resolvedStatusCode))
			}
			err := eventLogger.LogEvent(
				r.Context(),
				events.Event{
					Name:       codygateway.EventNameEmbeddingsFinished,
					Source:     act.Source.Name(),
					Identifier: act.ID,
					Metadata: map[string]any{
						"model": body.Model,
						codygateway.CompletionsEventFeatureMetadataField: codygateway.CompletionsEventFeatureEmbeddings,
						"upstream_request_duration_ms":                   upstreamFinished.Milliseconds(),
						"resolved_status_code":                           resolvedStatusCode,
						codygateway.EmbeddingsTokenUsageMetadataField:    usedTokens,
						"batch_size": len(body.Input),
						"input_character_count": func() (characters int) {
							for _, input := range body.Input {
								characters += len(input)
							}
							return characters
						}(),
					},
				},
			)
			if err != nil {
				logger.Error("failed to log event", log.Error(err))
			}
		}()

		resp, ut, err := c.GenerateEmbeddings(r.Context(), body)
		usedTokens = ut
		upstreamFinished = time.Since(upstreamStarted)
		if err != nil {
			// This is an error path, so always set a default retry-after
			// on errors that discourages Sourcegraph clients from retrying
			// at all - embeddings will likely be run by embeddings workers
			// that will eventually retry on a more reasonable schedule.
			w.Header().Set("retry-after", "60")

			// If a status error is returned, pass through the code and error
			var statusCodeErr response.HTTPStatusCodeError
			if errors.As(err, &statusCodeErr) {
				resolvedStatusCode = statusCodeErr.HTTPStatusCode()
				response.JSONError(logger, w, resolvedStatusCode, statusCodeErr)
				// Record original code if the status error is a custom one
				if originalCode, ok := statusCodeErr.IsCustom(); ok {
					upstreamStatusCode = originalCode
				}
				return
			}

			// More user-friendly message for timeouts
			if errors.Is(err, context.DeadlineExceeded) {
				resolvedStatusCode = http.StatusGatewayTimeout
				response.JSONError(logger, w, resolvedStatusCode,
					errors.Newf("request to upstream provider %s timed out", c.ProviderName()))
				return
			}

			// Return generic error for other unexpected errors.
			resolvedStatusCode = http.StatusInternalServerError
			response.JSONError(logger, w, resolvedStatusCode, err)
			return
		}

		w.Header().Add(usageHeaderName, strconv.Itoa(usedTokens))

		data, err := json.Marshal(resp)
		if err != nil {
			resolvedStatusCode = http.StatusInternalServerError
			response.JSONError(logger, w, resolvedStatusCode, errors.Wrap(err, "failed to marshal response"))
			return
		}

		w.Header().Add("Content-Type", "application/json; charset=utf-8")
		// Write implicitly returns a 200 status code if one isn't set yet
		if resolvedStatusCode <= 0 {
			resolvedStatusCode = 200
		}
		_, _ = w.Write(data)
	}
}

func isAllowedModel(allowedModels []string, model string) bool {
	for _, m := range allowedModels {
		if strings.EqualFold(m, model) {
			return true
		}
	}
	return false
}

func intersection(a, b []string) (c []string) {
	for _, val := range a {
		if slices.Contains(b, val) {
			c = append(c, val)
		}
	}
	return c
}
