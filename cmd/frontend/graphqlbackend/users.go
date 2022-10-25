package graphqlbackend

import (
	"context"
	"strconv"
	"sync"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type usersArgs struct {
	graphqlutil.ConnectionArgs
	After         *string
	Query         *string
	Tag           *string
	ActivePeriod  *string
	InactiveSince *gqlutil.DateTime
}

func (r *schemaResolver) Users(args *usersArgs) (*userConnectionResolver, error) {
	var opt database.UsersListOptions
	if args.Query != nil {
		opt.Query = *args.Query
	}
	if args.Tag != nil {
		opt.Tag = *args.Tag
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

// compute caches results from the more expensive user list creation that occurs when activePeriod
// is set to a specific length of time.
func (r *userConnectionResolver) compute(ctx context.Context) ([]*types.User, int, error) {
	if r.activePeriod == nil {
		return nil, 0, errors.New("activePeriod must not be nil")
	}
	r.once.Do(func() {
		var err error
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
	// 🚨 SECURITY: Only site admins can list users and only org members can
	// list other org members.
	if r.opt.OrgId == nil {
		if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
			return nil, err
		}
	} else {
		if err := auth.CheckOrgAccessOrSiteAdmin(ctx, r.db, *r.opt.OrgId); err != nil {
			if err == auth.ErrNotAnOrgMember {
				return nil, errors.New("must be a member of this organization to view members")
			}
			return nil, err
		}
	}

	var users []*types.User
	var err error
	if r.useCache() {
		users, _, err = r.compute(ctx)
	} else {
		users, err = r.db.Users().List(ctx, &r.opt)
	}
	if err != nil {
		return nil, err
	}

	var l []*UserResolver
	for _, user := range users {
		l = append(l, &UserResolver{
			db:   r.db,
			user: user,
			logger: log.Scoped("userResolver", "resolves a specific user").With(
				log.Object("repo",
					log.String("user", user.Username))),
		})
	}
	return l, nil
}

func (r *userConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	// 🚨 SECURITY: Only site admins can count users and only org members can
	// count other org members.
	if r.opt.OrgId == nil {
		if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
			return 0, err
		}
	} else {
		if err := auth.CheckOrgAccessOrSiteAdmin(ctx, r.db, *r.opt.OrgId); err != nil {
			if err == auth.ErrNotAnOrgMember {
				return 0, errors.New("must be a member of this organization to view members")
			}
			return 0, err
		}
	}

	var count int
	var err error
	if r.useCache() {
		_, count, err = r.compute(ctx)
	} else {
		count, err = r.db.Users().Count(ctx, &r.opt)
	}
	return int32(count), err
}

func (r *userConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	var users []*types.User
	var err error
	if r.useCache() {
		users, _, err = r.compute(ctx)
	} else {
		users, err = r.db.Users().List(ctx, &r.opt)
	}
	if err != nil {
		return nil, err
	}

	after := r.opt.LimitOffset.Offset + len(users)

	// We would have had all results when no limit set
	if r.opt.LimitOffset == nil {
		return graphqlutil.HasNextPage(false), nil
	}

	// We got less results than limit, means we've had all results
	if after < r.opt.Limit {
		return graphqlutil.HasNextPage(false), nil
	}

	// In case the number of results happens to be the same as the limit,
	// we need another query to get accurate total count with same cursor
	// to determine if there are more results than the limit we set.
	totalCount, err := r.TotalCount(ctx)
	if err != nil {
		return nil, err
	}

	if int(totalCount) > after {
		return graphqlutil.NextPageCursor(strconv.Itoa(after)), nil
	}
	return graphqlutil.HasNextPage(false), nil
}

func (r *userConnectionResolver) useCache() bool {
	return r.activePeriod != nil && *r.activePeriod != "ALL_TIME"
}
