package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type OrgMemberStore interface {
	basestore.ShareableStore
	With(basestore.ShareableStore) OrgMemberStore
	AutocompleteMembersSearch(ctx context.Context, OrgID int32, query string) ([]*types.OrgMemberAutocompleteSearchItem, error)
	WithTransact(context.Context, func(OrgMemberStore) error) error
	Create(ctx context.Context, orgID, userID int32) (*types.OrgMembership, error)
	GetByUserID(ctx context.Context, userID int32) ([]*types.OrgMembership, error)
	GetByOrgIDAndUserID(ctx context.Context, orgID, userID int32) (*types.OrgMembership, error)
	MemberCount(ctx context.Context, orgID int32) (int, error)
	Remove(ctx context.Context, orgID, userID int32) error
	GetByOrgID(ctx context.Context, orgID int32) ([]*types.OrgMembership, error)
	CreateMembershipInOrgsForAllUsers(ctx context.Context, orgNames []string) error
}

type orgMemberStore struct {
	*basestore.Store
}

// OrgMembersWith instantiates and returns a new OrgMemberStore using the other store handle.
func OrgMembersWith(other basestore.ShareableStore) OrgMemberStore {
	return &orgMemberStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *orgMemberStore) With(other basestore.ShareableStore) OrgMemberStore {
	return &orgMemberStore{Store: s.Store.With(other)}
}

func (m *orgMemberStore) WithTransact(ctx context.Context, f func(OrgMemberStore) error) error {
	return m.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&orgMemberStore{Store: tx})
	})
}

func (m *orgMemberStore) Create(ctx context.Context, orgID, userID int32) (*types.OrgMembership, error) {
	om := types.OrgMembership{
		OrgID:  orgID,
		UserID: userID,
	}
	err := m.Handle().QueryRowContext(
		ctx,
		"INSERT INTO org_members(org_id, user_id) VALUES($1, $2) RETURNING id, created_at, updated_at",
		om.OrgID, om.UserID).Scan(&om.ID, &om.CreatedAt, &om.UpdatedAt)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.ConstraintName == "org_members_org_id_user_id_key" {
			return nil, errors.New("user is already a member of the organization")
		}
		return nil, err
	}
	return &om, nil
}

func (m *orgMemberStore) GetByUserID(ctx context.Context, userID int32) ([]*types.OrgMembership, error) {
	return m.getBySQL(ctx, "INNER JOIN users ON org_members.user_id=users.id WHERE org_members.user_id=$1 AND users.deleted_at IS NULL", userID)
}

func (m *orgMemberStore) GetByOrgIDAndUserID(ctx context.Context, orgID, userID int32) (*types.OrgMembership, error) {
	return m.getOneBySQL(ctx, "INNER JOIN users ON org_members.user_id=users.id WHERE org_id=$1 AND user_id=$2 AND users.deleted_at IS NULL LIMIT 1", orgID, userID)
}

func (m *orgMemberStore) MemberCount(ctx context.Context, orgID int32) (int, error) {
	var memberCount int
	err := m.Handle().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM org_members INNER JOIN users ON org_members.user_id = users.id
		WHERE org_id=$1 AND users.deleted_at IS NULL`, orgID).Scan(&memberCount)
	if err != nil {
		return 0, err
	}
	return memberCount, nil
}

func (m *orgMemberStore) Remove(ctx context.Context, orgID, userID int32) error {
	_, err := m.Handle().ExecContext(ctx, "DELETE FROM org_members WHERE (org_id=$1 AND user_id=$2)", orgID, userID)
	return err
}

// GetByOrgID returns a list of all members of a given organization.
func (m *orgMemberStore) GetByOrgID(ctx context.Context, orgID int32) ([]*types.OrgMembership, error) {
	org, err := OrgsWith(m).GetByID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return m.getBySQL(ctx, "INNER JOIN users ON org_members.user_id = users.id WHERE org_id=$1 AND users.deleted_at IS NULL ORDER BY upper(users.display_name), users.id", org.ID)
}

func (u *orgMemberStore) AutocompleteMembersSearch(ctx context.Context, orgID int32, query string) ([]*types.OrgMemberAutocompleteSearchItem, error) {
	pattern := query + "%"
	q := sqlf.Sprintf(`SELECT u.id, u.username, u.display_name, u.avatar_url, (SELECT COUNT(o.org_id) from org_members o WHERE o.org_id = %d AND o.user_id = u.id) as inorg
		FROM users u WHERE (u.username ILIKE %s OR u.display_name ILIKE %s) AND u.deleted_at IS NULL ORDER BY id ASC LIMIT 10`, orgID, pattern, pattern)

	rows, err := u.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	users := []*types.OrgMemberAutocompleteSearchItem{}
	defer rows.Close()
	for rows.Next() {
		var u types.OrgMemberAutocompleteSearchItem
		var displayName, avatarURL sql.NullString
		err := rows.Scan(&u.ID, &u.Username, &displayName, &avatarURL, &u.InOrg)
		if err != nil {
			return nil, err
		}
		u.DisplayName = displayName.String
		u.AvatarURL = avatarURL.String
		users = append(users, &u)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// ErrOrgMemberNotFound is the error that is returned when
// a user is not in an org.
type ErrOrgMemberNotFound struct {
	args []any
}

func (err *ErrOrgMemberNotFound) Error() string {
	return fmt.Sprintf("org member not found: %v", err.args)
}

func (ErrOrgMemberNotFound) NotFound() bool { return true }

func (m *orgMemberStore) getOneBySQL(ctx context.Context, query string, args ...any) (*types.OrgMembership, error) {
	members, err := m.getBySQL(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	if len(members) != 1 {
		return nil, &ErrOrgMemberNotFound{args}
	}
	return members[0], nil
}

func (m *orgMemberStore) getBySQL(ctx context.Context, query string, args ...any) ([]*types.OrgMembership, error) {
	rows, err := m.Handle().QueryContext(ctx, "SELECT org_members.id, org_members.org_id, org_members.user_id, org_members.created_at, org_members.updated_at FROM org_members "+query, args...)
	if err != nil {
		return nil, err
	}

	members := []*types.OrgMembership{}
	defer rows.Close()
	for rows.Next() {
		m := types.OrgMembership{}
		err := rows.Scan(&m.ID, &m.OrgID, &m.UserID, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

// CreateMembershipInOrgsForAllUsers causes *ALL* users to become members of every org in the
// orgNames list.
func (m *orgMemberStore) CreateMembershipInOrgsForAllUsers(ctx context.Context, orgNames []string) error {
	if len(orgNames) == 0 {
		return nil
	}

	orgNameVars := []*sqlf.Query{}
	for _, orgName := range orgNames {
		orgNameVars = append(orgNameVars, sqlf.Sprintf("%s", orgName))
	}

	sqlQuery := sqlf.Sprintf(`
			WITH org_ids AS (SELECT id FROM orgs WHERE name IN (%s)),
				 user_ids AS (SELECT id FROM users WHERE deleted_at IS NULL),
				 to_join AS (SELECT org_ids.id AS org_id, user_ids.id AS user_id
						  FROM org_ids join user_ids ON true
						  LEFT JOIN org_members ON org_members.org_id=org_ids.id AND
									org_members.user_id=user_ids.id
						  WHERE org_members.id is null)
			INSERT INTO org_members(org_id,user_id) SELECT to_join.org_id, to_join.user_id FROM to_join;`,
		sqlf.Join(orgNameVars, ","))

	err := m.Exec(ctx, sqlQuery)
	return err
}
