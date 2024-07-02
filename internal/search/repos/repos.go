package repos

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grafana/regexp"
	regexpsyntax "github.com/grafana/regexp/syntax"
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/limits"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/searchcontexts"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	searchzoekt "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
)

// Resolved represents the repository revisions we need to search for a query.
// This usually involves querying the database and resolving revisions against
// gitserver.
type Resolved struct {
	RepoRevs []*search.RepositoryRevisions

	// BackendsMissing is the number of search backends that failed to be
	// searched. This is due to it being unreachable. The most common reason
	// for this is during zoekt rollout.
	BackendsMissing int
}

// MaybeSendStats is a convenience which will stream a stats event if r
// contains any missing backends.
func (r *Resolved) MaybeSendStats(stream streaming.Sender) {
	if r.BackendsMissing > 0 {
		stream.Send(streaming.SearchEvent{
			Stats: streaming.Stats{
				BackendsMissing: r.BackendsMissing,
			},
		})
	}
}

func (r *Resolved) String() string {
	return fmt.Sprintf("Resolved{RepoRevs=%d BackendsMissing=%d}", len(r.RepoRevs), r.BackendsMissing)
}

func NewResolver(logger log.Logger, db database.DB, gitserverClient gitserver.Client, searcher *endpoint.Map, searcherGRPCConnectionCache *defaults.ConnectionCache, zoekt zoekt.Streamer) *Resolver {
	return &Resolver{
		logger:                      logger,
		db:                          db,
		gitserver:                   gitserverClient,
		zoekt:                       zoekt,
		searcher:                    searcher,
		searcherGRPCConnectionCache: searcherGRPCConnectionCache,
	}
}

type Resolver struct {
	logger                      log.Logger
	db                          database.DB
	gitserver                   gitserver.Client
	zoekt                       zoekt.Streamer
	searcher                    *endpoint.Map
	searcherGRPCConnectionCache *defaults.ConnectionCache
}

// Iterator returns an iterator of Resolved for opts.
//
// Note: this will collect all MissingRepoRevsErrors per page and only return
// it at the end of the iteration. For other errors we stop iterating and
// return straight away.
func (r *Resolver) Iterator(ctx context.Context, opts search.RepoOptions) *iterator.Iterator[Resolved] {
	if opts.Limit == 0 {
		opts.Limit = 4096
	}

	var errs error
	done := false
	return iterator.New(func() ([]Resolved, error) {
		if done {
			return nil, errs
		}

		page, next, err := r.resolve(ctx, opts)
		if err != nil {
			errs = errors.Append(errs, err)
			// For missing repo revs, just collect the error and keep paging
			if !errors.Is(err, &MissingRepoRevsError{}) {
				return nil, errs
			}
		}

		done = next == nil
		opts.Cursors = next
		return []Resolved{page}, nil
	})
}

// IterateRepoRevs does the database portion of repository resolving. This API
// is exported for search jobs (exhaustive) to allow it to seperate the step
// which only speaks to the DB to the step speaks to gitserver/etc.
//
// NOTE: This iterator may return a *MissingRepoRevsError. However, it may be
// different to the error returned by Iterator since when speaking to
// gitserver it may find additional missing revs.
//
// The other error type that may be returned is ErrNoResolvedRepos.
func (r *Resolver) IterateRepoRevs(ctx context.Context, opts search.RepoOptions) *iterator.Iterator[RepoRevSpecs] {
	if opts.Limit == 0 {
		opts.Limit = 4096
	}

	var missing []RepoRevSpecs
	done := false
	return iterator.New(func() ([]RepoRevSpecs, error) {
		// We need to retry since page.Associated may be empty but there are
		// still more pages to fetch from the DB. The iterator will stop once
		// it receives an empty page.
		//
		// TODO(keegan) I don't like this whole MissingRepoRevsError behavior
		// in this iterator and the other. There is likely a more
		// straightforward behaviour here which will also avoid needs like
		// this extra for loop.
		for !done {
			page, next, err := r.queryDB(ctx, opts)
			if err != nil {
				return nil, err
			}

			missing = append(missing, page.Missing...)
			done = next == nil
			opts.Cursors = next

			// Found a non-zero result, pass it on to the iterator.
			if len(page.Associated) > 0 {
				return page.Associated, nil
			}
		}

		return nil, maybeMissingRepoRevsError(missing)
	})
}

// ResolveRevSpecs will resolve RepoRevSpecs returned by IterateRepoRevs. It
// requires passing in the same options to work correctly.
//
// NOTE: This API is not idiomatic and can return non-nil error with a useful
// Resolved. In particular the it may return a *MissingRepoRevsError.
func (r *Resolver) ResolveRevSpecs(ctx context.Context, op search.RepoOptions, repoRevSpecs []RepoRevSpecs) (_ Resolved, err error) {
	tr, ctx := trace.New(ctx, "searchrepos.ResolveRevSpecs", attribute.Stringer("opts", &op))
	defer tr.EndWithErr(&err)

	result := dbResolved{
		Associated: repoRevSpecs,
	}

	resolved, err := r.doFilterDBResolved(ctx, tr, r.searcherGRPCConnectionCache, op, result)
	return resolved, err
}

