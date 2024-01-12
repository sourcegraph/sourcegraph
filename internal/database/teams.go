package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type ListTeamsOpts struct {
	*LimitOffset

	// Only return teams past this cursor.
	Cursor int32
	// List teams of a specific parent team only.
	WithParentID int32
	// List teams that do not have given team as an ancestor in parent relationship.
	ExceptAncestorID int32
	// Only return root teams (teams that have no parent).
	// This is used on the main overview list of teams.
	RootOnly bool
	// Filter teams by search term. Currently, name and displayName are searchable.
	Search string
	// List teams that a specific user is a member of.
	ForUserMember int32
}

func (opts ListTeamsOpts) SQL() (where, joins, ctes []*sqlf.Query) {
	where = []*sqlf.Query{
		sqlf.Sprintf("teams.id >= %s", opts.Cursor),
	}
	joins = []*sqlf.Query{}

	if opts.WithParentID != 0 {
		where = append(where, sqlf.Sprintf("teams.parent_team_id = %s", opts.WithParentID))
	}
	if opts.RootOnly {
		where = append(where, sqlf.Sprintf("teams.parent_team_id IS NULL"))
	}
	if opts.Search != "" {
		term := "%" + opts.Search + "%"
		where = append(where, sqlf.Sprintf("(teams.name ILIKE %s OR teams.display_name ILIKE %s)", term, term))
	}
	if opts.ForUserMember != 0 {
		joins = append(joins, sqlf.Sprintf("JOIN team_members ON team_members.team_id = teams.id"))
		where = append(where, sqlf.Sprintf("team_members.user_id = %s", opts.ForUserMember))
	}
	if opts.ExceptAncestorID != 0 {
		joins = append(joins, sqlf.Sprintf("LEFT JOIN descendants ON descendants.team_id = teams.id"))
		where = append(where, sqlf.Sprintf("descendants.team_id IS NULL"))
		ctes = append(ctes, sqlf.Sprintf(
			`WITH RECURSIVE descendants AS (
				SELECT id AS team_id
				FROM teams
				WHERE id = %s
			UNION ALL
				SELECT t.id AS team_id
				FROM teams t
				INNER JOIN descendants d ON t.parent_team_id = d.team_id
			)`, opts.ExceptAncestorID))
	}

	return where, joins, ctes
}

type TeamMemberListCursor struct {
	TeamID int32
	UserID int32
}

type ListTeamMembersOpts struct {
	*LimitOffset

	// Only return members past this cursor.
	Cursor TeamMemberListCursor
	// Required. Scopes the list operation to the given team.
	TeamID int32
	// Filter members by search term. Currently, name and displayName of the users
	// are searchable.
	Search string
}

func (opts ListTeamMembersOpts) SQL() (where, joins []*sqlf.Query) {
	where = []*sqlf.Query{
		sqlf.Sprintf("team_members.team_id >= %s AND team_members.user_id >= %s", opts.Cursor.TeamID, opts.Cursor.UserID),
	}
	joins = []*sqlf.Query{}

	if opts.TeamID != 0 {
		where = append(where, sqlf.Sprintf("team_members.team_id = %s", opts.TeamID))
	}
	if opts.Search != "" {
		joins = append(joins, sqlf.Sprintf("JOIN users ON users.id = team_members.user_id"))
		term := "%" + opts.Search + "%"
		where = append(where, sqlf.Sprintf("(users.username ILIKE %s OR users.display_name ILIKE %s)", term, term))
	}

	return where, joins
}

// TeamNotFoundError is returned when a team cannot be found.
type TeamNotFoundError struct {
	args any
}

func (err TeamNotFoundError) Error() string {
	return fmt.Sprintf("team not found: %v", err.args)
}

func (TeamNotFoundError) NotFound() bool {
	return true
}

// ErrTeamNameAlreadyExists is returned when the team name is already in use, either
// by another team or another user/org.
var ErrTeamNameAlreadyExists = errors.New("team name is already taken (by a user, organization, or another team)")

