package graphqlbackend

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"

	otlog "github.com/opentracing/opentracing-go/log"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/mutablelimiter"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

const maxUnindexedRepoRevSearchesPerQuery = 200

// A global limiter on number of concurrent searcher searches.
var textSearchLimiter = mutablelimiter.New(32)

// FileMatchResolver is a resolver for the GraphQL type `FileMatch`
type FileMatchResolver struct {
	result.FileMatch

	RepoResolver *RepositoryResolver
	db           dbutil.DB
}

// Equal provides custom comparison which is used by go-cmp
func (fm *FileMatchResolver) Equal(other *FileMatchResolver) bool {
	return reflect.DeepEqual(fm, other)
}

func (fm *FileMatchResolver) Key() string {
	return fm.URI
}

func (fm *FileMatchResolver) File() *GitTreeEntryResolver {
	// NOTE(sqs): Omits other commit fields to avoid needing to fetch them
	// (which would make it slow). This GitCommitResolver will return empty
	// values for all other fields.
	return &GitTreeEntryResolver{
		db:     fm.db,
		commit: fm.Commit(),
		stat:   CreateFileInfo(fm.Path, false),
	}
}

func (fm *FileMatchResolver) Commit() *GitCommitResolver {
	return &GitCommitResolver{
		db:           fm.db,
		repoResolver: fm.RepoResolver,
		oid:          GitObjectID(fm.CommitID),
		inputRev:     fm.InputRev,
	}
}

func (fm *FileMatchResolver) Repository() *RepositoryResolver {
	return fm.RepoResolver
}

func (fm *FileMatchResolver) RevSpec() *gitRevSpec {
	if fm.InputRev == nil || *fm.InputRev == "" {
		return nil // default branch
	}
	return &gitRevSpec{
		expr: &gitRevSpecExpr{expr: *fm.InputRev, repo: fm.Repository()},
	}
}

func (fm *FileMatchResolver) Resource() string {
	return fm.URI
}

func (fm *FileMatchResolver) Symbols() []symbolResolver {
	commit := fm.Commit()
	symbols := make([]symbolResolver, len(fm.FileMatch.Symbols))
	for i, s := range fm.FileMatch.Symbols {
		symbols[i] = toSymbolResolver(fm.db, commit, s)
	}
	return symbols
}

func (fm *FileMatchResolver) LineMatches() []lineMatchResolver {
	r := make([]lineMatchResolver, 0, len(fm.FileMatch.LineMatches))
	for _, lm := range fm.FileMatch.LineMatches {
		r = append(r, lineMatchResolver{lm})
	}
	return r
}

func (fm *FileMatchResolver) LimitHit() bool {
	return fm.FileMatch.LimitHit
}

func (fm *FileMatchResolver) ToRepository() (*RepositoryResolver, bool) { return nil, false }
func (fm *FileMatchResolver) ToFileMatch() (*FileMatchResolver, bool)   { return fm, true }
func (fm *FileMatchResolver) ToCommitSearchResult() (*CommitSearchResultResolver, bool) {
	return nil, false
}

// path returns the path in repository for the file. This isn't directly
// exposed in the GraphQL API (we expose a URI), but is used a lot internally.
func (fm *FileMatchResolver) path() string {
	return fm.Path
}

// appendMatches appends the line matches from src as well as updating match
// counts and limit.
func (fm *FileMatchResolver) appendMatches(src *FileMatchResolver) {
	fm.FileMatch.AppendMatches(&src.FileMatch)
}

func (fm *FileMatchResolver) ResultCount() int32 {
	return int32(fm.FileMatch.ResultCount())
}

func (fm *FileMatchResolver) Select(t filter.SelectPath) SearchResultResolver {
	switch t.Type {
	case filter.Repository:
		return fm.Repository()
	case filter.File:
		fm.FileMatch.LineMatches = nil
		fm.FileMatch.Symbols = nil
		return fm
	case filter.Symbol:
		if len(fm.FileMatch.Symbols) > 0 {
			fm.FileMatch.LineMatches = nil // Only return symbol match if symbols exist
			if len(t.Fields) > 0 {
				filteredSymbols := filter.SelectSymbolKind(fm.FileMatch.Symbols, t.Fields[0])
				if len(filteredSymbols) == 0 {
					return nil // Remove file match if there are no symbol results after filtering
				}
				fm.FileMatch.Symbols = filteredSymbols
			}
			return fm
		}
		return nil
	case filter.Content:
		// Only return file match if line matches exist
		if len(fm.FileMatch.LineMatches) > 0 {
			fm.FileMatch.Symbols = nil
			return fm
		}
		return nil
	case filter.Commit:
		return nil
	}
	return nil
}

