package store

import (
	"context"
	"strconv"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// batchChangeColumns are used by the batch change related Store methods to insert,
// update and query batches.
var batchChangeColumns = []*sqlf.Query{
	sqlf.Sprintf("batch_changes.id"),
	sqlf.Sprintf("batch_changes.name"),
	sqlf.Sprintf("batch_changes.description"),
	sqlf.Sprintf("batch_changes.initial_applier_id"),
	sqlf.Sprintf("batch_changes.last_applier_id"),
	sqlf.Sprintf("batch_changes.last_applied_at"),
	sqlf.Sprintf("batch_changes.namespace_user_id"),
	sqlf.Sprintf("batch_changes.namespace_org_id"),
	sqlf.Sprintf("batch_changes.created_at"),
	sqlf.Sprintf("batch_changes.updated_at"),
	sqlf.Sprintf("batch_changes.closed_at"),
	sqlf.Sprintf("batch_changes.batch_spec_id"),
}

// batchChangeInsertColumns is the list of batch changes columns that are
// modified in CreateBatchChange and UpdateBatchChange.
// update and query batches.
var batchChangeInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("name"),
	sqlf.Sprintf("description"),
	sqlf.Sprintf("initial_applier_id"),
	sqlf.Sprintf("last_applier_id"),
	sqlf.Sprintf("last_applied_at"),
	sqlf.Sprintf("namespace_user_id"),
	sqlf.Sprintf("namespace_org_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
	sqlf.Sprintf("closed_at"),
	sqlf.Sprintf("batch_spec_id"),
}

// CreateBatchChange creates the given batch change.
func (s *Store) CreateBatchChange(ctx context.Context, c *batches.BatchChange) error {
	q := s.createBatchChangeQuery(c)

	return s.query(ctx, q, func(sc scanner) (err error) {
		return scanBatchChange(c, sc)
	})
}

var createBatchChangeQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CreateBatchChange
INSERT INTO batch_changes (%s)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING %s
`

func (s *Store) createBatchChangeQuery(c *batches.BatchChange) *sqlf.Query {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = s.now()
	}

	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	return sqlf.Sprintf(
		createBatchChangeQueryFmtstr,
		sqlf.Join(batchChangeInsertColumns, ", "),
		c.Name,
		c.Description,
		nullInt32Column(c.InitialApplierID),
		nullInt32Column(c.LastApplierID),
		c.LastAppliedAt,
		nullInt32Column(c.NamespaceUserID),
		nullInt32Column(c.NamespaceOrgID),
		c.CreatedAt,
		c.UpdatedAt,
		nullTimeColumn(c.ClosedAt),
		c.BatchSpecID,
		sqlf.Join(batchChangeColumns, ", "),
	)
}

// UpdateBatchChange updates the given bach change.
func (s *Store) UpdateBatchChange(ctx context.Context, c *batches.BatchChange) error {
	q := s.updateBatchChangeQuery(c)

	return s.query(ctx, q, func(sc scanner) (err error) { return scanBatchChange(c, sc) })
}

var updateBatchChangeQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:UpdateBatchChange
UPDATE batch_changes
SET (%s) = (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
WHERE id = %s
RETURNING %s
`

func (s *Store) updateBatchChangeQuery(c *batches.BatchChange) *sqlf.Query {
	c.UpdatedAt = s.now()

	return sqlf.Sprintf(
		updateBatchChangeQueryFmtstr,
		sqlf.Join(batchChangeInsertColumns, ", "),
		c.Name,
		c.Description,
		nullInt32Column(c.InitialApplierID),
		nullInt32Column(c.LastApplierID),
		c.LastAppliedAt,
		nullInt32Column(c.NamespaceUserID),
		nullInt32Column(c.NamespaceOrgID),
		c.CreatedAt,
		c.UpdatedAt,
		nullTimeColumn(c.ClosedAt),
		c.BatchSpecID,
		c.ID,
		sqlf.Join(batchChangeColumns, ", "),
	)
}

// DeleteBatchChange deletes the batch change with the given ID.
func (s *Store) DeleteBatchChange(ctx context.Context, id int64) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(deleteBatchChangeQueryFmtstr, id))
}

var deleteBatchChangeQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:DeleteBatchChange
DELETE FROM batch_changes WHERE id = %s
`

// CountBatchChangesOpts captures the query options needed for
// counting batches.
type CountBatchChangesOpts struct {
	ChangesetID int64
	State       batches.BatchChangeState

	InitialApplierID int32

	NamespaceUserID int32
	NamespaceOrgID  int32
}

// CountBatchChanges returns the number of batch changes in the database.
func (s *Store) CountBatchChanges(ctx context.Context, opts CountBatchChangesOpts) (int, error) {
	return s.queryCount(ctx, countBatchChangesQuery(&opts))
}

var countBatchChangesQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CountBatchChanges
SELECT COUNT(batch_changes.id)
FROM batch_changes
%s
WHERE %s
`

func countBatchChangesQuery(opts *CountBatchChangesOpts) *sqlf.Query {
	joins := []*sqlf.Query{
		sqlf.Sprintf("LEFT JOIN users namespace_user ON batch_changes.namespace_user_id = namespace_user.id"),
		sqlf.Sprintf("LEFT JOIN orgs namespace_org ON batch_changes.namespace_org_id = namespace_org.id"),
	}
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}

	if opts.ChangesetID != 0 {
		joins = append(joins, sqlf.Sprintf("INNER JOIN changesets ON changesets.batch_change_ids ? batch_changes.id::TEXT"))
		preds = append(preds, sqlf.Sprintf("changesets.id = %s", opts.ChangesetID))
	}

	switch opts.State {
	case batches.BatchChangeStateOpen:
		preds = append(preds, sqlf.Sprintf("batch_changes.closed_at IS NULL"))
	case batches.BatchChangeStateClosed:
		preds = append(preds, sqlf.Sprintf("batch_changes.closed_at IS NOT NULL"))
	}

	if opts.InitialApplierID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.initial_applier_id = %d", opts.InitialApplierID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(countBatchChangesQueryFmtstr, sqlf.Join(joins, "\n"), sqlf.Join(preds, "\n AND "))
}

// CountBatchChangeOpts captures the query options needed for getting a batch change
type CountBatchChangeOpts struct {
	ID int64

	NamespaceUserID int32
	NamespaceOrgID  int32

	BatchSpecID int64
	Name        string
}

// GetBatchChange gets a batch change matching the given options.
func (s *Store) GetBatchChange(ctx context.Context, opts CountBatchChangeOpts) (*batches.BatchChange, error) {
	q := getBatchChangeQuery(&opts)

	var c batches.BatchChange
	err := s.query(ctx, q, func(sc scanner) error {
		return scanBatchChange(&c, sc)
	})
	if err != nil {
		return nil, err
	}

	if c.ID == 0 {
		return nil, ErrNoResults
	}

	return &c, nil
}

var getBatchChangesQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:GetBatchChange
SELECT %s FROM batch_changes
LEFT JOIN users namespace_user ON batch_changes.namespace_user_id = namespace_user.id
LEFT JOIN orgs  namespace_org  ON batch_changes.namespace_org_id = namespace_org.id
WHERE %s
LIMIT 1
`

func getBatchChangeQuery(opts *CountBatchChangeOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}
	if opts.ID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.id = %s", opts.ID))
	}

	if opts.BatchSpecID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.batch_spec_id = %s", opts.BatchSpecID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if opts.Name != "" {
		preds = append(preds, sqlf.Sprintf("batch_changes.name = %s", opts.Name))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(
		getBatchChangesQueryFmtstr,
		sqlf.Join(batchChangeColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

type GetBatchChangeDiffStatOpts struct {
	BatchChangeID int64
}

func (s *Store) GetBatchChangeDiffStat(ctx context.Context, opts GetBatchChangeDiffStatOpts) (*diff.Stat, error) {
	authzConds, err := database.AuthzQueryConds(ctx, s.Handle().DB())
	if err != nil {
		return nil, errors.Wrap(err, "GetBatchChangeDiffStat generating authz query conds")
	}
	q := getBatchChangeDiffStatQuery(opts, authzConds)

	var diffStat diff.Stat
	err = s.query(ctx, q, func(sc scanner) error {
		return sc.Scan(&diffStat.Added, &diffStat.Changed, &diffStat.Deleted)
	})
	if err != nil {
		return nil, err
	}

	return &diffStat, nil
}

var getBatchChangeDiffStatQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:GetBatchChangeDiffStat
SELECT
	COALESCE(SUM(diff_stat_added), 0) AS added,
	COALESCE(SUM(diff_stat_changed), 0) AS changed,
	COALESCE(SUM(diff_stat_deleted), 0) AS deleted
FROM
	changesets
INNER JOIN repo ON changesets.repo_id = repo.id
WHERE
	changesets.batch_change_ids ? %s AND
	repo.deleted_at IS NULL AND
	-- authz conditions:
	%s
`

