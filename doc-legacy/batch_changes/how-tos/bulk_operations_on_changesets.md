# Bulk operations on changesets

Bulk operations allow a single action to be performed across many changesets in a batch change.

## Selecting changesets for a bulk operation

1. Click the checkbox next to a changeset in the list view. You can select all changesets you have permission to view.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/select_changeset.png" class="screenshot">
1. If you like, select all changesets in the list by using the checkbox in the list header.
    
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/select_all_changesets_in_view.png" class="screenshot">
    
    If you want to select _all_ changesets that meet the filters and search currently set, click the **(Select XX changesets)** link in the header toolbar.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/select_all_changesets.png" class="screenshot">
1. In the top right, select the action to perform on all the changesets.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/select_bulk_operation_type.png" class="screenshot">

1. Once changesets are selected, a query is made to determine the bulk operations that can be applied to the selected changesets.

## Supported types of bulk operations

Depending on the changesets selected, different types of bulk operations can be applied to the selected changesets. For a bulk operation to be available, it has to be applicable to all the selected changesets.

Below is a list of supported bulk operations for changesets and the conditions with which they're applicable:

- Commenting: Post a comment on all selected changesets. This can be particularly useful for pinging people, reminding them to take a look at the changeset, or posting your favorite emoji 🦡.
- Detach: Detach a selection of changesets from the batch change to remove them from the archived tab.
- Re-enqueue: Re-enqueues the pending changes for all selected changesets that failed.
- <span class="badge badge-experimental">Experimental</span> Merge: Tries to merge the selected changesets on the code hosts. Due to the nature of changesets, there are many states in which a changeset is not mergeable. This won't break the entire bulk operation, but single changesets may not be merged after the run for this reason. The bulk operations tab lists those where merging failed below the bulk operation in that case. In the confirmation modal, you can select to merge using the squash merge strategy. This is supported on GitHub, GitLab, and Bitbucket Cloud, but not on Bitbucket Server / Bitbucket Data Center. In this case, regular merges are always used for merging the changesets.
- Close: Tries to close the selected changesets on the code hosts.
- Publish: Publishes the selected changesets, provided they don't have a [`published` field](../references/batch_spec_yaml_reference.md#changesettemplate-published) in the batch spec. You can choose between draft and normal changesets in the confirmation modal.

## Monitoring bulk operations

On the **Bulk operations** tab, you can view all bulk operations that have been run over the batch change. Since bulk operations can involve quite some operations to perform, you can track the progress, and see what operations have been performed in the past.

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/bulk_operations_tab.png" class="screenshot">
