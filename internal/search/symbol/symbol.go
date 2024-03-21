package symbol

import (
	"context"
	"regexp/syntax" //nolint:depguard // zoekt requires this pkg
	"sync"
	"time"

	"github.com/RoaringBitmap/roaring"
	"github.com/grafana/regexp"
	"github.com/sourcegraph/zoekt"
	zoektquery "github.com/sourcegraph/zoekt/query"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/symbols"
	"github.com/sourcegraph/sourcegraph/internal/trace/policy"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const DefaultSymbolLimit = 100

// NOTE: this lives inside a syncx.OnceValue because search.Indexed depends on
// conf.Get, and running conf.Get() at init time can cause a deadlock. So,
// we construct it lazily instead.
var DefaultZoektSymbolsClient = sync.OnceValue(func() *ZoektSymbolsClient {
	return &ZoektSymbolsClient{
		subRepoPermsChecker: authz.DefaultSubRepoPermsChecker,
		zoektStreamer:       search.Indexed(),
		symbols:             symbols.DefaultClient,
	}
})

type ZoektSymbolsClient struct {
	subRepoPermsChecker authz.SubRepoPermissionChecker
	zoektStreamer       zoekt.Streamer
	symbols             *symbols.Client
}

func (s *ZoektSymbolsClient) Compute(ctx context.Context, repoName types.MinimalRepo, commitID api.CommitID, inputRev *string, query *string, first *int32, includePatterns *[]string) (res []*result.SymbolMatch, err error) {
	// TODO(keegancsmith) we should be able to use indexedSearchRequest here
	// and remove indexedSymbolsBranch.
	if branch := indexedSymbolsBranch(ctx, s.zoektStreamer, &repoName, string(commitID)); branch != "" {
		results, err := searchZoekt(ctx, s.zoektStreamer, repoName, commitID, inputRev, branch, query, first, includePatterns)
		if err != nil {
			return nil, errors.Wrap(err, "zoekt symbol search")
		}
		results, err = filterZoektResults(ctx, s.subRepoPermsChecker, repoName.Name, results)
		if err != nil {
			return nil, errors.Wrap(err, "checking permissions")
		}
		return results, nil
	}
	serverTimeout := 5 * time.Second
	clientTimeout := 2 * serverTimeout

	ctx, done := context.WithTimeout(ctx, clientTimeout)
	defer done()
	defer func() {
		if ctx.Err() != nil && len(res) == 0 {
			err = errors.Newf("The symbols service appears unresponsive, check the logs for errors.")
		}
	}()
	var includePatternsSlice []string
	if includePatterns != nil {
		includePatternsSlice = *includePatterns
	}

	searchArgs := search.SymbolsParameters{
		CommitID:        commitID,
		First:           limitOrDefault(first) + 1, // add 1 so we can determine PageInfo.hasNextPage
		Repo:            repoName.Name,
		IncludePatterns: includePatternsSlice,
		Timeout:         serverTimeout,
	}
	if query != nil {
		searchArgs.Query = *query
	}

	// We ignore LimitHit, which is consistent with how we treat stats coming
	// from Zoekt in indexedSymbolsBranch.
	symbols, _, err := s.symbols.Search(ctx, searchArgs)
	if err != nil {
		return nil, err
	}

	for i := range symbols {
		symbols[i].Line += 1 // callers expect 1-indexed lines
	}

	fileWithPathAndLanguage := func(path, language string) *result.File {
		return &result.File{
			Path:            path,
			Repo:            repoName,
			InputRev:        inputRev,
			CommitID:        commitID,
			PreciseLanguage: language,
		}
	}

	matches := make([]*result.SymbolMatch, 0, len(symbols))
	for _, symbol := range symbols {
		matches = append(matches, &result.SymbolMatch{
			Symbol: symbol,
			File:   fileWithPathAndLanguage(symbol.Path, symbol.Language),
		})
	}
	return matches, err
}

// GetMatchAtLineCharacter retrieves the shortest matching symbol (if exists) defined
// at a specific line number and character offset in the provided file.
func (s *ZoektSymbolsClient) GetMatchAtLineCharacter(ctx context.Context, repo types.MinimalRepo, commitID api.CommitID, filePath string, line int, character int) (*result.SymbolMatch, error) {
	// Should be large enough to include all symbols from a single file
	first := int32(999999)
	emptyString := ""
	includePatterns := []string{regexp.QuoteMeta(filePath)}
	symbolMatches, err := s.Compute(ctx, repo, commitID, &emptyString, &emptyString, &first, &includePatterns)
	if err != nil {
		return nil, err
	}

	var match *result.SymbolMatch
	for _, symbolMatch := range symbolMatches {
		symbolRange := symbolMatch.Symbol.Range()
		isWithinRange := line >= symbolRange.Start.Line && character >= symbolRange.Start.Character && line <= symbolRange.End.Line && character <= symbolRange.End.Character
		if isWithinRange && (match == nil || len(symbolMatch.Symbol.Name) < len(match.Symbol.Name)) {
			match = symbolMatch
		}
	}
	return match, nil
}

// indexedSymbols checks to see if Zoekt has indexed symbols information for a
// repository at a specific commit. If it has it returns the branch name (for
// use when querying zoekt). Otherwise an empty string is returned.
func indexedSymbolsBranch(ctx context.Context, zs zoekt.Searcher, repo *types.MinimalRepo, commit string) string {
	// We use ListAllIndexed since that is cached.
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	list, err := search.ListAllIndexed(ctx, zs)
	if err != nil {
		return ""
	}

	r, ok := list.ReposMap[uint32(repo.ID)]
	if !ok || !r.HasSymbols {
		return ""
	}

	for _, branch := range r.Branches {
		if branch.Version == commit {
			return branch.Name
		}
	}

	return ""
}

func filterZoektResults(ctx context.Context, checker authz.SubRepoPermissionChecker, repo api.RepoName, results []*result.SymbolMatch) ([]*result.SymbolMatch, error) {
	if !authz.SubRepoEnabled(checker) {
		return results, nil
	}
	// Filter out results from files we don't have access to:
	act := actor.FromContext(ctx)
	filtered := results[:0]
	for i, r := range results {
		ok, err := authz.FilterActorPath(ctx, checker, act, repo, r.File.Path)
		if err != nil {
			return nil, errors.Wrap(err, "checking permissions")
		}
		if ok {
			filtered = append(filtered, results[i])
		}
	}
	return filtered, nil
}

func searchZoekt(
	ctx context.Context,
	z zoekt.Searcher,
	repoName types.MinimalRepo,
	commitID api.CommitID,
	inputRev *string,
	branch string,
	queryString *string,
	first *int32,
	includePatterns *[]string,
) (res []*result.SymbolMatch, err error) {
	var raw string
	if queryString != nil {
		raw = *queryString
	}
	if raw == "" {
		raw = ".*"
	}

	expr, err := syntax.Parse(raw, syntax.ClassNL|syntax.PerlX|syntax.UnicodeGroups)
	if err != nil {
		return
	}

	var query zoektquery.Q
	if expr.Op == syntax.OpLiteral {
		query = &zoektquery.Substring{
			Pattern: string(expr.Rune),
			Content: true,
		}
	} else {
		query = &zoektquery.Regexp{
			Regexp:  expr,
			Content: true,
		}
	}

	ands := []zoektquery.Q{
		&zoektquery.BranchesRepos{List: []zoektquery.BranchRepos{
			{Branch: branch, Repos: roaring.BitmapOf(uint32(repoName.ID))},
		}},
		&zoektquery.Symbol{Expr: query},
	}
	if includePatterns != nil {
		for _, p := range *includePatterns {
			q, err := zoektutil.FileRe(p, true)
			if err != nil {
				return nil, err
			}
			ands = append(ands, q)
		}
	}

	final := zoektquery.Simplify(zoektquery.NewAnd(ands...))
	match := limitOrDefault(first) + 1
	resp, err := z.Search(ctx, final, &zoekt.SearchOptions{
		Trace:              policy.ShouldTrace(ctx),
		MaxWallTime:        3 * time.Second,
		ShardMaxMatchCount: match * 25,
		TotalMaxMatchCount: match * 25,
		MaxDocDisplayCount: match,
		ChunkMatches:       true,
		NumContextLines:    0,
	})
	if err != nil {
		return nil, err
	}

	for _, file := range resp.Files {
		newFile := &result.File{
			Repo:            repoName,
			CommitID:        commitID,
			InputRev:        inputRev,
			Path:            file.FileName,
			PreciseLanguage: file.Language,
		}

		for _, cm := range file.ChunkMatches {
			if cm.FileName || len(cm.SymbolInfo) == 0 {
				continue
			}

			for i, r := range cm.Ranges {
				si := cm.SymbolInfo[i]
				if si == nil {
					continue
				}

				res = append(res, result.NewSymbolMatch(
					newFile,
					int(r.Start.LineNumber),
					int(r.Start.Column),
					si.Sym,
					si.Kind,
					si.Parent,
					si.ParentKind,
					file.Language,
					"", // unused when column is set
					false,
				))
			}
		}
	}
	return
}

func limitOrDefault(first *int32) int {
	if first == nil {
		return DefaultSymbolLimit
	}
	return int(*first)
}
