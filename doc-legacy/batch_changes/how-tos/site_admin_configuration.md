# Site admin configuration for Batch Changes

#### Setup Batch Changes 

1. Using Batch Changes requires a [code host connection](../../admin/external_services/index.md) to a supported code host (currently GitHub, Bitbucket Server / Bitbucket Data Center, GitLab, and Bitbucket Cloud).

1. (Optional) [Configure which users have access to Batch Changes](../../admin/access_control/batch_changes.md) <span class="badge badge-beta">Beta</span>. By default, all users can create and view batch changes, but only the batch change's author or a site admin can administer a given batch change.
    * Additionally, you can also [customize org settings](../../admin/config/batch_changes.md#enable-organization-members-to-administer) to allow members of an organization to share administration privileges over batch changes created in that organization.

1. (Optional) [Configure repository permissions](../../admin/permissions/index.md), which Batch Changes will respect.

1. [Configure credentials](configuring_credentials.md).

1. [Setup incoming webhooks](../../admin/config/webhooks/incoming.md) to make sure changesets sync fast. See [Batch Changes effect on codehost rate limits](../references/requirements.md#batch-changes-effect-on-code-host-rate-limits).

1. Configure any desired optional features, such as:
    * [Rollout windows](../../admin/config/batch_changes.md#rollout-windows), which control the rate at which Batch Changes will publish changesets on code hosts.
    * [Forks](../../admin/config/batch_changes.md#forks), which push branches created by Batch Changes onto forks of the upstream repository instead than the repository itself.
    * [Outgoing webhooks](../../admin/config/webhooks/outgoing.md), which publish events related to batch changes and changesets to enable deeper integrations with your other tools and systems.
    * [Auto-delete branch on merge/close](../../admin/config/batch_changes.md#automatically-delete-branches-on-merge-close), which automatically deletes branches created by Batch Changes when changesets are merged or closed.
    * [Commit signing for GitHub](../../admin/config/batch_changes.md#commit-signing-for-github), which signs commits created by Batch Changes via a GitHub App <span class="badge badge-beta">Beta</span>.

#### Disable Batch Changes
- [Disable Batch Changes](../explanations/permissions_in_batch_changes.md#disabling-batch-changes).
- [Disable Batch Changes for non-site-admin users](../explanations/permissions_in_batch_changes.md#disabling-batch-changes-for-non-site-admin-users).
