package repos

import (
	"context"
	"fmt"
	"math"
	"regexp"
	regexpsyntax "regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/neelance/parallel"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/search/searchcontexts"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/search"
	searchbackend "github.com/sourcegraph/sourcegraph/internal/search/backend"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
	"github.com/sourcegraph/sourcegraph/schema"
)

type Resolved struct {
	RepoRevs        []*search.RepositoryRevisions
	MissingRepoRevs []*search.RepositoryRevisions
	ExcludedRepos   ExcludedRepos
	OverLimit       bool
}

type Resolver struct {
	DB               dbutil.DB
	Zoekt            *searchbackend.Zoekt
	DefaultReposFunc defaultReposFunc
	NamespaceStore   interface {
		GetByName(context.Context, string) (*database.Namespace, error)
	}
}

func (r *Resolver) Resolve(ctx context.Context, op Options) (Resolved, error) {
	var err error
	tr, ctx := trace.New(ctx, "resolveRepositories", op.String())
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	includePatterns := op.RepoFilters
	if includePatterns != nil {
		// Copy to avoid race condition.
		includePatterns = append([]string{}, includePatterns...)
	}

	excludePatterns := op.MinusRepoFilters

	maxRepoListSize := SearchLimits().MaxRepos

	// If any repo groups are specified, take the intersection of the repo
	// groups and the set of repos specified with repo:. (If none are specified
	// with repo:, then include all from the group.)
	if groupNames := op.RepoGroupFilters; len(groupNames) > 0 {
		groups, err := ResolveRepoGroups(ctx, op.UserSettings)
		if err != nil {
			return Resolved{}, err
		}
		patterns := repoGroupValuesToRegexp(groupNames, groups)
		tr.LazyPrintf("repogroups: adding %d repos to include pattern", len(patterns))
		includePatterns = append(includePatterns, UnionRegExps(patterns))

		// Ensure we don't omit any repos explicitly included via a repo group. (Each explicitly
		// listed repo generates at least one pattern.)
		if len(patterns) > maxRepoListSize {
			maxRepoListSize = len(patterns)
		}
	}

	// note that this mutates the strings in includePatterns, stripping their
	// revision specs, if they had any.
	includePatternRevs, err := findPatternRevs(includePatterns)
	if err != nil {
		return Resolved{}, err
	}

	// If a version context is specified, gather the list of repository names
	// to limit the results to these repositories.
	var versionContextRepositories []string
	var versionContext *schema.VersionContext
	// If a ref is specified we skip using version contexts.
	if len(includePatternRevs) == 0 && op.VersionContextName != "" {
		versionContext, err = resolveVersionContext(op.VersionContextName)
		if err != nil {
			return Resolved{}, err
		}

		for _, revision := range versionContext.Revisions {
			versionContextRepositories = append(versionContextRepositories, revision.Repo)
		}
	}

	searchContext, err := searchcontexts.ResolveSearchContextSpec(ctx, op.SearchContextSpec, r.NamespaceStore.GetByName)
	if err != nil {
		return Resolved{}, err
	}

	var defaultRepos []*types.RepoName

	if envvar.SourcegraphDotComMode() && len(includePatterns) == 0 && !query.HasTypeRepo(op.Query) && searchcontexts.IsGlobalSearchContext(searchContext) {
		start := time.Now()
		defaultRepos, err = defaultRepositories(ctx, r.DefaultReposFunc, r.Zoekt, excludePatterns)
		if err != nil {
			return Resolved{}, errors.Wrap(err, "getting list of default repos")
		}
		tr.LazyPrintf("defaultrepos: took %s to add %d repos", time.Since(start), len(defaultRepos))

		// Search all default repos since indexed search is fast.
		if len(defaultRepos) > maxRepoListSize {
			maxRepoListSize = len(defaultRepos)
		}
	}

	var repos []*types.RepoName
	var excluded ExcludedRepos
	if len(defaultRepos) > 0 {
		repos = defaultRepos
		if len(repos) > maxRepoListSize {
			repos = repos[:maxRepoListSize]
		}
	} else {
		tr.LazyPrintf("Repos.List - start")
		options := database.ReposListOptions{
			IncludePatterns: includePatterns,
			Names:           versionContextRepositories,
			ExcludePattern:  UnionRegExps(excludePatterns),
			// List N+1 repos so we can see if there are repos omitted due to our repo limit.
			LimitOffset:  &database.LimitOffset{Limit: maxRepoListSize + 1},
			NoForks:      op.NoForks,
			OnlyForks:    op.OnlyForks,
			NoArchived:   op.NoArchived,
			OnlyArchived: op.OnlyArchived,
			NoPrivate:    op.OnlyPublic,
			OnlyPrivate:  op.OnlyPrivate,
		}

		if searchContext != nil && searchContext.UserID != 0 {
			options.UserID = searchContext.UserID
			options.IncludeUserPublicRepos = true
		}

		// PERF: We Query concurrently since Count and List call can be slow
		// on Sourcegraph.com (100ms+).
		excludedC := make(chan ExcludedRepos)
		go func() {
			excludedC <- computeExcludedRepositories(ctx, r.DB, op.Query, options)
		}()

		repos, err = database.Repos(r.DB).ListRepoNames(ctx, options)
		tr.LazyPrintf("Repos.List - done")

		excluded = <-excludedC
		tr.LazyPrintf("excluded repos: %+v", excluded)

		if err != nil {
			return Resolved{}, err
		}
	}
	overLimit := len(repos) >= maxRepoListSize
	repoRevs := make([]*search.RepositoryRevisions, 0, len(repos))
	var missingRepoRevs []*search.RepositoryRevisions
	tr.LazyPrintf("Associate/validate revs - start")

	for _, repo := range repos {
		var repoRev search.RepositoryRevisions
		var revs []search.RevisionSpecifier
		// versionContext will be nil if the Query contains revision specifiers
		if versionContext != nil {
			for _, vcRepoRev := range versionContext.Revisions {
				if vcRepoRev.Repo == string(repo.Name) {
					repoRev.Repo = repo
					revs = append(revs, search.RevisionSpecifier{RevSpec: vcRepoRev.Rev})
				}
			}
		} else {
			var clashingRevs []search.RevisionSpecifier
			revs, clashingRevs = getRevsForMatchedRepo(repo.Name, includePatternRevs)
			repoRev.Repo = repo
			// if multiple specified revisions clash, report this usefully:
			if len(revs) == 0 && clashingRevs != nil {
				missingRepoRevs = append(missingRepoRevs, &search.RepositoryRevisions{
					Repo: repo,
					Revs: clashingRevs,
				})
			}
		}

		// We do in place filtering to reduce allocations. Common path is no
		// filtering of revs.
		if len(revs) > 0 {
			repoRev.Revs = revs[:0]
		}

		// Check if the repository actually has the revisions that the user specified.
		for _, rev := range revs {
			if rev.RefGlob != "" || rev.ExcludeRefGlob != "" {
				// Do not validate ref patterns. A ref pattern matching 0 refs is not necessarily
				// invalid, so it's not clear what validation would even mean.
				repoRev.Revs = append(repoRev.Revs, rev)
				continue
			}
			if rev.RevSpec == "" { // skip default branch resolution to save time
				repoRev.Revs = append(repoRev.Revs, rev)
				continue
			}

			// Validate the revspec.
			// Do not trigger a repo-updater lookup (e.g.,
			// backend.{GitRepo,Repos.ResolveRev}) because that would slow this operation
			// down by a lot (if we're looping over many repos). This means that it'll fail if a
			// repo is not on gitserver.
			//
			// TODO(sqs): make this NOT send gitserver this revspec in EnsureRevision, to avoid
			// searches like "repo:@foobar" (where foobar is an invalid revspec on most repos)
			// taking a long time because they all ask gitserver to try to fetch from the remote
			// repo.
			trimmedRefSpec := strings.TrimPrefix(rev.RevSpec, "^") // handle negated revisions, such as ^<branch>, ^<tag>, or ^<commit>
			if _, err := git.ResolveRevision(ctx, repoRev.GitserverRepo(), trimmedRefSpec, git.ResolveRevisionOptions{NoEnsureRevision: true}); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return Resolved{}, context.DeadlineExceeded
				}
				if errors.As(err, &git.BadCommitError{}) {
					return Resolved{}, err
				}
				if gitserver.IsRevisionNotFound(err) {
					// The revspec does not exist, so don't include it, and report that it's missing.
					if rev.RevSpec == "" {
						// Report as HEAD not "" (empty string) to avoid user confusion.
						rev.RevSpec = "HEAD"
					}
					missingRepoRevs = append(missingRepoRevs, &search.RepositoryRevisions{
						Repo: repo,
						Revs: []search.RevisionSpecifier{{RevSpec: rev.RevSpec}},
					})
				}
				// If err != nil and is not one of the err values checked for above, cloning and other errors will be handled later, so just ignore an error
				// if there is one.
				continue
			}
			repoRev.Revs = append(repoRev.Revs, rev)
		}
		repoRevs = append(repoRevs, &repoRev)
	}

	tr.LazyPrintf("Associate/validate revs - done")

	if op.CommitAfter != "" {
		start := time.Now()
		before := len(repoRevs)
		repoRevs, err = filterRepoHasCommitAfter(ctx, repoRevs, op.CommitAfter)
		tr.LazyPrintf("repohascommitafter removed %d repos in %s", before-len(repoRevs), time.Since(start))
	}

	return Resolved{
		RepoRevs:        repoRevs,
		MissingRepoRevs: missingRepoRevs,
		ExcludedRepos:   excluded,
		OverLimit:       overLimit,
	}, err
}

