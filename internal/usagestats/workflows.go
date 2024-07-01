package usagestats

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/database"
)

type workflows struct {
	TotalWorkflows int32

	UniqueUserWorkflowOwners, UniqueUsers int32 // 2nd is the old field name for backcompat

	UniqueOrgWorkflowOwners int32
	UserOwnedWorkflows     int32

	OrgOwnedWorkflows, OrgWorkflows int32 // 2nd is the old field name for backcompat

	WorkflowsCreatedLast24h int32
	WorkflowsUpdatedLast24h int32

	NotificationsSent    int32
	NotificationsClicked int32
	UniqueUserPageViews  int32
}

func GetWorkflows(ctx context.Context, db database.DB) (*workflows, error) {
	const q = `
	SELECT
	(SELECT COUNT(*) FROM workflows) AS totalWorkflows,
	(SELECT COUNT(DISTINCT user_id) FROM workflows) AS uniqueUserWorkflowOwners,
	(SELECT COUNT(DISTINCT org_id) FROM workflows) AS uniqueOrgWorkflowOwners,
	(SELECT COUNT(*) FROM workflows WHERE user_id IS NOT NULL) AS userOwnedWorkflows,
	(SELECT COUNT(*) FROM workflows WHERE org_id IS NOT NULL) AS orgOwnedWorkflows,
	(SELECT COUNT(*) FROM workflows WHERE created_at > NOW() - INTERVAL '24 hours') AS workflowsCreatedLast24h,
	(SELECT COUNT(*) FROM workflows WHERE updated_at > NOW() - INTERVAL '24 hours') AS workflowsUpdatedLast24h,
	(SELECT COUNT(*) FROM event_logs WHERE event_logs.name = 'WorkflowEmailNotificationSent') AS notificationsSent,
	(SELECT COUNT(*) FROM event_logs WHERE event_logs.name = 'WorkflowEmailClicked') AS notificationsClicked,
	(SELECT COUNT(DISTINCT user_id) FROM event_logs WHERE event_logs.name = 'ViewWorkflowListPage') AS uniqueUserPageViews
	`
	var ss workflows
	if err := db.QueryRowContext(ctx, q).Scan(
		&ss.TotalWorkflows,
		&ss.UniqueUserWorkflowOwners,
		&ss.UniqueOrgWorkflowOwners,
		&ss.UserOwnedWorkflows,
		&ss.OrgOwnedWorkflows,
		&ss.WorkflowsCreatedLast24h,
		&ss.WorkflowsUpdatedLast24h,
		&ss.NotificationsSent,
		&ss.NotificationsClicked,
		&ss.UniqueUserPageViews,
	); err != nil {
		return nil, err
	}

	// Set old field names for backcompat
	ss.UniqueUsers = ss.UniqueUserWorkflowOwners
	ss.OrgWorkflows = ss.OrgOwnedWorkflows

	return &ss, nil
}
