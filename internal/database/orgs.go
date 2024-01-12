package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// OrgNotFoundError occurs when an organization is not found.
type OrgNotFoundError struct {
	Message string
}

func (e *OrgNotFoundError) Error() string {
	return fmt.Sprintf("org not found: %s", e.Message)
}

func (e *OrgNotFoundError) NotFound() bool {
	return true
}

var errOrgNameAlreadyExists = errors.New("organization name is already taken (by a user, team, or another organization)")

type OrgStore interface {
	AddOrgsOpenBetaStats(ctx context.Context, userID int32, data string) (string, error)
	Count(context.Context, OrgsListOptions) (int, error)
	Create(ctx context.Context, name string, displayName *string) (*types.Org, error)
	Delete(ctx context.Context, id int32) (err error)
	Done(error) error
	GetByID(ctx context.Context, orgID int32) (*types.Org, error)
	GetByName(context.Context, string) (*types.Org, error)
	GetByUserID(ctx context.Context, userID int32) ([]*types.Org, error)
	HardDelete(ctx context.Context, id int32) (err error)
	List(context.Context, *OrgsListOptions) ([]*types.Org, error)
	Transact(context.Context) (OrgStore, error)
	Update(ctx context.Context, id int32, displayName *string) (*types.Org, error)
	UpdateOrgsOpenBetaStats(ctx context.Context, id string, orgID int32) error
	With(basestore.ShareableStore) OrgStore
	basestore.ShareableStore
}

type orgStore struct {
	*basestore.Store
}

// OrgsWith instantiates and returns a new OrgStore using the other store handle.
func OrgsWith(other basestore.ShareableStore) OrgStore {
	return &orgStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (o *orgStore) With(other basestore.ShareableStore) OrgStore {
	return &orgStore{Store: o.Store.With(other)}
}

func (o *orgStore) Transact(ctx context.Context) (OrgStore, error) {
	txBase, err := o.Store.Transact(ctx)
	return &orgStore{Store: txBase}, err
}

// GetByUserID returns a list of all organizations for the user. An empty slice is
// returned if the user is not authenticated or is not a member of any org.
func (o *orgStore) GetByUserID(ctx context.Context, userID int32) ([]*types.Org, error) {
	return o.getByUserID(ctx, userID, false)
}

// getByUserID returns a list of all organizations for the user. An empty slice is
// returned if the user is not authenticated or is not a member of any org.
//
// onlyOrgsWithRepositories parameter determines, if the function returns all organizations
// or only those with repositories attached
func (o *orgStore) getByUserID(ctx context.Context, userID int32, onlyOrgsWithRepositories bool) ([]*types.Org, error) {
	queryString :=
		`SELECT orgs.id, orgs.name, orgs.display_name, orgs.created_at, orgs.updated_at
		FROM org_members
		LEFT OUTER JOIN orgs ON org_members.org_id = orgs.id
		WHERE user_id=$1
			AND orgs.deleted_at IS NULL`
	if onlyOrgsWithRepositories {
		queryString += `
			AND EXISTS(
				SELECT
				FROM external_service_repos
				WHERE external_service_repos.org_id = orgs.id
				LIMIT 1
			)`
	}
	rows, err := o.Handle().QueryContext(ctx, queryString, userID)
	if err != nil {
		return []*types.Org{}, err
	}

	orgs := []*types.Org{}
	defer rows.Close()
	for rows.Next() {
		org := types.Org{}
		err := rows.Scan(&org.ID, &org.Name, &org.DisplayName, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			return nil, err
		}

		orgs = append(orgs, &org)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orgs, nil
}

func (o *orgStore) GetByID(ctx context.Context, orgID int32) (*types.Org, error) {
	orgs, err := o.getBySQL(ctx, "WHERE deleted_at IS NULL AND id=$1 LIMIT 1", orgID)
	if err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		return nil, &OrgNotFoundError{fmt.Sprintf("id %d", orgID)}
	}
	return orgs[0], nil
}

func (o *orgStore) GetByName(ctx context.Context, name string) (*types.Org, error) {
	orgs, err := o.getBySQL(ctx, "WHERE deleted_at IS NULL AND name=$1 LIMIT 1", name)
	if err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		return nil, &OrgNotFoundError{fmt.Sprintf("name %s", name)}
	}
	return orgs[0], nil
}

