package fakedb

import (
	"context"
	"sort"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Teams is a true fake implementing database.TeamStore interface.
// The behavior is expected to be semantically equivalent to a Postgres
// implementation, but in memory.
type Teams struct {
	database.TeamStore
	list       []*types.Team
	members    orderedTeamMembers
	lastUsedID int32
	users      *Users
}

// ListAllTeams returns all stsored teams. It is meant to be used
// for white-box testing, where we want to erify database contents.
func (fs Fakes) ListAllTeams() []*types.Team {
	return append([]*types.Team{}, fs.TeamStore.list...)
}

// AddTeamMember is a test setup tool for adding membership to fake Teams
// in-memory storage.
func (fs Fakes) AddTeamMember(moreMembers ...*types.TeamMember) {
	fs.TeamStore.members = append(fs.TeamStore.members, moreMembers...)
}

func (fs Fakes) AddTeam(t *types.Team) int32 {
	u := *t
	fs.TeamStore.addTeam(&u)
	return u.ID
}

func (teams *Teams) CreateTeam(_ context.Context, t *types.Team) (*types.Team, error) {
	u := *t
	teams.addTeam(&u)
	return &u, nil
}

func (teams *Teams) addTeam(t *types.Team) {
	teams.lastUsedID++
	t.ID = teams.lastUsedID
	teams.list = append(teams.list, t)
}

func (teams *Teams) UpdateTeam(_ context.Context, t *types.Team) error {
	if t == nil {
		return errors.New("UpdateTeam: team cannot be nil")
	}
	if t.ID == 0 {
		return errors.New("UpdateTeam: team.ID must be set (not 0)")
	}
	for _, u := range teams.list {
		if u.ID == t.ID {
			*u = *t
			return nil
		}
	}
	return errors.Newf("UpdateTeam: cannot find team with ID=%d", t.ID)
}

func (teams *Teams) GetTeamByID(_ context.Context, id int32) (*types.Team, error) {
	for _, t := range teams.list {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, database.TeamNotFoundError{}
}

func (teams *Teams) GetTeamByName(_ context.Context, name string) (*types.Team, error) {
	for _, t := range teams.list {
		if t.Name == name {
			return t, nil
		}
	}
	return nil, database.TeamNotFoundError{}
}

func (teams *Teams) DeleteTeam(_ context.Context, id int32) error {
	for i, t := range teams.list {
		if t.ID == id {
			maxI := len(teams.list) - 1
			teams.list[i], teams.list[maxI] = teams.list[maxI], teams.list[i]
			teams.list = teams.list[:maxI]
			return nil
		}
	}
	return database.TeamNotFoundError{}
}

func (teams *Teams) ListTeams(_ context.Context, opts database.ListTeamsOpts) (selected []*types.Team, next int32, err error) {
	for _, t := range teams.list {
		if teams.matches(t, opts) {
			selected = append(selected, t)
		}
	}
	if opts.LimitOffset != nil {
		selected = selected[opts.LimitOffset.Offset:]
		if limit := opts.LimitOffset.Limit; limit != 0 && len(selected) > limit {
			next = selected[opts.LimitOffset.Limit].ID
			selected = selected[:opts.LimitOffset.Limit]
		}
	}
	return selected, next, nil
}

func (teams *Teams) CountTeams(ctx context.Context, opts database.ListTeamsOpts) (int32, error) {
	selected, _, err := teams.ListTeams(ctx, opts)
	return int32(len(selected)), err
}

func (teams *Teams) ContainsTeam(ctx context.Context, teamID int32, opts database.ListTeamsOpts) (bool, error) {
	selected, _, err := teams.ListTeams(ctx, opts)
	if err != nil {
		return false, err
	}
	for _, t := range selected {
		if t.ID == teamID {
			return true, nil
		}
	}
	return false, nil
}

func (teams *Teams) matches(team *types.Team, opts database.ListTeamsOpts) bool {
	if opts.Cursor != 0 && team.ID < opts.Cursor {
		return false
	}
	if opts.WithParentID != 0 && team.ParentTeamID != opts.WithParentID {
		return false
	}
	if opts.RootOnly && team.ParentTeamID != 0 {
		return false
	}
	if opts.Search != "" {
		search := strings.ToLower(opts.Search)
		name := strings.ToLower(team.Name)
		displayName := strings.ToLower(team.DisplayName)
		if !strings.Contains(name, search) && !strings.Contains(displayName, search) {
			return false
		}
	}
	if opts.ExceptAncestorID != 0 {
		for _, id := range teams.ancestors(team.ID) {
			if opts.ExceptAncestorID == id {
				return false
			}
		}
	}
	// opts.ForUserMember is not supported yet as there is no membership fake.
	return true
}

func (teams *Teams) ancestors(id int32) []int32 {
	var ids []int32
	parentID := id
	for parentID != 0 {
		ids = append(ids, parentID)
		for _, t := range teams.list {
			if t.ID == parentID {
				parentID = t.ParentTeamID
			}
		}
	}
	return ids
}

type orderedTeamMembers []*types.TeamMember

func (o orderedTeamMembers) Len() int { return len(o) }
func (o orderedTeamMembers) Less(i, j int) bool {
	if o[i].TeamID < o[j].TeamID {
		return true
	}
	if o[i].TeamID == o[j].TeamID {
		return o[i].UserID < o[j].UserID
	}
	return false
}
func (o orderedTeamMembers) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (teams *Teams) CountTeamMembers(ctx context.Context, opts database.ListTeamMembersOpts) (int32, error) {
	ms, _, err := teams.ListTeamMembers(ctx, opts)
	return int32(len(ms)), err
}

func (teams *Teams) ListTeamMembers(ctx context.Context, opts database.ListTeamMembersOpts) (selected []*types.TeamMember, next *database.TeamMemberListCursor, err error) {
	sort.Sort(teams.members)
	for _, m := range teams.members {
		if opts.Cursor.TeamID > m.TeamID {
			continue
		}
		if opts.Cursor.TeamID == m.TeamID && opts.Cursor.UserID > m.UserID {
			continue
		}
		if opts.TeamID != 0 && opts.TeamID != m.TeamID {
			continue
		}
		if opts.Search != "" {
			if teams.users == nil {
				return nil, nil, errors.New("fakeTeamsDB needs reference to fakeUsersDB for ListTeamMembersOpts.Search")
			}
			u, err := teams.users.GetByID(ctx, m.UserID)
			if err != nil {
				return nil, nil, err
			}
			if u == nil {
				continue
			}
			search := strings.ToLower(opts.Search)
			username := strings.ToLower(u.Username)
			displayName := strings.ToLower(u.DisplayName)
			if !strings.Contains(username, search) && !strings.Contains(displayName, search) {
				continue
			}
		}
		selected = append(selected, m)
	}
	if opts.LimitOffset != nil {
		selected = selected[opts.LimitOffset.Offset:]
		if limit := opts.LimitOffset.Limit; limit != 0 && len(selected) > limit {
			next = &database.TeamMemberListCursor{
				TeamID: selected[opts.LimitOffset.Limit].TeamID,
				UserID: selected[opts.LimitOffset.Limit].UserID,
			}
			selected = selected[:opts.LimitOffset.Limit]
		}
	}
	return selected, next, nil
}

func (teams *Teams) CreateTeamMember(_ context.Context, members ...*types.TeamMember) error {
	for _, newMember := range members {
		exists := false
		for _, existingMember := range teams.members {
			if existingMember.UserID == newMember.UserID && existingMember.TeamID == newMember.TeamID {
				exists = true
				// on conflict do nothing.
				break
			}
		}
		if !exists {
			teams.members = append(teams.members, newMember)
		}
	}

	return nil
}

func (teams *Teams) DeleteTeamMember(_ context.Context, members ...*types.TeamMember) error {
	for _, m := range members {
		var index int
		var found bool
		for i := range len(teams.members) {
			if n := teams.members[i]; m.UserID == n.UserID && m.TeamID == n.TeamID {
				found = true
				index = i
			}
		}
		if found {
			maxI := len(teams.members) - 1
			teams.members[index], teams.members[maxI] = teams.members[maxI], teams.members[index]
			teams.members = teams.members[:maxI]
		}
	}
	return nil
}

func (teams *Teams) IsTeamMember(_ context.Context, teamID, userID int32) (bool, error) {
	for _, m := range teams.members {
		if m.TeamID == teamID && m.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}