type lineMatchResolver struct {
	*result.LineMatch
}

func (lm lineMatchResolver) Preview() string {
	return lm.LineMatch.Preview
}

func (lm lineMatchResolver) LineNumber() int32 {
	return lm.LineMatch.LineNumber
}

func (lm lineMatchResolver) OffsetAndLengths() [][]int32 {
	r := make([][]int32, len(lm.LineMatch.OffsetAndLengths))
	for i := range lm.LineMatch.OffsetAndLengths {
		r[i] = lm.LineMatch.OffsetAndLengths[i][:]
	}
	return r
}

func (lm lineMatchResolver) LimitHit() bool {
	return lm.LineMatch.LimitHit
}

var mockSearchFilesInRepo func(ctx context.Context, repo *types.RepoName, gitserverRepo api.RepoName, rev string, info *search.TextPatternInfo, fetchTimeout time.Duration) (matches []*FileMatchResolver, limitHit bool, err error)

func searchFilesInRepo(ctx context.Context, db dbutil.DB, searcherURLs *endpoint.Map, repo *types.RepoName, gitserverRepo api.RepoName, rev string, index bool, info *search.TextPatternInfo, fetchTimeout time.Duration) ([]*FileMatchResolver, bool, error) {
	if mockSearchFilesInRepo != nil {
		return mockSearchFilesInRepo(ctx, repo, gitserverRepo, rev, info, fetchTimeout)
	}

	// Do not trigger a repo-updater lookup (e.g.,
	// backend.{GitRepo,Repos.ResolveRev}) because that would slow this operation
	// down by a lot (if we're looping over many repos). This means that it'll fail if a
	// repo is not on gitserver.
	commit, err := git.ResolveRevision(ctx, gitserverRepo, rev, git.ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return nil, false, err
	}

	shouldBeSearched, err := repoShouldBeSearched(ctx, searcherURLs, info, gitserverRepo, commit, fetchTimeout)
	if err != nil {
		return nil, false, err
	}
	if !shouldBeSearched {
		return nil, false, err
	}

	var indexerEndpoints []string
	if info.IsStructuralPat {
		endpoints, err := search.Indexers().Map.Endpoints()
		for key := range endpoints {
			indexerEndpoints = append(indexerEndpoints, key)
		}
		if err != nil {
			return nil, false, err
		}
	}
	matches, limitHit, err := searcher.Search(ctx, searcherURLs, gitserverRepo, rev, commit, index, info, fetchTimeout, indexerEndpoints)
	if err != nil {
		return nil, false, err
	}

	workspace := fileMatchURI(repo.Name, rev, "")
	repoResolver := NewRepositoryResolver(db, repo.ToRepo())
	resolvers := make([]*FileMatchResolver, 0, len(matches))
	for _, fm := range matches {
		lineMatches := make([]*result.LineMatch, 0, len(fm.LineMatches))
		for _, lm := range fm.LineMatches {
			ranges := make([][2]int32, 0, len(lm.OffsetAndLengths))
			for _, ol := range lm.OffsetAndLengths {
				ranges = append(ranges, [2]int32{int32(ol[0]), int32(ol[1])})
			}
			lineMatches = append(lineMatches, &result.LineMatch{
				Preview:          lm.Preview,
				OffsetAndLengths: ranges,
				LineNumber:       int32(lm.LineNumber),
				LimitHit:         lm.LimitHit,
			})
		}

		resolvers = append(resolvers, &FileMatchResolver{
			db: db,
			FileMatch: result.FileMatch{
				Path:        fm.Path,
				LineMatches: lineMatches,
				LimitHit:    fm.LimitHit,
				Repo:        repo,
				URI:         workspace + fm.Path,
				CommitID:    commit,
				InputRev:    &rev,
			},
			RepoResolver: repoResolver,
		})
	}

	return resolvers, limitHit, err
}

// repoShouldBeSearched determines whether a repository should be searched in, based on whether the repository
// fits in the subset of repositories specified in the query's `repohasfile` and `-repohasfile` flags if they exist.
func repoShouldBeSearched(ctx context.Context, searcherURLs *endpoint.Map, searchPattern *search.TextPatternInfo, gitserverRepo api.RepoName, commit api.CommitID, fetchTimeout time.Duration) (shouldBeSearched bool, err error) {
	shouldBeSearched = true
	flagInQuery := len(searchPattern.FilePatternsReposMustInclude) > 0
	if flagInQuery {
		shouldBeSearched, err = repoHasFilesWithNamesMatching(ctx, searcherURLs, true, searchPattern.FilePatternsReposMustInclude, gitserverRepo, commit, fetchTimeout)
		if err != nil {
			return shouldBeSearched, err
		}
	}
	negFlagInQuery := len(searchPattern.FilePatternsReposMustExclude) > 0
	if negFlagInQuery {
		shouldBeSearched, err = repoHasFilesWithNamesMatching(ctx, searcherURLs, false, searchPattern.FilePatternsReposMustExclude, gitserverRepo, commit, fetchTimeout)
		if err != nil {
			return shouldBeSearched, err
		}
	}
	return shouldBeSearched, nil
}

