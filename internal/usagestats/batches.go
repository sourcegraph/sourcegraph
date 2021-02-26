package usagestats

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// GetBatchChangesUsageStatistics returns the current site's batch changes usage.
func GetBatchChangesUsageStatistics(ctx context.Context) (*types.CampaignsUsageStatistics, error) {
	stats := types.CampaignsUsageStatistics{}

	const batchChangesCountsQuery = `
SELECT
    COUNT(*)                                      AS batch_changes_count,
    COUNT(*) FILTER (WHERE closed_at IS NOT NULL) AS batch_changes_closed_count
FROM batch_changes;
`
	if err := dbconn.Global.QueryRowContext(ctx, batchChangesCountsQuery).Scan(
		&stats.CampaignsCount,
		&stats.CampaignsClosedCount,
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
		&stats.ActionChangesetsUnpublishedCount,
		&stats.ActionChangesetsCount,
		&stats.ActionChangesetsDiffStatAddedSum,
		&stats.ActionChangesetsDiffStatChangedSum,
		&stats.ActionChangesetsDiffStatDeletedSum,
		&stats.ActionChangesetsMergedCount,
		&stats.ActionChangesetsMergedDiffStatAddedSum,
		&stats.ActionChangesetsMergedDiffStatChangedSum,
		&stats.ActionChangesetsMergedDiffStatDeletedSum,
		&stats.ManualChangesetsCount,
		&stats.ManualChangesetsMergedCount,
	); err != nil {
		return nil, err
	}

	const eventLogsCountsQuery = `
SELECT
    COUNT(*)                                                FILTER (WHERE name = 'CampaignSpecCreated')                       AS campaign_specs_created,
    COALESCE(SUM((argument->>'changeset_specs_count')::int) FILTER (WHERE name = 'CampaignSpecCreated'), 0)                   AS changeset_specs_created_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewCampaignApplyPage')                     AS view_campaign_apply_page_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewCampaignDetailsPageAfterCreate')   AS view_campaign_details_page_after_create_count,
    COUNT(*)                                                FILTER (WHERE name = 'ViewCampaignDetailsPageAfterUpdate')   AS view_campaign_details_page_after_update_count
FROM event_logs
WHERE name IN ('CampaignSpecCreated', 'ViewCampaignApplyPage', 'ViewCampaignDetailsPageAfterCreate', 'ViewCampaignDetailsPageAfterUpdate');
`

	if err := dbconn.Global.QueryRowContext(ctx, eventLogsCountsQuery).Scan(
		&stats.CampaignSpecsCreatedCount,
		&stats.ChangesetSpecsCreatedCount,
		&stats.ViewCampaignApplyPageCount,
		&stats.ViewCampaignDetailsPageAfterCreateCount,
		&stats.ViewCampaignDetailsPageAfterUpdateCount,
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
		sqlf.Sprintf("%q", "CampaignSpecCreated"),
		sqlf.Sprintf("%q", "CampaignCreated"),
		sqlf.Sprintf("%q", "CampaignCreatedOrUpdated"),
		sqlf.Sprintf("%q", "CampaignClosed"),
		sqlf.Sprintf("%q", "CampaignDeleted"),
		sqlf.Sprintf("%q", "ViewCampaignApplyPage"),
	}

	if err := queryUniqueEventLogUsersCurrentMonth(contributorEvents).Scan(&stats.CurrentMonthContributorsCount); err != nil {
		return nil, err
	}

	var usersEvents = []*sqlf.Query{
		sqlf.Sprintf("%q", "CampaignSpecCreated"),
		sqlf.Sprintf("%q", "CampaignCreated"),
		sqlf.Sprintf("%q", "CampaignCreatedOrUpdated"),
		sqlf.Sprintf("%q", "CampaignClosed"),
		sqlf.Sprintf("%q", "CampaignDeleted"),
		sqlf.Sprintf("%q", "ViewCampaignApplyPage"),
		sqlf.Sprintf("%q", "ViewCampaignDetailsPagePage"),
		sqlf.Sprintf("%q", "ViewCampaignsListPage"),
	}

	if err := queryUniqueEventLogUsersCurrentMonth(usersEvents).Scan(&stats.CurrentMonthUsersCount); err != nil {
		return nil, err
	}

	const campaignsCohortsQuery = `
WITH
cohort_campaigns as (
  SELECT
    date_trunc('week', campaigns.created_at)::date AS creation_week,
    id
  FROM
    campaigns
  WHERE
    created_at >= now() - (INTERVAL '12 months')
),
changeset_counts AS (
  SELECT
    cohort_campaigns.creation_week,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id IS NULL OR changesets.owned_by_campaign_id != cohort_campaigns.id)  AS changesets_imported,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND publication_state = 'UNPUBLISHED')  AS changesets_unpublished,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND publication_state != 'UNPUBLISHED') AS changesets_published,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND external_state = 'OPEN') AS changesets_published_open,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND external_state = 'DRAFT') AS changesets_published_draft,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND external_state = 'MERGED') AS changesets_published_merged,
    COUNT(changesets) FILTER (WHERE changesets.owned_by_campaign_id = cohort_campaigns.id AND external_state = 'CLOSED') AS changesets_published_closed
  FROM changesets
  JOIN cohort_campaigns ON changesets.campaign_ids ? cohort_campaigns.id::text
  GROUP BY cohort_campaigns.creation_week
),
campaign_counts AS (
  SELECT
    date_trunc('week', campaigns.created_at)::date          AS creation_week,
    COUNT(distinct id) FILTER (WHERE closed_at IS NOT NULL) AS closed,
    COUNT(distinct id) FILTER (WHERE closed_at IS NULL)     AS open
  FROM campaigns
  WHERE
    created_at >= now() - (INTERVAL '12 months')
  GROUP BY date_trunc('week', campaigns.created_at)::date
)
SELECT to_char(campaign_counts.creation_week, 'yyyy-mm-dd')           AS creation_week,
       COALESCE(SUM(campaign_counts.closed), 0)                       AS campaigns_closed,
       COALESCE(SUM(campaign_counts.open), 0)                         AS campaigns_open,
       COALESCE(SUM(changeset_counts.changesets_imported), 0)         AS changesets_imported,
       COALESCE(SUM(changeset_counts.changesets_unpublished), 0)      AS changesets_unpublished,
       COALESCE(SUM(changeset_counts.changesets_published), 0)        AS changesets_published,
       COALESCE(SUM(changeset_counts.changesets_published_open), 0)   AS changesets_published_open,
       COALESCE(SUM(changeset_counts.changesets_published_draft), 0)  AS changesets_published_draft,
       COALESCE(SUM(changeset_counts.changesets_published_merged), 0) AS changesets_published_merged,
       COALESCE(SUM(changeset_counts.changesets_published_closed), 0) AS changesets_published_closed
FROM campaign_counts
LEFT JOIN changeset_counts ON campaign_counts.creation_week = changeset_counts.creation_week
GROUP BY campaign_counts.creation_week;
`

	stats.CampaignsCohorts = []*types.CampaignsCohort{}
	rows, err := dbconn.Global.QueryContext(ctx, campaignsCohortsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cohort types.CampaignsCohort

		if err := rows.Scan(
			&cohort.Week,
			&cohort.CampaignsClosed,
			&cohort.CampaignsOpen,
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

		stats.CampaignsCohorts = append(stats.CampaignsCohorts, &cohort)
	}

	return &stats, nil
}
