package graphqlbackend

import (
	"context"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/neelance/parallel"
	"github.com/pkg/errors"
	"github.com/sourcegraph/go-lsp"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gituri"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
)

const maxSearchSuggestions = 100

// SearchSuggestionResolver is a resolver for the GraphQL union type `SearchSuggestion`
type SearchSuggestionResolver interface {
	// Score defines how well this item matches the query for sorting purposes
	Score() int

	// Length holds the length of the item name as a second sorting criterium
	Length() int

	// Label to sort alphabetically by when all else is equal.
	Label() string

	// Key is a key used to deduplicate suggestion results
	Key() suggestionKey

	ToRepository() (*RepositoryResolver, bool)
	ToFile() (*GitTreeEntryResolver, bool)
	ToGitBlob() (*GitTreeEntryResolver, bool)
	ToGitTree() (*GitTreeEntryResolver, bool)
	ToSymbol() (*symbolResolver, bool)
	ToLanguage() (*languageResolver, bool)
}

// baseSuggestionResolver implements all the To* methods, returning false for all of them.
// Its intent is to be embedded into other suggestion resolvers to simplify implementing
// searchSuggestionResolver.
type baseSuggestionResolver struct{}

func (baseSuggestionResolver) ToRepository() (*RepositoryResolver, bool) { return nil, false }
func (baseSuggestionResolver) ToFile() (*GitTreeEntryResolver, bool)     { return nil, false }
func (baseSuggestionResolver) ToGitBlob() (*GitTreeEntryResolver, bool)  { return nil, false }
func (baseSuggestionResolver) ToGitTree() (*GitTreeEntryResolver, bool)  { return nil, false }
func (baseSuggestionResolver) ToSymbol() (*symbolResolver, bool)         { return &symbolResolver{}, false }
func (baseSuggestionResolver) ToLanguage() (*languageResolver, bool)     { return nil, false }

// repositorySuggestionResolver implements searchSuggestionResolver for RepositoryResolver
type repositorySuggestionResolver struct {
	baseSuggestionResolver
	repo  *RepositoryResolver
	score int
}

func (r repositorySuggestionResolver) Score() int                                { return r.score }
func (r repositorySuggestionResolver) Length() int                               { return len(r.repo.Name()) }
func (r repositorySuggestionResolver) Label() string                             { return r.repo.Name() }
func (r repositorySuggestionResolver) ToRepository() (*RepositoryResolver, bool) { return r.repo, true }
func (r repositorySuggestionResolver) Key() suggestionKey {
	return suggestionKey{repoName: r.repo.Name()}
}

// gitTreeSuggestionResolver implements searchSuggestionResolver for GitTreeEntryResolver
type gitTreeSuggestionResolver struct {
	baseSuggestionResolver
	gitTreeEntry *GitTreeEntryResolver
	score        int
}

func (g gitTreeSuggestionResolver) Score() int    { return g.score }
func (g gitTreeSuggestionResolver) Length() int   { return len(g.gitTreeEntry.Path()) }
func (g gitTreeSuggestionResolver) Label() string { return g.gitTreeEntry.Path() }
func (g gitTreeSuggestionResolver) ToFile() (*GitTreeEntryResolver, bool) {
	return g.gitTreeEntry, true
}
func (g gitTreeSuggestionResolver) ToGitBlob() (*GitTreeEntryResolver, bool) {
	return g.gitTreeEntry, g.gitTreeEntry.stat.Mode().IsRegular()
}
func (g gitTreeSuggestionResolver) ToGitTree() (*GitTreeEntryResolver, bool) {
	return g.gitTreeEntry, g.gitTreeEntry.stat.Mode().IsDir()
}
func (g gitTreeSuggestionResolver) Key() suggestionKey {
	return suggestionKey{
		repoName: g.gitTreeEntry.Commit().Repository().Name(),
		repoRev:  string(g.gitTreeEntry.Commit().OID()),
		file:     g.gitTreeEntry.Path(),
	}
}

// symbolSuggestionResolver implements searchSuggestionResolver for symbolResolver
type symbolSuggestionResolver struct {
	baseSuggestionResolver
	symbol symbolResolver
	score  int
}

func (s symbolSuggestionResolver) Score() int { return s.score }
func (s symbolSuggestionResolver) Length() int {
	return len(s.symbol.Symbol.Name) + len(s.symbol.Symbol.Parent)
}
func (s symbolSuggestionResolver) Label() string {
	return s.symbol.Symbol.Name + " " + s.symbol.Symbol.Parent
}
func (s symbolSuggestionResolver) ToSymbol() (*symbolResolver, bool) { return &s.symbol, true }
func (s symbolSuggestionResolver) Key() suggestionKey {
	return suggestionKey{
		uri:    s.symbol.URI(),
		symbol: s.symbol.Symbol.Name + s.symbol.Symbol.Parent,
	}
}

