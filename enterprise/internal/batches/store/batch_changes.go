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

// batchChangeColumns are used by the campaign related Store methods to insert,
// update and query batches.
var batchChangeColumns = []*sqlf.Query{
	sqlf.Sprintf("campaigns.id"),
	sqlf.Sprintf("campaigns.name"),
	sqlf.Sprintf("campaigns.description"),
	sqlf.Sprintf("campaigns.initial_applier_id"),
	sqlf.Sprintf("campaigns.last_applier_id"),
	sqlf.Sprintf("campaigns.last_applied_at"),
	sqlf.Sprintf("campaigns.namespace_user_id"),
	sqlf.Sprintf("campaigns.namespace_org_id"),
	sqlf.Sprintf("campaigns.created_at"),
	sqlf.Sprintf("campaigns.updated_at"),
	sqlf.Sprintf("campaigns.closed_at"),
	sqlf.Sprintf("campaigns.campaign_spec_id"),
}

// batchChangeInsertColumns is the list of campaign columns that are modified in
// CreateCampaign and UpdateCampaign.
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
	sqlf.Sprintf("campaign_spec_id"),
}

// CreateBatchChange creates the given Campaign.
func (s *Store) CreateBatchChange(ctx context.Context, c *batches.BatchChange) error {
	q := s.createCampaignQuery(c)

	return s.query(ctx, q, func(sc scanner) (err error) {
		return scanCampaign(c, sc)
	})
}

var createCampaignQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CreateBatchChange
INSERT INTO campaigns (%s)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING %s
`

func (s *Store) createCampaignQuery(c *batches.BatchChange) *sqlf.Query {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = s.now()
	}

	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	return sqlf.Sprintf(
		createCampaignQueryFmtstr,
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
		c.CampaignSpecID,
		sqlf.Join(batchChangeColumns, ", "),
	)
}

// UpdateCampaign updates the given Campaign.
func (s *Store) UpdateCampaign(ctx context.Context, c *batches.BatchChange) error {
	q := s.updateCampaignQuery(c)

	return s.query(ctx, q, func(sc scanner) (err error) { return scanCampaign(c, sc) })
}

var updateCampaignQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:UpdateCampaign
UPDATE campaigns
SET (%s) = (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
WHERE id = %s
RETURNING %s
`

func (s *Store) updateCampaignQuery(c *batches.BatchChange) *sqlf.Query {
	c.UpdatedAt = s.now()

	return sqlf.Sprintf(
		updateCampaignQueryFmtstr,
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
		c.CampaignSpecID,
		c.ID,
		sqlf.Join(batchChangeColumns, ", "),
	)
}

// DeleteCampaign deletes the Campaign with the given ID.
func (s *Store) DeleteCampaign(ctx context.Context, id int64) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(deleteCampaignQueryFmtstr, id))
}

var deleteCampaignQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:DeleteCampaign
DELETE FROM campaigns WHERE id = %s
`

// CountCampaignsOpts captures the query options needed for
// counting batches.
type CountCampaignsOpts struct {
	ChangesetID int64
	State       batches.BatchChangeState

	InitialApplierID int32

	NamespaceUserID int32
	NamespaceOrgID  int32
}

// CountCampaigns returns the number of campaigns in the database.
func (s *Store) CountCampaigns(ctx context.Context, opts CountCampaignsOpts) (int, error) {
	return s.queryCount(ctx, countCampaignsQuery(&opts))
}

var countCampaignsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CountCampaigns
SELECT COUNT(campaigns.id)
FROM campaigns
%s
WHERE %s
`

func countCampaignsQuery(opts *CountCampaignsOpts) *sqlf.Query {
	joins := []*sqlf.Query{
		sqlf.Sprintf("LEFT JOIN users namespace_user ON campaigns.namespace_user_id = namespace_user.id"),
		sqlf.Sprintf("LEFT JOIN orgs namespace_org ON campaigns.namespace_org_id = namespace_org.id"),
	}
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}

	if opts.ChangesetID != 0 {
		joins = append(joins, sqlf.Sprintf("INNER JOIN changesets ON changesets.campaign_ids ? campaigns.id::TEXT"))
		preds = append(preds, sqlf.Sprintf("changesets.id = %s", opts.ChangesetID))
	}

	switch opts.State {
	case batches.BatchChangeStateOpen:
		preds = append(preds, sqlf.Sprintf("campaigns.closed_at IS NULL"))
	case batches.BatchChangeStateClosed:
		preds = append(preds, sqlf.Sprintf("campaigns.closed_at IS NOT NULL"))
	}

	if opts.InitialApplierID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.initial_applier_id = %d", opts.InitialApplierID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(countCampaignsQueryFmtstr, sqlf.Join(joins, "\n"), sqlf.Join(preds, "\n AND "))
}

// GetCampaignOpts captures the query options needed for getting a Campaign
type GetCampaignOpts struct {
	ID int64

	NamespaceUserID int32
	NamespaceOrgID  int32

	CampaignSpecID int64
	Name           string
}

// GetCampaign gets a campaign matching the given options.
func (s *Store) GetCampaign(ctx context.Context, opts GetCampaignOpts) (*batches.BatchChange, error) {
	q := getCampaignQuery(&opts)

	var c batches.BatchChange
	err := s.query(ctx, q, func(sc scanner) error {
		return scanCampaign(&c, sc)
	})
	if err != nil {
		return nil, err
	}

	if c.ID == 0 {
		return nil, ErrNoResults
	}

	return &c, nil
}

var getCampaignsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:GetCampaign
SELECT %s FROM campaigns
LEFT JOIN users namespace_user ON campaigns.namespace_user_id = namespace_user.id
LEFT JOIN orgs  namespace_org  ON campaigns.namespace_org_id = namespace_org.id
WHERE %s
LIMIT 1
`

func getCampaignQuery(opts *GetCampaignOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}
	if opts.ID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.id = %s", opts.ID))
	}

	if opts.CampaignSpecID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.campaign_spec_id = %s", opts.CampaignSpecID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if opts.Name != "" {
		preds = append(preds, sqlf.Sprintf("campaigns.name = %s", opts.Name))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(
		getCampaignsQueryFmtstr,
		sqlf.Join(batchChangeColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

type GetCampaignDiffStatOpts struct {
	CampaignID int64
}

func (s *Store) GetCampaignDiffStat(ctx context.Context, opts GetCampaignDiffStatOpts) (*diff.Stat, error) {
	authzConds, err := database.AuthzQueryConds(ctx, s.Handle().DB())
	if err != nil {
		return nil, errors.Wrap(err, "GetCampaignDiffStat generating authz query conds")
	}
	q := getCampaignDiffStatQuery(opts, authzConds)

	var diffStat diff.Stat
	err = s.query(ctx, q, func(sc scanner) error {
		return sc.Scan(&diffStat.Added, &diffStat.Changed, &diffStat.Deleted)
	})
	if err != nil {
		return nil, err
	}

	return &diffStat, nil
}

var getCampaignDiffStatQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:GetCampaignDiffStat
SELECT
	COALESCE(SUM(diff_stat_added), 0) AS added,
	COALESCE(SUM(diff_stat_changed), 0) AS changed,
	COALESCE(SUM(diff_stat_deleted), 0) AS deleted
FROM
	changesets
INNER JOIN repo ON changesets.repo_id = repo.id
WHERE
	changesets.campaign_ids ? %s AND
	repo.deleted_at IS NULL AND
	-- authz conditions:
	%s
`

func getCampaignDiffStatQuery(opts GetCampaignDiffStatOpts, authzConds *sqlf.Query) *sqlf.Query {
	return sqlf.Sprintf(getCampaignDiffStatQueryFmtstr, strconv.Itoa(int(opts.CampaignID)), authzConds)
}

// ListCampaignsOpts captures the query options needed for
// listing batches.
type ListCampaignsOpts struct {
	LimitOpts
	ChangesetID int64
	Cursor      int64
	State       batches.BatchChangeState

	InitialApplierID int32

	NamespaceUserID int32
	NamespaceOrgID  int32
}

// ListCampaigns lists Campaigns with the given filters.
func (s *Store) ListCampaigns(ctx context.Context, opts ListCampaignsOpts) (cs []*batches.BatchChange, next int64, err error) {
	q := listCampaignsQuery(&opts)

	cs = make([]*batches.BatchChange, 0, opts.DBLimit())
	err = s.query(ctx, q, func(sc scanner) error {
		var c batches.BatchChange
		if err := scanCampaign(&c, sc); err != nil {
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

var listCampaignsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:ListCampaigns
SELECT %s FROM campaigns
%s
WHERE %s
ORDER BY id DESC
`

func listCampaignsQuery(opts *ListCampaignsOpts) *sqlf.Query {
	joins := []*sqlf.Query{
		sqlf.Sprintf("LEFT JOIN users namespace_user ON campaigns.namespace_user_id = namespace_user.id"),
		sqlf.Sprintf("LEFT JOIN orgs namespace_org ON campaigns.namespace_org_id = namespace_org.id"),
	}
	preds := []*sqlf.Query{
		sqlf.Sprintf("namespace_user.deleted_at IS NULL"),
		sqlf.Sprintf("namespace_org.deleted_at IS NULL"),
	}

	if opts.Cursor != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.id <= %s", opts.Cursor))
	}

	if opts.ChangesetID != 0 {
		joins = append(joins, sqlf.Sprintf("INNER JOIN changesets ON changesets.campaign_ids ? campaigns.id::TEXT"))
		preds = append(preds, sqlf.Sprintf("changesets.id = %s", opts.ChangesetID))
	}

	switch opts.State {
	case batches.BatchChangeStateOpen:
		preds = append(preds, sqlf.Sprintf("campaigns.closed_at IS NULL"))
	case batches.BatchChangeStateClosed:
		preds = append(preds, sqlf.Sprintf("campaigns.closed_at IS NOT NULL"))
	}

	if opts.InitialApplierID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.initial_applier_id = %d", opts.InitialApplierID))
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_user_id = %s", opts.NamespaceUserID))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf("campaigns.namespace_org_id = %s", opts.NamespaceOrgID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(
		listCampaignsQueryFmtstr+opts.LimitOpts.ToDB(),
		sqlf.Join(batchChangeColumns, ", "),
		sqlf.Join(joins, "\n"),
		sqlf.Join(preds, "\n AND "),
	)
}

func scanCampaign(c *batches.BatchChange, s scanner) error {
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
		&c.CampaignSpecID,
	)
}
