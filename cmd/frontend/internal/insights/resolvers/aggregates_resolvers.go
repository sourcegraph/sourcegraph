package resolvers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/insights/aggregation"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/querybuilder"
	"github.com/sourcegraph/sourcegraph/internal/insights/query/streaming"
	"github.com/sourcegraph/sourcegraph/internal/insights/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/settings"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	defaultAggregationBufferSize          = 500
	defaultSearchTimeLimitSeconds         = 2
	extendedSearchTimeLimitSecondsDefault = 55
	defaultproactiveResultsLimit          = 50000
	maxProactiveResultsLimit              = 200000
)

// Possible reasons that grouping is disabled
const invalidQueryMsg = "Grouping is disabled because the search query is not valid."
const fileUnsupportedFieldValueFmt = `Grouping by file is not available for searches with "%s:%s".`
const authNotCommitDiffMsg = "Grouping by author is only available for diff and commit searches."
const repoMetadataNotRepoSelectMsg = "Grouping by repo metadata is only available for repository searches."
const cgInvalidQueryMsg = "Grouping by capture group is only available for regexp searches that contain a capturing group."
const cgMultipleQueryPatternMsg = "Grouping by capture group does not support search patterns with the following: and, or, negation."
const cgUnsupportedSelectFmt = `Grouping by capture group is not available for searches with "%s:%s".`

// Possible reasons that grouping would fail
const shardTimeoutMsg = "The query was unable to complete in the allocated time."
const generalTimeoutMsg = "The query was unable to complete in the allocated time."
const proactiveResultLimitMsg = "The query exceeded the number of results allowed over this time period."

// These should be very rare
const unknownAggregationModeMsg = "The requested grouping is not supported."                    // example if a request with mode = NOT_A_REAL_MODE came in, should fail at graphql level
const unableToModifyQueryMsg = "The search query was unable to be updated to support grouping." // if the query was valid but we were unable to add timeout: & count:all
const unableToCountGroupsMsg = "The search results were unable to be grouped successfully."     // if there was a failure while adding up the results

type searchAggregateResolver struct {
	postgresDB database.DB

	searchQuery string
	patternType string
	logger      log.Logger
	operations  *aggregationsOperations
}

func (r *searchAggregateResolver) getLogger() log.Logger {
	if r.logger == nil {
		r.logger = log.Scoped("searchAggregations")
	}
	return r.logger
}

func (r *searchAggregateResolver) ModeAvailability(ctx context.Context) []graphqlbackend.AggregationModeAvailabilityResolver {
	resolvers := []graphqlbackend.AggregationModeAvailabilityResolver{}
	for _, mode := range types.SearchAggregationModes {
		resolvers = append(resolvers, newAggregationModeAvailabilityResolver(r.searchQuery, r.patternType, mode))
	}
	return resolvers
}

