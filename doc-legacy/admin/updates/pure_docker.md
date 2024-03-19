# Updating a pure-Docker Sourcegraph cluster

This document describes the exact changes needed to update a [pure-Docker Sourcegraph cluster](https://github.com/sourcegraph/deploy-sourcegraph-docker).
Each section comprehensively describes the changes needed in Docker images, environment variables, and added/removed services. **Always refer to this page before upgrading Sourcegraph,** as it comprehensively describes the steps needed to upgrade, and any manual migration steps you must perform.

1. Read our [update policy](index.md#update-policy) to learn about Sourcegraph updates.
2. Find the relevant entry for your update in the update notes on this page. **If the notes indicate a patch release exists, target the highest one.**

<!-- GENERATE UPGRADE GUIDE ON RELEASE (release tooling uses this to add entries) -->

## Unreleased

<!-- Add changes changes to this section before release. -->

## v5.2.4 ➔ v5.2.5

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.2.4...v5.2.5)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.2.4...v5.2.5)

#### Notes:

## v5.2.3 ➔ v5.2.4

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.2.3...v5.2.4)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.2.3...v5.2.4)

#### Notes:

## v5.2.2 ➔ v5.2.3

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.2.2...v5.2.3)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.2.2...v5.2.3)

#### Notes:

## v5.2.1 ➔ v5.2.2

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.2.1...v5.2.2)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.2.1...v5.2.2)

#### Notes:

## v5.2.0 ➔ v5.2.1

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.2.0...v5.2.1)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.2.0...v5.2.1)

#### Notes:

## v5.1.9 ➔ v5.2.0

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.2.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.9...v5.2.0)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.2.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.9...v5.2.0)

#### Notes:

## v5.1.8 ➔ v5.1.9

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.9`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.8...v5.1.9)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.9`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.8...v5.1.9)

#### Notes:

## v5.1.7 ➔ v5.1.8

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.8`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.7...v5.1.8)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.8`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.7...v5.1.8)

#### Notes:

## v5.1.6 ➔ v5.1.7

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.7`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.6...v5.1.7)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.7`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.6...v5.1.7)

#### Notes:

## v5.1.5 ➔ v5.1.6

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.6`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.5...v5.1.6)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.6`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.5...v5.1.6)

#### Notes:

## v5.1.4 ➔ v5.1.5

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.4...v5.1.5)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.4...v5.1.5)

#### Notes:

## v5.1.3 ➔ v5.1.4

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.3...v5.1.4)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.3...v5.1.4)

#### Notes:

## v5.1.2 ➔ v5.1.3

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.2...v5.1.3)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.2...v5.1.3)

#### Notes:

## v5.1.1 ➔ v5.1.2

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.1...v5.1.2)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.1...v5.1.2)

#### Notes:

## v5.1.0 ➔ v5.1.1

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.1.0...v5.1.1)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.1.0...v5.1.1)

#### Notes:

## v5.0.6 ➔ v5.1.0

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.1.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.6...v5.1.0)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.1.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.6...v5.1.0)

#### Notes:

## v5.0.5 ➔ v5.0.6

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.6`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.5...v5.0.6)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.6`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.5...v5.0.6)

#### Notes:

## v5.0.4 ➔ v5.0.5

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.4...v5.0.5)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.5`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.4...v5.0.5)

#### Notes:

## v5.0.3 ➔ v5.0.4

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.3...v5.0.4)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.3...v5.0.4)

#### Notes:

## v5.0.2 ➔ v5.0.3

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.2...v5.0.3)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.2...v5.0.3)

#### Notes:

## v5.0.1 ➔ v5.0.2

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.1...v5.0.2)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.1...v5.0.2)

#### Notes:

## v5.0.0 ➔ v5.0.1

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v5.0.0...v5.0.1)

For non-standard replica builds:
- [`Customer Replica 1: ➔ v5.0.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v5.0.0...v5.0.1)

#### Notes:

## v4.5.1 ➔ v5.0.0

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v5.0.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v4.5.1...v5.0.0)