// TeamStore provides database methods for interacting with teams and their members.
type TeamStore interface {
	basestore.ShareableStore
	Done(error) error

	// GetTeamByID returns the given team by ID. If not found, a NotFounder error is returned.
	GetTeamByID(ctx context.Context, id int32) (*types.Team, error)
	// GetTeamByName returns the given team by name. If not found, a NotFounder error is returned.
	GetTeamByName(ctx context.Context, name string) (*types.Team, error)
	// ListTeams lists teams given the options. The matching teams, plus the next cursor are
	// returned.
	ListTeams(ctx context.Context, opts ListTeamsOpts) ([]*types.Team, int32, error)
	// CountTeams counts teams given the options.
	CountTeams(ctx context.Context, opts ListTeamsOpts) (int32, error)
	// ContainsTeam tells whether given search conditions contain team with given ID.
	ContainsTeam(ctx context.Context, id int32, opts ListTeamsOpts) (bool, error)
	// ListTeamMembers lists team members given the options. The matching teams,
	// plus the next cursor are returned.
	ListTeamMembers(ctx context.Context, opts ListTeamMembersOpts) ([]*types.TeamMember, *TeamMemberListCursor, error)
	// CountTeamMembers counts teams given the options.
	CountTeamMembers(ctx context.Context, opts ListTeamMembersOpts) (int32, error)
	// CreateTeam creates the given team in the database.
	CreateTeam(ctx context.Context, team *types.Team) (*types.Team, error)
	// UpdateTeam updates the given team in the database.
	UpdateTeam(ctx context.Context, team *types.Team) error
	// DeleteTeam deletes the given team from the database.
	DeleteTeam(ctx context.Context, team int32) error
	// CreateTeamMember creates the team members in the database. If any of the inserts fail,
	// all inserts are reverted.
	CreateTeamMember(ctx context.Context, members ...*types.TeamMember) error
	// DeleteTeamMember deletes the given team members from the database.
	DeleteTeamMember(ctx context.Context, members ...*types.TeamMember) error
	// IsTeamMember checks if the given user is a member of the given team.
	IsTeamMember(ctx context.Context, teamID, userID int32) (bool, error)
}

type teamStore struct {
	*basestore.Store
}

// TeamsWith instantiates and returns a new TeamStore using the other store handle.
func TeamsWith(other basestore.ShareableStore) TeamStore {
	return &teamStore{
		Store: basestore.NewWithHandle(other.Handle()),
	}
}

func (s *teamStore) With(other basestore.ShareableStore) TeamStore {
	return &teamStore{
		Store: s.Store.With(other),
	}
}

func (s *teamStore) WithTransact(ctx context.Context, f func(TeamStore) error) error {
	return s.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&teamStore{
			Store: tx,
		})
	})
}

func (s *teamStore) GetTeamByID(ctx context.Context, id int32) (*types.Team, error) {
	conds := []*sqlf.Query{
		sqlf.Sprintf("teams.id = %s", id),
	}
	return s.getTeam(ctx, conds)
}

func (s *teamStore) GetTeamByName(ctx context.Context, name string) (*types.Team, error) {
	conds := []*sqlf.Query{
		sqlf.Sprintf("teams.name = %s", name),
	}
	return s.getTeam(ctx, conds)
}

func (s *teamStore) getTeam(ctx context.Context, conds []*sqlf.Query) (*types.Team, error) {
	q := sqlf.Sprintf(getTeamQueryFmtstr, sqlf.Join(teamColumns, ","), sqlf.Join(conds, "AND"))

	teams, err := scanTeams(s.Query(ctx, q))
	if err != nil {
		return nil, err
	}

	if len(teams) != 1 {
		return nil, TeamNotFoundError{args: conds}
	}

	return teams[0], nil
}

const getTeamQueryFmtstr = `
SELECT %s
FROM teams
WHERE
	%s
LIMIT 1
`

func (s *teamStore) ListTeams(ctx context.Context, opts ListTeamsOpts) (_ []*types.Team, next int32, err error) {
	conds, joins, ctes := opts.SQL()

	if opts.LimitOffset != nil && opts.Limit > 0 {
		opts.Limit++
	}

	q := sqlf.Sprintf(
		listTeamsQueryFmtstr,
		sqlf.Join(ctes, "\n"),
		sqlf.Join(teamColumns, ","),
		sqlf.Join(joins, "\n"),
		sqlf.Join(conds, "AND"),
		opts.LimitOffset.SQL(),
	)

	teams, err := scanTeams(s.Query(ctx, q))
	if err != nil {
		return nil, 0, err
	}

	if opts.LimitOffset != nil && opts.Limit > 0 && len(teams) == opts.Limit {
		next = teams[len(teams)-1].ID
		teams = teams[:len(teams)-1]
	}

	return teams, next, nil
}

