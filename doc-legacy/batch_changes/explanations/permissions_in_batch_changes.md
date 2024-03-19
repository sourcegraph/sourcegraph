# Permissions in Batch Changes

You can customize access to a batch change and propose changes to repositories with varying permission levels. Other people see the batch change's proposed changes to a repository if they can view that repository; otherwise, they can see only limited, non-identifying information about the change.

## Permission levels for Batch Changes

The permission levels for a batch change are:

- **Read:** For people who need to view the batch change.
- **Admin:** For people who need full access to the batch change, including editing, closing, and deleting it.

To see the batch change's proposed changes on a repository, a person *also* needs read access to that specific repository. Read or admin access on the batch change does not (by itself) entitle a person to viewing all of the batch changes's changesets. For more information, see "[Repository permissions for Batch Changes](#repository-permissions-for-batch-changes)".

Site admins have admin permissions on all batch changes.

Users have admin permissions on the batch changes they created and read access to other batch changes.

### Namespaces

A batch change can be created under either a user or organization namespace:

- User namespace: Only the user who created the batch change has admin access. Other users have read access.

- Organization namespace:
  - By default, all members of the organization have read access.
  - If the `orgs.allMembersBatchChangesAdmin` setting is enabled, all organization members also have admin access.
  - If that setting is disabled, only the user who created the batch change has admin access. Other organization members have read access.

### Batch Change access for each permission level

Batch change action | Read | Admin
--------------- | :--: | :----:
View batch change name and description<br/><small class="text-muted">Also applies to viewing the input branch name, created/updated date, and batch change status</small> | ⬤ | ⬤
View burndown chart (aggregate changeset statuses over time) | ⬤ | ⬤
View list of patches and changesets | ⬤ | ⬤
View diffstat (aggregate count of added/changed/deleted lines) | ⬤ | ⬤
View error messages (related to creating or syncing changesets) |  | ⬤
Edit batch spec |  | ⬤
Update batch change patches (and changesets on code hosts) |  | ⬤
Publish changesets to code host |  | ⬤
Add/remove existing changesets to/from batch change |  | ⬤
Refresh changeset statuses |  | ⬤
Close batch change |  | ⬤
Delete batch change |  | ⬤

Authorization for all actions is also subject to [repository permissions](#repository-permissions-for-batch-changes).

## Setting and viewing permissions in Batch Changes

When you create a batch change, you are given admin permissions on the batch change.

All users are automatically given read permissions to a batch change. Granular permissions are not yet supported. Assigning admin permissions on a batch change to another person or to all organization members is not yet supported. Transferring ownership of a batch change is not yet supported.

## Code host interactions in Batch Changes

Interactions with a code host are made possible by [configuring credentials](../how-tos/configuring_credentials.md) for that code host. When publishing a changeset to the code host with Batch Changes, the author and permissions will reflect the token used (e.g., on GitHub, the pull request author will be you).

## Repository permissions for Batch Changes

Your [repository permissions](../../../admin/permissions/index.md) determine what information in a batch change you can view. You can only see a batch change's proposed changes to a repository if you have read access to that repository. Read or admin permissions on a batch change does not (by itself) permit you to view all of the batch change's changes.

When you view a batch change, you can see a list of patches and changesets. For each patch and changeset:

- **If you have read access** to the repository for a patch or changeset, you can see the diff, changeset title, changeset link, detailed status, and other information.
- **If you do not have read access** to the repository for a patch or changeset, you can only see the status, last-updated date, and whether an error occurred (but not the error message). You can't see the diff, changeset title, changeset link, repository name, or any other information.

When you perform any batch change operation that involves repositories or code host interaction, your current repository permissions are taken into account.

- Creating, updating, or publishing a batch change; or publishing a single changeset: You must have access to view the repositories and the configured token must have the rights to push a branch and create the changesets on your code host (e.g., the [```repo```](https://docs.github.com/en/developers/apps/building-oauth-apps/scopes-for-oauth-apps#available-scopes) scope gives a GitHub access token the rights to push to branches and open pull requests on GitHub repositories).
- Adding existing changesets to a batch change: You must have read access to the existing changeset's repository.
- Closing or deleting a batch change: If you choose to also close associated changesets on the code host, you must have access to do so on the code host. If you do not have access to close a changeset on the code host, the changeset will remain in its current state. A person with repository permissions for the remaining changesets can view them and manually close them.

Your repository permissions can change at any time:

- If you've already published a changeset to a repository that you no longer have access to, then you won't be able to view its details or update it in your batch change. The changeset on the code host will remain in its current state. A person with permissions to the changeset on the code host will need to manually manage or close it. When possible, you'll be informed of this when updating a batch change that contains changesets you've lost access to.
- You need access to all repositories mentioned in a batch change plan to use it when updating a batch change.

If you are not permitted to view a repository on Sourcegraph, then you won't be able to perform any operations on it, even if you are authorized on the code host.

## Disabling Batch Changes

A site admin can disable Batch Changes for the entire site by setting the [site configuration](../../../admin/config/site_config.md) property `"batchChanges.enabled"` to `false`. <!--- TODO:check --->

## Disabling Batch Changes for non-site-admin users

A site admin can disable batch changes for normal users by setting the [site configuration](../../../admin/config/site_config.md) property `"batchChanges.restrictToAdmins"` to `true`. <!--- TODO:check --->
