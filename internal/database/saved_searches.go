package database

import (
	"context"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type SavedSearchStore interface {
	Create(context.Context, *types.SavedSearch) (*types.SavedSearch, error)
	Update(context.Context, *types.SavedSearch) (*types.SavedSearch, error)
	UpdateOwner(_ context.Context, id int32, newOwner types.Namespace) (*types.SavedSearch, error)
	Delete(context.Context, int32) error
	GetByID(context.Context, int32) (*types.SavedSearch, error)
	List(context.Context, SavedSearchListArgs, *PaginationArgs) ([]*types.SavedSearch, error)
	Count(context.Context, SavedSearchListArgs) (int, error)
	WithTransact(context.Context, func(SavedSearchStore) error) error
	With(basestore.ShareableStore) SavedSearchStore
	basestore.ShareableStore
}

type savedSearchStore struct {
	*basestore.Store
}

// SavedSearchesWith instantiates and returns a new SavedSearchStore using the other store handle.
func SavedSearchesWith(other basestore.ShareableStore) SavedSearchStore {
	return &savedSearchStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *savedSearchStore) With(other basestore.ShareableStore) SavedSearchStore {
	return &savedSearchStore{Store: s.Store.With(other)}
}

func (s *savedSearchStore) WithTransact(ctx context.Context, f func(SavedSearchStore) error) error {
	return s.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&savedSearchStore{Store: tx})
	})
}

var savedSearchColumns = sqlf.Sprintf("description, query, user_id, org_id")

// Create creates a new saved search with the specified parameters. The ID field must be zero, or an
// error will be returned.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to create the saved search.
func (s *savedSearchStore) Create(ctx context.Context, newSavedSearch *types.SavedSearch) (created *types.SavedSearch, err error) {
	if newSavedSearch.ID != 0 {
		return nil, errors.New("newSavedSearch.ID must be zero")
	}

	tr, ctx := trace.New(ctx, "database.SavedSearches.Create")
	defer tr.EndWithErr(&err)

	return scanSavedSearch(
		s.QueryRow(ctx,
			sqlf.Sprintf(`INSERT INTO saved_searches(%v) VALUES(%v, %v, %v, %v) RETURNING id, %v`,
				savedSearchColumns,
				newSavedSearch.Description,
				newSavedSearch.Query,
				newSavedSearch.Owner.User,
				newSavedSearch.Owner.Org,
				savedSearchColumns,
			),
		))
}

// Update updates an existing saved search.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to perform the update.
func (s *savedSearchStore) Update(ctx context.Context, savedSearch *types.SavedSearch) (updated *types.SavedSearch, err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.Update")
	defer tr.EndWithErr(&err)

	return s.update(ctx, savedSearch.ID, []*sqlf.Query{
		sqlf.Sprintf("description=%s", savedSearch.Description),
		sqlf.Sprintf("query=%s", savedSearch.Query),
	})
}

// UpdateOwner updates the owner of an existing saved search.
//
// 🚨 SECURITY: This method does NOT verify that the user has permissions to do this. The caller
// MUST do so.
func (s *savedSearchStore) UpdateOwner(ctx context.Context, id int32, newOwner types.Namespace) (updated *types.SavedSearch, err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.UpdateOwner")
	defer tr.EndWithErr(&err)
	return s.update(ctx, id, []*sqlf.Query{
		sqlf.Sprintf("user_id=%v", newOwner.User),
		sqlf.Sprintf("org_id=%v", newOwner.Org),
	})
}

func (s *savedSearchStore) update(ctx context.Context, id int32, updates []*sqlf.Query) (updated *types.SavedSearch, err error) {
	updates = append(updates, sqlf.Sprintf("updated_at=now()"))
	return scanSavedSearch(
		s.QueryRow(ctx,
			sqlf.Sprintf(
				`UPDATE saved_searches SET %s WHERE id=%v RETURNING id, %v`,
				sqlf.Join(updates, ", "),
				id,
				savedSearchColumns,
			),
		))
}

// Delete hard-deletes an existing saved search.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to perform the delete.
func (s *savedSearchStore) Delete(ctx context.Context, id int32) (err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.Delete")
	defer tr.EndWithErr(&err)
	_, err = s.Handle().ExecContext(ctx, `DELETE FROM saved_searches WHERE id=$1`, id)
	return err
}

// GetByID returns the saved search with the given ID.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure this response only makes it to users with proper
// permissions to access the saved search.
func (s *savedSearchStore) GetByID(ctx context.Context, id int32) (_ *types.SavedSearch, err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.GetByID")
	defer tr.EndWithErr(&err)

	return scanSavedSearch(s.QueryRow(ctx, sqlf.Sprintf(`SELECT id, %v FROM saved_searches WHERE id=%v`, savedSearchColumns, id)))
}

