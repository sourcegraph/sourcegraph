# Stitched Migration Generator

The stitched migration generator requires internet access, as it fetches files from a GCP bucket.

## Usage

`bazel run //dev:write_all_generated` will write a new stitched migration graph in the source tree, introducing changes only if something changed. The final stitched migration graph will incorporate new migrations that have been introduced since the last minor release if there are any, as it also takes in account the current migrations.

For determining what's the current migration, it relies on constants in `internal/database/migration/shared/data/cmd/generator/consts.go` which have be be manually updated. Once we roll-out the new release process, that new number will be automatically injected at build time by Bazel. If for some reason, the max version from that file has a corresponding tarball on GCS, which means that it simply wasn't updated after a minor or major release, it will use it instead, to avoid introducing changes that should not be there.

TODO @jhchabran.

## Why are we "stitching" migrations

[Periodically we squash](https://github.com/sourcegraph/sourcegraph/pull/41819) all migrations from two versions behind the current version into a single migration using `sg migration squash`. This reduces the time required to initialize a new database. This means that the migrator image is built with a set of definitions embedded that doesn’t reflect the definition set in older versions. For multiversion upgrades this presents a problem. To get around this, on minor releases we generate a `stitched-migration-graph.json` file

`stitched-migration-graph.json` stitches (can think of this as unsquashing) historic migrations enabling the migrator to have a reference of older migrations. This serves a few purposes:

1. When jumping across multiple versions we do not have access to a full record of migration definitions on migrator disk because some migrations will likely have been squashed. Therefore we need a way to ensure we don’t miss migrations on a squash boundary. Note we can’t just re-apply the root migration after a squash because some schema state that's already represented in the root migration. This means the squashed root migration isn’t always idempotent.

2. During a multiversion upgrade migrator must schedule out of band migrations to be started at some version and completed before upgrading past some later version. Migrator needs access to the unsquashed migration definitions to know which migrations must have run at the time the oob migration is triggered.

In standard/up upgrades `stitched-migration-graph.json` isn’t necessary. This is because up determines migrations to run by comparing migrations listed as already run in the relevant db’s migration_logs table directly to those migration definitions embedded in the migrator disk at build time for the current version, and running any which haven’t been run. We never squash away the previous minor version of Sourcegraph, in this way we can guarantee the `migration_logs` table migrations always has migrations in common with the migration definitions on disk.

## How it works

Previously, we used `git` commands to crawl previous minor releases in order to reconstruct the graph. This doesn't play well with Bazel and creates an additional step that requires to be performed and committed when cutting releases.

The new approach instead generates the stitched migration graph by fetching past migrations from a [GCP Cloud Storage bucket](https://console.cloud.google.com/storage/browser/schemas-migrations/migrations?project=sourcegraph-ci), allowing to avoid having to interact with `git` while driving the generation from `Bazel`. Bazel cannot provides its interesting deterministic properties if tasks are allowed to change the sources underneath and that's exactly what `git` does if you're switching revisions at build time.

So now, the stitched migrations are treated like any other generated files, i.e. it comes with a target to generate the stitched migrations (`//internal/database/migration/shared:generate_stitched_migration_graph`) and a runnable target to write them back to disk (`//internal/database/migration/shared:write_stitched_migration_graph`) that is included in the group of generating targets in the `//dev:write_all_generated` target that provides a single entry point to regenerate everything.

### Uploading a new migrations archive when cutting a minor release

In order for the system to work, whenever we are cutting a new minor release, we need to upload a tarball of the migrations folder, and name it by the release version number. Here are the commands to perform this action:

```
git archive --format=tar.gz vX.Y.0 migrations > migrations-vX.Y.0.tar.gz
CLOUDSDK_CORE_PROJECT="sourcegraph-ci"
gsutil cp migrations-vX.Y.0 gs://schemas-migrations/migrations/
```

Once we roll out the new release process, we'll automate that step, so nobody has to remember doing it whenever we release a new minor release.

TODO @jhchabran.

If something goes wrong with the GCP bucket, the `internal/database/migration/shared/data/cmd/migrationdump/` tool can be used to regenerate all previous migration dumps.
Hello World
