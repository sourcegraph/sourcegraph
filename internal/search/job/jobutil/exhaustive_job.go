package jobutil

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
)

// Exhaustive exports what is needed for the search jobs product (exhaustive
// search). The naming conflict between the product search jobs and the search
// job infrastructure is unfortunate. So we use the name exhaustive to
// differentiate ourselves from the infrastructure.
type Exhaustive struct {
	repoPagerJob *repoPagerJob
}

const (
	exhaustiveSupportedResultTypes = result.TypeCommit | result.TypeDiff | result.TypeFile | result.TypePath
	exhaustiveDefaultResultTypes   = result.TypeFile | result.TypePath
)

// NewExhaustive constructs Exhaustive from the search inputs.
//
// It will return an error if the input query is not supported by Exhaustive.
func NewExhaustive(inputs *search.Inputs) (Exhaustive, error) {
	if inputs.Protocol != search.Exhaustive {
		return Exhaustive{}, errors.New("only works for exhaustive search inputs")
	}

	if len(inputs.Plan) != 1 {
		return Exhaustive{}, errors.Errorf("Invalid query: Search Jobs does not support using AND/OR with filters (for example \"file:.go or file:.rs\"). Consider splitting your query into multiple subqueries and creating separate search jobs for each.")
	}

	b := inputs.Plan[0]

	if v, _ := b.ToParseTree().StringValue(query.FieldSelect); v != "" {
		return Exhaustive{}, errors.Errorf("select: is not supported in Search Jobs")
	}

	// We don't support file predicates, such as file:has.content(), because the
	// search breaks in unexpected ways. For example, for interactive search
	// file:has.content() is translated to an AND query which we don't support in
	// Search Jobs yet.
	if pred, ok := hasPredicates(query.FieldFile, inputs.Query); ok {
		return Exhaustive{}, errors.Errorf("file predicates are not supported. Got %v", pred)
	}

	// This is a very weak protection but should be enough to catch simple misuse.
	if inputs.PatternType == query.SearchTypeRegex {
		if term, ok := b.Pattern.(query.Pattern); ok && term.Value == ".*" {
			return Exhaustive{}, errors.Errorf("regex search with .* is not supported")
		}
	}

	repoOptions := toRepoOptions(b, inputs.UserSettings)
	resultTypes := computeResultTypes(b, inputs.PatternType, exhaustiveDefaultResultTypes)

	if resultTypes.Without(exhaustiveSupportedResultTypes) != 0 {
		return Exhaustive{}, errors.Errorf("your query contains the following type filters: %v. However Search Jobs only supports: %v.", resultTypes, exhaustiveSupportedResultTypes)
	}

	var planJob job.Job

	if resultTypes.Has(result.TypeCommit | result.TypeDiff) {
		_, _, own := isOwnershipSearch(b)
		diff := resultTypes.Has(result.TypeDiff)
		// Follows the logic of interactive search, see
		// https://github.com/sourcegraph/sourcegraph/pull/35741
		repoOptionsCopy := repoOptions
		repoOptionsCopy.OnlyCloned = true

		commitSearchJob := &commit.SearchJob{
			Query:                commit.QueryToGitQuery(b, diff),
			Diff:                 diff,
			Limit:                b.MaxResults(inputs.DefaultLimit()),
			IncludeModifiedFiles: authz.SubRepoEnabled(authz.DefaultSubRepoPermsChecker) || own,
		}

		planJob =
			&repoPagerJob{
				child:            &reposPartialJob{commitSearchJob},
				repoOpts:         repoOptionsCopy,
				containsRefGlobs: query.ContainsRefGlobs(b.ToParseTree()),
				skipPartitioning: true,
			}
	} else if resultTypes.Has(result.TypeFile | result.TypePath) {
		planJob = NewTextSearchJob(b, inputs, resultTypes, repoOptions)
	} else {
		// This should never happen because we checked for supported types above.
		return Exhaustive{}, errors.Errorf("internal error: unsupported result types %v", resultTypes)
	}

	repoPagerJob, ok := planJob.(*repoPagerJob)
	if !ok {
		return Exhaustive{}, errors.Errorf("internal error: expected a repo pager job when converting plan into search jobs got %T", planJob)
	}

	return Exhaustive{
		repoPagerJob: repoPagerJob,
	}, nil
}

func hasPredicates(field string, q query.Q) (pred string, ok bool) {
	values, negated := q.StringValues(field)
	for _, v := range append(values, negated...) {
		pred, _, ok = query.ScanPredicate(field, []byte(v), query.DefaultPredicateRegistry)
		if ok {
			break
		}
	}
	return
}

func (e Exhaustive) Job(repoRevs *search.RepositoryRevisions) job.Job {
	// TODO should we add in a timeout and limit here?
	// TODO should we support indexed search and run through zoekt.PartitionRepos?
	return e.repoPagerJob.child.Resolve(resolvedRepos{
		unindexed: []*search.RepositoryRevisions{repoRevs},
	})
}

// RepositoryRevSpecs is a wrapper around repos.Resolver.IterateRepoRevs.
func (e Exhaustive) RepositoryRevSpecs(ctx context.Context, clients job.RuntimeClients) *iterator.Iterator[repos.RepoRevSpecs] {
	return reposNewResolver(clients).IterateRepoRevs(ctx, e.repoPagerJob.repoOpts)
}

// ResolveRepositoryRevSpec is a wrapper around repos.Resolver.ResolveRevSpecs.
func (e Exhaustive) ResolveRepositoryRevSpec(ctx context.Context, clients job.RuntimeClients, repoRevSpecs []repos.RepoRevSpecs) (repos.Resolved, error) {
	return reposNewResolver(clients).ResolveRevSpecs(ctx, e.repoPagerJob.repoOpts, repoRevSpecs)
}

func reposNewResolver(clients job.RuntimeClients) *repos.Resolver {
	return repos.NewResolver(clients.Logger, clients.DB, clients.Gitserver, clients.SearcherURLs, clients.SearcherGRPCConnectionCache, clients.Zoekt)
}
