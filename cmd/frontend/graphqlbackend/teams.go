package graphqlbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ListTeamsArgs struct {
	First  *int32
	After  *string
	Search *string
}

type teamConnectionResolver struct {
	db       database.DB
	parentID int32
	search   string
	cursor   int32
	limit    int
	once     sync.Once
	teams    []*types.Team
	pageInfo *graphqlutil.PageInfo
	err      error
}

// applyArgs unmarshals query conditions and limites set in `ListTeamsArgs`
// into `teamConnectionResolver` fields for convenient use in database query.
func (r *teamConnectionResolver) applyArgs(args *ListTeamsArgs) error {
	if args.After != nil {
		cursor, err := graphqlutil.DecodeIntCursor(args.After)
		if err != nil {
			return err
		}
		r.cursor = int32(cursor)
		if int(r.cursor) != cursor {
			return errors.Newf("cursor int32 overflow: %d", cursor)
		}
	}
	if args.Search != nil {
		r.search = *args.Search
	}
	if args.First != nil {
		r.limit = int(*args.First)
	}
	return nil
}

// compute resolves teams queried for this resolver.
// The result of running it is setting `teams`, `next` and `err`
// fields on the resolver. This ensures that resolving multiple
// graphQL attributes that require listing (like `pageInfo` and `nodes`)
// results in just one query.
func (r *teamConnectionResolver) compute(ctx context.Context) {
	r.once.Do(func() {
		opts := database.ListTeamsOpts{
			Cursor:       r.cursor,
			WithParentID: r.parentID,
			Search:       r.search,
		}
		if r.limit != 0 {
			opts.LimitOffset = &database.LimitOffset{Limit: r.limit}
		}
		teams, next, err := r.db.Teams().ListTeams(ctx, opts)
		if err != nil {
			r.err = err
			return
		}
		r.teams = teams
		if next > 0 {
			r.pageInfo = graphqlutil.EncodeIntCursor(&next)
		} else {
			r.pageInfo = graphqlutil.HasNextPage(false)
		}
	})
}

func (r *teamConnectionResolver) TotalCount(ctx context.Context, args *struct{ CountDeeplyNestedTeams bool }) (int32, error) {
	if args != nil && args.CountDeeplyNestedTeams {
		return 0, errors.New("Not supported: counting deeply nested teams.")
	}
	// Not taking into account limit or cursor for count.
	opts := database.ListTeamsOpts{
		WithParentID: r.parentID,
		Search:       r.search,
	}
	return r.db.Teams().CountTeams(ctx, opts)
}

func (r *teamConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	r.compute(ctx)
	return r.pageInfo, r.err
}

func (r *teamConnectionResolver) Nodes(ctx context.Context) ([]*TeamResolver, error) {
	r.compute(ctx)
	if r.err != nil {
		return nil, r.err
	}
	var rs []*TeamResolver
	for _, t := range r.teams {
		rs = append(rs, &TeamResolver{
			db:   r.db,
			team: t,
		})
	}
	return rs, nil
}

type TeamResolver struct {
	db   database.DB
	team *types.Team
}

func (r *TeamResolver) ID() graphql.ID {
	return relay.MarshalID("Team", r.team.ID)
}

func (r *TeamResolver) Name() string {
	return r.team.Name
}

func (r *TeamResolver) URL() string {
	absolutePath := fmt.Sprintf("/teams/%s", r.team.Name)
	u := &url.URL{Path: absolutePath}
	return u.String()
}

func (r *TeamResolver) DisplayName() *string {
	if r.team.DisplayName == "" {
		return nil
	}
	return &r.team.DisplayName
}

func (r *TeamResolver) Readonly() bool {
	return r.team.ReadOnly
}

func (r *TeamResolver) ParentTeam(ctx context.Context) (*TeamResolver, error) {
	if r.team.ParentTeamID == 0 {
		return nil, nil
	}
	parentTeam, err := r.db.Teams().GetTeamByID(ctx, r.team.ParentTeamID)
	if err != nil {
		return nil, err
	}
	return &TeamResolver{team: parentTeam, db: r.db}, nil
}

