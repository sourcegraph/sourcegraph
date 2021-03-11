# Requirements

Batch Changes has requirements for the Sourcegraph server version, its connected code hosts and developer environments.

## Code hosts

Batch Changes is compatible with the following code hosts:

* Github.com
* Github Enterprise 2.20 and later
* GitLab 12.7 and later (burndown charts are only supported with 13.2 and later)
* Bitbucket Server 5.7 and later

>NOTE: Currently, in code hosts configured [to use SSH to clone repositories via the `gitURLType` setting](../../admin/repo/auth.md), only site admins will be able to publish changesets.

### Batch Changes effect on code host rate limits

For each changeset, Sourcegraph periodically makes API requests to its code host to update its status. Sourcegraph intelligently schedules these requests to avoid overwhelming the code host's rate limits. In environments with many open batch changes, this can result in outdated changesets as they await their turn in the update queue.

We **highly recommend** enabling webhooks on code hosts where batch change changesets are created. Doing so removes the lag time in updating the status of a changeset and reduces the API requests associated with large batch changes. We have instructions for each supported code host:

* [GitHub](../../admin/external_service/github.md#webhooks)
* [Bitbucket Server](../../admin/external_service/bitbucket_server.md#webhooks)
* [GitLab](../../admin/external_service/gitlab.md#webhooks)

### A note on Batch Changes effect on CI systems

Batch Changes makes it possible to create changesets in tens, hundreds, or thousands of repositories. Opening and updating these changesets may trigger many checks or continuous integration jobs, and in turn may stress the resources allotted to these systems. Batch Changes supports [partial publishing for changesets](../how-tos/publishing_changesets.md#publishing-a-subset-of-changesets) to help mitigate these issues. You may also consider publishing your changesets at times of low activity.  

## Sourcegraph server

While the latest version of Sourcegraph server is always recommended, version 3.22 is the minimum version required to run Batch Changes.

## Requirements for batch change creators

* Latest version of the [Sourcegraph CLI `src`](../../cli/index.md)
  * `src` is supported on Linux or Intel macOS; Windows and ARM (eg M1) macOS support is experimental
* Docker
  * MacOS:
      * If using Docker 3.x, ensure your version is at least 3.0.1
      * In 3.x versions, the gRPC setting must be disabled
  * You must be able to run `docker` commands as the same user `src` is running as. On Linux, this may require either `sudo` or adding your user to the `docker` group.
* Disk space
  * The required disk space is equal to each batch change's largest repository plus any dependencies or requirements specified by the run steps, times the number of parallel jobs.
      * The default number of parallel jobs defaults to the number of CPU cores on the system running src-cli. This setting can be configured with the [`-j` flag when running `src batch apply` or `src batch preview`](../../cli/references/batch/apply.md).
  * Disk space is also required for the generated patches. This requirement *is* cumulative across each repository altered.
* Git
