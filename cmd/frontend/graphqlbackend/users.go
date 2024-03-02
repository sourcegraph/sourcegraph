package graphqlbackend

import (
	"context"
	"strconv"
	"sync"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/rbac"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type usersArgs struct {
	graphqlutil.ConnectionArgs
	After         *string
	Query         *string
	ActivePeriod  *string
	InactiveSince *gqlutil.DateTime
}

func (r *schemaResolver) Users(ctx context.Context, args *usersArgs) (*userConnectionResolver, error) {
	// 🚨 SECURITY: Only license manager and site admins can list users on sourcegraph.com.
	if envvar.SourcegraphDotComMode() {
		// First, check if they have the rbac permission for license manager.
		if err := rbac.CheckCurrentUserHasPermission(ctx, r.db, rbac.LicenseManagerReadPermission); err != nil {
			// Otherwise, check that they are site admin.
			if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
				return nil, err
			}
		}
	}

	opt := database.UsersListOptions{
		ExcludeSourcegraphOperators: true,
	}
	if args.Query != nil {
		opt.Query = *args.Query
	}
	if args.InactiveSince != nil {
		opt.InactiveSince = args.InactiveSince.Time
	}
	args.ConnectionArgs.Set(&opt.LimitOffset)
	if args.After != nil && opt.LimitOffset != nil {
		cursor, err := strconv.ParseInt(*args.After, 10, 32)
		if err != nil {
			return nil, err
		}
		opt.LimitOffset.Offset = int(cursor)
	}

	return &userConnectionResolver{db: r.db, opt: opt, activePeriod: args.ActivePeriod}, nil
}

type UserConnectionResolver interface {
	Nodes(ctx context.Context) ([]*UserResolver, error)
	TotalCount(ctx context.Context) (int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
}

var _ UserConnectionResolver = &userConnectionResolver{}

type userConnectionResolver struct {
	db           database.DB
	opt          database.UsersListOptions
	activePeriod *string

	// cache results because they are used by multiple fields
	once       sync.Once
	users      []*types.User
	totalCount int
	err        error
}

func (r *userConnectionResolver) compute(ctx context.Context) ([]*types.User, int, error) {
	r.once.Do(func() {
		var err error
		if r.activePeriod != nil && *r.activePeriod != "ALL_TIME" {
			switch *r.activePeriod {
			case "TODAY":
				r.opt.UserIDs, err = usagestats.ListRegisteredUsersToday(ctx, r.db)
			case "THIS_WEEK":
				r.opt.UserIDs, err = usagestats.ListRegisteredUsersThisWeek(ctx, r.db)
			case "THIS_MONTH":
				r.opt.UserIDs, err = usagestats.ListRegisteredUsersThisMonth(ctx, r.db)
			default:
				err = errors.Errorf("unknown user active period %s", *r.activePeriod)
			}
		}
		if err != nil {
			r.err = err
			return
		}

		r.users, err = r.db.Users().List(ctx, &r.opt)
		if err != nil {
			r.err = err
			return
		}
		r.totalCount, r.err = r.db.Users().Count(ctx, &r.opt)
	})
	return r.users, r.totalCount, r.err
}

func (r *userConnectionResolver) Nodes(ctx context.Context) ([]*UserResolver, error) {
	users, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	var l []*UserResolver
	for _, user := range users {
		l = append(l, NewUserResolver(ctx, r.db, user))
	}
	return l, nil
}

func (r *userConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	_, count, err := r.compute(ctx)
	return int32(count), err
}

func (r *userConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	users, totalCount, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	// We would have had all results when no limit set
	if r.opt.LimitOffset == nil {
		return graphqlutil.HasNextPage(false), nil
	}

	after := r.opt.LimitOffset.Offset + len(users)

	// We got less results than limit, means we've had all results
	if after < r.opt.Limit {
		return graphqlutil.HasNextPage(false), nil
	}

	if totalCount > after {
		return graphqlutil.NextPageCursor(strconv.Itoa(after)), nil
	}
	return graphqlutil.HasNextPage(false), nil
}
