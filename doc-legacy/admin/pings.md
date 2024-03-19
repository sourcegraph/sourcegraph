# Pings

Sourcegraph periodically sends a ping to `pings.sourcegraph.com` to help our product and customer teams. It sends only the high-level data below. It never sends code, repository names, usernames, or any other specific data. To learn more, go to the **Site admin > Pings** page on your instance (the URL is `https://sourcegraph.example.com/site-admin/pings`). 

Sourcegraph will also periodically perform a license verification check, to verify the validity of the configured Sourcegraph license. Tampering with these checks, or preventing them from occuring, will cause Sourcegraph to disable many features until a successful check is completed. Certain Enterprise licenses can request to be exempt from these license verification checks.

## Telemetry

Sourcegraph aggregates usage and performance metrics for some product features in our enterprise deployments. No personal or specific information is ever included.

<details>
<summary>Click to expand a list of other telemetry</summary>

- Randomly generated site identifier
- The email address of the initial site installer (or if deleted, the first active site admin), to know who to contact regarding sales, product updates, security updates, and policy updates
- Sourcegraph version string (e.g. "vX.X.X")
- Dependency versions (e.g. "6.0.9" for Redis, or "13.0" for Postgres)
- Deployment type (single Docker image, Docker Compose, Kubernetes cluster, Helm, or pure Docker cluster)
- License key associated with your Sourcegraph subscription
- Aggregate count of current monthly users
- Total count of existing user accounts
- Aggregated repository statistics
  - Total size of git repositories stored in bytes
  - Total number of lines of code stored in text search index
- Whether the instance is deployed on localhost (true/false)
- Which category of authentication provider is in use (built-in, OpenID Connect, an HTTP proxy, SAML, GitHub, GitLab)
- Which code hosts are in use (GitHub, Bitbucket Server / Bitbucket Data Center, GitLab, Phabricator, Gitolite, AWS CodeCommit, Other)
  - Which versions of the code hosts are used
- Whether new user signup is allowed (true/false)
- Whether a repository has ever been added (true/false)
- Whether a code search has ever been executed (true/false)
- Whether code navigation has ever been used (true/false)
- Aggregate counts of current daily, weekly, and monthly users
- Aggregate counts of current daily, weekly, and monthly users, by whether they are using code host integrations
- Aggregate daily, weekly, and monthly latencies (in ms) of search queries
- Aggregate daily, weekly, and monthly integer counts of the following query syntax:
  - The number of boolean operators (`and`, `or`, `not` keywords)
  - The number of built-in predicate keywords (`contains`, `contains.file`, `contains.repo`, `contains.commit.after`, `dependencies`)
  - The number of `select` keywords by kind (`repo`, `file`, `content`, `symbol`, `commit.diff.added`, `commit.diff.removed`)
  - The number of queries using the `context:` filter without the default `global` value
  - The number of queries with only patterns (e.g., without filters like `repo:` or `file:`)
  - The number of queries with three or more patterns
- Aggregate daily, weekly, and monthly user counts of search queries with the above properties
- Code navigation usage data
  - Total number of repositories with and without an uploaded precise code navigation index
  - Total number of code navigation queries (e.g., hover tooltips) per week grouped by language
  - Number of users performing code navigation queries (e.g., hover tooltips) per week grouped by language
<!-- depends-on-source: ~/internal/usagestats/batches.go -->
- Batch Changes usage data
  - Total count of page views on the batch change apply page
  - Total count of page views on the batch change details page after creating a batch change
  - Total count of page views on the batch change details page after updating a batch change
  - Total count of created changeset specs
  - Total count of created batch change specs
  - Total count of created batch changes
  - Total count of closed batch changes
  - Total count of changesets created by batch changes
  - Aggregate counts of lines changed, added, deleted in all changesets
  - Total count of changesets created by batch changes that have been merged
  - Aggregate counts of lines changed, added, deleted in all merged changesets
  - Total count of changesets manually added to a batch change
  - Total count of changesets manually added to a batch change that have been merged
  - Aggregate counts of unique monthly users, by:
      - Whether they have contributed to batch changes
      - Whether they only viewed batch changes
      - Whether they have performed a bulk operation
  - Weekly batch change (open, closed) and changesets counts (imported, published, unpublished, open, draft, merged, closed) for batch change cohorts created in the last 12 months
  - Weekly bulk operations count (grouped by operation)
  - Total count of executors connected
  - Cumulative executor runtime monthly
  - Total count of `publish` bulk operation
  - Total count of bulk operations (grouped by operation type)
  - Changeset distribution for batch change (grouped by batch change source: `local` or `executor`)
  - Total count of users that ran a job on an executor monthly
  - Total count of published changesets and batch changes created via:
      - executor
      - local (using `src-cli`)
- Aggregated counts of users created, deleted, retained, resurrected and churned within the month
- Aggregated counts of access requests pending, approved, rejected
- Saved searches usage data
  - Count of saved searches
  - Count of users using saved searches
  - Count of notifications triggered
  - Count of notifications clicked
  - Count of saved search views
- Homepage panel engagement
  - Percentage of panel clicks (out of total views)
  - Total count of unique users engaging with the panels
