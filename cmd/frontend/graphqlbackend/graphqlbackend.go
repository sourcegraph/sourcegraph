package graphqlbackend

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/graph-gophers/graphql-go"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/introspection"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/graph-gophers/graphql-go/trace"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cloneurls"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	sgtrace "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var (
	graphqlFieldHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_graphql_field_seconds",
		Help:    "GraphQL field resolver latencies in seconds.",
		Buckets: []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30},
	}, []string{"type", "field", "error", "source", "request_name"})

	codeIntelSearchHistogram = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_graphql_code_intel_search_seconds",
		Help:    "Code intel search latencies in seconds.",
		Buckets: []float64{0.01, 0.02, 0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10, 30},
	}, []string{"exact", "error"})

	cf = httpcli.NewExternalHTTPClientFactory()
)

type prometheusTracer struct {
	db dbutil.DB
	trace.OpenTracingTracer
}

func (t *prometheusTracer) TraceQuery(ctx context.Context, queryString string, operationName string, variables map[string]interface{}, varTypes map[string]*introspection.Type) (context.Context, trace.TraceQueryFinishFunc) {
	start := time.Now()
	var finish trace.TraceQueryFinishFunc
	if ot.ShouldTrace(ctx) {
		ctx, finish = trace.OpenTracingTracer{}.TraceQuery(ctx, queryString, operationName, variables, varTypes)
	}

	ctx = context.WithValue(ctx, "graphql-query", queryString)

	_, disableLog := os.LookupEnv("NO_GRAPHQL_LOG")

	// Note: We don't care about the error here, we just extract the username if
	// we get a non-nil user object.
	currentUser, _ := CurrentUser(ctx, t.db)
	var currentUserName string
	if currentUser != nil {
		currentUserName = currentUser.Username()
	}

	// Requests made by our JS frontend and other internal things will have a concrete name attached to the
	// request which allows us to (softly) differentiate it from end-user API requests. For example,
	// /.api/graphql?Foobar where Foobar is the name of the request we make. If there is not a request name,
	// then it is an interesting query to log in the event it is harmful and a site admin needs to identify
	// it and the user issuing it.
	requestName := sgtrace.GraphQLRequestName(ctx)
	lvl := log15.Debug
	if requestName == "unknown" {
		lvl = log15.Info
	}
	requestSource := sgtrace.RequestSource(ctx)

	if !disableLog {
		lvl("serving GraphQL request", "name", requestName, "user", currentUserName, "source", requestSource)
		if requestName == "unknown" {
			log.Printf(`logging complete query for unnamed GraphQL request above name=%s user=%s source=%s:
QUERY
-----
%s

VARIABLES
---------
%v

`, requestName, currentUserName, requestSource, queryString, variables)
		}
	}

	return ctx, func(err []*gqlerrors.QueryError) {
		if finish != nil {
			finish(err)
		}
		d := time.Since(start)
		if v := conf.Get().ObservabilityLogSlowGraphQLRequests; v != 0 && d.Milliseconds() > int64(v) {
			encodedVariables, _ := json.Marshal(variables)
			log15.Warn("slow GraphQL request", "time", d, "name", requestName, "user", currentUserName, "source", requestSource, "error", err, "variables", string(encodedVariables))
			if requestName == "unknown" {
				log.Printf(`logging complete query for slow GraphQL request above time=%v name=%s user=%s source=%s error=%v:
QUERY
-----
%s

VARIABLES
---------
%s

`, d, requestName, currentUserName, requestSource, err, queryString, encodedVariables)
			}
		}
	}
}

func (prometheusTracer) TraceField(ctx context.Context, label, typeName, fieldName string, trivial bool, args map[string]interface{}) (context.Context, trace.TraceFieldFinishFunc) {
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

		origin := sgtrace.RequestOrigin(ctx)
		if origin != "unknown" && (fieldName == "search" || fieldName == "lsif") {
			isExact := strconv.FormatBool(fieldName == "lsif")
			codeIntelSearchHistogram.WithLabelValues(isExact, isErrStr).Observe(time.Since(start).Seconds())
		}
	}
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
// 	sum by (type)(src_graphql_field_seconds_count)
//
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

