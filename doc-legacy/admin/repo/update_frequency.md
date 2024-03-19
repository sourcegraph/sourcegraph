# Repository update frequency

By default, Sourcegraph polls code hosts to keep repository contents up to date, effectively running `git pull` periodically. You can also configure Sourcegraph to use [repository webhooks](webhooks.md), but this is usually not necessary.

The frequency at which Sourcegraph polls the code host for updates is determined by a smart heuristic based on past commit frequency in the repository. For example, if a repository's last commit was 8 hours ago, then the next sync will be scheduled 4 hours from now. If after 4 hours, there are still no new commits, then the next sync will be scheduled 6 hours from then.

Repositories will never be updated more frequently than 45 seconds, and no less frequently than every 8 hours.

After Sourcegraph has updated a repository's Git data, the global search index will automatically update a short while after (usually a few minutes).

## Rate Limiting
If you wish to control how frequently repositories are discovered or how frequently Sourcegraph polls your code host for updates, the following options are available:

### Limiting the number of Code host API requests

- Code host configuration: see [Rate limits](../external_service/rate_limits.md)

- Site configuration: [repoListUpdateInterval](../config/site_config.md#repoListUpdateInterval) controls how frequently we check the code host _for new repositories_ in minutes.

> NOTE: Internal rate limiting is currently only enforced for HTTP requests to code hosts. That means it's used when, for example, syncing changesets in [batch changes](../../batch_changes/index.md), repository permissions and repository metadata from code hosts.

### Limiting the number of Code host Git requests

- [gitMaxCodehostRequestsPerSecond](../config/site_config.md#gitMaxCodehostRequestsPerSecond) controls how many code host git operations can be run against a code host per second, per gitserver.
- [gitMaxConcurrentClones](../config/site_config.md#gitMaxConcurrentClones) controls the maximum number of _concurrent_ cloning/pulling operations per gitserver that Sourcegraph will perform.

You may also choose to disable automatic Git updates entirely and instead [configure repository webhooks](webhooks.md).

## Repo Updater State

> NOTE: [Instrumentation](../../admin/faq.md#i-am-getting-error-cluster-information-not-available-in-the-instrumentation-page-what-should-i-do) (where Repo Updater State resides) is only available for Kubernetes instances.

**Repo Updater State** is a useful debugging tool for site admins to monitor:

- **Schedule**: The schedule of when repositories get enqueued into the Update Queue.
- **Update Queue**: A priority queue of repositories to update. A worker continuously dequeues them and sends updates to gitserver.
- **Sync jobs**: The current list of external service sync jobs, ordered by start date descending

Site admin: Go to **Site admin > Instrumentation (under Maintenance) > repo-updater > Repo Updater State**
