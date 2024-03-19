# How to rebuild corrupt Postgres indexes

## Rebuilding indexes

There are multiple databases to rebuild indexes in. Repeat the below process for:

1. `pgsql`
2. `codeintel-db`

We need to ensure there's nothing writing or reading from/to the database before performing the next steps.

In Kubernetes, you can accomplish this by deleting the database service to prevent new connections from being established, followed by a query to terminate existing connections.

```shell
export DB=pgsql # change this for other databases
kubectl delete "svc/$DB"
kubectl port-forward "deploy/$DB" 3333:5432 # doesn't use the service that we just deleted
psql -U sg -d sg -h localhost -p 3333
```

In docker compose, you will need to scale down all the other services to prevent new connections from being established.
You must run these commands from the machine where sourcegraph is running. 

> NOTE: You can refer to the following instructions for accessing databases on your deployment type: [Docker Compose](../deploy/docker-compose/index.md#access-the-database), [Kubernetes](../deploy/kubernetes/operations.md#access-the-database).

```shell
export DB=pgsql # change for other databases
docker-compose down # bring all containers down
docker start $DB # bring only the db container back up
docker exec -it $DB sh
psql -U sg -d sg -h localhost -p 3333
```

Terminate existing client connections first. This will also terminate your own connection to the database, which you'll need to re-establish.

```sql
select pg_terminate_backend(pg_stat_activity.pid)
from pg_stat_activity where datname = 'sg';
```

With a Postgres client connected to the database, we now start by re-indexing system catalog indexes which may have been affected.

```sql
reindex (verbose) system sg;
```

Then we rebuild the database indexes.

```sql
reindex (verbose) database sg;
```


In docker, you can fix the indexes while the server is running. It is not required to stop the single server image.
The only risk here is that connections and every other process might be slow.

Using the following commands you can re-index the database: 

```sql
reindex (verbose) system sourcegraph;
```

Then we rebuild the database indexes.

```sql
reindex (verbose) database sourcegraph;
```

If any duplicate errors are reported, we must delete some rows by adapting and running the [duplicate deletion query](#duplicate-deletion-query) for each of the errors found.

After deleting duplicates, just re-run the above statement. Repeat the process until there are no errors.

At the end of the index rebuilding process, as a last sanity check, we use the amcheck extension to verify there are no corrupt indexes — an error is raised if there are (you should expect to see some output from this command).


```sql
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

## Duplicate deletion query

Here's an example for the `repo` table. The predicates that match the duplicate rows must be adjusted for your specific case, as well as the table name you want to remove duplicates from.

```sql
begin;

-- We must disable index scans before deleting so that we avoid
-- using the corrupt indexes to find the rows to delete. The database then
-- does a sequential scan, which is what we want in order to accomplish that.

set enable_indexscan = 'off';
set enable_bitmapscan = 'off';

delete from repo t1
using repo t2
where t1.ctid > t2.ctid
and (
  t1.name = t2.name or
  (
    t1.external_service_type = t2.external_service_type and
    t1.external_service_id = t2.external_service_id and
    t1.external_id = t2.external_id
  )
);

commit;
```

## Selective index rebuilding

In case your database is large and `reindex (verbose) database sg` takes too long to re-run multiple times as you remove duplicates, you can instead run individual index rebuilding statements, and resume where you left of.

Here's a query that produces a list of such statements for all indexes that contain collatable key columns (we had corruption in these indexes in the [3.30 upgrade](../migration/3_30.md)). This is a sub-set of the indexes that gets re-indexed by `reindex database sg`.

```sql
select
    distinct('reindex (verbose) index ' || i.relname || ';') as stmt
from
    pg_class t,
    pg_class i,
    pg_index ix,
    pg_attribute a,
    pg_namespace n
where
    t.oid = ix.indrelid
    and i.oid = ix.indexrelid
    and n.oid = i.relnamespace
    and a.attrelid = t.oid
    and a.attnum = ANY(ix.indkey)
    and t.relkind = 'r'
    and n.nspname = 'public'
    and ix.indcollation != oidvectorin(repeat('0 ', ix.indnkeyatts)::cstring)
order by stmt;
```

You'd take that output of that query and run each of the statements one by one.
