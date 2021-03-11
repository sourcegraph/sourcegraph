package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/search"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

// ChangesetColumns are used by by the changeset related Store methods and by
// workerutil.Worker to load changesets from the database for processing by
// the reconciler.
var ChangesetColumns = []*sqlf.Query{
	sqlf.Sprintf("changesets.id"),
	sqlf.Sprintf("changesets.repo_id"),
	sqlf.Sprintf("changesets.created_at"),
	sqlf.Sprintf("changesets.updated_at"),
	sqlf.Sprintf("changesets.metadata"),
	sqlf.Sprintf("changesets.batch_change_ids"),
	sqlf.Sprintf("changesets.external_id"),
	sqlf.Sprintf("changesets.external_service_type"),
	sqlf.Sprintf("changesets.external_branch"),
	sqlf.Sprintf("changesets.external_deleted_at"),
	sqlf.Sprintf("changesets.external_updated_at"),
	sqlf.Sprintf("changesets.external_state"),
	sqlf.Sprintf("changesets.external_review_state"),
	sqlf.Sprintf("changesets.external_check_state"),
	sqlf.Sprintf("changesets.diff_stat_added"),
	sqlf.Sprintf("changesets.diff_stat_changed"),
	sqlf.Sprintf("changesets.diff_stat_deleted"),
	sqlf.Sprintf("changesets.sync_state"),
	sqlf.Sprintf("changesets.owned_by_batch_change_id"),
	sqlf.Sprintf("changesets.current_spec_id"),
	sqlf.Sprintf("changesets.previous_spec_id"),
	sqlf.Sprintf("changesets.publication_state"),
	sqlf.Sprintf("changesets.reconciler_state"),
	sqlf.Sprintf("changesets.failure_message"),
	sqlf.Sprintf("changesets.started_at"),
	sqlf.Sprintf("changesets.finished_at"),
	sqlf.Sprintf("changesets.process_after"),
	sqlf.Sprintf("changesets.num_resets"),
	sqlf.Sprintf("changesets.num_failures"),
	sqlf.Sprintf("changesets.closing"),
	sqlf.Sprintf("changesets.syncer_error"),
}

// changesetInsertColumns is the list of changeset columns that are modified in
// CreateChangeset and UpdateChangeset.
var changesetInsertColumns = []*sqlf.Query{
	sqlf.Sprintf("repo_id"),
	sqlf.Sprintf("created_at"),
	sqlf.Sprintf("updated_at"),
	sqlf.Sprintf("metadata"),
	sqlf.Sprintf("batch_change_ids"),
	sqlf.Sprintf("external_id"),
	sqlf.Sprintf("external_service_type"),
	sqlf.Sprintf("external_branch"),
	sqlf.Sprintf("external_deleted_at"),
	sqlf.Sprintf("external_updated_at"),
	sqlf.Sprintf("external_state"),
	sqlf.Sprintf("external_review_state"),
	sqlf.Sprintf("external_check_state"),
	sqlf.Sprintf("diff_stat_added"),
	sqlf.Sprintf("diff_stat_changed"),
	sqlf.Sprintf("diff_stat_deleted"),
	sqlf.Sprintf("sync_state"),
	sqlf.Sprintf("owned_by_batch_change_id"),
	sqlf.Sprintf("current_spec_id"),
	sqlf.Sprintf("previous_spec_id"),
	sqlf.Sprintf("publication_state"),
	sqlf.Sprintf("reconciler_state"),
	sqlf.Sprintf("failure_message"),
	sqlf.Sprintf("started_at"),
	sqlf.Sprintf("finished_at"),
	sqlf.Sprintf("process_after"),
	sqlf.Sprintf("num_resets"),
	sqlf.Sprintf("num_failures"),
	sqlf.Sprintf("closing"),
	sqlf.Sprintf("syncer_error"),
}