const listTeamsQueryFmtstr = `
%s
SELECT %s
FROM teams
%s
WHERE %s
ORDER BY
	teams.id ASC
%s
`

func (s *teamStore) CountTeams(ctx context.Context, opts ListTeamsOpts) (int32, error) {
	// Disable cursor for counting.
	opts.Cursor = 0
	conds, joins, ctes := opts.SQL()

	q := sqlf.Sprintf(
		countTeamsQueryFmtstr,
		sqlf.Join(ctes, "\n"),
		sqlf.Join(joins, "\n"),
		sqlf.Join(conds, "AND"),
	)

	count, _, err := basestore.ScanFirstInt(s.Query(ctx, q))
	return int32(count), err
}

const countTeamsQueryFmtstr = `
%s
SELECT COUNT(*)
FROM teams
%s
WHERE %s
`

func (s *teamStore) ContainsTeam(ctx context.Context, id int32, opts ListTeamsOpts) (bool, error) {
	// Disable cursor for containment.
	opts.Cursor = 0
	conds, joins, ctes := opts.SQL()
	q := sqlf.Sprintf(
		containsTeamsQueryFmtstr,
		sqlf.Join(ctes, "\n"),
		sqlf.Join(joins, "\n"),
		id,
		sqlf.Join(conds, "AND"),
	)
	ids, err := basestore.ScanInts(s.Query(ctx, q))
	if err != nil {
		return false, err
	}
	return len(ids) > 0, nil
}

const containsTeamsQueryFmtstr = `
%s
SELECT 1
FROM teams
%s
WHERE teams.id = %s
AND %s
LIMIT 1
`

func (s *teamStore) ListTeamMembers(ctx context.Context, opts ListTeamMembersOpts) (_ []*types.TeamMember, next *TeamMemberListCursor, err error) {
	conds, joins := opts.SQL()

	if opts.LimitOffset != nil && opts.Limit > 0 {
		opts.Limit++
	}

	if opts.Search == "" {
		joins = append(joins, sqlf.Sprintf("LEFT JOIN users ON team_members.user_id = users.id"))
	}
	conds = append(conds, sqlf.Sprintf("users.deleted_at IS NULL"))

	q := sqlf.Sprintf(
		listTeamMembersQueryFmtstr,
		sqlf.Join(teamMemberColumns, ","),
		sqlf.Join(joins, "\n"),
		sqlf.Join(conds, "AND"),
		opts.LimitOffset.SQL(),
	)

	tms, err := scanTeamMembers(s.Query(ctx, q))
	if err != nil {
		return nil, nil, err
	}

	if opts.LimitOffset != nil && opts.Limit > 0 && len(tms) == opts.Limit {
		next = &TeamMemberListCursor{
			TeamID: tms[len(tms)-1].TeamID,
			UserID: tms[len(tms)-1].UserID,
		}
		tms = tms[:len(tms)-1]
	}

	return tms, next, nil
}

const listTeamMembersQueryFmtstr = `
SELECT %s
FROM team_members
%s
WHERE %s
ORDER BY
	team_members.team_id ASC, team_members.user_id ASC
%s
`

func (s *teamStore) CountTeamMembers(ctx context.Context, opts ListTeamMembersOpts) (int32, error) {
	// Disable cursor for counting.
	opts.Cursor = TeamMemberListCursor{}
	conds, joins := opts.SQL()

	if opts.Search == "" {
		joins = append(joins, sqlf.Sprintf("LEFT JOIN users ON team_members.user_id = users.id"))
	}
	conds = append(conds, sqlf.Sprintf("users.deleted_at IS NULL"))

	q := sqlf.Sprintf(
		countTeamMembersQueryFmtstr,
		sqlf.Join(joins, "\n"),
		sqlf.Join(conds, "AND"),
	)

	count, _, err := basestore.ScanFirstInt(s.Query(ctx, q))
	return int32(count), err
}

const countTeamMembersQueryFmtstr = `
SELECT COUNT(1)
FROM team_members
%s
WHERE %s
`