// repoHasFilesWithNamesMatching searches in a repository for matches for the patterns in the `repohasfile` or `-repohasfile` flags, and returns
// whether or not the repoShouldBeSearched in or not, based on whether matches were returned.
func repoHasFilesWithNamesMatching(ctx context.Context, searcherURLs *endpoint.Map, include bool, repoHasFileFlag []string, gitserverRepo api.RepoName, commit api.CommitID, fetchTimeout time.Duration) (bool, error) {
	for _, pattern := range repoHasFileFlag {
		p := search.TextPatternInfo{IsRegExp: true, FileMatchLimit: 1, IncludePatterns: []string{pattern}, PathPatternsAreCaseSensitive: false, PatternMatchesContent: true, PatternMatchesPath: true}
		matches, _, err := searcher.Search(ctx, searcherURLs, gitserverRepo, "", commit, false, &p, fetchTimeout, []string{})
		if err != nil {
			return false, err
		}
		if include && len(matches) == 0 || !include && len(matches) > 0 {
			// repo shouldn't be searched if it does not have matches for the patterns in `repohasfile`
			// or if it has file matches for the patterns in `-repohasfile`.
			return false, nil
		}
	}

	return true, nil
}

func fileMatchURI(name api.RepoName, ref, path string) string {
	var b strings.Builder
	ref = url.QueryEscape(ref)
	b.Grow(len(name) + len(ref) + len(path) + len("git://?#"))
	b.WriteString("git://")
	b.WriteString(string(name))
	if ref != "" {
		b.WriteByte('?')
		b.WriteString(ref)
	}
	b.WriteByte('#')
	b.WriteString(path)
	return b.String()
}

var mockSearchFilesInRepos func(args *search.TextParameters) ([]*FileMatchResolver, *streaming.Stats, error)

func fileMatchResultsToSearchResults(results []*FileMatchResolver) []SearchResultResolver {
	results2 := make([]SearchResultResolver, len(results))
	for i, result := range results {
		results2[i] = result
	}
	return results2
}

func searchResultsToFileMatchResults(results []SearchResultResolver) ([]*FileMatchResolver, error) {
	results2 := make([]*FileMatchResolver, len(results))
	for i, result := range results {
		fm, ok := result.ToFileMatch()
		if !ok {
			return nil, fmt.Errorf("expected only file match results")
		}
		results2[i] = fm
	}
	return results2, nil
}

// searchFilesInRepoBatch is a convenience function around searchFilesInRepos
// which collects the results from the stream.
func searchFilesInReposBatch(ctx context.Context, db dbutil.DB, args *search.TextParameters) ([]*FileMatchResolver, streaming.Stats, error) {
	results, stats, err := collectStream(func(stream Sender) error {
		return searchFilesInRepos(ctx, db, args, stream)
	})
	fms, fmErr := searchResultsToFileMatchResults(results)
	if fmErr != nil && err == nil {
		err = errors.Wrap(fmErr, "searchFilesInReposBatch failed to convert results")
	}
	return fms, stats, err
}