// queryDB is a lightweight wrapper of doQueryDB which adds tracing.
func (r *Resolver) queryDB(ctx context.Context, op search.RepoOptions) (_ dbResolved, _ types.MultiCursor, err error) {
	tr, ctx := trace.New(ctx, "searchrepos.queryDB", attribute.Stringer("opts", &op))
	defer tr.EndWithErr(&err)

	return r.doQueryDB(ctx, tr, op)
}

// resolve will take op and return the resolved RepositoryRevisions and any
// RepoRevSpecs we failed to resolve. Additionally Next is a cursor to the
// next page.
func (r *Resolver) resolve(ctx context.Context, op search.RepoOptions) (_ Resolved, _ types.MultiCursor, errs error) {
	tr, ctx := trace.New(ctx, "searchrepos.Resolve", attribute.Stringer("opts", &op))
	defer tr.EndWithErr(&errs)

	// First we speak to the DB to find the list of repositories.
	result, next, err := r.doQueryDB(ctx, tr, op)
	if err != nil {
		return Resolved{}, nil, err
	}

	// We then speak to gitserver (and others) to convert revspecs into
	// revisions to search.
	resolved, err := r.doFilterDBResolved(ctx, tr, r.searcherGRPCConnectionCache, op, result)
	return resolved, next, err
}

// dbResolved represents the results we can find by speaking to the DB but not
// yet gitserver.
type dbResolved struct {
	Associated []RepoRevSpecs
	Missing    []RepoRevSpecs
}

// doQueryDB is the part of searching op which only requires speaking to the
// DB (before we speak to gitserver).
func (r *Resolver) doQueryDB(ctx context.Context, tr trace.Trace, op search.RepoOptions) (dbResolved, types.MultiCursor, error) {
	excludePatterns := op.MinusRepoFilters
	includePatterns, includePatternRevs := findPatternRevs(op.RepoFilters)

	limit := op.Limit
	if limit == 0 {
		limit = limits.SearchLimits(conf.Get()).MaxRepos
	}

	searchContext, err := searchcontexts.ResolveSearchContextSpec(ctx, r.db, op.SearchContextSpec)
	if err != nil {
		return dbResolved{}, nil, err
	}

	kvpFilters := make([]database.RepoKVPFilter, 0, len(op.HasKVPs))
	for _, filter := range op.HasKVPs {
		kvpFilters = append(kvpFilters, database.RepoKVPFilter{
			Key:     filter.Key,
			Value:   filter.Value,
			Negated: filter.Negated,
			KeyOnly: filter.KeyOnly,
		})
	}

	topicFilters := make([]database.RepoTopicFilter, 0, len(op.HasTopics))
	for _, filter := range op.HasTopics {
		topicFilters = append(topicFilters, database.RepoTopicFilter{
			Topic:   filter.Topic,
			Negated: filter.Negated,
		})
	}

	options := database.ReposListOptions{
		IncludePatterns:       includePatterns,
		ExcludePattern:        query.UnionRegExps(excludePatterns),
		DescriptionPatterns:   op.DescriptionPatterns,
		CaseSensitivePatterns: op.CaseSensitiveRepoFilters,
		KVPFilters:            kvpFilters,
		TopicFilters:          topicFilters,
		Cursors:               op.Cursors,
		// List N+1 repos so we can see if there are repos omitted due to our repo limit.
		LimitOffset:  &database.LimitOffset{Limit: limit + 1},
		NoForks:      op.NoForks,
		OnlyForks:    op.OnlyForks,
		NoArchived:   op.NoArchived,
		OnlyArchived: op.OnlyArchived,
		NoPrivate:    op.Visibility == query.Public,
		OnlyPrivate:  op.Visibility == query.Private,
		OnlyCloned:   op.OnlyCloned,
		OrderBy: database.RepoListOrderBy{
			{
				Field:      database.RepoListStars,
				Descending: true,
				Nulls:      "LAST",
			},
			{
				Field:      database.RepoListID,
				Descending: true,
			},
		},
	}

	// Filter by search context repository revisions only if this search context doesn't have
	// a query, which replaces the context:foo term at query parsing time.
	if searchContext.Query == "" {
		options.SearchContextID = searchContext.ID
	}

	tr.AddEvent("Repos.ListRepos - start")
	repos, err := r.db.Repos().List(ctx, options)
	tr.AddEvent("Repos.ListRepos - done", attribute.Int("numRepos", len(repos)), trace.Error(err))

	if err != nil {
		return dbResolved{}, nil, err
	}

	if len(repos) == 0 && len(op.Cursors) == 0 { // Is the first page empty?
		return dbResolved{}, nil, ErrNoResolvedRepos
	}

	var next types.MultiCursor
	if len(repos) == limit+1 { // Do we have a next page?
		last := repos[len(repos)-1]
		for _, o := range options.OrderBy {
			c := types.Cursor{Column: string(o.Field)}

			switch c.Column {
			case "stars":
				c.Value = strconv.FormatInt(int64(last.Stars), 10)
			case "id":
				c.Value = strconv.FormatInt(int64(last.ID), 10)
			}

			if o.Descending {
				c.Direction = "prev"
			} else {
				c.Direction = "next"
			}

			next = append(next, &c)
		}
		repos = repos[:len(repos)-1]
	}

	var searchContextRepositoryRevisions map[api.RepoID]RepoRevSpecs
	if !searchcontexts.IsAutoDefinedSearchContext(searchContext) && searchContext.Query == "" {
		scRepoRevs, err := searchcontexts.GetRepositoryRevisions(ctx, r.db, searchContext.ID)
		if err != nil {
			return dbResolved{}, nil, err
		}

		searchContextRepositoryRevisions = make(map[api.RepoID]RepoRevSpecs, len(scRepoRevs))
		for _, repoRev := range scRepoRevs {
			revSpecs := make([]query.RevisionSpecifier, 0, len(repoRev.Revs))
			for _, rev := range repoRev.Revs {
				revSpecs = append(revSpecs, query.RevisionSpecifier{RevSpec: rev})
			}
			searchContextRepositoryRevisions[repoRev.Repo.ID] = RepoRevSpecs{
				Repo: repoRev.Repo,
				Revs: revSpecs,
			}
		}
	}

	tr.AddEvent("starting rev association")
	associatedRepoRevs, missingRepoRevs := r.associateReposWithRevs(ctx, repos, searchContextRepositoryRevisions, includePatternRevs)
	tr.AddEvent("completed rev association")

	return dbResolved{
		Associated: associatedRepoRevs,
		Missing:    missingRepoRevs,
	}, next, nil
}

