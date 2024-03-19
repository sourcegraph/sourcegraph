# Kubernetes Sourcegraph Upgrade Notes

This page lists the changes that are relevant for upgrading Sourcegraph on **Kubernetes with Kustomize and Helm**. 

For upgrade procedures or general info about sourcegraph versioning see the links below:
- [Kubernetes Kustomize Upgrade Procedures](../deploy/kubernetes/upgrade.md)
- [Kubernetes Helm Upgrade Procedures](../deploy/kubernetes/helm.md#upgrading-sourcegraph)
- [General Upgrade Info](./index.md)
- [Product changelog](../../../CHANGELOG.md)

> ***Attention:** These notes may contain relevant information about the infrastructure update such as resource requirement changes or versions of depencies (Docker, kubernetes, externalized databases).*
>
> ***If the notes indicate a patch release exists, target the highest one.***

<!-- GENERATE UPGRADE GUIDE ON RELEASE (release tooling uses this to add entries) -->

## Unreleased

- The GitHub proxy service has been removed in 5.2 and is now removed from kubernetes deployment options. [#55290](https://github.com/sourcegraph/sourcegraph/issues/55290)

#### Notes for 5.2:

- The GitHub proxy service has been removed and is no longer required. You can safely remove it. [#55290](https://github.com/sourcegraph/sourcegraph/issues/55290)

No applicable notes for unreleased versions.

<!-- Add changes changes to this section before release. -->
## v5.1.8 ➔ v5.1.9

#### Notes:

## v5.1.7 ➔ v5.1.8

#### Notes:

## v5.1.6 ➔ v5.1.7

#### Notes:

- v5.1.7 of the [`deploy-sourcegraph-helm`](https://github.com/sourcegraph/deploy-sourcegraph-helm) repo was initially released with the precise-code-intel worker service unable to write to `/tmp`. The release was [overwritten](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/343/files), users who have not yet upgraded will be unaffected. Users who have already upgraded may ammend this issue by pulling in the fix with `helm repo update` and rerunning `helm upgrade`. 

## v5.1.5 ➔ v5.1.6

#### Notes:

## v5.1.4 ➔ v5.1.5

#### Notes:

- Upgrades from versions `v5.0.3`, `v5.0.4`, `v5.0.5`, and `v5.0.6` to `v5.1.5` are affected by an ordering error in the `frontend` databases migration tree. Learn more from the [PR which resolves this bug](https://github.com/sourcegraph/sourcegraph/pull/55650) in `v5.1.6`. **For admins who have already attempted an upgrade to this release from one of the effected versions, see this issue which provides a description of [how to manually fix the frontend db](https://github.com/sourcegraph/sourcegraph/issues/55658).**

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

No upgrade notes.

## v4.5.1 ➔ v5.0.0

No upgrade notes.

## v4.5.0 ➔ v4.5.1

No upgrade notes.

## v4.4.2 ➔ v4.5.0

#### Notes:

- Our new [`kustomize` repo](https://github.com/sourcegraph/deploy-sourcegraph-k8s) is introduced. Admins are advised to follow our [migrate procedure](../deploy/kubernetes/kustomize/migrate.md) to migrate away from our [legacy deployment](https://github.com/sourcegraph/deploy-sourcegraph)
  - **See our [note](../deploy/kubernetes/upgrade.md#using-mvu-to-migrate-to-kustomize) on multiversion upgrades coinciding with this migration. Admins are advised to stop at this version, [migrate](../deploy/kubernetes/kustomize/migrate.md), and then proceed with upgrading.**

- This release introduces a background job that will convert all LSIF data into SCIP. **This migration is irreversible** and a rollback from this version may result in loss of precise code intelligence data. Please see the [migration notes](../how-to/lsif_scip_migration.md) for more details.

**Kubernetes with Helm**
- Searcher and Symbols now use StatefulSets and PVCs to avoid large `ephermeralStorage` requests [#242](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/242)
- This release updates `searcher` and `symbols` services to be headless.
  - Before upgrading, delete your `searcher` and `symbols` services (ex: `kubectl delete svc/searcher svc/symbols`) [#250](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/250)
- An env var `CACHE_DIR` was renamed to `SYMBOLS_CACHE_DIR` in `sourcegraph/sourcegraph`. This change was missed in the Helm charts, which caused a permissions issue during some symbols searches. For more details, see the PR to fix the env var: [#258](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/258).
  - A revision to the 4.5.1 chart (`4.5.1-rev.1`) was released to address the above issue. Use this revision for upgrades to 4.5.1. (ex: `helm upgrade --install --version 4.5.1-rev.1`) [#259](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/259)

## v4.4.1 ➔ v4.4.2

No upgrade notes.

## v4.3 ➔ v4.4.1

- Users attempting a multi-version upgrade to v4.4.0 may be affected by a [known bug](https://github.com/sourcegraph/sourcegraph/pull/46969) in which an outdated schema migration is included in the upgrade process. _This issue is fixed in patch v4.4.2_

  - The error will be encountered while running `upgrade`, and contains the following text: `"frontend": failed to apply migration 1648115472`. 
    - To resolve this issue run migrator with the args `'add-log', '-db=frontend', '-version=1648115472'`. 
    - If migrator was stopped while running `upgrade` the next run of upgrade will encounter drift, this drift should be disregarded by providing migrator with the `--skip-drift-check` flag.

## v4.2 ➔ v4.3.1

No upgrade notes.

## v4.1 ➔ v4.2.1

**Notes**:

- The `worker-executors` Service object is now included in manifests generated using `kustomize`. This object was already introduced in the base manifest, but omitted from manifests generated using `kustomize`. Its purpose is to enable ingested executor metrics to be scraped by Prometheus. It should have no impact on behavior.

<!-- Add changes changes to this section before release. -->

**Notes**:

- `minio` has been replaced with `blobstore`. Please see the update notes here: https://docs.sourcegraph.com/admin/how-to/blobstore_update_notes
- This upgrade adds a [node-exporter](https://github.com/prometheus/node_exporter) DaemonSet, which collects crucial machine-level metrics that help Sourcegraph scale your deployment.
  - **Note**: Similarly to `cadvisor`,  `node-exporter`:
    - runs as a DaemonSet
    - needs to mount various read-only directories from the host machine (`/`, `/proc`, and `/sys`)
    - ideally shares the machine's PID namespace

  For more information, see [deploy-sourcegraph-helm's Changelog](https://github.com/sourcegraph/deploy-sourcegraph-helm/blob/main/charts/sourcegraph/CHANGELOG.md) or contact customer support.

## v4.0 ➔ v4.1.3

No upgrade notes.

## v3.43 ➔ v4.0

**Patch releases**:

- `v4.0.1`

**Notes**:

- `jaeger-agent` sidecars have been removed in favor of an  [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) DaemonSet + Deployment configuration. See [Configure a tracing backend section.](#configure-a-tracing-backend)
- Exporting traces to an external observability backend is now available. Read the [documentation](../deploy/kubernetes/configure.md#configure-a-tracing-backend) to configure.
- The bundled Jaeger instance is now disabled by default. It can be [enabled](../deploy/kubernetes/configure.md#enable-the-bundled-jaeger-deployment) if you do not wish to utilise your own external tracing backend.

## v3.42 ➔ v3.43

**Patch releases**:

- `3.43.1`
- `3.43.2`

## v3.41 ➔ v3.42

**Patch releases**:

- `3.42.1`
- `3.42.2`

## v3.40 ➔ v3.41

**Notes**:

- The Postgres DBs `frontend` and `codeintel-db` are now given 1 hour to begin accepting connections before Kubernetes restarts the containers. [#4136](https://github.com/sourcegraph/deploy-sourcegraph/pull/4136)

## v3.39 ➔ v3.40

**Patch releases**:

- `v3.40.1`
- `v3.40.2`

**Notes**:

- `cadvisor` now defaults to run in `privileged` mode. This allows `cadvisor` to collect out of memory events happening to containers which can be used to discover underprovisoned resources. This is disabled by default in `non-privileged` overlay. [#4126](https://github.com/sourcegraph/deploy-sourcegraph/pull/4126)
- Updated the Nginx ingress controller to v1.2.0. Previously this image originated from quay.io, now it is pulled from the official k8s repository. A redeployment of the ingress
 controller may be necessary if your deployment used the manifests provided in `configure/ingress-nginx`. [#4128](https://github.com/sourcegraph/deploy-sourcegraph/pull/4128)
- The alpine-3.12 docker images used as init containers for some deployments have been replaced with images based on alpine-3.14. [#4129](https://github.com/sourcegraph/deploy-sourcegraph/pull/4129)

## v3.38 ➔ v3.39

**Notes**:

- The`codeinsights-db` container no longer uses TimescaleDB and is now based on the standard Postgres image [sourcegraph/deploy-sourcegraph#4103](https://github.com/sourcegraph/deploy-sourcegraph/pull/4103). Metrics scraping is also enabled.
- **CAUTION**: If you use a custom Code Insights postgres config, you must update the `shared_preload_libraries` list to remove timescaledb. The [above PR](https://github.com/sourcegraph/deploy-sourcegraph/pull/4103/files#diff-e5f8d6e46f8c9335c489c0d8e9ae9be4f4655f878f3ac569c73ebb3865b0eeeeL695-R688) demonstrates this change.

## v3.37 ➔ v3.38

No upgrade notes.

## v3.36 ➔ v3.37

**Notes**:

- This release adds a new `migrator` initContainer to the frontend deployment to run database migrations. Confirm the environment variables on this new container match your database settings. [Docs](https://docs.sourcegraph.com/admin/deploy/kubernetes/update#database-migrations)
- **If performing a multiversion upgrade from an instance prior to this version see our [upgrading early versions documentation](./migrator/upgrading-early-versions.md#before-v3370)**

## v3.35 ➔ v3.36

**Notes**:

- The `backend` service has been removed, so if you deploy with a method other than `kubectl-apply-all.sh`, a manual removal of the service may be necessary.

## v3.34 ➔ v3.35

**Patch releases**:

- `v3.35.1`

**Notes**:

- The query-runner deployment has been removed, so if you deploy with a method other than the `kubectl-apply-all.sh`, a manual removal of the deployment may be necessary.
Follow the [standard upgrade procedure](../deploy/kubernetes/upgrade.md) to upgrade your deployment.
- There is a [known issue](../../code_insights/how-tos/Troubleshooting.md#oob-migration-has-made-progress-but-is-stuck-before-reaching-100) with the Code Insights out-of-band settings migration not reaching 100% complete when encountering deleted users or organizations.

## v3.33 ➔ v3.34

No upgrade notes.

## v3.32 ➔ v3.33

No upgrade notes.

## v3.31 ➔ v3.32

No upgrade notes.

## v3.30 ➔ v3.31

> WARNING: **This upgrade must originate from `v3.30.3`.**

**Notes**:

- The **built-in** main Postgres (`pgsql`) and codeintel (`codeintel-db`) databases have switched to an alpine-based Docker image. Upon upgrading, Sourcegraph will need to re-index the entire database. All users that use our bundled (built-in) database instances **must** read through the [3.31 upgrade guide](../migration/3_31.md) _before_ upgrading.

## v3.29 ➔ v3.30

> WARNING: **If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2** please follow [this migration guide](../migration/3_30.md).

**Patch releases**:

- `v3.30.1`
- `v3.30.2`
- `v3.30.3`

**Notes**:

- This upgrade removes the `non-root` overlay, in favor of using only the `non-privileged` overlay for deploying Sourcegraph in secure environments. If you were
previously deploying using the `non-root` overlay, you should now generate overlays using the `non-privileged` overlay.

## v3.28 ➔ v3.29

**Notes**:

- This upgrade adds a new `worker` service that runs a number of background jobs that were previously run in the `frontend` service. See [notes on deploying workers](../workers.md#deploying-workers) for additional details. Good initial values for CPU and memory resources allocated to this new service should match the `frontend` service.

## v3.27 ➔ v3.28

**Notes**:

- All Sourcegraph images now have a registry prefix. [#2901](https://github.com/sourcegraph/deploy-sourcegraph/pull/2901)
- The memory requirements for `redis-cache` and `redis-store` have been increased by 1GB. See https://github.com/sourcegraph/deploy-sourcegraph/pull/2898 for more context.

## v3.26 ➔ v3.27

> WARNING: Sourcegraph 3.27 now requires **Postgres 12+**.

**Notes**:

> WARNING: We have updated the default replicas for `sourcegraph-frontend` and `precise-code-intel-worker` to `2`. If you use a custom value, make sure you do not merge the replica change.

<!---->

> NOTE: The Postgres 12 database migration scales with the size of your database, and the resources provided to the container.
> Expect to have downtime relative to the size of your database. Additionally, you must ensure that have enough storage
> space to accommodate the migration. A rough guide would be 2x the current on-disk database size

- If you are using an external database, [upgrade your database](https://docs.sourcegraph.com/admin/postgres#upgrading-external-postgresql-instances) to Postgres 12 or above prior to upgrading Sourcegraph. No action is required if you are using the supplied database images.
- **If performing a multiversion upgrade from an instance prior to this version see our [upgrading early versions documentation](./migrator/upgrading-early-versions.md#before-v3270)**

## v3.25 ➔ v3.26

No upgrade notes.

## v3.24 ➔ v3.25

**Notes**:

- Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and and connecting to it using SSL/TLS check whether the certificate is up to date.
  - AWS RDS customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) for steps to rotate your certificate.

## v3.23 ➔ v3.24

No upgrade notes.

## v3.22 ➔ v3.23

No upgrade notes.

## v3.21 ➔ v3.22

**Notes**:

- This upgrade removes the `code intel bundle manager`. This service has been deprecated and all references to it have been removed.
- This upgrade also adds a MinIO container that doesn't require any custom configuration. You can find more detailed documentation in https://docs.sourcegraph.com/admin/external_services/object_storage.

## v3.20 ➔ v3.21

**Notes**:

- This release introduces a second database instance, `codeintel-db`. If you have configured Sourcegraph with an external database, then update the `CODEINTEL_PG*` environment variables to point to a new external database as described in the [external database documentation](../external_services/postgres.md). Again, these must not point to the same database or the Sourcegraph instance will refuse to start.

## v3.19 ➔ v3.20

No upgrade notes.

## v3.18 ➔ v3.19

**Notes**:

- **WARNING**: If you use an overlay that does not reference one of the provided overlays, please add `- ../bases/pvcs` as an additional base to your `kustomization.yaml` file. Otherwise the PVCs could be pruned if `kubectl apply -prune` is used.

## v3.17 ➔ v3.18

No upgrade notes.

## v3.16 ➔ v3.17

No upgrade notes.

## v3.15 ➔ v3.16

**Notes**:

- The following deployments have had their `strategy` changed from `rolling` to `recreate`. This change was made to avoid two pods writing to the same volume and causing corruption. No special action is needed to apply the change.
  - redis-cache
  - redis-store
  - pgsql
  - precise-code-intel-bundle-manager
  - prometheus

## v3.14 ➔ v3.15

**Prometheus and Grafana resource requirements increase**

Resource _requests and limits_ for Grafana and Prometheus are now equal to the following:

- Grafana 100Mi -> 512Mi
- Prometheus: 500M -> 3G

This change was made to ensure that even if another Sourcegraph service starts consuming more memory than expected and the Kubernetes node has been over-provisioned, that Sourcegraph's monitoring will still have enough memory to run and monitor / send alerts to the site admin. For additional information see [#638](https://github.com/sourcegraph/deploy-sourcegraph/pull/638)

You may run the following commands to remove the now unused resources:

```shell script
kubectl delete svc lsif-server
kubectl delete deployment lsif-server
kubectl delete pvc lsif-server
```

**Configuration**

In Sourcegraph 3.0 all site configuration has been moved out of the `config-file.ConfigMap.yaml` and into the PostgreSQL database. We have an automatic migration if you use version 3.2 or before. Please do not upgrade directly from 2.x to 3.3 or higher.

After running 3.0, you should visit the configuration page (`/site-admin/configuration`) and [the management console](https://docs.sourcegraph.com/admin/management_console) and ensure that your configuration is as expected. In some rare cases, automatic migration may not be able to properly carry over some settings and you may need to reconfigure them.

**A new `sourcegraph-frontend` service type**

The type of the `sourcegraph-frontend` service ([base/frontend/sourcegraph-frontend.Service.yaml](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Service.yaml)) has changed
from `NodePort` to `ClusterIP`. Directly applying this change [will
fail](https://github.com/kubernetes/kubernetes/issues/42282). Instead, you must delete the old
service and then create the new one (this will result in a few seconds of downtime):

```shell
kubectl delete svc sourcegraph-frontend
kubectl apply -f base/frontend/sourcegraph-frontend.Service.yaml
```