type SavedSearchListArgs struct {
	Query          string
	AffiliatedUser *int32
	Owner          *types.Namespace
	OrderBy        SavedSearchesOrderBy
}

type SavedSearchesOrderBy uint8

const (
	SavedSearchesOrderByID SavedSearchesOrderBy = iota
	SavedSearchesOrderByDescription
	SavedSearchesOrderByUpdatedAt
)

func (a SavedSearchListArgs) toSQL() (where []*sqlf.Query, orderBy PaginationArgs, err error) {
	if a.Query != "" {
		queryStr := "%" + a.Query + "%"
		where = append(where, sqlf.Sprintf("(description ILIKE %v OR query ILIKE %v)", queryStr, queryStr))
	}
	if a.AffiliatedUser != nil {
		where = append(where,
			sqlf.Sprintf("(%v OR %v)",
				sqlf.Sprintf("user_id=%v", *a.AffiliatedUser),
				sqlf.Sprintf("org_id IN (SELECT org_members.org_id FROM org_members LEFT JOIN orgs ON orgs.id=org_members.org_id WHERE orgs.deleted_at IS NULL AND org_members.user_id=%v)", *a.AffiliatedUser),
			),
		)
	}
	if a.Owner != nil {
		if a.Owner.User != nil && *a.Owner.User != 0 {
			where = append(where, sqlf.Sprintf("user_id=%v", *a.Owner.User))
		} else if a.Owner.Org != nil && *a.Owner.Org != 0 {
			where = append(where, sqlf.Sprintf("org_id=%v", *a.Owner.Org))
		} else {
			return nil, PaginationArgs{}, errors.New("invalid owner (no user or org ID)")
		}
	}
	if len(where) == 0 {
		where = append(where, sqlf.Sprintf("TRUE"))
	}

	switch a.OrderBy {
	case SavedSearchesOrderByID:
		orderBy.OrderBy = []OrderByOption{{Field: "id"}}
		orderBy.Ascending = true
	case SavedSearchesOrderByDescription:
		orderBy.OrderBy = []OrderByOption{{Field: "description"}}
		orderBy.Ascending = true
	case SavedSearchesOrderByUpdatedAt:
		orderBy.OrderBy = []OrderByOption{{Field: "updated_at"}}
		orderBy.Ascending = false
	default:
		return nil, PaginationArgs{}, errors.New("invalid OrderBy")
	}

	return where, orderBy, nil
}

// List lists all saved searches matching the given filter args.
//
// 🚨 SECURITY: This method does NOT perform authorization checks.
func (s *savedSearchStore) List(ctx context.Context, args SavedSearchListArgs, paginationArgs *PaginationArgs) (_ []*types.SavedSearch, err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.List")
	defer tr.EndWithErr(&err)

	where, orderBy, err := args.toSQL()
	if err != nil {
		return nil, err
	}

	if paginationArgs == nil {
		paginationArgs = &PaginationArgs{}
	}
	paginationArgs.OrderBy = orderBy.OrderBy
	paginationArgs.Ascending = orderBy.Ascending
	pg := paginationArgs.SQL()
	if pg.Where != nil {
		where = append(where, pg.Where)
	}

	query := sqlf.Sprintf(`SELECT id, %v FROM saved_searches WHERE (%v)`,
		savedSearchColumns, sqlf.Join(where, ") AND ("),
	)
	query = pg.AppendOrderToQuery(query)
	query = pg.AppendLimitToQuery(query)
	return scanSavedSearches(s.Query(ctx, query))
}

var scanSavedSearches = basestore.NewSliceScanner(scanSavedSearch)

func scanSavedSearch(s dbutil.Scanner) (*types.SavedSearch, error) {
	var row types.SavedSearch
	if err := s.Scan(
		&row.ID,
		&row.Description,
		&row.Query,
		&row.Owner.User,
		&row.Owner.Org,
	); err != nil {
		return nil, errors.Wrap(err, "Scan")
	}
	return &row, nil
}

// Count counts all saved searches matching the given filter args.
//
// 🚨 SECURITY: This method does NOT perform authorization checks.
func (s *savedSearchStore) Count(ctx context.Context, args SavedSearchListArgs) (count int, err error) {
	tr, ctx := trace.New(ctx, "database.SavedSearches.Count")
	defer tr.EndWithErr(&err)

	where, _, err := args.toSQL()
	if err != nil {
		return 0, err
	}
	query := sqlf.Sprintf(`SELECT COUNT(*) FROM saved_searches WHERE (%v)`, sqlf.Join(where, ") AND ("))
	count, _, err = basestore.ScanFirstInt(s.Query(ctx, query))
	return count, err
}