// languageSuggestionResolver implements searchSuggestionResolver for languageResolver
type languageSuggestionResolver struct {
	baseSuggestionResolver
	lang  *languageResolver
	score int
}

func (l languageSuggestionResolver) Score() int                            { return l.score }
func (l languageSuggestionResolver) Length() int                           { return len(l.lang.Name()) }
func (l languageSuggestionResolver) Label() string                         { return l.lang.Name() }
func (l languageSuggestionResolver) ToLanguage() (*languageResolver, bool) { return l.lang, true }
func (l languageSuggestionResolver) Key() suggestionKey {
	return suggestionKey{
		lang: l.lang.Name(),
	}
}

func sortSearchSuggestions(s []SearchSuggestionResolver) {
	sort.Slice(s, func(i, j int) bool {
		// Sort by score
		a, b := s[i], s[j]
		if a.Score() != b.Score() {
			return a.Score() > b.Score()
		}
		// Prefer shorter strings for the same match score
		// E.g. prefer gorilla/mux over gorilla/muxy, Microsoft/vscode over g3ortega/vscode-crystal
		if a.Length() != b.Length() {
			return a.Length() < b.Length()
		}

		// All else equal, sort alphabetically.
		return a.Label() < b.Label()
	})
}

type suggestionKey struct {
	repoName string
	repoRev  string
	file     string
	symbol   string
	lang     string
	uri      *gituri.URI
}

type searchSuggestionsArgs struct {
	First *int32
}

func (a *searchSuggestionsArgs) applyDefaultsAndConstraints() {
	if a.First == nil || *a.First < 0 || *a.First > maxSearchSuggestions {
		n := int32(maxSearchSuggestions)
		a.First = &n
	}
}

type showSearchSuggestionResolvers func() ([]SearchSuggestionResolver, error)

var (
	mockShowRepoSuggestions showSearchSuggestionResolvers
	mockShowFileSuggestions showSearchSuggestionResolvers
	mockShowLangSuggestions showSearchSuggestionResolvers
	mockShowSymbolMatches   showSearchSuggestionResolvers
)

