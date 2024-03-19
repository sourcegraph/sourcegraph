package service

import (
	"context"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/exhaustive/types"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func FromSearchClient(client client.SearchClient) NewSearcher {
	return newSearcherFunc(func(ctx context.Context, userID int32, q string) (SearchQuery, error) {
		if err := isSameUser(ctx, userID); err != nil {
			return nil, err
		}

		// We run queries on searcher only which makes it easier to control limits. Low
		// latency is not a priority of Search Jobs.
		q = "index:no " + q

		if strings.Contains(strings.ToLower(q), "patterntype:structural") {
			return nil, errors.New("Structural search is not supported in Search Jobs")
		}

		inputs, err := client.Plan(
			ctx,
			"V3",
			nil,
			q,
			search.Precise,
			search.Exhaustive,
			pointers.Ptr(int32(0)),
		)
		if err != nil {
			return nil, err
		}

		exhaustive, err := jobutil.NewExhaustive(inputs)
		if err != nil {
			return nil, err
		}

		return searchQuery{
			userID:     userID,
			exhaustive: exhaustive,
			clients:    client.JobClients(),
		}, nil
	})
}

type searchQuery struct {
	userID     int32
	exhaustive jobutil.Exhaustive
	clients    job.RuntimeClients
}

func (s searchQuery) RepositoryRevSpecs(ctx context.Context) *iterator.Iterator[types.RepositoryRevSpecs] {
	if err := isSameUser(ctx, s.userID); err != nil {
		iterator.New(func() ([]types.RepositoryRevSpecs, error) {
			return nil, err
		})
	}

	it := s.exhaustive.RepositoryRevSpecs(ctx, s.clients)
	return iterator.New(func() ([]types.RepositoryRevSpecs, error) {
		if it.Next() {
			repoRev := it.Current()
			var revspecs []string
			for _, rev := range repoRev.Revs {
				revStr := rev.String()
				// avoid storing empty string since our DB expects non-empty
				// string + this is easier to read in the DB.
				if revStr == "" {
					revStr = "HEAD"
				}
				revspecs = append(revspecs, revStr)
			}
			return []types.RepositoryRevSpecs{{
				Repository:         repoRev.Repo.ID,
				RevisionSpecifiers: types.RevisionSpecifierJoin(revspecs),
			}}, nil
		}

		err := it.Err()
		if isReposMissingError(err) {
			// This isn't an error for us, we just don't search anything. We don't
			// have the concept of alerts yet in search jobs.
			return nil, nil
		}
		return nil, err
	})
}

func (s searchQuery) ResolveRepositoryRevSpec(ctx context.Context, repoRevSpec types.RepositoryRevSpecs) ([]types.RepositoryRevision, error) {
	if err := isSameUser(ctx, s.userID); err != nil {
		return nil, err
	}

	repoPagerRepoRevSpec, err := s.toRepoRevSpecs(ctx, repoRevSpec)
	if err != nil {
		return nil, err
	}

	page, err := s.exhaustive.ResolveRepositoryRevSpec(ctx, s.clients, []repos.RepoRevSpecs{repoPagerRepoRevSpec})
	if isReposMissingError(err) {
		// This isn't an error for us, we just don't search anything. We don't
		// have the concept of alerts yet in search jobs.
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if page.BackendsMissing > 0 {
		return nil, errors.New("job needs to be retried, some backends are down")
	}
	var repoRevs []types.RepositoryRevision
	for _, repoRev := range page.RepoRevs {
		if repoRev.Repo.ID != repoRevSpec.Repository {
			return nil, errors.Errorf("ResolveRepositoryRevSpec returned a different repo (%d) to the input %v", repoRev.Repo.ID, repoRevSpec)
		}
		for _, rev := range repoRev.Revs {
			repoRevs = append(repoRevs, types.RepositoryRevision{
				RepositoryRevSpecs: repoRevSpec,
				Revision:           rev,
			})
		}
	}
	return repoRevs, nil
}

func (s searchQuery) toRepoRevSpecs(ctx context.Context, repoRevSpec types.RepositoryRevSpecs) (repos.RepoRevSpecs, error) {
	repo, err := s.minimalRepo(ctx, repoRevSpec.Repository)
	if err != nil {
		return repos.RepoRevSpecs{}, err
	}

	var revs []query.RevisionSpecifier
	for _, revspec := range repoRevSpec.RevisionSpecifiers.Get() {
		revs = append(revs, query.ParseRevisionSpecifier(revspec))
	}

	return repos.RepoRevSpecs{
		Repo: repo,
		Revs: revs,
	}, nil
}

func (s searchQuery) Search(ctx context.Context, repoRev types.RepositoryRevision, matchWriter MatchWriter) error {
	if err := isSameUser(ctx, s.userID); err != nil {
		return err
	}

	repo, err := s.minimalRepo(ctx, repoRev.Repository)
	if err != nil {
		return err
	}

	job := s.exhaustive.Job(&search.RepositoryRevisions{
		Repo: repo,
		Revs: []string{repoRev.Revision},
	})

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex     // serialize writes to w
	var writeRowErr error // capture if w.Write fails

	// TODO currently ignoring returned Alert
	_, err = job.Run(ctx, s.clients, streaming.StreamFunc(func(se streaming.SearchEvent) {
		// TODO fail if se.Stats indicate missing backends or other things
		// which may indicate we are might miss data.

		mu.Lock()
		defer mu.Unlock()

		for _, match := range se.Results {
			err := matchWriter.Write(match)
			if err != nil {
				cancel()
				writeRowErr = err
			}
		}
	}))

	if writeRowErr != nil {
		return writeRowErr
	}

	// TODO how should we handle cloning (gitdomain.RepoNotExistError)?

	// An empty repository we treat as success. When searching HEAD we haven't
	// yet validated the commit actually exists so we need to ignore at this
	// point. We should consider
	if repoRev.Revision == "HEAD" && errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
		return nil
	}

	return err
}

func (s searchQuery) minimalRepo(ctx context.Context, repoID api.RepoID) (sgtypes.MinimalRepo, error) {
	minimalRepos, err := s.clients.DB.Repos().ListMinimalRepos(ctx, database.ReposListOptions{
		IDs: []api.RepoID{repoID},
	})
	if err != nil {
		return sgtypes.MinimalRepo{}, err
	}
	if len(minimalRepos) != 1 {
		return sgtypes.MinimalRepo{}, errors.Errorf("looking up repo %d found %d entries", repoID, len(minimalRepos))
	}
	return minimalRepos[0], nil
}

func isReposMissingError(err error) bool {
	var m repos.MissingRepoRevsError
	return errors.Is(err, repos.ErrNoResolvedRepos) || errors.HasType(err, &m)
}
