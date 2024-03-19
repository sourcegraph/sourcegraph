package jobutil

import (
	"slices"
	"strings"
	"time"

	"github.com/go-enry/go-enry/v2"
	"github.com/grafana/regexp"

	zoektquery "github.com/sourcegraph/zoekt/query"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	ownsearch "github.com/sourcegraph/sourcegraph/internal/own/search"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/codycontext"
	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	searchrepos "github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/searchcontexts"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/smartsearch"
	"github.com/sourcegraph/sourcegraph/internal/search/structural"
	"github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// NewPlanJob converts a query.Plan into its job tree representation.
func NewPlanJob(inputs *search.Inputs, plan query.Plan) (job.Job, error) {
	children := make([]job.Job, 0, len(plan))
	for _, q := range plan {
		child, err := NewBasicJob(inputs, q)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	jobTree := NewOrJob(children...)
	newJob := func(b query.Basic) (job.Job, error) {
		return NewBasicJob(inputs, b)
	}

	if inputs.PatternType == query.SearchTypeCodyContext {
		if inputs.SearchMode == search.SmartSearch {
			return nil, errors.New("The 'codycontext' patterntype is not compatible with Smart Search")
		}

		newJobTree, err := codycontext.NewSearchJob(plan, newJob)
		if err != nil {
			return nil, err
		}

		jobTree = newJobTree
	}

	if inputs.SearchMode == search.SmartSearch || inputs.PatternType == query.SearchTypeLucky {
		jobTree = smartsearch.NewSmartSearchJob(jobTree, newJob, plan)
	}

	alertJob := NewAlertJob(inputs, jobTree)
	logJob := NewLogJob(inputs, alertJob)
	return logJob, nil
}

// NewBasicJob converts a query.Basic into its job tree representation.
func NewBasicJob(inputs *search.Inputs, b query.Basic) (job.Job, error) {

	var children []job.Job
	addJob := func(j job.Job) {
		children = append(children, j)
	}

	// Modify the input query if the user specified `file:contains.content()`
	fileContainsPatterns := b.FileContainsContent()
	originalQuery := b
	if len(fileContainsPatterns) > 0 {
		newNodes := make([]query.Node, 0, len(fileContainsPatterns)+1)
		for _, pat := range fileContainsPatterns {
			node := query.Pattern{Value: pat}
			node.Annotation.Labels.Set(query.Regexp)
			newNodes = append(newNodes, node)
		}
		if b.Pattern != nil {
			newNodes = append(newNodes, b.Pattern)
		}
		b.Pattern = query.Operator{Operands: newNodes, Kind: query.And}
	}

	{
		// This block generates jobs that can be built directly from
		// a basic query rather than first being expanded into
		// flat queries.
		resultTypes := computeResultTypes(b, inputs.PatternType, defaultResultTypes)
		fileMatchLimit := int32(computeFileMatchLimit(b, inputs.DefaultLimit()))
		selector, _ := filter.SelectPathFromString(b.FindValue(query.FieldSelect)) // Invariant: select is validated
		repoOptions := toRepoOptions(b, inputs.UserSettings)
		repoUniverseSearch, skipRepoSubsetSearch, runZoektOverRepos := jobMode(b, repoOptions, resultTypes, inputs)

		builder := &jobBuilder{
			query:           b,
			patternType:     inputs.PatternType,
			resultTypes:     resultTypes,
			repoOptions:     repoOptions,
			features:        inputs.Features,
			fileMatchLimit:  fileMatchLimit,
			selector:        selector,
			numContextLines: int(inputs.ContextLines),
		}

		if resultTypes.Has(result.TypeFile | result.TypePath) {
			// Create Global Text Search jobs.
			if repoUniverseSearch {
				searchJob, err := builder.newZoektGlobalSearch(search.TextRequest)
				if err != nil {
					return nil, err
				}
				addJob(searchJob)
			}

			if !skipRepoSubsetSearch {
				if runZoektOverRepos {
					searchJob, err := builder.newZoektSearch(search.TextRequest)
					if err != nil {
						return nil, err
					}
					addJob(&repoPagerJob{
						child:            &reposPartialJob{searchJob},
						repoOpts:         repoOptions,
						containsRefGlobs: query.ContainsRefGlobs(b.ToParseTree()),
					})
				}

				// searcher to use full deadline if timeout: set or we are not batch.

				searcherJob := NewTextSearchJob(b, inputs, resultTypes, repoOptions)
				addJob(searcherJob)
			}
		}

		if resultTypes.Has(result.TypeSymbol) {
			// Create Global Symbol Search jobs.
			if repoUniverseSearch {
				searchJob, err := builder.newZoektGlobalSearch(search.SymbolRequest)
				if err != nil {
					return nil, err
				}
				addJob(searchJob)
			}

			if !skipRepoSubsetSearch && runZoektOverRepos {
				searchJob, err := builder.newZoektSearch(search.SymbolRequest)
				if err != nil {
					return nil, err
				}
				addJob(&repoPagerJob{
					child:            &reposPartialJob{searchJob},
					repoOpts:         repoOptions,
					containsRefGlobs: query.ContainsRefGlobs(b.ToParseTree()),
				})
			}
		}

		if resultTypes.Has(result.TypeCommit) || resultTypes.Has(result.TypeDiff) {
			_, _, own := isOwnershipSearch(b)
			diff := resultTypes.Has(result.TypeDiff)
			repoOptionsCopy := repoOptions
			repoOptionsCopy.OnlyCloned = true

			commitSearchJob := &commit.SearchJob{
				Query:                commit.QueryToGitQuery(originalQuery, diff),
				Diff:                 diff,
				Limit:                int(fileMatchLimit),
				IncludeModifiedFiles: authz.SubRepoEnabled(authz.DefaultSubRepoPermsChecker) || own,
			}

			addJob(
				&repoPagerJob{
					child:            &reposPartialJob{commitSearchJob},
					repoOpts:         repoOptionsCopy,
					containsRefGlobs: query.ContainsRefGlobs(b.ToParseTree()),
					skipPartitioning: true,
				})
		}

		addJob(&searchrepos.ComputeExcludedJob{
			RepoOpts: repoOptions,
		})
	}

	{
		// This block generates a job for all the backend types that cannot
		// directly use a query.Basic and need to be split into query.Flat
		// first.
		flatJob, err := toFlatJobs(inputs, b)
		if err != nil {
			return nil, err
		}
		addJob(flatJob)
	}

	basicJob := NewParallelJob(children...)

	{ // Apply file:contains.content() post-filter
		if len(fileContainsPatterns) > 0 {
			var err error
			basicJob, err = NewFileContainsFilterJob(fileContainsPatterns, originalQuery.Pattern, b.IsCaseSensitive(), basicJob)
			if err != nil {
				return nil, err
			}
		}
	}

	{ // Apply code ownership post-search filter
		if includeOwners, excludeOwners, ok := isOwnershipSearch(b); ok {
			basicJob = ownsearch.NewFileHasOwnersJob(basicJob, includeOwners, excludeOwners)
		}
	}

	{ // Apply file:has.contributor() post-search filter
		if includeContributors, excludeContributors, ok := isContributorSearch(b); ok {
			includeRe := contributorsAsRegexp(includeContributors, b.IsCaseSensitive())
			excludeRe := contributorsAsRegexp(excludeContributors, b.IsCaseSensitive())
			basicJob = NewFileHasContributorsJob(basicJob, includeRe, excludeRe)
		}
	}

	{ // Apply subrepo permissions checks
		checker := authz.DefaultSubRepoPermsChecker
		if authz.SubRepoEnabled(checker) {
			basicJob = NewFilterJob(basicJob)
		}
	}

	{ // Apply selectors
		if v, _ := b.ToParseTree().StringValue(query.FieldSelect); v != "" {
			sp, _ := filter.SelectPathFromString(v) // Invariant: select already validated
			if isSelectOwnersSearch(sp) {
				// the select owners job is ran separately as it requires state and can return multiple owners from one match.
				basicJob = ownsearch.NewSelectOwnersJob(basicJob)
			} else {
				basicJob = NewSelectJob(sp, basicJob)
			}
		}
	}

	{ // Apply search result sanitization post-filter if enabled
		if len(inputs.SanitizeSearchPatterns) > 0 {
			basicJob = NewSanitizeJob(inputs.SanitizeSearchPatterns, basicJob)
		}
	}

	{ // Apply limit
		maxResults := b.ToParseTree().MaxResults(inputs.DefaultLimit())
		basicJob = NewLimitJob(maxResults, basicJob)
	}

	{ // Apply timeout
		timeout := timeoutDuration(inputs.Protocol, b)
		basicJob = NewTimeoutJob(timeout, basicJob)
	}

	{
		// WORKAROUND: On Sourcegraph.com some jobs can race with Zoekt (which
		// does ranking). This leads to unpleasant results, especially due to
		// the large index on Sourcegraph.com. We have this hacky workaround
		// here to ensure we search Zoekt first. Context:
		// https://github.com/sourcegraph/sourcegraph/issues/35993
		// https://github.com/sourcegraph/sourcegraph/issues/35994

		if inputs.OnSourcegraphDotCom && b.Pattern != nil {
			if _, ok := b.Pattern.(query.Pattern); ok {
				basicJob = orderRacingJobs(basicJob)
			}
		}

	}

	return basicJob, nil
}

func NewTextSearchJob(b query.Basic, inputs *search.Inputs, types result.Types, options search.RepoOptions) job.Job {
	// searcher to use full deadline if timeout: set or we are not batch.
	useFullDeadline := b.GetTimeout() != nil || b.Count() != nil || inputs.Protocol != search.Batch
	patternInfo := toTextPatternInfo(b, types, inputs.Features, inputs.DefaultLimit())

	searcherJob := &searcher.TextSearchJob{
		PatternInfo:     patternInfo,
		Indexed:         false,
		UseFullDeadline: useFullDeadline,
		Features:        *inputs.Features,
		PathRegexps:     getPathRegexps(b, patternInfo),
		NumContextLines: int(inputs.ContextLines),
	}

	return &repoPagerJob{
		child:            &reposPartialJob{searcherJob},
		repoOpts:         options,
		containsRefGlobs: query.ContainsRefGlobs(b.ToParseTree()),
	}
}

// orderRacingJobs ensures that searcher and repo search jobs only ever run
// sequentially after a Zoekt search has returned all its results.
func orderRacingJobs(j job.Job) job.Job {
	// First collect the searcher and repo job, if any, and delete them from
	// the tree. The jobs will be sequentially ordered after any Zoekt jobs. We
	// assume at most one searcher and one repo job exists.
	var collection []job.Job

	newJob := job.MapType(j, func(pager *repoPagerJob) job.Job {
		if job.HasDescendent[*searcher.TextSearchJob](pager) {
			collection = append(collection, pager)
			return &NoopJob{}
		}

		return pager
	})

	newJob = job.MapType(newJob, func(j *RepoSearchJob) job.Job {
		collection = append(collection, j)
		return &NoopJob{}
	})

	if len(collection) == 0 {
		return j
	}

	// Map the tree to execute jobs in "collection" after any Zoekt jobs. We
	// assume at most one of either two Zoekt search jobs may exist.
	seenZoektRepoSearch := false
	newJob = job.MapType(newJob, func(pager *repoPagerJob) job.Job {
		if job.HasDescendent[*zoekt.RepoSubsetTextSearchJob](pager) {
			seenZoektRepoSearch = true
			return NewSequentialJob(false, append([]job.Job{pager}, collection...)...)
		}
		return pager
	})

	seenZoektGlobalSearch := false
	newJob = job.MapType(newJob, func(current *zoekt.GlobalTextSearchJob) job.Job {
		if !seenZoektGlobalSearch {
			seenZoektGlobalSearch = true
			return NewSequentialJob(false, append([]job.Job{current}, collection...)...)
		}
		return current
	})

	if !seenZoektRepoSearch && !seenZoektGlobalSearch {
		// There were no Zoekt jobs, so no need to modify the tree. Return original.
		return j
	}

	return newJob
}

// NewFlatJob creates all jobs that are built from a query.Flat.
func NewFlatJob(searchInputs *search.Inputs, f query.Flat) (job.Job, error) {
	maxResults := f.MaxResults(searchInputs.DefaultLimit())
	resultTypes := computeResultTypes(f.ToBasic(), searchInputs.PatternType, defaultResultTypes)

	// searcher to use full deadline if timeout: set or we are not batch.
	useFullDeadline := f.GetTimeout() != nil || f.Count() != nil || searchInputs.Protocol != search.Batch

	repoOptions := toRepoOptions(f.ToBasic(), searchInputs.UserSettings)

	_, skipRepoSubsetSearch, _ := jobMode(f.ToBasic(), repoOptions, resultTypes, searchInputs)

	var allJobs []job.Job
	addJob := func(job job.Job) {
		allJobs = append(allJobs, job)
	}

	{
		// Create Symbol Search Jobs
		if resultTypes.Has(result.TypeSymbol) {
			// Create Symbol Search jobs over repo set.
			if !skipRepoSubsetSearch {
				request, err := toSymbolSearchRequest(f, searchInputs.Features)
				if err != nil {
					return nil, err
				}

				symbolSearchJob := &searcher.SymbolSearchJob{
					Request: request,
					Limit:   maxResults,
				}
				addJob(&repoPagerJob{
					child:            &reposPartialJob{symbolSearchJob},
					repoOpts:         repoOptions,
					containsRefGlobs: query.ContainsRefGlobs(f.ToBasic().ToParseTree()),
				})
			}
		}

		if resultTypes.Has(result.TypeStructural) {
			patternInfo := toTextPatternInfo(f.ToBasic(), resultTypes, searchInputs.Features, searchInputs.DefaultLimit())
			searcherArgs := &search.SearcherParameters{
				PatternInfo:     patternInfo,
				UseFullDeadline: useFullDeadline,
				Features:        *searchInputs.Features,
			}

			addJob(&structural.SearchJob{
				SearcherArgs:     searcherArgs,
				UseIndex:         f.Index(),
				ContainsRefGlobs: query.ContainsRefGlobs(f.ToBasic().ToParseTree()),
				RepoOpts:         repoOptions,
				BatchRetry:       searchInputs.Protocol == search.Batch,
			})
		}

		if resultTypes.Has(result.TypeRepo) {
			valid := func() bool {
				fieldAllowlist := map[string]struct{}{
					query.FieldRepo:               {},
					query.FieldContext:            {},
					query.FieldType:               {},
					query.FieldDefault:            {},
					query.FieldIndex:              {},
					query.FieldCount:              {},
					query.FieldTimeout:            {},
					query.FieldFork:               {},
					query.FieldArchived:           {},
					query.FieldVisibility:         {},
					query.FieldCase:               {},
					query.FieldRepoHasFile:        {},
					query.FieldRepoHasCommitAfter: {},
					query.FieldPatternType:        {},
					query.FieldSelect:             {},
				}

				// Don't run a repo search if the search contains fields that aren't on the allowlist.
				exists := true
				query.VisitParameter(f.ToBasic().ToParseTree(), func(field, _ string, _ bool, _ query.Annotation) {
					if _, ok := fieldAllowlist[field]; !ok {
						exists = false
					}
				})
				return exists
			}

			// returns an updated RepoOptions if the pattern part of a query can be used to
			// search repos. A problematic case we check for is when the pattern contains `@`,
			// which may confuse downstream logic to interpret it as part of `repo@rev` syntax.
			addPatternAsRepoFilter := func(pattern string, opts search.RepoOptions) (search.RepoOptions, bool) {
				if pattern == "" {
					return opts, true
				}

				opts.RepoFilters = append(make([]query.ParsedRepoFilter, 0, len(opts.RepoFilters)), opts.RepoFilters...)
				opts.CaseSensitiveRepoFilters = f.IsCaseSensitive()

				patternPrefix := strings.SplitN(pattern, "@", 2)
				if len(patternPrefix) == 1 || patternPrefix[0] != "" {
					// Extend the repo search using the pattern value, but
					// if the pattern contains @, only search the part
					// prefixed by the first @. This because downstream
					// logic will get confused by the presence of @ and try
					// to resolve repo revisions. See #27816.
					repoFilter, err := query.ParseRepositoryRevisions(patternPrefix[0])
					if err != nil {
						// Prefix is not valid regexp, so just reject it. This can happen for patterns where we've automatically added `(...).*?(...)`
						// such as `foo @bar` which becomes `(foo).*?(@bar)`, which when stripped becomes `(foo).*?(` which is unbalanced and invalid.
						// Why is this a mess? Because validation for everything, including repo values, should be done up front so far possible, not downtsream
						// after possible modifications. By the time we reach this code, the pattern should already have been considered valid to continue with
						// a search. But fixing the order of concerns for repo code is not something @rvantonder is doing today.
						return search.RepoOptions{}, false
					}
					opts.RepoFilters = append(opts.RepoFilters, repoFilter)
					return opts, true
				}

				// This pattern starts with @, of the form "@thing". We can't
				// consistently handle search repos of this form, because
				// downstream logic will attempt to interpret "thing" as a repo
				// revision, may fail, and cause us to raise an alert for any
				// non `type:repo` search. Better to not attempt a repo search.
				return search.RepoOptions{}, false
			}

			if valid() {
				if repoOptions, ok := addPatternAsRepoFilter(f.ToBasic().PatternString(), repoOptions); ok {
					descriptionPatterns := make([]*regexp.Regexp, 0, len(repoOptions.DescriptionPatterns))
					for _, pat := range repoOptions.DescriptionPatterns {
						descriptionPatterns = append(descriptionPatterns, regexp.MustCompile(`(?is)`+pat))
					}

					repoNamePatterns := make([]*regexp.Regexp, 0, len(repoOptions.RepoFilters))
					for _, repoFilter := range repoOptions.RepoFilters {
						repoNamePatterns = append(repoNamePatterns, repoFilter.RepoRegex)
					}

					addJob(&RepoSearchJob{
						RepoOpts:            repoOptions,
						DescriptionPatterns: descriptionPatterns,
						RepoNamePatterns:    repoNamePatterns,
					})
				}
			}
		}
	}

	return NewParallelJob(allJobs...), nil
}

// getPathRegexps parses the search and returns a set of regular expressions that represent
// how it matches file paths. These regexes are later used to create result matches on the
// file paths. We pass in the original query.Basic in addition to the TextPatternInfo just
// for convenience, so we can use methods like query.VisitPattern.
func getPathRegexps(b query.Basic, p *search.TextPatternInfo) (pathRegexps []*regexp.Regexp) {
	for _, pattern := range p.IncludePaths {
		if p.IsCaseSensitive {
			pathRegexps = append(pathRegexps, regexp.MustCompile(pattern))
		} else {
			pathRegexps = append(pathRegexps, regexp.MustCompile(`(?i)`+pattern))
		}
	}

	if b.Pattern != nil && p.PatternMatchesPath {
		query.VisitPattern([]query.Node{b.Pattern}, func(pattern string, negated bool, annotation query.Annotation) {
			if negated {
				return
			}

			if !annotation.Labels.IsSet(query.Regexp) {
				pattern = regexp.QuoteMeta(pattern)
			}
			if p.IsCaseSensitive {
				pathRegexps = append(pathRegexps, regexp.MustCompile(pattern))
			} else {
				pathRegexps = append(pathRegexps, regexp.MustCompile(`(?i)`+pattern))
			}
		})
	}
	return pathRegexps
}

func computeFileMatchLimit(b query.Basic, defaultLimit int) int {
	// Temporary fix:
	// If doing ownership or contributor search, we post-filter results so we may need more than
	// b.Count() results from the search backends to end up with enough results
	// sent down the stream.
	//
	// This is actually a more general problem with other post-filters, too but
	// keeps the scope of this change minimal.
	// The proper fix will likely be to establish proper result streaming and cancel
	// the stream once enough results have been consumed. We will revisit this
	// post-Starship March 2023 as part of search performance improvements for
	// ownership search.
	if _, _, ok := isContributorSearch(b); ok {
		// This is the int equivalent of count:all.
		return query.CountAllLimit
	}
	if _, _, ok := isOwnershipSearch(b); ok {
		// This is the int equivalent of count:all.
		return query.CountAllLimit
	}
	if v, _ := b.ToParseTree().StringValue(query.FieldSelect); v != "" {
		sp, _ := filter.SelectPathFromString(v) // Invariant: select already validated
		if isSelectOwnersSearch(sp) {
			// This is the int equivalent of count:all.
			return query.CountAllLimit
		}
	}

	return b.MaxResults(defaultLimit)
}

func isOwnershipSearch(b query.Basic) (include, exclude []string, ok bool) {
	if includeOwners, excludeOwners := b.FileHasOwner(); len(includeOwners) > 0 || len(excludeOwners) > 0 {
		return includeOwners, excludeOwners, true
	}
	return nil, nil, false
}

func isSelectOwnersSearch(sp filter.SelectPath) bool {
	// If the filter is for file.owners, this is a select:file.owners search, and we should apply special limits.
	return sp.Root() == filter.File && len(sp) == 2 && sp[1] == "owners"
}

func isContributorSearch(b query.Basic) (include, exclude []string, ok bool) {
	if includeContributors, excludeContributors := b.FileHasContributor(); len(includeContributors) > 0 || len(excludeContributors) > 0 {
		return includeContributors, excludeContributors, true
	}
	return nil, nil, false
}

func contributorsAsRegexp(contributors []string, isCaseSensitive bool) (res []*regexp.Regexp) {
	for _, pattern := range contributors {
		if isCaseSensitive {
			res = append(res, regexp.MustCompile(pattern))
		} else {
			res = append(res, regexp.MustCompile(`(?i)`+pattern))
		}
	}
	return res
}

func timeoutDuration(protocol search.Protocol, b query.Basic) time.Duration {
	// If we are an exhaustive search our logic is much simpler since we have
	// a very high default timeout and we ignore maxTimeout. We either use
	// the default or the value specified in timeout.
	if protocol == search.Exhaustive {
		if timeout := b.GetTimeout(); timeout != nil {
			return *timeout
		} else {
			return limits.DefaultTimeoutExhaustive
		}
	}

	d := limits.DefaultTimeout
	maxTimeout := time.Duration(limits.SearchLimits(conf.Get()).MaxTimeoutSeconds) * time.Second
	timeout := b.GetTimeout()
	if timeout != nil {
		d = *timeout
	} else if b.Count() != nil {
		// If `count:` is set but `timeout:` is not explicitly set, use the max timeout
		d = maxTimeout
	}
	if d > maxTimeout {
		d = maxTimeout
	}
	return d
}

func mapSlice(values []string, f func(string) string) []string {
	res := make([]string, len(values))
	for i, v := range values {
		res[i] = f(v)
	}
	return res
}

func toSymbolSearchRequest(f query.Flat, feat *search.Features) (*searcher.SymbolSearchRequest, error) {
	if f.Pattern != nil && f.Pattern.Negated {
		return nil, &query.UnsupportedError{
			Msg: "symbol search does not support negation.",
		}
	}

	// We convert literal searches to regexes, since the symbol search logic
	// assumes that a literal pattern is an escaped regular expression.
	regexpPattern := f.ToBasic().PatternString()

	// Handle file: and -file: filters.
	filesInclude, filesExclude := f.IncludeExcludeValues(query.FieldFile)

	// Handle lang: and -lang: filters.
	langAliasInclude, langAliasExclude := f.IncludeExcludeValues(query.FieldLang)
	var langInclude, langExclude []string
	if feat.ContentBasedLangFilters {
		langInclude = toLangFilters(langAliasInclude)
		langExclude = toLangFilters(langAliasExclude)
	} else {
		// If the 'search-content-based-lang-detection' feature is disabled, then we convert the filters
		// to file path regexes and do not pass any explicit language filters to the backend.
		filesInclude = append(filesInclude, mapSlice(langAliasInclude, query.LangToFileRegexp)...)
		filesExclude = append(filesExclude, mapSlice(langAliasExclude, query.LangToFileRegexp)...)
	}

	return &searcher.SymbolSearchRequest{
		RegexpPattern:   regexpPattern,
		IsCaseSensitive: f.IsCaseSensitive(),
		IncludePatterns: filesInclude,
		ExcludePattern:  query.UnionRegExps(filesExclude),
		IncludeLangs:    langInclude,
		ExcludeLangs:    langExclude,
	}, nil
}

// toTextPatternInfo converts a query to internal values that drive text search.
func toTextPatternInfo(b query.Basic, resultTypes result.Types, feat *search.Features, defaultLimit int) *search.TextPatternInfo {
	// Handle file: and -file: filters.
	filesInclude, filesExclude := b.IncludeExcludeValues(query.FieldFile)

	// Handle lang: and -lang: filters.
	langAliasInclude, langAliasExclude := b.IncludeExcludeValues(query.FieldLang)
	var langInclude, langExclude []string
	if feat.ContentBasedLangFilters {
		langInclude = toLangFilters(langAliasInclude)
		langExclude = toLangFilters(langAliasExclude)
	} else {
		// If the 'search-content-based-lang-detection' feature is disabled, then we convert the filters
		// to file path regexes and do not pass any explicit language filters to the backend.
		filesInclude = append(filesInclude, mapSlice(langAliasInclude, query.LangToFileRegexp)...)
		filesExclude = append(filesExclude, mapSlice(langAliasExclude, query.LangToFileRegexp)...)
	}

	selector, _ := filter.SelectPathFromString(b.FindValue(query.FieldSelect)) // Invariant: select is validated
	count := b.MaxResults(defaultLimit)

	return &search.TextPatternInfo{
		Query:                        protocol.FromJobNode(b.Pattern),
		IsStructuralPat:              b.IsStructural(),
		IsCaseSensitive:              b.IsCaseSensitive(),
		FileMatchLimit:               int32(count),
		Languages:                    langAliasInclude,
		IncludePaths:                 filesInclude,
		ExcludePaths:                 query.UnionRegExps(filesExclude),
		IncludeLangs:                 langInclude,
		ExcludeLangs:                 langExclude,
		PatternMatchesPath:           resultTypes.Has(result.TypePath),
		PatternMatchesContent:        resultTypes.Has(result.TypeFile),
		PathPatternsAreCaseSensitive: b.IsCaseSensitive(),
		CombyRule:                    b.FindValue(query.FieldCombyRule),
		Index:                        b.Index(),
		Select:                       selector,
	}
}

func toLangFilters(aliases []string) []string {
	var filters []string
	for _, alias := range aliases {
		lang, _ := enry.GetLanguageByAlias(alias) // Invariant: lang is valid.
		if !slices.Contains(filters, lang) {
			filters = append(filters, lang)
		}
	}
	return filters
}

const defaultResultTypes = result.TypeFile | result.TypePath | result.TypeRepo

// computeResultTypes returns result types based three inputs: `type:...` in the query,
// the `pattern`, and top-level `searchType` (coming from a GQL value).
func computeResultTypes(b query.Basic, searchType query.SearchType, defaultTypes result.Types) result.Types {
	if searchType == query.SearchTypeStructural && !b.IsEmptyPattern() {
		return result.TypeStructural
	}

	types, _ := b.IncludeExcludeValues(query.FieldType)

	if len(types) == 0 && b.Pattern != nil {
		// When the pattern is set via `content:`, we set the annotation on
		// the pattern to IsContent. So if all Patterns are from content: we
		// should only search TypeFile.
		hasPattern := false
		allIsContent := true
		query.VisitPattern([]query.Node{b.Pattern}, func(value string, negated bool, annotation query.Annotation) {
			hasPattern = true
			allIsContent = allIsContent && annotation.Labels.IsSet(query.IsContent)
		})
		if hasPattern && allIsContent {
			return result.TypeFile
		}
	}

	if len(types) == 0 {
		return defaultTypes
	}

	var rts result.Types
	for _, t := range types {
		rts = rts.With(result.TypeFromString[t])
	}

	return rts
}

func toRepoOptions(b query.Basic, userSettings *schema.Settings) search.RepoOptions {
	repoFilters, minusRepoFilters := b.Repositories()

	var settingForks, settingArchived bool
	if v := userSettings.SearchIncludeForks; v != nil {
		settingForks = *v
	}
	if v := userSettings.SearchIncludeArchived; v != nil {
		settingArchived = *v
	}

	fork := query.No
	if searchrepos.ExactlyOneRepo(repoFilters) || settingForks {
		// fork defaults to No unless either of:
		// (1) exactly one repo is being searched, or
		// (2) user/org/global setting includes forks
		fork = query.Yes
	}
	if setFork := b.Fork(); setFork != nil {
		fork = *setFork
	}

	archived := query.No
	if searchrepos.ExactlyOneRepo(repoFilters) || settingArchived {
		// archived defaults to No unless either of:
		// (1) exactly one repo is being searched, or
		// (2) user/org/global setting includes archives in all searches
		archived = query.Yes
	}
	if setArchived := b.Archived(); setArchived != nil {
		archived = *setArchived
	}

	visibility := b.Visibility()
	searchContextSpec := b.FindValue(query.FieldContext)

	return search.RepoOptions{
		RepoFilters:         repoFilters,
		MinusRepoFilters:    minusRepoFilters,
		DescriptionPatterns: b.RepoHasDescription(),
		SearchContextSpec:   searchContextSpec,
		ForkSet:             b.Fork() != nil,
		OnlyForks:           fork == query.Only,
		NoForks:             fork == query.No,
		ArchivedSet:         b.Archived() != nil,
		OnlyArchived:        archived == query.Only,
		NoArchived:          archived == query.No,
		Visibility:          visibility,
		HasFileContent:      b.RepoHasFileContent(),
		CommitAfter:         b.RepoContainsCommitAfter(),
		UseIndex:            b.Index(),
		HasKVPs:             b.RepoHasKVPs(),
		HasTopics:           b.RepoHasTopics(),
	}
}

// jobBuilder represents computed static values that are backend agnostic: we
// generally need to compute these values before we're able to create (or build)
// multiple specific jobs. If you want to add new fields or state to run a
// search, ask yourself: is this value specific to a backend like Zoekt,
// searcher, or gitserver, or a new backend? If yes, then that new field does
// not belong in this builder type, and your new field should probably be
// computed either using values in this builder, or obtained from the outside
// world where you construct your specific search job.
//
// If you _may_ need the value available to start a search across differnt
// backends, then this builder type _may_ be the right place for it to live.
// If in doubt, ask the search team.
type jobBuilder struct {
	query           query.Basic
	patternType     query.SearchType
	resultTypes     result.Types
	repoOptions     search.RepoOptions
	features        *search.Features
	fileMatchLimit  int32
	selector        filter.SelectPath
	numContextLines int
}

func (b *jobBuilder) newZoektGlobalSearch(typ search.IndexedRequestType) (job.Job, error) {
	zoektQuery, err := zoekt.QueryToZoektQuery(b.query, b.resultTypes, b.features, typ)
	if err != nil {
		return nil, err
	}

	defaultScope, err := zoekt.DefaultGlobalQueryScope(b.repoOptions)
	if err != nil {
		return nil, err
	}

	includePrivate := b.repoOptions.Visibility == query.Private || b.repoOptions.Visibility == query.Any
	globalZoektQuery := zoekt.NewGlobalZoektQuery(zoektQuery, defaultScope, includePrivate)

	zoektParams := &search.ZoektParameters{
		// TODO(rvantonder): the Query value is set when the global zoekt query is
		// enriched with private repository data in the search job's Run method, and
		// is therefore set to `nil` below.
		// Ideally, The ZoektParameters type should not expose this field for Universe text
		// searches at all, and will be removed once jobs are fully migrated.
		Query:           nil,
		Typ:             typ,
		FileMatchLimit:  b.fileMatchLimit,
		Select:          b.selector,
		Features:        *b.features,
		PatternType:     b.patternType,
		NumContextLines: b.numContextLines,
	}

	switch typ {
	case search.SymbolRequest:
		return &zoekt.GlobalSymbolSearchJob{
			GlobalZoektQuery: globalZoektQuery,
			ZoektParams:      zoektParams,
			RepoOpts:         b.repoOptions,
		}, nil
	case search.TextRequest:
		return &zoekt.GlobalTextSearchJob{
			GlobalZoektQuery:        globalZoektQuery,
			ZoektParams:             zoektParams,
			RepoOpts:                b.repoOptions,
			GlobalZoektQueryRegexps: zoektQueryPatternsAsRegexps(globalZoektQuery.Query),
		}, nil
	}
	return nil, errors.Errorf("attempt to create unrecognized zoekt global search with value %v", typ)
}

func (b *jobBuilder) newZoektSearch(typ search.IndexedRequestType) (job.Job, error) {
	zoektQuery, err := zoekt.QueryToZoektQuery(b.query, b.resultTypes, b.features, typ)
	if err != nil {
		return nil, err
	}

	zoektParams := &search.ZoektParameters{
		FileMatchLimit:  b.fileMatchLimit,
		Select:          b.selector,
		Features:        *b.features,
		PatternType:     b.patternType,
		NumContextLines: b.numContextLines,
	}

	switch typ {
	case search.SymbolRequest:
		return &zoekt.SymbolSearchJob{
			Query:       zoektQuery,
			ZoektParams: zoektParams,
		}, nil
	case search.TextRequest:
		return &zoekt.RepoSubsetTextSearchJob{
			Query:             zoektQuery,
			ZoektQueryRegexps: zoektQueryPatternsAsRegexps(zoektQuery),
			Typ:               typ,
			ZoektParams:       zoektParams,
		}, nil
	}
	return nil, errors.Errorf("attempt to create unrecognized zoekt search with value %v", typ)
}

func zoektQueryPatternsAsRegexps(q zoektquery.Q) (res []*regexp.Regexp) {
	zoektquery.VisitAtoms(q, func(zoektQ zoektquery.Q) {
		switch typedQ := zoektQ.(type) {
		case *zoektquery.Regexp:
			if !typedQ.Content {
				if typedQ.CaseSensitive {
					res = append(res, regexp.MustCompile(typedQ.Regexp.String()))
				} else {
					res = append(res, regexp.MustCompile(`(?i)`+typedQ.Regexp.String()))
				}
			}
		case *zoektquery.Substring:
			if !typedQ.Content {
				if typedQ.CaseSensitive {
					res = append(res, regexp.MustCompile(regexp.QuoteMeta(typedQ.Pattern)))
				} else {
					res = append(res, regexp.MustCompile(`(?i)`+regexp.QuoteMeta(typedQ.Pattern)))
				}
			}
		}
	})
	return res
}

func jobMode(b query.Basic, repoOptions search.RepoOptions, resultTypes result.Types, inputs *search.Inputs) (repoUniverseSearch, skipRepoSubsetSearch, runZoektOverRepos bool) {
	// Exhaustive search avoids zoekt since it splits up a search in a worker
	// run per repo@revision.
	if inputs.Protocol == search.Exhaustive {
		repoUniverseSearch = false
		skipRepoSubsetSearch = false
		runZoektOverRepos = false
		return
	}

	isGlobalSearch := isGlobal(repoOptions) && inputs.PatternType != query.SearchTypeStructural

	hasGlobalSearchResultType := resultTypes.Has(result.TypeFile | result.TypePath | result.TypeSymbol)
	isIndexedSearch := b.Index() != query.No
	noPattern := b.IsEmptyPattern()
	noFile := !b.Exists(query.FieldFile)
	noLang := !b.Exists(query.FieldLang)
	isEmpty := noPattern && noFile && noLang

	repoUniverseSearch = isGlobalSearch && isIndexedSearch && hasGlobalSearchResultType && !isEmpty
	// skipRepoSubsetSearch is a value that controls whether to
	// run unindexed search in a specific scenario of queries that
	// contain no repo-affecting filters (global mode). When on
	// sourcegraph.com, we resolve only a subset of all indexed
	// repos to search. This control flow implies len(searcherRepos)
	// is always 0, meaning that we should not create jobs to run
	// unindexed searcher.
	skipRepoSubsetSearch = isEmpty || (repoUniverseSearch && inputs.OnSourcegraphDotCom)

	// runZoektOverRepos controls whether we run Zoekt over a set of
	// resolved repositories. Because Zoekt can run natively run over all
	// repositories (AKA global search), we can sometimes skip searching
	// over resolved repos.
	//
	// The decision to run over a set of repos is as follows:
	// (1) When we don't run global search, run Zoekt over repositories (we have to, otherwise
	// we'd be skipping indexed search entirely).
	// (2) If on Sourcegraph.com, resolve repos unconditionally (we run both global search
	// and search over resolved repos, and return results from either job).
	runZoektOverRepos = !repoUniverseSearch || inputs.OnSourcegraphDotCom

	return repoUniverseSearch, skipRepoSubsetSearch, runZoektOverRepos
}

// toAndJob creates a new job from a basic query whose pattern is an And operator at the root.
func toAndJob(inputs *search.Inputs, b query.Basic) (job.Job, error) {
	// Invariant: this function is only reachable from callers that
	// guarantee a root node with one or more queryOperands.
	queryOperands := b.Pattern.(query.Operator).Operands

	// Limit the number of results from each child to avoid a huge amount of memory bloat.
	// With streaming, we should re-evaluate this number.
	//
	// NOTE: It may be possible to page over repos so that each intersection is only over
	// a small set of repos, limiting massive number of results that would need to be
	// kept in memory otherwise.
	maxTryCount := 40000

	operands := make([]job.Job, 0, len(queryOperands))
	for _, queryOperand := range queryOperands {
		operand, err := toPatternExpressionJob(inputs, b.MapPattern(queryOperand))
		if err != nil {
			return nil, err
		}
		operands = append(operands, NewLimitJob(maxTryCount, operand))
	}

	return NewAndJob(operands...), nil
}

// toOrJob creates a new job from a basic query whose pattern is an Or operator at the top level
func toOrJob(inputs *search.Inputs, b query.Basic) (job.Job, error) {
	// Invariant: this function is only reachable from callers that
	// guarantee a root node with one or more queryOperands.
	queryOperands := b.Pattern.(query.Operator).Operands

	operands := make([]job.Job, 0, len(queryOperands))
	for _, term := range queryOperands {
		operand, err := toPatternExpressionJob(inputs, b.MapPattern(term))
		if err != nil {
			return nil, err
		}
		operands = append(operands, operand)
	}
	return NewOrJob(operands...), nil
}

func toPatternExpressionJob(inputs *search.Inputs, b query.Basic) (job.Job, error) {
	switch term := b.Pattern.(type) {
	case query.Operator:
		if len(term.Operands) == 0 {
			return NewNoopJob(), nil
		}

		switch term.Kind {
		case query.And:
			return toAndJob(inputs, b)
		case query.Or:
			return toOrJob(inputs, b)
		}
	case query.Pattern:
		return NewFlatJob(inputs, query.Flat{Parameters: b.Parameters, Pattern: &term})
	case query.Parameter:
		// evaluatePatternExpression does not process Parameter nodes.
		return NewNoopJob(), nil
	}
	// Unreachable.
	return nil, errors.Errorf("unrecognized type %T in evaluatePatternExpression", b.Pattern)
}

// toFlatJobs takes a query.Basic and expands it into a set query.Flat that are converted
// to jobs and joined with AndJob and OrJob.
func toFlatJobs(inputs *search.Inputs, b query.Basic) (job.Job, error) {
	if b.Pattern == nil {
		return NewFlatJob(inputs, query.Flat{Parameters: b.Parameters, Pattern: nil})
	} else {
		return toPatternExpressionJob(inputs, b)
	}
}

// isGlobal returns whether a given set of repo options can be fulfilled
// with a global search with Zoekt.
func isGlobal(op search.RepoOptions) bool {
	// We do not do global searches if a repo: filter was specified. I
	// (@camdencheek) could not find any documentation or historical reasons
	// for why this is, so I'm going to speculate here for future wanderers.
	//
	// If a user specifies a single repo, that repo may or may not be indexed
	// but we still want to search it. A Zoekt search will not tell us that a
	// search returned no results because the repo filtered to was unindexed,
	// it will just return no results.
	//
	// Additionally, if a user specifies a repo: filter, they are likely
	// targeting only a few repos, so the benefits of running a filtered global
	// search vs just paging over the few repos that match the query are
	// probably do not outweigh the cost of potentially skipping unindexed
	// repos.
	//
	// We see this assumption break down with filters like `repo:github.com/`
	// or `repo:.*`, in which case a global search would be much faster than
	// paging through all the repos.
	if len(op.RepoFilters) > 0 {
		return false
	}

	// Zoekt does not know about repo descriptions, so we depend on the
	// database to handle this filter.
	if len(op.DescriptionPatterns) > 0 {
		return false
	}

	// Zoekt does not know about repo key-value pairs or tags, so we depend on the
	// database to handle this filter.
	if len(op.HasKVPs) > 0 {
		return false
	}

	// Zoekt does not know about repo topics, so we depend on the database to
	// handle this filter.
	if len(op.HasTopics) > 0 {
		return false
	}

	// If a search context is specified, we do not know ahead of time whether
	// the repos in the context are indexed and we need to go through the repo
	// resolution process.
	if !searchcontexts.IsGlobalSearchContextSpec(op.SearchContextSpec) {
		return false
	}

	// repo:has.commit.after() is handled during the repo resolution step,
	// and we cannot depend on Zoekt for this information.
	if op.CommitAfter != nil {
		return false
	}

	// There should be no cursors when calling this, but if there are that
	// means we're already paginating. Cursors should probably not live on this
	// struct since they are an implementation detail of pagination.
	if len(op.Cursors) > 0 {
		return false
	}

	// If indexed search is explicitly disabled, that implicitly means global
	// search is also disabled since global search means Zoekt.
	if op.UseIndex == query.No {
		return false
	}

	// All the fields not mentioned above can be handled by Zoekt global search.
	// Listing them here for posterity:
	// - MinusRepoFilters
	// - CaseSensitiveRepoFilters
	// - HasFileContent
	// - Visibility
	// - Limit
	// - ForkSet
	// - NoForks
	// - OnlyForks
	// - OnlyCloned
	// - ArchivedSet
	// - NoArchived
	// - OnlyArchived
	return true
}
