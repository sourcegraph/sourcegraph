# Troubleshooting Code Insights

This is a collection of issues or other information that might be helpful when troubleshooting
a problem with Code Insights.

## Recurring OOM (out of memory) alerts from the `frontend` service
This may be the result of an excessively large query being executed by code insights.

Code Insights processes some queries in the background of the `worker` service. These queries
use the GraphQL API, which means they are aggregated entirely on the `frontend` service. Large result
sets can cause the `frontend` service to run out of memory and crash with an OOM error. These queries
can get stuck in an error loop until they hit the maximum retry value, causing repeated `frontend` crashes.

Queries such as matching on every line in every repository or other queries with a similar
scale may be responsible.

### Diagnose
1. Check the `frontend` dashboards (General / Frontend) in Grafana
   1. Check for individual instances with spiked (to 100%) memory usage on `Container monitoring` - `Container memory by instance`
2. Check the background `worker` dashboards for Code Insights (General / Worker) in Grafana
   1. Check for elevated error rates on `Codeinsights: dbstore stats` - `Aggregate store operation error rate over 5m`
   2. Check for a queue size greater than zero on `Codeinsights: Query Runner Queue` - `Code insights query runner queue size`
3. (admin-only) Check the queries currently in background processing using the GraphQL query

   ``` gql 
      query seriesStatus {
      insightSeriesQueryStatus {
      seriesId
      query
      enabled
      completed
      errored
      processing
      failed
      queued
      }
    }
   ```
   1. Inspecting queries with `errored` or `failed` counts may provide a hint to which query is responsible.

4. Check Postgres `pgsql` for any queries stuck in a retry loop

  ``` sql
  select * from insights_query_runner_jobs
  where state = 'errored'
  and started_at > current_timestamp - INTERVAL '1 day'
  order by insights_query_runner_jobs.started_at desc;
  ```


### Resolution Options
1. Increase the memory available to the `frontend` pods until it is sufficiently large enough to execute the responsible query.
   1. The error rate on the Code Insights dashboards should return to zero.
2. (admin-only) Disable any specific queries identified to be problematic using the GraphQL operation by providing a specific `SeriesId`.

``` gql
mutation updateInsightSeries($input: UpdateInsightSeriesInput!) {
  updateInsightSeries(input:$input) {
    series {
      seriesId
      query
      enabled
    }
  }
}
{
  "input": {
    "seriesId": "s:5FE04D15D1150A134407E7EF078028F6DA5224BBADB1718A92E46046AC9F2E0B",
    "enabled": false
  }
}
```

3. Disable any problematic queries stuck in an error loop in Postgres `pgsql`

```sql
update insights_query_runner_jobs
    set state = 'failed'
where id = ?;
```

## OOB Migration has made progress, but is stuck before reaching 100%
This out-of-band migration is titled: **Migrating insight definitions from settings files to database tables as a last stage to use the GraphQL API.**

The out-of-band migration shouldn't take more than an hour to complete. (It really shouldn't take more than a few minutes.) If the progress hasn't reached 100% in this duration some records may be stuck due to errors.

Known issues:

- Deleted users/orgs will cause processing errors, and those jobs will need to be manually marked as complete.

### Diagnose and Resolve
1. First check the Recent Errors under the migration in the UI.
    1. If the error messages are all: `UserStoreGetById: user not found`
        - This is caused by deleted users. It will be safe to mark these rows as completed by running the following against `pgsql`:

            ```sql
            UPDATE insights_settings_migration_jobs SET completed_at = NOW() WHERE completed_at IS NULL;
            ```

    2. If the error messages are all: `OrgStoreGetByID: org not found`
        - This is caused by deleted orgs. In this case, mark just the org rows as completed by running the following against `pgsql`:

            ```sql
            UPDATE insights_settings_migration_jobs SET completed_at = NOW() WHERE completed_at IS NULL AND org_id IS NOT NULL;
            ```
        
        - Note: this only completes the failing org jobs. You may then see the `user not found` error above, and will still need to mark the rest of the jobs as complete.
        
    3. If the error messages are neither of those two things, this is not currently a known issue. Contact support and we can help!