// doFilterDBResolved is what we do after obtaining the list of repos to
// search from the DB. It will potentially reach out to gitserver to convert
// those lists of refs into actual revisions to search (and return
// MissingRepoRevsError for those refs which do not exist).
//
// NOTE: This API is not idiomatic and can return non-nil error with a useful
// Resolved.
func (r *Resolver) doFilterDBResolved(ctx context.Context, tr trace.Trace, searcherGRPCConnectionCache *defaults.ConnectionCache, op search.RepoOptions, result dbResolved) (Resolved, error) {
	// At each step we will discover RepoRevSpecs that do not actually exist.
	// We keep appending to this.
	missing := result.Missing

	filteredRepoRevs, filteredMissing, err := r.filterGitserver(ctx, tr, op, result.Associated)
	if err != nil {
		return Resolved{}, err
	}
	missing = append(missing, filteredMissing...)

	tr.AddEvent("starting contains filtering")
	filteredRepoRevs, missingHasFileContentRevs, backendsMissing, err := r.filterRepoHasFileContent(ctx, searcherGRPCConnectionCache, filteredRepoRevs, op)
	missing = append(missing, missingHasFileContentRevs...)
	if err != nil {
		return Resolved{}, errors.Wrap(err, "filter has file content")
	}
	tr.AddEvent("finished contains filtering")

	return Resolved{
		RepoRevs:        filteredRepoRevs,
		BackendsMissing: backendsMissing,
	}, maybeMissingRepoRevsError(missing)
}

// filterGitserver will take the found associatedRepoRevs and transform them
// into RepositoryRevisions. IE it will communicate with gitserver.
func (r *Resolver) filterGitserver(ctx context.Context, tr trace.Trace, op search.RepoOptions, associatedRepoRevs []RepoRevSpecs) (repoRevs []*search.RepositoryRevisions, missing []RepoRevSpecs, _ error) {
	tr.AddEvent("starting glob expansion")
	normalized, normalizedMissingRepoRevs, err := r.normalizeRefs(ctx, associatedRepoRevs)
	if err != nil {
		return nil, nil, errors.Wrap(err, "normalize refs")
	}
	tr.AddEvent("finished glob expansion")

	tr.AddEvent("starting rev filtering")
	filteredRepoRevs, err := r.filterHasCommitAfter(ctx, normalized, op)
	if err != nil {
		return nil, nil, errors.Wrap(err, "filter has commit after")
	}
	tr.AddEvent("completed rev filtering")

	return filteredRepoRevs, normalizedMissingRepoRevs, nil
}

