package service

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/gobwas/glob"
	"github.com/grafana/regexp"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/api/internalapi"
	"github.com/sourcegraph/sourcegraph/internal/batches/store"
	btypes "github.com/sourcegraph/sourcegraph/internal/batches/types"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	streamapi "github.com/sourcegraph/sourcegraph/internal/search/streaming/api"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	batcheslib "github.com/sourcegraph/sourcegraph/lib/batches"
	onlib "github.com/sourcegraph/sourcegraph/lib/batches/on"
	"github.com/sourcegraph/sourcegraph/lib/batches/template"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const searchAPIVersion = "V3"

// RepoRevision describes a repository on a branch at a fixed revision.
type RepoRevision struct {
	Repo        *types.Repo
	Branch      string
	Commit      api.CommitID
	FileMatches []string
}

func (r *RepoRevision) HasBranch() bool {
	return r.Branch != ""
}

type RepoWorkspace struct {
	*RepoRevision
	Path string

	OnlyFetchWorkspace bool

	Ignored     bool
	Unsupported bool
}

type WorkspaceResolver interface {
	ResolveWorkspacesForBatchSpec(
		ctx context.Context,
		batchSpec *batcheslib.BatchSpec,
	) (
		workspaces []*RepoWorkspace,
		err error,
	)
}

type WorkspaceResolverBuilder func(tx *store.Store) WorkspaceResolver

func NewWorkspaceResolver(s *store.Store) WorkspaceResolver {
	return &workspaceResolver{
		store:               s,
		logger:              log.Scoped("batches.workspaceResolver"),
		gitserverClient:     gitserver.NewClient("batches.workspaceresolver"),
		frontendInternalURL: internalapi.Client.URL + "/.internal",
	}
}

type workspaceResolver struct {
	logger              log.Logger
	store               *store.Store
	gitserverClient     gitserver.Client
	frontendInternalURL string
}

func (wr *workspaceResolver) ResolveWorkspacesForBatchSpec(ctx context.Context, batchSpec *batcheslib.BatchSpec) (workspaces []*RepoWorkspace, err error) {
	tr, ctx := trace.New(ctx, "workspaceResolver.ResolveWorkspacesForBatchSpec")
	defer tr.EndWithErr(&err)

	// First, find all repositories that match the batch spec `on` definitions.
	// This list is filtered by permissions using database.Repos.List.
	repos, err := wr.determineRepositories(ctx, batchSpec)
	if err != nil {
		return nil, err
	}

	// Next, find the repos that are ignored through a .batchignore file.
	ignored, err := findIgnoredRepositories(ctx, wr.gitserverClient, repos)
	if err != nil {
		return nil, err
	}

	// Now build the workspaces for the list of repos.
	workspaces, err = findWorkspaces(ctx, batchSpec, wr, repos)
	if err != nil {
		return nil, err
	}

	// Finally, tag the workspaces if they're (a) on an unsupported code host
	// or (b) ignored.
	for _, ws := range workspaces {
		if !btypes.IsKindSupported(extsvc.TypeToKind(ws.Repo.ExternalRepo.ServiceType)) {
			ws.Unsupported = true
		}

		if _, ok := ignored[ws.Repo]; ok {
			ws.Ignored = true
		}
	}

	// Sort the workspaces so that the list of workspaces is kinda stable when
	// using `replaceBatchSpecInput`.
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].Repo.Name != workspaces[j].Repo.Name {
			return workspaces[i].Repo.Name < workspaces[j].Repo.Name
		}
		if workspaces[i].Path != workspaces[j].Path {
			return workspaces[i].Path < workspaces[j].Path
		}
		return workspaces[i].Branch < workspaces[j].Branch
	})

	return workspaces, nil
}