func (r *TeamResolver) ViewerCanAdminister(ctx context.Context) bool {
	// 🚨 SECURITY: For now administration is only allowed for site admins.
	err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db)
	return err == nil
}

func (r *TeamResolver) Members(ctx context.Context, args *ListTeamMembersArgs) (*teamMemberConnection, error) {
	c := &teamMemberConnection{
		db:     r.db,
		teamID: r.team.ID,
	}
	if err := c.applyArgs(args); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *TeamResolver) ChildTeams(ctx context.Context, args *ListTeamsArgs) (*teamConnectionResolver, error) {
	c := &teamConnectionResolver{
		db:       r.db,
		parentID: r.team.ID,
	}
	if err := c.applyArgs(args); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *TeamResolver) OwnerField() string {
	return EnterpriseResolvers.ownResolver.TeamOwnerField(r)
}

type ListTeamMembersArgs struct {
	First  *int32
	After  *string
	Search *string
}

type teamMemberConnection struct {
	db       database.DB
	teamID   int32
	cursor   teamMemberListCursor
	search   string
	limit    int
	once     sync.Once
	nodes    []*types.TeamMember
	pageInfo *graphqlutil.PageInfo
	err      error
}

type teamMemberListCursor struct {
	TeamID int32 `json:"team,omitempty"`
	UserID int32 `json:"user,omitempty"`
}

// applyArgs unmarshals query conditions and limites set in `ListTeamMembersArgs`
// into `teamMemberConnection` fields for convenient use in database query.
func (r *teamMemberConnection) applyArgs(args *ListTeamMembersArgs) error {
	if args.After != nil && *args.After != "" {
		cursorText, err := graphqlutil.DecodeCursor(args.After)
		if err != nil {
			return err
		}
		if err := json.Unmarshal([]byte(cursorText), &r.cursor); err != nil {
			return err
		}
	}
	if args.Search != nil {
		r.search = *args.Search
	}
	if args.First != nil {
		r.limit = int(*args.First)
	}
	return nil
}

// compute resolves team members queried for this resolver.
// The result of running it is setting `nodes`, `pageInfo` and `err`
// fields on the resolver. This ensures that resolving multiple
// graphQL attributes that require listing (like `pageInfo` and `nodes`)
// results in just one query.
func (r *teamMemberConnection) compute(ctx context.Context) {
	r.once.Do(func() {
		opts := database.ListTeamMembersOpts{
			Cursor: database.TeamMemberListCursor{
				TeamID: r.cursor.TeamID,
				UserID: r.cursor.UserID,
			},
			TeamID: r.teamID,
			Search: r.search,
		}
		if r.limit != 0 {
			opts.LimitOffset = &database.LimitOffset{Limit: r.limit}
		}
		nodes, next, err := r.db.Teams().ListTeamMembers(ctx, opts)
		if err != nil {
			r.err = err
			return
		}
		r.nodes = nodes
		if next != nil {
			cursorStruct := teamMemberListCursor{
				TeamID: next.TeamID,
				UserID: next.UserID,
			}
			cursorBytes, err := json.Marshal(&cursorStruct)
			if err != nil {
				r.err = errors.Wrap(err, "error encoding pageInfo")
			}
			cursorString := string(cursorBytes)
			r.pageInfo = graphqlutil.EncodeCursor(&cursorString)
		} else {
			r.pageInfo = graphqlutil.HasNextPage(false)
		}
	})
}

func (r *teamMemberConnection) TotalCount(ctx context.Context, args *struct{ CountDeeplyNestedTeamMembers bool }) (int32, error) {
	if args != nil && args.CountDeeplyNestedTeamMembers {
		return 0, errors.New("Not supported: counting deeply nested team members.")
	}
	// Not taking into account limit or cursor for count.
	opts := database.ListTeamMembersOpts{
		TeamID: r.teamID,
		Search: r.search,
	}
	return r.db.Teams().CountTeamMembers(ctx, opts)
}