// searchFilesInRepos searches a set of repos for a pattern.
func searchFilesInRepos(ctx context.Context, db dbutil.DB, args *search.TextParameters, stream Sender) (err error) {
	if mockSearchFilesInRepos != nil {
		results, mockStats, err := mockSearchFilesInRepos(args)
		stream.Send(SearchEvent{
			Results: fileMatchResultsToSearchResults(results),
			Stats:   statsDeref(mockStats),
		})
		return err
	}

	ctx, stream, cleanup := WithLimit(ctx, stream, int(args.PatternInfo.FileMatchLimit))
	defer cleanup()

	tr, ctx := trace.New(ctx, "searchFilesInRepos", fmt.Sprintf("query: %s", args.PatternInfo.Pattern))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	tr.LogFields(
		trace.Stringer("query", args.Query),
		trace.Stringer("info", args.PatternInfo),
		trace.Stringer("global_search_mode", args.Mode),
	)

	indexedTyp := textRequest
	if args.PatternInfo.IsStructuralPat {
		// Structural Patterns queries zoekt just file files to reduce the set
		// of files it searches.
		indexedTyp = fileRequest
	}

	// performance: for global searches, we avoid calling newIndexedSearchRequest
	// because zoekt will anyway have to search all its shards.
	var indexed *indexedSearchRequest
	if args.Mode == search.ZoektGlobalSearch {
		indexed = &indexedSearchRequest{
			args:  args,
			typ:   indexedTyp,
			repos: &indexedRepoRevs{},
		}
	} else {
		indexed, err = newIndexedSearchRequest(ctx, db, args, indexedTyp, stream)
		if err != nil {
			return err
		}
	}

	if args.PatternInfo.IsEmpty() {
		// Empty query isn't an error, but it has no results.
		return nil
	}

	// Indexed regex and literal search go via zoekt
	isIndexedTextSearch := args.Mode != search.SearcherOnly && !args.PatternInfo.IsStructuralPat
	// Structural search goes via zoekt then searcher.
	isStructuralSearch := args.Mode != search.SearcherOnly && args.PatternInfo.IsStructuralPat

	g, ctx := errgroup.WithContext(ctx)

	if isIndexedTextSearch {
		g.Go(func() error {
			return indexed.Search(ctx, stream)
		})
	}

	if isStructuralSearch && args.PatternInfo.CombyRule != `where "backcompat" == "backcompat"` {
		g.Go(func() error {
			repos := make([]*search.RepositoryRevisions, 0, len(indexed.Repos()))
			for _, repo := range indexed.Repos() {
				repos = append(repos, repo)
			}

			return callSearcherOverRepos(ctx, db, args, stream, repos, nil, true)
		})

		g.Go(func() error {
			return callSearcherOverRepos(ctx, db, args, stream, indexed.Unindexed, nil, false)
		})
	} else if isStructuralSearch {
		g.Go(func() error {
			return structuralSearchBackcompat(ctx, db, args, stream, indexed)
		})
	}

	// This guard disables
	// - unindexed structural search
	// - unindexed search of negated content
	if !args.PatternInfo.IsStructuralPat {
		g.Go(func() error {
			return callSearcherOverRepos(ctx, db, args, stream, indexed.Unindexed, nil, false)
		})
	}

	return g.Wait()
}

// callSearcherOverRepos calls searcher on searcherRepos.
//
// searcherReposFilteredFiles is an optional map of {repo name => file list}
// that forces the searcher to only include the file list in the search. It is
// currently only set when Zoekt restricts the file list for structural
// search.
func callSearcherOverRepos(
	ctx context.Context,
	db dbutil.DB,
	args *search.TextParameters,
	stream Sender,
	searcherRepos []*search.RepositoryRevisions,
	searcherReposFilteredFiles map[string][]string,
	index bool,
) (err error) {
	tr, ctx := trace.New(ctx, "searcherOverRepos", fmt.Sprintf("query: %s", args.PatternInfo.Pattern))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	var fetchTimeout time.Duration
	if len(searcherRepos) == 1 || args.UseFullDeadline {
		// When searching a single repo or when an explicit timeout was specified, give it the remaining deadline to fetch the archive.
		deadline, ok := ctx.Deadline()
		if ok {
			fetchTimeout = time.Until(deadline)
		} else {
			// In practice, this case should not happen because a deadline should always be set
			// but if it does happen just set a long but finite timeout.
			fetchTimeout = time.Minute
		}
	} else {
		// When searching many repos, don't wait long for any single repo to fetch.
		fetchTimeout = 500 * time.Millisecond
	}

	tr.LogFields(
		otlog.Int64("fetch_timeout_ms", fetchTimeout.Milliseconds()),
		otlog.Int64("repos_count", int64(len(searcherRepos))),
		otlog.Int64("repos_filtered_files_count", int64(len(searcherReposFilteredFiles))),
	)

	if len(searcherRepos) == 0 {
		return nil
	}

	// The number of searcher endpoints can change over time. Inform our
	// limiter of the new limit, which is a multiple of the number of
	// searchers.
	eps, err := args.SearcherURLs.Endpoints()
	if err != nil {
		return err
	}
	textSearchLimiter.SetLimit(len(eps) * 32)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for _, repoAllRevs := range searcherRepos {
			if len(repoAllRevs.Revs) == 0 {
				continue
			}

			revSpecs, err := repoAllRevs.ExpandedRevSpecs(ctx)
			if err != nil {
				return err
			}

			for _, rev := range revSpecs {
				limitCtx, limitDone, err := textSearchLimiter.Acquire(ctx)
				if err != nil {
					return err
				}

				// Make a new repoRev for just the operation of searching this revspec.
				repoRev := &search.RepositoryRevisions{Repo: repoAllRevs.Repo, Revs: []search.RevisionSpecifier{{RevSpec: rev}}}

				args := *args
				if args.PatternInfo.IsStructuralPat && searcherReposFilteredFiles != nil {
					// Modify the search query to only run for the filtered files
					if v, ok := searcherReposFilteredFiles[string(repoRev.Repo.Name)]; ok {
						patternCopy := *args.PatternInfo
						args.PatternInfo = &patternCopy
						includePatternsCopy := []string{}
						args.PatternInfo.IncludePatterns = append(includePatternsCopy, v...)
					}
				}

				g.Go(func() error {
					ctx, done := limitCtx, limitDone
					defer done()

					matches, repoLimitHit, err := searchFilesInRepo(ctx, db, args.SearcherURLs, repoRev.Repo, repoRev.GitserverRepo(), repoRev.RevSpecs()[0], index, args.PatternInfo, fetchTimeout)
					if err != nil {
						tr.LogFields(otlog.String("repo", string(repoRev.Repo.Name)), otlog.Error(err), otlog.Bool("timeout", errcode.IsTimeout(err)), otlog.Bool("temporary", errcode.IsTemporary(err)))
						log15.Warn("searchFilesInRepo failed", "error", err, "repo", repoRev.Repo.Name)
					}
					// non-diff search reports timeout through err, so pass false for timedOut
					stats, err := handleRepoSearchResult(repoRev, repoLimitHit, false, err)
					stream.Send(SearchEvent{
						Results: fileMatchResultsToSearchResults(matches),
						Stats:   stats,
					})
					return err
				})
			}
		}

		return nil
	})

	return g.Wait()
}

