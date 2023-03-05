package usagestats

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func GetMigratedExtensionsUsageStatistics(ctx context.Context, db database.DB) (*types.MigratedExtensionsUsageStatistics, error) {
	stats := types.MigratedExtensionsUsageStatistics{}

	if err := db.QueryRowContext(ctx, MigratedExtensionsUsageQuery).Scan(
		&stats.GitBlameEnabled,
		&stats.GitBlameEnabledUniqueUsers,
		&stats.GitBlameDisabled,
		&stats.GitBlameDisabledUniqueUsers,
		&stats.GitBlamePopupViewed,
		&stats.GitBlamePopupViewedUniqueUsers,
		&stats.GitBlamePopupClicked,
		&stats.GitBlamePopupClickedUniqueUsers,

		&stats.SearchExportPerformed,
		&stats.SearchExportPerformedUniqueUsers,
		&stats.SearchExportFailed,
		&stats.SearchExportFailedUniqueUsers,
	); err != nil {
		return nil, err
	}

	openInEditorUsageByIde := []*types.MigratedExtensionsOpenInEditorUsageStatistics{}
	rows, err := db.QueryContext(ctx, MigratedExtensionsOpenInEditorUsageQuery)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		openInEditorUsage := types.MigratedExtensionsOpenInEditorUsageStatistics{}

		if err := rows.Scan(
			&openInEditorUsage.IdeKind,
			&openInEditorUsage.Clicked,
			&openInEditorUsage.ClickedUniqueUsers,
		); err != nil {
			return nil, err
		}

		openInEditorUsageByIde = append(openInEditorUsageByIde, &openInEditorUsage)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	stats.OpenInEditor = openInEditorUsageByIde

	return &stats, nil
}

var MigratedExtensionsUsageQuery = `
	WITH event_log_stats AS (
		SELECT
			NULLIF(COUNT(*) FILTER (WHERE name = 'GitBlameEnabled'), 0) :: INT AS git_blame_enabled,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'GitBlameEnabled'), 0) :: INT AS git_blame_enabled_unique_users,
			NULLIF(COUNT(*) FILTER (WHERE name = 'GitBlameDisabled'), 0) :: INT AS git_blame_disabled,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'GitBlameDisabled'), 0) :: INT AS git_blame_disabled_unique_users,
			NULLIF(COUNT(*) FILTER (WHERE name = 'GitBlamePopupViewed'), 0) :: INT AS git_blame_popup_viewed,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'GitBlamePopupViewed'), 0) :: INT AS git_blame_popup_viewed_unique_users,
			NULLIF(COUNT(*) FILTER (WHERE name = 'GitBlamePopupClicked'), 0) :: INT AS git_blame_popup_clicked,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'GitBlamePopupClicked'), 0) :: INT AS git_blame_popup_clicked_unique_users,

			NULLIF(COUNT(*) FILTER (WHERE name = 'SearchExportPerformed'), 0) :: INT AS search_export_performed,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'SearchExportPerformed'), 0) :: INT AS search_export_performed_unique_users,
			NULLIF(COUNT(*) FILTER (WHERE name = 'SearchExportFailed'), 0) :: INT AS search_export_failed,
			NULLIF(COUNT(DISTINCT event_logs.user_id) FILTER (WHERE name = 'SearchExportFailed'), 0) :: INT AS search_export_failed_unique_users
		FROM event_logs
		WHERE
			name IN (
				'GitBlameEnabled',
				'GitBlameDisabled',
				'GitBlamePopupViewed',
				'GitBlamePopupClicked',

				'SearchExportPerformed',
				'SearchExportFailed',
			)
	)
	SELECT
		event_log_stats.git_blame_enabled,
		event_log_stats.git_blame_enabled_unique_users,
		event_log_stats.git_blame_disabled,
		event_log_stats.git_blame_disabled_unique_users,
		event_log_stats.git_blame_popup_viewed,
		event_log_stats.git_blame_popup_viewed_unique_users,
		event_log_stats.git_blame_popup_clicked,
		event_log_stats.git_blame_popup_clicked_unique_users,


		event_log_stats.search_export_performed,
		event_log_stats.search_export_performed_unique_users,
		event_log_stats.search_export_failed,
		event_log_stats.search_export_failed_unique_users,
	FROM
		event_log_stats;
`

var MigratedExtensionsOpenInEditorUsageQuery = `
	WITH events_with_ide_kind AS (
		SELECT
			public_argument ->> 'editor'::text AS ide_kind,
			name,
			user_id
		FROM event_logs
		WHERE
			name IN (
				'OpenInEditorClicked'
			)
	), event_log_stats AS (
		SELECT
			ide_kind,
			NULLIF(COUNT(*) FILTER (WHERE events_with_ide_kind.name = 'OpenInEditorClicked'), 0) :: INT AS open_in_editor_clicked,
			NULLIF(COUNT(DISTINCT events_with_ide_kind.user_id) FILTER (WHERE events_with_ide_kind.name = 'OpenInEditorClicked'), 0) :: INT AS open_in_editor_clicked_unique_users
		FROM events_with_ide_kind
		GROUP BY
			ide_kind
	)
	SELECT
		ide_kind,
		event_log_stats.open_in_editor_clicked,
		event_log_stats.open_in_editor_clicked_unique_users
	FROM
		event_log_stats
`