func NewSchema(db dbutil.DB, batchChanges BatchChangesResolver, codeIntel CodeIntelResolver, insights InsightsResolver, authz AuthzResolver, codeMonitors CodeMonitorsResolver, license LicenseResolver) (*graphql.Schema, error) {
	resolver := &schemaResolver{
		db: db,

		BatchChangesResolver: defaultBatchChangesResolver{},
		AuthzResolver:        defaultAuthzResolver{},
		CodeIntelResolver:    defaultCodeIntelResolver{},
		InsightsResolver:     defaultInsightsResolver{},
		LicenseResolver:      defaultLicenseResolver{},
	}
	if batchChanges != nil {
		EnterpriseResolvers.batchChangesResolver = batchChanges
		resolver.BatchChangesResolver = batchChanges
	}
	if codeIntel != nil {
		EnterpriseResolvers.codeIntelResolver = codeIntel
		resolver.CodeIntelResolver = codeIntel
	}
	if insights != nil {
		EnterpriseResolvers.insightsResolver = insights
		resolver.InsightsResolver = insights
	}
	if authz != nil {
		EnterpriseResolvers.authzResolver = authz
		resolver.AuthzResolver = authz
	}
	if codeMonitors != nil {
		EnterpriseResolvers.codeMonitorsResolver = codeMonitors
		resolver.CodeMonitorsResolver = codeMonitors
	}
	if license != nil {
		EnterpriseResolvers.licenseResolver = license
		resolver.LicenseResolver = license
	}
	return graphql.ParseSchema(
		Schema,
		resolver,
		graphql.Tracer(&prometheusTracer{db: db}),
		graphql.UseStringDescriptions(),
	)
}

// EmptyResponse is a type that can be used in the return signature for graphql queries
// that don't require a return value.
type EmptyResponse struct{}

// AlwaysNil exists since various graphql tools expect at least one field to be
// present in the schema so we provide a dummy one here that is always nil.
func (er *EmptyResponse) AlwaysNil() *string {
	return nil
}

type Node interface {
	ID() graphql.ID
}

type NodeResolver struct {
	Node
}

func (r *NodeResolver) ToAccessToken() (*accessTokenResolver, bool) {
	n, ok := r.Node.(*accessTokenResolver)
	return n, ok
}

func (r *NodeResolver) ToMonitor() (MonitorResolver, bool) {
	n, ok := r.Node.(MonitorResolver)
	return n, ok
}

func (r *NodeResolver) ToMonitorQuery() (MonitorQueryResolver, bool) {
	n, ok := r.Node.(MonitorQueryResolver)
	return n, ok
}

func (r *NodeResolver) ToMonitorEmail() (MonitorEmailResolver, bool) {
	n, ok := r.Node.(MonitorEmailResolver)
	return n, ok
}

func (r *NodeResolver) ToMonitorActionEvent() (MonitorActionEventResolver, bool) {
	n, ok := r.Node.(MonitorActionEventResolver)
	return n, ok
}

func (r *NodeResolver) ToMonitorTriggerEvent() (MonitorTriggerEventResolver, bool) {
	n, ok := r.Node.(MonitorTriggerEventResolver)
	return n, ok
}

// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
func (r *NodeResolver) ToCampaign() (BatchChangeResolver, bool) {
	if n, ok := r.Node.(BatchChangeResolver); ok {
		return n, n.ActAsCampaign()
	}
	return nil, false
}

// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
func (r *NodeResolver) ToCampaignSpec() (BatchSpecResolver, bool) {
	if n, ok := r.Node.(BatchSpecResolver); ok {
		return n, n.ActAsCampaignSpec()
	}
	return nil, false
}

func (r *NodeResolver) ToBatchChange() (BatchChangeResolver, bool) {
	if n, ok := r.Node.(BatchChangeResolver); ok {
		// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
		return n, !n.ActAsCampaign()
	}
	return nil, false
}

