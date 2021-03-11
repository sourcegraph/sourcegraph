package graphqlbackend

import (
	"context"
	"math"
	"regexp"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
)

var mockSearchRepositories func(args *search.TextParameters) ([]SearchResultResolver, *streaming.Stats, error)
var repoIcon = "data:image/svg+xml;base64,PHN2ZyB2ZXJzaW9uPSIxLjEiIGlkPSJMYXllcl8xIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHhtbG5zOnhsaW5rPSJodHRwOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIiB4PSIwcHgiIHk9IjBweCIKCSB2aWV3Qm94PSIwIDAgNjQgNjQiIHN0eWxlPSJlbmFibGUtYmFja2dyb3VuZDpuZXcgMCAwIDY0IDY0OyIgeG1sOnNwYWNlPSJwcmVzZXJ2ZSI+Cjx0aXRsZT5JY29ucyA0MDA8L3RpdGxlPgo8Zz4KCTxwYXRoIGQ9Ik0yMywyMi40YzEuMywwLDIuNC0xLjEsMi40LTIuNHMtMS4xLTIuNC0yLjQtMi40Yy0xLjMsMC0yLjQsMS4xLTIuNCwyLjRTMjEuNywyMi40LDIzLDIyLjR6Ii8+Cgk8cGF0aCBkPSJNMzUsMjYuNGMxLjMsMCwyLjQtMS4xLDIuNC0yLjRzLTEuMS0yLjQtMi40LTIuNHMtMi40LDEuMS0yLjQsMi40UzMzLjcsMjYuNCwzNSwyNi40eiIvPgoJPHBhdGggZD0iTTIzLDQyLjRjMS4zLDAsMi40LTEuMSwyLjQtMi40cy0xLjEtMi40LTIuNC0yLjRzLTIuNCwxLjEtMi40LDIuNFMyMS43LDQyLjQsMjMsNDIuNHoiLz4KCTxwYXRoIGQ9Ik01MCwxNmgtMS41Yy0wLjMsMC0wLjUsMC4yLTAuNSwwLjV2MzVjMCwwLjMtMC4yLDAuNS0wLjUsMC41aC0yN2MtMC41LDAtMS0wLjItMS40LTAuNmwtMC42LTAuNmMtMC4xLTAuMS0wLjEtMC4yLTAuMS0wLjQKCQljMC0wLjMsMC4yLTAuNSwwLjUtMC41SDQ0YzEuMSwwLDItMC45LDItMlYxMmMwLTEuMS0wLjktMi0yLTJIMTRjLTEuMSwwLTIsMC45LTIsMnYzNi4zYzAsMS4xLDAuNCwyLjEsMS4yLDIuOGwzLjEsMy4xCgkJYzEuMSwxLjEsMi43LDEuOCw0LjIsMS44SDUwYzEuMSwwLDItMC45LDItMlYxOEM1MiwxNi45LDUxLjEsMTYsNTAsMTZ6IE0xOSwyMGMwLTIuMiwxLjgtNCw0LTRjMS40LDAsMi44LDAuOCwzLjUsMgoJCWMxLjEsMS45LDAuNCw0LjMtMS41LDUuNFYzM2MxLTAuNiwyLjMtMC45LDQtMC45YzEsMCwyLTAuNSwyLjgtMS4zQzMyLjUsMzAsMzMsMjkuMSwzMywyOHYtMC42Yy0xLjItMC43LTItMi0yLTMuNQoJCWMwLTIuMiwxLjgtNCw0LTRjMi4yLDAsNCwxLjgsNCw0YzAsMS41LTAuOCwyLjctMiwzLjVoMGMtMC4xLDIuMS0wLjksNC40LTIuNSw2Yy0xLjYsMS42LTMuNCwyLjQtNS41LDIuNWMtMC44LDAtMS40LDAuMS0xLjksMC4zCgkJYy0wLjIsMC4xLTEsMC44LTEuMiwwLjlDMjYuNiwzOCwyNywzOC45LDI3LDQwYzAsMi4yLTEuOCw0LTQsNHMtNC0xLjgtNC00YzAtMS41LDAuOC0yLjcsMi0zLjRWMjMuNEMxOS44LDIyLjcsMTksMjEuNCwxOSwyMHoiLz4KPC9nPgo8L3N2Zz4K"

