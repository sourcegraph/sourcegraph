# Closing or deleting a batch change

You can close a batch change when you don't need it anymore, when all changes have been merged, or when you decide not to proceed with making changes. A closed batch change still appears in the [batch changes list](viewing_batch_changes.md). To completely remove it, you can delete the batch change.

Any person with [admin access to the batch change](../explanations/permissions_in_batch_changes.md#permission-levels-for-batch-changes) can close or delete it.

## Closing a batch change

1. Click the <img src="../batch_changes-icon.svg" alt="Batch Changes icon" /> Batch Changes icon in the top navigation bar.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/batch_changes_icon_in_menu.png" class="screenshot">
1. In the list of batch changes, click the batch change that you'd like to close or delete.
1. In the top right, click the **Close** button.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/closing_batch_change_close_icon.png" class="screenshot">
1. Select whether you want to close all of the batch change's open changesets (e.g., closing all associated GitHub pull requests on the code host).

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/closing_batch_change_close_changesets.png" class="screenshot">
1. Click **Close batch change**.

Once a batch change is closed it can't be updated or reopened anymore.

## Delete a batch change

1. First, close the batch change.
1. Instead of a "Close batch change" button you'll now see a **Delete** button.

    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/closing_batch_change_deleting.png" class="screenshot">
1. Click **Delete**.

The batch change is now deleted from the Sourcegraph instance. The changesets it created (and possibly closed) will still exist on the code hosts, since most code hosts don't support deleting changesets.
