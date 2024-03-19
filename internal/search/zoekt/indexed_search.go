package zoekt

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/RoaringBitmap/roaring"
	"github.com/grafana/regexp"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/atomic"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// IndexedRepoRevs creates both the Sourcegraph and Zoekt representation of a
// list of repository and refs to search.
type IndexedRepoRevs struct {
	// RepoRevs is the Sourcegraph representation of a the list of repoRevs
	// repository and revisions to search.
	RepoRevs map[api.RepoID]*search.RepositoryRevisions

	// branchRepos is used to construct a zoektquery.BranchesRepos to efficiently
	// marshal and send to zoekt
	branchRepos map[string]*zoektquery.BranchRepos
}

// GetRepoRevsFromBranchRepos updates RepoRevs by replacing revision values that are not defined branches in
// Zoekt and replaces with a known indexed branch.
// This is used for structural search querying revisions of RepositoryRevisions that are indexed but not the branch name.
func (rb *IndexedRepoRevs) GetRepoRevsFromBranchRepos() map[api.RepoID]*search.RepositoryRevisions {
	repoRevs := make(map[api.RepoID]*search.RepositoryRevisions, len(rb.RepoRevs))

	for repoID, repoRev := range rb.RepoRevs {
		updated := *repoRev

		for i, rev := range updated.Revs {
			// check if revision should be used as a branch name for zoekt branchRepos queries and replace if not
			if rev != "" && rb.branchRepos[rev] == nil {
				if len(rb.branchRepos) == 1 {
					// use the single branch that zoekt returned in branchRepos as the revision
					for k := range rb.branchRepos {
						updated.Revs[i] = k
						break
					}
				} else {
					// if there are multiple branches then fall back to HEAD
					// clear value to identify to zoekt to utilize branch HEAD regardless of repo ID
					updated.Revs[i] = ""
				}
			}
		}

		repoRevs[repoID] = &updated
	}

	return repoRevs
}

// add will add reporev and repo to the list of repository and branches to
// search if reporev's refs are a subset of repo's branches. It will return
// the revision specifiers it can't add.
func (rb *IndexedRepoRevs) add(reporev *search.RepositoryRevisions, repo zoekt.MinimalRepoListEntry) []string {
	// A repo should only appear once in revs. However, in case this
	// invariant is broken we will treat later revs as if it isn't
	// indexed.
	if _, ok := rb.RepoRevs[reporev.Repo.ID]; ok {
		return reporev.Revs
	}

	// Assume for large searches they will mostly involve indexed
	// revisions, so just allocate that.
	var unindexed []string

	branches := make([]string, 0, len(reporev.Revs))
	reporev = reporev.Copy()
	indexed := reporev.Revs[:0]

	for _, inputRev := range reporev.Revs {
		found := false
		rev := inputRev
		if rev == "" {
			rev = "HEAD"
		}

		for _, branch := range repo.Branches {
			if branch.Name == rev {
				branches = append(branches, branch.Name)
				found = true
				break
			}
			// Check if rev is an abbrev commit SHA
			if len(rev) >= 4 && strings.HasPrefix(branch.Version, rev) {
				branches = append(branches, branch.Name)
				found = true
				break
			}
		}

		if found {
			indexed = append(indexed, inputRev)
		} else {
			unindexed = append(unindexed, inputRev)
		}
	}

	// We found indexed branches! Track them.
	if len(indexed) > 0 {
		reporev.Revs = indexed
		rb.RepoRevs[reporev.Repo.ID] = reporev
		for _, branch := range branches {
			br, ok := rb.branchRepos[branch]
			if !ok {
				br = &zoektquery.BranchRepos{Branch: branch, Repos: roaring.New()}
				rb.branchRepos[branch] = br
			}
			br.Repos.Add(uint32(reporev.Repo.ID))
		}
	}

	return unindexed
}

