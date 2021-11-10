package graphqlbackend

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

func (r *UserResolver) EventLogs(ctx context.Context, args *struct {
	graphqlutil.ConnectionArgs
	EventName *string // return only event logs matching the event name
}) (*userEventLogsConnectionResolver, error) {
	// 🚨 SECURITY: Event logs can only be viewed by the user or site admin.
	if err := backend.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}
	var opt database.EventLogsListOptions
	args.ConnectionArgs.Set(&opt.LimitOffset)
	opt.UserID = r.user.ID
	opt.EventName = args.EventName
	return &userEventLogsConnectionResolver{db: r.db, opt: opt}, nil
}

type userEventLogsConnectionResolver struct {
	db  database.DB
	opt database.EventLogsListOptions
}

func (r *userEventLogsConnectionResolver) Nodes(ctx context.Context) ([]*userEventLogResolver, error) {
	events, err := r.db.EventLogs().ListAll(ctx, r.opt)
	if err != nil {
		return nil, err
	}

	eventLogs := make([]*userEventLogResolver, 0, len(events))
	for _, event := range events {
		eventLogs = append(eventLogs, &userEventLogResolver{db: r.db, event: event})
	}

	return eventLogs, nil
}

func (r *userEventLogsConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	var count int
	var err error

	if r.opt.EventName != nil {
		count, err = r.db.EventLogs().CountByUserIDAndEventName(ctx, r.opt.UserID, *r.opt.EventName)
	} else {
		count, err = r.db.EventLogs().CountByUserID(ctx, r.opt.UserID)
	}

	return int32(count), err
}

func (r *userEventLogsConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	var count int
	var err error

	if r.opt.EventName != nil {
		count, err = r.db.EventLogs().CountByUserIDAndEventName(ctx, r.opt.UserID, *r.opt.EventName)
	} else {
		count, err = r.db.EventLogs().CountByUserID(ctx, r.opt.UserID)
	}

	if err != nil {
		return nil, err
	}
	return graphqlutil.HasNextPage(r.opt.LimitOffset != nil && count > r.opt.Limit), nil
}