type Options struct {
	RepoFilters        []string
	MinusRepoFilters   []string
	RepoGroupFilters   []string
	SearchContextSpec  string
	VersionContextName string
	UserSettings       *schema.Settings
	NoForks            bool
	OnlyForks          bool
	NoArchived         bool
	OnlyArchived       bool
	CommitAfter        string
	OnlyPrivate        bool
	OnlyPublic         bool
	Query              query.Q
}

func (op *Options) String() string {
	var b strings.Builder
	if len(op.RepoFilters) == 0 {
		b.WriteString("r=[]")
	}
	for i, r := range op.RepoFilters {
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Quote(r))
	}

	if len(op.MinusRepoFilters) > 0 {
		_, _ = fmt.Fprintf(&b, " -r=%v", op.MinusRepoFilters)
	}
	if len(op.RepoGroupFilters) > 0 {
		_, _ = fmt.Fprintf(&b, " groups=%v", op.RepoGroupFilters)
	}
	if op.VersionContextName != "" {
		_, _ = fmt.Fprintf(&b, " versionContext=%q", op.VersionContextName)
	}
	if op.CommitAfter != "" {
		_, _ = fmt.Fprintf(&b, " CommitAfter=%q", op.CommitAfter)
	}

	if op.NoForks {
		b.WriteString(" NoForks")
	}
	if op.OnlyForks {
		b.WriteString(" OnlyForks")
	}
	if op.NoArchived {
		b.WriteString(" NoArchived")
	}
	if op.OnlyArchived {
		b.WriteString(" OnlyArchived")
	}
	if op.OnlyPrivate {
		b.WriteString(" OnlyPrivate")
	}
	if op.OnlyPublic {
		b.WriteString(" OnlyPublic")
	}

	return b.String()
}