func (s *Store) changesetWriteQuery(q string, includeID bool, c *batches.Changeset) (*sqlf.Query, error) {
	metadata, err := jsonbColumn(c.Metadata)
	if err != nil {
		return nil, err
	}

	assocsAsMap := make(map[int64]batches.BatchChangeAssoc, len(c.BatchChanges))
	for _, assoc := range c.BatchChanges {
		assocsAsMap[assoc.BatchChangeID] = assoc
	}

	batchChanges, err := json.Marshal(assocsAsMap)
	if err != nil {
		return nil, err
	}

	syncState, err := json.Marshal(c.SyncState)
	if err != nil {
		return nil, err
	}

	vars := []interface{}{
		sqlf.Join(changesetInsertColumns, ", "),
		c.RepoID,
		c.CreatedAt,
		c.UpdatedAt,
		metadata,
		batchChanges,
		nullStringColumn(c.ExternalID),
		c.ExternalServiceType,
		nullStringColumn(c.ExternalBranch),
		nullTimeColumn(c.ExternalDeletedAt),
		nullTimeColumn(c.ExternalUpdatedAt),
		nullStringColumn(string(c.ExternalState)),
		nullStringColumn(string(c.ExternalReviewState)),
		nullStringColumn(string(c.ExternalCheckState)),
		c.DiffStatAdded,
		c.DiffStatChanged,
		c.DiffStatDeleted,
		syncState,
		nullInt64Column(c.OwnedByBatchChangeID),
		nullInt64Column(c.CurrentSpecID),
		nullInt64Column(c.PreviousSpecID),
		c.PublicationState,
		c.ReconcilerState.ToDB(),
		c.FailureMessage,
		nullTimeColumn(c.StartedAt),
		nullTimeColumn(c.FinishedAt),
		nullTimeColumn(c.ProcessAfter),
		c.NumResets,
		c.NumFailures,
		c.Closing,
		c.SyncErrorMessage,
	}

	if includeID {
		vars = append(vars, c.ID)
	}

	vars = append(vars, sqlf.Join(ChangesetColumns, ", "))

	return sqlf.Sprintf(q, vars...), nil
}

// UpsertChangeset creates or updates the given Changeset.
func (s *Store) UpsertChangeset(ctx context.Context, c *batches.Changeset) error {
	if c.ID == 0 {
		return s.CreateChangeset(ctx, c)
	}
	return s.UpdateChangeset(ctx, c)
}

// CreateChangeset creates the given Changeset.
func (s *Store) CreateChangeset(ctx context.Context, c *batches.Changeset) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = s.now()
	}

	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}

	q, err := s.changesetWriteQuery(createChangesetQueryFmtstr, false, c)
	if err != nil {
		return err
	}

	return s.query(ctx, q, func(sc scanner) error { return scanChangeset(c, sc) })
}

var createChangesetQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CreateChangeset
INSERT INTO changesets (%s)
VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
RETURNING %s
`

// DeleteChangeset deletes the Changeset with the given ID.
func (s *Store) DeleteChangeset(ctx context.Context, id int64) error {
	return s.Store.Exec(ctx, sqlf.Sprintf(deleteChangesetQueryFmtstr, id))
}

var deleteChangesetQueryFmtstr = `
DELETE FROM changesets WHERE id = %s
`

// CountChangesetsOpts captures the query options needed for
// counting changesets.
type CountChangesetsOpts struct {
	BatchChangeID        int64
	ExternalState        *batches.ChangesetExternalState
	ExternalReviewState  *batches.ChangesetReviewState
	ExternalCheckState   *batches.ChangesetCheckState
	ReconcilerStates     []batches.ReconcilerState
	OwnedByBatchChangeID int64
}

// CountChangesets returns the number of changesets in the database.
func (s *Store) CountChangesets(ctx context.Context, opts CountChangesetsOpts) (int, error) {
	return s.queryCount(ctx, countChangesetsQuery(&opts))
}

var countChangesetsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:CountChangesets
SELECT COUNT(changesets.id)
FROM changesets
INNER JOIN repo ON repo.id = changesets.repo_id
WHERE %s
`

