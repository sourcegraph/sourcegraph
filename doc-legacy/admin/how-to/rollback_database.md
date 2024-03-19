# Rolling back a Postgres database

> WARNING: **Rolling back the database may result in data loss.** Rolling back the database schema may remove tables or columns that were not present in previous versions of Sourcegraph.

If a customer downgrades their instance to a previous version, they need to downgrade their database schema in two circumstances:

1. Developer created **backwards-incompatible** migration: if a migration added in the successor version that was not backwards-compatible, the code of the previous version may struggle operating with the new schema. This may be an emergency situation in which the previous schema must be restored to bring the instance back to a stable state.
1. The customer is **downgrading multiple versions**: if a customer has downgraded their instance once, they will need to downgrade their database schema before downgrading their instance a subsequent time. This is because an instance two (or more) versions ago has no guarantee to run properly against the current database schema. Multiple instance downgrades therefore need to be performed in an alternating fashion with database downgrades.

## Resolution

> NOTE: This process applies only to versions `3.37` and later.

<!---->

> NOTE: A customer rollback is considered an **emergency operation**. Please contact support at <mailto:support@sourcegraph.com> for guidance on this operation.

A database schema downgrade will not always be enough. If a newer version was running even for a small time, it could have migrated data in the background into a format that's no longer readable by the previous version of Sourcegraph.

The `migrator` can be used to run both schema and data migrations (in the appropriate order) so that the old version of Sourcegraph can start and run without broken features. See the [command documentation](../updates/migrator/migrator-operations.md#downgrade) for additional details.

The log output of the `migrator` should include `INFO`-level logs and successfully terminate with `migrator exited with code 0`. If you see an error message or any of the databases have been flagged as "dirty", please follow ["How to troubleshoot a dirty database"](dirty_database.md). A dirty database at this stage requires manual intervention. Please contact support at <mailto:support@sourcegraph.com> or via your enterprise support channel for further assistance.
