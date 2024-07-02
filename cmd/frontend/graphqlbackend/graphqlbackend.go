package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/introspection"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/graph-gophers/graphql-go/trace/otel"
	"github.com/graph-gophers/graphql-go/trace/tracer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/log"

	oteltracer "go.opentelemetry.io/otel"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cloneurls"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	sgtrace "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var graphqlFieldHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_graphql_field_seconds",
	Help:    "GraphQL field resolver latencies in seconds.",
	Buckets: []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30},
}, []string{"type", "field", "error", "source", "request_name"})

// Note: we have both pointer and value receivers on this type, and we are fine with that.
type requestTracer struct {
	logger log.Logger
	db     database.DB

	tracer *otel.Tracer
}

func newRequestTracer(logger log.Logger, db database.DB) tracer.Tracer {
	return &requestTracer{
		db: db,
		tracer: &otel.Tracer{
			Tracer: oteltracer.Tracer("GraphQL"),
		},
		logger: logger,
	}
}

func (t *requestTracer) TraceQuery(ctx context.Context, queryString string, operationName string, variables map[string]any, varTypes map[string]*introspection.Type) (context.Context, func([]*gqlerrors.QueryError)) {
	start := time.Now()

	ctx = context.WithValue(ctx, sgtrace.GraphQLQueryKey, queryString)

	var finish func([]*gqlerrors.QueryError)
	ctx, finish = t.tracer.TraceQuery(ctx, queryString, operationName, variables, varTypes)

	// Note: We don't care about the error here, we just extract the username if
	// we get a non-nil user object.
	var currentUserID int32
	a := actor.FromContext(ctx)
	if a.IsAuthenticated() {
		currentUserID = a.UID
	}

	// 🚨 SECURITY: We want to log every single operation the Sourcegraph operator
	// has done on the instance, so we need to do additional logging here. Sometimes
	// we would end up having logging twice for the same operation (here and the web
	// app), but we would not want to risk missing logging operations. Also in the
	// future, we expect audit logging of Sourcegraph operators to live outside the
	// instance, which makes this pattern less of a concern in terms of redundancy.
	if a.SourcegraphOperator {
		const eventName = "SourcegraphOperatorGraphQLRequest"
		args, err := json.Marshal(map[string]any{
			"queryString": queryString,
			"variables":   variables,
		})
		if err != nil {
			t.logger.Error(
				"failed to marshal JSON for event log argument",
				log.String("eventName", eventName),
				log.Error(err),
			)
		}

		// NOTE: It is important to propagate the correct context that carries the
		// information of the actor, especially whether the actor is a Sourcegraph
		// operator or not.
		//
		// TODO: Use EventRecorder from internal/telemetryrecorder instead.
		//lint:ignore SA1019 existing usage of deprecated functionality.
		err = usagestats.LogEvent(
			ctx,
			t.db,
			usagestats.Event{
				EventName: eventName,
				UserID:    a.UID,
				Argument:  args,
				Source:    "BACKEND",
			},
		)
		if err != nil {
			t.logger.Error(
				"failed to log event",
				log.String("eventName", eventName),
				log.Error(err),
			)
		}
	}

	// Requests made by our JS frontend and other internal things will have a concrete name attached to the
	// request which allows us to (softly) differentiate it from end-user API requests. For example,
	// /.api/graphql?Foobar where Foobar is the name of the request we make. If there is not a request name,
	// then it is an interesting query to log in the event it is harmful and a site admin needs to identify
	// it and the user issuing it.
	requestName := sgtrace.GraphQLRequestName(ctx)
	requestSource := sgtrace.RequestSource(ctx)

	return ctx, func(err []*gqlerrors.QueryError) {
		finish(err) // always non-nil

		d := time.Since(start)
		if v := conf.Get().ObservabilityLogSlowGraphQLRequests; v != 0 && d.Milliseconds() > int64(v) {
			enc, _ := json.Marshal(variables)
			t.logger.Warn(
				"slow GraphQL request",
				log.Duration("duration", d),
				log.Int32("user_id", currentUserID),
				log.String("request_name", requestName),
				log.String("source", string(requestSource)),
				log.String("variables", string(enc)),
			)
			if requestName == "unknown" {
				errFields := make([]string, 0, len(err))
				for _, e := range err {
					errFields = append(errFields, e.Error())
				}
				t.logger.Info(
					"slow unknown GraphQL request",
					log.Duration("duration", d),
					log.Int32("user_id", currentUserID),
					log.Strings("errors", errFields),
					log.String("query", queryString),
					log.String("source", string(requestSource)),
					log.String("variables", string(enc)),
				)
			}
			errFields := make([]string, 0, len(err))
			for _, e := range err {
				errFields = append(errFields, e.Error())
			}
			req := &types.SlowRequest{
				Start:     start,
				Duration:  d,
				UserID:    currentUserID,
				Name:      requestName,
				Source:    string(requestSource),
				Variables: variables,
				Errors:    errFields,
				Query:     queryString,
			}
			captureSlowRequest(t.logger, req)
		}
	}
}