func (r *searchResolver) Suggestions(ctx context.Context, args *searchSuggestionsArgs) ([]SearchSuggestionResolver, error) {

	// If globbing is activated, convert regex patterns of repo, file, and repohasfile
	// from "field:^foo$" to "field:^foo".
	globbing := false
	if getBoolPtr(r.UserSettings.SearchGlobbing, false) {
		globbing = true
	}
	if globbing {
		r.Query = query.FuzzifyRegexPatterns(r.Query)
	}

	args.applyDefaultsAndConstraints()

	if len(r.Query) == 0 {
		return nil, nil
	}

	// Only suggest for type:file.
	typeValues, _ := r.Query.StringValues(query.FieldType)
	for _, resultType := range typeValues {
		if resultType != "file" {
			return nil, nil
		}
	}

	var suggesters []func(ctx context.Context) ([]SearchSuggestionResolver, error)

	showRepoSuggestions := func(ctx context.Context) ([]SearchSuggestionResolver, error) {
		if mockShowRepoSuggestions != nil {
			return mockShowRepoSuggestions()
		}

		// * If query contains only a single term (or 1 repogroup: token and a single term), treat it as a repo field here and ignore the other repo queries.
		// * If only repo fields (except 1 term in query), show repo suggestions.

		var effectiveRepoFieldValues []string
		if len(r.Query.Values(query.FieldDefault)) == 1 && (len(r.Query.Fields()) == 1 || (len(r.Query.Fields()) == 2 && len(r.Query.Values(query.FieldRepoGroup)) == 1)) {
			effectiveRepoFieldValues = append(effectiveRepoFieldValues, r.Query.Values(query.FieldDefault)[0].ToString())
		} else if len(r.Query.Values(query.FieldRepo)) > 0 && ((len(r.Query.Values(query.FieldRepoGroup)) > 0 && len(r.Query.Fields()) == 2) || (len(r.Query.Values(query.FieldRepoGroup)) == 0 && len(r.Query.Fields()) == 1)) {
			effectiveRepoFieldValues, _ = r.Query.Repositories()
		}

		// If we have a query which is not valid, just ignore it since this is for a suggestion.
		i := 0
		for _, v := range effectiveRepoFieldValues {
			if _, err := regexp.Compile(v); err == nil {
				effectiveRepoFieldValues[i] = v
				i++
			}
		}
		effectiveRepoFieldValues = effectiveRepoFieldValues[:i]

		if len(effectiveRepoFieldValues) > 0 {
			resolved, err := r.resolveRepositories(ctx, effectiveRepoFieldValues)

			resolvers := make([]SearchSuggestionResolver, 0, len(resolved.RepoRevs))
			for _, rev := range resolved.RepoRevs {
				resolvers = append(resolvers, repositorySuggestionResolver{
					repo:  NewRepositoryResolver(r.db, rev.Repo.ToRepo()),
					score: math.MaxInt32,
				})
			}

			return resolvers, err
		}
		return nil, nil
	}
	suggesters = append(suggesters, showRepoSuggestions)

	showFileSuggestions := func(ctx context.Context) ([]SearchSuggestionResolver, error) {
		if mockShowFileSuggestions != nil {
			return mockShowFileSuggestions()
		}

		// If only repos/repogroups and files are specified (and at most 1 term), then show file
		// suggestions.  If the query has a single term, then consider it to be a `file:` filter (to
		// make it easy to jump to files by just typing in their name, not `file:<their name>`).
		hasOnlyEmptyRepoField := len(r.Query.Values(query.FieldRepo)) > 0 && allEmptyStrings(r.Query.RegexpPatterns(query.FieldRepo)) && len(r.Query.Fields()) == 1
		hasRepoOrFileFields := len(r.Query.Values(query.FieldRepoGroup)) > 0 || len(r.Query.Values(query.FieldRepo)) > 0 || len(r.Query.Values(query.FieldFile)) > 0
		if !hasOnlyEmptyRepoField && hasRepoOrFileFields && len(r.Query.Values(query.FieldDefault)) <= 1 {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			return r.suggestFilePaths(ctx, maxSearchSuggestions)
		}
		return nil, nil
	}
	suggesters = append(suggesters, showFileSuggestions)

	showLangSuggestions := func(ctx context.Context) ([]SearchSuggestionResolver, error) {
		if mockShowLangSuggestions != nil {
			return mockShowLangSuggestions()
		}

		// The "repo:" field must be specified for showing language suggestions.
		// For performance reasons, only try to get languages of the first repository found
		// within the scope of the "repo:" field value.
		if len(r.Query.Values(query.FieldRepo)) == 0 {
			return nil, nil
		}
		effectiveRepoFieldValues, _ := r.Query.Repositories()

		validValues := effectiveRepoFieldValues[:0]
		for _, v := range effectiveRepoFieldValues {
			if i := strings.LastIndexByte(v, '@'); i > -1 {
				// Strip off the @revision suffix so that we can use
				// the trigram index on the name column in Postgres.
				v = v[:i]
			}

			if _, err := regexp.Compile(v); err == nil {
				validValues = append(validValues, v)
			}
		}
		if len(validValues) == 0 {
			return nil, nil
		}

		// Only care about the first found repository.
		repos, err := backend.Repos.List(ctx, database.ReposListOptions{
			IncludePatterns: validValues,
			LimitOffset: &database.LimitOffset{
				Limit: 1,
			},
		})
		if err != nil || len(repos) == 0 {
			return nil, err
		}
		repo := repos[0]

		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		commitID, err := backend.Repos.ResolveRev(ctx, repo, "")
		if err != nil {
			return nil, err
		}

		inventory, err := backend.Repos.GetInventory(ctx, repo, commitID, false)
		if err != nil {
			return nil, err
		}

		resolvers := make([]SearchSuggestionResolver, 0, len(inventory.Languages))
		for _, l := range inventory.Languages {
			resolvers = append(resolvers, languageSuggestionResolver{
				lang:  &languageResolver{db: r.db, name: strings.ToLower(l.Name)},
				score: math.MaxInt32,
			})
		}

		return resolvers, err
	}
	suggesters = append(suggesters, showLangSuggestions)

	showSymbolMatches := func(ctx context.Context) (results []SearchSuggestionResolver, err error) {
		if mockShowSymbolMatches != nil {
			return mockShowSymbolMatches()
		}

		resolved, err := r.resolveRepositories(ctx, nil)
		if err != nil {
			return nil, err
		}

		p, err := r.getPatternInfo(nil)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		fileMatches, _, err := collectStream(func(stream Sender) error {
			return searchSymbols(ctx, r.db, &search.TextParameters{
				PatternInfo:  p,
				RepoPromise:  (&search.Promise{}).Resolve(resolved.RepoRevs),
				Query:        r.Query,
				Zoekt:        r.zoekt,
				SearcherURLs: r.searcherURLs,
			}, 7, stream)
		})
		if err != nil {
			return nil, err
		}

		results = make([]SearchSuggestionResolver, 0)
		for _, match := range fileMatches {
			fileMatch, ok := match.ToFileMatch()
			if !ok {
				continue
			}
			for _, sr := range fileMatch.Symbols() {
				score := 20
				if sr.Symbol.Parent == "" {
					score++
				}
				if len(sr.Symbol.Name) < 12 {
					score++
				}
				switch ctagsKindToLSPSymbolKind(sr.Symbol.Kind) {
				case lsp.SKFunction, lsp.SKMethod:
					score += 2
				case lsp.SKClass:
					score += 3
				}
				if len(sr.Symbol.Name) >= 4 && strings.Contains(strings.ToLower(sr.URI().String()), strings.ToLower(sr.Symbol.Name)) {
					score++
				}
				results = append(results, symbolSuggestionResolver{
					symbol: sr,
					score:  score,
				})
			}
		}

		sortSearchSuggestions(results)
		const maxBoostedSymbolResults = 3
		boost := maxBoostedSymbolResults
		if len(results) < boost {
			boost = len(results)
		}
		if boost > 0 {
			for i := 0; i < boost; i++ {
				if res, ok := results[i].(symbolSuggestionResolver); ok {
					res.score += 200
					results[i] = res
				}
			}
		}

		return results, nil
	}
	suggesters = append(suggesters, showSymbolMatches)

	showFilesWithTextMatches := func(ctx context.Context) ([]SearchSuggestionResolver, error) {
		// If terms are specified, then show files that have text matches. Set an aggressive timeout
		// to avoid delaying repo and file suggestions for too long.
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		if len(r.Query.Values(query.FieldDefault)) > 0 {
			results, err := r.doResults(ctx, "file") // only "file" result type
			if err == context.DeadlineExceeded {
				err = nil // don't log as error below
			}
			var suggestions []SearchSuggestionResolver
			if results != nil {
				if len(results.SearchResults) > int(*args.First) {
					results.SearchResults = results.SearchResults[:*args.First]
				}
				suggestions = make([]SearchSuggestionResolver, 0, len(results.SearchResults))
				for i, res := range results.SearchResults {
					if fm, ok := res.ToFileMatch(); ok {
						entryResolver := fm.File()
						suggestions = append(suggestions, gitTreeSuggestionResolver{
							gitTreeEntry: entryResolver,
							score:        len(results.SearchResults) - i,
						})
					}
				}
			}
			return suggestions, err
		}
		return nil, nil
	}
	suggesters = append(suggesters, showFilesWithTextMatches)

	// Run suggesters.
	var (
		allSuggestions []SearchSuggestionResolver
		mu             sync.Mutex
		par            = parallel.NewRun(len(suggesters))
	)
	for _, suggester := range suggesters {
		par.Acquire()
		go func(suggester func(ctx context.Context) ([]SearchSuggestionResolver, error)) {
			defer par.Release()
			ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			suggestions, err := suggester(ctx)
			if err == nil {
				mu.Lock()
				allSuggestions = append(allSuggestions, suggestions...)
				mu.Unlock()
			} else {
				if errors.Cause(err) == context.DeadlineExceeded || errors.Cause(err) == context.Canceled {
					log15.Warn("search suggestions exceeded deadline (skipping)", "query", r.rawQuery())
				} else if !errcode.IsBadRequest(err) {
					// We exclude bad user input. Note that this means that we
					// may have some tokens in the input that are valid, but
					// typing something "bad" results in no suggestions from the
					// this suggester. In future we should just ignore the bad
					// token.
					par.Error(err)
				}
			}
		}(suggester)
	}
	if err := par.Wait(); err != nil {
		if len(allSuggestions) == 0 {
			return nil, err
		}
		// If we got partial results, only log the error and return partial results
		log15.Error("error getting search suggestions: ", "error", err)
	}

	// Eliminate duplicates.
	seen := make(map[suggestionKey]struct{}, len(allSuggestions))
	uniqueSuggestions := allSuggestions[:0]
	for _, s := range allSuggestions {
		k := s.Key()
		if _, dup := seen[k]; !dup {
			uniqueSuggestions = append(uniqueSuggestions, s)
			seen[k] = struct{}{}
		}
	}
	allSuggestions = uniqueSuggestions

	sortSearchSuggestions(allSuggestions)
	if len(allSuggestions) > int(*args.First) {
		allSuggestions = allSuggestions[:*args.First]
	}

	return allSuggestions, nil
}

func allEmptyStrings(ss1, ss2 []string) bool {
	for _, s := range ss1 {
		if s != "" {
			return false
		}
	}
	for _, s := range ss2 {
		if s != "" {
			return false
		}
	}
	return true
}

type languageResolver struct {
	db   dbutil.DB
	name string
}

func (r *languageResolver) Name() string { return r.name }
