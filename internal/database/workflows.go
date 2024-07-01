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

type WorkflowStore interface {
	Create(_ context.Context, _ *types.Workflow, actorUID int32) (*types.Workflow, error)
	Update(_ context.Context, _ *types.Workflow, actorUID int32) (*types.Workflow, error)
	Delete(context.Context, int32) error
	GetByID(context.Context, int32) (*types.Workflow, error)
	List(context.Context, WorkflowListArgs, *PaginationArgs) ([]*types.Workflow, error)
	Count(context.Context, WorkflowListArgs) (int, error)
	WithTransact(context.Context, func(WorkflowStore) error) error
	With(basestore.ShareableStore) WorkflowStore
	basestore.ShareableStore
}

type workflowStore struct {
	*basestore.Store
}

// WorkflowsWith instantiates and returns a new WorkflowStore using the other store handle.
func WorkflowsWith(other basestore.ShareableStore) WorkflowStore {
	return &workflowStore{Store: basestore.NewWithHandle(other.Handle())}
}

func (s *workflowStore) With(other basestore.ShareableStore) WorkflowStore {
	return &workflowStore{Store: s.Store.With(other)}
}

func (s *workflowStore) WithTransact(ctx context.Context, f func(WorkflowStore) error) error {
	return s.Store.WithTransact(ctx, func(tx *basestore.Store) error {
		return f(&workflowStore{Store: tx})
	})
}

var (
	workflowWriteColumns = sqlf.Sprintf("name, description, template_text, draft")
	workflowReadColumns  = sqlf.Sprintf("%v, owner_user_id, owner_org_id, created_by, created_at, updated_by, updated_at", workflowWriteColumns)
)

// Create creates a new workflow with the specified parameters. The ID field must be zero, or an
// error will be returned.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to create the workflow.
func (s *workflowStore) Create(ctx context.Context, newWorkflow *types.Workflow, actorUID int32) (created *types.Workflow, err error) {
	if newWorkflow.ID != 0 {
		return nil, errors.New("newWorkflow.ID must be zero")
	}

	tr, ctx := trace.New(ctx, "database.Workflows.Create")
	defer tr.EndWithErr(&err)

	return scanWorkflow(
		s.QueryRow(ctx,
			sqlf.Sprintf(`
			INSERT INTO workflows(%v, owner_user_id, owner_org_id, created_by, created_at, updated_by, updated_at)
			VALUES(%v, %v, %v, %v, %v, %v, %v, DEFAULT, %v, DEFAULT)
			RETURNING id, %v`,
				workflowWriteColumns,
				newWorkflow.Name,
				newWorkflow.Description,
				newWorkflow.TemplateText,
				newWorkflow.Draft,
				newWorkflow.Owner.User,
				newWorkflow.Owner.Org,
				actorUID,
				actorUID,
				workflowReadColumns,
			),
		))
}

// Update updates an existing workflow.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to perform the update.
func (s *workflowStore) Update(ctx context.Context, workflow *types.Workflow, actorUID int32) (updated *types.Workflow, err error) {
	tr, ctx := trace.New(ctx, "database.Workflows.Update")
	defer tr.EndWithErr(&err)

	fieldUpdates := []*sqlf.Query{
		sqlf.Sprintf("name=%v", workflow.Name),
		sqlf.Sprintf("description=%v", workflow.Description),
		sqlf.Sprintf("template_text=%v", workflow.TemplateText),
		sqlf.Sprintf("draft=%v", workflow.Draft),
		// Updating the owner is not currently supported.
		sqlf.Sprintf("updated_by=%v", actorUID),
		sqlf.Sprintf("updated_at=now()"),
	}

	return scanWorkflow(
		s.QueryRow(ctx,
			sqlf.Sprintf(
				`UPDATE workflows SET %s WHERE id=%v RETURNING id, %v`,
				sqlf.Join(fieldUpdates, ", "),
				workflow.ID,
				workflowReadColumns,
			),
		))
}

// Delete hard-deletes an existing workflow.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure the user has proper permissions to perform the delete.
func (s *workflowStore) Delete(ctx context.Context, id int32) (err error) {
	tr, ctx := trace.New(ctx, "database.Workflows.Delete")
	defer tr.EndWithErr(&err)
	_, err = s.Handle().ExecContext(ctx, `DELETE FROM workflows WHERE id=$1`, id)
	return err
}

// GetByID returns the workflow with the given ID.
//
// 🚨 SECURITY: This method does NOT verify the user's identity or that the user is an admin. It is
// the caller's responsibility to ensure this response only makes it to users with proper
// permissions to access the workflow.
func (s *workflowStore) GetByID(ctx context.Context, id int32) (_ *types.Workflow, err error) {
	tr, ctx := trace.New(ctx, "database.Workflows.GetByID")
	defer tr.EndWithErr(&err)

	return scanWorkflow(s.QueryRow(ctx, sqlf.Sprintf(`SELECT id, %v FROM workflows WHERE id=%v`, workflowReadColumns, id)))
}