func SearchLimits() schema.SearchLimits {
	// Our configuration reader does not set defaults from schema. So we rely
	// on Go default values to mean defaults.
	withDefault := func(x *int, def int) {
		if *x <= 0 {
			*x = def
		}
	}

	c := conf.Get()

	var limits schema.SearchLimits
	if c.SearchLimits != nil {
		limits = *c.SearchLimits
	}

	// If MaxRepos unset use deprecated value
	if limits.MaxRepos == 0 {
		limits.MaxRepos = c.MaxReposToSearch
	}

	withDefault(&limits.MaxRepos, math.MaxInt32>>1)
	withDefault(&limits.CommitDiffMaxRepos, 50)
	withDefault(&limits.CommitDiffWithTimeFilterMaxRepos, 10000)
	withDefault(&limits.MaxTimeoutSeconds, 60)

	return limits
}

// ExactlyOneRepo returns whether exactly one repo: literal field is specified and
// delineated by regex anchors ^ and $. This function helps determine whether we
// should return results for a single repo regardless of whether it is a fork or
// archive.
func ExactlyOneRepo(repoFilters []string) bool {
	if len(repoFilters) == 1 {
		filter, _ := search.ParseRepositoryRevisions(repoFilters[0])
		if strings.HasPrefix(filter, "^") && strings.HasSuffix(filter, "$") {
			filter := strings.TrimSuffix(strings.TrimPrefix(filter, "^"), "$")
			r, err := regexpsyntax.Parse(filter, regexpFlags)
			if err != nil {
				return false
			}
			return r.Op == regexpsyntax.OpLiteral
		}
	}
	return false
}

func UnionRegExps(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	if len(patterns) == 1 {
		return patterns[0]
	}

	// We only need to wrap the pattern in parentheses if it contains a "|" because
	// "|" has the lowest precedence of any operator.
	patterns2 := make([]string, len(patterns))
	for i, p := range patterns {
		if strings.Contains(p, "|") {
			p = "(" + p + ")"
		}
		patterns2[i] = p
	}
	return strings.Join(patterns2, "|")
}

