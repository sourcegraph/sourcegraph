# How to troubleshoot an unfinished migration

This document will take you through how to resolve an "unfinished migration" error.

Out-of-band migrations run in the background of your Sourcegraph instance and slowly migrate data from an "old" format into a "new" format (depending on the type of data). All out-of-band migrations will eventually become deprecated at a specific version. Sourcegraph instances pre-deprecation can read (and perhaps write) both the "old" and "new" formats. However, Sourcegraph instances post-deprecation only guarantee the ability to read and write the "new" format. An incomplete migration means that some data is no longer understood by the application.

If Sourcegraph detects that there is unmigrated data of an already-deprecated out-of-band migration, it will fail to start-up successfully with an error similar to:

```
ERROR: Unfinished migrations. Please revert Sourcegraph to the previous version and wait for the following migrations to complete.

- migration 12 expected to be at 100% (at 63.27%)
- migration 16 expected to be at 100% (at 41.84%)
```

## Resolution

If you were performing a [standard upgrade](../updates/index.md#standard-upgrades) between two minor versions, then the suggested action is to perform an infrastructure rollback and continue running the previous instance version until the violating out-of-band migrations have completed. The progress of the migrations can be checked [in the UI](#checking-progress). Older versions of Sourcegraph may have performed schema migrations prior to this check, but a schema rollback should not be necessary as our database schemas are backwards-compatible with one minor version.

Alternatively to rolling back and waiting, the unfinished migrations can be run directly via the `migrator`. See the [command documentation](../updates/migrator/migrator-operations.md#run-out-of-band-migrations) for additional details.

[Multi-version upgrades](../updates/index.md#multi-version-upgrades) and downgrade operations ensure that the required out-of-band migrations have completed or finished rolling back. If this is not the case, contact support as it indicates a non-obvious error in your environment or a bug Sourcegraph's migration tooling.

As an emergency escape hatch, the environment variable `SRC_DISABLE_OOBMIGRATION_VALIDATION` can be set to `true` on the `frontend` and `worker` services to disable the startup check. This is not recommended as it may result in broken features or data loss.

## Checking progress

Progress of out-of-band migrations can be checked via the UI. Navigate to the `Site Admin > Maintenance > Migrations` page to see an overview of progress and errors of each active out-of-band migration.

![Unfinished migration warning](https://storage.googleapis.com/sourcegraph-assets/oobmigration-warning-4.0.png)

An explicit warning will be shown if the progress of some out-of-band migrations would cause issues with a standard upgrade to the next version.

If an out-of-band migration is not making progress or there are errors associated with it, contact support.

## Further resources

* [Sourcegraph - Upgrading Sourcegraph to a new version](../updates/index.md)
