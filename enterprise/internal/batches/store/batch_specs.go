package store

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/dineshappavoo/basex"
	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// batchSpecColumns are used by the batchSpec related Store methods to insert,
// update and query batches.
var batchSpecColumns = []*sqlf.Query{
	sqlf.Sprintf("batch_specs.id"),
	sqlf.Sprintf("batch_specs.rand_id"),
	sqlf.Sprintf("batch_specs.raw_spec"),
	sqlf.Sprintf("batch_specs.spec"),
	sqlf.Sprintf("batch_specs.namespace_user_id"),
	sqlf.Sprintf("batch_specs.namespace_org_id"),
	sqlf.Sprintf("batch_specs.user_id"),
	sqlf.Sprintf("batch_specs.created_at"),
	sqlf.Sprintf("batch_specs.updated_at"),
}

// batchSpecInsertColumns is the list of batch_specs columns that are
// modified when updating/inserting batch specs.
var batchSpecInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("rand_id"),
	sqlf.Sprintf("raw_spec"),
	sqlf.Sprintf("spec"),
	sqlf.Sprintf("namespace_user_id"),
	sqlf.Sprintf("namespace_org_id"),
	sqlf.Sprintf("user_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
}

const batchSpecInsertColsFmt = `(%s, %s, %s, %s, %s, %s, %s, %s)`

// CreateBatchSpec creates the given BatchSpec.
func (s *Store) CreateBatchSpec(ctx context.Context, c *batches.BatchSpec) error {
	q, err := s.createBatchSpecQuery(c)
	if err != nil {
		return err
	}
	return s.query(ctx, q, func(sc scanner) error { return scanBatchSpec(c, sc) })
}

var createBatchSpecQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:CreateBatchSpec
INSERT INTO batch_specs (%s)
VALUES ` + batchSpecInsertColsFmt + `
RETURNING %s`

func (s *Store) createBatchSpecQuery(c *batches.BatchSpec) (*sqlf.Query, error) {
	spec, err := jsonbColumn(c.Spec)
	if err != nil {
		return nil, err
	}

	if c.CreatedAt.IsZero() {
		c.CreatedAt = s.now()
	}

	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	if c.RandID == "" {
		if c.RandID, err = basex.Encode(strconv.Itoa(seededRand.Int())); err != nil {
			return nil, errors.Wrap(err, "creating RandID failed")
		}
	}

	return sqlf.Sprintf(
		createBatchSpecQueryFmtstr,
		sqlf.Join(batchSpecInsertColumns, ", "),
		c.RandID,
		c.RawSpec,
		spec,
		nullInt32Column(c.NamespaceUserID),
		nullInt32Column(c.NamespaceOrgID),
		nullInt32Column(c.UserID),
		c.CreatedAt,
		c.UpdatedAt,
		sqlf.Join(batchSpecColumns, ", "),
	), nil
}

// UpdateBatchSpec updates the given BatchSpec.
func (s *Store) UpdateBatchSpec(ctx context.Context, c *batches.BatchSpec) error {
	q, err := s.updateBatchSpecQuery(c)
	if err != nil {
		return err
	}

	return s.query(ctx, q, func(sc scanner) error {
		return scanBatchSpec(c, sc)
	})
}

var updateBatchSpecQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:UpdateBatchSpec
UPDATE batch_specs
SET (%s) = ` + batchSpecInsertColsFmt + `
WHERE id = %s
RETURNING %s`

func (s *Store) updateBatchSpecQuery(c *batches.BatchSpec) (*sqlf.Query, error) {
	spec, err := jsonbColumn(c.Spec)
	if err != nil {
		return nil, err
	}

	c.UpdatedAt = s.now()

	return sqlf.Sprintf(
		updateBatchSpecQueryFmtstr,
		sqlf.Join(batchSpecInsertColumns, ", "),
		c.RandID,
		c.RawSpec,
		spec,
		nullInt32Column(c.NamespaceUserID),
		nullInt32Column(c.NamespaceOrgID),
		nullInt32Column(c.UserID),
		c.CreatedAt,
		c.UpdatedAt,
		c.ID,
		sqlf.Join(batchSpecColumns, ", "),
	), nil
}

// DeleteBatchSpec deletes the BatchSpec with the given ID.
func (s *Store) DeleteBatchSpec(ctx context.Context, id int64) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(deleteBatchSpecQueryFmtstr, id))
}

var deleteBatchSpecQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:DeleteBatchSpec
DELETE FROM batch_specs WHERE id = %s
`

// CountBatchSpecs returns the number of code mods in the database.
func (s *Store) CountBatchSpecs(ctx context.Context) (int, error) {
	return s.queryCount(ctx, sqlf.Sprintf(countBatchSpecsQueryFmtstr))
}

var countBatchSpecsQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:CountBatchSpecs
SELECT COUNT(id)
FROM batch_specs
`

// GetBatchSpecOpts captures the query options needed for getting a BatchSpec
type GetBatchSpecOpts struct {
	ID     int64
	RandID string
}

// GetBatchSpec gets a BatchSpec matching the given options.
func (s *Store) GetBatchSpec(ctx context.Context, opts GetBatchSpecOpts) (*batches.BatchSpec, error) {
	q := getBatchSpecQuery(&opts)

	var c batches.BatchSpec
	err := s.query(ctx, q, func(sc scanner) (err error) {
		return scanBatchSpec(&c, sc)
	})
	if err != nil {
		return nil, err
	}

	if c.ID == 0 {
		return nil, ErrNoResults
	}

	return &c, nil
}

var getBatchSpecsQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:GetBatchSpec
SELECT %s FROM batch_specs
WHERE %s
LIMIT 1
`

func getBatchSpecQuery(opts *GetBatchSpecOpts) *sqlf.Query {
	var preds []*sqlf.Query
	if opts.ID != 0 {
		preds = append(preds, sqlf.Sprintf("id = %s", opts.ID))
	}

	if opts.RandID != "" {
		preds = append(preds, sqlf.Sprintf("rand_id = %s", opts.RandID))
	}

	if len(preds) == 0 {
		preds = append(preds, sqlf.Sprintf("TRUE"))
	}

	return sqlf.Sprintf(
		getBatchSpecsQueryFmtstr,
		sqlf.Join(batchSpecColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

// GetNewestBatchSpecOpts captures the query options needed to get the latest
// batch spec for the given parameters. One of the namespace fields and all
// the others must be defined.
type GetNewestBatchSpecOpts struct {
	NamespaceUserID int32
	NamespaceOrgID  int32
	UserID          int32
	Name            string
}

// GetNewestBatchSpec returns the newest batch spec that matches the given
// options.
func (s *Store) GetNewestBatchSpec(ctx context.Context, opts GetNewestBatchSpecOpts) (*batches.BatchSpec, error) {
	q := getNewestBatchSpecQuery(&opts)

	var c batches.BatchSpec
	err := s.query(ctx, q, func(sc scanner) (err error) {
		return scanBatchSpec(&c, sc)
	})
	if err != nil {
		return nil, err
	}

	if c.ID == 0 {
		return nil, ErrNoResults
	}

	return &c, nil
}

const getNewestBatchSpecQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:GetNewestBatchSpec
SELECT %s FROM batch_specs
WHERE %s
ORDER BY id DESC
LIMIT 1
`

func getNewestBatchSpecQuery(opts *GetNewestBatchSpecOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("user_id = %s", opts.UserID),
		sqlf.Sprintf("spec->>'name' = %s", opts.Name),
	}

	if opts.NamespaceUserID != 0 {
		preds = append(preds, sqlf.Sprintf(
			"namespace_user_id = %s",
			opts.NamespaceUserID,
		))
	}

	if opts.NamespaceOrgID != 0 {
		preds = append(preds, sqlf.Sprintf(
			"namespace_org_id = %s",
			opts.NamespaceOrgID,
		))
	}

	return sqlf.Sprintf(
		getNewestBatchSpecQueryFmtstr,
		sqlf.Join(batchSpecColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

// ListBatchSpecsOpts captures the query options needed for
// listing batch specs.
type ListBatchSpecsOpts struct {
	LimitOpts
	Cursor int64
}

// ListBatchSpecs lists BatchSpecs with the given filters.
func (s *Store) ListBatchSpecs(ctx context.Context, opts ListBatchSpecsOpts) (cs []*batches.BatchSpec, next int64, err error) {
	q := listBatchSpecsQuery(&opts)

	cs = make([]*batches.BatchSpec, 0, opts.DBLimit())
	err = s.query(ctx, q, func(sc scanner) error {
		var c batches.BatchSpec
		if err := scanBatchSpec(&c, sc); err != nil {
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

var listBatchSpecsQueryFmtstr = `
-- source: enterprise/internal/batches/store/batch_specs.go:ListBatchSpecs
SELECT %s FROM batch_specs
WHERE %s
ORDER BY id ASC
`

func listBatchSpecsQuery(opts *ListBatchSpecsOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("id >= %s", opts.Cursor),
	}

	return sqlf.Sprintf(
		listBatchSpecsQueryFmtstr+opts.LimitOpts.ToDB(),
		sqlf.Join(batchSpecColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

// DeleteExpiredBatchSpecs deletes BatchSpecs that have not been attached
// to a Batch change within BatchSpecTTL.
func (s *Store) DeleteExpiredBatchSpecs(ctx context.Context) error {
	expirationTime := s.now().Add(-batches.BatchSpecTTL)
	q := sqlf.Sprintf(deleteExpiredBatchSpecsQueryFmtstr, expirationTime)

	return s.Store.Exec(ctx, q)
}

var deleteExpiredBatchSpecsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:DeleteExpiredBatchSpecs
DELETE FROM
  batch_specs
WHERE
  created_at < %s
AND
NOT EXISTS (
  SELECT 1 FROM batch_changes WHERE batch_spec_id = batch_specs.id
)
AND NOT EXISTS (
  SELECT 1 FROM changeset_specs WHERE batch_spec_id = batch_specs.id
);
`

func scanBatchSpec(c *batches.BatchSpec, s scanner) error {
	var spec json.RawMessage

	err := s.Scan(
		&c.ID,
		&c.RandID,
		&c.RawSpec,
		&spec,
		&dbutil.NullInt32{N: &c.NamespaceUserID},
		&dbutil.NullInt32{N: &c.NamespaceOrgID},
		&dbutil.NullInt32{N: &c.UserID},
		&c.CreatedAt,
		&c.UpdatedAt,
	)

	if err != nil {
		return errors.Wrap(err, "scanning batch spec")
	}

	if err = json.Unmarshal(spec, &c.Spec); err != nil {
		return errors.Wrap(err, "scanBatchSpec: failed to unmarshal spec")
	}

	return nil
}