func (requestTracer) TraceField(ctx context.Context, _, typeName, fieldName string, _ bool, _ map[string]any) (context.Context, func(*gqlerrors.QueryError)) {
	// We don't call into t.tracer.TraceField since it generates too many spans which is really hard to read.
	start := time.Now()
	return ctx, func(err *gqlerrors.QueryError) {
		isErrStr := strconv.FormatBool(err != nil)
		graphqlFieldHistogram.WithLabelValues(
			prometheusTypeName(typeName),
			prometheusFieldName(typeName, fieldName),
			isErrStr,
			string(sgtrace.RequestSource(ctx)),
			prometheusGraphQLRequestName(sgtrace.GraphQLRequestName(ctx)),
		).Observe(time.Since(start).Seconds())
	}
}

func (t requestTracer) TraceValidation(ctx context.Context) func([]*gqlerrors.QueryError) {
	return t.tracer.TraceValidation(ctx)
}

var allowedPrometheusFieldNames = map[[2]string]struct{}{
	{"AccessTokenConnection", "nodes"}:          {},
	{"File", "isDirectory"}:                     {},
	{"File", "name"}:                            {},
	{"File", "path"}:                            {},
	{"File", "repository"}:                      {},
	{"File", "url"}:                             {},
	{"File2", "content"}:                        {},
	{"File2", "externalURLs"}:                   {},
	{"File2", "highlight"}:                      {},
	{"File2", "isDirectory"}:                    {},
	{"File2", "richHTML"}:                       {},
	{"File2", "url"}:                            {},
	{"FileDiff", "hunks"}:                       {},
	{"FileDiff", "internalID"}:                  {},
	{"FileDiff", "mostRelevantFile"}:            {},
	{"FileDiff", "newPath"}:                     {},
	{"FileDiff", "oldPath"}:                     {},
	{"FileDiff", "stat"}:                        {},
	{"FileDiffConnection", "diffStat"}:          {},
	{"FileDiffConnection", "nodes"}:             {},
	{"FileDiffConnection", "pageInfo"}:          {},
	{"FileDiffConnection", "totalCount"}:        {},
	{"FileDiffHunk", "body"}:                    {},
	{"FileDiffHunk", "newRange"}:                {},
	{"FileDiffHunk", "oldNoNewlineAt"}:          {},
	{"FileDiffHunk", "oldRange"}:                {},
	{"FileDiffHunk", "section"}:                 {},
	{"FileDiffHunkRange", "lines"}:              {},
	{"FileDiffHunkRange", "Line"}:               {},
	{"FileMatch", "file"}:                       {},
	{"FileMatch", "limitHit"}:                   {},
	{"FileMatch", "lineMatches"}:                {},
	{"FileMatch", "repository"}:                 {},
	{"FileMatch", "revSpec"}:                    {},
	{"FileMatch", "symbols"}:                    {},
	{"GitBlob", "blame"}:                        {},
	{"GitBlob", "commit"}:                       {},
	{"GitBlob", "content"}:                      {},
	{"GitBlob", "lsif"}:                         {},
	{"GitBlob", "path"}:                         {},
	{"GitBlob", "repository"}:                   {},
	{"GitBlob", "url"}:                          {},
	{"GitCommit", "abbreviatedOID"}:             {},
	{"GitCommit", "ancestors"}:                  {},
	{"GitCommit", "author"}:                     {},
	{"GitCommit", "blob"}:                       {},
	{"GitCommit", "body"}:                       {},
	{"GitCommit", "canonicalURL"}:               {},
	{"GitCommit", "committer"}:                  {},
	{"GitCommit", "externalURLs"}:               {},
	{"GitCommit", "file"}:                       {},
	{"GitCommit", "id"}:                         {},
	{"GitCommit", "message"}:                    {},
	{"GitCommit", "oid"}:                        {},
	{"GitCommit", "parents"}:                    {},
	{"GitCommit", "repository"}:                 {},
	{"GitCommit", "subject"}:                    {},
	{"GitCommit", "symbols"}:                    {},
	{"GitCommit", "tree"}:                       {},
	{"GitCommit", "url"}:                        {},
	{"GitCommitConnection", "nodes"}:            {},
	{"GitRefConnection", "nodes"}:               {},
	{"GitTree", "canonicalURL"}:                 {},
	{"GitTree", "entries"}:                      {},
	{"GitTree", "files"}:                        {},
	{"GitTree", "isRoot"}:                       {},
	{"GitTree", "url"}:                          {},
	{"Mutation", "configurationMutation"}:       {},
	{"Mutation", "createOrganization"}:          {},
	{"Mutation", "logEvent"}:                    {},
	{"Mutation", "logUserEvent"}:                {},
	{"Query", "clientConfiguration"}:            {},
	{"Query", "currentUser"}:                    {},
	{"Query", "dotcom"}:                         {},
	{"Query", "extensionRegistry"}:              {},
	{"Query", "highlightCode"}:                  {},
	{"Query", "node"}:                           {},
	{"Query", "organization"}:                   {},
	{"Query", "repositories"}:                   {},
	{"Query", "repository"}:                     {},
	{"Query", "repositoryRedirect"}:             {},
	{"Query", "search"}:                         {},
	{"Query", "settingsSubject"}:                {},
	{"Query", "site"}:                           {},
	{"Query", "user"}:                           {},
	{"Query", "viewerConfiguration"}:            {},
	{"Query", "viewerSettings"}:                 {},
	{"RegistryExtensionConnection", "nodes"}:    {},
	{"Repository", "cloneInProgress"}:           {},
	{"Repository", "commit"}:                    {},
	{"Repository", "comparison"}:                {},
	{"Repository", "gitRefs"}:                   {},
	{"RepositoryComparison", "commits"}:         {},
	{"RepositoryComparison", "fileDiffs"}:       {},
	{"RepositoryComparison", "range"}:           {},
	{"RepositoryConnection", "nodes"}:           {},
	{"Search", "results"}:                       {},
	{"Search", "suggestions"}:                   {},
	{"SearchAlert", "description"}:              {},
	{"SearchAlert", "proposedQueries"}:          {},
	{"SearchAlert", "title"}:                    {},
	{"SearchQueryDescription", "description"}:   {},
	{"SearchQueryDescription", "query"}:         {},
	{"SearchResultMatch", "body"}:               {},
	{"SearchResultMatch", "highlights"}:         {},
	{"SearchResultMatch", "url"}:                {},
	{"SearchResults", "alert"}:                  {},
	{"SearchResults", "approximateResultCount"}: {},
	{"SearchResults", "cloning"}:                {},
	{"SearchResults", "dynamicFilters"}:         {},
	{"SearchResults", "elapsedMilliseconds"}:    {},
	{"SearchResults", "indexUnavailable"}:       {},
	{"SearchResults", "limitHit"}:               {},
	{"SearchResults", "matchCount"}:             {},
	{"SearchResults", "missing"}:                {},
	{"SearchResults", "repositoriesCount"}:      {},
	{"SearchResults", "results"}:                {},
	{"SearchResults", "timedout"}:               {},
	{"SettingsCascade", "final"}:                {},
	{"SettingsMutation", "editConfiguration"}:   {},
	{"SettingsSubject", "latestSettings"}:       {},
	{"SettingsSubject", "settingsCascade"}:      {},
	{"Signature", "date"}:                       {},
	{"Signature", "person"}:                     {},
	{"Site", "alerts"}:                          {},
	{"SymbolConnection", "nodes"}:               {},
	{"TreeEntry", "isDirectory"}:                {},
	{"TreeEntry", "isSingleChild"}:              {},
	{"TreeEntry", "name"}:                       {},
	{"TreeEntry", "path"}:                       {},
	{"TreeEntry", "submodule"}:                  {},
	{"TreeEntry", "url"}:                        {},
	{"UserConnection", "nodes"}:                 {},
}