func (r *NodeResolver) ToBatchSpec() (BatchSpecResolver, bool) {
	if n, ok := r.Node.(BatchSpecResolver); ok {
		// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
		return n, !n.ActAsCampaignSpec()
	}
	return nil, false
}

func (r *NodeResolver) ToExternalChangeset() (ExternalChangesetResolver, bool) {
	n, ok := r.Node.(ChangesetResolver)
	if !ok {
		return nil, false
	}
	return n.ToExternalChangeset()
}

func (r *NodeResolver) ToHiddenExternalChangeset() (HiddenExternalChangesetResolver, bool) {
	n, ok := r.Node.(ChangesetResolver)
	if !ok {
		return nil, false
	}
	return n.ToHiddenExternalChangeset()
}

func (r *NodeResolver) ToChangesetEvent() (ChangesetEventResolver, bool) {
	n, ok := r.Node.(ChangesetEventResolver)
	return n, ok
}

func (r *NodeResolver) ToHiddenChangesetSpec() (HiddenChangesetSpecResolver, bool) {
	n, ok := r.Node.(ChangesetSpecResolver)
	if !ok {
		return nil, ok
	}
	return n.ToHiddenChangesetSpec()
}

func (r *NodeResolver) ToVisibleChangesetSpec() (VisibleChangesetSpecResolver, bool) {
	n, ok := r.Node.(ChangesetSpecResolver)
	if !ok {
		return nil, ok
	}
	return n.ToVisibleChangesetSpec()
}

// TODO(campaigns-deprecation): This should be removed once we remove campaigns completely
func (r *NodeResolver) ToCampaignsCredential() (CampaignsCredentialResolver, bool) {
	n, ok := r.Node.(CampaignsCredentialResolver)
	return n, ok
}

func (r *NodeResolver) ToBatchChangesCredential() (BatchChangesCredentialResolver, bool) {
	n, ok := r.Node.(BatchChangesCredentialResolver)
	return n, ok
}

func (r *NodeResolver) ToProductLicense() (ProductLicense, bool) {
	n, ok := r.Node.(ProductLicense)
	return n, ok
}

func (r *NodeResolver) ToProductSubscription() (ProductSubscription, bool) {
	n, ok := r.Node.(ProductSubscription)
	return n, ok
}

func (r *NodeResolver) ToExternalAccount() (*externalAccountResolver, bool) {
	n, ok := r.Node.(*externalAccountResolver)
	return n, ok
}

func (r *NodeResolver) ToExternalService() (*externalServiceResolver, bool) {
	n, ok := r.Node.(*externalServiceResolver)
	return n, ok
}

func (r *NodeResolver) ToGitRef() (*GitRefResolver, bool) {
	n, ok := r.Node.(*GitRefResolver)
	return n, ok
}

func (r *NodeResolver) ToRepository() (*RepositoryResolver, bool) {
	n, ok := r.Node.(*RepositoryResolver)
	return n, ok
}

func (r *NodeResolver) ToUser() (*UserResolver, bool) {
	n, ok := r.Node.(*UserResolver)
	return n, ok
}

func (r *NodeResolver) ToOrg() (*OrgResolver, bool) {
	n, ok := r.Node.(*OrgResolver)
	return n, ok
}

func (r *NodeResolver) ToOrganizationInvitation() (*organizationInvitationResolver, bool) {
	n, ok := r.Node.(*organizationInvitationResolver)
	return n, ok
}

func (r *NodeResolver) ToGitCommit() (*GitCommitResolver, bool) {
	n, ok := r.Node.(*GitCommitResolver)
	return n, ok
}

func (r *NodeResolver) ToRegistryExtension() (RegistryExtension, bool) {
	if NodeToRegistryExtension == nil {
		return nil, false
	}
	return NodeToRegistryExtension(r.Node)
}

func (r *NodeResolver) ToSavedSearch() (*savedSearchResolver, bool) {
	n, ok := r.Node.(*savedSearchResolver)
	return n, ok
}

