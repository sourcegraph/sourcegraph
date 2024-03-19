# How to remove a repository from Sourcegraph

## Prerequisites

This document assumes that you have:

- site-admin level permissions on your Sourcegraph instance
- access to your Sourcegraph deployment

## Steps to remove a repository from Sourcegraph

Open the repository's `Settings` page on Sourcegraph and from the `Options` tab click `Exclude repository` to exclude it from the specific code host.

![Exclude repository from code host](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/how-to/exclude-repo.png)

Alternately, if a repository is synced from multiple code host connections you may exclude it from all code hosts by clicking the `Exclude repository from all code hosts` button instead.

![Exclude repository from all code hosts](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/how-to/exclude-repo-from-all-code-hosts.png)

## Remove corrupted repository data from Sourcegraph

1. Exclude the repository as shown above from your code host(s)
1. Wait for the repository to disappear from the Repository Status Page located in your Site Admin panel.
1. Once you have confirmed the previous step has been completed, you will then exec into Gitserver (for docker-compose and kubernetes deployments) to locate the files that are associated with the repository.
1. Look for a directory with the name of the repository in the Gitserver. It should be located in the following file path: `data/repos/{name-of-code-host}/{name-of-repository}`
1. Delete the directory for that repository from the previous step.

## To reclone a removed repository

Open the repository's `Settings` page on Sourcegraph and from the `Mirroring` tab click `Reclone`.

![Reclone repository](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/how-to/reclone-repo.png)

## Manually purge deleted repository data from disk

After a repository is deleted from Sourcegraph in the database, its data still remains on disk on gitserver so that in the event the repository is added again it doesn't need to be recloned. These repos are automatically removed when disk space is low on gitserver. However, it is possible to manually trigger removal of deleted repos in the following way:

**NOTE:** This is not available on Docker Compose deployments.

1. Browse to Site Admin -> Instrumentation -> Repo Updater -> Manual Repo Purge
2. You'll be at a url similar to `https://sourcegraph-instance/-/debug/proxies/repo-updater/manual-purge`
3. You need to specify a limit parameter which specifies the upper limit to the number of repos that will be removed, for example: `https://sourcegraph-instance/-/debug/proxies/repo-updater/manual-purge?limit=1000`
4. This will trigger a background process to delete up to `limit` repos, rate limited at 1 per second.

It's possible to see the number of repos that can be cleaned up on disk in Grafana using this query:

```
max(src_repoupdater_purgeable_repos)
```