// searchRepositories searches for repositories by name.
//
// For a repository to match a query, the repository's name must match all of the repo: patterns AND the
// default patterns (i.e., the patterns that are not prefixed with any search field).
func searchRepositories(ctx context.Context, db dbutil.DB, args *search.TextParameters, limit int32, stream Sender) error {
	if mockSearchRepositories != nil {
		results, stats, err := mockSearchRepositories(args)
		stream.Send(SearchEvent{
			Results: results,
			Stats:   statsDeref(stats),
		})
		return err
	}

	ctx, stream, cancel := WithLimit(ctx, stream, int(limit))
	defer cancel()

	fieldAllowlist := map[string]struct{}{
		query.FieldRepo:               {},
		query.FieldRepoGroup:          {},
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
	// Don't return repo results if the search contains fields that aren't on the allowlist.
	// Matching repositories based whether they contain files at a certain path (etc.) is not yet implemented.
	for field := range args.Query.Fields() {
		if _, ok := fieldAllowlist[field]; !ok {
			return nil
		}
	}

	patternRe := args.PatternInfo.Pattern
	if !args.Query.IsCaseSensitive() {
		patternRe = "(?i)" + patternRe
	}

	pattern, err := regexp.Compile(patternRe)
	if err != nil {
		return err
	}

	// Filter args.Repos by matching their names against the query pattern.
	resolved, err := getRepos(ctx, args.RepoPromise)
	if err != nil {
		return err
	}

	results := make(chan []*search.RepositoryRevisions)
	go func() {
		defer close(results)
		matchRepos(pattern, resolved, results)
	}()

	// Filter the repos if there is a repohasfile: or -repohasfile field.
	if len(args.PatternInfo.FilePatternsReposMustExclude) > 0 || len(args.PatternInfo.FilePatternsReposMustInclude) > 0 {
		// Fallback to batch for reposToAdd
		var repos []*search.RepositoryRevisions
		for matched := range results {
			repos = append(repos, matched...)
		}
		repos, err = reposToAdd(ctx, db, args, repos)
		if err != nil {
			return err
		}
		stream.Send(SearchEvent{
			Results: repoRevsToSearchResultResolver(ctx, db, repos),
		})
		return nil
	}

	for repos := range results {
		stream.Send(SearchEvent{
			Results: repoRevsToSearchResultResolver(ctx, db, repos),
		})
	}

	return nil
}

func repoRevsToSearchResultResolver(ctx context.Context, db dbutil.DB, repos []*search.RepositoryRevisions) []SearchResultResolver {
	results := make([]SearchResultResolver, 0, len(repos))
	for _, r := range repos {
		revs, err := r.ExpandedRevSpecs(ctx)
		if err != nil { // fallback to just return revspecs
			revs = r.RevSpecs()
		}
		for _, rev := range revs {
			rr := NewRepositoryResolver(db, r.Repo.ToRepo())
			rr.icon = repoIcon
			rr.rev = rev
			results = append(results, rr)
		}
	}
	return results
}

func matchRepos(pattern *regexp.Regexp, resolved []*search.RepositoryRevisions, results chan<- []*search.RepositoryRevisions) {
	/*
		Local benchmarks showed diminishing returns for higher levels of concurrency.
		5 workers seems to be a good trade-off for now. We might want to revisit this
		benchmark over time.

		go test -cpu 1,2,3,4,5,6,7,8,9,10 -count=5 -bench=SearchRepo .

		   goos: darwin
		   goarch: amd64
		   pkg: github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend
		   BenchmarkSearchRepositories       	      13	 132088878 ns/op
		   BenchmarkSearchRepositories-2     	      16	  69968357 ns/op
		   BenchmarkSearchRepositories-3     	      24	  48294832 ns/op
		   BenchmarkSearchRepositories-4     	      28	  42497674 ns/op
		   BenchmarkSearchRepositories-5     	      27	  42851670 ns/op
		   BenchmarkSearchRepositories-6     	      28	  39327860 ns/op
		   BenchmarkSearchRepositories-7     	      27	  38198665 ns/op
		   BenchmarkSearchRepositories-8     	      28	  38877182 ns/op
		   BenchmarkSearchRepositories-9     	      26	  42457771 ns/op
		   BenchmarkSearchRepositories-10    	      26	  40519692 ns/op
	*/
	step := len(resolved) / 5 // for benchmarking, replace 5 with runtime.GOMAXPROCS(0)
	if step == 0 {
		step = len(resolved)
	} else {
		step += 1
	}

	var wg sync.WaitGroup
	offset := 0
	for offset < len(resolved) {
		next := offset + step
		if next > len(resolved) {
			next = len(resolved)
		}
		wg.Add(1)
		go func(repos []*search.RepositoryRevisions) {
			defer wg.Done()

			var matched []*search.RepositoryRevisions
			for _, r := range repos {
				if pattern.MatchString(string(r.Repo.Name)) {
					matched = append(matched, r)
				}
			}
			if len(matched) > 0 {
				results <- matched
			}
		}(resolved[offset:next])
		offset = next
	}

	wg.Wait()
}

// reposToAdd determines which repositories should be included in the result set based on whether they fit in the subset
// of repostiories specified in the query's `repohasfile` and `-repohasfile` fields if they exist.
func reposToAdd(ctx context.Context, db dbutil.DB, args *search.TextParameters, repos []*search.RepositoryRevisions) ([]*search.RepositoryRevisions, error) {
	matchingIDs := make(map[api.RepoID]bool)
	if len(args.PatternInfo.FilePatternsReposMustInclude) > 0 {
		for _, pattern := range args.PatternInfo.FilePatternsReposMustInclude {
			// The high FileMatchLimit here is to make sure we get all the repo matches we can. Setting it to
			// len(repos) could mean we miss some repos since there could be for example len(repos) file matches in
			// the first repo and some more in other repos.
			p := search.TextPatternInfo{IsRegExp: true, FileMatchLimit: math.MaxInt32, IncludePatterns: []string{pattern}, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
			q, err := query.ParseLiteral("file:" + pattern)
			if err != nil {
				return nil, err
			}
			newArgs := *args
			newArgs.PatternInfo = &p
			newArgs.RepoPromise = (&search.Promise{}).Resolve(repos)
			newArgs.Query = q
			newArgs.UseFullDeadline = true
			matches, _, err := searchFilesInReposBatch(ctx, db, &newArgs)
			if err != nil {
				return nil, err
			}
			for _, m := range matches {
				matchingIDs[m.Repo.ID] = true
			}
		}
	} else {
		// Default to including all the repos, then excluding some of them below.
		for _, r := range repos {
			matchingIDs[r.Repo.ID] = true
		}
	}

	if len(args.PatternInfo.FilePatternsReposMustExclude) > 0 {
		for _, pattern := range args.PatternInfo.FilePatternsReposMustExclude {
			p := search.TextPatternInfo{IsRegExp: true, FileMatchLimit: math.MaxInt32, IncludePatterns: []string{pattern}, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
			q, err := query.ParseLiteral("file:" + pattern)
			if err != nil {
				return nil, err
			}
			newArgs := *args
			newArgs.PatternInfo = &p
			rp := (&search.Promise{}).Resolve(repos)
			newArgs.RepoPromise = rp
			newArgs.Query = q
			newArgs.UseFullDeadline = true
			matches, _, err := searchFilesInReposBatch(ctx, db, &newArgs)
			if err != nil {
				return nil, err
			}
			for _, m := range matches {
				matchingIDs[m.Repo.ID] = false
			}
		}
	}

	var rsta []*search.RepositoryRevisions
	for _, r := range repos {
		if matchingIDs[r.Repo.ID] {
			rsta = append(rsta, r)
		}
	}

	return rsta, nil
}