func (r *NodeResolver) ToSearchContext() (*searchContextResolver, bool) {
	n, ok := r.Node.(*searchContextResolver)
	return n, ok
}

func (r *NodeResolver) ToSite() (*siteResolver, bool) {
	n, ok := r.Node.(*siteResolver)
	return n, ok
}

func (r *NodeResolver) ToLSIFUpload() (LSIFUploadResolver, bool) {
	n, ok := r.Node.(LSIFUploadResolver)
	return n, ok
}

func (r *NodeResolver) ToLSIFIndex() (LSIFIndexResolver, bool) {
	n, ok := r.Node.(LSIFIndexResolver)
	return n, ok
}

func (r *NodeResolver) ToOutOfBandMigration() (*outOfBandMigrationResolver, bool) {
	n, ok := r.Node.(*outOfBandMigrationResolver)
	return n, ok
}

// schemaResolver handles all GraphQL queries for Sourcegraph. To do this, it
// uses subresolvers which are globals. Enterprise-only resolvers are assigned
// to a field of EnterpriseResolvers.
type schemaResolver struct {
	BatchChangesResolver
	AuthzResolver
	CodeIntelResolver
	InsightsResolver
	CodeMonitorsResolver
	LicenseResolver

	db dbutil.DB
}

// EnterpriseResolvers holds the instances of resolvers which are enabled only
// in enterprise mode. These resolver instances are nil when running as OSS.
var EnterpriseResolvers = struct {
	codeIntelResolver    CodeIntelResolver
	insightsResolver     InsightsResolver
	authzResolver        AuthzResolver
	batchChangesResolver BatchChangesResolver
	codeMonitorsResolver CodeMonitorsResolver
	licenseResolver      LicenseResolver
}{
	codeIntelResolver:    defaultCodeIntelResolver{},
	authzResolver:        defaultAuthzResolver{},
	batchChangesResolver: defaultBatchChangesResolver{},
	codeMonitorsResolver: defaultCodeMonitorsResolver{},
	licenseResolver:      defaultLicenseResolver{},
}

// DEPRECATED
func (r *schemaResolver) Root() *schemaResolver {
	return &schemaResolver{db: r.db}
}

func (r *schemaResolver) Node(ctx context.Context, args *struct{ ID graphql.ID }) (*NodeResolver, error) {
	n, err := r.nodeByID(ctx, args.ID)
	if err != nil {
		return nil, err
	}
	if n == nil {
		return nil, nil
	}
	return &NodeResolver{n}, nil
}

func (r *schemaResolver) nodeByID(ctx context.Context, id graphql.ID) (Node, error) {
	switch relay.UnmarshalKind(id) {
	case "AccessToken":
		return accessTokenByID(ctx, r.db, id)
	case "Campaign":
		return r.CampaignByID(ctx, id)
	case "BatchChange":
		return r.BatchChangeByID(ctx, id)
	case "CampaignSpec":
		return r.CampaignSpecByID(ctx, id)
	case "BatchSpec":
		return r.BatchSpecByID(ctx, id)
	case "ChangesetSpec":
		return r.ChangesetSpecByID(ctx, id)
	case "Changeset":
		return r.ChangesetByID(ctx, id)
	case "CampaignsCredential":
		return r.CampaignsCredentialByID(ctx, id)
	case "BatchChangesCredential":
		return r.BatchChangesCredentialByID(ctx, id)
	case "ProductLicense":
		if f := ProductLicenseByID; f != nil {
			return f(ctx, r.db, id)
		}
		return nil, errors.New("not implemented")
	case "ProductSubscription":
		if f := ProductSubscriptionByID; f != nil {
			return f(ctx, r.db, id)
		}
		return nil, errors.New("not implemented")
	case "ExternalAccount":
		return externalAccountByID(ctx, r.db, id)
	case externalServiceIDKind:
		return externalServiceByID(ctx, r.db, id)
	case "GitRef":
		return r.gitRefByID(ctx, id)
	case "Repository":
		return r.repositoryByID(ctx, id)
	case "User":
		return UserByID(ctx, r.db, id)
	case "Org":
		return OrgByID(ctx, r.db, id)
	case "OrganizationInvitation":
		return orgInvitationByID(ctx, r.db, id)
	case "GitCommit":
		return r.gitCommitByID(ctx, id)
	case "RegistryExtension":
		return RegistryExtensionByID(ctx, r.db, id)
	case "SavedSearch":
		return r.savedSearchByID(ctx, id)
	case "Site":
		return r.siteByGQLID(ctx, id)
	case "LSIFUpload":
		return r.LSIFUploadByID(ctx, id)
	case "LSIFIndex":
		return r.LSIFIndexByID(ctx, id)
	case "CodeMonitor":
		return r.MonitorByID(ctx, id)
	case "OutOfBandMigration":
		return r.OutOfBandMigrationByID(ctx, id)
	default:
		return nil, errors.New("invalid id")
	}
}