func (rb *IndexedRepoRevs) BranchRepos() []zoektquery.BranchRepos {
	brs := make([]zoektquery.BranchRepos, 0, len(rb.branchRepos))
	for _, br := range rb.branchRepos {
		brs = append(brs, *br)
	}
	return brs
}

// getRepoInputRev returns the repo and inputRev associated with file.
func (rb *IndexedRepoRevs) getRepoInputRev(file *zoekt.FileMatch) (repo types.MinimalRepo, inputRevs []string) {
	repoRev, ok := rb.RepoRevs[api.RepoID(file.RepositoryID)]

	// We search zoekt by repo ID. It is possible that the name has come out
	// of sync, so the above lookup will fail. We fallback to linking the rev
	// hash in that case. We intend to restucture this code to avoid this, but
	// this is the fix to avoid potential nil panics.
	if !ok {
		repo := types.MinimalRepo{
			ID:   api.RepoID(file.RepositoryID),
			Name: api.RepoName(file.Repository),
		}
		return repo, []string{file.Version}
	}

	// We inverse the logic in add to work out the revspec from the zoekt
	// branches.
	//
	// Note: RevSpec is guaranteed to be explicit via zoektIndexedRepos
	inputRevs = make([]string, 0, len(file.Branches))
	for _, rev := range repoRev.Revs {
		// We rely on the Sourcegraph implementation that the HEAD branch is
		// indexed as "HEAD" rather than resolving the symref.
		revBranchName := rev
		if revBranchName == "" {
			revBranchName = "HEAD" // empty string in Sourcegraph means HEAD
		}

		found := false
		for _, branch := range file.Branches {
			if branch == revBranchName {
				found = true
				break
			}
		}
		if found {
			inputRevs = append(inputRevs, rev)
			continue
		}

		// Check if rev is an abbrev commit SHA
		if len(rev) >= 4 && strings.HasPrefix(file.Version, rev) {
			inputRevs = append(inputRevs, rev)
			continue
		}
	}

	if len(inputRevs) == 0 {
		// Did not find a match. This is unexpected, but we can fallback to
		// file.Version to generate correct links.
		inputRevs = append(inputRevs, file.Version)
	}

	return repoRev.Repo, inputRevs
}

func PartitionRepos(
	ctx context.Context,
	logger log.Logger,
	repos []*search.RepositoryRevisions,
	zoektStreamer zoekt.Streamer,
	typ search.IndexedRequestType,
	useIndex query.YesNoOnly,
	containsRefGlobs bool,
) (indexed *IndexedRepoRevs, unindexed []*search.RepositoryRevisions, err error) {
	// Fallback to Unindexed if the query contains valid ref-globs.
	if containsRefGlobs {
		return &IndexedRepoRevs{}, repos, nil
	}
	// Fallback to Unindexed if index:no
	if useIndex == query.No {
		return &IndexedRepoRevs{}, repos, nil
	}

	tr, ctx := trace.New(ctx, "PartitionRepos", attribute.String("type", string(typ)))
	defer tr.EndWithErr(&err)

	// Only include indexes with symbol information if a symbol request.
	var filterFunc func(repo zoekt.MinimalRepoListEntry) bool
	if typ == search.SymbolRequest {
		filterFunc = func(repo zoekt.MinimalRepoListEntry) bool {
			return repo.HasSymbols
		}
	}

	// Consult Zoekt to find out which repository revisions can be searched.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	list, err := zoektStreamer.List(ctx, &zoektquery.Const{Value: true}, &zoekt.ListOptions{Field: zoekt.RepoListFieldReposMap})
	if err != nil {
		if ctx.Err() == nil {
			// Only hard fail if the user specified index:only
			if useIndex == query.Only {
				return nil, nil, errors.New("index:only failed since indexed search is not available yet")
			}

			logger.Warn("zoektIndexedRepos failed", log.Error(err))
		}

		return &IndexedRepoRevs{}, repos, ctx.Err()
	}

	// Note: We do not need to handle list.Crashes since we will fallback to
	// unindexed search for any repository unavailable due to rollout.

	tr.SetAttributes(attribute.Int("all_indexed_set.size", len(list.ReposMap)))

	// Split based on indexed vs unindexed
	indexed, unindexed = zoektIndexedRepos(list.ReposMap, repos, filterFunc)

	tr.SetAttributes(
		attribute.Int("indexed.size", len(indexed.RepoRevs)),
		attribute.Int("unindexed.size", len(unindexed)))

	// Disable unindexed search
	if useIndex == query.Only {
		unindexed = unindexed[:0]
	}

	return indexed, unindexed, nil
}

