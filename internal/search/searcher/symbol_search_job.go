package searcher

import (
	"context"
	"sort"

	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/symbols"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type SymbolSearchJob struct {
	Request *SymbolSearchRequest
	Repos   []*search.RepositoryRevisions // the set of repositories to search with searcher.
	Limit   int
}

// Run calls the searcher service to search symbols.
func (s *SymbolSearchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	tr, ctx, stream, finish := job.StartSpan(ctx, stream, s)
	defer func() { finish(alert, err) }()

	p := pool.New().
		WithContext(ctx).
		WithCancelOnError().
		WithFirstError().
		WithMaxGoroutines(conf.SearchSymbolsParallelism())

	for _, repoRevs := range s.Repos {
		repoRevs := repoRevs
		if ctx.Err() != nil {
			break
		}
		if len(repoRevs.Revs) == 0 {
			continue
		}

		p.Go(func(ctx context.Context) error {
			matches, limitHit, err := searchInRepo(ctx, clients.Gitserver, repoRevs, s.Request, s.Limit)
			isLimitHit := len(matches) > s.Limit || limitHit
			status, err := search.HandleRepoSearchResult(repoRevs.Repo.ID, repoRevs.Revs, isLimitHit, false, err)
			stream.Send(streaming.SearchEvent{
				Results: matches,
				Stats: streaming.Stats{
					Status:     status,
					IsLimitHit: isLimitHit,
				},
			})
			if err != nil {
				tr.SetAttributes(repoRevs.Repo.Name.Attr(), trace.Error(err))
			}
			return err
		})
	}

	return nil, p.Wait()
}

func (s *SymbolSearchJob) Name() string {
	return "SearcherSymbolSearchJob"
}

func (s *SymbolSearchJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		fallthrough
	case job.VerbosityBasic:
		res = append(res, trace.Scoped("request", s.Request.Fields()...)...)
		res = append(res,
			attribute.Int("numRepos", len(s.Repos)),
			attribute.Int("limit", s.Limit),
		)
	}
	return res
}

func (s *SymbolSearchJob) Children() []job.Describer       { return nil }
func (s *SymbolSearchJob) MapChildren(job.MapFunc) job.Job { return s }

func searchInRepo(ctx context.Context, gitserverClient gitserver.Client, repoRevs *search.RepositoryRevisions, request *SymbolSearchRequest, limit int) (res []result.Match, limitHit bool, err error) {
	inputRev := repoRevs.Revs[0]
	tr, ctx := trace.New(ctx, "symbols.searchInRepo",
		repoRevs.Repo.Name.Attr(),
		attribute.String("rev", inputRev))
	defer tr.EndWithErr(&err)

	// Do not trigger a repo-updater lookup (e.g.,
	// backend.{GitRepo,Repos.ResolveRev}) because that would slow this operation
	// down by a lot (if we're looping over many repos). This means that it'll fail if a
	// repo is not on gitserver.
	commitID, err := gitserverClient.ResolveRevision(ctx, repoRevs.GitserverRepo(), inputRev, gitserver.ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return nil, false, err
	}
	tr.SetAttributes(commitID.Attr())

	symbols, limitHit, err := symbols.DefaultClient.Search(ctx, search.SymbolsParameters{
		Repo:            repoRevs.Repo.Name,
		CommitID:        commitID,
		Query:           request.RegexpPattern,
		IsCaseSensitive: request.IsCaseSensitive,
		IsRegExp:        true,
		IncludePatterns: request.IncludePatterns,
		ExcludePattern:  request.ExcludePattern,
		IncludeLangs:    request.IncludeLangs,
		ExcludeLangs:    request.ExcludeLangs,
		// Ask for limit + 1 so we can detect whether there are more results than the limit.
		First: limit + 1,
	})
	if err != nil {
		return nil, false, err
	}

	for i := range symbols {
		symbols[i].Line += 1 // callers expect 1-indexed lines
	}

	// All symbols are from the same repo, so we can just partition them by path
	// to build file matches
	return symbolsToMatches(symbols, repoRevs.Repo, commitID, inputRev), limitHit, err
}

func symbolsToMatches(symbols []result.Symbol, repo types.MinimalRepo, commitID api.CommitID, inputRev string) result.Matches {
	type pathAndLanguage struct {
		path     string
		language string
	}
	symbolsByPath := make(map[pathAndLanguage][]result.Symbol)
	for _, symbol := range symbols {
		cur := symbolsByPath[pathAndLanguage{symbol.Path, symbol.Language}]
		symbolsByPath[pathAndLanguage{symbol.Path, symbol.Language}] = append(cur, symbol)
	}

	// Create file matches from partitioned symbols
	matches := make(result.Matches, 0, len(symbolsByPath))
	for pl, symbols := range symbolsByPath {
		file := result.File{
			Path:            pl.path,
			Repo:            repo,
			CommitID:        commitID,
			InputRev:        &inputRev,
			PreciseLanguage: pl.language,
		}

		symbolMatches := make([]*result.SymbolMatch, 0, len(symbols))
		for _, symbol := range symbols {
			symbolMatches = append(symbolMatches, &result.SymbolMatch{
				File:   &file,
				Symbol: symbol,
			})
		}

		matches = append(matches, &result.FileMatch{
			Symbols: symbolMatches,
			File:    file,
		})
	}

	// Make the results deterministic
	sort.Sort(matches)
	return matches
}

// SymbolSearchRequest defines a symbol search. It's only used to build the job tree,
// and is converted to search.SymbolsParameters when calling the symbols client.
type SymbolSearchRequest struct {
	RegexpPattern   string
	IsCaseSensitive bool
	IncludePatterns []string
	ExcludePattern  string
	IncludeLangs    []string
	ExcludeLangs    []string
}

func (r *SymbolSearchRequest) Fields() []attribute.KeyValue {
	res := make([]attribute.KeyValue, 0, 4)
	add := func(fs ...attribute.KeyValue) {
		res = append(res, fs...)
	}

	add(attribute.String("pattern", r.RegexpPattern))
	if r.IsCaseSensitive {
		add(attribute.Bool("isCaseSensitive", r.IsCaseSensitive))
	}

	if len(r.IncludePatterns) > 0 {
		add(attribute.StringSlice("includePatterns", r.IncludePatterns))
	}
	if r.ExcludePattern != "" {
		add(attribute.String("excludePattern", r.ExcludePattern))
	}
	if len(r.IncludeLangs) > 0 {
		add(attribute.StringSlice("includeLangs", r.IncludeLangs))
	}
	if len(r.ExcludeLangs) > 0 {
		add(attribute.StringSlice("excludeLangs", r.ExcludeLangs))
	}
	return res
}