func (r *searchAggregateResolver) Aggregations(ctx context.Context, args graphqlbackend.AggregationsArgs) (_ graphqlbackend.SearchAggregationResultResolver, err error) {
	var aggregationMode types.SearchAggregationMode

	ctx, _, endObservation := r.operations.aggregations.With(ctx, &err, observation.Args{
		MetricLabelValues: []string{strconv.FormatBool(args.ExtendedTimeout)},
	})
	defer func() {
		endObservation(1, observation.Args{MetricLabelValues: []string{string(aggregationMode)}})
	}()

	// Steps:
	// 1. - If no mode get the default mode
	// 2. - Validate mode is supported (if in default mode this is done in that step)
	// 3. - Modify search query (timeout: & count:)
	// 3. - Run Search
	// 4. - Check search for errors/alerts
	// 5 -  Generate correct resolver pass search results if valid
	if args.Mode == nil {
		aggregationMode = getDefaultAggregationMode(r.searchQuery, r.patternType)
	} else {
		aggregationMode = types.SearchAggregationMode(*args.Mode)
	}

	notAvailable, err := getNotAvailableReason(r.searchQuery, r.patternType, aggregationMode)
	if notAvailable != nil {
		return &searchAggregationResultResolver{resolver: newSearchAggregationNotAvailableResolver(*notAvailable, aggregationMode)}, nil
	}
	// It should not be possible for the getNotAvailableReason to return an err without giving a reason but leaving a fallback here incase.
	if err != nil {
		r.getLogger().Debug("unable to determine why aggregation is unavailable", log.String("mode", string(aggregationMode)), log.Error(err))
		return nil, err
	}
	proactiveLimit := getProactiveResultLimit()
	countValue := fmt.Sprintf("%d", proactiveLimit)
	searchTimelimit := defaultSearchTimeLimitSeconds
	if args.ExtendedTimeout {
		searchTimelimit = getExtendedTimeout(ctx, r.postgresDB)
		countValue = "all"
	}

	// If a search includes a timeout it reports as completing succesfully with the timeout is hit
	// This includes a timeout in the search that is a second longer than the context we will cancel as a fail safe
	modifiedQuery, err := querybuilder.AggregationQuery(querybuilder.BasicQuery(r.searchQuery), searchTimelimit+1, countValue)
	if err != nil {
		r.getLogger().Debug("unable to build aggregation query", log.Error(err))
		return &searchAggregationResultResolver{
			resolver: newSearchAggregationNotAvailableResolver(notAvailableReason{reason: unableToModifyQueryMsg, reasonType: types.ERROR_OCCURRED}, aggregationMode),
		}, nil
	}

	aggregationBufferSize := conf.Get().InsightsAggregationsBufferSize
	if aggregationBufferSize <= 0 {
		aggregationBufferSize = defaultAggregationBufferSize
	}
	cappedAggregator := aggregation.NewLimitedAggregator(aggregationBufferSize)
	tabulationErrors := []error{}
	tabulationFunc := func(amr *aggregation.AggregationMatchResult, err error) {
		if err != nil {
			r.getLogger().Debug("unable to aggregate results", log.Error(err))
			tabulationErrors = append(tabulationErrors, err)
			return
		}
		cappedAggregator.Add(amr.Key.Group, int32(amr.Count))
	}

	countingFunc, err := aggregation.GetCountFuncForMode(r.searchQuery, r.patternType, aggregationMode)
	if err != nil {
		r.getLogger().Debug("no aggregation counting function for mode", log.String("mode", string(aggregationMode)), log.Error(err))
		return &searchAggregationResultResolver{
			resolver: newSearchAggregationNotAvailableResolver(
				notAvailableReason{reason: unknownAggregationModeMsg, reasonType: types.ERROR_OCCURRED},
				aggregationMode),
		}, nil
	}

	requestContext, cancelReqContext := context.WithTimeout(ctx, time.Second*time.Duration(searchTimelimit))
	defer cancelReqContext()
	searchClient := streaming.NewInsightsSearchClient(r.postgresDB)
	searchResultsAggregator := aggregation.NewSearchResultsAggregatorWithContext(requestContext, tabulationFunc, countingFunc, r.postgresDB, aggregationMode)

	_, err = searchClient.Search(requestContext, string(modifiedQuery), &r.patternType, searchResultsAggregator)
	if err != nil || requestContext.Err() != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			r.getLogger().Debug("aggregation search did not complete in time", log.String("mode", string(aggregationMode)), log.Bool("extendedTimeout", args.ExtendedTimeout))
			reasonType := types.TIMEOUT_EXTENSION_AVAILABLE
			if args.ExtendedTimeout {
				reasonType = types.TIMEOUT_NO_EXTENSION_AVAILABLE
			}
			return &searchAggregationResultResolver{resolver: newSearchAggregationNotAvailableResolver(notAvailableReason{reason: generalTimeoutMsg, reasonType: reasonType}, aggregationMode)}, nil
		} else {
			return nil, err
		}
	}

	successful, failureReason := searchSuccessful(tabulationErrors, searchResultsAggregator.ShardTimeoutOccurred(), args.ExtendedTimeout, searchResultsAggregator.ResultLimitHit(proactiveLimit))
	if !successful {
		return &searchAggregationResultResolver{resolver: newSearchAggregationNotAvailableResolver(failureReason, aggregationMode)}, nil
	}

	results := buildResults(cappedAggregator, int(args.Limit), aggregationMode, r.searchQuery, r.patternType)

	return &searchAggregationResultResolver{resolver: &searchAggregationModeResultResolver{
		searchQuery:  r.searchQuery,
		patternType:  r.patternType,
		mode:         aggregationMode,
		results:      results,
		isExhaustive: cappedAggregator.OtherCounts().GroupCount == 0,
	}}, nil
}

func getProactiveResultLimit() int {
	configLimit := conf.Get().InsightsAggregationsProactiveResultLimit
	if configLimit <= 0 {
		configLimit = defaultproactiveResultsLimit
	}
	return min(configLimit, maxProactiveResultsLimit)

}