For non-standard replica builds: 
- [`Customer Replica 1: ➔ v5.0.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v4.5.1...v5.0.0)

#### Notes:

## v4.5.0 ➔ v4.5.1

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v4.5.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v4.5.0...v4.5.1)

For non-standard replica builds: 
- [`Customer Replica 1: ➔ v4.5.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v4.5.0...v4.5.1)

#### Notes:

## v4.4.2 ➔ v4.5.0

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v4.5.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v4.4.2...v4.5.0)

For non-standard replica builds: 
- [`Customer Replica 1: ➔ v4.5.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v4.4.2...v4.5.0)

#### Notes:

- This release introduces a background job that will convert all LSIF data into SCIP. **This migration is irreversible** and a rollback from this version may result in loss of precise code intelligence data. Please see the [migration notes](../how-to/lsif_scip_migration.md) for more details.

## v4.4.1 ➔ v4.4.2

As a template, perform the same actions as the following diff in your own deployment: [`Upgrade to v4.4.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/v4.4.1...v4.4.2)

For non-standard replica builds: 
- [`Customer Replica 1: ➔ v4.4.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker-customer-replica-1/compare/v4.4.1...v4.4.2)

#### Notes:

## v4.4.0 ➔ v4.4.1

As a template, perform the same actions as the following diffs in your own deployment:
- [`➔ v4.4.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/9f597b9fb42ea1a170e4456e57e4340d3f722e65)
- 
## v4.3.1 ➔ v4.4.1

As a template, perform the same actions as the following diffs in your own deployment:
- [`➔ v4.4.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/8cdeb7616b73e100aec41806b1118264fea0615d)
- Users attempting a multi-version upgrade to v4.4.0 may be affected by a [known bug](https://github.com/sourcegraph/sourcegraph/pull/46969) in which an outdated schema migration is included in the upgrade process. _This issue is fixed in patch v4.4.2_

  - The error will be encountered while running `upgrade`, and contains the following text: `"frontend": failed to apply migration 1648115472`. 
    - To resolve this issue run migrator with the args `'add-log', '-db=frontend', '-version=1648115472'`. 
    - If migrator was stopped while running `upgrade` the next run of upgrade will encounter drift, this drift should be disregarded by providing migrator with the `--skip-drift-check` flag.

## v4.2 ➔ v4.3.1

As a template, perform the same actions as the following diffs in your own deployment:
- [`➔ v4.3.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/1a8f9a3d71664bf31a1fae9d2ad99c9074eaebe9)

## v4.1 ➔ v4.2.1

- `minio` has been replaced with `blobstore`. Please see the update notes here: https://docs.sourcegraph.com/admin/how-to/blobstore_update_notes

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v4.2.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/a2bd81af53c8b8ad5b0d69e7857945a1f96e331f)

## v4.0 ➔ v4.1.3

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v4.1.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/c1684d0e613630bbe70bc81693e56c906d8f2d08)

**Patch releases**:

- [`v4.1.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/88523d785a0a2fcf943fca44f8d7be381209f3d7)

## v3.43 ➔ v4.0

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v4.0.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/916d2f79e04955e5bef2a47dba738d68655f20ac)

**Patch releases**:

- [`➔ v4.0.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/46a3d0652ad6396a99d2c8b601ff362fbcf4a1c3)

## v3.42 ➔ v3.43

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.43.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/94be340c1b0e57c866d2f530c489da4f65d453e2)

**Patch releases**:

- `v3.43.1`
- `v3.43.2`

## v3.41 ➔ v3.42

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.42.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/a189e495813bc33d544b302eb98c197d70eacc87)

**Patch releases**:

- `v3.42.1`
- `v3.42.2`

## v3.40 ➔ v3.41

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.41.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/8bfd70892c1bf56c5a88db0329826800c7a1097b)

## v3.39 ➔ v3.40

As a template, perform the same actions as the following diffs in your own deployment:

- [`-> 3.40.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/2c94a1fb5fa396759d4800a717af6658548943f7)
- [`-> 3.40.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/312b9f8308148cf9403cc7868eee7b5c9611b121)

**Patch releases**:

- `v3.40.1`
- `v3.40.2`

**Notes**:

