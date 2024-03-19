# Administration and security of Code Insights

## Code Insights enforce user permissions 

Users can only create code insights that include repositories they have access to. Moreover, when creating code insights, the repository field will *not* validate nor show users repositories they would not have access to otherwise.

When a user is viewing an insight, any repositories they do not have access to will not be included in the total counts.

## Security of native Sourcegraph Code Insights (Search-based and Language Insights)

Sourcegraph search-based and language insights run natively on a Sourcegraph instance using the instance's Sourcegraph search API. This means they don't send any information about your code to third-party servers. 

## Insight and Dashboard permissions

Note: there are no separate read/write permissions. If a user can view an insight or dashboard, they can also edit it.

A user can view an insight if at least one of the following is true:

1. The user created the insight.
2. The user has permission to view a dashboard that the insight is on.

Except for the singular, non-transferable creator's permission noted in case 1 above, permissions can be thought of as belonging to a dashboard. Dashboards have 3 permission levels:

- User: only this specific user can view this dashboard.
- Organization: only users within this organization can view this dashboard.
- Global: any user on the Sourcegraph instance can view this dashboard.

### Changing permission levels

Because there are no separate read/write permissions and no dashboard owners, any user who can view a dashboard can change its permission level or add/remove insights from the dashboard. The only way to guarantee continued access to an insight that you did not create is to add it to a private dashboard.

If a user gets deleted, any insights they created will still be visible to other users via the dashboards they appear on. However, if one of these insights is removed from all dashboards, it will no longer be accessible.

## Code Insights Admin  (>=5.0)

Sourcegraph administrators can view and manage the background jobs that Code Insights runs when historically backfilling an insight. The following functionality is available under _Code Insights jobs_ in the _Maintenance_ section within the Site Admin (`/site-admin/code-insights-jobs`):
  - See a list of the jobs that backfill an insight and their current state
  - See any errors that occurred when backfilling an insight
  - Retry a failed backfill
  - Move a backfill job to the front or back of the processing queue
  
The jobs can be searched by state and by an insight's title or the label of any of its series.
## Code Insights Site Configuration

While the default configuration is appropriate for most deployments, in the site configuration there are values that allow admins more control over the rate at which insights runs in the background. 
Raising these values will increase the speed at which insights are populated however will it cause insights to consume more system resources.  
Care should be taken when changing these values and it is recommended to update them in small increments.

The following settings apply when backfilling data for a Code Insight:
- `insights.historical.worker.rateLimit` - Maximum number of historical Code Insights data frames that may be analyzed per second.
- `insights.backfill.interruptAfter` - The amount of time an Code Insights will spend backfilling a series before checking if there is higher priority work.
- `insights.backfill.repositoryGroupSize` - The number of repositories that Code Insights will pull as a batch to backfill in one iteration.
- `insights.backfill.repositoryConcurrency` - The number of repositories that Code Insights will backfill at once.

The following setting(s) apply to adding new data to a previously backfilled Code Insight:
- `insights.query.worker.concurrency` - Number of concurrent executions of a code insight query on a worker node.

The following setting(s) apply to both backfilling data and adding new data
- `insights.query.worker.rateLimit` - Maximum number of Code Insights queries initiated per second on a worker node.