// associateReposWithRevs re-associates revisions with the repositories fetched from the db
func (r *Resolver) associateReposWithRevs(
	ctx context.Context,
	repos []*types.Repo,
	searchContextRepoRevs map[api.RepoID]RepoRevSpecs,
	includePatternRevs []patternRevspec,
) (
	associated []RepoRevSpecs,
	missing []RepoRevSpecs,
) {
	p := pool.New().WithMaxGoroutines(8)

	associatedRevs := make([]RepoRevSpecs, len(repos))
	revsAreMissing := make([]bool, len(repos))

	c := conf.Get()
	isPerforceChangelistMappingEnabled := c.ExperimentalFeatures != nil && c.ExperimentalFeatures.PerforceChangelistMapping == "enabled"

	for i, repo := range repos {
		i, repo := i, repo
		p.Go(func() {
			var (
				revs      []query.RevisionSpecifier
				isMissing bool
			)

			if len(searchContextRepoRevs) > 0 && len(revs) == 0 {
				if scRepoRev, ok := searchContextRepoRevs[repo.ID]; ok {
					revs = scRepoRev.Revs
				}
			}

			if len(revs) == 0 {
				var clashingRevs []query.RevisionSpecifier
				revs, clashingRevs = getRevsForMatchedRepo(repo.Name, includePatternRevs)

				// if multiple specified revisions clash, report this usefully:
				if len(revs) == 0 && len(clashingRevs) != 0 {
					revs = clashingRevs
					isMissing = true
				}
			}

			if isPerforceChangelistMappingEnabled &&
				repo.ExternalRepo.ServiceType == "perforce" &&
				len(revs) > 0 {
				// If the repo is a perforce repo and the revs looks like changelist ids
				// then we will try to map them to shas
				// Best effort mapping, if we don't find it just leave as it
				tryMapPerforceChangelistIdsToGitShas(ctx, revs, r.db, repo)

			}

			associatedRevs[i] = RepoRevSpecs{Repo: repo.ToMinimalRepo(), Revs: revs}
			revsAreMissing[i] = isMissing
		})
	}

	p.Wait()

	// Sort missing revs to the end, but maintain order otherwise.
	sort.SliceStable(associatedRevs, func(i, j int) bool {
		return !revsAreMissing[i] && revsAreMissing[j]
	})

	notMissingCount := 0
	for _, isMissing := range revsAreMissing {
		if !isMissing {
			notMissingCount++
		}
	}

	return associatedRevs[:notMissingCount], associatedRevs[notMissingCount:]
}

func tryMapPerforceChangelistIdsToGitShas(ctx context.Context, revs []query.RevisionSpecifier, db database.DB, repo *types.Repo) {
	for i := range revs {
		rev := &revs[i]
		if changeListId, err := strconv.ParseInt(rev.RevSpec, 10, 0); err == nil {
			commit, err := db.RepoCommitsChangelists().GetRepoCommitChangelist(ctx, repo.ID, changeListId)

			if err == nil {
				rev.RevSpec = string(commit.CommitSHA)
			}
		}
	}
}

// normalizeRefs handles three jobs:
// 1) expanding each ref glob into a set of refs
// 2) checking that every revision (except HEAD) exists
// 3) expanding the empty string revision (which implicitly means HEAD) into an explicit "HEAD"
func (r *Resolver) normalizeRefs(ctx context.Context, repoRevSpecs []RepoRevSpecs) ([]*search.RepositoryRevisions, []RepoRevSpecs, error) {
	results := make([]*search.RepositoryRevisions, len(repoRevSpecs))

	var (
		mu         sync.Mutex
		missing    []RepoRevSpecs
		addMissing = func(revSpecs RepoRevSpecs) {
			mu.Lock()
			missing = append(missing, revSpecs)
			mu.Unlock()
		}
	)

	p := pool.New().WithContext(ctx).WithMaxGoroutines(128)
	for i, repoRev := range repoRevSpecs {
		i, repoRev := i, repoRev
		p.Go(func(ctx context.Context) error {
			expanded, err := r.normalizeRepoRefs(ctx, repoRev.Repo, repoRev.Revs, addMissing)
			if err != nil {
				return err
			}
			results[i] = &search.RepositoryRevisions{
				Repo: repoRev.Repo,
				Revs: expanded,
			}
			return nil
		})
	}

	if err := p.Wait(); err != nil {
		return nil, nil, err
	}

	// Filter out any results whose revSpecs expanded to nothing
	filteredResults := results[:0]
	for _, result := range results {
		if len(result.Revs) > 0 {
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, missing, nil
}

func (r *Resolver) normalizeRepoRefs(
	ctx context.Context,
	repo types.MinimalRepo,
	revSpecs []query.RevisionSpecifier,
	reportMissing func(RepoRevSpecs),
) ([]string, error) {
	revs := make([]string, 0, len(revSpecs))
	var globs []gitdomain.RefGlob
	for _, rev := range revSpecs {
		switch {
		case rev.RefGlob != "":
			globs = append(globs, gitdomain.RefGlob{Include: rev.RefGlob})
		case rev.ExcludeRefGlob != "":
			globs = append(globs, gitdomain.RefGlob{Exclude: rev.ExcludeRefGlob})
		case rev.RevAtTime != nil:
			commitOID, found, err := r.gitserver.RevAtTime(ctx, repo.Name, rev.RevAtTime.RevSpec, rev.RevAtTime.Timestamp)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.HasType[*gitdomain.BadCommitError](err) {
					return nil, err
				}
				reportMissing(RepoRevSpecs{Repo: repo, Revs: []query.RevisionSpecifier{rev}})
				continue
			}
			if found {
				revs = append(revs, string(commitOID))
			}
		case rev.RevSpec == "" || rev.RevSpec == "HEAD":
			// NOTE: HEAD is the only case here that we don't resolve to a
			// commit ID. We should consider building []gitdomain.Ref here
			// instead of just []string because we have the exact commit hashes,
			// so we could avoid resolving later.
			revs = append(revs, rev.RevSpec)
		case rev.RevSpec != "":
			trimmedRev := strings.TrimPrefix(rev.RevSpec, "^")
			_, err := r.gitserver.ResolveRevision(ctx, repo.Name, trimmedRev, gitserver.ResolveRevisionOptions{EnsureRevision: false})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.HasType[*gitdomain.BadCommitError](err) {
					return nil, err
				}
				reportMissing(RepoRevSpecs{Repo: repo, Revs: []query.RevisionSpecifier{rev}})
				continue
			}
			revs = append(revs, rev.RevSpec)
		}
	}

	if len(globs) == 0 {
		// Happy path with no globs to expand
		return revs, nil
	}

	rg, err := gitdomain.CompileRefGlobs(globs)
	if err != nil {
		return nil, err
	}

	allRefs, err := r.gitserver.ListRefs(ctx, repo.Name, gitserver.ListRefsOpts{})
	if err != nil {
		return nil, err
	}

	for _, ref := range allRefs {
		if rg.Match(ref.Name) {
			revs = append(revs, strings.TrimPrefix(ref.Name, "refs/heads/"))
		}
	}

	return revs, nil

}