func (r *teamMemberConnection) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	r.compute(ctx)
	if r.err != nil {
		return nil, r.err
	}
	return r.pageInfo, nil
}

func (r *teamMemberConnection) Nodes(ctx context.Context) ([]*UserResolver, error) {
	r.compute(ctx)
	if r.err != nil {
		return nil, r.err
	}
	var rs []*UserResolver
	// 🚨 Query in a loop is inefficient: Follow up with another pull request
	// to where team members query joins with users and fetches them in one go.
	for _, n := range r.nodes {
		if n.UserID == 0 {
			// 🚨 At this point only User can be a team member, so user ID should
			// always be present. If not, return a `null` team member.
			rs = append(rs, nil)
			continue
		}
		user, err := r.db.Users().GetByID(ctx, n.UserID)
		if err != nil {
			return nil, err
		}
		rs = append(rs, NewUserResolver(r.db, user))
	}
	return rs, nil
}

type CreateTeamArgs struct {
	Name           string
	DisplayName    *string
	ReadOnly       bool
	ParentTeam     *graphql.ID
	ParentTeamName *string
}

func (r *schemaResolver) CreateTeam(ctx context.Context, args *CreateTeamArgs) (*TeamResolver, error) {
	// 🚨 SECURITY: For now we only allow site admins to create teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can create teams")
	}
	teams := r.db.Teams()
	var t types.Team
	t.Name = args.Name
	if args.DisplayName != nil {
		t.DisplayName = *args.DisplayName
	}
	t.ReadOnly = args.ReadOnly
	if args.ParentTeam != nil && args.ParentTeamName != nil {
		return nil, errors.New("must specify at most one: ParentTeam or ParentTeamName")
	}
	parentTeam, err := findTeam(ctx, teams, args.ParentTeam, args.ParentTeamName)
	if err != nil {
		return nil, errors.Wrap(err, "parent team")
	}
	if parentTeam != nil {
		t.ParentTeamID = parentTeam.ID
	}
	t.CreatorID = actor.FromContext(ctx).UID
	if err := teams.CreateTeam(ctx, &t); err != nil {
		return nil, err
	}
	return &TeamResolver{team: &t, db: r.db}, nil
}

type UpdateTeamArgs struct {
	ID             *graphql.ID
	Name           *string
	DisplayName    *string
	ParentTeam     *graphql.ID
	ParentTeamName *string
}

func (r *schemaResolver) UpdateTeam(ctx context.Context, args *UpdateTeamArgs) (*TeamResolver, error) {
	// 🚨 SECURITY: For now we only allow site admins to create teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can update teams")
	}
	if args.ID == nil && args.Name == nil {
		return nil, errors.New("team to update is identified by either id or name, but neither was specified")
	}
	if args.ID != nil && args.Name != nil {
		return nil, errors.New("team to update is identified by either id or name, but both were specified")
	}
	if args.ParentTeam != nil && args.ParentTeamName != nil {
		return nil, errors.New("parent team is identified by either id or name, but both were specified")
	}
	var t *types.Team
	err := r.db.WithTransact(ctx, func(tx database.DB) (err error) {
		t, err = findTeam(ctx, tx.Teams(), args.ID, args.Name)
		if err != nil {
			return err
		}
		var needsUpdate bool
		if args.DisplayName != nil && *args.DisplayName != t.DisplayName {
			needsUpdate = true
			t.DisplayName = *args.DisplayName
		}
		if args.ParentTeam != nil || args.ParentTeamName != nil {
			parentTeam, err := findTeam(ctx, tx.Teams(), args.ParentTeam, args.ParentTeamName)
			if err != nil {
				return errors.Wrap(err, "cannot find parent team")
			}
			if parentTeam.ID != t.ParentTeamID {
				needsUpdate = true
				t.ParentTeamID = parentTeam.ID
			}
		}
		if needsUpdate {
			return tx.Teams().UpdateTeam(ctx, t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &TeamResolver{team: t, db: r.db}, nil
}

// findTeam returns a team by either GraphQL ID or name.
// If both parameters are nil, the result is nil.
func findTeam(ctx context.Context, teams database.TeamStore, graphqlID *graphql.ID, name *string) (*types.Team, error) {
	if graphqlID != nil {
		var id int32
		err := relay.UnmarshalSpec(*graphqlID, &id)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot interpret team id: %q", *graphqlID)
		}
		team, err := teams.GetTeamByID(ctx, id)
		if errcode.IsNotFound(err) {
			return nil, errors.Wrapf(err, "team id=%d not found", id)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "error fetching team id=%d", id)
		}
		return team, nil
	}
	if name != nil {
		team, err := teams.GetTeamByName(ctx, *name)
		if errcode.IsNotFound(err) {
			return nil, errors.Wrapf(err, "team name=%q not found", *name)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "could not fetch team name=%q", *name)
		}
		return team, nil
	}
	return nil, nil
}