func (s *teamStore) CreateTeam(ctx context.Context, team *types.Team) (*types.Team, error) {
	tx, err := s.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	if team.CreatedAt.IsZero() {
		team.CreatedAt = timeutil.Now()
	}

	if team.UpdatedAt.IsZero() {
		team.UpdatedAt = team.CreatedAt
	}

	q := sqlf.Sprintf(
		createTeamQueryFmtstr,
		sqlf.Join(teamInsertColumns, ","),
		team.Name,
		dbutil.NewNullString(team.DisplayName),
		team.ReadOnly,
		dbutil.NewNullInt32(team.ParentTeamID),
		dbutil.NewNullInt32(team.CreatorID),
		team.CreatedAt,
		team.UpdatedAt,
		sqlf.Join(teamColumns, ","),
	)

	row := tx.Handle().QueryRowContext(
		ctx,
		q.Query(sqlf.PostgresBindVar),
		q.Args()...,
	)
	if err := row.Err(); err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) {
			switch e.ConstraintName {
			case "teams_name":
				return nil, ErrTeamNameAlreadyExists
			case "orgs_name_max_length", "orgs_name_valid_chars":
				return nil, errors.Errorf("team name invalid: %s", e.ConstraintName)
			case "orgs_display_name_max_length":
				return nil, errors.Errorf("team display name invalid: %s", e.ConstraintName)
			}
		}

		return nil, err
	}

	if err := scanTeam(row, team); err != nil {
		return nil, err
	}

	q = sqlf.Sprintf(createTeamNameReservationQueryFmtstr, team.Name, team.ID)

	// Reserve team name in shared users+orgs+teams namespace.
	if _, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...); err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) {
			switch e.ConstraintName {
			case "names_pkey":
				return nil, ErrTeamNameAlreadyExists
			}
		}
		return nil, err
	}

	return team, nil
}

const createTeamQueryFmtstr = `
INSERT INTO teams
(%s)
VALUES (%s, %s, %s, %s, %s, %s, %s)
RETURNING %s
`

const createTeamNameReservationQueryFmtstr = `
INSERT INTO names
	(name, team_id)
VALUES
	(%s, %s)
`

func (s *teamStore) UpdateTeam(ctx context.Context, team *types.Team) error {
	team.UpdatedAt = timeutil.Now()

	conds := []*sqlf.Query{
		sqlf.Sprintf("id = %s", team.ID),
	}

	q := sqlf.Sprintf(
		updateTeamQueryFmtstr,
		dbutil.NewNullString(team.DisplayName),
		dbutil.NewNullInt32(team.ParentTeamID),
		team.UpdatedAt,
		sqlf.Join(conds, "AND"),
		sqlf.Join(teamColumns, ","),
	)

	return scanTeam(s.QueryRow(ctx, q), team)
}

const updateTeamQueryFmtstr = `
UPDATE
	teams
SET
	display_name = %s,
	parent_team_id = %s,
	updated_at = %s
WHERE
	%s
RETURNING
	%s
`

func (s *teamStore) DeleteTeam(ctx context.Context, team int32) (err error) {
	return s.WithTransact(ctx, func(tx TeamStore) error {
		conds := []*sqlf.Query{
			sqlf.Sprintf("teams.id = %s", team),
		}

		q := sqlf.Sprintf(deleteTeamQueryFmtstr, sqlf.Join(conds, "AND"))

		res, err := tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return TeamNotFoundError{args: conds}
		}

		conds = []*sqlf.Query{
			sqlf.Sprintf("names.team_id = %s", team),
		}

		q = sqlf.Sprintf(deleteTeamNameReservationQueryFmtstr, sqlf.Join(conds, "AND"))

		// Release the teams name so it can be used by another user, team or org.
		_, err = tx.Handle().ExecContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
		return err
	})
}

const deleteTeamQueryFmtstr = `
DELETE FROM
	teams
WHERE %s
`

const deleteTeamNameReservationQueryFmtstr = `
DELETE FROM
	names
WHERE %s
`