// filterHasCommitAfter filters the revisions on each of a set of RepositoryRevisions to ensure that
// any repo-level filters (e.g. `repo:contains.commit.after()`) apply to this repo/rev combo.
func (r *Resolver) filterHasCommitAfter(
	ctx context.Context,
	repoRevs []*search.RepositoryRevisions,
	op search.RepoOptions,
) (
	[]*search.RepositoryRevisions,
	error,
) {
	// Early return if HasCommitAfter is not set
	if op.CommitAfter == nil {
		return repoRevs, nil
	}

	timeRef, err := gitdomain.ParseGitDate(op.CommitAfter.TimeRef, time.Now)
	if err != nil {
		return nil, errors.Wrap(err, "invalid time ref")
	}

	p := pool.New().WithContext(ctx).WithMaxGoroutines(128)

	for _, repoRev := range repoRevs {
		repoRev := repoRev

		allRevs := repoRev.Revs

		var mu sync.Mutex
		repoRev.Revs = make([]string, 0, len(allRevs))

		for _, rev := range allRevs {
			rev := rev
			p.Go(func(ctx context.Context) error {
				if hasCommitAfter, err := hasCommitAfter(ctx, r.gitserver, repoRev.Repo.Name, timeRef, rev); err != nil {
					if errors.HasType[*gitdomain.RevisionNotFoundError](err) || gitdomain.IsRepoNotExist(err) {
						// If the revision does not exist or the repo does not exist,
						// it certainly does not have any commits after some time.
						// Ignore the error, but filter this repo out.
						return nil
					}
					return err
				} else if !op.CommitAfter.Negated && !hasCommitAfter {
					return nil
				} else if op.CommitAfter.Negated && hasCommitAfter {
					return nil
				}

				mu.Lock()
				repoRev.Revs = append(repoRev.Revs, rev)
				mu.Unlock()
				return nil
			})
		}
	}

	if err := p.Wait(); err != nil {
		return nil, err
	}

	// Filter out any repo revs with empty revs
	filteredRepoRevs := repoRevs[:0]
	for _, repoRev := range repoRevs {
		if len(repoRev.Revs) > 0 {
			filteredRepoRevs = append(filteredRepoRevs, repoRev)
		}
	}

	return filteredRepoRevs, nil
}

// hasCommitAfter indicates the staleness of a repository. It returns a boolean indicating if a repository
// contains a commit past a specified date.
func hasCommitAfter(ctx context.Context, gitserverClient gitserver.Client, repoName api.RepoName, timeRef time.Time, revspec string) (bool, error) {
	if revspec == "" {
		revspec = "HEAD"
	}

	// TODO: Because N: 1 currently has a special meaning because of `isRequestForSingleCommit`,
	// we ask for two commits here, but the second one we never actually need.
	// One we figure out why `isRequestForSingleCommit` exists in the first place,
	// we should update this.
	commits, err := gitserverClient.Commits(ctx, repoName, gitserver.CommitsOptions{N: 2, After: timeRef, Ranges: []string{revspec}})
	if err != nil {
		return false, err
	}

	return len(commits) > 0, nil
}