type DeleteTeamArgs struct {
	ID   *graphql.ID
	Name *string
}

func (r *schemaResolver) DeleteTeam(ctx context.Context, args *DeleteTeamArgs) (*EmptyResponse, error) {
	// 🚨 SECURITY: For now we only allow site admins to create teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can delete teams")
	}
	if args.ID == nil && args.Name == nil {
		return nil, errors.New("team to delete is identified by either id or name, but neither was specified")
	}
	if args.ID != nil && args.Name != nil {
		return nil, errors.New("team to delete is identified by either id or name, but both were specified")
	}
	t, err := findTeam(ctx, r.db.Teams(), args.ID, args.Name)
	if err != nil {
		return nil, err
	}
	if err := r.db.Teams().DeleteTeam(ctx, t.ID); err != nil {
		return nil, err
	}
	return &EmptyResponse{}, nil
}

type TeamMembersArgs struct {
	Team     *graphql.ID
	TeamName *string
	Members  []TeamMemberInput
}

type TeamMemberInput struct {
	ID                         *graphql.ID
	Username                   *string
	Email                      *string
	ExternalAccountServiceID   *string
	ExternalAccountServiceType *string
	ExternalAccountAccountID   *string
	ExternalAccountLogin       *string
}

func (r *schemaResolver) AddTeamMembers(ctx context.Context, args *TeamMembersArgs) (*TeamResolver, error) {
	// 🚨 SECURITY: For now we only allow site admins to use teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can modify team members")
	}
	if args.Team == nil && args.TeamName == nil {
		return nil, errors.New("team must be identified by either id (team parameter) or name (teamName parameter), none specified")
	}
	if args.Team != nil && args.TeamName != nil {
		return nil, errors.New("team must be identified by either id (team parameter) or name (teamName parameter), both specified")
	}

	var team *types.Team
	if args.Team != nil {
		var id int32
		err := relay.UnmarshalSpec(*args.Team, id)
		if err != nil {
			return nil, err
		}
		team, err = r.db.Teams().GetTeamByID(ctx, id)
		if err != nil {
			return nil, err
		}
	} else if args.TeamName != nil {
		var err error
		team, err = r.db.Teams().GetTeamByName(ctx, *args.TeamName)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("must specify team name or team id")
	}

	users, _, err := usersForTeamMembers(ctx, r.db, args.Members)
	if err != nil {
		return nil, err
	}
	ms := make([]*types.TeamMember, 0, len(users))
	for _, u := range users {
		ms = append(ms, &types.TeamMember{
			UserID: u.ID,
			TeamID: team.ID,
		})
	}
	if err := r.db.Teams().CreateTeamMember(ctx, ms...); err != nil {
		return nil, err
	}

	return &TeamResolver{
		db:   r.db,
		team: team,
	}, nil
}

func (r *schemaResolver) SetTeamMembers(args *TeamMembersArgs) *TeamResolver {
	return &TeamResolver{}
}

func (r *schemaResolver) RemoveTeamMembers(args *TeamMembersArgs) *TeamResolver {
	return &TeamResolver{}
}

func (r *schemaResolver) Teams(ctx context.Context, args *ListTeamsArgs) (*teamConnectionResolver, error) {
	// 🚨 SECURITY: For now we only allow site admins to use teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can view teams")
	}
	c := &teamConnectionResolver{db: r.db}
	if err := c.applyArgs(args); err != nil {
		return nil, err
	}
	return c, nil
}

