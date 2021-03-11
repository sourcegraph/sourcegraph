package usagestats

import (
	"context"

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

	err := dbconn.Global.QueryRowContext(ctx, eventLogsCountsQuery).Scan(
		&stats.CampaignSpecsCreatedCount,
		&stats.ChangesetSpecsCreatedCount,
		&stats.ViewCampaignApplyPageCount,
		&stats.ViewCampaignDetailsPageAfterCreateCount,
		&stats.ViewCampaignDetailsPageAfterUpdateCount,
	)

	return &stats, err
}