func getExtendedTimeout(ctx context.Context, db database.DB) int {
	searchLimit := limits.SearchLimits(conf.Get()).MaxTimeoutSeconds

	settings, err := settings.CurrentUserFinal(ctx, db)
	if err != nil || settings == nil {
		return extendedSearchTimeLimitSecondsDefault
	}
	val := settings.InsightsAggregationsExtendedTimeout
	if val > 0 {
		return min(searchLimit, val)
	}
	return extendedSearchTimeLimitSecondsDefault
}

// getDefaultAggregationMode returns a default aggregation mode for a potential query
// this function should not fail because any search can be aggregated by repo
func getDefaultAggregationMode(searchQuery, patternType string) types.SearchAggregationMode {
	captureGroup, _, _ := canAggregateByCaptureGroup(searchQuery, patternType)
	if captureGroup {
		return types.CAPTURE_GROUP_AGGREGATION_MODE
	}
	author, _, _ := canAggregateByAuthor(searchQuery, patternType)
	if author {
		return types.AUTHOR_AGGREGATION_MODE
	}
	file, _, _ := canAggregateByPath(searchQuery, patternType)
	// We ignore the error here as the function errors if the query has multiple query steps.
	targetsSingleRepo, _ := querybuilder.IsSingleRepoQuery(querybuilder.BasicQuery(searchQuery))
	if file && targetsSingleRepo {
		return types.PATH_AGGREGATION_MODE
	}

	return types.REPO_AGGREGATION_MODE
}

func searchSuccessful(tabulationErrors []error, shardTimeoutOccurred, runningWithExtendedTimeout, resultLimitHit bool) (bool, notAvailableReason) {
	if len(tabulationErrors) > 0 {
		return false, notAvailableReason{reason: unableToCountGroupsMsg, reasonType: types.ERROR_OCCURRED}
	}
	if shardTimeoutOccurred {
		reasonType := types.TIMEOUT_EXTENSION_AVAILABLE
		if runningWithExtendedTimeout {
			reasonType = types.TIMEOUT_NO_EXTENSION_AVAILABLE
		}
		return false, notAvailableReason{reason: shardTimeoutMsg, reasonType: reasonType}
	}

	// This is a protective feature to limit the number of results proactive aggregations could process
	// It behaves like a timeout so the user has an option to re-run with the extended timeout that has no result limit
	if !runningWithExtendedTimeout && resultLimitHit {
		return false, notAvailableReason{reason: proactiveResultLimitMsg, reasonType: types.TIMEOUT_EXTENSION_AVAILABLE}
	}
	return true, notAvailableReason{}
}

type aggregationResults struct {
	groups           []graphqlbackend.AggregationGroup
	otherResultCount int
	otherGroupCount  int
	totalCount       uint32
}

type AggregationGroup struct {
	label string
	count int
	query *string
}

func (r *AggregationGroup) Label() string {
	return r.label
}
func (r *AggregationGroup) Count() int32 {
	return int32(r.count)
}
func (r *AggregationGroup) Query() (*string, error) {
	return r.query, nil
}

func buildResults(aggregator aggregation.LimitedAggregator, limit int, mode types.SearchAggregationMode, originalQuery string, patternType string) aggregationResults {
	sorted := aggregator.SortAggregate()
	groups := make([]graphqlbackend.AggregationGroup, 0, limit)
	otherResults := aggregator.OtherCounts().ResultCount
	otherGroups := aggregator.OtherCounts().GroupCount
	var totalCount uint32

	for i := range len(sorted) {
		if i < limit {
			label := sorted[i].Label
			drilldownQuery, err := buildDrilldownQuery(mode, originalQuery, label, patternType)
			if err != nil {
				// for some reason we couldn't generate a new query, so fallback to the original
				drilldownQuery = originalQuery
			}
			groups = append(groups, &AggregationGroup{
				label: label,
				count: int(sorted[i].Count),
				query: &drilldownQuery,
			})
		} else {
			otherGroups++
			otherResults += sorted[i].Count
		}
		totalCount += uint32(sorted[i].Count)
	}

	return aggregationResults{
		groups:           groups,
		otherResultCount: int(otherResults),
		otherGroupCount:  int(otherGroups),
		totalCount:       totalCount,
	}
}

func newAggregationModeAvailabilityResolver(searchQuery string, patternType string, mode types.SearchAggregationMode) graphqlbackend.AggregationModeAvailabilityResolver {
	return &aggregationModeAvailabilityResolver{searchQuery: searchQuery, patternType: patternType, mode: mode}
}

