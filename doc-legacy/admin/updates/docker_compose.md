# Docker Compose Upgrade Notes

This page lists the changes that are relevant for upgrading Sourcegraph on **Docker Compose**. 

For upgrade procedures or general info about sourcegraph versioning see the links below:
- [Docker Compose Upgrade Procedures](../deploy/docker-compose/upgrade.md)
- [General Upgrade Info](./index.md)
- [Product changelog](../../../CHANGELOG.md)

> ***Attention:** These notes may contain relevant information about the infrastructure update such as resource requirement changes or versions of depencies (Docker, Docker Compose, externalized databases).*
>
> ***If the notes indicate a patch release exists, target the highest one.***

<!-- GENERATE UPGRADE GUIDE ON RELEASE (release tooling uses this to add entries) -->

## Unreleased

<!-- Add changes changes to this section before release. -->

## v5.2.4 ➔ v5.2.5

#### Notes:

## v5.2.3 ➔ v5.2.4

#### Notes:

## v5.2.2 ➔ v5.2.3

#### Notes:

## v5.2.1 ➔ v5.2.2

#### Notes:

## v5.2.0 ➔ v5.2.1

#### Notes:

## v5.1.9 ➔ v5.2.0

#### Notes:

## v5.1.8 ➔ v5.1.9

#### Notes:

## v5.1.7 ➔ v5.1.8

#### Notes:

## v5.1.6 ➔ v5.1.7

#### Notes:

## v5.1.5 ➔ v5.1.6

#### Notes:

## v5.1.4 ➔ v5.1.5