- Weekly retention rates for user cohorts created in the last 12 weeks
- Search onboarding engagement
  - Total number of views of the onboarding tour
  - Total number of views of each step in the onboarding tour
  - Total number of tours closed
- Sourcegraph extension activation statistics
  - Total number of users that use a given non-default Sourcegraph extension
  - Average number of activations for users that use a given non-default Sourcegraph extension
  - Total number of users that use non-default Sourcegraph extensions
  - Average number of non-default extensions enabled for users that use non-default Sourcegraph extensions
- Code insights usage data
  - Total count of insights
  - Weekly count of page views on the insights pages
  - Weekly count of unique viewers on the insights pages
  - Weekly counts of hovers and clicks insights by type (e.g. search, code stats)
  - Weekly counts of resizing of insights by type
  - Weekly counts of edits, additions, and removals of insights by type
  - Total count of clicks on the "Add more insights" and "Configure insights" buttons on the insights page
  - Weekly count of users that have created an insight
  - Weekly count of users that have created their first insight this week
  - Weekly count of total and unique views to the different `Create`, `Create search insight`, and `Create language insight` pages
  - Weekly count of total and unique clicks of the different `Create search insight` and `Create language usage insight`, and `Explore the extensions` buttons on the `Create new insight` page
  - Weekly count of total and unique clicks of the `Create` and `Cancel` buttons on the `Create search insight` and `Create language insight` pages
  - Total count of insights grouped by time interval (step size) in days
  - Total count of insights that are organization visible grouped by insight type
  - Total count of insights grouped by presentation type, series type, and presentation-series type.
  - Weekly count of unique users that have viewed code insights in-product landing page
  - Weekly count of per user changes that have been made over the query field insight example.
  - Weekly count of per user changes that have been made over the repositories field insight example.
  - Weekly count of clicks on the "Create your first insight" CTA button on the in-product landing page.
  - Weekly count of clicks on the code insights in-product template section's tabs, with tab title data.
  - Weekly count of clicks on the use/explore template card's button.
  - Weekly count of clicks on the "view more" template section button.
  - Weekly count of clicks on the in-product landing page documentation links.
  - Weekly count of filters usage on the standalone insight page
  - Weekly count of navigation to dashboards from the standalone insight page
  - Weekly count of clicks on "Edit" from the standalone insight page
  - Total count of individual view series, grouped by presentation type and generation method
  - Total count of insight series, grouped by generation method
  - Total count of views, grouped by presentation type
  - Total count of organisations with at least one dashboard
  - Total count of dashboards
  - Total count of insights per dashboard
  - Weekly count of time to complete an insight series backfill in seconds 
  - Weekly count of requests of exports of Code Insights data
- Search aggregations usage data
  - Weekly count of hovers over the search aggregations information icon
  - Weekly count of open/collapse clicks on the sidebar and expanded view of search aggregations
  - Weekly count of search aggregation mode clicks and hovers
  - Weekly count of search aggregation bars clicks and hovers
  - Weekly count of search aggregation success and timeouts 
- Code monitoring usage data
  - Total number of views of the code monitoring page
  - Total number of views of the create code monitor page
  - Total number of views of the create code monitor page with a pre-populated trigger query
  - Total number of views of the create code monitor page without a pre-populated trigger query
  - Total number of views of the manage code monitor page
  - Total number of clicks on the code monitor email search link
  - Total number of clicks on example monitors
  - Total number of views of the getting started page
  - Total number of submissions of the create code monitor form
  - Total number of submissions of the manage code monitor form
  - Total number of deletions from the manage code monitor form
  - Total number of views of the logs page
  - Current number of Slack, webhook, and email actions enabled
  - Current number of unique users with Slack, webhook, and email actions enabled
  - Total number of Slack, webhook, and email actions triggered
  - Total number of Slack, webhook, and email action triggers that errored
  - Total number of unique users that have had Slack, webhook, and email actions triggered
  - Total number of search executions
  - Total number of search executions that errored
  - 50th and 90th percentile runtimes for search executions
- Notebooks usage data
  - Total number of views of the notebook page
  - Total number of views of the notebooks list page
  - Total number of views of the embedded notebook page
  - Total number of created notebooks
  - Total number of added notebook stars
  - Total number of added notebook markdown blocks
  - Total number of added notebook query blocks
  - Total number of added notebook file blocks
  - Total number of added notebook symbol blocks
  - Total number of added notebook compute blocks
- Code Host integration usage data (Browser extension / Native Integration)
  - Aggregate counts of current daily, weekly, and monthly unique users and total events
  - Aggregate counts of current daily, weekly, and monthly unique users and total events who visited Sourcegraph instance from browser extension
- IDE extensions usage data
  - Aggregate counts of current daily, weekly, and monthly searches performed:
    - Count of unique users who performed searches
    - Count of total searches performed
  - Aggregate counts of current daily user state:
    - Count of users who installed the extension
    - Count of users who uninstalled the extension
  - Aggregate count of current daily redirects from extension to Sourcegraph instance