// filterRepoHasFileContent filters a page of repos to only those that match the
// given contains predicates in RepoOptions.HasFileContent.
// Brief overview of the method:
// 1) We partition the set of repos into indexed and unindexed
// 2) We kick off a single zoekt search that handles all the indexed revs
// 3) We kick off a searcher job for the product of every rev * every contains predicate
// 4) We collect the set of revisions that matched all contains predicates and return them.
func (r *Resolver) filterRepoHasFileContent(
	ctx context.Context,
	searcherGRPCConnectionCache *defaults.ConnectionCache,
	repoRevs []*search.RepositoryRevisions,
	op search.RepoOptions,
) (
	_ []*search.RepositoryRevisions,
	_ []RepoRevSpecs,
	_ int,
	err error,
) {
	tr, ctx := trace.New(ctx, "Resolve.FilterHasFileContent")
	tr.SetAttributes(attribute.Int("inputRevCount", len(repoRevs)))
	defer func() {
		tr.SetError(err)
		tr.End()
	}()

	// Early return if there are no filters
	if len(op.HasFileContent) == 0 {
		return repoRevs, nil, 0, nil
	}

	indexed, unindexed, err := searchzoekt.PartitionRepos(
		ctx,
		r.logger,
		repoRevs,
		r.zoekt,
		search.TextRequest,
		op.UseIndex,
		false,
	)
	if err != nil {
		return nil, nil, 0, err
	}

	minimalRepoMap := make(map[api.RepoID]types.MinimalRepo, len(repoRevs))
	for _, repoRev := range repoRevs {
		minimalRepoMap[repoRev.Repo.ID] = repoRev.Repo
	}

	var (
		mu         sync.Mutex
		filtered   = map[api.RepoID]*search.RepositoryRevisions{}
		addRepoRev = func(id api.RepoID, rev string) {
			mu.Lock()
			defer mu.Unlock()
			repoRev := filtered[id]
			if repoRev == nil {
				minimalRepo, ok := minimalRepoMap[id]
				if !ok {
					// Skip any repos that weren't in our requested repos.
					// This should never happen.
					return
				}
				repoRev = &search.RepositoryRevisions{
					Repo: minimalRepo,
				}
			}
			repoRev.Revs = append(repoRev.Revs, rev)
			filtered[id] = repoRev
		}
		backendsMissing    = 0
		addBackendsMissing = func(c int) {
			if c == 0 {
				return
			}
			mu.Lock()
			backendsMissing += c
			mu.Unlock()
		}
	)

	var (
		missingMu  sync.Mutex
		missing    []RepoRevSpecs
		addMissing = func(rs RepoRevSpecs) {
			missingMu.Lock()
			missing = append(missing, rs)
			missingMu.Unlock()
		}
	)

	p := pool.New().WithContext(ctx).WithMaxGoroutines(16)

	{ // Use zoekt for indexed revs
		p.Go(func(ctx context.Context) error {
			type repoAndRev struct {
				id  api.RepoID
				rev string
			}
			var revsMatchingAllPredicates Set[repoAndRev]
			for i, opt := range op.HasFileContent {
				q := searchzoekt.QueryForFileContentArgs(opt, op.CaseSensitiveRepoFilters)
				q = zoektquery.NewAnd(&zoektquery.BranchesRepos{List: indexed.BranchRepos()}, q)

				repos, err := r.zoekt.List(ctx, q, &zoekt.ListOptions{Field: zoekt.RepoListFieldReposMap})
				if err != nil {
					return err
				}

				addBackendsMissing(repos.Crashes)

				foundRevs := Set[repoAndRev]{}
				for repoID, repo := range repos.ReposMap {
					inputRevs := indexed.RepoRevs[api.RepoID(repoID)].Revs
					for _, branch := range repo.Branches {
						for _, inputRev := range inputRevs {
							if branch.Name == inputRev || (branch.Name == "HEAD" && inputRev == "") {
								foundRevs.Add(repoAndRev{id: api.RepoID(repoID), rev: inputRev})
							}
						}
					}
				}

				if i == 0 {
					revsMatchingAllPredicates = foundRevs
				} else {
					revsMatchingAllPredicates.IntersectWith(foundRevs)
				}
			}

			for rr := range revsMatchingAllPredicates {
				addRepoRev(rr.id, rr.rev)
			}
			return nil
		})
	}

	{ // Use searcher for unindexed revs

		checkHasMatches := func(ctx context.Context, arg query.RepoHasFileContentArgs, repo types.MinimalRepo, rev string) (bool, error) {
			commitID, err := r.gitserver.ResolveRevision(ctx, repo.Name, rev, gitserver.ResolveRevisionOptions{EnsureRevision: false})
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.HasType[*gitdomain.BadCommitError](err) {
					return false, err
				} else if e := (&gitdomain.RevisionNotFoundError{}); errors.As(err, &e) && (rev == "HEAD" || rev == "") {
					// In the case that we can't find HEAD, that means there are no commits, which means
					// we can safely say this repo does not have the file being requested.
					return false, nil
				}

				// For any other error, add this repo/rev pair to the set of missing repos
				addMissing(RepoRevSpecs{Repo: repo, Revs: []query.RevisionSpecifier{{RevSpec: rev}}})
				return false, nil
			}

			return r.repoHasFileContentAtCommit(ctx, searcherGRPCConnectionCache, repo, commitID, arg)
		}

		for _, repoRevs := range unindexed {
			for _, rev := range repoRevs.Revs {
				repo, rev := repoRevs.Repo, rev

				p.Go(func(ctx context.Context) error {
					for _, arg := range op.HasFileContent {
						hasMatches, err := checkHasMatches(ctx, arg, repo, rev)
						if err != nil {
							return err
						}

						wantMatches := !arg.Negated
						if wantMatches != hasMatches {
							// One of the conditions has failed, so we can return early
							return nil
						}
					}

					// If we made it here, we found a match for each of the contains filters.
					addRepoRev(repo.ID, rev)
					return nil
				})
			}
		}
	}

	if err := p.Wait(); err != nil {
		return nil, nil, 0, err
	}

	// Filter the input revs to only those that matched all the contains conditions
	matchedRepoRevs := repoRevs[:0]
	for _, repoRev := range repoRevs {
		if matched, ok := filtered[repoRev.Repo.ID]; ok {
			matchedRepoRevs = append(matchedRepoRevs, matched)
		}
	}

	tr.SetAttributes(
		attribute.Int("filteredRevCount", len(matchedRepoRevs)),
		attribute.Int("backendsMissing", backendsMissing))
	return matchedRepoRevs, missing, backendsMissing, nil
}

