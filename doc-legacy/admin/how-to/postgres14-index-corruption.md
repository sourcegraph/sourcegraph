# How to identify and resolve index corruption in Postgres v14.0 - v14.3

A bug has ben identified in PostgreSQL 14.0-14.3 that can cause database index corruption in indexes created concurrently. Sourcegraph [utilizes](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CREATE+INDEX+AND+CONCURRENTLY&patternType=standard) concurrent index creation, and if you are on these versions, you may experience slow queries or database corruption. To learn more about this bug see postgres's [out-of-cycle release announcment](https://www.postgresql.org/message-id/165473835807.573551.1512237163040609764%40wrigleys.postgresql.org) or [migops article](https://www.migops.com/blog/important-postgresql-14-update-to-avoid-silent-corruption-of-indexes/) on the subject.

> NOTE: If you are running a default Sourcegraph deployment with default Postgres image values, it is likely Postgres is running in version 12 and your instance will be unaffected by this bug.

## Identify your database version
To identify which version of Sourcegraph you are running in a default Sourcegraph deployment. You can access your database via the `docker` or `kubectl` cli tools and run the following command:
```
SELECT version();
```
> NOTE: You can refer to the following instructions for accessing databases on your deployment type: [Docker Compose](../deploy/docker-compose/index.md#access-the-database), [Kubernetes](../deploy/kubernetes/operations.md#access-the-database).

You may also check for index corruption in your database using the `amcheck` by running the following query in your database
```
create extension amcheck;

select bt_index_parent_check(c.oid, true), c.relname, c.relpages
from pg_index i
join pg_opclass op ON i.indclass[0] = op.oid
join pg_am am ON op.opcmethod = am.oid
join pg_class c ON i.indexrelid = c.oid
join pg_namespace n ON c.relnamespace = n.oid
where am.amname = 'btree'
-- Don't check temp tables, which may be from another session:
and c.relpersistence != 't'
-- Function may throw an error when this is omitted:
and i.indisready AND i.indisvalid;
```
This query will return error if there are corrupted indexes. 

## Resolve
If you are impacted, you can remediate this by upgrading to a newer version of Postgres (14.4+) and running the following commands in your database

Determine database name:
```
SELECT current_database();
```
```
REINDEX DATABASE <dbname>;
```
*You may want to use the amcheck query above to verify the reindex has resolved index corruption.*

### Upgrading your database

In default Sourcegraph deployments internal PostgreSQL instances are used and may be upgraded via the [pg_upgrade](https://www.postgresql.org/docs/11/pgupgrade.html). For external databases consult your service providers documentation. For a deeper look at database upgrade operations please consult our [PostgreSQL documentation](https://docs.sourcegraph.com/admin/postgres#upgrading-postgresql).

If you have any questions, please reach out to support on Slack or email support@sourcegraph.com.