// prometheusFieldName reduces the cardinality of GraphQL field names to make it suitable
// for use in a Prometheus metric. We only track the ones most valuable to us.
//
// See https://github.com/sourcegraph/sourcegraph/issues/9895
func prometheusFieldName(typeName, fieldName string) string {
	if _, ok := allowedPrometheusFieldNames[[2]string{typeName, fieldName}]; ok {
		return fieldName
	}
	return "other"
}

var blocklistedPrometheusTypeNames = map[string]struct{}{
	"__Type":                                 {},
	"__Schema":                               {},
	"__InputValue":                           {},
	"__Field":                                {},
	"__EnumValue":                            {},
	"__Directive":                            {},
	"UserEmail":                              {},
	"UpdateSettingsPayload":                  {},
	"ExtensionRegistryCreateExtensionResult": {},
	"Range":                                  {},
	"LineMatch":                              {},
	"DiffStat":                               {},
	"DiffHunk":                               {},
	"DiffHunkRange":                          {},
	"FileDiffResolver":                       {},
}

// prometheusTypeName reduces the cardinality of GraphQL type names to make it
// suitable for use in a Prometheus metric. This is a blocklist of type names
// which involve non-complex calculations in the GraphQL backend and thus are
// not worth tracking. You can find a complete list of the ones Prometheus is
// currently tracking via:
//
//	sum by (type)(src_graphql_field_seconds_count)
func prometheusTypeName(typeName string) string {
	if _, ok := blocklistedPrometheusTypeNames[typeName]; ok {
		return "other"
	}
	return typeName
}