// NOTE: This function is not called if the version context is not used
func resolveVersionContext(versionContext string) (*schema.VersionContext, error) {
	for _, vc := range conf.Get().ExperimentalFeatures.VersionContexts {
		if vc.Name == versionContext {
			return vc, nil
		}
	}

	return nil, errors.New("version context not found")
}

// Cf. golang/go/src/regexp/syntax/parse.go.
const regexpFlags = regexpsyntax.ClassNL | regexpsyntax.PerlX | regexpsyntax.UnicodeGroups

// A type that counts how many repos with a certain label were excluded from search results.
type ExcludedRepos struct {
	Forks    int
	Archived int
}

// computeExcludedRepositories returns a list of excluded repositories (Forks or
// archives) based on the search Query.
func computeExcludedRepositories(ctx context.Context, db dbutil.DB, q query.Q, op database.ReposListOptions) (excluded ExcludedRepos) {
	if q == nil {
		return ExcludedRepos{}
	}

	// PERF: We Query concurrently since each count call can be slow on
	// Sourcegraph.com (100ms+).
	var wg sync.WaitGroup
	var numExcludedForks, numExcludedArchived int

	if q.Fork() == nil && !ExactlyOneRepo(op.IncludePatterns) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 'fork:...' was not specified and Forks are excluded, find out
			// which repos are excluded.
			selectForks := op
			selectForks.OnlyForks = true
			selectForks.NoForks = false
			var err error
			numExcludedForks, err = database.Repos(db).Count(ctx, selectForks)
			if err != nil {
				log15.Warn("repo count for excluded fork", "err", err)
			}
		}()
	}

	if q.Archived() == nil && !ExactlyOneRepo(op.IncludePatterns) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Archived...: was not specified and archives are excluded,
			// find out which repos are excluded.
			selectArchived := op
			selectArchived.OnlyArchived = true
			selectArchived.NoArchived = false
			var err error
			numExcludedArchived, err = database.Repos(db).Count(ctx, selectArchived)
			if err != nil {
				log15.Warn("repo count for excluded archive", "err", err)
			}
		}()
	}

	wg.Wait()

	return ExcludedRepos{Forks: numExcludedForks, Archived: numExcludedArchived}
}

// a patternRevspec maps an include pattern to a list of revisions
// for repos matching that pattern. "map" in this case does not mean
// an actual map, because we want regexp matches, not identity matches.
type patternRevspec struct {
	includePattern *regexp.Regexp
	revs           []search.RevisionSpecifier
}

// given a repo name, determine whether it matched any patterns for which we have
// revspecs (or ref globs), and if so, return the matching/allowed ones.
func getRevsForMatchedRepo(repo api.RepoName, pats []patternRevspec) (matched []search.RevisionSpecifier, clashing []search.RevisionSpecifier) {
	revLists := make([][]search.RevisionSpecifier, 0, len(pats))
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
		matched = []search.RevisionSpecifier{{RevSpec: ""}}
		return
	}
	// if two repo specs match, and both provided non-empty rev lists,
	// we want their intersection
	allowedRevs := make(map[search.RevisionSpecifier]struct{}, len(revLists[0]))
	allRevs := make(map[search.RevisionSpecifier]struct{}, len(revLists[0]))
	// starting point: everything is "true" if it is currently allowed
	for _, rev := range revLists[0] {
		allowedRevs[rev] = struct{}{}
		allRevs[rev] = struct{}{}
	}
	// in theory, "master-by-default" entries won't even be participating
	// in this.
	for _, revList := range revLists[1:] {
		restrictedRevs := make(map[search.RevisionSpecifier]struct{}, len(revList))
		for _, rev := range revList {
			allRevs[rev] = struct{}{}
			if _, ok := allowedRevs[rev]; ok {
				restrictedRevs[rev] = struct{}{}
			}
		}
		allowedRevs = restrictedRevs
	}
	if len(allowedRevs) > 0 {
		matched = make([]search.RevisionSpecifier, 0, len(allowedRevs))
		for rev := range allowedRevs {
			matched = append(matched, rev)
		}
		sort.Slice(matched, func(i, j int) bool { return matched[i].Less(matched[j]) })
		return
	}
	// build a list of the revspecs which broke this, return it
	// as the "clashing" list.
	clashing = make([]search.RevisionSpecifier, 0, len(allRevs))
	for rev := range allRevs {
		clashing = append(clashing, rev)
	}
	// ensure that lists are always returned in sorted order.
	sort.Slice(clashing, func(i, j int) bool { return clashing[i].Less(clashing[j]) })
	return
}

