# How to apply privileged migrations

Postgres database migrations can be _privileged_ or _unprivileged_. The vast majority of migrations are unprivileged, and should require relatively low capabilities within the connected database. Some migrations are privileged and contain queries that require additional capabilities within the database. Currently, this includes the installation and modification (attached comments) of Postgres extensions.

If your Sourcegraph instance does not connect to the database with a superuser, then privileged migrations will fail. There are currently two methods to apply privileged migrations by hand to allow the installation or update of your Sourcegraph instance to proceed.

Note that these flags affect the `migrator` commands `up`, `upto`, `downto`, `upgrade`, and `downgrade`.

## Option 1: `--unprivileged-only`

Add the optional flag `--unprivileged-only` when [running the migrator](../updates/migrator/migrator-operations.md) against your Postgres instance. When the migration runner encounters an unapplied privileged migration, it will halt with an error message similar to the following.

```
❌ failed to run migration for schema "frontend": refusing to apply a privileged migration: schema "frontend" requires database migrations 1657908958 and 1657908965 to be applied by a database user with elevated permissions
The migration runner is currently being run with -unprivileged-only. The indicated migration is marked as privileged and cannot be applied by this invocation of the migration runner. Before re-invoking the migration runner, follow the instructions on https://docs.sourcegraph.com/admin/how-to/privileged_migrations. Please contact support@sourcegraph.com for further assistance.
```

This option is used to fail-fast upgrades that require manual user intervention. To allow the migrator to make additional progress, the privileged query/queries must be applied manually with a superuser (most commonly via a psql shell attached to the Postgres instance).

To be interactively instructed through the manual process, re-run the migrator with the [`--noop-privileged`](#option-2-noop-privileged) flag. Otherwise, you can manually [find and apply the target privileged migrations](dirty_database.md#2-run-the-sql-queries-to-finish-incomplete-migrations) and [manually add a migration log entry](dirty_database.md#3-add-a-migration-log-entry).

## Option 2: `--noop-privileged`

Add the optional flag `--noop-privileged` when [running the migrator](../updates/migrator/migrator-operations.md) against your Postgres instance. When the migration runner encounters an unapplied privileged migration, it will initially halt with an error message similar to the following.

```
❌ failed to run migration for schema "frontend": refusing to apply a privileged migration: apply the following SQL and re-run with the added flag `--privileged-hash=vp6EzmVmJfHgfchaShhJPUCq5v4=` to continue.
```

```sql
BEGIN;
-- Migration 1657908958
COMMENT ON EXTENSION citext IS 'first privileged migration';
-- Migration 1657908965
COMMENT ON EXTENSION citext IS 'second privileged migration';
COMMIT;
```

Manually apply the provided SQL with superuser access in the target schema. Then re-invoke the migrator with the suggested flag (e.g., `-privileged-hash=vp6EzmVmJfHgfchaShhJPUCq5v4=`). This value is unique to the set of privileged migration definitions and ensures that you have followed the instructions specific to this installation or upgrade.

The migrator may print multiple such error messages for different schemas that contain privileged migrations. In this case, multiple `-privileged-hash` flags are expected on the same re-invocation of the migrator.

Re-running the migrator should then succeed, skipping each of the migrations which were just manually applied.

```
WARN migrations.runner runner/run.go:309 The migrator assumes that the following SQL queries have already been applied. Failure to have done so may cause the following operation to fail. {"schema": "frontend", "sql": "BEGIN;\n\n-- Migration 1657908958\nCOMMENT ON EXTENSION citext IS 'first privileged migration';\n\n-- Migration 1657908965\nCOMMENT ON EXTENSION citext IS 'second privileged migration';\n\nCOMMIT;\n"}
WARN migrations.runner runner/run.go:339 Adding migrating log for privileged migration, but not applying its changes {"schema": "frontend", "migrationID": 1657908958, "up": true}
WARN migrations.runner runner/run.go:339 Adding migrating log for privileged migration, but not applying its changes {"schema": "frontend", "migrationID": 1657908965, "up": true}
✅ Schema(s) are up-to-date!
```