// prometheusGraphQLRequestName is a allowlist of GraphQL request names (e.g. /.api/graphql?Foobar)
// to include in a Prometheus metric. Be extremely careful
func prometheusGraphQLRequestName(requestName string) string {
	if requestName == "CodeIntelSearch" {
		return requestName
	}
	return "other"
}

func NewSchemaWithoutResolvers(db database.DB) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{})
}

func NewSchemaWithGitserverClient(db database.DB, gitserverClient gitserver.Client) (*graphql.Schema, error) {
	return NewSchema(db, gitserverClient, []OptionalResolver{})
}

func NewSchemaWithNotebooksResolver(db database.DB, notebooks NotebooksResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{NotebooksResolver: notebooks}})
}

func NewSchemaWithAuthzResolver(db database.DB, authz AuthzResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{AuthzResolver: authz}})
}

func NewSchemaWithBatchChangesResolver(db database.DB, batchChanges BatchChangesResolver, githubApps GitHubAppsResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{BatchChangesResolver: batchChanges}, {GitHubAppsResolver: githubApps}})
}

func NewSchemaWithCodeMonitorsResolver(db database.DB, codeMonitors CodeMonitorsResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{CodeMonitorsResolver: codeMonitors}})
}

func NewSchemaWithLicenseResolver(db database.DB, license LicenseResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{LicenseResolver: license}})
}

func NewSchemaWithWebhooksResolver(db database.DB, webhooksResolver WebhooksResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{WebhooksResolver: webhooksResolver}})
}

func NewSchemaWithRBACResolver(db database.DB, rbacResolver RBACResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{RBACResolver: rbacResolver}})
}

func NewSchemaWithOwnResolver(db database.DB, own OwnResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{OwnResolver: own}})
}

func NewSchemaWithCompletionsResolver(db database.DB, completionsResolver CompletionsResolver) (*graphql.Schema, error) {
	return NewSchema(db, gitserver.NewClient("graphql.schemaresolver"), []OptionalResolver{{CompletionsResolver: completionsResolver}})
}