func (r *schemaResolver) Repository(ctx context.Context, args *struct {
	Name     *string
	CloneURL *string
	// TODO(chris): Remove URI in favor of Name.
	URI *string
}) (*RepositoryResolver, error) {
	// Deprecated query by "URI"
	if args.URI != nil && args.Name == nil {
		args.Name = args.URI
	}
	resolver, err := r.RepositoryRedirect(ctx, &struct {
		Name     *string
		CloneURL *string
	}{args.Name, args.CloneURL})
	if err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, nil
	}
	return resolver.repo, nil
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

func (r *repositoryRedirect) ToRepository() (*RepositoryResolver, bool) {
	return r.repo, r.repo != nil
}

func (r *repositoryRedirect) ToRedirect() (*RedirectResolver, bool) {
	return r.redirect, r.redirect != nil
}

func (r *schemaResolver) RepositoryRedirect(ctx context.Context, args *struct {
	Name     *string
	CloneURL *string
}) (*repositoryRedirect, error) {
	var name api.RepoName
	if args.Name != nil {
		// Query by name
		name = api.RepoName(*args.Name)
	} else if args.CloneURL != nil {
		// Query by git clone URL
		var err error
		name, err = cloneurls.ReposourceCloneURLToRepoName(ctx, *args.CloneURL)
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

	repo, err := backend.Repos.GetByName(ctx, name)
	if err != nil {
		if err, ok := err.(backend.ErrRepoSeeOther); ok {
			return &repositoryRedirect{redirect: &RedirectResolver{url: err.RedirectURL}}, nil
		}
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &repositoryRedirect{repo: NewRepositoryResolver(r.db, repo)}, nil
}

func (r *schemaResolver) PhabricatorRepo(ctx context.Context, args *struct {
	Name *string
	// TODO(chris): Remove URI in favor of Name.
	URI *string
}) (*phabricatorRepoResolver, error) {
	if args.Name != nil {
		args.URI = args.Name
	}

	repo, err := database.Phabricator(r.db).GetByName(ctx, api.RepoName(*args.URI))
	if err != nil {
		return nil, err
	}
	return &phabricatorRepoResolver{repo}, nil
}

func (r *schemaResolver) CurrentUser(ctx context.Context) (*UserResolver, error) {
	return CurrentUser(ctx, r.db)
}

func (r *schemaResolver) AffiliatedRepositories(ctx context.Context, args *struct {
	User     graphql.ID
	CodeHost *graphql.ID
	Query    *string
}) (*codeHostRepositoryConnectionResolver, error) {
	userID, err := UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}
	// 🚨 SECURITY: make sure the user is either site admin or the same user being requested
	if err := backend.CheckSiteAdminOrSameUser(ctx, userID); err != nil {
		return nil, err
	}
	var codeHost int64
	if args.CodeHost != nil {
		codeHost, err = unmarshalExternalServiceID(*args.CodeHost)
		if err != nil {
			return nil, err
		}
	}
	var query string
	if args.Query != nil {
		query = *args.Query
	}

	return &codeHostRepositoryConnectionResolver{
		db:       r.db,
		userID:   userID,
		codeHost: codeHost,
		query:    query,
	}, nil
}

type codeHostRepositoryConnectionResolver struct {
	userID   int32
	codeHost int64
	query    string

	once  sync.Once
	nodes []*codeHostRepositoryResolver
	err   error
	db    dbutil.DB
}

func (r *codeHostRepositoryConnectionResolver) Nodes(ctx context.Context) ([]*codeHostRepositoryResolver, error) {
	r.once.Do(func() {
		var (
			svcs []*types.ExternalService
			err  error
		)
		// get all external services for user, or for the specified external service
		if r.codeHost == 0 {
			svcs, err = database.GlobalExternalServices.List(ctx, database.ExternalServicesListOptions{NamespaceUserID: r.userID})
			if err != nil {
				r.err = err
				return
			}
		} else {
			svc, err := database.GlobalExternalServices.GetByID(ctx, r.codeHost)
			if err != nil {
				r.err = err
				return
			}
			// 🚨 SECURITY: if the user doesn't own this service, check they're site admin
			if err := backend.CheckUserIsSiteAdmin(ctx, r.userID); svc.NamespaceUserID != r.userID && err != nil {
				r.err = err
				return
			}
			svcs = []*types.ExternalService{svc}
		}
		// get Source for all external services
		var (
			results  = make(chan []types.CodeHostRepository)
			g, ctx   = errgroup.WithContext(ctx)
			svcsByID = make(map[int64]*types.ExternalService)
		)
		for _, svc := range svcs {
			svcsByID[svc.ID] = svc
			src, err := repos.NewSource(svc, cf)
			if err != nil {
				r.err = err
				return
			}
			if af, ok := src.(repos.AffiliatedRepositorySource); ok {
				g.Go(func() error {
					repos, err := af.AffiliatedRepositories(ctx)
					if err != nil {
						return err
					}
					select {
					case results <- repos:
					case <-ctx.Done():
						return ctx.Err()
					}
					return nil
				})
			}
		}
		go func() {
			// wait for all sources to return their repos
			err = g.Wait()
			// signal the collector to finish
			close(results)
		}()

		// are we allowed to show the user private repos?
		allowPrivate, err := allowPrivate(ctx, r.userID)
		if err != nil {
			r.err = err
			return
		}

		// collect all results
		r.nodes = []*codeHostRepositoryResolver{}
		for repos := range results {
			for _, repo := range repos {
				repo := repo
				if r.query != "" && !strings.Contains(strings.ToLower(repo.Name), r.query) {
					continue
				}
				if !allowPrivate && repo.Private {
					continue
				}
				r.nodes = append(r.nodes, &codeHostRepositoryResolver{
					db:       r.db,
					codeHost: svcsByID[repo.CodeHostID],
					repo:     &repo,
				})
			}
		}
		sort.Slice(r.nodes, func(i, j int) bool {
			return r.nodes[i].repo.Name < r.nodes[j].repo.Name
		})
	})
	return r.nodes, r.err
}

type codeHostRepositoryResolver struct {
	repo     *types.CodeHostRepository
	codeHost *types.ExternalService
	db       dbutil.DB
}

func (r *codeHostRepositoryResolver) Name() string {
	return r.repo.Name
}

func (r *codeHostRepositoryResolver) Private() bool {
	return r.repo.Private
}

func (r *codeHostRepositoryResolver) CodeHost(ctx context.Context) *externalServiceResolver {
	return &externalServiceResolver{
		db:              r.db,
		externalService: r.codeHost,
	}
}

func allowPrivate(ctx context.Context, userID int32) (bool, error) {
	if conf.ExternalServiceUserMode() == conf.ExternalServiceModeAll {
		return true, nil
	}
	return database.GlobalUsers.HasTag(ctx, userID, database.TagAllowUserExternalServicePrivate)
}
