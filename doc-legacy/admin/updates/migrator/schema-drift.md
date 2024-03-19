# How to use `migrator` operation `drift`

> WARNING: When running the drift command, the version supplied (or inferred from your instance's state) is meant to indicate the **most recently running version of the instance** (not the target version during an upgrade process). Drift output is meant to show you the difference between the schema expected by Sourcegraph during operation and upgrades against your database's actual schema so that the database can be put in a healthy, known state. Following the instructions provided when supplying a different version will move your database schema **further out of sync**.

During an upgrade you may run into the following message.

```
* Sourcegraph migrator v4.1.3
❌ Schema drift detected for frontend
💡 Before continuing with this operation, run the migrator's drift command and follow instructions to repair the schema to the expected current state. See https://docs.sourcegraph.com/admin/how-to/manual_database_migrations#drift for additional instructions.
```

This error indicates that `migrator` has detected some difference between the state of the schema in your database and the expected schema for the database in the `-from` or current version of your Sourcegraph instance.

When the schema [drift](./migrator-operations.md#drift) command is run you'll see a set of diffs representing the areas where your instance schema has diverged from the expected state as well as the SQL operations to fix these examples of drift. For example:

```
❌ Missing index "external_service_repos"."external_service_repos_repo_id_external_service_id_unique"
💡 Suggested action: define the index.

ALTER TABLE external_service_repos ADD CONSTRAINT
external_service_repos_repo_id_external_service_id_unique UNIQUE
(repo_id, external_service_id);
```

```
❌ Unexpected properties of column "batch_spec_resolution_jobs"."batch_spec_id"

schemas.ColumnDescription{
  	Name:                   "batch_spec_id",
  	Index:                  -1,
  	TypeName:               "integer",
- 	IsNullable:             false,
+ 	IsNullable:             true,
  	Default:                "",
  	CharacterMaximumLength: 0,
  	... // 5 identical fields
  }

💡 Suggested action: change the column nullability constraint.

ALTER TABLE batch_spec_resolution_jobs ALTER COLUMN
batch_spec_id SET NOT NULL;
```

To correct these errors in the database run the suggested SQL queries via `psql` in internal databases, or via the tools provided by your cloud database provider. 

*docker example*
```
docker exec -it pgsql psql -U sg -c 'ALTER TABLE external_service_repos ADD CONSTRAINT external_service_repos_repo_id_external_service_id_unique UNIQUE (repo_id, external_service_id);'
```
*kubernetes example*
```
kubectl -n ns-sourcegraph exec -it pgsql -- psql -U sg -c 'ALTER TABLE external_service_repos ADD CONSTRAINT external_service_repos_repo_id_external_service_id_unique UNIQUE (repo_id, external_service_id);'
```

Then check the database again with the `drift` command and proceed with your multiversion upgrade.

> Note: It is possible for the drift command to detect diffs which will not prevent upgrades. For example the following drift output picked up formating differences `\n` vs `""`:
```
❌ Unexpected definition of function "lsif_data_docs_search_private_delete"
strings.Join({
    "CREATE OR REPLACE FUNCTION
public.lsif_data_docs_search_private_",
    "delete()\n RETURNS trigger\n LANGUAGE plpgsql\nAS
$function$\nBEGIN\n",
    "UPDATE lsif_data_apidocs_num_search_results_private SET
count =",
-   " ",
+   "\n",
    "count - (select count(*) from oldtbl);\nRETURN NULL;\nEND
$functio",
    "n$\n",
  }, "")
💡 Suggested action: replace the function definition.
CREATE OR REPLACE FUNCTION
public.lsif_data_docs_search_private_delete()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
UPDATE lsif_data_apidocs_num_search_results_private SET count =
count - (select count(*) from oldtbl);
RETURN NULL;
END $function$;
```

If migrator drift suggests SQL queries which don't make sense please report to support@sourcegraph.com or open an issue in the [`sourcegraph/sourcegraph`](https://github.com/sourcegraph/sourcegraph/issues/new?assignees=&labels=&template=bug_report.md&title=) repo. You may proceed with a migrator `upgrade` command using the `-skip-drift-check=true` flag.

