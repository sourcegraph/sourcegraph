package registry

import (
	"context"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	frontendregistry.ExtensionRegistry.PublishersFunc = extensionRegistryPublishers
}

func extensionRegistryPublishers(ctx context.Context, db dbutil.DB, args *graphqlutil.ConnectionArgs) (graphqlbackend.RegistryPublisherConnection, error) {
	var opt dbPublishersListOptions
	args.Set(&opt.LimitOffset)
	return &registryPublisherConnection{opt: opt, db: db}, nil
}

// registryPublisherConnection resolves a list of registry publishers.
type registryPublisherConnection struct {
	opt dbPublishersListOptions

	// cache results because they are used by multiple fields
	once               sync.Once
	registryPublishers []*dbPublisher
	err                error
	db                 dbutil.DB
}

func (r *registryPublisherConnection) compute(ctx context.Context) ([]*dbPublisher, error) {
	r.once.Do(func() {
		opt2 := r.opt
		if opt2.LimitOffset != nil {
			tmp := *opt2.LimitOffset
			opt2.LimitOffset = &tmp
			opt2.Limit++ // so we can detect if there is a next page
		}

		r.registryPublishers, r.err = dbExtensions{}.ListPublishers(ctx, opt2)
	})
	return r.registryPublishers, r.err
}

func (r *registryPublisherConnection) Nodes(ctx context.Context) ([]graphqlbackend.RegistryPublisher, error) {
	publishers, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	var l []graphqlbackend.RegistryPublisher
	for _, publisher := range publishers {
		p, err := getRegistryPublisher(ctx, r.db, *publisher)
		if err != nil {
			return nil, err
		}
		l = append(l, p)
	}
	return l, nil
}

func (r *registryPublisherConnection) TotalCount(ctx context.Context) (int32, error) {
	count, err := dbExtensions{}.CountPublishers(ctx, r.opt)
	return int32(count), err
}

func (r *registryPublisherConnection) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	publishers, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	return graphqlutil.HasNextPage(r.opt.LimitOffset != nil && len(publishers) > r.opt.Limit), nil
}