- Migrated extensions usage data
  - Aggregate data of:
    - Count interactions with the Git blame feature
    - Count of unique users who interacted with the Git blame feature
    - Count interactions with the open in editor feature
    - Count of unique users who interacted with the open in editor feature
    - Count interactions with the search exports feature
    - Count of unique users who interacted with the search exports feature
    - Count interactions with the go imports search query transformation feature
    - Count of unique users who interacted with the go imports search query transformation feature
- Code ownership usage data
  - Number and ratio of repositories for which ownership data is available via CODEOWNERS file or the API.
  - Number of owners assigned through Own.
  <!-- - Aggregate monthly weekly and daily active users for the following activities: -->
    - Narrowing search results by owner using `file:has.owners` predicate.
    - Selecting owner search result through `select:file.owners`.
    - Displaying ownership panel in file view.
- Histogram of cloned repository sizes
- Aggregate daily, weekly, monthly repository metadata usage statistics
- Cody providers data
  - Completions
    - Provider name
    - Chat, fast chat, and completion model names (only for "sourcegraph" provider)
  - Embeddings
    - Provider name
    - Model name (only for "sourcegraph" provider)
</details>

## Allowlist IPs / CIDR Ranges for Sourcegraph

Starting 5.2.0:
- For `pings.sourcegraph.com`, allowlist the IP address: `34.36.231.254`
- For `sourcegraph.com`, allowlist the full [Cloudflare IP ranges](https://www.cloudflare.com/ips/)

Prior to 5.2.0, allowlist the full [Cloudflare IP ranges](https://www.cloudflare.com/ips/).

## Using an HTTP proxy for telemetry requests

The environment variable `TELEMETRY_HTTP_PROXY` can be set on the `sourcegraph-frontend` service, to use an HTTP proxy for telemetry requests.


Be sure to update the enviornment variable like so : ```TELEMETRY_HTTP_PROXY:"http://proxy.example.com:8080"```

## Connections to Sourcegraph-managed services


Sourcegraph only connects to Sourcegraph-managed services for three purposes:

1. The pings described above are sent, in order to:
   - Check for new product updates.
   - Send [anonymous, non-specific, aggregate metrics](#pings) back to Sourcegraph.com (see the full list above).
1. [Verify](./licensing/index.md) the validity of the configured Sourcegraph license.
1. Legacy Sourcegraph extensions are fetched from Sourcegraph.com`s extension registry.

There are no other automatic external connections to Sourcegraph.com (or any other site on the internet).

## Connections to Sourcegraph.com via Cody app

The Cody app connects to Sourcegraph.com to send a limited selection of the pings described above in order to infer value to our users and send update notifications. They include:  

- Randomly generated site identifier
- Deployment type (the Cody app)
- Release version
- Operating system
- Total number of repositories added
- Whether a user was active today (boolean) 

## Troubleshooting Pings

It may happen that Sourcegraph will stop sending critical telemetry to Sourcegraph.com, if this happens it may indicate a problem with Sourcegraphs frontend database, or a site settings misconfiguration. Below are some debugging steps.

Sourcegraph telemetry pings are handled by a goroutine running on Sourcegraphs frontend service called [`updatecheck`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/internal/app/updatecheck/client.go?subtree=true), `updatecheck` is [started](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Ecmd/frontend/internal/cli/serve_cmd%5C.go+updatecheck.Start%28db%29&patternType=literal) on container startup and periodically requests a variety of queries be run in the `pgsql` database.


### Misconfigured update.channel
The most common scenario in which Sourcegraph stops sending pings is a change to the `update.channel` setting in an instance's [site config](https://docs.sourcegraph.com/admin/config/site_config)
```
"update.channel": "release",
```
*This setting [must be set to "release"](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/updatecheck/client.go?L803-806) in order for the telemetry goroutine to run.*


### Check if the goroutine is running

*This section is under development and currently only applies for docker-compose instances: [39710](https://github.com/sourcegraph/sourcegraph/issues/39710)*

If it's reported that pings aren't being sent to the Sourcegraph.com, you can check that the goroutine is running with the following command:
```
docker exec -it sourcegraph-frontend-0 sh -c 'wget -nv -O- 'http://127.0.0.1:6060/debug/pprof/goroutine?debug=1' | grep updatecheck'
```
Example:
```
[ec2-user@latveria-ip ~]$ docker exec -it sourcegraph-frontend-0 sh -c 'wget -nv -O- 'http://127.0.0.1:6060/debug/pprof/goroutine?debug=1' | grep updatecheck'
2022-04-05 20:53:32 URL:http://127.0.0.1:6060/debug/pprof/goroutine?debug=1 [14660] -> "-" [1]
#	0x1f052c5	github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/updatecheck.Start+0xc5	github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/updatecheck/client.go:697
```

### Looking for errors in the logs

If the `update.check` is running, and the site config is correctly configured, then it may be the case that `pgsql` is failing to return data from the SQL queries to the `frontend`. Check out the frontend logs for logs tagged [`telemetry: updatecheck failed`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Ecmd/frontend/internal/app/updatecheck/client%5C.go+telemetry:+updatecheck+failed&patternType=literal).

If issues persist, please reach out to a team member for support at support@sourcegraph.com
