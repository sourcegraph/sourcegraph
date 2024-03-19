# Materialized cache

## Problem

For tables that get very large, it may be too expensive to calculate common aggregates over large portions of the table online using a basic `COUNT(...)` function. Consider the following table `conditions`, which stores data sampled every second from a number of temperature sensors over time.

```sql
CREATE TABLE conditions (
    time timestamptz NOT NULL,
    location_id int NOT NULL,
    temperature_celsius double precision NOT NULL,
    PRIMARY KEY (time, location_id)
);

CREATE INDEX conditions_location_id_time ON conditions(location_id, time);
```

Suppose a frequent operation is to select unique locations present in this table. To make this a more realistic example, we can also suppose that the table is partitioned by time and the hot table is still massive. The most obvious query to attain this data uses `DISTINCT` or `GROUP BY`.

```sql
SELECT DISTINCT location_id FROM conditions;
```

In a randomly generated table with 5 million rows and 50k unique locations, the query plan looks as follows.

```
                                                         QUERY PLAN
----------------------------------------------------------------------------------------------------------------------------
 HashAggregate  (cost=94347.90..94841.93 rows=49403 width=4) (actual time=1155.067..1171.370 rows=50000 loops=1)
   Group Key: location_id
   Batches: 5  Memory Usage: 4145kB  Disk Usage: 3368kB
   ->  Seq Scan on conditions  (cost=0.00..81847.92 rows=4999992 width=4) (actual time=0.025..392.017 rows=5000000 loops=1)
 Planning Time: 0.055 ms
 Execution Time: 1176.541 ms
(6 rows)

Time: 1176.797 ms (00:01.177)
```

Aggregate queries will usually either sort an intermediate result set, or build a hash table while the underlying table is scanned. Queries that need to scan a large amount of the table will frequently devolve into a sequential scan over that table. We cannot change the former, but we can reduce the effect of the latter by reducing the size of the table that gets scanned.

## Solution

In the following solution, we create an aggregate table that stores pre-calculated aggregate data from the source table that is kept updated with triggers. Note that this solution can be expanded to calculate more complex aggregates than show here. For example, data can be pre-calculated to take conditions into account. This will require more bookkeeping at around the triggers, but the complexity may be worth the trade for efficiency.

### Create table

For this example, we create a `condition_locations` table that materializes the result of the query `SELECT DISTINCT location_id FROM conditions`.

```sql
CREATE TABLE condition_locations (
    location_id int NOT NULL,
    PRIMARY KEY (location_id)
);
```

Note that we are explicitly choosing not to use a [materialized view](https://www.postgresql.org/docs/12/rules-materializedviews.html) here as they are refreshed by running the source query and storing the results. The refresh cost is too high for our purposes, and we need to maintain the materialized view incrementally.

### Triggers

Triggers should be added to the source table to keep the counts in the aggregate table up to date on all relevant operations. Some tables may have different write semantics such as append/write-only tables or tables whose records are only ever soft deleted. Such tables may not need to apply a trigger on every type of write operation if it cannot change value of the aggregates we are trying to keep up to date.

In our running example, the insertion of a record may introduce a new location. Therefore, we need to update the aggregate value on insertions. In the following, we create a trigger that invokes the `update_condition_locations_insert` for each *statement* that inserts into the `conditions` table. Note that this will be called _once_ for an insert statement that happens to write multiple rows to the table. This function inserts the distinct location from the new batch of rows into the aggregate table, no-oping the insert if the value already exists.

```sql
CREATE OR REPLACE FUNCTION update_condition_locations_insert() RETURNS trigger AS $$ BEGIN
    INSERT INTO condition_locations SELECT DISTINCT location_id FROM newtab ON CONFLICT (location_id) DO NOTHING;
    RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER condition_locations_insert
AFTER INSERT ON conditions REFERENCING NEW TABLE AS newtab
FOR EACH STATEMENT EXECUTE PROCEDURE update_condition_locations_insert();
```

Similarly, the deletion of a record may remove the last reference of a location. Therefore, we need to update the aggregate value on deletions as well. In the following, we create a trigger that invokes the `update_condition_locations_delete` function on each statement that deletes from the `conditions` table. This function determines if any of the locations removed by the statement are still referenced by the remaining rows. If there are no remaining references, the associated location is deleted from the aggregate table.

```sql
CREATE OR REPLACE FUNCTION update_condition_locations_delete() RETURNS trigger AS $$ BEGIN
    DELETE FROM condition_locations cl
    WHERE
        cl.location_id IN (SELECT location_id FROM oldtab) AND
        NOT EXISTS (SELECT 1 FROM conditions c WHERE c.location_id = cl.location_id);
    RETURN NULL;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER condition_locations_delete
AFTER DELETE ON conditions REFERENCING OLD TABLE AS oldtab
FOR EACH STATEMENT EXECUTE PROCEDURE update_condition_locations_delete();
```

Sample uses:

- [update_lsif_data_documents_schema_versions_insert](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+CREATE+FUNCTION+update_lsif_data_documents_schema_versions_insert&patternType=literal): Code navigation stores the minimum and maximum row versions for each LSIF document grouped by dump identifier. This is used to efficiently determine sets of records that need to be migrated from an old version to a new version.
- [lsif_data_documentation_pages_insert](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%27CREATE+OR+REPLACE+FUNCTION+lsif_data_documentation_pages_insert%27&patternType=regexp): API Docs updates three aggregate counts including the number of rows, the number of distinct dump identifiers, and the number of distinct indexed dump identifiers (a conditional value).

## Result

Looping back to our original query, we can achieve the same result with much lower latency by querying the aggregate table instead of the source table.

```sql
SELECT location_id FROM condition_locations;
```

The query plan shows that the query is orders of magnitude more efficient, even though the shape of the query plan is the same, including the same aggregate hash table build and underlying sequential scan. The differing factor is the size of the underlying data and the benefits of having a smaller working set (data locality, hot buffers, etc).

```
                                                         QUERY PLAN
-----------------------------------------------------------------------------------------------------------------------------
 HashAggregate  (cost=847.00..1347.00 rows=50000 width=4) (actual time=11.717..17.368 rows=50000 loops=1)
   Group Key: location_id
   Batches: 5  Memory Usage: 4145kB  Disk Usage: 200kB
   ->  Seq Scan on condition_locations  (cost=0.00..722.00 rows=50000 width=4) (actual time=0.006..3.415 rows=50000 loops=1)
 Planning Time: 0.045 ms
 Execution Time: 20.475 ms
(6 rows)

Time: 20.766 ms
```
