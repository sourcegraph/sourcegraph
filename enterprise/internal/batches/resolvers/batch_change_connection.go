package resolvers

import (
	"context"
	"strconv"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/store"
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

var _ graphqlbackend.BatchChangesConnectionResolver = &batchChangesConnectionResolver{}

type batchChangesConnectionResolver struct {
	store *store.Store
	opts  store.ListBatchChangesOpts

	// cache results because they are used by multiple fields
	once         sync.Once
	batchChanges []*batches.BatchChange
	next         int64
	err          error
}

func (r *batchChangesConnectionResolver) Nodes(ctx context.Context) ([]graphqlbackend.BatchChangeResolver, error) {
	nodes, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	resolvers := make([]graphqlbackend.BatchChangeResolver, 0, len(nodes))
	for _, c := range nodes {
		resolvers = append(resolvers, &batchChangeResolver{store: r.store, batchChange: c})
	}
	return resolvers, nil
}

func (r *batchChangesConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	opts := store.CountBatchChangesOpts{
		ChangesetID:      r.opts.ChangesetID,
		State:            r.opts.State,
		InitialApplierID: r.opts.InitialApplierID,
		NamespaceUserID:  r.opts.NamespaceUserID,
		NamespaceOrgID:   r.opts.NamespaceOrgID,
	}
	count, err := r.store.CountBatchChanges(ctx, opts)
	return int32(count), err
}

func (r *batchChangesConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	_, next, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	if next != 0 {
		return graphqlutil.NextPageCursor(strconv.Itoa(int(next))), nil
	}
	return graphqlutil.HasNextPage(false), nil
}

func (r *batchChangesConnectionResolver) compute(ctx context.Context) ([]*batches.BatchChange, int64, error) {
	r.once.Do(func() {
		r.batchChanges, r.next, r.err = r.store.ListBatchChanges(ctx, r.opts)
	})
	return r.batchChanges, r.next, r.err
}