// structuralSearchBackcompat is the old way we did structural search. It runs
// a query through zoekt first to get back a list of filepaths to search. Then
// calls searcher limiting it to just those files.
func structuralSearchBackcompat(ctx context.Context, db dbutil.DB, args *search.TextParameters, stream Sender, indexed *indexedSearchRequest) (err error) {
	tr, ctx := trace.New(ctx, "structuralSearchBackcompt", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	g, ctx := errgroup.WithContext(ctx)

	err = indexed.Search(ctx, StreamFunc(func(event SearchEvent) {
		g.Go(func() error {
			tr.LogFields(otlog.Int("matches.len", len(event.Results)))
			stream.Send(SearchEvent{
				Stats: event.Stats,
			})

			if len(event.Results) == 0 {
				return nil
			}

			// For structural search, we run callSearcherOverRepos
			// over the set of repos and files known to contain
			// parts of the pattern as determined by Zoekt.

			// A partition of {repo name => file list} that we will build from Zoekt matches
			partition := make(map[string][]string)
			var repos []*search.RepositoryRevisions

			for _, m := range event.Results {
				fm, ok := m.ToFileMatch()
				if !ok {
					return errors.New("structual search: Events from indexed.Search could not be converted to FileMatch")
				}
				name := string(fm.Repo.Name)
				partition[name] = append(partition[name], fm.Path)
			}

			// Filter Zoekt repos that didn't contain matches
			for _, repo := range indexed.Repos() {
				if _, ok := partition[string(repo.Repo.Name)]; ok {
					repos = append(repos, repo)
				}
			}

			return callSearcherOverRepos(ctx, db, args, stream, repos, partition, true)
		})
	}))

	if gErr := g.Wait(); gErr != nil {
		err = gErr
	}
	return err
}

// limitSearcherRepos limits the number of repo@revs searched by the unindexed searcher codepath.
// Sending many requests to searcher would otherwise cause a flood of system and network requests
// that result in timeouts or long delays.
//
// It returns the new repositories destined for the unindexed searcher code path, and the
// repositories that are limited / excluded.
//
// A slice to the input list is returned, it is not copied.
func limitSearcherRepos(unindexed []*search.RepositoryRevisions, limit int) (searcherRepos []*search.RepositoryRevisions, limitedSearcherRepos []*types.RepoName) {
	totalRepoRevs := 0
	limitedRepos := 0
	for _, repoRevs := range unindexed {
		totalRepoRevs += len(repoRevs.Revs)
		if totalRepoRevs > limit {
			limitedSearcherRepos = append(limitedSearcherRepos, repoRevs.Repo)
			limitedRepos++
		}
	}
	searcherRepos = unindexed[:len(unindexed)-limitedRepos]
	return
}