func countChangesetsQuery(opts *CountChangesetsOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}
	if opts.BatchChangeID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.batch_change_ids ? %s", strconv.Itoa(int(opts.BatchChangeID))))
	}

	if opts.ExternalState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_state = %s", *opts.ExternalState))
	}
	if opts.ExternalReviewState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_review_state = %s", *opts.ExternalReviewState))
	}
	if opts.ExternalCheckState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_check_state = %s", *opts.ExternalCheckState))
	}
	if len(opts.ReconcilerStates) != 0 {
		states := make([]*sqlf.Query, len(opts.ReconcilerStates))
		for i, reconcilerState := range opts.ReconcilerStates {
			states[i] = sqlf.Sprintf("%s", reconcilerState.ToDB())
		}
		preds = append(preds, sqlf.Sprintf("changesets.reconciler_state IN (%s)", sqlf.Join(states, ",")))
	}
	if opts.OwnedByBatchChangeID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.owned_by_batch_change_id = %s", opts.OwnedByBatchChangeID))
	}

	return sqlf.Sprintf(countChangesetsQueryFmtstr, sqlf.Join(preds, "\n AND "))
}

// GetChangesetByID is a convenience method if only the ID needs to be passed in. It's also used for abstraction in
// the testing package.
func (s *Store) GetChangesetByID(ctx context.Context, id int64) (*batches.Changeset, error) {
	return s.GetChangeset(ctx, GetChangesetOpts{ID: id})
}

// GetChangesetOpts captures the query options needed for getting a Changeset
type GetChangesetOpts struct {
	ID                  int64
	RepoID              api.RepoID
	ExternalID          string
	ExternalServiceType string
	ExternalBranch      string
	ReconcilerState     batches.ReconcilerState
	PublicationState    batches.ChangesetPublicationState
}

// GetChangeset gets a changeset matching the given options.
func (s *Store) GetChangeset(ctx context.Context, opts GetChangesetOpts) (*batches.Changeset, error) {
	q := getChangesetQuery(&opts)

	var c batches.Changeset
	err := s.query(ctx, q, func(sc scanner) error { return scanChangeset(&c, sc) })
	if err != nil {
		return nil, err
	}

	if c.ID == 0 {
		return nil, ErrNoResults
	}

	return &c, nil
}

var getChangesetsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:GetChangeset
SELECT %s FROM changesets
INNER JOIN repo ON repo.id = changesets.repo_id
WHERE %s
LIMIT 1
`

func getChangesetQuery(opts *GetChangesetOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}
	if opts.ID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.id = %s", opts.ID))
	}

	if opts.RepoID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.repo_id = %s", opts.RepoID))
	}

	if opts.ExternalID != "" && opts.ExternalServiceType != "" {
		preds = append(preds,
			sqlf.Sprintf("changesets.external_id = %s", opts.ExternalID),
			sqlf.Sprintf("changesets.external_service_type = %s", opts.ExternalServiceType),
		)
	}
	if opts.ExternalBranch != "" {
		preds = append(preds, sqlf.Sprintf("changesets.external_branch = %s", opts.ExternalBranch))
	}
	if opts.ReconcilerState != "" {
		preds = append(preds, sqlf.Sprintf("changesets.reconciler_state = %s", opts.ReconcilerState.ToDB()))
	}
	if opts.PublicationState != "" {
		preds = append(preds, sqlf.Sprintf("changesets.publication_state = %s", opts.PublicationState))
	}

	return sqlf.Sprintf(
		getChangesetsQueryFmtstr,
		sqlf.Join(ChangesetColumns, ", "),
		sqlf.Join(preds, "\n AND "),
	)
}

type ListChangesetSyncDataOpts struct {
	// Return only the supplied changesets. If empty, all changesets are returned
	ChangesetIDs []int64

	ExternalServiceID string
}

// ListChangesetSyncData returns sync data on all non-externally-deleted changesets
// that are part of at least one open batch change.
func (s *Store) ListChangesetSyncData(ctx context.Context, opts ListChangesetSyncDataOpts) ([]*batches.ChangesetSyncData, error) {
	q := listChangesetSyncDataQuery(opts)
	results := make([]*batches.ChangesetSyncData, 0)
	err := s.query(ctx, q, func(sc scanner) (err error) {
		var h batches.ChangesetSyncData
		if err := scanChangesetSyncData(&h, sc); err != nil {
			return err
		}
		results = append(results, &h)
		return err
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func scanChangesetSyncData(h *batches.ChangesetSyncData, s scanner) error {
	return s.Scan(
		&h.ChangesetID,
		&h.UpdatedAt,
		&dbutil.NullTime{Time: &h.LatestEvent},
		&dbutil.NullTime{Time: &h.ExternalUpdatedAt},
		&h.RepoExternalServiceID,
	)
}

const listChangesetSyncDataQueryFmtstr = `
-- source: enterprise/internal/batches/store_changesets.go:ListChangesetSyncData
SELECT changesets.id,
	changesets.updated_at,
	max(ce.updated_at) AS latest_event,
	changesets.external_updated_at,
	r.external_service_id
