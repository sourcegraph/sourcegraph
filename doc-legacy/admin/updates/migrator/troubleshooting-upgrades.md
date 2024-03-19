# Troubleshooting Upgrades

This document covers problems that may come up during an upgrade and ways to debug them.

**If you encounter trouble during an upgrade please reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com)**

### Reporting drift errors

If you have conducted an upgrade and see drift in your `Site admin -> Updates` page please report it to us. This is generally not a major cause of concern but may indicate problems in our `migrator` schema migration *squashing* logic.

Please provide us with the drift as well and if possible the contents of the `migration_logs` table in your database:
```sql
SELECT * FROM migration_logs;
```

### Failing migrations in kubernetes

During a standard upgrade migrations may fail due to transient or application errors. When this happens, the database will be marked by the migrator as _dirty_. A dirty database requires manual intervention to ensure the schema is in the expected state before continuing with migrations or application startup.

In order to retrieve the error message printed by the migrator on startup, you'll need to use the `kubectl logs <frontend pod> -c migrator` to specify the init container, not the main application container. Using a bare `kubectl logs` command will result in the following error:

```
Error from server (BadRequest): container "frontend" in pod "sourcegraph-frontend-69f4b68d75-w98lx" is waiting to start: PodInitializing
```

Once a failing migration error message can be found, follow the guide on [how to troubleshoot a dirty database](../../how-to/dirty_database.md).
