package graphqlbackend

import (
	"context"

	logger "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
)

func (r *schemaResolver) Organizations(args *struct {
	graphqlutil.ConnectionArgs
	Query *string
}) *orgConnectionResolver {
	var opt database.OrgsListOptions
	if args.Query != nil {
		opt.Query = *args.Query
	}
	args.ConnectionArgs.Set(&opt.LimitOffset)
	return &orgConnectionResolver{db: r.db, opt: opt}
}

type orgConnectionResolver struct {
	db  database.DB
	opt database.OrgsListOptions
}

func (r *orgConnectionResolver) Nodes(ctx context.Context) ([]*OrgResolver, error) {
	// 🚨 SECURITY: Only site admins can list organisations.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	orgs, err := r.db.Orgs().List(ctx, &r.opt)
	if err != nil {
		return nil, err
	}

	var l []*OrgResolver
	for _, org := range orgs {
		l = append(l, &OrgResolver{
			db:  r.db,
			org: org,
		})
	}
	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

		// Log an event when listing organizations.
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameOrgListViewed, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", nil); err != nil {
			logger.Error(err)

		}
	}
	return l, nil
}

func (r *orgConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	// 🚨 SECURITY: Only site admins can count organisations.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return 0, err
	}

	count, err := r.db.Orgs().Count(ctx, r.opt)
	return int32(count), err
}

type orgConnectionStaticResolver struct {
	nodes []*OrgResolver
}

func (r *orgConnectionStaticResolver) Nodes() []*OrgResolver { return r.nodes }
func (r *orgConnectionStaticResolver) TotalCount() int32     { return int32(len(r.nodes)) }
func (r *orgConnectionStaticResolver) PageInfo() *graphqlutil.PageInfo {
	return graphqlutil.HasNextPage(false)
}