type WorkflowListArgs struct {
	Query          string
	AffiliatedUser *int32
	Owner          *types.Namespace
	HideDrafts     bool
	OrderBy        WorkflowsOrderBy
}

type WorkflowsOrderBy uint8

const (
	WorkflowsOrderByID WorkflowsOrderBy = iota
	WorkflowsOrderByNameWithOwner
	WorkflowsOrderByUpdatedAt
)

func (a WorkflowListArgs) toSQL() (where []*sqlf.Query, orderBy PaginationArgs, err error) {
	if a.Query != "" {
		queryStr := "%" + a.Query + "%"
		where = append(where, sqlf.Sprintf("(name_with_owner ILIKE %v OR description ILIKE %v OR template_text ILIKE %v)", queryStr, queryStr, queryStr))
	}
	if a.AffiliatedUser != nil {
		where = append(where,
			sqlf.Sprintf("(%v OR %v)",
				sqlf.Sprintf("owner_user_id=%v", *a.AffiliatedUser),
				sqlf.Sprintf("owner_org_id IN (SELECT org_members.org_id FROM org_members LEFT JOIN orgs ON orgs.id=org_members.org_id WHERE orgs.deleted_at IS NULL AND org_members.user_id=%v)", *a.AffiliatedUser),
			),
		)
	}
	if a.Owner != nil {
		if a.Owner.User != nil && *a.Owner.User != 0 {
			where = append(where, sqlf.Sprintf("owner_user_id=%v", *a.Owner.User))
		} else if a.Owner.Org != nil && *a.Owner.Org != 0 {
			where = append(where, sqlf.Sprintf("owner_org_id=%v", *a.Owner.Org))
		} else {
			return nil, PaginationArgs{}, errors.New("invalid owner (no user or org ID)")
		}
	}
	if a.HideDrafts {
		where = append(where, sqlf.Sprintf("NOT draft"))
	}
	if len(where) == 0 {
		where = append(where, sqlf.Sprintf("TRUE"))
	}

	switch a.OrderBy {
	case WorkflowsOrderByID:
		orderBy.OrderBy = []OrderByOption{{Field: "id"}}
		orderBy.Ascending = true
	case WorkflowsOrderByNameWithOwner:
		orderBy.OrderBy = []OrderByOption{{Field: "name_with_owner"}}
		orderBy.Ascending = true
	case WorkflowsOrderByUpdatedAt:
		orderBy.OrderBy = []OrderByOption{{Field: "updated_at"}}
		orderBy.Ascending = false
	default:
		return nil, PaginationArgs{}, errors.New("invalid OrderBy")
	}

	return where, orderBy, nil
}

// List lists all workflows matching the given filter args.
//
// 🚨 SECURITY: This method does NOT perform authorization checks.
func (s *workflowStore) List(ctx context.Context, args WorkflowListArgs, paginationArgs *PaginationArgs) (_ []*types.Workflow, err error) {
	tr, ctx := trace.New(ctx, "database.Workflows.List")
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

	query := sqlf.Sprintf(`SELECT id, %v FROM workflows_view WHERE (%v)`,
		workflowReadColumns, sqlf.Join(where, ") AND ("),
	)
	query = pg.AppendOrderToQuery(query)
	query = pg.AppendLimitToQuery(query)
	return scanWorkflows(s.Query(ctx, query))
}

var scanWorkflows = basestore.NewSliceScanner(scanWorkflow)

func scanWorkflow(s dbutil.Scanner) (*types.Workflow, error) {
	var row types.Workflow
	if err := s.Scan(
		&row.ID,
		&row.Name,
		&row.Description,
		&row.TemplateText,
		&row.Draft,
		&row.Owner.User,
		&row.Owner.Org,
		&row.CreatedByUser,
		&row.CreatedAt,
		&row.UpdatedByUser,
		&row.UpdatedAt,
	); err != nil {
		return nil, errors.Wrap(err, "Scan")
	}
	return &row, nil
}

// Count counts all workflows matching the given filter args.
//
// 🚨 SECURITY: This method does NOT perform authorization checks.
func (s *workflowStore) Count(ctx context.Context, args WorkflowListArgs) (count int, err error) {
	tr, ctx := trace.New(ctx, "database.Workflows.Count")
	defer tr.EndWithErr(&err)

	where, _, err := args.toSQL()
	if err != nil {
		return 0, err
	}
	query := sqlf.Sprintf(`SELECT COUNT(*) FROM workflows_view WHERE (%v)`, sqlf.Join(where, ") AND ("))
	count, _, err = basestore.ScanFirstInt(s.Query(ctx, query))
	return count, err
}
