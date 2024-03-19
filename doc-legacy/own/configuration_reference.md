# Configuration reference

## Ownership signals and background compute

Ownership signals are used to guide the assignment of ownership. These are:

*   **Recent contributors signal** counts files modified by commits in the last 90 days.
*   **Recent views signal** counts file views within Sourcegraph in the last 90 days.

Both of these signals are computed by background tasks.
The values of signals are aggregted and bubble up the file tree.
That is, for the Ownership data displayed `/a/` directory, all descendant file signals contribute.
For instance contributions and views of `/a/b/c.go`.

The **Site admin > Code graph > Ownership signals** page allows enabling and disabling each signal individually.
These need to be explicitly enabled by site admin in order for signals to surface in the UI.

<picture title="Site admin ownership configuration page">
  <img class="theme-dark-only" src="https://sourcegraphstatic.com/own-signals-configuration-dark.png">
  <img class="theme-light-only" src="https://sourcegraphstatic.com/own-signals-configuration.png">
</picture>

### Repository filtering

In some cases ownership signals need to be disabled for specified repositories:

*   Ownership signals are not desired for certain repositories.
*   Computing signals is expensive due to size of some repositories.

**Exclude repositories** section under each signal in **Site admin > Code graph > Ownership signals** allows to match repositories to exclude from computing ownership signals.
Materialized list of excluded repositories is displayed for feedback.

<picture title="Site admin ownership configuration page">
  <img class="theme-dark-only" src="https://sourcegraphstatic.com/own-signals-exclude-dark.png">
  <img class="theme-light-only" src="https://sourcegraphstatic.com/own-signals-exclude.png">
</picture>

## Analytics

In order to measure how many files have owners, Sourcegraph exposes analytics through **Site admin > Analytics > Own**.
These present percentage of files that have:

*   Any ownership associated,
*   assigned ownership (through the UI),
*   ownership via matching rule in CODEOWNERS file.

Analytics data is computed periodically.
The background process for computing analytics data has to be enabled explicitly through **Site admin > Code graph > Ownership signals**.
This is because the process can become computationally expensive.

## Assigned ownership access control

In order to grant users the ability to assign ownership, please use [ownership permission](../admin/access_control/ownership.md) in role-based access control.
This is a coarse-grained permission, allowing users to assign ownership throughout the instance.
At this point there is no finer-grained ownership assigning access control.

## Disabling ownership in the UI

Ownership data is displayed in various places in Code search user interface, among others:

*   in a card on repository and directory pages,
*   in a top bar on file page.

If needed, the visibility of ownership data embedded in Code search UI can be disabled by creating a boolean feature flag `enable-ownership-panels` and setting its value to `false`.
