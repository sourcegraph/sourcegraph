# How to troubleshoot a dirty database

> NOTE: If you are on a version **strictly lower than** Sourcegraph 3.37.0, see the [legacy dirty database documentation](./dirty_database_pre_3_37.md). The following documentation applies only to Sourcegraph instances version 3.37.0 and above.

This document will take you through how to resolve a 'dirty database' error.

During instance startup, the `migrator` process will run any schema migrations necessary to bring the database up to the shape the code expects. If an error is encountered during this startup sequence or a multi-version upgrade, or if execution of either actions were interrupted, the schema will be marked as dirty and will require some form of user intervention.

Such an error will look something like this:

```log
INFO[02-08|00:40:55] Checked current version
  schema=frontend
  appliedVersions=[1528395834 1528395835 ... 1528395970 1528395971]
  pendingVersions=[1644515056]
  failedVersions=[]
  error: 1 error occurred:
	  * dirty database: schema "frontend" marked the following migrations as failed: 1644515056

The target schema is marked as dirty and no other migration operation is seen running on this schema. The last migration operation over this schema has failed (or, at least, the migrator instance issuing that migration has died). Please contact support@sourcegraph.com for further assistance.
```

## Prerequisites

* This document assumes that you are installing Sourcegraph or were attempting an upgrade when an error occurred.
* **NOTE: If you encountered this error during an upgrade, ensure you followed the [proper step upgrade process documented here.](https://docs.sourcegraph.com/admin/updates) If you skipped a minor version during an upgrade, you will need to revert back to the last minor version your instance was on before following the steps in this document.**

The following procedure requires that you are able to execute commands from inside the database container. Learn more about shelling into [kubernetes](../deploy/kubernetes/operations.md#access-the-database), [docker-compose](../deploy/docker-compose/index.md#access-the-database), and [Sourcegraph single-container](../deploy/docker-single-container/index.md#access-the-database) instances at these links.

## Steps to resolve

### 0. Attempt re-application

Some classes of errors can be successfully retried via a manual invocation of the failed `migrator` command. If the previous operation was interrupted due to a network issue, a timeout, or a crashed/restarted database host, or if the error that occurred was due to a transient error such as a SQL deadlock, then re-application of the migration is likely to succeed.

When [re-running the migrator](../updates/migrator/migrator-operations.md), add the optional flag `--ignore-single-dirty-log` to attempt re-application of a previously failed migration, and add the optional flag `--ignore-single-pending-log` to to attempt re-application of a migration which was never completed by a previous invocation of the `migrator`. Both flags apply to the `migrator` commands `up`, `upto`, `downto`, `upgrade`, and `downgrade`. These flags only allow re-application of the **next** migration in the sequence (and multiple sequential failures will require multiple invocations).

**DO NOT** set either of these flags permanently on a `migrator` attached as an init container (in Kubernetes) or as a dependent container (in Docker Compose), as it may allow mutation of the database schema with non-mutual access. Concurrent modification may result in greater error frequency (at best) and data corruption (at worst). These flags should only be used on a manual invocation of a `migrator` command.

### 1. Identify incomplete migration

_First, some background:_

When migrations run, the `migration_logs` table is updated. Before each migration attempt, a new row is inserted indicating the migration version and direction and the start time. Once the migration is complete (or fails), the row is updated with the finished time and message with details about any error that occurred.

A failed migration may have explicitly failed due to a SQL/environment error, which allowed the migrator instance to write an error message to the `migration_logs` table. A failed migration may also be left _pending_ if the migrator instance running that migration has disappeared. To handle this case, the validation mechanism that runs on app startup will wait for running migrators to complete their current work. If we do not detect a migrator instance performing any work, we'll correctly interpret those migration logs as implicitly failed. The following example does just this (note the values of the `pendingVersions` and `failedVersions` log fields).

_End background!_

In order to identify the migration that needs to be run, note the particular versions called out by the error message in the `migrator` or one of the application servers on startup.

```
INFO[02-08|00:40:55] Checked current version
  schema=frontend
  appliedVersions=[1528395834 1528395835 ... 1528395970 1528395971]
  pendingVersions=[1644515056]
  failedVersions=[]
  error: 1 error occurred:
	  * dirty database: schema "frontend" marked the following migrations as failed: 1644515056

The target schema is marked as dirty and no other migration operation is seen running on this schema. The last migration operation over this schema has failed (or, at least, the migrator instance issuing that migration has died). Please contact support@sourcegraph.com for further assistance.
```

In this example, we need to re-apply the migration `1644515056` on the `frontend` schema, which is explicitly called out in the error message. **We'll note this number for use in step 2.**

The `migration_logs` table can also be queried directly. The following query gives an overview of the most recent migration attempts broken down by version.

```sql
WITH ranked_migration_logs AS (
	SELECT
		migration_logs.*,
		ROW_NUMBER() OVER (PARTITION BY schema, version ORDER BY started_at DESC) AS row_number
	FROM migration_logs
)
SELECT *
FROM ranked_migration_logs
WHERE row_number = 1
ORDER BY version
```

### 2. Run the sql queries to finish incomplete migrations

Migration definitions for each database schema can be found in the children of the [`migrations/` directory](https://github.com/sourcegraph/sourcegraph/tree/main/migrations).

**Find the target migration with the version number identified in [step 1](#1-identify-incomplete-migration)**.

**Run the code from the identified `<version>/up.sql` file explicitly using the `psql` CLI**. For example, if we take `version=1644515056` as an example, we can run the contents of [up migration file](https://github.com/sourcegraph/sourcegraph/blob/b20107113548ed7eeb8ba22d1fdb41e8d692cf18/migrations/frontend/1644515056/up.sql) via psql.

```bash
$ psql -h ... -U sg
sg@sourcegraph > BEGIN;

ALTER TABLE IF EXISTS org_invitations ALTER COLUMN recipient_user_id DROP NOT NULL;
ALTER TABLE IF EXISTS org_invitations
  ADD CONSTRAINT either_user_id_or_email_defined CHECK ((recipient_user_id IS NULL) != (recipient_email IS NULL));

COMMIT;
Time: 0.283 ms
Time: 32.646 ms
Time: 25.477 ms
Time: 0.762 ms
```

It’s possible that one or more commands from the migration ran successfully already. In these cases you may need to run the sql transaction in pieces. For example if a migration file creates multiple indexes and one index already exists you'll need to manually run this transaction skipping that line or adding `IF NOT EXISTS` to the transaction.

If you're running into errors such as being unable to create a unique index due to duplicate values, please contact support at <mailto:support@sourcegraph.com> or via your enterprise support channel for further assistance. There may be other error cases that don't have an easy admin-only resolution, in these cases please contact us for engineering support.

### 3. Add a migration log entry

**Ensure the migration applied, then signal that the migration has been run**. Run the `migrator` instance against your database to create an explicit migration log. For the following, consult the [Kubernetes](../updates/migrator/migrator-operations.md#kubernetes), [Docker-compose](../updates/migrator/migrator-operations.md#docker--docker-compose), or [local development](../updates/migrator/migrator-operations.md#local-development) instructions on how to manually run database operations. The specific migrator command to run is:

- For Kubernetes: replace container args with `["add-log", "-db=<schema>", "-version=<version>"]`
- For Docker-compose: replace container args with `"add-log" "-db=<schema>" "-version=<version>"`
- For local development: run `sg add-log -db=<schema> -version=<version>` in a clone of [sourcegraph/sourcegraph](https://github.com/sourcegraph/sourcegraph)

Adding this migration log entry indicates to future migrator and application instances that a complete application of the migration at that version has been run.

**Do not mark the migration table as clean if you have not verified that the migration was successfully completed.** Checking to see if a migration ran successfully requires looking at the migration’s `sql` file, and verifying that `sql` queries contained in the migration file have been applied to tables in the database. You can get a description of a table and its associated indexes quickly using the `\d <table name>` `psql` shell command (note lack of semicolon). Using this information, you can determine whether a table exists, what columns it contains, and what indexes on it exist. Use this information to determine if commands in a migration ran successfully before adding a migration log entry.

**Start Sourcegraph again and any remaining migrations should apply automatically, otherwise repeat this procedure again starting from the [Identify incomplete migration](#1-identify-incomplete-migration) step.**
