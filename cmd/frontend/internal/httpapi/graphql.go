package httpapi

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"github.com/throttled/throttled/v2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/audit"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/cookie"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const costEstimationMetricActorTypeLabel = "actor_type"

var (
	costHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_graphql_cost_distribution",
		Help:    "The maximum cost seen from a GraphQL query",
		Buckets: prometheus.ExponentialBucketsRange(1, 500_000, 8),
	}, []string{costEstimationMetricActorTypeLabel})
)

func actorTypeLabel(isInternal, anonymous bool, requestSource trace.SourceType) string {
	if isInternal {
		return "internal"
	}
	if anonymous {
		return "anonymous"
	}
	if requestSource != "" {
		return string(requestSource)
	}
	return "unknown"
}

func exceedsLimit(costValue, limitValue int) bool {
	return costValue > limitValue
}

func writeViolationError(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusBadRequest) // 400 because retrying won't help
	return writeJSON(w, graphql.Response{
		Errors: []*gqlerrors.QueryError{
			{
				Message: "query exceeds maximum query cost",
			},
		},
	})
}

func serveGraphQL(logger log.Logger, schema *graphql.Schema, rlw graphqlbackend.LimitWatcher, isInternal bool) func(w http.ResponseWriter, r *http.Request) (err error) {
	return func(w http.ResponseWriter, r *http.Request) (err error) {
		if r.Method != "POST" {
			// The URL router should not have routed to this handler if method is not POST, but just in
			// case.
			return errors.New("method must be POST")
		}

		// We use the query to denote the name of a GraphQL request, e.g. for /.api/graphql?Repositories
		// the name is "Repositories".
		requestName := "unknown"
		if r.URL.RawQuery != "" {
			requestName = r.URL.RawQuery
		}
		requestSource := search.GuessSource(r)

		// Used by the prometheus tracer
		r = r.WithContext(trace.WithGraphQLRequestName(r.Context(), requestName))
		r = r.WithContext(trace.WithRequestSource(r.Context(), requestSource))

		if r.Header.Get("Content-Encoding") == "gzip" {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				return errors.Wrap(err, "failed to decompress request body")
			}

			r.Body = gzipReader

			defer gzipReader.Close()
		}

		var params graphQLQueryParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			return errors.Wrapf(err, "failed to decode request")
		}

		traceData := traceData{
			queryParams:   params,
			isInternal:    isInternal,
			requestName:   requestName,
			requestSource: string(requestSource),
		}

		defer func() {
			instrumentGraphQL(traceData)
			recordAuditLog(r.Context(), logger, traceData)
		}()

		uid, isIP, anonymous := getUID(r)
		traceData.uid = uid
		traceData.anonymous = anonymous

		var cost *graphqlbackend.QueryCost
		var costErr error

		// Calculating the cost is cheaper than validating the schema. We calculate the cost first to prevent resource exhaustion.
		cost, costErr = graphqlbackend.EstimateQueryCost(params.Query, params.Variables)
		if costErr != nil {
			logger.Debug("failed to estimate GraphQL cost",
				log.Error(costErr))
			traceData.costError = costErr
		} else if cost != nil {
			traceData.cost = cost

			// Track the cost distribution of requests in a histogram.
			costHistogram.WithLabelValues(actorTypeLabel(isInternal, anonymous, requestSource)).Observe(float64(cost.FieldCount))

			rl := conf.RateLimits()
			if !isInternal && (exceedsLimit(cost.AliasCount, rl.GraphQLMaxAliases) ||
				exceedsLimit(cost.HighestDuplicateFieldCount, rl.GraphQLMaxDuplicateFieldCount) ||
				exceedsLimit(cost.UniqueFieldCount, rl.GraphQLMaxUniqueFieldCount) ||
				exceedsLimit(cost.FieldCount, rl.GraphQLMaxFieldCount)) {
				writeViolationError(w)
				return
			}

			if rl, enabled := rlw.Get(); enabled {
				limited, result, err := rl.RateLimit(r.Context(), uid, cost.FieldCount, graphqlbackend.LimiterArgs{
					IsIP:          isIP,
					Anonymous:     anonymous,
					RequestName:   requestName,
					RequestSource: requestSource,
				})
				if err != nil {
					logger.Error("checking GraphQL rate limit", log.Error(err))
					traceData.limitError = err
				} else {
					traceData.limited = limited
					traceData.limitResult = result
					if limited {
						w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
						w.WriteHeader(http.StatusTooManyRequests)
						return nil
					}
				}
			}
		}

		traceData.execStart = time.Now()
		response := schema.Exec(r.Context(), params.Query, params.OperationName, params.Variables)
		traceData.queryErrors = response.Errors
		responseJSON, err := json.Marshal(response)
		if err != nil {
			return errors.Wrap(err, "failed to marshal GraphQL response")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(responseJSON)

		return nil
	}
}

type graphQLQueryParams struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables"`
}

type traceData struct {
	queryParams   graphQLQueryParams
	execStart     time.Time
	uid           string
	anonymous     bool
	isInternal    bool
	requestName   string
	requestSource string
	queryErrors   []*gqlerrors.QueryError

	cost      *graphqlbackend.QueryCost
	costError error

	limited     bool
	limitError  error
	limitResult throttled.RateLimitResult
}

func getUID(r *http.Request) (uid string, ip bool, anonymous bool) {
	a := actor.FromContext(r.Context())
	anonymous = !a.IsAuthenticated()
	if !anonymous {
		return a.UIDString(), false, anonymous
	}
	if uid, ok := cookie.AnonymousUID(r); ok && uid != "" {
		return uid, false, anonymous
	}
	// The user is anonymous with no cookie, use IP
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip, true, anonymous
	}
	return "unknown", false, anonymous
}

func recordAuditLog(ctx context.Context, logger log.Logger, data traceData) {
	if !audit.IsEnabled(conf.SiteConfig(), audit.GraphQL) {
		return
	}

	audit.Log(ctx, logger, audit.Record{
		Entity: "GraphQL",
		Action: "request",
		Fields: []log.Field{
			log.Object("request",
				log.String("name", data.requestName),
				log.String("source", data.requestSource),
				log.String("variables", toJson(data.queryParams.Variables)),
				log.String("query", data.queryParams.Query)),
			log.Bool("mutation", strings.Contains(data.queryParams.Query, "mutation")),
			log.Bool("successful", len(data.queryErrors) == 0),
		},
	})
}

func toJson(variables map[string]any) string {
	encoded, err := json.Marshal(variables)
	if err != nil {
		return "query variables marshalling failure"
	}
	return string(encoded)
}
