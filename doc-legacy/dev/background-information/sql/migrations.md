# Migrations

For each of our Postgres instances, we define a sequence of SQL schema commands that must be applied before the database is in the state the application expects. We define migrations to be backwards compatible with the previous minor version release. This aids in minimizing downtime when using rolling restarts, as new and old services can operate with the same schema without failure. Such deployments are used on Sourcegraph.com, Cloud managed instances, and enterprise instances deployed via Kubernetes.

In development environments, these migrations are applied automatically on application startup. This is a specific choice to keep the response latency small during development. In production environments, a typical upgrade requires that the site-administrator first run a `migrator` service to prepare the database schema for the new version of the application. This is a type of _database-first_ deployment (opposed to _code-first_ deployments), where database migrations are applied prior to the corresponding code change.

Database migrations may be applied arbitrarily long before the new version is deployed. This implies that an old version of Sourcegraph (up to one minor version) can run against a new schema. This requires that all of our database schema changes be *backwards-compatible* with respect to the previous release; any changes to the database schema that would alter the behavior of an old instance is disallowed (and enforced in CI).

## Common migrations

Some migrations are difficult to do in a single step or idempotently. For instance, renaming a column, table, or view, or adding a column with a non-nullable constraint will all break existing code that accesses that table or view. In order to do such changes you may need to break your changes into several parts separated by a minor release.

The remainder of this document is formatted as a recipe book of common types of migrations. We encourage any developer to add a recipe here when a specific type of migration is under-documented.

To learn the process of file changes necessary to implement a migration please refer to [the README file](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/migrations/README.md).

### Adding a non-nullable column (without a default)

_On the `3.X` branch:_

1. Create a migration that adds a _nullable_ column to the target table. This will ensure that any existing queries that insert data into the target table do not begin to fail due to a new constraint on a new column.
1. Update all code paths to ensure the new column is always populated.

**Wait for the branch cut of the next minor release.**

_On the `3.{X+1}` branch:_

1. Create a migration that backfills rows written before version `3.X`.
1. Create a migration that adds a `NOT NULL` constraint to the target column.

_Note: These migration operations can be sub-steps in the same migration definition._

### Removing a database object

_On the `3.X` branch:_

1. Remove all _transitive_ references of the object from code. Note that views (as a trivial example) may reference columns that do not occur in the database layer's queries; hence, this action item may require defining migrations to remove such references.

**Wait for the branch cut of the next minor release.**

_On the `3.{X+1}` branch:_

1. Create a migration to drop the unused object.

### Changing the format of a column

Let's assume that we're trying to change the format of a column _destructively_ so that the new version of the data will not be readable by old code. This can happen, for example, if we're encrypting or re-hashing the values of a column with a new algorithm.

_On the `3.X` branch:_

1. Create a migration that creates a new target column `c2`. We will refer to the original column as `c1`. This migration should also add a [SQL-level comment](https://www.postgresql.org/docs/12/sql-comment.html) on column `c1` noting that it is being deprecated in favor of the new column `c2`.
1. Update all code paths to attempt to read from column `c2`, falling back to column `c1` if no value is found; omitting this fallback may cause new code to mis-understand old writes and in extreme cases may lead to data loss or corruption.
1. Update all code paths to **additionally** write to column `c2` where it writes to `c1`. Writes to column `c1` should not yet be removed as services on the previous version may still be reading from and writing to column `c1`.

**Wait for the branch cut of the next minor release.**

_On the `3.{X+1}` branch:_

1. Remove all writes to column `c1` as there are no more _exclusive_ readers of this column—all readers are able to read from column `c2` as well.
1. Create a regular migration or an [out-of-band migration](../oobmigrations.md) that backfills values for column `c2` from column `c1`. Out-of-band migrations should be preferred for large or non-trivial migrations, and must be used if non-Postgres compute is required to convert values of the old format into the new format.

If using a regular migration, continue immediately. If using an out-of-band migration, mark it deprecated at some future version `3.{X+Y}` and wait for this version's branch cut; out-of-band migrations are not guaranteed to have completed until the underlying instance has been upgraded past the migration's deprecation version. This means there may exist yet-to-be-migrated rows with a value for `c1` but no value for column `c2` until this version.

1. Remove the fallback reads from column `c1`. There should be no remaining references to column `c1`, which can now be removed or abandoned in-place.

**Creating a new type (enum etc).**

When creating new types, such as enums, you may hit upon issues with the migration idempotency tests caused by `CREATE TYPE` not supporting the `IF NOT EXISTS` clause commonly found in other statements. When trying to use `DROP TYPE` in the up-migration, you'll notice you would first have to drop the newly added columns that reference that type too, and quickly you can end up having your down-migration duplicated between both up- and down-migration. We can emulate the `IF NOT EXISTS` clause with the following:

```sql
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'YOUR_TYPENAME_HERE') THEN
        -- create YOUR_TYPENAME_HERE type here
    END IF;
END
$$;
```
