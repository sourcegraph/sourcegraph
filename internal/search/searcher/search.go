package searcher

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/grafana/regexp"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/sourcegraph/log"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/endpoint"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/mutablelimiter"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// A global limiter on number of concurrent searcher searches.
var textSearchLimiter = mutablelimiter.New(32)

type TextSearchJob struct {
	PatternInfo *search.TextPatternInfo
	Repos       []*search.RepositoryRevisions // the set of repositories to search with searcher.

	TextPatternRegexps []*regexp.Regexp // used for getting file path and repo name match ranges

	// Indexed represents whether the set of repositories are indexed (used
	// to communicate whether searcher should call Zoekt search on these
	// repos).
	Indexed bool

	// UseFullDeadline indicates that the search should try do as much work as
	// it can within context.Deadline. If false the search should try and be
	// as fast as possible, even if a "slow" deadline is set.
	//
	// For example searcher will wait to full its archive cache for a
	// repository if this field is true. Another example is we set this field
	// to true if the user requests a specific timeout or maximum result size.
	UseFullDeadline bool

	Features search.Features
}

// Run calls the searcher service on a set of repositories.
func (s *TextSearchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	tr, ctx, stream, finish := job.StartSpan(ctx, stream, s)
	defer func() { finish(alert, err) }()

	var fetchTimeout time.Duration
	if len(s.Repos) == 1 || s.UseFullDeadline {
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
		otlog.Int64("repos_count", int64(len(s.Repos))),
	)

	if len(s.Repos) == 0 {
		return nil, nil
	}

	// The number of searcher endpoints can change over time. Inform our
	// limiter of the new limit, which is a multiple of the number of
	// searchers.
	eps, err := clients.SearcherURLs.Endpoints()
	if err != nil {
		return nil, err
	}
	textSearchLimiter.SetLimit(len(eps) * 32)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		for _, repoAllRevs := range s.Repos {
			repo := repoAllRevs.Repo // capture repo
			if len(repoAllRevs.Revs) == 0 {
				continue
			}

			for _, rev := range repoAllRevs.Revs {
				rev := rev // capture rev
				limitCtx, limitDone, err := textSearchLimiter.Acquire(ctx)
				if err != nil {
					return err
				}

				g.Go(func() error {
					ctx, done := limitCtx, limitDone
					defer done()

					repoLimitHit, err := s.searchFilesInRepo(ctx, clients.DB, clients.SearcherURLs, repo, repo.Name, rev, s.Indexed, s.PatternInfo, fetchTimeout, stream)
					if err != nil {
						tr.LogFields(otlog.String("repo", string(repo.Name)), otlog.Error(err), otlog.Bool("timeout", errcode.IsTimeout(err)), otlog.Bool("temporary", errcode.IsTemporary(err)))
						clients.Logger.Warn("searchFilesInRepo failed", log.Error(err), log.String("repo", string(repo.Name)))
					}
					// non-diff search reports timeout through err, so pass false for timedOut
					status, limitHit, err := search.HandleRepoSearchResult(repo.ID, []string{rev}, repoLimitHit, false, err)
					stream.Send(streaming.SearchEvent{
						Stats: streaming.Stats{
							Status:     status,
							IsLimitHit: limitHit,
						},
					})
					return err
				})
			}
		}

		return nil
	})

	return nil, g.Wait()
}

func (s *TextSearchJob) Name() string {
	return "SearcherTextSearchJob"
}

func (s *TextSearchJob) Fields(v job.Verbosity) (res []otlog.Field) {
	switch v {
	case job.VerbosityMax:
		res = append(res,
			otlog.Bool("useFullDeadline", s.UseFullDeadline),
			trace.Scoped("patternInfo", s.PatternInfo.Fields()...),
			otlog.Int("numRepos", len(s.Repos)),
			otlog.Object("textPatternRegexps", s.TextPatternRegexps),
		)
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			otlog.Bool("indexed", s.Indexed),
		)
	}
	return res
}

func (s *TextSearchJob) Children() []job.Describer       { return nil }
func (s *TextSearchJob) MapChildren(job.MapFunc) job.Job { return s }

var MockSearchFilesInRepo func(
	ctx context.Context,
	repo types.MinimalRepo,
	gitserverRepo api.RepoName,
	rev string,
	info *search.TextPatternInfo,
	fetchTimeout time.Duration,
	stream streaming.Sender,
) (limitHit bool, err error)