- A fix that corrects the default behavior of the `migrator` service is included in this release. An attempt to standardize CLI packages in v3.39.0 unintentionally
broke the default behavior. In order to guard against this, all command line arguments are explicitly set in the deployment manifest.
- **CAUTION** Added the ability to customize postgres server configuration by mounting external configuration files. If you have customized the config in any way, you should copy your changes to the added `postgresql.conf` files [sourcegraph/deploy-sourcegraph-docker#806](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/806)

## v3.38 ➔ v3.39

As a template, perform the same actions as the following diffs in your own deployment:

- [`-> 3.39.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/b37367c738d28ef7e27c8b1f833eb9355bd9e8b1)
- [`-> 3.39.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/c2450311e385f077679d7666c09fd5a2aa7a6b6e)

**Patch releases**:

- `v3.39.1`

**Notes**:

- In this release we need to remove timescaledb from `shared_preload_libraries` configuration in `codeinsights-db`'s `postgresql.conf`. This step will be [performed automatically](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/b37367c738d28ef7e27c8b1f833eb9355bd9e8b1#diff-916162e35509bb582798c4306953fec9f43779d82420cb4435576e2873869f78R17). It can be performed manually instead of run as part of the deploy script.

## v3.37 ➔ v3.38

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.38.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/a66a74ce9a120a9da743eb44c6fea3a55f51842a)
- [`➔ v3.38.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/19735936834aab31134888c179bf07387f09a647)

**Patch releases**:

- `v3.38.1`

**Notes**:

- This release adds the requirement that the environment variables `SRC_GIT_SERVERS`, `SEARCHER_URL`, `SYMBOLS_URL`, and `INDEXED_SEARCH_SERVERS` are set for the worker process.

## v3.36 ➔ v3.37

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.37.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/9e369ec86cdef50b9e2a8350040d011cf2c7cd49)

**Notes**:

- This release adds a new container that runs database migrations (`migrator`) independently of the frontend container. Confirm the environment variables on this new container match your database settings. [Read more about manual operation of the migrator](https://docs.sourcegraph.com/admin/how-to/manual_database_migrations)

## v3.35 ➔ v3.36

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.36.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/25cdf1858de7fe3d0a3e3479a7e5620a02ac6a2c)
- [`➔ v3.36.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/45946fd69dd061cb39c85cfd06a037aeeaf74808)
- [`➔ v3.36.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/3273d3c7da750ff15ba9d4f24d1e09e835bf11d9)

**Patch releases**:

- `v3.36.1`
- `v3.36.3`
- `v3.36.3`

## v3.34 ➔ v3.35

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.35.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/33b076a123c23930cc3339167bdd5502bebc5a3c)
- [`➔ v3.35.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/ba0d94eb945fd3371ed888e4b7177828b33acd3d)
- [`➔ v3.35.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/abc948e60a489f559ebd5cc8f0affcd3c4371fa4)

**Patch releases**:

- `v3.35.1`
- `v3.35.2`

**Notes**:

- The `query-runner` service has been decomissioned in the 3.35.0 release. You can safely remove the `query-runner` service from your installation.
- There is a [known issue](../../code_insights/how-tos/Troubleshooting.md#oob-migration-has-made-progress-but-is-stuck-before-reaching-100) with the Code Insights out-of-band settings migration not reaching 100% complete when encountering deleted users or organizations.


## v3.33 ➔ v3.34

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.34.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/d615dd5f63ec0984d60076aecf0bc598d9ffc1a8)

**Patch releases**:

- `v3.34.1`
- `v3.34.2`

## v3.32 ➔ v3.33

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.33.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/f6dc5c4a859b09faaea44a34e3ba8e85c92fcf58)

## v3.31 ➔ v3.32

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.32.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/2c4c283ae9f89fa48232f0b99ed1982008034fee)

## v3.30 ➔ v3.31

> WARNING: **This upgrade must originate from `v3.30.3`.**

**Notes**:

- The **built-in** main Postgres (`pgsql`) and codeintel (`codeintel-db`) databases have switched to an alpine-based Docker image. Upon upgrading, Sourcegraph will need to re-index the entire database. All users that use our bundled (built-in) database instances **must** read through the [3.31 upgrade guide](../migration/3_31.md) _before_ upgrading.

## v3.29 ➔ v3.30

> WARNING: **If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2** please follow [this migration guide](../migration/3_30.md).

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.30.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/63802ca5966754162c2b3e077e64e60687138874)

**Patch releases**:

- `v3.30.1`
- `v3.30.2`
- `v3.30.3`

## v3.28 ➔ v3.29

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.29.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/940100429fdd59f930436d47e226f5a7116bf6d9)

**Patch releases**:

- `v3.29.1`

**Notes**:

- This upgrade adds a new `worker` service that runs a number of background jobs that were previously run in the `frontend` service. See [notes on deploying workers](../workers.md#deploying-workers) for additional details. Good initial values for CPU and memory resources allocated to this new service should match the `frontend` service.

## v3.27 ➔ v3.28

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.28.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/fa9bd6b4749697e09a4a74537e180e8331d84a5b)

## v3.26 ➔ v3.27

> WARNING: Sourcegraph 3.27 now requires **Postgres 12+**.

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.27.4`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/1d01302a86d219a0f00f6dcbd27d4a511581ff27)