func (wr *workspaceResolver) determineRepositories(ctx context.Context, batchSpec *batcheslib.BatchSpec) ([]*RepoRevision, error) {
	agg := onlib.NewRepoRevisionAggregator()

	var errs error
	// TODO: this could be trivially parallelised in the future.
	for _, on := range batchSpec.On {
		revs, ruleType, err := wr.resolveRepositoriesOn(ctx, &on)
		if err != nil {
			errs = errors.Append(errs, errors.Wrapf(err, "resolving %q", on.String()))
			continue
		}

		result := agg.NewRuleRevisions(ruleType)
		for _, rev := range revs {
			// Skip repos where no branch exists.
			if !rev.HasBranch() {
				continue
			}

			result.AddRepoRevision(rev.Repo.ID, rev)
		}
	}

	repoRevs := []*RepoRevision{}
	for _, rev := range agg.Revisions() {
		repoRevs = append(repoRevs, rev.(*RepoRevision))
	}
	return repoRevs, errs
}

// ignoredWorkspaceResolverConcurrency defines the maximum concurrency level at that
// findIgnoredRepositories will hit gitserver for file info.
const ignoredWorkspaceResolverConcurrency = 5

func findIgnoredRepositories(ctx context.Context, gitserverClient gitserver.Client, repos []*RepoRevision) (map[*types.Repo]struct{}, error) {
	type result struct {
		repo           *RepoRevision
		hasBatchIgnore bool
		err            error
	}

	var (
		ignored = make(map[*types.Repo]struct{})

		input   = make(chan *RepoRevision, len(repos))
		results = make(chan result, len(repos))

		wg sync.WaitGroup
	)

	// Spawn N workers.
	for i := 0; i < ignoredWorkspaceResolverConcurrency; i++ {
		wg.Add(1)
		go func(in chan *RepoRevision, out chan result) {
			defer wg.Done()
			for repo := range in {
				hasBatchIgnore, err := hasBatchIgnoreFile(ctx, gitserverClient, repo)
				out <- result{repo, hasBatchIgnore, err}
			}
		}(input, results)
	}

	// Queue all the repos for processing.
	for _, repo := range repos {
		input <- repo
	}
	close(input)

	go func(wg *sync.WaitGroup) {
		wg.Wait()
		close(results)
	}(&wg)

	var errs error
	for result := range results {
		if result.err != nil {
			errs = errors.Append(errs, result.err)
			continue
		}

		if result.hasBatchIgnore {
			ignored[result.repo.Repo] = struct{}{}
		}
	}

	return ignored, errs
}

var ErrMalformedOnQueryOrRepository = batcheslib.NewValidationError(errors.New("malformed 'on' field; missing either a repository name or a query"))

// resolveRepositoriesOn resolves a single on: entry in a batch spec.
func (wr *workspaceResolver) resolveRepositoriesOn(ctx context.Context, on *batcheslib.OnQueryOrRepository) (_ []*RepoRevision, _ onlib.RepositoryRuleType, err error) {
	tr, ctx := trace.New(ctx, "workspaceResolver.resolveRepositoriesOn")
	defer tr.EndWithErr(&err)

	if on.RepositoriesMatchingQuery != "" {
		revs, err := wr.resolveRepositoriesMatchingQuery(ctx, on.RepositoriesMatchingQuery)
		return revs, onlib.RepositoryRuleTypeQuery, err
	}

	branches, err := on.GetBranches()
	if err != nil {
		return nil, onlib.RepositoryRuleTypeExplicit, err
	}

	if on.Repository != "" && len(branches) > 0 {
		revs := make([]*RepoRevision, len(branches))
		for i, branch := range branches {
			repo, err := wr.resolveRepositoryNameAndBranch(ctx, on.Repository, branch)
			if err != nil {
				return nil, onlib.RepositoryRuleTypeExplicit, err
			}

			revs[i] = repo
		}
		return revs, onlib.RepositoryRuleTypeExplicit, nil
	}

	if on.Repository != "" {
		repo, err := wr.resolveRepositoryName(ctx, on.Repository)
		if err != nil {
			return nil, onlib.RepositoryRuleTypeExplicit, err
		}
		return []*RepoRevision{repo}, onlib.RepositoryRuleTypeExplicit, nil
	}

	// This shouldn't happen on any batch spec that has passed validation, but,
	// alas, software.
	return nil, onlib.RepositoryRuleTypeExplicit, ErrMalformedOnQueryOrRepository
}