func (s *TextSearchJob) searchFilesInRepo(
	ctx context.Context,
	db database.DB,
	searcherURLs *endpoint.Map,
	repo types.MinimalRepo,
	gitserverRepo api.RepoName,
	rev string,
	index bool,
	info *search.TextPatternInfo,
	fetchTimeout time.Duration,
	stream streaming.Sender,
) (bool, error) {
	if MockSearchFilesInRepo != nil {
		return MockSearchFilesInRepo(ctx, repo, gitserverRepo, rev, info, fetchTimeout, stream)
	}

	// Do not trigger a repo-updater lookup (e.g.,
	// backend.{GitRepo,Repos.ResolveRev}) because that would slow this operation
	// down by a lot (if we're looping over many repos). This means that it'll fail if a
	// repo is not on gitserver.
	commit, err := gitserver.NewClient(db).ResolveRevision(ctx, gitserverRepo, rev, gitserver.ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return false, err
	}

	// Structural and hybrid search both speak to zoekt so need the endpoints.
	var indexerEndpoints []string
	if info.IsStructuralPat || s.Features.HybridSearch {
		indexerEndpoints, err = search.Indexers().Map.Endpoints()
		if err != nil {
			return false, err
		}
	}

	onMatches := func(searcherMatches []*protocol.FileMatch) {
		stream.Send(streaming.SearchEvent{
			Results: convertMatches(repo, commit, &rev, searcherMatches, s.TextPatternRegexps, s.PatternInfo),
		})
	}

	return Search(ctx, searcherURLs, gitserverRepo, repo.ID, rev, commit, index, info, fetchTimeout, indexerEndpoints, s.Features, onMatches)
}

// convert converts a set of searcher matches into []result.Match
func convertMatches(repo types.MinimalRepo, commit api.CommitID, rev *string, searcherMatches []*protocol.FileMatch, textPatternRegexps []*regexp.Regexp, patternInfo *search.TextPatternInfo) []result.Match {
	matches := make([]result.Match, 0, len(searcherMatches))
	for _, fm := range searcherMatches {
		if patternInfo.Select.Root() == filter.Repository {
			matches = append(matches, &result.RepoMatch{
				Name:            repo.Name,
				ID:              repo.ID,
				RepoNameMatches: protocolFileMatchFieldToMatchRanges(string(repo.Name), textPatternRegexps),
			})
			continue
		}

		chunkMatches := make(result.ChunkMatches, 0, len(fm.ChunkMatches))

		for _, cm := range fm.ChunkMatches {
			ranges := make(result.Ranges, 0, len(cm.Ranges))
			for _, rr := range cm.Ranges {
				ranges = append(ranges, result.Range{
					Start: result.Location{
						Offset: int(rr.Start.Offset),
						Line:   int(rr.Start.Line),
						Column: int(rr.Start.Column),
					},
					End: result.Location{
						Offset: int(rr.End.Offset),
						Line:   int(rr.End.Line),
						Column: int(rr.End.Column),
					},
				})
			}

			chunkMatches = append(chunkMatches, result.ChunkMatch{
				Content: cm.Content,
				ContentStart: result.Location{
					Offset: int(cm.ContentStart.Offset),
					Line:   int(cm.ContentStart.Line),
					Column: 0,
				},
				Ranges: ranges,
			})
		}

		matches = append(matches, &result.FileMatch{
			File: result.File{
				Path:     fm.Path,
				Repo:     repo,
				CommitID: commit,
				InputRev: rev,
			},
			ChunkMatches: chunkMatches,
			PathMatches:  protocolFileMatchFieldToMatchRanges(fm.Path, textPatternRegexps),
			LimitHit:     fm.LimitHit,
		})
	}
	return matches
}

func protocolFileMatchFieldToMatchRanges(field string, textPatternRegexps []*regexp.Regexp) (res []result.Range) {
	for _, pathRe := range textPatternRegexps {
		pathSubmatches := pathRe.FindAllStringSubmatchIndex(field, -1)
		for _, sm := range pathSubmatches {
			res = append(res, result.Range{
				Start: result.Location{
					Offset: sm[0],
					Line:   0,
					Column: utf8.RuneCountInString(field[:sm[0]]),
				},
				End: result.Location{
					Offset: sm[1],
					Line:   0,
					Column: utf8.RuneCountInString(field[:sm[1]]),
				},
			})
		}
	}
	return res
}