FROM changesets
LEFT JOIN changeset_events ce ON changesets.id = ce.changeset_id
JOIN batch_changes ON changesets.batch_change_ids ? batch_changes.id::TEXT
JOIN repo r ON changesets.repo_id = r.id
WHERE %s
GROUP BY changesets.id, r.id
ORDER BY changesets.id ASC
`

func listChangesetSyncDataQuery(opts ListChangesetSyncDataOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("batch_changes.closed_at IS NULL"),
		sqlf.Sprintf("r.deleted_at IS NULL"),
		sqlf.Sprintf("changesets.publication_state = %s", batches.ChangesetPublicationStatePublished),
		sqlf.Sprintf("changesets.reconciler_state = %s", batches.ReconcilerStateCompleted.ToDB()),
	}
	if len(opts.ChangesetIDs) > 0 {
		ids := make([]*sqlf.Query, 0, len(opts.ChangesetIDs))
		for _, id := range opts.ChangesetIDs {
			if id != 0 {
				ids = append(ids, sqlf.Sprintf("%d", id))
			}
		}
		preds = append(preds, sqlf.Sprintf("changesets.id IN (%s)", sqlf.Join(ids, ",")))
	}

	if opts.ExternalServiceID != "" {
		preds = append(preds, sqlf.Sprintf("r.external_service_id = %s", opts.ExternalServiceID))
	}

	return sqlf.Sprintf(listChangesetSyncDataQueryFmtstr, sqlf.Join(preds, "\n AND"))
}

// ListChangesetsOpts captures the query options needed for listing changesets.
//
// Note that TextSearch is potentially expensive, and should only be specified
// in conjunction with at least one other option (most likely, BatchChangeID).
type ListChangesetsOpts struct {
	LimitOpts
	Cursor               int64
	BatchChangeID        int64
	IDs                  []int64
	WithoutDeleted       bool
	PublicationState     *batches.ChangesetPublicationState
	ReconcilerStates     []batches.ReconcilerState
	ExternalState        *batches.ChangesetExternalState
	ExternalReviewState  *batches.ChangesetReviewState
	ExternalCheckState   *batches.ChangesetCheckState
	OwnedByBatchChangeID int64
	ExternalServiceID    string
	TextSearch           []search.TextSearchTerm
}

// ListChangesets lists Changesets with the given filters.
func (s *Store) ListChangesets(ctx context.Context, opts ListChangesetsOpts) (cs batches.Changesets, next int64, err error) {
	q := listChangesetsQuery(&opts)

	cs = make([]*batches.Changeset, 0, opts.DBLimit())
	err = s.query(ctx, q, func(sc scanner) (err error) {
		var c batches.Changeset
		if err = scanChangeset(&c, sc); err != nil {
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

var listChangesetsQueryFmtstr = `
-- source: enterprise/internal/batches/store.go:ListChangesets
SELECT %s FROM changesets
INNER JOIN repo ON repo.id = changesets.repo_id
%s -- optional LEFT JOIN to changeset_specs if required
WHERE %s
ORDER BY id ASC
`

func listChangesetsQuery(opts *ListChangesetsOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("changesets.id >= %s", opts.Cursor),
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}

	if opts.BatchChangeID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.batch_change_ids ? %s", strconv.Itoa(int(opts.BatchChangeID))))
	}

	if len(opts.IDs) > 0 {
		ids := make([]*sqlf.Query, 0, len(opts.IDs))
		for _, id := range opts.IDs {
			if id != 0 {
				ids = append(ids, sqlf.Sprintf("%d", id))
			}
		}
		preds = append(preds, sqlf.Sprintf("changesets.id IN (%s)", sqlf.Join(ids, ",")))
	}

	if opts.WithoutDeleted {
		preds = append(preds, sqlf.Sprintf("changesets.external_deleted_at IS NULL"))
	}

	if opts.PublicationState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.publication_state = %s", *opts.PublicationState))
	}
	if len(opts.ReconcilerStates) != 0 {
		states := make([]*sqlf.Query, len(opts.ReconcilerStates))
		for i, reconcilerState := range opts.ReconcilerStates {
			states[i] = sqlf.Sprintf("%s", reconcilerState.ToDB())
		}
		preds = append(preds, sqlf.Sprintf("changesets.reconciler_state IN (%s)", sqlf.Join(states, ",")))
	}
	if opts.ExternalState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_state = %s", *opts.ExternalState))
	}
	if opts.ExternalReviewState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_review_state = %s", *opts.ExternalReviewState))
	}
	if opts.ExternalCheckState != nil {
		preds = append(preds, sqlf.Sprintf("changesets.external_check_state = %s", *opts.ExternalCheckState))
	}
	if opts.OwnedByBatchChangeID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.owned_by_batch_change_id = %s", opts.OwnedByBatchChangeID))
	}
	if opts.ExternalServiceID != "" {
		preds = append(preds, sqlf.Sprintf("repo.external_service_id = %s", opts.ExternalServiceID))
	}

	join := sqlf.Sprintf("")
	if len(opts.TextSearch) != 0 {
		// TextSearch predicates require changeset_specs to be joined into the
		// query as well.
		join = sqlf.Sprintf("LEFT JOIN changeset_specs ON changesets.current_spec_id = changeset_specs.id")

		for _, term := range opts.TextSearch {
			preds = append(preds, textSearchTermToClause(
				term,
				// The COALESCE() is required to handle the actual title on the
				// changeset, if it has been published or if it's tracked.
				// Unfortunately, the metadata field isn't standard, so we have
				// to get both variations that exist between the code hosts we
				// support.
				sqlf.Sprintf("COALESCE(changesets.metadata->>'Title', changesets.metadata->>'title', changeset_specs.spec->>'title')"),
				sqlf.Sprintf("repo.name"),
			))
		}
	}

	return sqlf.Sprintf(
		listChangesetsQueryFmtstr+opts.LimitOpts.ToDB(),
		sqlf.Join(ChangesetColumns, ", "),
		join,
		sqlf.Join(preds, "\n AND "),
	)
}

// UpdateChangeset updates the given Changeset.
func (s *Store) UpdateChangeset(ctx context.Context, cs *batches.Changeset) error {
	cs.UpdatedAt = s.now()

	q, err := s.changesetWriteQuery(updateChangesetQueryFmtstr, true, cs)
	if err != nil {
		return err
	}

	return s.query(ctx, q, func(sc scanner) (err error) {
		return scanChangeset(cs, sc)
	})
}

var updateChangesetQueryFmtstr = `
-- source: enterprise/internal/batches/store_changesets.go:UpdateChangeset
UPDATE changesets
SET (%s) = (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
WHERE id = %s
RETURNING
  %s
`

// GetChangesetExternalIDs allows us to find the external ids for pull requests based on
// a slice of head refs. We need this in order to match incoming webhooks to pull requests as
// the only information they provide is the remote branch
func (s *Store) GetChangesetExternalIDs(ctx context.Context, spec api.ExternalRepoSpec, refs []string) ([]string, error) {
	queryFmtString := `
	SELECT cs.external_id FROM changesets cs
	JOIN repo r ON cs.repo_id = r.id
	WHERE cs.external_service_type = %s
	AND cs.external_branch IN (%s)
	AND r.external_id = %s
	AND r.external_service_type = %s
	AND r.external_service_id = %s
	AND r.deleted_at IS NULL
	ORDER BY cs.id ASC;
	`

	inClause := make([]*sqlf.Query, 0, len(refs))
	for _, ref := range refs {
		if ref == "" {
			continue
		}
		inClause = append(inClause, sqlf.Sprintf("%s", ref))
	}

	q := sqlf.Sprintf(queryFmtString, spec.ServiceType, sqlf.Join(inClause, ","), spec.ID, spec.ServiceType, spec.ServiceID)
	return basestore.ScanStrings(s.Store.Query(ctx, q))
}

// CanceledChangesetFailureMessage is set on changesets as the FailureMessage
// by CancelQueuedBatchChangeChangesets which is called at the beginning of
// ApplyBatchChange to stop enqueued changesets being processed while we're
// applying the new batch spec.
var CanceledChangesetFailureMessage = "Canceled"

func (s *Store) CancelQueuedBatchChangeChangesets(ctx context.Context, batchChangeID int64) error {
	// Note that we don't cancel queued "syncing" changesets, since their
	// owned_by_batch_change_id is not set. That's on purpose. It's okay if they're
	// being processed after this, since they only pull data and not create
	// changesets on the code hosts.
	q := sqlf.Sprintf(
		cancelQueuedBatchChangeChangesetsFmtstr,
		batchChangeID,
		CanceledChangesetFailureMessage,
	)
	return s.Store.Exec(ctx, q)
}

const cancelQueuedBatchChangeChangesetsFmtstr = `
-- source: enterprise/internal/batches/store_changesets.go:CancelQueuedBatchChangeChangesets
WITH changeset_ids AS (
  SELECT id FROM changesets
  WHERE
    owned_by_batch_change_id = %s
  AND
    reconciler_state IN ('queued', 'processing', 'errored')
  FOR UPDATE
)
UPDATE
  changesets
SET
  reconciler_state = 'failed',
  failure_message = %s
WHERE id IN (SELECT id FROM changeset_ids);
`

// EnqueueChangesetsToClose updates all changesets that are owned by the given
// batch change to set their reconciler status to 'queued' and the Closing boolean
// to true.
//
// It does not update the changesets that are fully processed and already
// closed/merged.
//
// This method will *block* if some of the changesets are currently being processed.
func (s *Store) EnqueueChangesetsToClose(ctx context.Context, batchChangeID int64) error {
	q := sqlf.Sprintf(
		enqueueChangesetsToCloseFmtstr,
		batches.ReconcilerStateQueued.ToDB(),
		batchChangeID,
		batches.ChangesetPublicationStatePublished,
		batches.ChangesetExternalStateClosed,
		batches.ChangesetExternalStateMerged,
	)
	return s.Store.Exec(ctx, q)
}

const enqueueChangesetsToCloseFmtstr = `
-- source: enterprise/internal/batches/store_changesets.go:EnqueueChangesetsToClose
UPDATE
  changesets
SET
  reconciler_state = %s,
  failure_message = NULL,
  num_resets = 0,
  num_failures = 0,
  closing = TRUE,
  syncer_error = NULL
WHERE
  owned_by_batch_change_id = %d AND
  publication_state = %s AND
  NOT (
    reconciler_state = 'completed'
    AND
    (external_state = %s OR external_state = %s)
  )
`

func ScanFirstChangeset(rows *sql.Rows, err error) (*batches.Changeset, bool, error) {
	changesets, err := scanChangesets(rows, err)
	if err != nil || len(changesets) == 0 {
		return &batches.Changeset{}, false, err
	}
	return changesets[0], true, nil
}

func scanChangesets(rows *sql.Rows, queryErr error) ([]*batches.Changeset, error) {
	if queryErr != nil {
		return nil, queryErr
	}

	var cs []*batches.Changeset

	return cs, scanAll(rows, func(sc scanner) (err error) {
		var c batches.Changeset
		if err = scanChangeset(&c, sc); err != nil {
			return err
		}
		cs = append(cs, &c)
		return nil
	})
}

// jsonBatchChangeChangesetSet represents a "join table" set as a JSONB object
// where the keys are the ids and the values are json objects holding the properties.
// It implements the sql.Scanner interface so it can be used as a scan destination,
// similar to sql.NullString.
type jsonBatchChangeChangesetSet struct {
	Assocs *[]batches.BatchChangeAssoc
}

// Scan implements the Scanner interface.
func (n *jsonBatchChangeChangesetSet) Scan(value interface{}) error {
	m := make(map[int64]batches.BatchChangeAssoc)

	switch value := value.(type) {
	case nil:
	case []byte:
		if err := json.Unmarshal(value, &m); err != nil {
			return err
		}
	default:
		return fmt.Errorf("value is not []byte: %T", value)
	}

	if *n.Assocs == nil {
		*n.Assocs = make([]batches.BatchChangeAssoc, 0, len(m))
	} else {
		*n.Assocs = (*n.Assocs)[:0]
	}

	for id, assoc := range m {
		*n.Assocs = append(*n.Assocs, batches.BatchChangeAssoc{BatchChangeID: id, Detach: assoc.Detach})
	}

	return nil
}

// Value implements the driver Valuer interface.
func (n jsonBatchChangeChangesetSet) Value() (driver.Value, error) {
	if n.Assocs == nil {
		return nil, nil
	}
	return *n.Assocs, nil
}

func scanChangeset(t *batches.Changeset, s scanner) error {
	var metadata, syncState json.RawMessage

	var (
		externalState       string
		externalReviewState string
		externalCheckState  string
		failureMessage      string
		syncErrorMessage    string
		reconcilerState     string
	)
	err := s.Scan(
		&t.ID,
		&t.RepoID,
		&t.CreatedAt,
		&t.UpdatedAt,
		&metadata,
		&jsonBatchChangeChangesetSet{Assocs: &t.BatchChanges},
		&dbutil.NullString{S: &t.ExternalID},
		&t.ExternalServiceType,
		&dbutil.NullString{S: &t.ExternalBranch},
		&dbutil.NullTime{Time: &t.ExternalDeletedAt},
		&dbutil.NullTime{Time: &t.ExternalUpdatedAt},
		&dbutil.NullString{S: &externalState},
		&dbutil.NullString{S: &externalReviewState},
		&dbutil.NullString{S: &externalCheckState},
		&t.DiffStatAdded,
		&t.DiffStatChanged,
		&t.DiffStatDeleted,
		&syncState,
		&dbutil.NullInt64{N: &t.OwnedByBatchChangeID},
		&dbutil.NullInt64{N: &t.CurrentSpecID},
		&dbutil.NullInt64{N: &t.PreviousSpecID},
		&t.PublicationState,
		&reconcilerState,
		&dbutil.NullString{S: &failureMessage},
		&dbutil.NullTime{Time: &t.StartedAt},
		&dbutil.NullTime{Time: &t.FinishedAt},
		&dbutil.NullTime{Time: &t.ProcessAfter},
		&t.NumResets,
		&t.NumFailures,
		&t.Closing,
		&dbutil.NullString{S: &syncErrorMessage},
	)
	if err != nil {
		return errors.Wrap(err, "scanning changeset")
	}

	t.ExternalState = batches.ChangesetExternalState(externalState)
	t.ExternalReviewState = batches.ChangesetReviewState(externalReviewState)
	t.ExternalCheckState = batches.ChangesetCheckState(externalCheckState)
	if failureMessage != "" {
		t.FailureMessage = &failureMessage
	}
	if syncErrorMessage != "" {
		t.SyncErrorMessage = &syncErrorMessage
	}
	t.ReconcilerState = batches.ReconcilerState(strings.ToUpper(reconcilerState))

	switch t.ExternalServiceType {
	case extsvc.TypeGitHub:
		t.Metadata = new(github.PullRequest)
	case extsvc.TypeBitbucketServer:
		t.Metadata = new(bitbucketserver.PullRequest)
	case extsvc.TypeGitLab:
		t.Metadata = new(gitlab.MergeRequest)
	default:
		return errors.New("unknown external service type")
	}

	if err = json.Unmarshal(metadata, t.Metadata); err != nil {
		return errors.Wrapf(err, "scanChangeset: failed to unmarshal %q metadata", t.ExternalServiceType)
	}
	if err = json.Unmarshal(syncState, &t.SyncState); err != nil {
		return errors.Wrapf(err, "scanChangeset: failed to unmarshal sync state: %s", syncState)
	}

	return nil
}

// GetChangesetsStatsOpts captures the query options needed for
// retrieving changesets stats.
type GetChangesetsStatsOpts struct {
	BatchChangeID int64
}

// GetChangesetsStats returns statistics on all the changesets associated to the given batch change,
// or all changesets across the instance.
func (s *Store) GetChangesetsStats(ctx context.Context, opts GetChangesetsStatsOpts) (stats batches.ChangesetsStats, err error) {
	q := getChangesetsStatsQuery(opts)
	err = s.query(ctx, q, func(sc scanner) error {
		if err := sc.Scan(
			&stats.Total,
			&stats.Retrying,
			&stats.Failed,
			&stats.Processing,
			&stats.Unpublished,
			&stats.Closed,
			&stats.Draft,
			&stats.Merged,
			&stats.Open,
			&stats.Deleted,
		); err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return stats, err
	}
	return stats, nil
}

const getChangesetStatsFmtstr = `
-- source: enterprise/internal/batches/store_changesets.go:GetChangesetsStats
SELECT
	COUNT(*) AS total,
	COUNT(*) FILTER (WHERE changesets.reconciler_state = 'errored') AS retrying,
	COUNT(*) FILTER (WHERE changesets.reconciler_state = 'failed') AS failed,
	COUNT(*) FILTER (WHERE changesets.reconciler_state NOT IN ('failed', 'errored', 'completed')) AS processing,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'UNPUBLISHED' AND changesets.reconciler_state = 'completed') AS unpublished,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'PUBLISHED' AND changesets.reconciler_state = 'completed' AND changesets.external_state = 'CLOSED') AS closed,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'PUBLISHED' AND changesets.reconciler_state = 'completed' AND changesets.external_state = 'DRAFT') AS draft,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'PUBLISHED' AND changesets.reconciler_state = 'completed' AND changesets.external_state = 'MERGED') AS merged,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'PUBLISHED' AND changesets.reconciler_state = 'completed' AND changesets.external_state = 'OPEN') AS open,
	COUNT(*) FILTER (WHERE changesets.publication_state = 'PUBLISHED' AND changesets.reconciler_state = 'completed' AND changesets.external_state = 'DELETED') AS deleted
FROM changesets
INNER JOIN repo on repo.id = changesets.repo_id
WHERE
	%s
`

func getChangesetsStatsQuery(opts GetChangesetsStatsOpts) *sqlf.Query {
	preds := []*sqlf.Query{
		sqlf.Sprintf("repo.deleted_at IS NULL"),
	}
	if opts.BatchChangeID != 0 {
		preds = append(preds, sqlf.Sprintf("changesets.batch_change_ids ? %s", strconv.Itoa(int(opts.BatchChangeID))))
	}
	return sqlf.Sprintf(getChangesetStatsFmtstr, sqlf.Join(preds, " AND "))
}