func DoZoektSearchGlobal(ctx context.Context, client zoekt.Streamer, params *search.ZoektParameters, pathRegexps []*regexp.Regexp, c streaming.Sender) error {
	searchOpts := params.ToSearchOptions(ctx)

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

	return client.StreamSearch(ctx, params.Query, searchOpts, backend.ZoektStreamFunc(func(event *zoekt.SearchResult) {
		sendMatches(event, pathRegexps, func(file *zoekt.FileMatch) (types.MinimalRepo, []string) {
			repo := types.MinimalRepo{
				ID:   api.RepoID(file.RepositoryID),
				Name: api.RepoName(file.Repository),
			}
			return repo, []string{""}
		}, params.Typ, params.Select, c)
	}))
}

// zoektSearch searches repositories using zoekt.
func zoektSearch(ctx context.Context, repos *IndexedRepoRevs, q zoektquery.Q, pathRegexps []*regexp.Regexp, typ search.IndexedRequestType, client zoekt.Streamer, zoektParams *search.ZoektParameters, since func(t time.Time) time.Duration, c streaming.Sender) error {
	if len(repos.RepoRevs) == 0 {
		return nil
	}

	brs := repos.BranchRepos()

	finalQuery := zoektquery.NewAnd(&zoektquery.BranchesRepos{List: brs}, q)
	searchOpts := zoektParams.ToSearchOptions(ctx)

	// Start event stream.
	t0 := time.Now()

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

	foundResults := atomic.Bool{}
	err := client.StreamSearch(ctx, finalQuery, searchOpts, backend.ZoektStreamFunc(func(event *zoekt.SearchResult) {
		foundResults.CompareAndSwap(false, event.FileCount != 0 || event.MatchCount != 0)
		sendMatches(event, pathRegexps, repos.getRepoInputRev, typ, zoektParams.Select, c)
	}))
	if err != nil {
		return err
	}

	mkStatusMap := func(mask search.RepoStatus) search.RepoStatusMap {
		var statusMap search.RepoStatusMap
		for _, r := range repos.RepoRevs {
			statusMap.Update(r.Repo.ID, mask)
		}
		return statusMap
	}

	if !foundResults.Load() && since(t0) >= searchOpts.MaxWallTime {
		c.Send(streaming.SearchEvent{Stats: streaming.Stats{Status: mkStatusMap(search.RepoStatusTimedOut)}})
	}
	return nil
}

