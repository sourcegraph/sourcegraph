package resolvers

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/syncer"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type changesetsConnectionResolver struct {
	store *store.Store

	opts store.ListChangesetsOpts
	// 🚨 SECURITY: If the given opts do not reveal hidden information about a
	// changeset by including the changeset in the result set, this should be
	// set to true.
	optsSafe bool

	// changesets contains all changesets in this connection,
	// without any pagination.
	// We need them to reliably determine pages, TotalCount and Stats and we
	// need to load all, without a limit, because some might be filtered out by
	// the authzFilter.
	once           sync.Once
	changesets     batches.Changesets
	changesetsPage batches.Changesets
	err            error
	reposByID      map[api.RepoID]*types.Repo
}

func (r *changesetsConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.ChangesetResolver, error) {
	_, changesetsPage, reposByID, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	scheduledSyncs := make(map[int64]time.Time)
	changesetIDs := changesetsPage.IDs()
	if len(changesetIDs) > 0 {
		syncData, err := r.store.ListChangesetSyncData(ctx, store.ListChangesetSyncDataOpts{ChangesetIDs: changesetIDs})
		if err != nil {
			return nil, err
		}
		for _, d := range syncData {
			scheduledSyncs[d.ChangesetID] = syncer.NextSync(r.store.Clock(), d)
		}
	}

	resolvers := make([]graphqlbackend.ChangesetResolver, 0, len(changesetsPage))
	for _, c := range changesetsPage {
		resolvers = append(resolvers, NewChangesetResolverWithNextSync(r.store, c, reposByID[c.RepoID], scheduledSyncs[c.ID]))
	}

	return resolvers, nil
}

func (r *changesetsConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	cs, _, _, err := r.compute(ctx)
	if err != nil {
		return 0, err
	}
	return int32(len(cs)), nil
}

// compute loads all changesets matched by r.opts, but without a
// limit.
// If r.optsSafe is true, it returns all of them. If not, it filters out the
// ones to which the user doesn't have access.
func (r *changesetsConnectionResolver) compute(ctx context.Context) (allChangesets, currentPage batches.Changesets, reposByID map[api.RepoID]*types.Repo, err error) {
	r.once.Do(func() {
		pageSlice := func(changesets batches.Changesets) batches.Changesets {
			limit := r.opts.Limit
			if limit <= 0 {
				limit = len(changesets)
			}
			slice := changesets.Filter(func(cs *batches.Changeset) bool { return cs.ID > r.opts.Cursor })
			if len(slice) > limit {
				slice = slice[:limit]
			}
			return slice
		}

		opts := r.opts
		opts.Limit = 0
		opts.Cursor = 0

		cs, _, err := r.store.ListChangesets(ctx, opts)
		if err != nil {
			r.err = err
			return
		}

		// 🚨 SECURITY: database.Repos.GetRepoIDsSet uses the authzFilter under the hood and
		// filters out repositories that the user doesn't have access to.
		r.reposByID, err = r.store.Repos().GetReposSetByIDs(ctx, cs.RepoIDs()...)
		if err != nil {
			r.err = err
			return
		}

		// 🚨 SECURITY: If the opts do not leak information, we can return the
		// number of changesets. Otherwise we have to filter the changesets by
		// accessible repos.
		if r.optsSafe {
			r.changesets = cs
			r.changesetsPage = pageSlice(cs)
			return
		}

		accessibleChangesets := make(batches.Changesets, 0)
		for _, c := range cs {
			if _, ok := r.reposByID[c.RepoID]; !ok {
				continue
			}
			accessibleChangesets = append(accessibleChangesets, c)
		}

		r.changesets = accessibleChangesets
		r.changesetsPage = pageSlice(accessibleChangesets)
	})

	return r.changesets, r.changesetsPage, r.reposByID, r.err
}

func (r *changesetsConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	all, page, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	if len(all) > 0 && len(page) > 0 && page[len(page)-1].ID != all[len(all)-1].ID {
		return graphqlutil.NextPageCursor(strconv.Itoa(int(page[len(page)-1].ID))), nil
	}

	return graphqlutil.HasNextPage(false), nil
}
