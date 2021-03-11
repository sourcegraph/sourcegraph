package graphqlbackend

import (
	"context"
	"errors"
	"regexp/syntax"
	"strings"
	"time"

	"github.com/google/zoekt"
	zoektquery "github.com/google/zoekt/query"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gituri"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	zoektutil "github.com/sourcegraph/sourcegraph/internal/search/zoekt"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

type symbolsArgs struct {
	graphqlutil.ConnectionArgs
	Query           *string
	IncludePatterns *[]string
}

func (r *GitTreeEntryResolver) Symbols(ctx context.Context, args *symbolsArgs) (*symbolConnectionResolver, error) {
	symbols, err := computeSymbols(ctx, r.db, r.commit, args.Query, args.First, args.IncludePatterns)
	if err != nil && len(symbols) == 0 {
		return nil, err
	}
	return &symbolConnectionResolver{symbols: symbols, first: args.First}, nil
}

func (r *GitCommitResolver) Symbols(ctx context.Context, args *symbolsArgs) (*symbolConnectionResolver, error) {
	symbols, err := computeSymbols(ctx, r.db, r, args.Query, args.First, args.IncludePatterns)
	if err != nil && len(symbols) == 0 {
		return nil, err
	}
	return &symbolConnectionResolver{symbols: symbols, first: args.First}, nil
}

type symbolConnectionResolver struct {
	first   *int32
	symbols []symbolResolver
}

func limitOrDefault(first *int32) int {
	if first == nil {
		return 100
	}
	return int(*first)
}

// indexedSymbols checks to see if Zoekt has indexed symbols information for a
// repository at a specific commit. If it has it returns the branch name (for
// use when querying zoekt). Otherwise an empty string is returned.
func indexedSymbolsBranch(ctx context.Context, repository, commit string) string {
	z := search.Indexed()
	if !z.Enabled() {
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	set, err := z.ListAll(ctx)
	if err != nil {
		return ""
	}

	repo, ok := set[repository]
	if !ok || !repo.HasSymbols {
		return ""
	}

	for _, branch := range repo.Branches {
		if branch.Version == commit {
			return branch.Name
		}
	}

	return ""
}

func searchZoektSymbols(ctx context.Context, db dbutil.DB, commit *GitCommitResolver, branch string, queryString *string, first *int32, includePatterns *[]string) (res []symbolResolver, err error) {
	raw := *queryString
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
		&zoektquery.RepoBranches{Set: map[string][]string{
			commit.repoResolver.Name(): {branch},
		}},
		&zoektquery.Symbol{Expr: query},
	}
	for _, p := range *includePatterns {
		q, err := zoektutil.FileRe(p, true)
		if err != nil {
			return nil, err
		}
		ands = append(ands, q)
	}

	final := zoektquery.Simplify(zoektquery.NewAnd(ands...))
	match := limitOrDefault(first) + 1
	resp, err := search.Indexed().Client.Search(ctx, final, &zoekt.SearchOptions{
		Trace:                  ot.ShouldTrace(ctx),
		MaxWallTime:            3 * time.Second,
		ShardMaxMatchCount:     match * 25,
		TotalMaxMatchCount:     match * 25,
		ShardMaxImportantMatch: match * 25,
		TotalMaxImportantMatch: match * 25,
		MaxDocDisplayCount:     match,
	})
	if err != nil {
		return nil, err
	}

	baseURI, err := gituri.Parse("git://" + commit.repoResolver.Name() + "?" + string(commit.oid))
	for _, file := range resp.Files {
		for _, l := range file.LineMatches {
			if l.FileName {
				continue
			}

			for _, m := range l.LineFragments {
				if m.SymbolInfo == nil {
					continue
				}

				res = append(res, toSymbolResolver(
					db,
					commit,
					&result.SymbolMatch{
						Symbol: result.Symbol{
							Name:       m.SymbolInfo.Sym,
							Kind:       m.SymbolInfo.Kind,
							Parent:     m.SymbolInfo.Parent,
							ParentKind: m.SymbolInfo.ParentKind,
							Path:       file.FileName,
							Line:       l.LineNumber,
						},
						BaseURI: baseURI,
						Lang:    strings.ToLower(file.Language),
					},
				))
			}
		}
	}
	return
}

func computeSymbols(ctx context.Context, db dbutil.DB, commit *GitCommitResolver, query *string, first *int32, includePatterns *[]string) (res []symbolResolver, err error) {
	// TODO(keegancsmith) we should be able to use indexedSearchRequest here
	// and remove indexedSymbolsBranch.
	if branch := indexedSymbolsBranch(ctx, commit.repoResolver.Name(), string(commit.oid)); branch != "" {
		return searchZoektSymbols(ctx, db, commit, branch, query, first, includePatterns)
	}

	ctx, done := context.WithTimeout(ctx, 5*time.Second)
	defer done()
	defer func() {
		if ctx.Err() != nil && len(res) == 0 {
			err = errors.New("processing symbols is taking longer than expected. Try again in a while")
		}
	}()
	var includePatternsSlice []string
	if includePatterns != nil {
		includePatternsSlice = *includePatterns
	}

	searchArgs := search.SymbolsParameters{
		CommitID:        api.CommitID(commit.oid),
		First:           limitOrDefault(first) + 1, // add 1 so we can determine PageInfo.hasNextPage
		Repo:            commit.repoResolver.name,
		IncludePatterns: includePatternsSlice,
	}
	if query != nil {
		searchArgs.Query = *query
	}
	baseURI, err := gituri.Parse("git://" + commit.repoResolver.Name() + "?" + string(commit.oid))
	if err != nil {
		return nil, err
	}
	symbols, err := backend.Symbols.ListTags(ctx, searchArgs)
	if baseURI == nil {
		return
	}
	resolvers := make([]symbolResolver, 0, len(symbols))
	for _, symbol := range symbols {
		sr := result.SymbolMatch{
			Symbol:  symbol,
			BaseURI: baseURI,
			Lang:    strings.ToLower(symbol.Language),
		}
		resolver := toSymbolResolver(db, commit, &sr)
		resolvers = append(resolvers, resolver)
	}
	return resolvers, err
}

func toSymbolResolver(db dbutil.DB, commit *GitCommitResolver, sr *result.SymbolMatch) symbolResolver {
	return symbolResolver{
		db:          db,
		commit:      commit,
		SymbolMatch: sr,
	}
}

func (r *symbolConnectionResolver) Nodes(ctx context.Context) ([]symbolResolver, error) {
	symbols := r.symbols
	if len(r.symbols) > limitOrDefault(r.first) {
		symbols = symbols[:limitOrDefault(r.first)]
	}
	return symbols, nil
}

func (r *symbolConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	return graphqlutil.HasNextPage(len(r.symbols) > limitOrDefault(r.first)), nil
}

type symbolResolver struct {
	db     dbutil.DB
	commit *GitCommitResolver
	*result.SymbolMatch
}

func (r symbolResolver) Name() string { return r.Symbol.Name }

func (r symbolResolver) ContainerName() *string {
	if r.Symbol.Parent == "" {
		return nil
	}
	return &r.Symbol.Parent
}

func (r symbolResolver) Kind() string /* enum SymbolKind */ {
	kind := ctagsKindToLSPSymbolKind(r.Symbol.Kind)
	if kind == 0 {
		return "UNKNOWN"
	}
	return strings.ToUpper(kind.String())
}

func (r symbolResolver) Language() string { return r.Symbol.Language }

func (r symbolResolver) Location() *locationResolver {
	uri := r.BaseURI.WithFilePath(r.Symbol.Path)
	stat := CreateFileInfo(uri.Fragment, false)
	sr := symbolRange(r.Symbol)
	return &locationResolver{
		resource: NewGitTreeEntryResolver(r.commit, r.db, stat),
		lspRange: &sr,
	}
}

func (r symbolResolver) URL(ctx context.Context) (string, error) { return r.Location().URL(ctx) }

func (r symbolResolver) CanonicalURL() (string, error) { return r.Location().CanonicalURL() }

func (r symbolResolver) FileLocal() bool { return r.Symbol.FileLimited }