func NewSchema(
	db database.DB,
	gitserverClient gitserver.Client,
	optionals []OptionalResolver,
	graphqlOpts ...graphql.SchemaOpt,
) (*graphql.Schema, error) {
	resolver := newSchemaResolver(db, gitserverClient)
	schemas := []string{
		mainSchema,
		outboundWebhooksSchema,
		viewerSchema,
	}

	for _, optional := range optionals {
		if batchChanges := optional.BatchChangesResolver; batchChanges != nil {
			EnterpriseResolvers.batchChangesResolver = batchChanges
			resolver.BatchChangesResolver = batchChanges
			schemas = append(schemas, batchesSchema)
			// Register NodeByID handlers.
			for kind, res := range batchChanges.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if codeIntel := optional.CodeIntelResolver; codeIntel != nil {
			EnterpriseResolvers.codeIntelResolver = codeIntel
			resolver.CodeIntelResolver = codeIntel

			entires, err := codeIntelSchema.ReadDir(".")
			if err != nil {
				return nil, err
			}
			for _, entry := range entires {
				content, err := codeIntelSchema.ReadFile(entry.Name())
				if err != nil {
					return nil, err
				}

				schemas = append(schemas, string(content))
			}

			// Register NodeByID handlers.
			for kind, res := range codeIntel.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if insights := optional.InsightsResolver; insights != nil {
			EnterpriseResolvers.insightsResolver = insights
			resolver.InsightsResolver = insights
			schemas = append(schemas, insightsSchema)
		}

		if authz := optional.AuthzResolver; authz != nil {
			EnterpriseResolvers.authzResolver = authz
			resolver.AuthzResolver = authz
			schemas = append(schemas, authzSchema)
		}

		if codeMonitors := optional.CodeMonitorsResolver; codeMonitors != nil {
			EnterpriseResolvers.codeMonitorsResolver = codeMonitors
			resolver.CodeMonitorsResolver = codeMonitors
			schemas = append(schemas, codeMonitorsSchema)
			// Register NodeByID handlers.
			for kind, res := range codeMonitors.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if gitHubApps := optional.GitHubAppsResolver; gitHubApps != nil {
			EnterpriseResolvers.gitHubAppsResolver = gitHubApps
			resolver.GitHubAppsResolver = gitHubApps
			schemas = append(schemas, gitHubAppsSchema)
			for kind, res := range gitHubApps.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if license := optional.LicenseResolver; license != nil {
			EnterpriseResolvers.licenseResolver = license
			resolver.LicenseResolver = license
			schemas = append(schemas, licenseSchema)
			// No NodeByID handlers currently.
		}

		if dotcom := optional.DotcomRootResolver; dotcom != nil {
			EnterpriseResolvers.dotcomResolver = dotcom
			resolver.DotcomRootResolver = dotcom
			schemas = append(schemas, dotcomSchema)
			// Register NodeByID handlers.
			for kind, res := range dotcom.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if searchContexts := optional.SearchContextsResolver; searchContexts != nil {
			EnterpriseResolvers.searchContextsResolver = searchContexts
			resolver.SearchContextsResolver = searchContexts
			schemas = append(schemas, searchContextsSchema)
			// Register NodeByID handlers.
			for kind, res := range searchContexts.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if notebooks := optional.NotebooksResolver; notebooks != nil {
			EnterpriseResolvers.notebooksResolver = notebooks
			resolver.NotebooksResolver = notebooks
			schemas = append(schemas, notebooksSchema)
			// Register NodeByID handlers.
			for kind, res := range notebooks.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if compute := optional.ComputeResolver; compute != nil {
			EnterpriseResolvers.computeResolver = compute
			resolver.ComputeResolver = compute
			schemas = append(schemas, computeSchema)
		}

		if insightsAggregation := optional.InsightsAggregationResolver; insightsAggregation != nil {
			EnterpriseResolvers.insightsAggregationResolver = insightsAggregation
			resolver.InsightsAggregationResolver = insightsAggregation
			schemas = append(schemas, insightsAggregationsSchema)
		}

		if webhooksResolver := optional.WebhooksResolver; webhooksResolver != nil {
			EnterpriseResolvers.webhooksResolver = webhooksResolver
			resolver.WebhooksResolver = webhooksResolver
			// Register NodeByID handlers.
			for kind, res := range webhooksResolver.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if embeddingsResolver := optional.EmbeddingsResolver; embeddingsResolver != nil {
			EnterpriseResolvers.embeddingsResolver = embeddingsResolver
			resolver.EmbeddingsResolver = embeddingsResolver
			schemas = append(schemas, embeddingsSchema)
		}

		if contextResolver := optional.CodyContextResolver; contextResolver != nil {
			EnterpriseResolvers.contextResolver = contextResolver
			resolver.CodyContextResolver = contextResolver
			schemas = append(schemas, codyContextSchema)
		}

		if rbacResolver := optional.RBACResolver; rbacResolver != nil {
			EnterpriseResolvers.rbacResolver = rbacResolver
			resolver.RBACResolver = rbacResolver
			schemas = append(schemas, rbacSchema)
		}

		if ownResolver := optional.OwnResolver; ownResolver != nil {
			EnterpriseResolvers.ownResolver = ownResolver
			resolver.OwnResolver = ownResolver
			schemas = append(schemas, ownSchema)
			// Register NodeByID handlers.
			for kind, res := range ownResolver.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if completionsResolver := optional.CompletionsResolver; completionsResolver != nil {
			EnterpriseResolvers.completionsResolver = completionsResolver
			resolver.CompletionsResolver = completionsResolver
			schemas = append(schemas, completionSchema)
		}

		if guardrailsResolver := optional.GuardrailsResolver; guardrailsResolver != nil {
			EnterpriseResolvers.guardrailsResolver = guardrailsResolver
			resolver.GuardrailsResolver = guardrailsResolver
			schemas = append(schemas, guardrailsSchema)
		}

		if contentLibraryResolver := optional.ContentLibraryResolver; contentLibraryResolver != nil {
			EnterpriseResolvers.contentLibraryResolver = contentLibraryResolver
			resolver.ContentLibraryResolver = contentLibraryResolver
			schemas = append(schemas, contentLibrary)
		}

		if searchJobsResolver := optional.SearchJobsResolver; searchJobsResolver != nil {
			EnterpriseResolvers.searchJobsResolver = searchJobsResolver
			resolver.SearchJobsResolver = searchJobsResolver
			schemas = append(schemas, searchJobSchema)
			// Register NodeByID handlers.
			for kind, res := range searchJobsResolver.NodeResolvers() {
				resolver.nodeByIDFns[kind] = res
			}
		}

		if telemetryResolver := optional.TelemetryRootResolver; telemetryResolver != nil {
			EnterpriseResolvers.telemetryResolver = telemetryResolver
			resolver.TelemetryRootResolver = telemetryResolver
			schemas = append(schemas, telemetrySchema)
		}
	}

	opts := []graphql.SchemaOpt{
		graphql.Tracer(newRequestTracer(log.Scoped("GraphQL"), db)),
		graphql.UseStringDescriptions(),
		graphql.MaxDepth(conf.RateLimits().GraphQLMaxDepth),
	}
	opts = append(opts, graphqlOpts...)
	return graphql.ParseSchema(
		strings.Join(schemas, "\n"),
		resolver,
		opts...)
}

// schemaResolver handles all GraphQL queries for Sourcegraph. To do this, it
// uses subresolvers which are globals. Enterprise-only resolvers are assigned
// to a field of EnterpriseResolvers.
//
// schemaResolver must be instantiated using newSchemaResolver.
type schemaResolver struct {
	logger            log.Logger
	db                database.DB
	gitserverClient   gitserver.Client
	repoupdaterClient *repoupdater.Client
	nodeByIDFns       map[string]NodeByIDFunc

	OptionalResolver
}

// OptionalResolver are the resolvers that do not have to be set. If a field
// is non-nil, NewSchema will register the corresponding graphql schema.
type OptionalResolver struct {
	AuthzResolver
	BatchChangesResolver
	CodeIntelResolver
	CodeMonitorsResolver
	CompletionsResolver
	ComputeResolver
	CodyContextResolver
	DotcomRootResolver
	EmbeddingsResolver
	SearchJobsResolver
	GitHubAppsResolver
	GuardrailsResolver
	InsightsAggregationResolver
	InsightsResolver
	LicenseResolver
	NotebooksResolver
	OwnResolver
	RBACResolver
	SearchContextsResolver
	WebhooksResolver
	ContentLibraryResolver
	*TelemetryRootResolver
}

// newSchemaResolver will return a new, safely instantiated schemaResolver with some
// defaults. It does not implement any sub-resolvers.
func newSchemaResolver(db database.DB, gitserverClient gitserver.Client) *schemaResolver {
	r := &schemaResolver{
		logger:            log.Scoped("schemaResolver"),
		db:                db,
		gitserverClient:   gitserverClient,
		repoupdaterClient: repoupdater.DefaultClient,
	}

	r.nodeByIDFns = map[string]NodeByIDFunc{
		"AccessRequest": func(ctx context.Context, id graphql.ID) (Node, error) {
			return accessRequestByID(ctx, db, id)
		},
		"AccessToken": func(ctx context.Context, id graphql.ID) (Node, error) {
			return accessTokenByID(ctx, db, id)
		},
		"ExternalAccount": func(ctx context.Context, id graphql.ID) (Node, error) {
			return externalAccountByID(ctx, db, id)
		},
		externalServiceIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return externalServiceByID(ctx, db, id)
		},
		"GitRef": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.gitRefByID(ctx, id)
		},
		"Repository": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.repositoryByID(ctx, id)
		},
		"User": func(ctx context.Context, id graphql.ID) (Node, error) {
			return UserByID(ctx, db, id)
		},
		"Org": func(ctx context.Context, id graphql.ID) (Node, error) {
			return OrgByID(ctx, db, id)
		},
		"OrganizationInvitation": func(ctx context.Context, id graphql.ID) (Node, error) {
			return orgInvitationByID(ctx, db, id)
		},
		"GitCommit": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.gitCommitByID(ctx, id)
		},
		"SavedSearch": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.savedSearchByID(ctx, id)
		},
		"Site": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.siteByGQLID(ctx, id)
		},
		"OutOfBandMigration": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.OutOfBandMigrationByID(ctx, id)
		},
		"WebhookLog": func(ctx context.Context, id graphql.ID) (Node, error) {
			return webhookLogByID(ctx, db, id)
		},
		"OutboundRequest": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.outboundRequestByID(ctx, id)
		},
		"BackgroundJob": func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.backgroundJobByID(ctx, id)
		},
		"Executor": func(ctx context.Context, id graphql.ID) (Node, error) {
			return executorByID(ctx, db, id)
		},
		"ExternalServiceSyncJob": func(ctx context.Context, id graphql.ID) (Node, error) {
			return externalServiceSyncJobByID(ctx, db, id)
		},
		"ExecutorSecret": func(ctx context.Context, id graphql.ID) (Node, error) {
			return executorSecretByID(ctx, db, id)
		},
		"ExecutorSecretAccessLog": func(ctx context.Context, id graphql.ID) (Node, error) {
			return executorSecretAccessLogByID(ctx, db, id)
		},
		teamIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return teamByID(ctx, db, id)
		},
		outboundWebhookIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return OutboundWebhookByID(ctx, db, id)
		},
		roleIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.roleByID(ctx, id)
		},
		permissionIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.permissionByID(ctx, id)
		},
		CodeHostKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return CodeHostByID(ctx, r.db, id)
		},
		gitserverIDKind: func(ctx context.Context, id graphql.ID) (Node, error) {
			return r.gitserverByID(ctx, id)
		},
		indexedSearchInstanceIDKind: func(ctx context.Context, id graphql.ID) (Node, error) { return r.indexedSearchInstanceByID(ctx, id) },
	}
	return r
}

