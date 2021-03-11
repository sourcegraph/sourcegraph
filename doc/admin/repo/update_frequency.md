# Repository update frequency

By default, Sourcegraph polls code hosts to keep repository contents up to date, effectively running `git pull` periodically. You can also configure Sourcegraph to use [repository webhooks](webhooks.md), but this is usually not necessary.

The frequency at which Sourcegraph polls the code host for updates is determined by a smart heuristic based on past commit frequency in the repository. For example, if a repository's last commit was 8 hours ago, then the next sync will be scheduled 4 hours from now. If after 4 hours, there are still no new commits, then the next sync will be scheduled 6 hours from then.

Repositories will never be updated more frequently than 45 seconds, and no less frequently than every 8 hours.

After Sourcegraph has updated a repository's Git data, the global search index will automatically update a short while after (usually a few minutes).

## Limiting repository updates

If you wish to control how frequently repositories are discovered or how frequently Sourcegraph polls your code host for updates, tuning parameters are available in the site configuration:

- [repoListUpdateInterval](../config/site_config.md#repoListUpdateInterval) controls how frequently we check the code host _for new repositories_ in minutes.
- [gitMaxConcurrentClones](../config/site_config.md#gitMaxConcurrentClones) controls the maximum number of _concurrent_ cloning / pulling operations per gitserver that Sourcegraph will perform.

You may also choose to disable automatic Git updates entirely and instead [configure repository webhooks](webhooks.md).

## Code host API rate limiting

Sourcegraph uses a configurable internal rate limiter for API requests made from Sourcegraph to [GitHub](../external_service/github.md#internal-rate-limits), [GitLab](../external_service/gitlab.md#internal-rate-limits), [Bitucket Server](../external_service/bitbucket_server.md#internal-rate-limits) and [Bitbucket Cloud](../external_service/bitbucket_cloud.md#internal-rate-limits).

**NOTE** Internal rate limiting is currently only enforced for syncing changesets in [batch changes](../../batch_changes/index.md)