func sendMatches(event *zoekt.SearchResult, pathRegexps []*regexp.Regexp, getRepoInputRev repoRevFunc, typ search.IndexedRequestType, selector filter.SelectPath, c streaming.Sender) {
	files := event.Files
	stats := streaming.Stats{
		// In the case of Zoekt the only time we get non-zero Crashes in
		// practice is when a backend is missing.
		BackendsMissing: event.Crashes,
		IsLimitHit:      event.FilesSkipped+event.ShardsSkipped > 0,
	}

	if selector.Root() == filter.Repository {
		// By default we stream up to "all" repository results per
		// select:repo request, and we never communicate whether a limit
		// is reached here based on Zoekt progress (because Zoekt can't
		// tell us the value of something like `ReposSkipped`). Instead,
		// limitHit is determined by other factors, like whether the
		// request is cancelled, or when we find the maximum number of
		// `count` results. I.e., from the webapp, this is
		// `max(defaultMaxSearchResultsStreaming,count)` which comes to
		// `max(500,count)`.
		stats.IsLimitHit = false
	}

	if len(files) == 0 {
		c.Send(streaming.SearchEvent{
			Stats: stats,
		})
		return
	}

	matches := make([]result.Match, 0, len(files))
	for _, file := range files {
		repo, inputRevs := getRepoInputRev(&file)

		if selector.Root() == filter.Repository {
			matches = append(matches, &result.RepoMatch{
				Name: repo.Name,
				ID:   repo.ID,
			})
			continue
		}

		var hms result.ChunkMatches
		if typ != search.SymbolRequest {
			hms = zoektFileMatchToMultilineMatches(&file)
		}

		pathMatches := zoektFileMatchToPathMatchRanges(&file, pathRegexps)

		for _, inputRev := range inputRevs {
			inputRev := inputRev // copy so we can take the pointer

			var symbols []*result.SymbolMatch
			if typ == search.SymbolRequest {
				symbols = zoektFileMatchToSymbolResults(repo, inputRev, &file)
			}
			fm := result.FileMatch{
				ChunkMatches: hms,
				Symbols:      symbols,
				PathMatches:  pathMatches,
				File: result.File{
					InputRev:        &inputRev,
					CommitID:        api.CommitID(file.Version),
					Repo:            repo,
					Path:            file.FileName,
					PreciseLanguage: file.Language,
				},
			}
			if debug := file.Debug; debug != "" {
				fm.Debug = &debug
			}
			matches = append(matches, &fm)
		}
	}

	c.Send(streaming.SearchEvent{
		Results: matches,
		Stats:   stats,
	})
}

func zoektFileMatchToMultilineMatches(file *zoekt.FileMatch) result.ChunkMatches {
	cms := make(result.ChunkMatches, 0, len(file.ChunkMatches))
	for _, cm := range file.ChunkMatches {
		if cm.FileName {
			continue
		}

		ranges := make([]result.Range, 0, len(cm.Ranges))
		for _, r := range cm.Ranges {
			ranges = append(ranges, result.Range{
				Start: result.Location{
					Offset: int(r.Start.ByteOffset),
					Line:   int(r.Start.LineNumber) - 1,
					Column: int(r.Start.Column) - 1,
				},
				End: result.Location{
					Offset: int(r.End.ByteOffset),
					Line:   int(r.End.LineNumber) - 1,
					Column: int(r.End.Column) - 1,
				},
			})
		}

		cms = append(cms, result.ChunkMatch{
			Content: string(cm.Content),
			ContentStart: result.Location{
				Offset: int(cm.ContentStart.ByteOffset),
				Line:   int(cm.ContentStart.LineNumber) - 1,
				Column: int(cm.ContentStart.Column) - 1,
			},
			Ranges: ranges,
		})
	}

	return cms
}

func zoektFileMatchToPathMatchRanges(file *zoekt.FileMatch, pathRegexps []*regexp.Regexp) (pathMatchRanges []result.Range) {
	for _, re := range pathRegexps {
		pathSubmatches := re.FindAllStringSubmatchIndex(file.FileName, -1)
		for _, sm := range pathSubmatches {
			pathMatchRanges = append(pathMatchRanges, result.Range{
				Start: result.Location{
					Offset: sm[0],
					Line:   0, // we can treat path matches as a single-line
					Column: utf8.RuneCountInString(file.FileName[:sm[0]]),
				},
				End: result.Location{
					Offset: sm[1],
					Line:   0,
					Column: utf8.RuneCountInString(file.FileName[:sm[1]]),
				},
			})
		}
	}

	return pathMatchRanges
}