type aggregationModeAvailabilityResolver struct {
	searchQuery string
	patternType string
	mode        types.SearchAggregationMode
}

func (r *aggregationModeAvailabilityResolver) Mode() string {
	return string(r.mode)
}

func (r *aggregationModeAvailabilityResolver) Available() bool {
	canAggregateByFunc := getAggregateBy(r.mode)
	if canAggregateByFunc == nil {
		return false
	}
	available, _, err := canAggregateByFunc(r.searchQuery, r.patternType)
	if err != nil {
		return false
	}
	return available
}

func (r *aggregationModeAvailabilityResolver) ReasonUnavailable() (*string, error) {
	notAvailable, err := getNotAvailableReason(r.searchQuery, r.patternType, r.mode)
	if err != nil {
		return nil, err
	}
	if notAvailable != nil {
		return &notAvailable.reason, nil
	}
	return nil, nil

}

func getNotAvailableReason(query, patternType string, mode types.SearchAggregationMode) (*notAvailableReason, error) {
	canAggregateByFunc := getAggregateBy(mode)
	if canAggregateByFunc == nil {
		reason := fmt.Sprintf(`Grouping by "%v" is not supported.`, mode)
		return &notAvailableReason{reason: reason, reasonType: types.ERROR_OCCURRED}, nil
	}
	_, reason, err := canAggregateByFunc(query, patternType)
	if reason != nil {
		return reason, nil
	}

	return nil, err
}

func getAggregateBy(mode types.SearchAggregationMode) canAggregateBy {
	checkByMode := map[types.SearchAggregationMode]canAggregateBy{
		types.REPO_AGGREGATION_MODE:          canAggregateByRepo,
		types.PATH_AGGREGATION_MODE:          canAggregateByPath,
		types.AUTHOR_AGGREGATION_MODE:        canAggregateByAuthor,
		types.CAPTURE_GROUP_AGGREGATION_MODE: canAggregateByCaptureGroup,
		types.REPO_METADATA_AGGREGATION_MODE: canAggregateByRepoMetadata,
	}
	canAggregateByFunc, ok := checkByMode[mode]
	if !ok {
		return nil
	}
	return canAggregateByFunc
}

type notAvailableReason struct {
	reason     string
	reasonType types.AggregationNotAvailableReasonType
}

type canAggregateBy func(searchQuery, patternType string) (bool, *notAvailableReason, error)

func canAggregateByRepo(searchQuery, patternType string) (bool, *notAvailableReason, error) {
	_, err := querybuilder.ParseQuery(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: invalidQueryMsg, reasonType: types.INVALID_QUERY}, errors.Wrapf(err, "ParseQuery")
	}
	// We can always aggregate by repo.
	return true, nil, nil
}

func canAggregateByPath(searchQuery, patternType string) (bool, *notAvailableReason, error) {
	plan, err := querybuilder.ParseQuery(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: invalidQueryMsg, reasonType: types.INVALID_QUERY}, errors.Wrapf(err, "ParseQuery")
	}
	parameters := querybuilder.ParametersFromQueryPlan(plan)
	// cannot aggregate over:
	// - searches by commit, diff or repo
	for _, parameter := range parameters {
		if parameter.Field == query.FieldSelect || parameter.Field == query.FieldType {
			if strings.EqualFold(parameter.Value, "commit") || strings.EqualFold(parameter.Value, "diff") || strings.EqualFold(parameter.Value, "repo") {
				reason := fmt.Sprintf(fileUnsupportedFieldValueFmt,
					parameter.Field, parameter.Value)
				return false, &notAvailableReason{reason: reason, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
			}
		}
	}
	return true, nil, nil
}

func canAggregateByAuthor(searchQuery, patternType string) (bool, *notAvailableReason, error) {
	plan, err := querybuilder.ParseQuery(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: invalidQueryMsg, reasonType: types.INVALID_QUERY}, errors.Wrapf(err, "ParseQuery")
	}
	parameters := querybuilder.ParametersFromQueryPlan(plan)
	// can only aggregate over type:diff and select/type:commit searches.
	// users can make searches like `type:commit fix select:repo` but assume a faulty search like that is on them.
	for _, parameter := range parameters {
		if parameter.Field == query.FieldSelect || parameter.Field == query.FieldType {
			if parameter.Value == "diff" || parameter.Value == "commit" {
				return true, nil, nil
			}
		}
	}
	return false, &notAvailableReason{reason: authNotCommitDiffMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
}

