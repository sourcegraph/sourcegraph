package graphqlbackend

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"

	"github.com/google/zoekt"
	zoektquery "github.com/google/zoekt/query"
	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gituri"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type indexedRequestType string

const (
	textRequest   indexedRequestType = "text"
	symbolRequest indexedRequestType = "symbol"
	fileRequest   indexedRequestType = "file"
)

// indexedSearchRequest is responsible for translating a Sourcegraph search
// query into a Zoekt query and mapping the results from zoekt back to
// Sourcegraph result types.
type indexedSearchRequest struct {
	// Unindexed is a slice of repository revisions that can't be searched by
	// Zoekt. The repository revisions should be searched by the searcher
	// service.
	//
	// If IndexUnavailable is true or the query specifies index:no then all
	// repository revisions will be listed. Otherwise it will just be
	// repository revisions not indexed.
	Unindexed []*search.RepositoryRevisions

	// IndexUnavailable is true if zoekt is offline or disabled.
	IndexUnavailable bool

	// DisableUnindexedSearch is true if the query specified that only index
	// search should be used.
	DisableUnindexedSearch bool

	// inputs
	args *search.TextParameters
	typ  indexedRequestType

	// repos is the repository revisions that are indexed and will be
	// searched.
	repos *indexedRepoRevs

	// since if non-nil will be used instead of time.Since. For tests
	since func(time.Time) time.Duration

	db dbutil.DB
}

func newIndexedSearchRequest(ctx context.Context, db dbutil.DB, args *search.TextParameters, typ indexedRequestType, stream Sender) (_ *indexedSearchRequest, err error) {
	tr, ctx := trace.New(ctx, "newIndexedSearchRequest", string(typ))
	tr.LogFields(trace.Stringer("global_search_mode", args.Mode))
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()
	repos, err := getRepos(ctx, args.RepoPromise)
	if err != nil {
		return nil, err
	}

	// If Zoekt is disabled just fallback to Unindexed.
	if !args.Zoekt.Enabled() {
		if args.PatternInfo.Index == query.Only {
			return nil, fmt.Errorf("invalid index:%q (indexed search is not enabled)", args.PatternInfo.Index)
		}

		return &indexedSearchRequest{
			db:               db,
			Unindexed:        limitUnindexedRepos(repos, maxUnindexedRepoRevSearchesPerQuery, stream),
			IndexUnavailable: true,
		}, nil
	}

	// Fallback to Unindexed if the query contains ref-globs
	if query.ContainsRefGlobs(args.Query) {
		if args.PatternInfo.Index == query.Only {
			return nil, fmt.Errorf("invalid index:%q (revsions with glob pattern cannot be resolved for indexed searches)", args.PatternInfo.Index)
		}
		return &indexedSearchRequest{
			db:        db,
			Unindexed: limitUnindexedRepos(repos, maxUnindexedRepoRevSearchesPerQuery, stream),
		}, nil
	}

	// Fallback to Unindexed if index:no
	if args.PatternInfo.Index == query.No {
		return &indexedSearchRequest{
			db:        db,
			Unindexed: limitUnindexedRepos(repos, maxUnindexedRepoRevSearchesPerQuery, stream),
		}, nil
	}

	// Only include indexes with symbol information if a symbol request.
	var filter func(repo *zoekt.Repository) bool
	if typ == symbolRequest {
		filter = func(repo *zoekt.Repository) bool {
			return repo.HasSymbols
		}
	}

	// Consult Zoekt to find out which repository revisions can be searched.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	indexedSet, err := args.Zoekt.ListAll(ctx)
	if err != nil {
		if ctx.Err() == nil {
			// Only hard fail if the user specified index:only
			if args.PatternInfo.Index == query.Only {
				return nil, errors.New("index:only failed since indexed search is not available yet")
			}

			log15.Warn("zoektIndexedRepos failed", "error", err)
		}

		return &indexedSearchRequest{
			db:               db,
			Unindexed:        limitUnindexedRepos(repos, maxUnindexedRepoRevSearchesPerQuery, stream),
			IndexUnavailable: true,
		}, ctx.Err()
	}

	tr.LogFields(log.Int("all_indexed_set.size", len(indexedSet)))

	// Split based on indexed vs unindexed
	indexed, searcherRepos := zoektIndexedRepos(indexedSet, repos, filter)

	tr.LogFields(
		log.Int("indexed.size", len(indexed.repoRevs)),
		log.Int("searcher_repos.size", len(searcherRepos)),
	)

	// We do not support non-head searches for the old structural search code path.
	// Once the new code path (triggered by the CombyRule below) is default, this can be removed.
	// https://github.com/sourcegraph/sourcegraph/issues/17616
	if typ == fileRequest && indexed.NotHEADOnlySearch && args.PatternInfo.CombyRule == `where "backcompat" == "backcompat"` {
		return nil, errors.New("structural search only supports searching the default branch https://github.com/sourcegraph/sourcegraph/issues/11906")
	}

	// Disable unindexed search
	if args.PatternInfo.Index == query.Only {
		searcherRepos = limitUnindexedRepos(searcherRepos, 0, stream)
	}

	return &indexedSearchRequest{
		db:   db,
		args: args,
		typ:  typ,

		Unindexed: limitUnindexedRepos(searcherRepos, maxUnindexedRepoRevSearchesPerQuery, stream),
		repos:     indexed,

		DisableUnindexedSearch: args.PatternInfo.Index == query.Only,
	}, nil
}