**Patch releases**:

- `v3.27.1`
- `v3.27.2`
- `v3.27.3`
- `v3.27.4`

**Notes**:

- If you are using an external database, [upgrade your database](https://docs.sourcegraph.com/admin/postgres#upgrading-external-postgresql-instances) to Postgres 12.5 or above prior to upgrading Sourcegraph. No action is required if you are using the supplied supplied database images.

## v3.26 ➔ v3.26

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.26.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/68ffad230fc6f99477cedd303a55b91a8c4d9acb)
- [`➔ v3.26.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/de80a4af2ef2eeb23526e3ea560f7f72e1a71a5f)

**Patch releases**:

- `v3.26.1`
- `v3.26.2`

## v3.24 ➔ v3.25

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.25.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/ee2cbb59c80a382fb6cc649d4547b044d9a8b28d)

**Notes**:

- **If you are connecting to an external Postgres database using SSL/TLS:** Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and connecting to it using SSL/TLS check whether the certificate is up to date.
  - AWS RDS customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) for steps to rotate your certificate.
- Confirm that `codeinsights-db-disk` has the correct file permissions via the following command.


```bash
sudo chown -R 999:999 ~/sourcegraph-docker/codeinsights-db-disk/
```

## v3.23 ➔ v3.24

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.24.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/4acc4c7ed5d49ce41b1f68d654a3f4e2f35bd622)

## v3.22 ➔ v3.23

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.23.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/10de1a4e34ab2c716bd63e52a68a6af896bd81b7)

## v3.21 ➔ v3.22

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.22.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/223c11dacffafb985c2d29b6c6a9b84bcc8255be)

**Notes**:

- This upgrade removes the `code intel bundle manager`. This service has been deprecated and all references to it have been removed.
- This upgrade also adds a MinIO container that doesn't require any custom configuration. You can find more detailed documentation in https://docs.sourcegraph.com/admin/external_services/object_storage.

## v3.20 ➔ v3.21

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.21.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/4629ddfcdfd070b41801106199817ae903ead510)

**Patch releases**:

- `v3.21.1`
- `v3.21.2`

**Notes***:

- This upgrade includes a new code-intel DB (`deploy-codeintel-db.sh`) and a new service `minio` (`deploy-minio.sh`) to store precise code intel indexes.
- There is a new environment variable for frontend and frontend-internal called `CODEINTEL_PGHOST`.

## v3.19 ➔ v3.20

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.20.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/7c57315a1ce05981d436860d79ca01553931e274)

**Patch releases**:

- `v3.20.1`

**Notes**:

- Confirm that `lsif-server-disk` has the correct file permissions via the following command.

```bash
sudo chown -R 100:101 ~/sourcegraph-docker/lsif-server-disk/ ~/sourcegraph-docker/lsif-server-disk/
```

## v3.18 ➔ v3.19

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.19.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/a1648452c6f7c16637b0e069776df12604c27f73)

**Patch releases**:

- `v3.19.1`

**Notes**:

- Confirm that `lsif-server-disk` has the correct file permissions via the following command.

```bash
sudo chown -R 100:101 ~/sourcegraph-docker/lsif-server-disk/ ~/sourcegraph-docker/lsif-server-disk/
```

## v3.17 ➔ v3.18

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.18.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/7e6b23cdfead3be639048c5fa7fffe07441610f2)

**Notes**:

- `deploy-grafana.sh` and `deploy-prometheus.sh` had environment variables changed, otherwise only image tags have changed.

## v3.16 ➔ v3.17

As a template, perform the same actions as the following diffs in your own deployment:

- [`➔ v3.17.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/2895236661de3ff633ee56fe0b87e9a0f530cc60)

