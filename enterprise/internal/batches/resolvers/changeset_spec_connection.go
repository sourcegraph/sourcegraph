package resolvers

import (
	"context"
	"strconv"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

var _ graphqlbackend.ChangesetSpecConnectionResolver = &changesetSpecConnectionResolver{}

type changesetSpecConnectionResolver struct {
	store *store.Store

	opts        store.ListChangesetSpecsOpts
	batchSpecID int64

	// Cache results because they are used by multiple fields
	once           sync.Once
	changesetSpecs batches.ChangesetSpecs
	reposByID      map[api.RepoID]*types.Repo
	next           int64
	err            error
}

func (r *changesetSpecConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	count, err := r.store.CountChangesetSpecs(ctx, store.CountChangesetSpecsOpts{
		BatchSpecID: r.batchSpecID,
	})
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *changesetSpecConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	_, _, next, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	if next != 0 {
		// We don't use the RandID for pagination, because we can't paginate database
		// entries based on the RandID.
		return graphqlutil.NextPageCursor(strconv.Itoa(int(next))), nil
	}

	return graphqlutil.HasNextPage(false), nil
}

func (r *changesetSpecConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.ChangesetSpecResolver, error) {
	changesetSpecs, reposByID, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	resolvers := make([]graphqlbackend.ChangesetSpecResolver, 0, len(changesetSpecs))
	for _, c := range changesetSpecs {
		repo := reposByID[c.RepoID]
		// If it's not in reposByID the repository was filtered out by the
		// authz-filter.
		// In that case we'll set it anyway to nil and changesetSpecResolver
		// will treat it as "hidden".

		resolvers = append(resolvers, NewChangesetSpecResolverWithRepo(r.store, repo, c))
	}

	return resolvers, nil
}

func (r *changesetSpecConnectionResolver) compute(ctx context.Context) (batches.ChangesetSpecs, map[api.RepoID]*types.Repo, int64, error) {
	r.once.Do(func() {
		opts := r.opts
		opts.BatchSpecID = r.batchSpecID
		r.changesetSpecs, r.next, r.err = r.store.ListChangesetSpecs(ctx, opts)
		if r.err != nil {
			return
		}

		// 🚨 SECURITY: database.Repos.GetRepoIDsSet uses the authzFilter under the hood and
		// filters out repositories that the user doesn't have access to.
		r.reposByID, r.err = r.store.Repos().GetReposSetByIDs(ctx, r.changesetSpecs.RepoIDs()...)
	})

	return r.changesetSpecs, r.reposByID, r.next, r.err
}