// EnterpriseResolvers holds the instances of resolvers which are enabled only
// in enterprise mode. These resolver instances are nil when running as OSS.
var EnterpriseResolvers = struct {
	authzResolver               AuthzResolver
	batchChangesResolver        BatchChangesResolver
	codeIntelResolver           CodeIntelResolver
	codeMonitorsResolver        CodeMonitorsResolver
	completionsResolver         CompletionsResolver
	computeResolver             ComputeResolver
	contextResolver             CodyContextResolver
	dotcomResolver              DotcomRootResolver
	embeddingsResolver          EmbeddingsResolver
	searchJobsResolver          SearchJobsResolver
	gitHubAppsResolver          GitHubAppsResolver
	guardrailsResolver          GuardrailsResolver
	insightsAggregationResolver InsightsAggregationResolver
	insightsResolver            InsightsResolver
	licenseResolver             LicenseResolver
	notebooksResolver           NotebooksResolver
	ownResolver                 OwnResolver
	rbacResolver                RBACResolver
	searchContextsResolver      SearchContextsResolver
	webhooksResolver            WebhooksResolver
	contentLibraryResolver      ContentLibraryResolver
	telemetryResolver           *TelemetryRootResolver
}{}

// Root returns a new schemaResolver.
//
// DEPRECATED
func (r *schemaResolver) Root() *schemaResolver {
	return newSchemaResolver(r.db, r.gitserverClient)
}