**Patch releases**:

- `v3.17.2`

## v3.15 ➔ v3.16

As a template, perform the same actions as this [diff](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/customer-replica-v3.15.1...customer-replica-v3.16.0) in your own deployment.

**Steps**:

1. Change `3.15.1` image tags to `3.16.0`.
1. Update `prometheus/prometheus_targets.yml` [as shown here](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/customer-replica-v3.15.1...customer-replica-v3.16.0#diff-1d4c5a677b37d150c65ea8356cad978a).

## v3.14 ➔ v3.15

**Patch releases**:

- `v3.15.1`

**Steps**:

1. Update environment variables
  - On `frontend` and `frontend-internal` containers, remove the `LSIF_SERVER_URL` environment variable.
  - On `frontend` and `frontend-internal` containers, set `PRECISE_CODE_INTEL_API_SERVER_URL=http://precise-code-intel-api-server:3186`
  - On all containers, change `JAEGER_AGENT_HOST=jaeger-agent` to `JAEGER_AGENT_HOST=jaeger`
1. Remove all old container deployments
  - `jaeger-agent` container (`deploy-jaeger-agent.sh`)
  - `jaeger-cassandra` container (`deploy-jaeger-cassandra.sh`)
  - `jaeger-collector` container (`deploy-jaeger-collector.sh`)
  - `jaeger-query` container (`deploy-jaeger-query.sh`)
  - `lsif-server` container (`deploy-lsif-server.sh`)
1. Add new container deployments
  - Add a single `jaeger` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-jaeger.sh#L1)
  - Add a single `precise-code-intel-api-server` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-api-server.sh)
  - Add a single `precise-code-intel-bundle-manager` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-bundle-manager.sh)
  - Add a single `precise-code-intel-worker` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-worker.sh)
1. Update prometheus_targets.yml by replacing `lsif-server:3186` with `precise-code-intel-api-server:3186` and replacing `lsif-server:3187` with `precise-code-intel-bundle-manager:3187`
1. Update image tags to `3.15.1`. Change *all sourcegraph/<service>* image tags to `3.15.1`. This includes all images you previously had as `3.14.2` AND all `sourcegraph/<service>` images:

- `index.docker.io/sourcegraph/grafana:3.15.1`
- `index.docker.io/sourcegraph/prometheus:3.15.1`
- `index.docker.io/sourcegraph/redis-cache:3.15.1`
- `index.docker.io/sourcegraph/redis-store:3.15.1`
- `index.docker.io/sourcegraph/pgsql:3.15.1`

The following _images_ have been renamed AND use Sourcegraph versions now (their container names and shell script names remain the same for now):

```diff
- index.docker.io/sourcegraph/syntect_server:c0297a1@sha256:333abb45cfaae9c9d37e576c3853843b00eca33a40a7c71f6b93211ed96528df
+ index.docker.io/sourcegraph/syntax-highlighter:3.15.1

- index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200318141948-0b140b7@sha256:b022fd7e4884a71786acae32e0ec8baf785c18350ebf5d574d52335a346364f9
+ index.docker.io/sourcegraph/search-indexer:3.15.1

- index.docker.io/sourcegraph/zoekt-webserver:0.0.20200318141342-0b140b7@sha256:0d0fbce55b51ec7bdd37927539f50459cd0f207b7cf219ca5122d07792012fb1
+ index.docker.io/sourcegraph/indexed-searcher:3.15.1
```
