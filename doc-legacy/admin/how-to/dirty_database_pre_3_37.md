# How to troubleshoot a dirty database

> NOTE: This document refers to Sourcegraph instances with a version **strictly lower than** 3.37.0. For instructions on dealing with a dirty database for newer Sourcegraph instances, see [the updated documentation](./dirty_database.md).

This document will take you through how to resolve a 'dirty database' error. During an upgrade, the `pgsql`, `codeintel-db`, and `codeinsights-db` databases must be migrated. If the upgrade was interrupted during the migration, this can result in a 'dirty database' error.

The error will look something like this:

```log
ERROR: Failed to migrate the DB. Please contact support@sourcegraph.com for further assistance: Dirty database version 1528395797. Fix and force version.
```
Resolving this error requires discovering which migration file failed to run, and manually attempting to run that migration.

## Prerequisites

* This document assumes that you are installing Sourcegraph or were attempting an upgrade when an error occurred.
* **NOTE: If you encountered this error during an upgrade, ensure you followed the [proper step upgrade process documented here.](https://docs.sourcegraph.com/admin/updates) If you skipped a minor version during an upgrade, you will need to revert back to the last minor version your instance was on before following the steps in this document.**

The following procedure requires that you are able to execute commands from inside the database container. Learn more about shelling into [kubernetes](https://docs.sourcegraph.com/admin/deploy/kubernetes/operations#access-the-database), [docker-compose](https://docs.sourcegraph.com/admin/deploy/docker-compose/operations#access-the-database), and [Sourcegraph single-container](https://docs.sourcegraph.com/admin/deploy/docker-single-container/operations#access-the-database) instances at these links.

## TL;DR Steps to resolve

_These steps pertain to the frontend database (pgsql) and are meant as a quick read for admins familiar with sql and database administration, for more explanation and details see the [detailed steps to resolution](#detailed-steps-to-resolve) below._

1. **Check the schema version in `psql` using the following query: `SELECT * FROM schema_migrations;`. If it's dirty, note the migration's version number.**
2. **Find the up migration with that migration's version number in [https://github.com/sourcegraph/sourcegraph/tree/\<YOUR-SOURCEGRAPH-VERSION\>/migrations/frontend](https://github.com/sourcegraph/sourcegraph/tree/main/migrations/frontend), making sure to go to \<YOUR-SOURCEGRAPH-VERSION\>**
   * _Note: migrations in this directory are specific to the `pgsql` frontend database, learn about other databases in the [detailed steps to resolution](#detailed-steps-to-resolve)_
3. **Run the code there explicitly.**
4. **Manually clear the dirty flag on the `schema_migrations` table.**
5. **Start up again and the remaining migrations should succeed, otherwise repeat.**

## Detailed Steps to resolve

### 1. Identify incomplete migration

When migrations run, the `schema_migrations` table is updated to show the state of migrations. The `dirty` column, when set `t` (true), _indicates a migration was attempted but did not complete successfully_ (either did not yet complete or failed to complete), and the `version` column indicates the version of the migration the database is on (when not dirty), or attempted to migrate to (when dirty). On startup, the frontend will not start if the `schema_migrations` `dirty` column is set to `t`.

**Check schema version, by querying the database version table:** `SELECT * FROM schema_migrations;` **If it's dirty, then note the version number for use in step 2.**

Example:
```sql
SELECT * FROM schema_migrations;
version | dirty
------------+-------
1528395539 | t
(1 row)
```
This indicates that migration `1528395539` was running, but has not yet completed.

_Note: for codeintel the schema version table is called `codeintel_schema_migrations` and for codeinsights its called `codeinsights_schema_migrations`_

### 2. Run the sql queries to finish incomplete migrations

Sourcegraph's migration files take for form of `sql` files following the snake case naming schema `<version>_<description>.<up or down>.sql` and can be found [here](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/migrations) in subdirectories for the specific database. _Note frontend is the pgsql database_.

1. **Find the up migration starting with the migration's version number identified in [step 1](#1-identify-incomplete-migration):** [https://github.com/sourcegraph/sourcegraph/tree/\<YOUR-SOURCEGRAPH-VERSION\>/migrations](https://github.com/sourcegraph/sourcegraph/tree/main/migrations), making sure to go to \<YOUR-SOURCEGRAPH-VERSION\>

2. **Run the code from the identified migration _up_ file explicitly using the `psql` CLI:**
   * It’s possible that one or more commands from the migration ran successfully already. In these cases you may need to run the sql transaction in pieces. For example if a migration file creates multiple indexes and one index already exists you'll need to manually run this transaction skipping that line or adding `IF NOT EXISTS` to the transaction.
   * If you’re running into unique index creation errors because of duplicate values please let us know at support@sourcegraph.com or via your enterprise support channel.
   * There may be other error cases that don't have an easy admin-only resolution, in these cases please let us know at support@sourcegraph.com or via your enterprise support channel.

### 3. Verify database is clean and declare `dirty=false`

1. **Ensure the migration applied, and manually clear the dirty flag on the `schema_migrations` table.**
   * example `psql` query: `UPDATE schema_migrations SET version=1528395918, dirty=false;`
   * **Do not mark the migration table as clean if you have not verified that the migration was successfully completed.**
   * Checking to see if a migration ran successfully requires looking at the migration’s `sql` file, and verifying that `sql` queries contained in the migration file have been applied to tables in the database.
   * _Note: Many migrations do nothing but create tables and/or indexes or alter them._
   * You can get a description of a table and its associated indexes quickly using the `\d <table name>` `psql` shell command (note lack of semicolon). Using this information, you can determine whether a table exists, what columns it contains, and what indexes on it exist. Use this information to determine if commands in a migration ran successfully before setting `dirty=false`.

2. **Start Sourcegraph again and the remaining migrations should succeed, otherwise repeat this procedure again starting from the [Identify incomplete migration](#1-identify-incomplete-migration) step.**

## Additional Information

### `CREATE_INDEX_CONCURRENTLY`
Some migrations utilize the `CREATE INDEX CONCURRENTLY` migration option which runs a query to create a table index as a background process ([learn more here](https://www.postgresql.org/docs/12/sql-createindex.html)). If one of these migrations fails to complete, the database will register that a table index has been created, however the index will be unusable. If you use `\d <table name>` you will see the index for the table, but there will be nothing to tell you the indexing operation has failed. This database state can lead to poor search query performance, with searches attempting to utilize the incomplete table index.

To resolve this, you will then need to run the migration that creates the relevant index again, replacing `CREATE INDEX CONCURRENTLY` with `REINDEX CONCURRENTLY`. You can also drop and recreate the index.

To discover if such a damaged index exists by run the following query:

```sql
SELECT
    current_database() AS datname,
    pc.relname AS relname,
    1 AS count
FROM pg_class pc
JOIN pg_index pi ON pi.indexrelid = pc.oid
WHERE
    NOT indisvalid AND
    NOT EXISTS (SELECT 1 FROM pg_stat_progress_create_index pci WHERE pci.index_relid = pi.indexrelid)
```
Additionally Grafana will alert you of an index is in this state. _The Grafana alert can be found under it's database's charts._ Ex: `Site Admin > Monitoring > Postgres > Invalid Indexes (unusable by query planner)`

## Further resources

* [Sourcegraph - Upgrading Sourcegraph to a new version](https://docs.sourcegraph.com/admin/updates)
* [Migrations README.md](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/README.md) (Note some of the info contained here pertains to running Sourcegraphs development environment and should not be used on production instances)
