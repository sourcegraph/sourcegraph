# Assigned ownership

Owners (both users and teams) can be manually assigned. Assignment can be done on a repository, directory or a file level.

Assigned ownership propagates in a top-down manner: e.g.
- If an owner is assigned to the repository, they will be considered an owner for all files and directories of this repository.
- If an owner is assigned to some directory (e.g. `repo-name/src/abc`) within the repo, they will be considered an owner for all the directories and files within `repo-name/src/abc`, but not for any other directories and files higher up in the hierarchy.
- If an owner is assigned to some file (e.g. `repo-name/src/abc/a.go`) within the repo, they will be considered an owner only for this file.

## Who can assign ownership

Only site admins can assign ownership by default. For other users, [RBAC should be used](../admin/access_control/ownership.md) to grant assign ownership right.
[More](../admin/access_control/index.md) about RBAC in Sourcegraph.

## How to assign ownership

### Repository level ownership

Go to the repository page and click "Ownership" button.

<picture title="Repository page with Ownership button selected"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-ownership-1-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-ownership-1-lightv2.png"></picture>

1. Click "Add owner" button, the modal window will open.
2. Activate "New owner" panel.
3. Search for the user/team to assign ownership to.
4. Click "Add owner" button on the modal window.

<picture title="Add repository owner"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-2-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-2-light.png"></picture>

Owner is assigned successfully.

<picture title="Repository owner added"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-3-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-3-light.png"></picture>

### Directory level ownership

Go to any directory view (in our example it is `cmd/gitserver/internal`) and click "Show more" on Own panel.

<picture title="Repository page with Ownership button selected"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-1-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-1-light.png"></picture>

1. Click "Add owner" button, the modal window will open.
2. Activate "New owner" panel.
3. Search for the user/team to assign ownership to.
4. Click "Add owner" button on the modal window.

<picture title="Add repository owner"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-2-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-2-light.png"></picture>

Owner is assigned successfully.

<picture title="Repository owner added"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-3-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-3-light.png"></picture>

### File level ownership

Go to the blob view of any file (in our example it is `cmd/gitserver/internal/cleanup.go`).

1. Click on the Own bar in the top-right corner.
2. Click "Add owner" button in the bottom-right corner of an opened Ownership tab.
3. Activate "New owner" panel.
4. Search for the user/team to assign ownership to.
5. Click "Add owner" button on the modal window.

<picture title="Add repository owner"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-2-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-2-light.png"></picture>

Owner is assigned successfully.

<picture title="Repository owner added"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-3-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-dir-3-light.png"></picture>

## How to remove an assigned owner

### Repository and directory level

Go to the repository page and click "Ownership" button **or** Go to any directory view and click "Show more" on Own panel.

Click on "Remove ownership" button.

<picture title="Repository/directory owner removed"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-remove-repo-1-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-remove-repo-1-light.png"></picture>

### File level

Go to the blob view of any file and to the Ownership tab.

Click on "Remove ownership" button.

<picture title="File owner removed"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-remove-file-1-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-remove-file-1-light.png"></picture>

### Note on removing assigned owners

Owners can be removed only at the same level they were initially assigned. For example:
- If an owner is assigned to the repository, they should be removed from the repository level. They cannot be removed from any directory/file within this repo, even though they are considered owners of these directories/files.
- If an owner is assigned to some directory within the repo, they should be removed from the same directory. They cannot be removed from any subdirectory/file below this directory, even though they are considered owners of these subdirectories/files.
- If an owner is assigned to some file within the repo, they should be removed from the same file.

**"Remove ownership" buttons are disabled and the tooltip is shown when a user is attempting to remove an owner from the different place from where this owner has been assigned.**

<picture title="Remove ownership disabled tooltip"><img class="theme-dark-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-cannot-remove-dark.png"><img class="theme-light-only" src="https://storage.googleapis.com/sourcegraph-assets/docs/own/assigned-owners-cannot-remove-light.png"></picture>