func (o *orgStore) Count(ctx context.Context, opt OrgsListOptions) (int, error) {
	q := sqlf.Sprintf("SELECT COUNT(*) FROM orgs WHERE %s", o.listSQL(opt))

	var count int
	if err := o.QueryRow(ctx, q).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// OrgsListOptions specifies the options for listing organizations.
type OrgsListOptions struct {
	// Query specifies a search query for organizations.
	Query string

	*LimitOffset
}

func (o *orgStore) List(ctx context.Context, opt *OrgsListOptions) ([]*types.Org, error) {
	if opt == nil {
		opt = &OrgsListOptions{}
	}
	q := sqlf.Sprintf("WHERE %s ORDER BY id ASC %s", o.listSQL(*opt), opt.LimitOffset.SQL())
	return o.getBySQL(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
}

func (*orgStore) listSQL(opt OrgsListOptions) *sqlf.Query {
	conds := []*sqlf.Query{sqlf.Sprintf("deleted_at IS NULL")}
	if opt.Query != "" {
		query := "%" + opt.Query + "%"
		conds = append(conds, sqlf.Sprintf("name ILIKE %s OR display_name ILIKE %s", query, query))
	}
	return sqlf.Sprintf("(%s)", sqlf.Join(conds, ") AND ("))
}

func (o *orgStore) getBySQL(ctx context.Context, query string, args ...any) ([]*types.Org, error) {
	rows, err := o.Handle().QueryContext(ctx, "SELECT id, name, display_name, created_at, updated_at FROM orgs "+query, args...)
	if err != nil {
		return nil, err
	}

	orgs := []*types.Org{}
	defer rows.Close()
	for rows.Next() {
		org := types.Org{}
		err := rows.Scan(&org.ID, &org.Name, &org.DisplayName, &org.CreatedAt, &org.UpdatedAt)
		if err != nil {
			return nil, err
		}

		orgs = append(orgs, &org)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return orgs, nil
}

func (o *orgStore) Create(ctx context.Context, name string, displayName *string) (newOrg *types.Org, err error) {
	tx, err := o.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	newOrg = &types.Org{
		Name:        name,
		DisplayName: displayName,
	}
	newOrg.CreatedAt = time.Now()
	newOrg.UpdatedAt = newOrg.CreatedAt
	err = tx.Handle().QueryRowContext(
		ctx,
		"INSERT INTO orgs(name, display_name, created_at, updated_at) VALUES($1, $2, $3, $4) RETURNING id",
		newOrg.Name, newOrg.DisplayName, newOrg.CreatedAt, newOrg.UpdatedAt).Scan(&newOrg.ID)
	if err != nil {
		var e *pgconn.PgError
		if errors.As(err, &e) {
			switch e.ConstraintName {
			case "orgs_name":
				return nil, errOrgNameAlreadyExists
			case "orgs_name_max_length", "orgs_name_valid_chars":
				return nil, errors.Errorf("org name invalid: %s", e.ConstraintName)
			case "orgs_display_name_max_length":
				return nil, errors.Errorf("org display name invalid: %s", e.ConstraintName)
			}
		}

		return nil, err
	}

	// Reserve organization name in shared users+orgs+teams namespace.
	if _, err := tx.Handle().ExecContext(ctx, "INSERT INTO names(name, org_id) VALUES($1, $2)", newOrg.Name, newOrg.ID); err != nil {
		return nil, errOrgNameAlreadyExists
	}

	return newOrg, nil
}

func (o *orgStore) Update(ctx context.Context, id int32, displayName *string) (*types.Org, error) {
	org, err := o.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// NOTE: It is not possible to update an organization's name. If it becomes possible, we need to
	// also update the `names` table to ensure the new name is available in the shared users+orgs
	// namespace.

	if displayName != nil {
		org.DisplayName = displayName
		if _, err := o.Handle().ExecContext(ctx, "UPDATE orgs SET display_name=$1 WHERE id=$2 AND deleted_at IS NULL", org.DisplayName, id); err != nil {
			return nil, err
		}
	}
	org.UpdatedAt = time.Now()
	if _, err := o.Handle().ExecContext(ctx, "UPDATE orgs SET updated_at=$1 WHERE id=$2 AND deleted_at IS NULL", org.UpdatedAt, id); err != nil {
		return nil, err
	}

	return org, nil
}

func (o *orgStore) Delete(ctx context.Context, id int32) (err error) {
	// Wrap in transaction because we delete from multiple tables.
	tx, err := o.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Done(err)
	}()

	res, err := tx.Handle().ExecContext(ctx, "UPDATE orgs SET deleted_at=now() WHERE id=$1 AND deleted_at IS NULL", id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return &OrgNotFoundError{fmt.Sprintf("id %d", id)}
	}

	// Release the organization name so it can be used by another user or org.
	if _, err := tx.Handle().ExecContext(ctx, "DELETE FROM names WHERE org_id=$1", id); err != nil {
		return err
	}

	if _, err := tx.Handle().ExecContext(ctx, "UPDATE org_invitations SET deleted_at=now() WHERE deleted_at IS NULL AND org_id=$1", id); err != nil {
		return err
	}
	if _, err := tx.Handle().ExecContext(ctx, "UPDATE registry_extensions SET deleted_at=now() WHERE deleted_at IS NULL AND publisher_org_id=$1", id); err != nil {
		return err
	}

	return nil
}

func (o *orgStore) HardDelete(ctx context.Context, id int32) (err error) {
	// Check if the org exists even if it has been previously soft deleted
	orgs, err := o.getBySQL(ctx, "WHERE id=$1 LIMIT 1", id)
	if err != nil {
		return err
	}

	if len(orgs) == 0 {
		return &OrgNotFoundError{fmt.Sprintf("id %d", id)}
	}

	tx, err := o.Transact(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err = tx.Done(err)
	}()

	// Some tables that reference the "orgs" table do not have ON DELETE CASCADE set, so we need to manually delete their entries before
	// hard deleting an org.
	tablesAndKeys := map[string]string{
		"org_members":         "org_id",
		"org_invitations":     "org_id",
		"registry_extensions": "publisher_org_id",
		"saved_searches":      "org_id",
		"notebooks":           "namespace_org_id",
		"settings":            "org_id",
		"orgs":                "id",
	}

	// 🚨 SECURITY: Be cautious about changing order here.
	tables := []string{"org_members", "org_invitations", "registry_extensions", "saved_searches", "notebooks", "settings", "orgs"}
	for _, t := range tables {
		query := sqlf.Sprintf(fmt.Sprintf("DELETE FROM %s WHERE %s=%d", t, tablesAndKeys[t], id))

		_, err := tx.Handle().ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *orgStore) AddOrgsOpenBetaStats(ctx context.Context, userID int32, data string) (id string, err error) {
	query := sqlf.Sprintf("INSERT INTO orgs_open_beta_stats(user_id, data) VALUES(%d, %s) RETURNING id;", userID, data)

	err = o.Handle().QueryRowContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...).Scan(&id)
	return id, err
}

func (o *orgStore) UpdateOrgsOpenBetaStats(ctx context.Context, id string, orgID int32) error {
	query := sqlf.Sprintf("UPDATE orgs_open_beta_stats SET org_id=%d WHERE id=%s;", orgID, id)

	_, err := o.Handle().ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	return err
}