type TeamArgs struct {
	Name string
}

func (r *schemaResolver) Team(ctx context.Context, args *TeamArgs) (*TeamResolver, error) {
	// 🚨 SECURITY: For now we only allow site admins to use teams.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, errors.New("only site admins can view teams")
	}

	t, err := r.db.Teams().GetTeamByName(ctx, args.Name)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &TeamResolver{db: r.db, team: t}, nil
}

// usersForTeamMembers returns the matching users for the given slice of TeamMemberInput.
// For each input, we look at ID, Username, Email, and then External Account in this precedence
// order. If one field is specified, it is used. If not found, under that predicate, the
// next one is tried. If the record doesn't match a user entirely, it is skipped. (As opposed
// to an error being returned. This is more convenient for ingestion as it allows us to
// skip over users for now.) We might want to revisit this later.
func usersForTeamMembers(ctx context.Context, db database.DB, members []TeamMemberInput) (users []*types.User, noMatch []TeamMemberInput, err error) {
	// First, look at IDs.
	ids := []int32{}
	members = filterMembers(members, func(m TeamMemberInput) (drop bool) {
		// If ID is specified for the member, we try to find the user by ID.
		if m.ID == nil {
			return false
		}
		id, err := UnmarshalUserID(*m.ID)
		if err != nil {
			// Invalid ID, continue with next best option.
			return false
		}
		ids = append(ids, id)
		return true
	})
	if len(ids) > 0 {
		users, err = db.Users().List(ctx, &database.UsersListOptions{UserIDs: ids})
		if err != nil {
			return nil, nil, err
		}
	}

	// Now, look at all that have username set.
	usernames := []string{}
	members = filterMembers(members, func(m TeamMemberInput) (drop bool) {
		if m.Username == nil {
			return false
		}
		usernames = append(usernames, *m.Username)
		return true
	})
	if len(usernames) > 0 {
		us, err := db.Users().List(ctx, &database.UsersListOptions{Usernames: usernames})
		if err != nil {
			return nil, nil, err
		}
		users = append(users, us...)
	}

	// Next up: Email.
	members = filterMembers(members, func(m TeamMemberInput) (drop bool) {
		if m.Email == nil {
			return false
		}
		user, err := db.Users().GetByVerifiedEmail(ctx, *m.Email)
		if err != nil {
			return false
		}
		users = append(users, user)
		return true
	})

	// Next up: ExternalAccount.
	members = filterMembers(members, func(m TeamMemberInput) (drop bool) {
		if m.ExternalAccountServiceID == nil || m.ExternalAccountServiceType == nil {
			return false
		}

		eas, err := db.UserExternalAccounts().List(ctx, database.ExternalAccountsListOptions{
			ServiceType: *m.ExternalAccountServiceType,
			ServiceID:   *m.ExternalAccountServiceID,
		})
		if err != nil {
			return false
		}
		for _, ea := range eas {
			if m.ExternalAccountAccountID != nil {
				if ea.AccountID == *m.ExternalAccountAccountID {
					u, err := db.Users().GetByID(ctx, ea.UserID)
					if err != nil {
						return false
					}
					users = append(users, u)
					return true
				}
				continue
			}
			if m.ExternalAccountLogin != nil {
				if ea.PublicAccountData.Login == nil {
					continue
				}
				if *ea.PublicAccountData.Login == *m.ExternalAccountAccountID {
					u, err := db.Users().GetByID(ctx, ea.UserID)
					if err != nil {
						return false
					}
					users = append(users, u)
					return true
				}
				continue
			}
		}
		return false
	})

	return users, members, nil
}

func filterMembers(members []TeamMemberInput, pred func(member TeamMemberInput) (drop bool)) []TeamMemberInput {
	remaining := []TeamMemberInput{}
	for _, member := range members {
		if !pred(member) {
			remaining = append(remaining, member)
		}
	}
	return remaining
}