func (r *Resolver) repoHasFileContentAtCommit(ctx context.Context, searcherGRPCConnectionCache *defaults.ConnectionCache, repo types.MinimalRepo, commitID api.CommitID, args query.RepoHasFileContentArgs) (bool, error) {
	patternInfo := search.TextPatternInfo{
		Query: &protocol.PatternNode{
			Value:     args.Content,
			IsNegated: args.Negated,
			IsRegExp:  true,
		},
		IsCaseSensitive:       false,
		FileMatchLimit:        1,
		PatternMatchesContent: true,
	}

	if args.Path != "" {
		patternInfo.IncludePaths = []string{args.Path}
		patternInfo.PatternMatchesPath = true
	}

	foundMatches := false
	onMatch := func(fm *protocol.FileMatch) {
		foundMatches = true
	}

	_, err := searcher.Search(
		ctx,
		r.searcher,
		searcherGRPCConnectionCache,
		repo.Name,
		repo.ID,
		"", // not using zoekt, don't need branch
		commitID,
		false, // not using zoekt, don't need indexing
		&patternInfo,
		time.Hour,         // depend on context for timeout
		search.Features{}, // not using any search features
		0,                 // don't care about the actual content, so don't fetch extra context
		onMatch,
	)
	return foundMatches, err
}

// computeExcludedRepos computes the ExcludedRepos that the given RepoOptions would not match. This is
// used to show in the search UI what repos are excluded precisely.
func computeExcludedRepos(ctx context.Context, db database.DB, op search.RepoOptions) (ex ExcludedRepos, err error) {
	tr, ctx := trace.New(ctx, "searchrepos.Excluded", attribute.Stringer("opts", &op))
	defer func() {
		tr.SetAttributes(
			attribute.Int("excludedForks", ex.Forks),
			attribute.Int("excludedArchived", ex.Archived))
		tr.EndWithErr(&err)
	}()

	excludePatterns := op.MinusRepoFilters
	includePatterns, _ := findPatternRevs(op.RepoFilters)

	limit := op.Limit
	if limit == 0 {
		limit = limits.SearchLimits(conf.Get()).MaxRepos
	}

	searchContext, err := searchcontexts.ResolveSearchContextSpec(ctx, db, op.SearchContextSpec)
	if err != nil {
		return ExcludedRepos{}, err
	}

	options := database.ReposListOptions{
		IncludePatterns: includePatterns,
		ExcludePattern:  query.UnionRegExps(excludePatterns),
		// List N+1 repos so we can see if there are repos omitted due to our repo limit.
		LimitOffset:     &database.LimitOffset{Limit: limit + 1},
		NoForks:         op.NoForks,
		OnlyForks:       op.OnlyForks,
		NoArchived:      op.NoArchived,
		OnlyArchived:    op.OnlyArchived,
		NoPrivate:       op.Visibility == query.Public,
		OnlyPrivate:     op.Visibility == query.Private,
		SearchContextID: searchContext.ID,
	}

	g, ctx := errgroup.WithContext(ctx)

	var excluded struct {
		sync.Mutex
		ExcludedRepos
	}

	if !op.ForkSet && !ExactlyOneRepo(op.RepoFilters) {
		g.Go(func() error {
			// 'fork:...' was not specified and Forks are excluded, find out
			// which repos are excluded.
			selectForks := options
			selectForks.OnlyForks = true
			selectForks.NoForks = false
			numExcludedForks, err := db.Repos().Count(ctx, selectForks)
			if err != nil {
				return err
			}

			excluded.Lock()
			excluded.Forks = numExcludedForks
			excluded.Unlock()

			return nil
		})
	}

	if !op.ArchivedSet && !ExactlyOneRepo(op.RepoFilters) {
		g.Go(func() error {
			// Archived...: was not specified and archives are excluded,
			// find out which repos are excluded.
			selectArchived := options
			selectArchived.OnlyArchived = true
			selectArchived.NoArchived = false
			numExcludedArchived, err := db.Repos().Count(ctx, selectArchived)
			if err != nil {
				return err
			}

			excluded.Lock()
			excluded.Archived = numExcludedArchived
			excluded.Unlock()

			return nil
		})
	}

	return excluded.ExcludedRepos, g.Wait()
}

// ExactlyOneRepo returns whether exactly one repo: literal field is specified and
// delineated by regex anchors ^ and $. This function helps determine whether we
// should return results for a single repo regardless of whether it is a fork or
// archive.
func ExactlyOneRepo(repoFilters []query.ParsedRepoFilter) bool {
	if len(repoFilters) == 1 {
		repo := repoFilters[0].Repo
		if strings.HasPrefix(repo, "^") && strings.HasSuffix(repo, "$") {
			filter := strings.TrimSuffix(strings.TrimPrefix(repo, "^"), "$")
			r, err := regexpsyntax.Parse(filter, regexpFlags)
			if err != nil {
				return false
			}
			return r.Op == regexpsyntax.OpLiteral
		}
	}
	return false
}

