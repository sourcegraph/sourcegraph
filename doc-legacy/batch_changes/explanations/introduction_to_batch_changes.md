# Introduction to Batch Changes

## Overview

Batch Changes let you make large-scale code changes across many repositories and code hosts. Batch Change lets you create pull requests on all affected repositories, and it tracks their progress until they're all merged. You can preview the changes and update them at any time.

People usually use Batch Changes to make the following kinds of changes:

- Cleaning up common problems using linters.
- Updating uses of deprecated library APIs.
- Upgrading dependencies.
- Patching critical security issues.
- Standardizing build, configuration, and deployment files.

A batch change tracks all of its changesets (a generic term for pull requests or merge requests) for updates to:

- Status: open, merged, or closed
- Checks: passed (green), failed (red), or pending (yellow)
- Review status: approved, changes requested, pending, or other statuses (depending on your code host or code review tool)

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/batch_tracking_sourcegraph_prs.png" class="screenshot">

You can see the overall trend of a batch change in the burndown chart, which shows the proportion of changesets that have been merged over time since the batch change was created.
<img src="https://sourcegraphstatic.com/docs/images/batch_changes/batch_tracking_sourcegraph_prs_burndown.png" class="screenshot">

## Supported code hosts and changeset types

The generic term **changeset** is used to refer to any of the following:

- GitHub pull requests.
- Bitbucket Server / Bitbucket Data Center and Bitbucket Data Center pull requests.
- GitLab merge requests.
- Bitbucket Cloud pull requests.
- Gerrit changes.
- <span class="badge badge-beta">Beta</span> Perforce changelists.
- Phabricator diffs (not yet supported).

A single batch change can span many repositories and many code hosts.

## Concepts

- {#batch-change} A **batch change** is a group of related changes to code, along with a title and description.
- {#batch-spec} A **batch spec** is a YAML file describing the batch change: repositories to change, commands to run, and a template for changesets and commits. You describe your high-level intent in the batch spec, such as "lint files in all repositories with a `package.json` file".
- {#changesets} The batch change has associated **changesets**, which is a generic term for pull requests, merge requests, or any other reviewable chunk of code. (Code hosts use different terms for this, which is why we chose a generic term.)
- {#published-changeset} A **published changeset** means the commit, branch, and changeset have been created on the code host. An **unpublished changeset** is just a preview that you can view in the batch change but does not exist on the code host yet.
- {#spec} A **spec** (batch spec or changeset spec) is a "record of intent". When you provide a spec for a thing, the system will continuously try to reconcile the actual thing with your desired intent (as described by the spec). This involves creating, updating, and deleting things as needed.
- {#changeset-spec} A batch change has many **changeset specs**, which are produced by executing the batch spec (i.e., running the commands on each selected repository) and then using its changeset template to produce a list of changesets, including the diffs, commit messages, changeset title, and changeset body. You don't need to view or edit the raw changeset specs; you will edit the batch spec and view the changesets in the UI.
- {#batch-changes-controller} The **batch change controller** reconciles the actual state of the batch change's changesets on the code host so that they match your desired intent (as described in the changeset specs).

To learn about the internals of Batch Changes, see [Batch Changes](../../../dev/background-information/batch_changes/index.md) in the developer documentation.

## Ownership

When a user is deleted, their Batch Changes become inaccessible in the UI but the data is not permanently deleted.
This allows recovering the Batch Changes if the user is restored.

However, if the user deletion is permanent, deleting both account and data, then the associated Batch Changes are also permanently deleted from the database. This frees storage space and removes dangling references.

## Known issues

- Batch Changes currently support **GitHub**, **GitLab** and **Bitbucket Server and Bitbucket Data Center** repositories. If you're interested in using Batch Changes on other code hosts, [let us know](https://sourcegraph.com/contact).
- {#server-execution} Batch change steps are run locally (in the [Sourcegraph CLI](https://github.com/sourcegraph/src-cli)) or [server-side](https://docs.sourcegraph.com/batch_changes/explanations/server_side) `Beta`. For this reason, the APIs for creating and updating a batch change require you to upload all of the changeset specs (which are produced by executing the batch spec locally). Also see [how scalable is Batch Changes](../references/faq.md#how-scalable-is-batch-changes-how-many-changesets-can-i-create).
- It is not yet possible for multiple users to edit the same batch change that was created under an organization.
- It is not yet possible to reuse a branch in a repository across multiple batch changes.
- The only type of user credential supported by Sourcegraph right now is a [personal access token](../how-tos/configuring_credentials.md), either per user, or via a global service account. Further credential types may be supported in the future.