func (wr *workspaceResolver) resolveRepositoryName(ctx context.Context, name string) (_ *RepoRevision, err error) {
	tr, ctx := trace.New(ctx, "workspaceResolver.resolveRepositoryName")
	defer tr.EndWithErr(&err)

	repo, err := wr.store.Repos().GetByName(ctx, api.RepoName(name))
	if err != nil {
		return nil, err
	}

	return repoToRepoRevisionWithDefaultBranch(
		ctx,
		wr.gitserverClient,
		repo,
		// Directly resolved repos don't have any file matches.
		[]string{},
	)
}

func (wr *workspaceResolver) resolveRepositoryNameAndBranch(ctx context.Context, name, branch string) (_ *RepoRevision, err error) {
	tr, ctx := trace.New(ctx, "workspaceResolver.resolveRepositoryNameAndBranch")
	defer tr.EndWithErr(&err)

	repo, err := wr.store.Repos().GetByName(ctx, api.RepoName(name))
	if err != nil {
		return nil, err
	}

	commit, err := wr.gitserverClient.ResolveRevision(ctx, repo.Name, branch, gitserver.ResolveRevisionOptions{
		NoEnsureRevision: true,
	})
	if err != nil && errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
		return nil, errors.Newf("no branch matching %q found for repository %s", branch, name)
	}

	return &RepoRevision{
		Repo:   repo,
		Branch: branch,
		Commit: commit,
		// Directly resolved repos don't have any file matches.
		FileMatches: []string{},
	}, nil
}