// Cf. golang/go/src/regexp/syntax/parse.go.
const regexpFlags = regexpsyntax.ClassNL | regexpsyntax.PerlX | regexpsyntax.UnicodeGroups

// ExcludedRepos is a type that counts how many repos with a certain label were
// excluded from search results.
type ExcludedRepos struct {
	Forks    int
	Archived int
}

// a patternRevspec maps an include pattern to a list of revisions
// for repos matching that pattern. "map" in this case does not mean
// an actual map, because we want regexp matches, not identity matches.
type patternRevspec struct {
	includePattern *regexp.Regexp
	revs           []query.RevisionSpecifier
}

// given a repo name, determine whether it matched any patterns for which we have
// revspecs (or ref globs), and if so, return the matching/allowed ones.
func getRevsForMatchedRepo(repo api.RepoName, pats []patternRevspec) (matched []query.RevisionSpecifier, clashing []query.RevisionSpecifier) {
	revLists := make([][]query.RevisionSpecifier, 0, len(pats))
	for _, rev := range pats {
		if rev.includePattern.MatchString(string(repo)) {
			revLists = append(revLists, rev.revs)
		}
	}
	// exactly one match: we accept that list
	if len(revLists) == 1 {
		matched = revLists[0]
		return
	}
	// no matches: we generate a dummy list containing only master
	if len(revLists) == 0 {
		matched = []query.RevisionSpecifier{{RevSpec: ""}}
		return
	}
	// if two repo specs match, and both provided non-empty rev lists,
	// we want their intersection, so we count the number of times we
	// see a revision in the rev lists, and make sure it matches the number
	// of rev lists
	revCounts := make(map[query.RevisionSpecifier]int, len(revLists[0]))

	var aliveCount int
	for i, revList := range revLists {
		aliveCount = 0
		for _, rev := range revList {
			if revCounts[rev] == i {
				aliveCount += 1
			}
			revCounts[rev] += 1
		}
	}

	if aliveCount > 0 {
		matched = make([]query.RevisionSpecifier, 0, len(revCounts))
		for rev, seenCount := range revCounts {
			if seenCount == len(revLists) {
				matched = append(matched, rev)
			}
		}
		slices.SortFunc(matched, query.RevisionSpecifier.Compare)
		return
	}

	clashing = make([]query.RevisionSpecifier, 0, len(revCounts))
	for rev := range revCounts {
		clashing = append(clashing, rev)
	}
	// ensure that lists are always returned in sorted order.
	slices.SortFunc(clashing, query.RevisionSpecifier.Compare)
	return
}

// findPatternRevs separates out each repo filter into its repository name
// pattern and its revision specs (if any). It also applies small optimizations
// to the repository name.
func findPatternRevs(includePatterns []query.ParsedRepoFilter) (outputPatterns []string, includePatternRevs []patternRevspec) {
	outputPatterns = make([]string, 0, len(includePatterns))
	includePatternRevs = make([]patternRevspec, 0, len(includePatterns))

	for _, pattern := range includePatterns {
		repo, repoRegex, revs := pattern.Repo, pattern.RepoRegex, pattern.Revs
		repo = optimizeRepoPatternWithHeuristics(repo)
		outputPatterns = append(outputPatterns, repo)

		if len(revs) > 0 {
			patternRev := patternRevspec{includePattern: repoRegex, revs: revs}
			includePatternRevs = append(includePatternRevs, patternRev)
		}
	}
	return
}

func optimizeRepoPatternWithHeuristics(repoPattern string) string {
	if dotcom.SourcegraphDotComMode() && (strings.HasPrefix(repoPattern, "github.com") || strings.HasPrefix(repoPattern, `github\.com`)) {
		repoPattern = "^" + repoPattern
	}
	// Optimization: make the "." in "github.com" a literal dot
	// so that the regexp can be optimized more effectively.
	repoPattern = strings.ReplaceAll(repoPattern, "github.com", `github\.com`)
	return repoPattern
}

var ErrNoResolvedRepos = errors.New("no resolved repositories")

func maybeMissingRepoRevsError(missing []RepoRevSpecs) error {
	if len(missing) > 0 {
		return &MissingRepoRevsError{
			Missing: missing,
		}
	}
	return nil
}

type MissingRepoRevsError struct {
	Missing []RepoRevSpecs
}

func (MissingRepoRevsError) Error() string { return "missing repo revs" }

type RepoRevSpecs struct {
	Repo types.MinimalRepo
	Revs []query.RevisionSpecifier
}

func (r *RepoRevSpecs) RevSpecs() []string {
	res := make([]string, 0, len(r.Revs))
	for _, rev := range r.Revs {
		switch {
		case rev.RefGlob != "":
		case rev.ExcludeRefGlob != "":
		default:
			res = append(res, rev.RevSpec)
		}
	}
	return res
}

// Set is a small helper utility for a unique set of objects
type Set[T comparable] map[T]struct{}

func (s Set[T]) Add(t T) {
	s[t] = struct{}{}
}

// IntersectWith mutates `s`, removing any elements not in `other`
func (s Set[T]) IntersectWith(other Set[T]) {
	for k := range s {
		if _, ok := other[k]; !ok {
			delete(s, k)
		}
	}
}