#### Notes:
- Upgrades from versions `v5.0.3`, `v5.0.4`, `v5.0.5`, and `v5.0.6` to `v5.1.5` are affected by an ordering error in the `frontend` databases migration tree. Learn more from the [PR which resolves this bug](https://github.com/sourcegraph/sourcegraph/pull/55650). **For admins who have already attempted an upgrade to this release from one of the effected versions, see this issue which provides a description of [how to manually fix the frontend db](https://github.com/sourcegraph/sourcegraph/issues/55658).**
  
## v5.1.3 ➔ v5.1.4

#### Notes:
- Migrator images were built without the `v5.1.x` tag in this version, as such multiversion upgrades using this image version will fail to upgrade to versions in `v5.1.x`. See [this issue](https://github.com/sourcegraph/sourcegraph/issues/55048) for more details.

## v5.1.2 ➔ v5.1.3

#### Notes:
- Migrator images were built without the `v5.1.x` tag in this version, as such multiversion upgrades using this image version will fail to upgrade to versions in `v5.1.x`. See [this issue](https://github.com/sourcegraph/sourcegraph/issues/55048) for more details.

## v5.1.1 ➔ v5.1.2

#### Notes:
- Migrator images were built without the `v5.1.x` tag in this version, as such multiversion upgrades using this image version will fail to upgrade to versions in `v5.1.x`. See [this issue](https://github.com/sourcegraph/sourcegraph/issues/55048) for more details.

## v5.1.0 ➔ v5.1.1

#### Notes:
- Migrator images were built without the `v5.1.x` tag in this version, as such multiversion upgrades using this image version will fail to upgrade to versions in `v5.1.x`. See [this issue](https://github.com/sourcegraph/sourcegraph/issues/55048) for more details.

## v5.0.6 ➔ v5.1.0

#### Notes:
- See note under v5.1.5 release on issues with standard and multiversion upgrades to v5.1.5.

## v5.0.5 ➔ v5.0.6

#### Notes:
- See note under v5.1.5 release on issues with standard and multiversion upgrades to v5.1.5.

## v5.0.4 ➔ v5.0.5

#### Notes:
- See note under v5.1.5 release on issues with standard and multiversion upgrades to v5.1.5.

## v5.0.3 ➔ v5.0.4

#### Notes:
- See note under v5.1.5 release on issues with standard and multiversion upgrades to v5.1.5.

## v5.0.2 ➔ v5.0.3

#### Notes:

## v5.0.1 ➔ v5.0.2

#### Notes:

## v5.0.0 ➔ v5.0.1

#### Notes:

## v4.5.1 ➔ v5.0.0

#### Notes:

## v4.5.0 ➔ v4.5.1

#### Notes:

## v4.4.2 ➔ v4.5.0

#### Notes:

This release introduces a background job that will convert all LSIF data into SCIP. **This migration is irreversible** and a rollback from this version may result in loss of precise code intelligence data. Please see the [migration notes](../how-to/lsif_scip_migration.md) for more details.

## v4.4.1 ➔ v4.4.2

#### Notes:

## v4.3 ➔ v4.4.1

- Users attempting a multi-version upgrade to v4.4.0 may be affected by a [known bug](https://github.com/sourcegraph/sourcegraph/pull/46969) in which an outdated schema migration is included in the upgrade process. _This issue is fixed in patch v4.4.2_

  - The error will be encountered while running `upgrade`, and contains the following text: `"frontend": failed to apply migration 1648115472`. 
    - To resolve this issue run migrator with the args `'add-log', '-db=frontend', '-version=1648115472'`. 
    - If migrator was stopped while running `upgrade` the next run of upgrade will encounter drift, this drift should be disregarded by providing migrator with the `--skip-drift-check` flag.

## v4.2 ➔ v4.3.1

_No notes._

## v4.2 ➔ v4.3.1

_No notes._

## v4.1 ➔ v4.2.1

This upgrade adds the [node-exporter](https://github.com/prometheus/node_exporter) deployment, which collects crucial machine-level metrics that help Sourcegraph scale your deployment.

## v4.0 ➔ v4.1.3

_No notes._

## v3.43 ➔ v4.0

Target the tag [`v4.0.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v4.0.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v4.0.1`

**Notes**:

- `jaeger` (deployed with the `jaeger-all-in-one` image) has been removed in favor of an [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) DaemonSet + Deployment configuration. See [Configure a tracing backend](../deploy/docker-compose/operations.md#configure-a-tracing-backend)
- Exporting traces to an external observability backend is now available. Read the [documentation](../deploy/docker-compose/operations.md#configure-a-tracing-backend) to configure.
- The bundled Jaeger instance is now disabled by default. It can be [enabled](../deploy/docker-compose/operations.md#enable-the-bundled-jaeger-deployment) if you do not wish to utilise your own external tracing backend.

## v3.42 ➔ v3.43

Target the tag [`v3.43.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.43.2/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.43.1`
- `v3.43.2`

## v3.41 ➔ v3.42

Target the tag [`v3.42.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.42.2/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.42.1`
- `v3.42.2`

## v3.40 ➔ v3.41

Target the tag [`v3.41.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.41.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- `caddy` is upgraded to version 2.5.1 and contains a breaking change from version 2.5.0. Incoming `X-Forwarded-*` headers will no longer be trusted automatically. In order to preserve existing product functionality, the Caddyfile was updated to trust all incoming `X-Forwarded-*` headers. [#828](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/828)
- The Postgres DBs `frontend` and `codeintel-db` are now given 1 hour to begin accepting connections before Kubernetes restarts the containers. [#4136](https://github.com/sourcegraph/deploy-sourcegraph/pull/4136)

## v3.39 ➔ v3.40

Target the tag [`v3.40.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.40.2/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.40.1`
- `v3.40.2`

**Notes**:

- `cadvisor` now defaults to run in `privileged` mode. This allows `cadvisor` to collect out of memory events happening to containers which can be used to discover underprovisoned resources. [#804](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/804)

## v3.38 ➔ v3.39

Target the tag [`v3.39.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.39.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.39.1`

**Notes**:

- We made a number of changes to our built-in postgres databases (the `pgsql`, `codeintel-db`, and `codeinsights-db` container)
  - **CAUTION**: Added the ability to customize postgres server configuration by mounting external configuration files. If you have customized the config in any way, you should copy your changes to the added `postgresql.conf` files [sourcegraph/deploy-sourcegraph-docker#792](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/792).
  - Increased the minimal memory requirement of `pgsql` and `codeintel-db` from `2GB` to `4GB`.
  -`codeinsights-db` container no longer uses TimescaleDB and is now based on the standard Postgres image [sourcegraph/deploy-sourcegraph-docker#780](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/780). Metrics scraping is also enabled.

## v3.37 ➔ v3.38

Target the tag [`v3.38.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.38.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.38.1`

**Notes**:

- **Minimum version of 1.29 for docker compose is required for this update**
- This release adds the requirement that the environment variables `SRC_GIT_SERVERS`, `SEARCHER_URL`, `SYMBOLS_URL`, and `INDEXED_SEARCH_SERVERS` are set for the worker process.

## v3.36 ➔ v3.37

Target the tag [`v3.37.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.37.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- This release adds a new container that runs database migrations (`migrator`) independently of the frontend container. Confirm the environment variables on this new container match your database settings. 
- **If performing a multiversion upgrade from an instance prior to this version see our [upgrading early versions documentation](./migrator/upgrading-early-versions.md#before-v3370)**

## v3.35 ➔ v3.36

Target the tag [`v3.36.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.36.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.34 ➔ v3.35

Target the tag [`v3.35.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.35.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.35.1`

**Notes**:

- The `query-runner` service has been decommissioned in the 3.35 release and will be removed during the upgrade. To delete the `query-runner` service, specify `--remove-orphans` to your `docker-compose` command.
- There is a [known issue](../../code_insights/how-tos/Troubleshooting.md#oob-migration-has-made-progress-but-is-stuck-before-reaching-100) with the Code Insights out-of-band settings migration not reaching 100% complete when encountering deleted users or organizations.

## v3.33 ➔ v3.34

Target the tag [`v3.34.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.34.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.32 ➔ v3.33

Target the tag [`v3.33.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.33.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.31 ➔ v3.32

Target the tag [`v3.32.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.32.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.30 ➔ v3.31

> WARNING: **This upgrade must originate from `v3.30.3`.**

Target the tag [`v3.31.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.31.2/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.31.1`
- `v3.31.2`

**Notes**:

- The **built-in** main Postgres (`pgsql`) and codeintel (`codeintel-db`) databases have switched to an alpine-based Docker image. Upon upgrading, Sourcegraph will need to re-index the entire database. All users that use our bundled (built-in) database instances **must** read through the [3.31 upgrade guide](../migration/3_31.md) _before_ upgrading.

## v3.29 ➔ v3.30

> WARNING: **If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2** please follow [this migration guide](../migration/3_30.md).

Target the tag [`v3.30.3`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.30.3/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.30.1`
- `v3.30.2`
- `v3.30.3`

## v3.28 ➔ v3.29

Target the tag [`v3.29.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.29.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- This upgrade adds a new `worker` service that runs a number of background jobs that were previously run in the `frontend` service. See [notes on deploying workers](../workers.md#deploying-workers) for additional details. Good initial values for CPU and memory resources allocated to this new service should match the `frontend` service.

## v3.27 ➔ v3.28

Target the tag [`v3.28.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.28.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- The memory requirements for `redis-cache` and `redis-store` have been increased by 1GB. See https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/373 for more context.

## v3.26 ➔ v3.27

> WARNING: Sourcegraph 3.27 now requires **Postgres 12+**.

Target the tag [`v3.27.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.27.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- If you are using an external database, [upgrade your database](https://docs.sourcegraph.com/admin/postgres#upgrading-external-postgresql-instances) to Postgres 12 or above prior to upgrading Sourcegraph. No action is required if you are using the supplied supplied database images.
- **If performing a multiversion upgrade from an instance prior to this version see our [upgrading early versions documentation](./migrator/upgrading-early-versions.md#before-v3270)**

## v3.25 ➔ v3.26

Target the tag [`v3.26.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.26.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.24 ➔ v3.25

Target the tag [`v3.25.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.25.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and and connecting to it using SSL/TLS check whether the certificate is up to date.
    - AWS RDS customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) for steps to rotate your certificate.

## v3.23 ➔ v3.24

Target the tag [`v3.24.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.24.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.22 ➔ v3.23

Target the tag [`v3.23.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.23.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.21 ➔ v3.22

Target the tag [`v3.22.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.22.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Notes**:

- This upgrade removes the `code intel bundle manager`. This service has been deprecated and all references to it have been removed.
- This upgrade also adds a MinIO container that doesn't require any custom configuration. You can find more detailed documentation in https://docs.sourcegraph.com/admin/external_services/object_storage.

## v3.20 ➔ v3.21

Target the tag [`v3.21.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.21.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.21.1`

**Notes**:

- This release introduces a second database instance, `codeintel-db`. If you have configured Sourcegraph with an external database, then update the `CODEINTEL_PG*` environment variables to point to a new external database as described in the [external database documentation](../external_services/postgres.md). Again, these must not point to the same database or the Sourcegraph instance will refuse to start.

## v3.19 ➔ v3.20

Target the tag [`v3.20.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.20.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.20.1`

## v3.18 ➔ v3.19

Target the tag [`v3.19.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.19.2/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.19.1`
- `v3.19.2`

## v3.17 ➔ v3.18

Target the tag [`v3.18.0-1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/3.18/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.18.0-1`

## v3.16 ➔ v3.17

**Patch releases**:

Target the tag [`v3.17.2`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.16.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

- `v3.17.1`
- `v3.17.2`

## v3.15 ➔ v3.16

Target the tag [`v3.16.0`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.16.0/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

## v3.14 ➔ v3.15

Target the tag [`v3.15.1`](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/v3.15.1/docker-compose) when fetching upstream from `deploy-sourcegraph-docker`.

**Patch releases**:

- `v3.15.1`