func getBatchChangeDiffStatQuery(opts GetBatchChangeDiffStatOpts, authzConds *sqlf.Query) *sqlf.Query {
	return sqlf.Sprintf(getBatchChangeDiffStatQueryFmtstr, strconv.Itoa(int(opts.BatchChangeID)), authzConds)
}

// ListBatchChangesOpts captures the query options needed for
// listing batches.
type ListBatchChangesOpts struct {
	LimitOpts
	ChangesetID int64
	Cursor      int64
	State       batches.BatchChangeState

	InitialApplierID int32

	NamespaceUserID int32
	NamespaceOrgID  int32
}

// ListBatchChanges lists batch changes with the given filters.
func (s *Store) ListBatchChanges(ctx context.Context, opts ListBatchChangesOpts) (cs []*batches.BatchChange, next int64, err error) {
	q := listBatchChangesQuery(&opts)

	cs = make([]*batches.BatchChange, 0, opts.DBLimit())
	err = s.query(ctx, q, func(sc scanner) error {
		var c batches.BatchChange
		if err := scanBatchChange(&c, sc); err != nil {
			return err
		}
		cs = append(cs, &c)
		return nil
	})

	if opts.Limit != 0 && len(cs) == opts.DBLimit() {
		next = cs[len(cs)-1].ID
		cs = cs[:len(cs)-1]
	}

	return cs, next, err
}

var listBatchChangesQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:ListBatchChanges
SELECT %s FROM batch_changes
%s
WHERE %s
ORDER BY id DESC
`

func listBatchChangesQuery(opts *ListBatchChangesOpts) *sqlf.Query {
	joins := []*sqlf.Query{
		sqlf.Sprintf("LEFT JOIN users namespace_user ON batch_changes.namespace_user_id = namespace_user.id"),
		sqlf.Sprintf("LEFT JOIN orgs namespace_org ON batch_changes.namespace_org_id = namespace_org.id"),
	}
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}

	if opts.Cursor != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.id <= %s", opts.Cursor))
	}

	if opts.ChangesetID != 0 {
		joins = append(joins, sqlf.Sprintf("INNER JOIN changesets ON changesets.batch_change_ids ? batch_changes.id::TEXT"))
		preds = append(preds, sqlf.Sprintf("changesets.id = %s", opts.ChangesetID))
	}

	switch opts.State {
	case batches.BatchChangeStateOpen:
		preds = append(preds, sqlf.Sprintf("batch_changes.closed_at IS NULL"))
	case batches.BatchChangeStateClosed:
		preds = append(preds, sqlf.Sprintf("batch_changes.closed_at IS NOT NULL"))
	}

	if opts.InitialApplierID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.initial_applier_id = %d", opts.InitialApplierID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("batch_changes.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(
		listBatchChangesQueryFmtstr+opts.LimitOpts.ToDB(),
		sqlf.Join(batchChangeColumns, ", "),
		sqlf.Join(joins, "\n"),
		sqlf.Join(preds, "\n AND "),
	)
}

func scanBatchChange(c *batches.BatchChange, s scanner) error {
	return s.Scan(
		&c.ID,
		&c.Name,
		&dbutil.NullString{S: &c.Description},
		&dbutil.NullInt32{N: &c.InitialApplierID},
		&dbutil.NullInt32{N: &c.LastApplierID},
		&c.LastAppliedAt,
		&dbutil.NullInt32{N: &c.NamespaceUserID},
		&dbutil.NullInt32{N: &c.NamespaceOrgID},
		&c.CreatedAt,
		&c.UpdatedAt,
		&dbutil.NullTime{Time: &c.ClosedAt},
		&c.BatchSpecID,
	)
}
