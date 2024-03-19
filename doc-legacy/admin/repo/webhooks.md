# Repository webhooks

## Webhook for manually telling Sourcegraph to update a repository

By default, Sourcegraph polls code hosts to keep repository contents up to date. It uses intelligent heuristics like average update frequency to determine the polling frequency per repository.

Polling, however, falls short in cases where immediate updates are desired or when the number of repositories causes significant load on the code host.

To address this, there is a repository update webhook that triggers a repository update on Sourcegraph within minutes. The webhook is authenticated using access tokens (which you can create at e.g. `https://sourcegraph.example.com/site-admin/tokens`).

Here's an example using curl.

```bash
curl -XPOST -H 'Authorization: token $ACCESS_TOKEN' $SOURCEGRAPH_ORIGIN/.api/repos/$REPO_NAME/-/refresh
```

## Disabling built-in repo updating

Sourcegraph will periodically ask your code-host to list its repositories (e.g. via its HTTP API) to _discover repositories_. You can control how often this occurs by changing [`repoListUpdateInterval`](../config/site_config.md) in the site config.

For repositories that Sourcegraph is already aware of, it will periodically perform background Git repository updates. You can disable this if you wish by setting [`disableAutoGitUpdates`](../config/site_config.md) to `true`. In which case, the repository will only update when the webhook is used or, e.g., if a user visits the repository directly. This may be desirable in cases where you wish to rely solely on the repository update webhook, for example.

## Code host webhooks

We support receiving webhooks directly from your code host for [GitHub](../config/webhooks/incoming.md#github), [GitLab](../config/webhooks/incoming.md#gitlab), [Bitbucket Server](../config/webhooks/incoming.md#bitbucket-server) and [Bitbucket Cloud](../config/webhooks/incoming.md#bitbucket-cloud).
