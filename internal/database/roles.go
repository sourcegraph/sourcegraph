package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// errCannotCreateRole is the error returned when a role cannot be inserted
// into the database due to a constraint.
type errCannotCreateRole struct {
	code string
}

const errorCodeRoleNameExists = "err_name_exists"

func (err errCannotCreateRole) Error() string {
	return fmt.Sprintf("cannot create role: %v", err.code)
}

func (err errCannotCreateRole) Code() string {
	return err.code
}

var roleColumns = []*sqlf.Query{
	sqlf.Sprintf("roles.id"),
	sqlf.Sprintf("roles.name"),
	sqlf.Sprintf("roles.system"),
	sqlf.Sprintf("roles.created_at"),
}

var roleInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("name"),
	sqlf.Sprintf("system"),
}

type RoleStore interface {
	basestore.ShareableStore

	// Count counts all roles in the database.
	Count(ctx context.Context, opts RolesListOptions) (int, error)
	// Create inserts the given role into the database.
	Create(ctx context.Context, name string, isSystemRole bool) (*types.Role, error)
	// Delete removes an existing role from the database.
	Delete(ctx context.Context, opts DeleteRoleOpts) error
	// Get returns the role matching the given ID or name provided. If no such role exists,
	// a RoleNotFoundErr is returned.
	Get(ctx context.Context, opts GetRoleOpts) (*types.Role, error)
	// List returns all roles matching the given options.
	List(ctx context.Context, opts RolesListOptions) ([]*types.Role, error)
	// Update updates an existing role in the database.
	Update(ctx context.Context, role *types.Role) (*types.Role, error)
}

func RolesWith(other basestore.ShareableStore) RoleStore {
	return &roleStore{Store: basestore.NewWithHandle(other.Handle())}
}

type RoleOpts struct {
	ID   int32
	Name string
}

type (
	DeleteRoleOpts RoleOpts
	GetRoleOpts    RoleOpts
)

type RolesListOptions struct {
	PaginationArgs *PaginationArgs

	System bool
	UserID int32
}

type RoleNotFoundErr struct {
	ID int32
}

func (e *RoleNotFoundErr) Error() string {
	return fmt.Sprintf("role with ID %d not found", e.ID)
}

func (e *RoleNotFoundErr) NotFound() bool {
	return true
}

type roleStore struct {
	*basestore.Store
}

var _ RoleStore = &roleStore{}

const getRoleFmtStr = `
SELECT %s FROM roles
WHERE %s
LIMIT 1;
`

func (r *roleStore) Get(ctx context.Context, opts GetRoleOpts) (*types.Role, error) {
	if opts.ID == 0 && opts.Name == "" {
		return nil, errors.New("missing id or name")
	}

	var conds []*sqlf.Query
	if opts.ID != 0 {
		conds = append(conds, sqlf.Sprintf("id = %s", opts.ID))
	}

	if opts.Name != "" {
		conds = append(conds, sqlf.Sprintf("name = %s", opts.Name))
	}

	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}

	q := sqlf.Sprintf(
		getRoleFmtStr,
		sqlf.Join(roleColumns, ", "),
		sqlf.Join(conds, " AND "),
	)

	role, err := scanRole(r.QueryRow(ctx, q))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, &RoleNotFoundErr{ID: opts.ID}
		}
		return nil, errors.Wrap(err, "scanning role")
	}

	return role, nil
}

func scanRole(sc dbutil.Scanner) (*types.Role, error) {
	var role types.Role
	if err := sc.Scan(
		&role.ID,
		&role.Name,
		&role.System,
		&role.CreatedAt,
	); err != nil {
		return nil, err
	}

	return &role, nil
}

func (r *roleStore) List(ctx context.Context, opts RolesListOptions) ([]*types.Role, error) {
	var roles []*types.Role

	scanFunc := func(rows *sql.Rows) error {
		role, err := scanRole(rows)
		if err != nil {
			return err
		}
		roles = append(roles, role)
		return nil
	}

	err := r.list(ctx, opts, sqlf.Join(roleColumns, ", "), scanFunc)
	return roles, err
}

const roleListQueryFmtstr = `
SELECT %s FROM roles
%s
WHERE %s
`

