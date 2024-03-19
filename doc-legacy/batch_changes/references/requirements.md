# Requirements

Batch Changes has requirements for the Sourcegraph server version, its connected code hosts and developer environments.

## Sourcegraph server

While the latest version of Sourcegraph server is always recommended, version 3.22 is the minimum version required to run Batch Changes.

## Code hosts

Batch Changes is compatible with the following code hosts:

* Github.com
* Github Enterprise 2.20 and later
* GitLab 12.7 and later (burndown charts are only supported with 13.2 and later)
* Bitbucket Server 5.7 and later, Bitbucket Data Center 7.6 and later
* Bitbucket Cloud (bitbucket.org)
* Azure DevOps Services
* Gerrit 3.1.7 and later
* <span class="badge badge-beta">Beta</span> Perforce

In order for Sourcegraph to interface with these, admins and users must first [configure credentials](../how-tos/configuring_credentials.md) for each relevant code host.

> WARNING: Currently, for customers on an instance of GitHub Enterprise Cloud that uses [SSH certificate authorities](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-git-access-to-your-organizations-repositories/about-ssh-certificate-authorities) and requires SSH certificates to authenticate, we are unable to provide a means of authenticating Batch Changes to your code host.

### Batch Changes effect on code host rate limits

For each changeset, Sourcegraph periodically makes API requests to its code host to update its status. Sourcegraph intelligently schedules these requests to avoid overwhelming the code host's rate limits. In environments with many open batch changes, this can result in outdated changesets as they await their turn in the update queue.

We **highly recommend** enabling webhooks on code hosts where batch change changesets are created. Doing so removes the lag time in updating the status of a changeset and reduces the API requests associated with large batch changes. We have instructions for each supported code host:

* [GitHub](../../admin/external_service/github.md#webhooks)
* [Bitbucket / Bitbucket Data Center](../../admin/external_service/bitbucket_server.md#webhooks)
* [GitLab](../../admin/external_service/gitlab.md#webhooks)

If you are unable to enable webhooks, you can disable the warning Sourcegraph displays when viewing batch changes by setting the `batchChanges.disableWebhooksWarning` [site configuration setting](../../admin/config/site_config.md) to `true`.

### A note on Batch Changes effect on CI systems

Batch Changes makes it possible to create changesets in tens, hundreds, or thousands of repositories. Opening and updating these changesets may trigger many checks or continuous integration jobs, and in turn may stress the resources allotted to these systems. Batch Changes supports [partial publishing for changesets](../how-tos/publishing_changesets.md#publishing-a-subset-of-changesets) to help mitigate these issues. You may also consider publishing your changesets at times of low activity.  

## Requirements for batch change creators

* Latest version of the [Sourcegraph CLI `src`](../../cli/index.md)
  * `src` is supported on Linux or Intel macOS
  * <span class="badge badge-beta">Beta</span> ARM (eg. M1) macOS support is in beta
* Docker
  * MacOS:
      * If using Docker 3.x, ensure your version is at least 3.0.1
      * In 3.x versions, the gRPC setting must be disabled
  * You must be able to run `docker` commands as the same user `src` is running as. On Linux, this may require either `sudo` or adding your user to the `docker` group.
  * Note: podman is not supported ([#855](https://github.com/sourcegraph/src-cli/issues/855))
* Disk space
  * The required disk space is equal to each batch change's largest repository plus any dependencies or requirements specified by the run steps, times the number of parallel jobs.
      * The default number of parallel jobs defaults to the number of CPU cores on the system running src-cli. This setting can be configured with the [`-j` flag when running `src batch apply` or `src batch preview`](../../cli/references/batch/apply.md).
  * Disk space is also required for the generated patches. This requirement *is* cumulative across each repository altered.
* Git