func (wr *workspaceResolver) resolveRepositoriesMatchingQuery(ctx context.Context, query string) (_ []*RepoRevision, err error) {
	tr, ctx := trace.New(ctx, "workspaceResolver.resolveRepositorySearch")
	defer tr.EndWithErr(&err)

	query = setDefaultQueryCount(query)

	repoIDs := []api.RepoID{}
	repoFileMatches := make(map[api.RepoID]map[string]bool)
	addRepoFilePatch := func(repoID api.RepoID, path string) {
		repoMap, ok := repoFileMatches[repoID]
		if !ok {
			repoMap = make(map[string]bool)
			repoFileMatches[repoID] = repoMap
		}
		if _, ok := repoMap[path]; !ok {
			repoMap[path] = true
		}
	}
	if err := wr.runSearch(ctx, query, func(matches []streamhttp.EventMatch) {
		for _, match := range matches {
			switch m := match.(type) {
			case *streamhttp.EventRepoMatch:
				repoIDs = append(repoIDs, api.RepoID(m.RepositoryID))
			case *streamhttp.EventContentMatch:
				repoIDs = append(repoIDs, api.RepoID(m.RepositoryID))
				addRepoFilePatch(api.RepoID(m.RepositoryID), m.Path)
			case *streamhttp.EventPathMatch:
				repoIDs = append(repoIDs, api.RepoID(m.RepositoryID))
				addRepoFilePatch(api.RepoID(m.RepositoryID), m.Path)
			case *streamhttp.EventSymbolMatch:
				repoIDs = append(repoIDs, api.RepoID(m.RepositoryID))
				addRepoFilePatch(api.RepoID(m.RepositoryID), m.Path)
			}
		}
	}); err != nil {
		return nil, err
	}

	// If no repos matched the search query, we can early return.
	if len(repoIDs) == 0 {
		return []*RepoRevision{}, nil
	}

	// 🚨 SECURITY: We use database.Repos.List to check whether the user has access to
	// the repositories or not. We also impersonate on the internal search request to
	// properly respect these permissions.
	accessibleRepos, err := wr.store.Repos().List(ctx, database.ReposListOptions{IDs: repoIDs})
	if err != nil {
		return nil, err
	}

	revs := make([]*RepoRevision, 0, len(accessibleRepos))
	for _, repo := range accessibleRepos {
		fileMatches := make([]string, 0, len(repoFileMatches[repo.ID]))
		for path := range repoFileMatches[repo.ID] {
			fileMatches = append(fileMatches, path)
		}
		// Sort file matches so cache results always match.
		sort.Strings(fileMatches)
		rev, err := repoToRepoRevisionWithDefaultBranch(ctx, wr.gitserverClient, repo, fileMatches)
		if err != nil {
			// There is an edge-case where a repo might be returned by a search query that does not exist in gitserver yet.
			if errcode.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		revs = append(revs, rev)
	}

	return revs, nil
}

const internalSearchClientUserAgent = "Batch Changes repository resolver"

func (wr *workspaceResolver) runSearch(ctx context.Context, query string, onMatches func(matches []streamhttp.EventMatch)) (err error) {
	req, err := streamhttp.NewRequestWithVersion(wr.frontendInternalURL, query, searchAPIVersion)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)

	req.Header.Set("User-Agent", internalSearchClientUserAgent)

	// We impersonate as the user who initiated this search. This is to properly
	// scope repository permissions while running the search.
	a := actor.FromContext(ctx)
	if !a.IsAuthenticated() {
		return errors.New("no user set in workspaceResolver.runSearch")
	}

	cli, err := httpcli.NewInternalClientFactory("batches_resolver").Doer()
	if err != nil {
		return errors.Wrap(err, "failed to create internal client")
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := streamhttp.FrontendStreamDecoder{
		OnMatches: onMatches,
		OnError: func(ee *streamhttp.EventError) {
			err = errors.New(ee.Message)
		},
		OnProgress: func(p *streamapi.Progress) {
			// TODO: Evaluate skipped for values we care about.
		},
	}
	decErr := dec.ReadAll(resp.Body)
	if decErr != nil {
		return decErr
	}
	return err
}

func repoToRepoRevisionWithDefaultBranch(ctx context.Context, gitserverClient gitserver.Client, repo *types.Repo, fileMatches []string) (_ *RepoRevision, err error) {
	tr, ctx := trace.New(ctx, "repoToRepoRevision")
	defer tr.EndWithErr(&err)

	branch, commit, err := gitserverClient.GetDefaultBranch(ctx, repo.Name, false)
	if err != nil {
		return nil, err
	}

	repoRev := &RepoRevision{
		Repo:        repo,
		Branch:      branch,
		Commit:      commit,
		FileMatches: fileMatches,
	}
	return repoRev, nil
}

const batchIgnoreFilePath = ".batchignore"

func hasBatchIgnoreFile(ctx context.Context, gitserverClient gitserver.Client, r *RepoRevision) (_ bool, err error) {
	tr, ctx := trace.New(ctx, "hasBatchIgnoreFile", attribute.Int("repoID", int(r.Repo.ID)))
	defer tr.EndWithErr(&err)

	stat, err := gitserverClient.Stat(ctx, r.Repo.Name, r.Commit, batchIgnoreFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !stat.Mode().IsRegular() {
		return false, errors.Errorf("not a blob: %q", batchIgnoreFilePath)
	}
	return true, nil
}

var defaultQueryCountRegex = regexp.MustCompile(`\bcount:(\d+|all)\b`)

const hardCodedCount = " count:all"

func setDefaultQueryCount(query string) string {
	if defaultQueryCountRegex.MatchString(query) {
		return query
	}

	return query + hardCodedCount
}

// findDirectoriesInReposConcurrency defines the maximum concurrency level at that
// FindDirectoriesInRepos will run searches for file paths.
const findDirectoriesInReposConcurrency = 10

// FindDirectoriesInRepos returns a map of repositories and the locations of
// files matching the given file name in the repository.
// The locations are paths relative to the root of the directory.
// No "/" at the beginning.
// A dot (".") represents the root directory.
func (wr *workspaceResolver) FindDirectoriesInRepos(ctx context.Context, fileName string, repos ...*RepoRevision) (map[repoRevKey][]string, error) {
	findForRepoRev := func(repoRev *RepoRevision) ([]string, error) {
		query := fmt.Sprintf(`file:(^|/)%s$ repo:^%s$@%s type:path count:all`, regexp.QuoteMeta(fileName), regexp.QuoteMeta(string(repoRev.Repo.Name)), repoRev.Commit)

		results := []string{}
		err := wr.runSearch(ctx, query, func(matches []streamhttp.EventMatch) {
			for _, match := range matches {
				switch m := match.(type) {
				case *streamhttp.EventPathMatch:
					dir := path.Dir(m.Path)

					// "." means the path is root, but in the executor we use "" to signify root.
					if dir == "." {
						dir = ""
					}

					results = append(results, dir)
				}
			}
		})
		if err != nil {
			return nil, err
		}

		return results, nil
	}

	// Limit concurrency.
	sem := make(chan struct{}, findDirectoriesInReposConcurrency)
	for i := 0; i < findDirectoriesInReposConcurrency; i++ {
		sem <- struct{}{}
	}

	var (
		// mu protects both the errs variable and the results map from concurrent writes.
		errs    error
		mu      sync.Mutex
		results = make(map[repoRevKey][]string)
	)
	for _, repoRev := range repos {
		<-sem
		go func(repoRev *RepoRevision) {
			defer func() {
				sem <- struct{}{}
			}()

			result, err := findForRepoRev(repoRev)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = errors.Append(errs, err)
				return
			}
			results[repoRev.Key()] = result
		}(repoRev)
	}

	// Wait for all to finish.
	for i := 0; i < findDirectoriesInReposConcurrency; i++ {
		<-sem
	}

	return results, errs
}

type directoryFinder interface {
	FindDirectoriesInRepos(ctx context.Context, fileName string, repos ...*RepoRevision) (map[repoRevKey][]string, error)
}

// findWorkspaces matches the given repos to the workspace configs and
// searches, via the Sourcegraph instance, the locations of the workspaces in
// each repository.
// The repositories that were matched by a workspace config and all repos that didn't
// match a config are returned as workspaces.
func findWorkspaces(
	ctx context.Context,
	spec *batcheslib.BatchSpec,
	finder directoryFinder,
	repoRevs []*RepoRevision,
) ([]*RepoWorkspace, error) {
	// Pre-compile all globs.
	workspaceMatchers := make(map[batcheslib.WorkspaceConfiguration]glob.Glob)
	var errs error
	for _, conf := range spec.Workspaces {
		in := conf.In
		// Empty `in` should fall back to matching all, instead of nothing.
		if in == "" {
			in = "*"
		}
		g, err := glob.Compile(in)
		if err != nil {
			errs = errors.Append(errs, batcheslib.NewValidationError(errors.Errorf("failed to compile glob %q: %v", in, err)))
		}
		workspaceMatchers[conf] = g
	}
	if errs != nil {
		return nil, errs
	}

	root := []*RepoRevision{}

	// Maps workspace config indexes to repositories matching them.
	matched := map[int][]*RepoRevision{}
	for _, repoRev := range repoRevs {
		found := false

		// Try to find a workspace configuration matching this repo.
		for idx, conf := range spec.Workspaces {
			if !workspaceMatchers[conf].Match(string(repoRev.Repo.Name)) {
				continue
			}

			// Don't allow duplicate matches. Collect the error so we return
			// them all so users don't have to run it 1 by 1.
			if found {
				errs = errors.Append(errs, batcheslib.NewValidationError(errors.Errorf("repository %s matches multiple workspaces.in globs in the batch spec. glob: %q", repoRev.Repo.Name, conf.In)))
				continue
			}

			matched[idx] = append(matched[idx], repoRev)
			found = true
		}

		if !found {
			root = append(root, repoRev)
		}
	}
	if errs != nil {
		return nil, errs
	}

	type repoWorkspaces struct {
		*RepoRevision
		Paths              []string
		OnlyFetchWorkspace bool
	}
	workspacesByRepoRev := map[repoRevKey]repoWorkspaces{}
	for idx, repoRevs := range matched {
		conf := spec.Workspaces[idx]
		repoRevDirs, err := finder.FindDirectoriesInRepos(ctx, conf.RootAtLocationOf, repoRevs...)
		if err != nil {
			return nil, err
		}

		repoRevsByKey := map[repoRevKey]*RepoRevision{}
		for _, repoRev := range repoRevs {
			repoRevsByKey[repoRev.Key()] = repoRev
		}

		for repoRevKey, dirs := range repoRevDirs {
			// Don't add repos that don't have any matched workspaces.
			if len(dirs) == 0 {
				continue
			}
			workspacesByRepoRev[repoRevKey] = repoWorkspaces{
				RepoRevision:       repoRevsByKey[repoRevKey],
				Paths:              dirs,
				OnlyFetchWorkspace: conf.OnlyFetchWorkspace,
			}
		}
	}

	// And add the root for repos.
	for _, repoRev := range root {
		conf, ok := workspacesByRepoRev[repoRev.Key()]
		if !ok {
			workspacesByRepoRev[repoRev.Key()] = repoWorkspaces{
				RepoRevision: repoRev,
				// Root.
				Paths:              []string{""},
				OnlyFetchWorkspace: false,
			}
			continue
		}
		conf.Paths = append(conf.Paths, "")
	}

	workspaces := make([]*RepoWorkspace, 0, len(workspacesByRepoRev))
	for _, workspace := range workspacesByRepoRev {
		for _, path := range workspace.Paths {
			fetchWorkspace := workspace.OnlyFetchWorkspace
			if path == "" {
				fetchWorkspace = false
			}

			// Filter file matches by workspace. Only include paths that are
			// _within_ the directory.
			paths := []string{}
			for _, probe := range workspace.RepoRevision.FileMatches {
				if strings.HasPrefix(probe, path) {
					paths = append(paths, probe)
				}
			}

			repoRevision := *workspace.RepoRevision
			repoRevision.FileMatches = paths

			steps, err := stepsForRepo(spec, template.Repository{
				Name:        string(repoRevision.Repo.Name),
				Branch:      repoRevision.Branch,
				FileMatches: repoRevision.FileMatches,
			})
			if err != nil {
				return nil, err
			}

			// If the workspace doesn't have any steps we don't need to include it.
			if len(steps) == 0 {
				continue
			}

			workspaces = append(workspaces, &RepoWorkspace{
				RepoRevision:       &repoRevision,
				Path:               path,
				OnlyFetchWorkspace: fetchWorkspace,
			})
		}
	}

	// Stable sorting.
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].Repo.Name == workspaces[j].Repo.Name {
			return workspaces[i].Path < workspaces[j].Path
		}
		return workspaces[i].Repo.Name < workspaces[j].Repo.Name
	})

	return workspaces, nil
}

