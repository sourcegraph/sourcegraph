# Migrating code intelligence data from LSIF to SCIP (Sourcegraph 4.5 -> 4.6)

> WARNING: The migration from LSIF -> SCIP is **destructive and irreversible**. A downgrade from 4.6 to a previous version will result in the inability to access migrated code intelligence data (powering precise code navigation features).

Sourcegraph 4.5 introduced an [out-of-band migration](unfinished_migration.md#checking-progress) that re-encodes LSIF code intelligence data as SCIP in the `codeintel-db`. This migration is required to complete prior to a subsequent upgrade to 4.6, as support for reading LSIF-encoded data has been removed as of this version.

As of [src-cli 4.5](https://github.com/sourcegraph/src-cli/releases/tag/4.5.0), LSIF indexes will be converted to SCIP prior to upload. This ensures that only _existing_ data needs to be migrated in the background. Using an older version of src-cli to upload code intelligence to your index may continue to feed additional LSIF data that needs to be subsequently migrated. This will ultimately block the ability to upgrade to the next version as the migration will never reach 100% (and remain there).

Once the migration has completed, Postgres may continue to hold on to disk space that was previously occupied by LSIF data. Future versions of Sourcegraph will drop these tables completely, freeing this space. Heavier users of precise code intelligence may wish to reclaim this disk space earlier. Once the migration is complete, you can [truncate the LSIF data tables](clear_codeintel_data.md#clearing-lsif-data) to immediately reclaim this space.

---

If you wish to take the scorched earth route and clear all existing code intelligence data from your instance and start fresh, follow the entire guide on [clearing code intelligence data](clear_codeintel_data.md).
