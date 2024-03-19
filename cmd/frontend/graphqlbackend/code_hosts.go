package graphqlbackend

import (
	"context"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type CodeHostsArgs struct {
	First  int32
	After  *string
	Search *string
}

func (r *schemaResolver) CodeHosts(ctx context.Context, args *CodeHostsArgs) (*codeHostConnectionResolver, error) {
	// Security 🚨: Code Hosts may only be viewed by site admins for now.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	opts := database.ListCodeHostsOpts{
		LimitOffset: &database.LimitOffset{Limit: int(args.First)},
	}
	if args.Search != nil {
		opts.Search = *args.Search
	}
	if args.After != nil {
		id, err := UnmarshalCodeHostID(graphql.ID(*args.After))
		if err != nil {
			return nil, err
		}
		opts.Cursor = id
	}

	return &codeHostConnectionResolver{
		db:   r.db,
		opts: opts,
	}, nil
}

type codeHostConnectionResolver struct {
	db   database.DB
	opts database.ListCodeHostsOpts

	// cache results because they are used by multiple fields
	once sync.Once
	chs  []*types.CodeHost
	next int32
	err  error
}

func (r *codeHostConnectionResolver) IsMigrationDone(ctx context.Context) (bool, error) {
	store := oobmigration.NewStoreWithDB(r.db)
	// 24 is the magical hard-coded ID of the migration that creates code hosts.
	m, ok, err := store.GetByID(ctx, 24)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return m.Complete(), nil
}

func (r *codeHostConnectionResolver) Nodes(ctx context.Context) ([]*codeHostResolver, error) {
	nodes, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	resolvers := make([]*codeHostResolver, 0, len(nodes))
	for _, ch := range nodes {
		resolvers = append(resolvers, &codeHostResolver{db: r.db, ch: ch})
	}
	return resolvers, nil
}

func (r *codeHostConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	// Reset pagination cursor to get correct total count
	opt := r.opts
	opt.Cursor = 0
	return r.db.CodeHosts().Count(ctx, opt)
}

func (r *codeHostConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	_, next, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	if next != 0 {
		return graphqlutil.NextPageCursor(string(MarshalCodeHostID(next))), nil
	}
	return graphqlutil.HasNextPage(false), nil

}

func (r *codeHostConnectionResolver) compute(ctx context.Context) ([]*types.CodeHost, int32, error) {
	r.once.Do(func() {
		r.chs, r.next, r.err = r.db.CodeHosts().List(ctx, r.opts)
	})
	return r.chs, r.next, r.err
}

const CodeHostKind = "CodeHost"

func MarshalCodeHostID(id int32) graphql.ID {
	return relay.MarshalID(CodeHostKind, id)
}

func UnmarshalCodeHostID(gqlID graphql.ID) (id int32, err error) {
	err = relay.UnmarshalSpec(gqlID, &id)
	return
}

func CodeHostByID(ctx context.Context, db database.DB, id graphql.ID) (*codeHostResolver, error) {
	// Security 🚨: Code Hosts may only be viewed by site admins for now.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, db); err != nil {
		return nil, err
	}

	intID, err := UnmarshalCodeHostID(id)
	if err != nil {
		return nil, err
	}
	return CodeHostByIDInt32(ctx, db, intID)
}

func CodeHostByIDInt32(ctx context.Context, db database.DB, id int32) (*codeHostResolver, error) {
	ch, err := db.CodeHosts().GetByID(ctx, id)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &codeHostResolver{ch: ch, db: db}, nil
}