func (r *roleStore) list(ctx context.Context, opts RolesListOptions, selects *sqlf.Query, scanRole func(rows *sql.Rows) error) error {
	conds, joins := r.computeConditionsAndJoins(opts)

	queryArgs := opts.PaginationArgs.SQL()
	if queryArgs.Where != nil {
		conds = append(conds, queryArgs.Where)
	}

	query := sqlf.Sprintf(
		roleListQueryFmtstr,
		selects,
		joins,
		sqlf.Join(conds, " AND "),
	)

	query = queryArgs.AppendOrderToQuery(query)
	query = queryArgs.AppendLimitToQuery(query)

	rows, err := r.Query(ctx, query)
	if err != nil {
		return errors.Wrap(err, "error running query")
	}
	defer rows.Close()
	for rows.Next() {
		if err := scanRole(rows); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (r *roleStore) computeConditionsAndJoins(opts RolesListOptions) ([]*sqlf.Query, *sqlf.Query) {
	var conds []*sqlf.Query
	var joins = sqlf.Sprintf("")

	if opts.System {
		conds = append(conds, sqlf.Sprintf("system IS TRUE"))
	}

	if opts.UserID != 0 {
		conds = append(conds, sqlf.Sprintf("user_roles.user_id = %s", opts.UserID))
		joins = sqlf.Sprintf("INNER JOIN user_roles ON user_roles.role_id = roles.id")
	}

	if len(conds) == 0 {
		conds = append(conds, sqlf.Sprintf("TRUE"))
	}

	return conds, joins
}

const roleCreateQueryFmtStr = `
INSERT INTO
	roles (%s)
	VALUES (
		%s,
		%s
	)
	RETURNING %s

`

func (r *roleStore) Create(ctx context.Context, name string, isSystemRole bool) (_ *types.Role, err error) {
	q := sqlf.Sprintf(
		roleCreateQueryFmtStr,
		sqlf.Join(roleInsertColumns, ", "),
		name,
		isSystemRole,
		// Returning
		sqlf.Join(roleColumns, ", "),
	)

	role, err := scanRole(r.QueryRow(ctx, q))
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) && e.ConstraintName == "unique_role_name" {
			return nil, errCannotCreateRole{errorCodeRoleNameExists}
		}
		return nil, errors.Wrap(err, "scanning role")
	}

	return role, nil
}

const roleCountQueryFmtstr = `
SELECT COUNT(1) FROM roles
%s
WHERE %s
`

func (r *roleStore) Count(ctx context.Context, opts RolesListOptions) (c int, err error) {
	conds, joins := r.computeConditionsAndJoins(opts)

	query := sqlf.Sprintf(
		roleCountQueryFmtstr,
		joins,
		sqlf.Join(conds, " AND "),
	)
	count, _, err := basestore.ScanFirstInt(r.Query(ctx, query))
	return count, err
}

const roleUpdateQueryFmtstr = `
UPDATE roles
SET
    name = %s
WHERE
	id = %s AND NOT system
RETURNING
	%s
`

func (r *roleStore) Update(ctx context.Context, role *types.Role) (*types.Role, error) {
	q := sqlf.Sprintf(roleUpdateQueryFmtstr, role.Name, role.ID, sqlf.Join(roleColumns, ", "))

	updated, err := scanRole(r.QueryRow(ctx, q))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &RoleNotFoundErr{ID: role.ID}
		}
		return nil, errors.Wrap(err, "scanning role")
	}
	return updated, nil
}

const roleDeleteQueryFmtStr = `
DELETE FROM roles
WHERE id = %s AND NOT system
`

func (r *roleStore) Delete(ctx context.Context, opts DeleteRoleOpts) error {
	if opts.ID <= 0 {
		return errors.New("missing id from sql query")
	}

	// We don't allow deletion of system roles such as USER & SITE_ADMINISTRATOR
	q := sqlf.Sprintf(roleDeleteQueryFmtStr, opts.ID)
	result, err := r.ExecResult(ctx, q)
	if err != nil {
		return errors.Wrap(err, "running delete query")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "checking deleted rows")
	}

	if rowsAffected == 0 {
		return errors.Wrap(&RoleNotFoundErr{opts.ID}, "failed to delete role")
	}
	return nil
}