// findPatternRevs mutates the given list of include patterns to
// be a raw list of the repository name patterns we want, separating
// out their revision specs, if any.
func findPatternRevs(includePatterns []string) (includePatternRevs []patternRevspec, err error) {
	includePatternRevs = make([]patternRevspec, 0, len(includePatterns))
	for i, includePattern := range includePatterns {
		repoPattern, revs := search.ParseRepositoryRevisions(includePattern)
		// Validate pattern now so the error message is more recognizable to the
		// user
		if _, err := regexp.Compile(repoPattern); err != nil {
			return nil, &badRequestError{err}
		}
		repoPattern = optimizeRepoPatternWithHeuristics(repoPattern)
		includePatterns[i] = repoPattern
		if len(revs) > 0 {
			p, err := regexp.Compile("(?i:" + includePatterns[i] + ")")
			if err != nil {
				return nil, &badRequestError{err}
			}
			patternRev := patternRevspec{includePattern: p, revs: revs}
			includePatternRevs = append(includePatternRevs, patternRev)
		}
	}
	return
}

type defaultReposFunc func(ctx context.Context) ([]*types.RepoName, error)

func defaultRepositories(ctx context.Context, getRawDefaultRepos defaultReposFunc, z *searchbackend.Zoekt, excludePatterns []string) ([]*types.RepoName, error) {
	// Get the list of default repos from the database.
	defaultRepos, err := getRawDefaultRepos(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "querying database for default repos")
	}

	// Remove excluded repos
	if len(excludePatterns) > 0 {
		patterns, _ := regexp.Compile(`(?i)` + UnionRegExps(excludePatterns))
		filteredRepos := defaultRepos[:0]
		for _, repo := range defaultRepos {
			if matched := patterns.MatchString(string(repo.Name)); !matched {
				filteredRepos = append(filteredRepos, repo)
			}
		}
		defaultRepos = filteredRepos
	}

	// Ask Zoekt which repos it has indexed
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	set, err := z.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// In place filtering of defaultRepos to only include names from set.
	repos := defaultRepos[:0]
	for _, r := range defaultRepos {
		if _, ok := set[string(r.Name)]; ok {
			repos = append(repos, r)
		}
	}

	return repos, nil
}

func filterRepoHasCommitAfter(ctx context.Context, revisions []*search.RepositoryRevisions, after string) ([]*search.RepositoryRevisions, error) {
	var (
		mut  sync.Mutex
		pass []*search.RepositoryRevisions
		res  = make(chan *search.RepositoryRevisions, 100)
		run  = parallel.NewRun(128)
	)

	goroutine.Go(func() {
		for rev := range res {
			if len(rev.Revs) != 0 {
				mut.Lock()
				pass = append(pass, rev)
				mut.Unlock()
			}
			run.Release()
		}
	})

	for _, revs := range revisions {
		run.Acquire()

		revs := revs
		goroutine.Go(func() {
			var specifiers []search.RevisionSpecifier
			for _, rev := range revs.Revs {
				ok, err := git.HasCommitAfter(ctx, revs.GitserverRepo(), after, rev.RevSpec)
				if err != nil {
					if gitserver.IsRevisionNotFound(err) || vcs.IsRepoNotExist(err) {
						continue
					}

					run.Error(err)
					continue
				}
				if ok {
					specifiers = append(specifiers, rev)
				}
			}
			res <- &search.RepositoryRevisions{Repo: revs.Repo, Revs: specifiers}
		})
	}

	err := run.Wait()
	close(res)

	return pass, err
}

func optimizeRepoPatternWithHeuristics(repoPattern string) string {
	if envvar.SourcegraphDotComMode() && (strings.HasPrefix(repoPattern, "github.com") || strings.HasPrefix(repoPattern, `github\.com`)) {
		repoPattern = "^" + repoPattern
	}
	// Optimization: make the "." in "github.com" a literal dot
	// so that the regexp can be optimized more effectively.
	repoPattern = strings.Replace(repoPattern, "github.com", `github\.com`, -1)
	return repoPattern
}

type badRequestError struct {
	err error
}

func (e *badRequestError) BadRequest() bool {
	return true
}

func (e *badRequestError) Error() string {
	return "bad request: " + e.err.Error()
}

func (e *badRequestError) Cause() error {
	return e.err
}