func canAggregateByCaptureGroup(searchQuery, patternType string) (bool, *notAvailableReason, error) {
	plan, err := querybuilder.ParseQuery(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: invalidQueryMsg, reasonType: types.INVALID_QUERY}, errors.Wrapf(err, "ParseQuery")
	}

	searchType, err := querybuilder.DetectSearchType(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: cgInvalidQueryMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, err
	}
	if !(searchType == query.SearchTypeRegex || searchType == query.SearchTypeStandard || searchType == query.SearchTypeLucky) {
		return false, &notAvailableReason{reason: cgInvalidQueryMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
	}

	// A query should contain at least a regexp pattern and capture group to allow capture group aggregation.
	// Only the first capture group will be used for aggregation.
	replacer, err := querybuilder.NewPatternReplacer(querybuilder.BasicQuery(searchQuery), searchType)
	if errors.Is(err, querybuilder.UnsupportedPatternTypeErr) {
		return false, &notAvailableReason{reason: cgInvalidQueryMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
	} else if errors.Is(err, querybuilder.MultiplePatternErr) {
		return false, &notAvailableReason{reason: cgMultipleQueryPatternMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
	} else if err != nil {
		return false, &notAvailableReason{reason: cgInvalidQueryMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, errors.Wrap(err, "pattern parsing")
	}

	if !replacer.HasCaptureGroups() {
		return false, &notAvailableReason{reason: cgInvalidQueryMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
	}

	// We use the plan to obtain the query parameters. The pattern is already validated in `NewPatternReplacer`.
	parameters := querybuilder.ParametersFromQueryPlan(plan)

	// Exclude "select" for anything except "content" because if it's not content it means the regexp is not applying to the return values
	notAllowedSelectValues := map[string]struct{}{"repo": {}, "file": {}, "commit": {}, "symbol": {}}
	// At the moment we don't allow capture group aggregation for diff or symbol searches
	notAllowedFieldTypeValues := map[string]struct{}{"diff": {}, "symbol": {}}
	for _, parameter := range parameters {
		paramValue := strings.ToLower(parameter.Value)
		_, notAllowedSelect := notAllowedSelectValues[paramValue]
		if strings.EqualFold(parameter.Field, query.FieldSelect) && notAllowedSelect {
			reason := fmt.Sprintf(cgUnsupportedSelectFmt, strings.ToLower(parameter.Field), strings.ToLower(parameter.Value))
			return false, &notAvailableReason{reason: reason, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
		}
		_, notAllowedFieldType := notAllowedFieldTypeValues[paramValue]
		if strings.EqualFold(parameter.Field, query.FieldType) && notAllowedFieldType {
			reason := fmt.Sprintf(cgUnsupportedSelectFmt, strings.ToLower(parameter.Field), strings.ToLower(parameter.Value))
			return false, &notAvailableReason{reason: reason, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
		}
	}

	return true, nil, nil
}

func canAggregateByRepoMetadata(searchQuery, patternType string) (bool, *notAvailableReason, error) {
	plan, err := querybuilder.ParseQuery(searchQuery, patternType)
	if err != nil {
		return false, &notAvailableReason{reason: invalidQueryMsg, reasonType: types.INVALID_QUERY}, errors.Wrapf(err, "ParseQuery")
	}
	parameters := querybuilder.ParametersFromQueryPlan(plan)
	// we allow aggregating only for select:repo searches
	for _, parameter := range parameters {
		if parameter.Field == query.FieldSelect {
			if parameter.Value == "repo" {
				return true, nil, nil
			}
		}
	}
	return false, &notAvailableReason{reason: repoMetadataNotRepoSelectMsg, reasonType: types.INVALID_AGGREGATION_MODE_FOR_QUERY}, nil
}

// A  type to represent the GraphQL union SearchAggregationResult
type searchAggregationResultResolver struct {
	resolver any
}

// ToExhaustiveSearchAggregationResult is used by the GraphQL library to resolve type fragments for unions
func (r *searchAggregationResultResolver) ToExhaustiveSearchAggregationResult() (graphqlbackend.ExhaustiveSearchAggregationResultResolver, bool) {
	res, ok := r.resolver.(*searchAggregationModeResultResolver)
	if ok && res.isExhaustive {
		return res, ok
	}
	return nil, false
}

// ToNonExhaustiveSearchAggregationResult is used by the GraphQL library to resolve type fragments for unions
func (r *searchAggregationResultResolver) ToNonExhaustiveSearchAggregationResult() (graphqlbackend.NonExhaustiveSearchAggregationResultResolver, bool) {
	res, ok := r.resolver.(*searchAggregationModeResultResolver)
	if ok && !res.isExhaustive {
		return res, ok
	}
	return nil, false
}

// ToSearchAggregationNotAvailable is used by the GraphQL library to resolve type fragments for unions
func (r *searchAggregationResultResolver) ToSearchAggregationNotAvailable() (graphqlbackend.SearchAggregationNotAvailable, bool) {
	res, ok := r.resolver.(*searchAggregationNotAvailableResolver)
	return res, ok
}

func newSearchAggregationNotAvailableResolver(reason notAvailableReason, mode types.SearchAggregationMode) graphqlbackend.SearchAggregationNotAvailable {
	return &searchAggregationNotAvailableResolver{
		reason:     reason.reason,
		reasonType: reason.reasonType,
		mode:       mode,
	}
}

type searchAggregationNotAvailableResolver struct {
	reason     string
	mode       types.SearchAggregationMode
	reasonType types.AggregationNotAvailableReasonType
}

func (r *searchAggregationNotAvailableResolver) Reason() string {
	return r.reason
}
func (r *searchAggregationNotAvailableResolver) ReasonType() string {
	return string(r.reasonType)
}
func (r *searchAggregationNotAvailableResolver) Mode() string {
	return string(r.mode)
}

// Resolver to calculate aggregations for a combination of search query, pattern type, aggregation mode
type searchAggregationModeResultResolver struct {
	searchQuery  string
	patternType  string
	mode         types.SearchAggregationMode
	results      aggregationResults
	isExhaustive bool
}

func (r *searchAggregationModeResultResolver) Groups() ([]graphqlbackend.AggregationGroup, error) {
	return r.results.groups, nil
}

func (r *searchAggregationModeResultResolver) OtherResultCount() (*int32, error) {
	var count = int32(r.results.otherResultCount)
	return &count, nil
}

// OtherGroupCount - used for exhaustive aggregations to indicate count of additional groups
func (r *searchAggregationModeResultResolver) OtherGroupCount() (*int32, error) {
	var count = int32(r.results.otherGroupCount)
	return &count, nil
}

// ApproximateOtherGroupCount - used for nonexhaustive aggregations to indicate approx count of additional groups
func (r *searchAggregationModeResultResolver) ApproximateOtherGroupCount() (*int32, error) {
	var count = int32(r.results.otherGroupCount)
	return &count, nil
}

func (r *searchAggregationModeResultResolver) SupportsPersistence() (*bool, error) {
	supported := false
	return &supported, nil
}

func (r *searchAggregationModeResultResolver) Mode() (string, error) {
	return string(r.mode), nil
}

func buildDrilldownQuery(mode types.SearchAggregationMode, originalQuery string, drilldown string, patternType string) (string, error) {
	caseSensitive := false
	var modifierFunc func(querybuilder.BasicQuery, string) (querybuilder.BasicQuery, error)
	switch mode {
	case types.REPO_AGGREGATION_MODE:
		modifierFunc = querybuilder.AddRepoFilter
	case types.REPO_METADATA_AGGREGATION_MODE:
		modifierFunc = querybuilder.AddRepoMetadataFilter
	case types.PATH_AGGREGATION_MODE:
		modifierFunc = querybuilder.AddFileFilter
	case types.AUTHOR_AGGREGATION_MODE:
		modifierFunc = querybuilder.AddAuthorFilter
	case types.CAPTURE_GROUP_AGGREGATION_MODE:
		searchType, err := client.SearchTypeFromString(patternType)
		if err != nil {
			return "", err
		}
		replacer, err := querybuilder.NewPatternReplacer(querybuilder.BasicQuery(originalQuery), searchType)
		if err != nil {
			return "", err
		}
		modifierFunc = func(basicQuery querybuilder.BasicQuery, s string) (querybuilder.BasicQuery, error) {
			return replacer.Replace(s)
		}
		caseSensitive = true
	default:
		return "", errors.New("unsupported aggregation mode")
	}

	newQuery, err := modifierFunc(querybuilder.BasicQuery(originalQuery), drilldown)
	if err != nil {
		return "", err
	}
	if caseSensitive {
		newQuery, err = querybuilder.SetCaseSensitivity(newQuery, true)
	}
	return string(newQuery), err
}