func zoektFileMatchToSymbolResults(repoName types.MinimalRepo, inputRev string, file *zoekt.FileMatch) []*result.SymbolMatch {
	newFile := &result.File{
		Path:            file.FileName,
		Repo:            repoName,
		CommitID:        api.CommitID(file.Version),
		InputRev:        &inputRev,
		PreciseLanguage: file.Language,
	}

	symbols := make([]*result.SymbolMatch, 0, len(file.ChunkMatches))
	for _, cm := range file.ChunkMatches {
		if cm.FileName || len(cm.SymbolInfo) == 0 {
			continue
		}

		for i, r := range cm.Ranges {
			si := cm.SymbolInfo[i]
			if si == nil {
				continue
			}

			symbols = append(symbols, result.NewSymbolMatch(
				newFile,
				int(r.Start.LineNumber),
				int(r.Start.Column)-1,
				si.Sym,
				si.Kind,
				si.Parent,
				si.ParentKind,
				file.Language,
				"", // Unused when column is set
				false,
			))
		}
	}

	// We deduplicate symbol matches. For example searching for "foo AND bar"
	// will return the symbol "foobar" twice. However, sourcegraph's result
	// type for symbol is modeled as a result per symbol.
	return result.DedupSymbols(symbols)
}

// contextWithoutDeadline returns a context which will cancel if the cOld is
// canceled.
func contextWithoutDeadline(cOld context.Context) (context.Context, context.CancelFunc) {
	cNew := context.WithoutCancel(cOld)
	cNew, cancel := context.WithCancel(cNew)

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

// zoektIndexedRepos splits the revs into two parts: (1) the repository
// revisions in indexedSet (indexed) and (2) the repositories that are
// unindexed.
func zoektIndexedRepos(indexedSet zoekt.ReposMap, revs []*search.RepositoryRevisions, filter func(repo zoekt.MinimalRepoListEntry) bool) (indexed *IndexedRepoRevs, unindexed []*search.RepositoryRevisions) {
	// PERF: If len(revs) is large, we expect to be doing an indexed
	// search. So set indexed to the max size it can be to avoid growing.
	indexed = &IndexedRepoRevs{
		RepoRevs:    make(map[api.RepoID]*search.RepositoryRevisions, len(revs)),
		branchRepos: make(map[string]*zoektquery.BranchRepos, 1),
	}
	unindexed = make([]*search.RepositoryRevisions, 0)

	for _, reporev := range revs {
		repo, ok := indexedSet[uint32(reporev.Repo.ID)]
		if !ok || (filter != nil && !filter(repo)) {
			unindexed = append(unindexed, reporev)
			continue
		}

		unindexedRevs := indexed.add(reporev, repo)
		if len(unindexedRevs) > 0 {
			copy := reporev.Copy()
			copy.Revs = unindexedRevs
			unindexed = append(unindexed, copy)
		}
	}

	return indexed, unindexed
}

type RepoSubsetTextSearchJob struct {
	Repos             *IndexedRepoRevs // the set of indexed repository revisions to search.
	Query             zoektquery.Q
	ZoektQueryRegexps []*regexp.Regexp // used for getting file path match ranges
	Typ               search.IndexedRequestType
	ZoektParams       *search.ZoektParameters
	Since             func(time.Time) time.Duration `json:"-"` // since if non-nil will be used instead of time.Since. For tests
}

// ZoektSearch is a job that searches repositories using zoekt.
func (z *RepoSubsetTextSearchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, z)
	defer func() { finish(alert, err) }()

	if z.Repos == nil {
		return nil, nil
	}
	if len(z.Repos.RepoRevs) == 0 {
		return nil, nil
	}

	since := time.Since
	if z.Since != nil {
		since = z.Since
	}

	return nil, zoektSearch(ctx, z.Repos, z.Query, z.ZoektQueryRegexps, z.Typ, clients.Zoekt, z.ZoektParams, since, stream)
}

func (*RepoSubsetTextSearchJob) Name() string {
	return "ZoektRepoSubsetTextSearchJob"
}

func (z *RepoSubsetTextSearchJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		res = append(res,
			attribute.Int("fileMatchLimit", int(z.ZoektParams.FileMatchLimit)),
			attribute.Stringer("select", z.ZoektParams.Select),
			trace.Stringers("zoektQueryRegexps", z.ZoektQueryRegexps),
		)
		// z.Repos is nil for un-indexed search
		if z.Repos != nil {
			res = append(res,
				attribute.Int("numRepoRevs", len(z.Repos.RepoRevs)),
				attribute.Int("numBranchRepos", len(z.Repos.branchRepos)),
			)
		}
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			attribute.Stringer("query", z.Query),
			attribute.String("type", string(z.Typ)),
		)
	}
	return res
}

func (*RepoSubsetTextSearchJob) Children() []job.Describer         { return nil }
func (j *RepoSubsetTextSearchJob) MapChildren(job.MapFunc) job.Job { return j }

type GlobalTextSearchJob struct {
	GlobalZoektQuery        *GlobalZoektQuery
	ZoektParams             *search.ZoektParameters
	RepoOpts                search.RepoOptions
	GlobalZoektQueryRegexps []*regexp.Regexp // used for getting file path match ranges
}

func (t *GlobalTextSearchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, t)
	defer func() { finish(alert, err) }()

	userPrivateRepos := privateReposForActor(ctx, clients.Logger, clients.DB, t.RepoOpts)
	t.GlobalZoektQuery.ApplyPrivateFilter(userPrivateRepos)
	t.ZoektParams.Query = t.GlobalZoektQuery.Generate()

	return nil, DoZoektSearchGlobal(ctx, clients.Zoekt, t.ZoektParams, t.GlobalZoektQueryRegexps, stream)
}

func (*GlobalTextSearchJob) Name() string {
	return "ZoektGlobalTextSearchJob"
}

func (t *GlobalTextSearchJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		res = append(res,
			attribute.Int("fileMatchLimit", int(t.ZoektParams.FileMatchLimit)),
			attribute.Stringer("select", t.ZoektParams.Select),
			trace.Stringers("repoScope", t.GlobalZoektQuery.RepoScope),
			attribute.Bool("includePrivate", t.GlobalZoektQuery.IncludePrivate),
			trace.Stringers("globalZoektQueryRegexps", t.GlobalZoektQueryRegexps),
		)
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			attribute.Stringer("query", t.GlobalZoektQuery.Query),
			attribute.String("type", string(t.ZoektParams.Typ)),
		)
		res = append(res, trace.Scoped("repoOpts", t.RepoOpts.Attributes()...)...)
	}
	return res
}

func (t *GlobalTextSearchJob) Children() []job.Describer       { return nil }
func (t *GlobalTextSearchJob) MapChildren(job.MapFunc) job.Job { return t }

// Get all private repos for the the current actor.
func privateReposForActor(ctx context.Context, logger log.Logger, db database.DB, repoOptions search.RepoOptions) []types.MinimalRepo {
	tr, ctx := trace.New(ctx, "privateReposForActor")
	defer tr.End()

	// TODO: We should use repos.Resolve here. However, this logic was added
	// when we used UserID on sourcegraph.com and it was handled differently
	// in repos.Resolve. We need to confirm and test the change to
	// repos.Resolve.
	userPrivateRepos, err := db.Repos().ListMinimalRepos(ctx, database.ReposListOptions{
		OnlyPrivate:    true,
		LimitOffset:    &database.LimitOffset{Limit: limits.SearchLimits(conf.Get()).MaxRepos + 1},
		OnlyForks:      repoOptions.OnlyForks,
		NoForks:        repoOptions.NoForks,
		OnlyArchived:   repoOptions.OnlyArchived,
		NoArchived:     repoOptions.NoArchived,
		ExcludePattern: query.UnionRegExps(repoOptions.MinusRepoFilters),
	})

	if err != nil {
		var userID int32
		if a := actor.FromContext(ctx); a != nil {
			userID = a.UID
		}
		logger.Error("doResults: failed to list user private repos", log.Error(err), log.Int32("user-id", userID))
		tr.AddEvent("error resolving user private repos", trace.Error(err))
	}
	return userPrivateRepos
}