// Repos is a map of repository revisions that are indexed and will be
// searched by Zoekt. Do not mutate.
func (s *indexedSearchRequest) Repos() map[string]*search.RepositoryRevisions {
	if s.repos == nil {
		return nil
	}
	return s.repos.repoRevs
}

// Search streams 0 or more events to c.
func (s *indexedSearchRequest) Search(ctx context.Context, c Sender) error {
	if s.args == nil {
		return nil
	}
	if len(s.Repos()) == 0 && s.args.Mode != search.ZoektGlobalSearch {
		return nil
	}

	since := time.Since
	if s.since != nil {
		since = s.since
	}

	var zoektStream func(ctx context.Context, db dbutil.DB, args *search.TextParameters, repos *indexedRepoRevs, typ indexedRequestType, since func(t time.Time) time.Duration, c Sender) error
	switch s.typ {
	case textRequest, symbolRequest:
		zoektStream = zoektSearch
	case fileRequest:
		zoektStream = zoektSearchHEADOnlyFiles
	default:
		return fmt.Errorf("unexpected indexedSearchRequest type: %q", s.typ)
	}

	return zoektStream(ctx, s.db, s.args, s.repos, s.typ, since, c)
}

// zoektSearch searches repositories using zoekt.
//
// Timeouts are reported through the context, and as a special case errNoResultsInTimeout
// is returned if no results are found in the given timeout (instead of the more common
// case of finding partial or full results in the given timeout).
func zoektSearch(ctx context.Context, db dbutil.DB, args *search.TextParameters, repos *indexedRepoRevs, typ indexedRequestType, since func(t time.Time) time.Duration, c Sender) error {
	if args == nil {
		return nil
	}
	if len(repos.repoRevs) == 0 && args.Mode != search.ZoektGlobalSearch {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	queryExceptRepos, err := queryToZoektQuery(args.PatternInfo, typ)
	if err != nil {
		return err
	}
	// Performance optimization: For queries without repo: filters, it is not
	// necessary to send the list of all repoBranches to zoekt. Zoekt can simply
	// search all its shards and we filter the results later against the list of
	// repos we resolve concurrently.
	var finalQuery zoektquery.Q
	if args.Mode == search.ZoektGlobalSearch {
		finalQuery = zoektquery.NewAnd(&zoektquery.Branch{Pattern: "HEAD", Exact: true}, queryExceptRepos)
	} else {
		finalQuery = zoektquery.NewAnd(&zoektquery.RepoBranches{Set: repos.repoBranches}, queryExceptRepos)
	}

	k := zoektutil.ResultCountFactor(len(repos.repoBranches), args.PatternInfo.FileMatchLimit, args.Mode == search.ZoektGlobalSearch)
	searchOpts := zoektutil.SearchOpts(ctx, k, args.PatternInfo)

	// Start event stream.
	t0 := time.Now()

	// mu protects matchLimiter and foundResults.
	mu := sync.Mutex{}
	matchLimiter := zoektutil.MatchLimiter{
		Limit: int(args.PatternInfo.FileMatchLimit),
	}
	foundResults := false

	// We use reposResolved to synchronize repo resolution and event processing.
	reposResolved := make(chan struct{})
	var getRepoInputRev zoektutil.RepoRevFunc
	var repoRevMap map[string]*search.RepositoryRevisions

	g, ctx := errgroup.WithContext(ctx)

	// Resolve repositories.
	g.Go(func() error {
		defer close(reposResolved)
		if args.Mode == search.ZoektGlobalSearch || args.PatternInfo.Select.Type == filter.Repository {
			repos, err := getRepos(ctx, args.RepoPromise)
			if err != nil {
				return err
			}
			repoRevMap = make(map[string]*search.RepositoryRevisions, len(repos))
			for _, r := range repos {
				repoRevMap[string(r.Repo.Name)] = r
			}
			getRepoInputRev = func(file *zoekt.FileMatch) (repo *types.RepoName, revs []string, ok bool) {
				if repoRev, ok := repoRevMap[file.Repository]; ok {
					return repoRev.Repo, repoRev.RevSpecs(), true
				}
				return nil, nil, false
			}
		} else {
			getRepoInputRev = func(file *zoekt.FileMatch) (repo *types.RepoName, revs []string, ok bool) {
				repo, inputRevs := repos.GetRepoInputRev(file)
				return repo, inputRevs, true
			}
		}
		return nil
	})

	g.Go(func() error {
		ctx := ctx
		if deadline, ok := ctx.Deadline(); ok {
			// If the user manually specified a timeout, allow zoekt to use all of the remaining timeout.
			searchOpts.MaxWallTime = time.Until(deadline)
			if searchOpts.MaxWallTime < 0 {
				return ctx.Err()
			}
			// We don't want our context's deadline to cut off zoekt so that we can get the results
			// found before the deadline.
			//
			// We'll create a new context that gets cancelled if the other context is cancelled for any
			// reason other than the deadline being exceeded. This essentially means the deadline for the new context
			// will be `deadline + time for zoekt to cancel + network latency`.
			var cancel context.CancelFunc
			ctx, cancel = contextWithoutDeadline(ctx)
			defer cancel()
		}

		// PERF: if we are going to be selecting to repo results only anyways, we can just ask
		// zoekt for only results of type repo.
		if args.PatternInfo.Select.Type == filter.Repository {
			return zoektSearchReposOnly(ctx, args.Zoekt.Client, finalQuery, db, c, func() map[string]*search.RepositoryRevisions {
				<-reposResolved
				// getRepoInputRev is nil only if we encountered an error during repo resolution.
				if getRepoInputRev == nil {
					return nil
				}
				return repoRevMap
			})
		}

		// The buffered backend.ZoektStreamFunc allows us to consume events from Zoekt
		// while we wait for repo resolution.
		bufSender, cleanup := bufferedSender(30, backend.ZoektStreamFunc(func(event *zoekt.SearchResult) {

			mu.Lock()
			foundResults = foundResults || event.FileCount != 0 || event.MatchCount != 0
			mu.Unlock()

			files := event.Files
			limitHit := event.FilesSkipped+event.ShardsSkipped > 0

			if len(files) == 0 {
				c.Send(SearchEvent{
					Stats: streaming.Stats{IsLimitHit: limitHit},
				})
				return
			}

			maxLineMatches := 25 + k
			maxLineFragmentMatches := 3 + k

			<-reposResolved
			// getRepoInputRev is nil only if we encountered an error during repo resolution.
			if getRepoInputRev == nil {
				return
			}

			mu.Lock()
			// Partial is populated with repositories we may have not fully
			// searched due to limits.
			partial, files := matchLimiter.Slice(files, getRepoInputRev)
			mu.Unlock()

			var statusMap search.RepoStatusMap
			for r := range partial {
				statusMap.Update(r, search.RepoStatusLimitHit)
			}

			limitHit = limitHit || len(partial) > 0

			matches := make([]SearchResultResolver, 0, len(files))
			repoResolvers := make(RepositoryResolverCache)
			for _, file := range files {
				fileLimitHit := false
				if len(file.LineMatches) > maxLineMatches {
					file.LineMatches = file.LineMatches[:maxLineMatches]
					fileLimitHit = true
					limitHit = true
				}
				mu.Lock()
				repo, inputRevs, ok := getRepoInputRev(&file)
				mu.Unlock()
				if !ok {
					continue
				}
				repoResolver := repoResolvers[repo.Name]
				if repoResolver == nil {
					repoResolver = NewRepositoryResolver(db, repo.ToRepo())
					repoResolvers[repo.Name] = repoResolver
				}

				var lines []*result.LineMatch
				if typ != symbolRequest {
					lines = zoektFileMatchToLineMatches(maxLineFragmentMatches, &file)
				}

				for _, inputRev := range inputRevs {
					inputRev := inputRev // copy so we can take the pointer

					var symbols []*result.SymbolMatch
					if typ == symbolRequest {
						symbols = zoektFileMatchToSymbolResults(repoResolver, inputRev, &file)
					}
					fm := &FileMatchResolver{
						db: db,
						FileMatch: result.FileMatch{
							Path:        file.FileName,
							LineMatches: lines,
							LimitHit:    fileLimitHit,
							URI:         fileMatchURI(repo.Name, inputRev, file.FileName),
							Symbols:     symbols,
							Repo:        repo,
							CommitID:    api.CommitID(file.Version),
							InputRev:    &inputRev,
						},
						RepoResolver: repoResolver,
					}
					matches = append(matches, fm)
				}
			}

			c.Send(SearchEvent{
				Results: matches,
				Stats: streaming.Stats{
					Status:     statusMap,
					IsLimitHit: limitHit,
				},
			})
		}))
		defer cleanup()

		return args.Zoekt.Client.StreamSearch(ctx, finalQuery, &searchOpts, bufSender)
	})

	if err := g.Wait(); err != nil {
		return err
	}

	mkStatusMap := func(mask search.RepoStatus) search.RepoStatusMap {
		var statusMap search.RepoStatusMap
		for _, r := range repos.repoRevs {
			statusMap.Update(r.Repo.ID, mask)
		}
		return statusMap
	}

	if !foundResults && since(t0) >= searchOpts.MaxWallTime {
		c.Send(SearchEvent{Stats: streaming.Stats{Status: mkStatusMap(search.RepoStatusTimedout)}})
		return nil
	}
	return nil
}

// bufferedSender returns a buffered Sender with capacity cap, and a cleanup
// function which blocks until the buffer is drained. The cleanup function may
// only be called once. For cap=0, bufferedSender returns the input sender.
func bufferedSender(cap int, sender zoekt.Sender) (zoekt.Sender, func()) {
	if cap == 0 {
		return sender, func() {}
	}
	buf := make(chan *zoekt.SearchResult, cap-1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range buf {
			sender.Send(e)
		}
	}()
	cleanup := func() {
		close(buf)
		<-done
	}
	return backend.ZoektStreamFunc(func(event *zoekt.SearchResult) {
		buf <- event
	}), cleanup
}

// zoektSearchReposOnly is used when select:repo is set, in which case we can ask zoekt
// only for the repos that contain matches for the query. This is a performance optimization,
// and not required for proper function of select:repo.
func zoektSearchReposOnly(ctx context.Context, client zoekt.Streamer, query zoektquery.Q, db dbutil.DB, c Sender, getRepoRevMap func() map[string]*search.RepositoryRevisions) error {
	repoList, err := client.List(ctx, query)
	if err != nil {
		return err
	}

	repoRevMap := getRepoRevMap()
	if repoRevMap == nil {
		return nil
	}

	resolvers := make([]SearchResultResolver, 0, len(repoList.Repos))
	for _, repo := range repoList.Repos {
		rev, ok := repoRevMap[repo.Repository.Name]
		if !ok {
			continue
		}

		resolvers = append(resolvers, NewRepositoryResolver(db, &types.Repo{Name: rev.Repo.Name, ID: rev.Repo.ID}))
	}

	c.Send(SearchEvent{
		Results: resolvers,
		Stats:   streaming.Stats{}, // TODO
	})
	return nil
}

func zoektFileMatchToLineMatches(maxLineFragmentMatches int, file *zoekt.FileMatch) []*result.LineMatch {
	lines := make([]*result.LineMatch, 0, len(file.LineMatches))

	for _, l := range file.LineMatches {
		if l.FileName {
			continue
		}

		if len(l.LineFragments) > maxLineFragmentMatches {
			l.LineFragments = l.LineFragments[:maxLineFragmentMatches]
		}
		offsets := make([][2]int32, len(l.LineFragments))
		for k, m := range l.LineFragments {
			offset := utf8.RuneCount(l.Line[:m.LineOffset])
			length := utf8.RuneCount(l.Line[m.LineOffset : m.LineOffset+m.MatchLength])
			offsets[k] = [2]int32{int32(offset), int32(length)}
		}
		lines = append(lines, &result.LineMatch{
			Preview:          string(l.Line),
			LineNumber:       int32(l.LineNumber - 1),
			OffsetAndLengths: offsets,
		})
	}

	return lines
}

func escape(s string) string {
	isSpecial := func(c rune) bool {
		switch c {
		case '\\', '/':
			return true
		default:
			return false
		}
	}

	// Avoid extra work by counting additions. regexp.QuoteMeta does the same,
	// but is more efficient since it does it via bytes.
	count := 0
	for _, c := range s {
		if isSpecial(c) {
			count++
		}
	}
	if count == 0 {
		return s
	}

	escaped := make([]rune, 0, len(s)+count)
	for _, c := range s {
		if isSpecial(c) {
			escaped = append(escaped, '\\')
		}
		escaped = append(escaped, c)
	}
	return string(escaped)
}

func zoektFileMatchToSymbolResults(repo *RepositoryResolver, inputRev string, file *zoekt.FileMatch) []*result.SymbolMatch {
	// Symbol search returns a resolver so we need to pass in some
	// extra stuff. This is a sign that we can probably restructure
	// resolvers to avoid this.
	baseURI := &gituri.URI{URL: url.URL{Scheme: "git", Host: repo.Name(), RawQuery: url.QueryEscape(inputRev)}}
	lang := strings.ToLower(file.Language)

	symbols := make([]*result.SymbolMatch, 0, len(file.LineMatches))
	for _, l := range file.LineMatches {
		if l.FileName {
			continue
		}

		for _, m := range l.LineFragments {
			if m.SymbolInfo == nil {
				continue
			}

			symbols = append(symbols, &result.SymbolMatch{
				Symbol: result.Symbol{
					Name:       m.SymbolInfo.Sym,
					Kind:       m.SymbolInfo.Kind,
					Parent:     m.SymbolInfo.Parent,
					ParentKind: m.SymbolInfo.ParentKind,
					Path:       file.FileName,
					Line:       l.LineNumber,
					// symbolRange requires a Pattern /^...$/ containing the line of the symbol to compute the symbol's offsets.
					// This Pattern is directly accessible on the unindexed code path. But on the indexed code path, we need to
					// populate it, or we will always compute a 0 offset, which messes up API use (e.g., highlighting).
					// It must escape `/` or `\` in the line.
					Pattern: fmt.Sprintf("/^%s$/", escape(string(l.Line))),
				},
				Lang:    lang,
				BaseURI: baseURI,
			})
		}
	}

	return symbols
}

// contextWithoutDeadline returns a context which will cancel if the cOld is
// canceled.
func contextWithoutDeadline(cOld context.Context) (context.Context, context.CancelFunc) {
	cNew, cancel := context.WithCancel(context.Background())

	// Set trace context so we still get spans propagated
	cNew = trace.CopyContext(cNew, cOld)

	// Copy actor from cOld to cNew.
	cNew = actor.WithActor(cNew, actor.FromContext(cOld))

	go func() {
		select {
		case <-cOld.Done():
			// cancel the new context if the old one is done for some reason other than the deadline passing.
			if cOld.Err() != context.DeadlineExceeded {
				cancel()
			}
		case <-cNew.Done():
		}
	}()

	return cNew, cancel
}

func queryToZoektQuery(query *search.TextPatternInfo, typ indexedRequestType) (zoektquery.Q, error) {
	var and []zoektquery.Q

	var q zoektquery.Q
	var err error
	if query.IsRegExp {
		fileNameOnly := query.PatternMatchesPath && !query.PatternMatchesContent
		contentOnly := !query.PatternMatchesPath && query.PatternMatchesContent
		q, err = zoektutil.ParseRe(query.Pattern, fileNameOnly, contentOnly, query.IsCaseSensitive)
		if err != nil {
			return nil, err
		}
	} else {
		q = &zoektquery.Substring{
			Pattern:       query.Pattern,
			CaseSensitive: query.IsCaseSensitive,

			FileName: true,
			Content:  true,
		}
	}

	if query.IsNegated {
		q = &zoektquery.Not{Child: q}
	}

	if typ == symbolRequest {
		// Tell zoekt q must match on symbols
		q = &zoektquery.Symbol{
			Expr: q,
		}
	}

	and = append(and, q)

	// zoekt also uses regular expressions for file paths
	// TODO PathPatternsAreCaseSensitive
	// TODO whitespace in file path patterns?
	for _, p := range query.IncludePatterns {
		q, err := zoektutil.FileRe(p, query.IsCaseSensitive)
		if err != nil {
			return nil, err
		}
		and = append(and, q)
	}
	if query.ExcludePattern != "" {
		q, err := zoektutil.FileRe(query.ExcludePattern, query.IsCaseSensitive)
		if err != nil {
			return nil, err
		}
		and = append(and, &zoektquery.Not{Child: q})
	}

	// For conditionals that happen on a repo we can use type:repo queries. eg
	// (type:repo file:foo) (type:repo file:bar) will match all repos which
	// contain a filename matching "foo" and a filename matchinb "bar".
	//
	// Note: (type:repo file:foo file:bar) will only find repos with a
	// filename containing both "foo" and "bar".
	for _, p := range query.FilePatternsReposMustInclude {
		q, err := zoektutil.FileRe(p, query.IsCaseSensitive)
		if err != nil {
			return nil, err
		}
		and = append(and, &zoektquery.Type{Type: zoektquery.TypeRepo, Child: q})
	}
	for _, p := range query.FilePatternsReposMustExclude {
		q, err := zoektutil.FileRe(p, query.IsCaseSensitive)
		if err != nil {
			return nil, err
		}
		and = append(and, &zoektquery.Not{Child: &zoektquery.Type{Type: zoektquery.TypeRepo, Child: q}})
	}

	return zoektquery.Simplify(zoektquery.NewAnd(and...)), nil
}

// zoektIndexedRepos splits the revs into two parts: (1) the repository
// revisions in indexedSet (indexed) and (2) the repositories that are
// unindexed.
func zoektIndexedRepos(indexedSet map[string]*zoekt.Repository, revs []*search.RepositoryRevisions, filter func(*zoekt.Repository) bool) (indexed *indexedRepoRevs, unindexed []*search.RepositoryRevisions) {
	// PERF: If len(revs) is large, we expect to be doing an indexed
	// search. So set indexed to the max size it can be to avoid growing.
	indexed = &indexedRepoRevs{
		repoRevs:     make(map[string]*search.RepositoryRevisions, len(revs)),
		repoBranches: make(map[string][]string, len(revs)),
	}
	unindexed = make([]*search.RepositoryRevisions, 0)

	for _, reporev := range revs {
		repo, ok := indexedSet[string(reporev.Repo.Name)]
		if !ok || (filter != nil && !filter(repo)) {
			unindexed = append(unindexed, reporev)
			continue
		}

		unindexedRevs := indexed.Add(reporev, repo)
		if len(unindexedRevs) > 0 {
			copy := *reporev
			copy.Revs = unindexedRevs
			unindexed = append(unindexed, &copy)
		}
	}

	return indexed, unindexed
}

// indexedRepoRevs creates both the Sourcegraph and Zoekt representation of a
// list of repository and refs to search.
type indexedRepoRevs struct {
	// repoRevs is the Sourcegraph representation of a the list of repoRevs
	// repository and revisions to search.
	repoRevs map[string]*search.RepositoryRevisions

	// repoBranches will be used when we query zoekt. The order of branches
	// must match that in a reporev such that we can map back results. IE this
	// invariant is maintained:
	//
	//  repoBranches[reporev.Repo.Name][i] <-> reporev.Revs[i]
	repoBranches map[string][]string

	// NotHEADOnlySearch is true if we are searching a branch other than HEAD.
	//
	// This option can be removed once structural search supports searching
	// more than HEAD.
	NotHEADOnlySearch bool
}

// headBranch is used as a singleton of the indexedRepoRevs.repoBranches to save
// common-case allocations within indexedRepoRevs.Add.
var headBranch = []string{"HEAD"}

// Add will add reporev and repo to the list of repository and branches to
// search if reporev's refs are a subset of repo's branches. It will return
// the revision specifiers it can't add.
func (rb *indexedRepoRevs) Add(reporev *search.RepositoryRevisions, repo *zoekt.Repository) []search.RevisionSpecifier {
	// A repo should only appear once in revs. However, in case this
	// invariant is broken we will treat later revs as if it isn't
	// indexed.
	if _, ok := rb.repoBranches[string(reporev.Repo.Name)]; ok {
		return reporev.Revs
	}

	if !reporev.OnlyExplicit() {
		// Contains a RefGlob or ExcludeRefGlob so we can't do indexed
		// search on it.
		//
		// TODO we could only process the explicit revs and return the non
		// explicit ones as unindexed.
		return reporev.Revs
	}

	if len(reporev.Revs) == 1 && repo.Branches[0].Name == "HEAD" && (reporev.Revs[0].RevSpec == "" || reporev.Revs[0].RevSpec == "HEAD") {
		rb.repoRevs[string(reporev.Repo.Name)] = reporev
		rb.repoBranches[string(reporev.Repo.Name)] = headBranch
		return nil
	}

	// notHEADOnlySearch is set to true if we search any branch other than
	// repo.Branches[0]
	notHEADOnlySearch := false

	// Assume for large searches they will mostly involve indexed
	// revisions, so just allocate that.
	var unindexed []search.RevisionSpecifier
	indexed := make([]search.RevisionSpecifier, 0, len(reporev.Revs))

	branches := make([]string, 0, len(reporev.Revs))
	for _, rev := range reporev.Revs {
		if rev.RevSpec == "" || rev.RevSpec == "HEAD" {
			// Zoekt convention that first branch is HEAD
			branches = append(branches, repo.Branches[0].Name)
			indexed = append(indexed, rev)
			continue
		}

		found := false
		for i, branch := range repo.Branches {
			if branch.Name == rev.RevSpec {
				branches = append(branches, branch.Name)
				notHEADOnlySearch = notHEADOnlySearch || i > 0
				found = true
				break
			}
			// Check if rev is an abbrev commit SHA
			if len(rev.RevSpec) >= 4 && strings.HasPrefix(branch.Version, rev.RevSpec) {
				branches = append(branches, branch.Name)
				notHEADOnlySearch = notHEADOnlySearch || i > 0
				found = true
				break
			}
		}

		if found {
			indexed = append(indexed, rev)
		} else {
			unindexed = append(unindexed, rev)
		}
	}

	// We found indexed branches! Track them.
	if len(indexed) > 0 {
		rb.repoRevs[string(reporev.Repo.Name)] = reporev
		rb.repoBranches[string(reporev.Repo.Name)] = branches
		rb.NotHEADOnlySearch = rb.NotHEADOnlySearch || notHEADOnlySearch
	}

	return unindexed
}

// GetRepoInputRev returns the repo and inputRev associated with file.
func (rb *indexedRepoRevs) GetRepoInputRev(file *zoekt.FileMatch) (repo *types.RepoName, inputRevs []string) {
	repoRev := rb.repoRevs[file.Repository]

	inputRevs = make([]string, 0, len(file.Branches))
	for _, branch := range file.Branches {
		for i, b := range rb.repoBranches[file.Repository] {
			if branch == b {
				// RevSpec is guaranteed to be explicit via zoektIndexedRepos
				inputRevs = append(inputRevs, repoRev.Revs[i].RevSpec)
			}
		}
	}

	if len(inputRevs) == 0 {
		// Did not find a match. This is unexpected, but we can fallback to
		// file.Version to generate correct links.
		inputRevs = append(inputRevs, file.Version)
	}

	return repoRev.Repo, inputRevs
}

// limitUnindexedRepos limits the number of repo@revs searched by the
// unindexed searcher codepath.  Sending many requests to searcher would
// otherwise cause a flood of system and network requests that result in
// timeouts or long delays.
//
// It returns the new repositories destined for the unindexed searcher code
// path, and sends an event to stream for any repositories that are limited /
// excluded.
//
// A slice to the input list is returned, it is not copied.
func limitUnindexedRepos(unindexed []*search.RepositoryRevisions, limit int, stream Sender) []*search.RepositoryRevisions {
	var missing []*search.RepositoryRevisions

	for i, repoRevs := range unindexed {
		limit -= len(repoRevs.Revs)
		if limit < 0 {
			missing = unindexed[i:]
			unindexed = unindexed[:i]
			break
		}
	}

	if len(missing) > 0 {
		var status search.RepoStatusMap
		for _, r := range missing {
			status.Update(r.Repo.ID, search.RepoStatusMissing)
		}
		stream.Send(SearchEvent{
			Stats: streaming.Stats{
				Status: status,
			},
		})
	}

	return unindexed
}