func (r *schemaResolver) Repository(ctx context.Context, args *struct {
	Name     *string
	CloneURL *string
	// TODO(chris): Remove URI in favor of Name.
	URI *string
},
) (*RepositoryResolver, error) {
	// Deprecated query by "URI"
	if args.URI != nil && args.Name == nil {
		args.Name = args.URI
	}
	resolver, err := r.RepositoryRedirect(ctx, &repositoryRedirectArgs{args.Name, args.CloneURL, nil})
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, nil
	}
	return resolver.repo, nil
}

// RecloneRepository deletes a repository from the gitserver disk and marks it as not cloned
// in the database, and then starts a repo clone.
func (r *schemaResolver) RecloneRepository(ctx context.Context, args *struct {
	Repo graphql.ID
},
) (*EmptyResponse, error) {
	// 🚨 SECURITY: Only site admins can reclone repositories.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repo, err := r.repositoryByID(ctx, args.Repo)
	if err != nil {
		return nil, err
	}

	if _, err := r.DeleteRepositoryFromDisk(ctx, args); err != nil {
		return &EmptyResponse{}, errors.Wrap(err, fmt.Sprintf("could not delete repository with ID %d", repo.repo.ID))
	}

	return &EmptyResponse{}, nil
}

// DeleteRepositoryFromDisk deletes a repository from the gitserver disk and marks it as not cloned
// in the database.
func (r *schemaResolver) DeleteRepositoryFromDisk(ctx context.Context, args *struct {
	Repo graphql.ID
},
) (*EmptyResponse, error) {
	var repoID api.RepoID
	if err := relay.UnmarshalSpec(args.Repo, &repoID); err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Only site admins can delete repositories from disk.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repo, err := r.db.GitserverRepos().GetByID(ctx, repoID)
	if err != nil {
		return &EmptyResponse{}, errors.Wrap(err, fmt.Sprintf("error while fetching repository with ID %d", repoID))
	}

	if repo.CloneStatus == types.CloneStatusCloning {
		return &EmptyResponse{}, errors.Wrap(err, fmt.Sprintf("cannot delete repository %d: busy cloning", repo.RepoID))
	}

	if err := backend.NewRepos(r.logger, r.db, r.gitserverClient).RecloneRepository(ctx, repoID); err != nil {
		return &EmptyResponse{}, errors.Wrap(err, fmt.Sprintf("error while deleting repository with ID %d", repoID))
	}

	return &EmptyResponse{}, nil
}

