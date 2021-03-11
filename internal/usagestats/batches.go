package usagestats

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// GetBatchChangesUsageStatistics returns the current site's batch changes usage.
func GetBatchChangesUsageStatistics(ctx context.Context) (*types.BatchChangesUsageStatistics, error) {
	stats := types.BatchChangesUsageStatistics{}

	const batchChangesCountsQuery = `
SELECT
    COUNT(*)                                      AS batch_changes_count,
    COUNT(*) FILTER (WHERE closed_at IS NOT NULL) AS batch_changes_closed_count
FROM batch_changes;
`
	if err := dbconn.Global.QueryRowContext(ctx, batchChangesCountsQuery).Scan(
		&stats.BatchChangesCount,
		&stats.BatchChangesClosedCount,
	); err != nil {
		return nil, err
	}

	const changesetCountsQuery = `
SELECT
    COUNT(*)                        FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'UNPUBLISHED') AS action_changesets_unpublished,
    COUNT(*)                        FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED') AS action_changesets,
    COALESCE(SUM(diff_stat_added)   FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED'), 0) AS action_changesets_diff_stat_added_sum,
    COALESCE(SUM(diff_stat_changed) FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED'), 0) AS action_changesets_diff_stat_changed_sum,
    COALESCE(SUM(diff_stat_deleted) FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED'), 0) AS action_changesets_diff_stat_deleted_sum,
    COUNT(*)                        FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED' AND external_state = 'MERGED') AS action_changesets_merged,
    COALESCE(SUM(diff_stat_added)   FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED' AND external_state = 'MERGED'), 0) AS action_changesets_merged_diff_stat_added_sum,
    COALESCE(SUM(diff_stat_changed) FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED' AND external_state = 'MERGED'), 0) AS action_changesets_merged_diff_stat_changed_sum,
    COALESCE(SUM(diff_stat_deleted) FILTER (WHERE owned_by_batch_change_id IS NOT NULL AND publication_state = 'PUBLISHED' AND external_state = 'MERGED'), 0) AS action_changesets_merged_diff_stat_deleted_sum,
    COUNT(*) FILTER (WHERE owned_by_batch_change_id IS NULL) AS manual_changesets,
    COUNT(*) FILTER (WHERE owned_by_batch_change_id IS NULL AND external_state = 'MERGED') AS manual_changesets_merged
FROM changesets;
`
	if err := dbconn.Global.QueryRowContext(ctx, changesetCountsQuery).Scan(
		&stats.PublishedChangesetsUnpublishedCount,
		&stats.PublishedChangesetsCount,
		&stats.PublishedChangesetsDiffStatAddedSum,
		&stats.PublishedChangesetsDiffStatChangedSum,
		&stats.PublishedChangesetsDiffStatDeletedSum,
		&stats.PublishedChangesetsMergedCount,
		&stats.PublishedChangesetsMergedDiffStatAddedSum,
		&stats.PublishedChangesetsMergedDiffStatChangedSum,
		&stats.PublishedChangesetsMergedDiffStatDeletedSum,
		&stats.ImportedChangesetsCount,
		&stats.ImportedChangesetsMergedCount,
	); err != nil {
		return nil, err
	}

	const eventLogsCountsQuery = `
SELECT
    COUNT(*)                                                FILTER (WHERE name = 'BatchSpecCreated')                       AS batch_specs_created,
    COALESCE(SUM((argument->>'changeset_specs_count')::int) FILTER (WHERE name = 'BatchSpecCreated'), 0)                   AS changeset_specs_created_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewBatchChangeApplyPage')               AS view_batch_change_apply_page_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewBatchChangeDetailsPageAfterCreate')  AS view_batch_change_details_page_after_create_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewBatchChangeDetailsPageAfterUpdate')  AS view_batch_change_details_page_after_update_count
FROM event_logs
WHERE name IN ('BatchSpecCreated', 'ViewBatchChangeApplyPage', 'ViewBatchChangeDetailsPageAfterCreate', 'ViewBatchChangeDetailsPageAfterUpdate');
`

	if err := dbconn.Global.QueryRowContext(ctx, eventLogsCountsQuery).Scan(
		&stats.BatchSpecsCreatedCount,
		&stats.ChangesetSpecsCreatedCount,
		&stats.ViewBatchChangeApplyPageCount,
		&stats.ViewBatchChangeDetailsPageAfterCreateCount,
		&stats.ViewBatchChangeDetailsPageAfterUpdateCount,
	); err != nil {
		return nil, err
	}

	queryUniqueEventLogUsersCurrentMonth := func(events []*sqlf.Query) *sql.Row {
		q := sqlf.Sprintf(
			`SELECT COUNT(DISTINCT user_id) FROM event_logs WHERE name IN (%s) AND timestamp >= date_trunc('month', CURRENT_DATE);`,
			sqlf.Join(events, ","),
		)

		return dbconn.Global.QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	}

	var contributorEvents = []*sqlf.Query{
		sqlf.Sprintf("%q", "BatchSpecCreated"),
		sqlf.Sprintf("%q", "BatchChangeCreated"),
		sqlf.Sprintf("%q", "BatchChangeCreatedOrUpdated"),
		sqlf.Sprintf("%q", "BatchChangeClosed"),
		sqlf.Sprintf("%q", "BatchChangeDeleted"),
		sqlf.Sprintf("%q", "ViewBatchChangeApplyPage"),
	}

	if err := queryUniqueEventLogUsersCurrentMonth(contributorEvents).Scan(&stats.CurrentMonthContributorsCount); err != nil {
		return nil, err
	}

	var usersEvents = []*sqlf.Query{
		sqlf.Sprintf("%q", "BatchSpecCreated"),
		sqlf.Sprintf("%q", "BatchChangeCreated"),
		sqlf.Sprintf("%q", "BatchChangeCreatedOrUpdated"),
		sqlf.Sprintf("%q", "BatchChangeClosed"),
		sqlf.Sprintf("%q", "BatchChangeDeleted"),
		sqlf.Sprintf("%q", "ViewBatchChangeApplyPage"),
		sqlf.Sprintf("%q", "ViewBatchChangeDetailsPagePage"),
		sqlf.Sprintf("%q", "ViewBatchChangesListPage"),
	}

	if err := queryUniqueEventLogUsersCurrentMonth(usersEvents).Scan(&stats.CurrentMonthUsersCount); err != nil {
		return nil, err
	}

	const batchChangesCohortQuery = `
WITH
cohort_batch_changes as (
  SELECT
    date_trunc('week', batch_changes.created_at)::date AS creation_week,
    id
  FROM
    batch_changes
  WHERE
    created_at >= NOW() - (INTERVAL '12 months')
),
changeset_counts AS (
  SELECT
    cohort_batch_changes.creation_week,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id IS NULL OR changesets.owned_by_batch_change_id != cohort_batch_changes.id)  AS changesets_imported,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND publication_state = 'UNPUBLISHED')  AS changesets_unpublished,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND publication_state != 'UNPUBLISHED') AS changesets_published,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND external_state = 'OPEN') AS changesets_published_open,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND external_state = 'DRAFT') AS changesets_published_draft,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND external_state = 'MERGED') AS changesets_published_merged,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_batch_change_id = cohort_batch_changes.id AND external_state = 'CLOSED') AS changesets_published_closed
  FROM changesets
  JOIN cohort_batch_changes ON changesets.batch_change_ids ? cohort_batch_changes.id::text
  GROUP BY cohort_batch_changes.creation_week
),
batch_change_counts AS (
  SELECT
    date_trunc('week', batch_changes.created_at)::date      AS creation_week,
    COUNT(distinct id) FILTER (WHERE closed_at IS NOT NULL) AS closed,
    COUNT(distinct id) FILTER (WHERE closed_at IS NULL)     AS open
  FROM batch_changes
  WHERE
    created_at >= NOW() - (INTERVAL '12 months')
  GROUP BY date_trunc('week', batch_changes.created_at)::date
)
SELECT to_char(batch_change_counts.creation_week, 'yyyy-mm-dd')           AS creation_week,
       COALESCE(SUM(batch_change_counts.closed), 0)                       AS batch_changes_closed,
       COALESCE(SUM(batch_change_counts.open), 0)                         AS batch_changes_open,
       COALESCE(SUM(changeset_counts.changesets_imported), 0)         AS changesets_imported,
       COALESCE(SUM(changeset_counts.changesets_unpublished), 0)      AS changesets_unpublished,
       COALESCE(SUM(changeset_counts.changesets_published), 0)        AS changesets_published,
       COALESCE(SUM(changeset_counts.changesets_published_open), 0)   AS changesets_published_open,
       COALESCE(SUM(changeset_counts.changesets_published_draft), 0)  AS changesets_published_draft,
       COALESCE(SUM(changeset_counts.changesets_published_merged), 0) AS changesets_published_merged,
       COALESCE(SUM(changeset_counts.changesets_published_closed), 0) AS changesets_published_closed
FROM batch_change_counts
LEFT JOIN changeset_counts ON batch_change_counts.creation_week = changeset_counts.creation_week
GROUP BY batch_change_counts.creation_week;
`

	stats.BatchChangesCohorts = []*types.BatchChangesCohort{}
	rows, err := dbconn.Global.QueryContext(ctx, batchChangesCohortQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohort types.BatchChangesCohort

		if err := rows.Scan(
			&cohort.Week,
			&cohort.BatchChangesClosed,
			&cohort.BatchChangesOpen,
			&cohort.ChangesetsImported,
			&cohort.ChangesetsUnpublished,
			&cohort.ChangesetsPublished,
			&cohort.ChangesetsPublishedOpen,
			&cohort.ChangesetsPublishedDraft,
			&cohort.ChangesetsPublishedMerged,
			&cohort.ChangesetsPublishedClosed,
		); err != nil {
			return nil, err
		}

		stats.BatchChangesCohorts = append(stats.BatchChangesCohorts, &cohort)
	}

	return &stats, nil
}