func (s *teamStore) CreateTeamMember(ctx context.Context, members ...*types.TeamMember) error {
	inserter := func(inserter *batch.Inserter) error {
		for _, m := range members {
			if m.CreatedAt.IsZero() {
				m.CreatedAt = timeutil.Now()
			}

			if m.UpdatedAt.IsZero() {
				m.UpdatedAt = m.CreatedAt
			}

			if err := inserter.Insert(
				ctx,
				m.TeamID,
				m.UserID,
				m.CreatedAt,
				m.UpdatedAt,
			); err != nil {
				return err
			}
		}
		return nil
	}

	i := -1
	return batch.WithInserterWithReturn(
		ctx,
		s.Handle(),
		"team_members",
		batch.MaxNumPostgresParameters,
		teamMemberInsertColumns,
		"ON CONFLICT DO NOTHING",
		teamMemberStringColumns,
		func(sc dbutil.Scanner) error {
			i++
			return scanTeamMember(sc, members[i])
		},
		inserter,
	)
}

func (s *teamStore) DeleteTeamMember(ctx context.Context, members ...*types.TeamMember) error {
	ms := []*sqlf.Query{}
	for _, m := range members {
		ms = append(ms, sqlf.Sprintf("(%s, %s)", m.TeamID, m.UserID))
	}
	conds := []*sqlf.Query{
		sqlf.Sprintf("(team_id, user_id) IN (%s)", sqlf.Join(ms, ",")),
	}

	q := sqlf.Sprintf(deleteTeamMemberQueryFmtstr, sqlf.Join(conds, "AND"))
	return s.Exec(ctx, q)
}

const deleteTeamMemberQueryFmtstr = `
DELETE FROM
	team_members
WHERE %s
`

func (s *teamStore) IsTeamMember(ctx context.Context, teamID, userID int32) (bool, error) {
	conds := []*sqlf.Query{
		sqlf.Sprintf("team_id = %s", teamID),
		sqlf.Sprintf("user_id = %s", userID),
	}

	q := sqlf.Sprintf(isTeamMemberQueryFmtstr, sqlf.Join(conds, "AND"))
	ok, _, err := basestore.ScanFirstBool(s.Query(ctx, q))
	return ok, err
}

const isTeamMemberQueryFmtstr = `
SELECT
	COUNT(*) = 1
FROM
	team_members
WHERE %s
`

var teamColumns = []*sqlf.Query{
	sqlf.Sprintf("teams.id"),
	sqlf.Sprintf("teams.name"),
	sqlf.Sprintf("teams.display_name"),
	sqlf.Sprintf("teams.readonly"),
	sqlf.Sprintf("teams.parent_team_id"),
	sqlf.Sprintf("teams.creator_id"),
	sqlf.Sprintf("teams.created_at"),
	sqlf.Sprintf("teams.updated_at"),
}

var teamInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("name"),
	sqlf.Sprintf("display_name"),
	sqlf.Sprintf("readonly"),
	sqlf.Sprintf("parent_team_id"),
	sqlf.Sprintf("creator_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
}

var teamMemberColumns = []*sqlf.Query{
	sqlf.Sprintf("team_members.team_id"),
	sqlf.Sprintf("team_members.user_id"),
	sqlf.Sprintf("team_members.created_at"),
	sqlf.Sprintf("team_members.updated_at"),
}

var teamMemberStringColumns = []string{
	"team_members.team_id",
	"team_members.user_id",
	"team_members.created_at",
	"team_members.updated_at",
}

var teamMemberInsertColumns = []string{
	"team_id",
	"user_id",
	"created_at",
	"updated_at",
}

var scanTeams = basestore.NewSliceScanner(func(s dbutil.Scanner) (*types.Team, error) {
	var t types.Team
	err := scanTeam(s, &t)
	return &t, err
})

func scanTeam(sc dbutil.Scanner, t *types.Team) error {
	return sc.Scan(
		&t.ID,
		&t.Name,
		&dbutil.NullString{S: &t.DisplayName},
		&t.ReadOnly,
		&dbutil.NullInt32{N: &t.ParentTeamID},
		&dbutil.NullInt32{N: &t.CreatorID},
		&t.CreatedAt,
		&t.UpdatedAt,
	)
}

var scanTeamMembers = basestore.NewSliceScanner(func(s dbutil.Scanner) (*types.TeamMember, error) {
	var t types.TeamMember
	err := scanTeamMember(s, &t)
	return &t, err
})

func scanTeamMember(sc dbutil.Scanner, tm *types.TeamMember) error {
	return sc.Scan(
		&tm.TeamID,
		&tm.UserID,
		&tm.CreatedAt,
		&tm.UpdatedAt,
	)
}