func (r *schemaResolver) repositoryByID(ctx context.Context, id graphql.ID) (*RepositoryResolver, error) {
	var repoID api.RepoID
	if err := relay.UnmarshalSpec(id, &repoID); err != nil {
		return nil, err
	}
	repo, err := r.db.Repos().Get(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return NewRepositoryResolver(r.db, r.gitserverClient, repo), nil
}

type RedirectResolver struct {
	url string
}

func (r *RedirectResolver) URL() string {
	return r.url
}

type repositoryRedirect struct {
	repo     *RepositoryResolver
	redirect *RedirectResolver
}

type repositoryRedirectArgs struct {
	Name       *string
	CloneURL   *string
	HashedName *string
}

func (r *repositoryRedirect) ToRepository() (*RepositoryResolver, bool) {
	return r.repo, r.repo != nil
}

func (r *repositoryRedirect) ToRedirect() (*RedirectResolver, bool) {
	return r.redirect, r.redirect != nil
}

func (r *schemaResolver) RepositoryRedirect(ctx context.Context, args *repositoryRedirectArgs) (*repositoryRedirect, error) {
	if args.HashedName != nil {
		// Query by repository hashed name
		repo, err := r.db.Repos().GetByHashedName(ctx, api.RepoHashedName(*args.HashedName))
		if err != nil {
			return nil, err
		}
		return &repositoryRedirect{repo: NewRepositoryResolver(r.db, r.gitserverClient, repo)}, nil
	}
	var name api.RepoName
	if args.Name != nil {
		// Query by name
		name = api.RepoName(*args.Name)
	} else if args.CloneURL != nil {
		// Query by git clone URL
		var err error
		name, err = cloneurls.RepoSourceCloneURLToRepoName(ctx, r.db, *args.CloneURL)
		if err != nil {
			return nil, err
		}
		if name == "" {
			// Clone URL could not be mapped to a code host
			return nil, nil
		}
	} else {
		return nil, errors.New("neither name nor cloneURL given")
	}

	repo, err := backend.NewRepos(r.logger, r.db, r.gitserverClient).GetByName(ctx, name)
	if err != nil {
		var e backend.ErrRepoSeeOther
		if errors.As(err, &e) {
			return &repositoryRedirect{redirect: &RedirectResolver{url: e.RedirectURL}}, nil
		}
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &repositoryRedirect{repo: NewRepositoryResolver(r.db, r.gitserverClient, repo)}, nil
}

func (r *schemaResolver) PhabricatorRepo(ctx context.Context, args *struct {
	Name *string
	// TODO(chris): Remove URI in favor of Name.
	URI *string
},
) (*phabricatorRepoResolver, error) {
	if args.Name != nil {
		args.URI = args.Name
	}

	repo, err := r.db.Phabricator().GetByName(ctx, api.RepoName(*args.URI))
	if err != nil {
		return nil, err
	}
	return &phabricatorRepoResolver{repo}, nil
}

func (r *schemaResolver) CurrentUser(ctx context.Context) (*UserResolver, error) {
	return CurrentUser(ctx, r.db)
}

// CodeHostSyncDue returns true if any of the supplied code hosts are due to sync
// now or within "seconds" from now.
func (r *schemaResolver) CodeHostSyncDue(ctx context.Context, args *struct {
	IDs     []graphql.ID
	Seconds int32
},
) (bool, error) {
	if len(args.IDs) == 0 {
		return false, errors.New("no ids supplied")
	}
	ids := make([]int64, len(args.IDs))
	for i, gqlID := range args.IDs {
		id, err := UnmarshalExternalServiceID(gqlID)
		if err != nil {
			return false, errors.New("unable to unmarshal id")
		}
		ids[i] = id
	}
	return r.db.ExternalServices().SyncDue(ctx, ids, time.Duration(args.Seconds)*time.Second)
}