type repoRevKey struct {
	RepoID int32
	Branch string
	Commit string
}

func (r *RepoRevision) Key() repoRevKey {
	return repoRevKey{
		RepoID: int32(r.Repo.ID),
		Branch: r.Branch,
		Commit: string(r.Commit),
	}
}

// stepsForRepo calculates the steps required to run on the given repo.
func stepsForRepo(spec *batcheslib.BatchSpec, repo template.Repository) ([]batcheslib.Step, error) {
	taskSteps := []batcheslib.Step{}
	for _, step := range spec.Steps {
		// If no if condition is given, just go ahead and add the step to the list.
		if step.IfCondition() == "" {
			taskSteps = append(taskSteps, step)
			continue
		}

		batchChange := template.BatchChangeAttributes{
			Name:        spec.Name,
			Description: spec.Description,
		}
		stepCtx := &template.StepContext{
			Repository:  repo,
			BatchChange: batchChange,
		}
		static, boolVal, err := template.IsStaticBool(step.IfCondition(), stepCtx)
		if err != nil {
			return nil, err
		}

		// If we could evaluate the condition statically and the resulting
		// boolean is false, we don't add that step.
		if !static {
			taskSteps = append(taskSteps, step)
		} else if boolVal {
			taskSteps = append(taskSteps, step)
		}
	}
	return taskSteps, nil
}
