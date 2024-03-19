# How to troubleshoot a repo that is not being updated on Sourcegraph

A repository on Sourcegraph that fails to be updated could have different reasons. For example, the repository is a bad repository, the number of repositories causes significant load on the code host, a large repository, or code host permission issues etc. This guide will walk you through the steps you can take to investigate further when encountering this issue. 

## Prerequisites

This document assumes that you are a [site admin](https://docs.sourcegraph.com/admin) and **do not** have `disableAutoGitUpdates` set to `true` in your site configuration.

## Steps to investigate

1. First of all, you should identify how long has it been since the repo was last updated on Sourcegraph by going to the repository page > Settings > Mirroring
2. From there you can check:
   1. Last refreshed: Time when the repo was last synced
   2. Next scheduled update: Estimated time of when the repo will be updated next (this could change as it is determined by a [smart heuristic](https://docs.sourcegraph.com/admin/repo/update_frequency#repository-update-frequency))
   3. Queued for update: Its position in queue to be updated next
   4. Connection: Connection status to the repository
   5. Last sync log: Output from the most recent sync event
3. If clicking on the `Refresh Now` button has triggered the repository to be updated instantly for you then congratulations! You can now move on from this troubleshooting guide!
4. If clicking on the `Refresh Now` button does not work for you, try using webhooks following the instructions detailed in our [Repository Webhooks Docs](https://docs.sourcegraph.com/admin/repo/webhooks#webhook-for-manually-telling-sourcegraph-to-update-a-repository)
5. Look for errors related to this repository in gitserver logs, which should help you to determine the next best course of action.
6. Check the size of the repository. If it's a large repository, it may take a long time to sync and update. To find the size of your .git directory where the repository resides, you may use our [`git-stats` script](https://docs.sourcegraph.com/admin/monorepo#statistics). 
7. Check the allocated resources to see if the instance has enough resources to process the repository sync using our [Resource Estimator](https://docs.sourcegraph.com/admin/deploy/resource_estimator)
8. Check the code host connection from your Sourcegraph instance. If there is any issue, it needs to be resolved for the repository to sync and update.

## FAQs:

### What is a bad repository?

A bad repository is simiply a repository that Sourcegraph cannot handle. For example:

1. A large repository
   - This could result in gitserver running out of memory.
2. An empty/locked/disabled repository
   - For example, if you've created a repo repository GitHub and did not click the "initialize the repository for me" button, the repository would then become an empty repository as it has no commits at all.
3. A corrupted repository
   - The repository could be corrupted on the code host, or corrupted on disk.
