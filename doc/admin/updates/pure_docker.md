# Updating a pure-Docker Sourcegraph cluster

This document describes the exact changes needed to update a [pure-Docker Sourcegraph cluster](https://github.com/sourcegraph/deploy-sourcegraph-docker).
Each section comprehensively describes the changes needed in Docker images, environment variables, and added/removed services.

A new version of Sourcegraph is released every month (with patch releases in between, released as needed). Check the [Sourcegraph blog](https://about.sourcegraph.com/blog) or the site admin updates page to learn about updates. We actively maintain the two most recent monthly releases of Sourcegraph.

Upgrades should happen across consecutive minor versions of Sourcegraph. For example, if you are running Sourcegraph 3.1 and want to upgrade to 3.3, you should upgrade to 3.2 and then 3.3.

**Always refer to this page before upgrading Sourcegraph,** as it comprehensively describes the steps needed to upgrade, and any manual migration steps you must perform.

## 3.24 -> 3.25

Confirm that `codeinsights-db-disk` has the correct file permissions:

```
sudo chown -R 999:999 ~/sourcegraph-docker/codeinsights-db-disk/
```

- **If your are connecting to an external Postgres database using SSL/TLS:** Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and connecting to it using SSL/TLS check whether the certificate is up to date.
  - AWS RDS customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) for steps to rotate your certificate.

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/ee2cbb59c80a382fb6cc649d4547b044d9a8b28d

## 3.23.0 -> 3.24.0

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/4acc4c7ed5d49ce41b1f68d654a3f4e2f35bd622

## 3.22.0 -> 3.23.0

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/10de1a4e34ab2c716bd63e52a68a6af896bd81b7

## 3.21.2 -> 3.22.0

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/223c11dacffafb985c2d29b6c6a9b84bcc8255be

This upgrade removes the `code intel bundle manager`. This service has been deprecated and all references to it have been removed.

This upgrade also adds a MinIO container that doesn't require any custom configuration. You can find more detailed documentation in https://docs.sourcegraph.com/admin/external_services/object_storage.


## 3.20.1 -> 3.21.2

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/4629ddfcdfd070b41801106199817ae903ead510

### Note new services

This upgrade includes a new code-intel DB (`deploy-codeintel-db.sh`) and a new service `minio` (`deploy-minio.sh`)
to store LSIF indices.
There is a new environment variable for frontend and frontend-internal called `CODEINTEL_PGHOST`.

(both of these changes are described exactly in the diff above)

## 3.19.1 -> 3.20.1

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/7c57315a1ce05981d436860d79ca01553931e274

### Confirm file permissions

Confirm that `lsif-server-disk` has the correct file permissions:

```
sudo chown -R 100:101 ~/sourcegraph-docker/lsif-server-disk/ ~/sourcegraph-docker/lsif-server-disk/
```

## 3.18.0 -> 3.19.1

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/a1648452c6f7c16637b0e069776df12604c27f73

### Confirm file permissions

Confirm that `lsif-server-disk` has the correct file permissions:

```
sudo chown -R 100:101 ~/sourcegraph-docker/lsif-server-disk/ ~/sourcegraph-docker/lsif-server-disk/
```

## 3.17.2 -> 3.18.0 changes

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/7e6b23cdfead3be639048c5fa7fffe07441610f2

Note: `deploy-grafana.sh` and `deploy-prometheus.sh` had environment variables changed, otherwise only image tags have changed.

## v3.16.0 -> v3.17.2 changes

To upgrade, please perform the changes in the following diff:

https://github.com/sourcegraph/deploy-sourcegraph-docker/commit/2895236661de3ff633ee56fe0b87e9a0f530cc60

## v3.15.1 → v3.16.0 changes

This release involves two steps:

1. Change `3.15.1` image tags to `3.16.0`
2. Update `prometheus/prometheus_targets.yml` [as shown here](https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/customer-replica-v3.15.1...customer-replica-v3.16.0#diff-1d4c5a677b37d150c65ea8356cad978a)

Exact diff of changes to make: https://github.com/sourcegraph/deploy-sourcegraph-docker/compare/customer-replica-v3.15.1...customer-replica-v3.16.0

## v3.14.2 → v3.15.1 changes

This release:

- Removes the 4-container Jaeger deployment, which had performance issues, in favor of a single-container one that works well.
- Removes the old `lsif-server` service deployment, replacing it with 3 `precise-code-intel` services.
- Changes the versions of all Sourcegraph images to be consistent (just `3.15.1` instead of some being inconsistent), which in some cases required changing the names of the images (but the containers / services / shell scripts remain the same for now).

### Update environment variables

- On `frontend` and `frontend-internal` containers: Remove the `LSIF_SERVER_URL` environment variable.
- On `frontend` and `frontend-internal` containers: Set `PRECISE_CODE_INTEL_API_SERVER_URL=http://precise-code-intel-api-server:3186`
- On all containers: Change `JAEGER_AGENT_HOST=jaeger-agent` to `JAEGER_AGENT_HOST=jaeger`

### Remove all old container deployments

- `jaeger-agent` container (`deploy-jaeger-agent.sh`).
- `jaeger-cassandra` container (`deploy-jaeger-cassandra.sh`).
- `jaeger-collector` container (`deploy-jaeger-collector.sh`).
- `jaeger-query` container (`deploy-jaeger-query.sh`).
- `lsif-server` container (`deploy-lsif-server.sh`).

### Add new container deployments

- Add a single `jaeger` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-jaeger.sh#L1).
- Add a single `precise-code-intel-api-server` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-api-server.sh)
- Add a single `precise-code-intel-bundle-manager` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-bundle-manager.sh)
- Add a single `precise-code-intel-worker` container [following this spec](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/v3.15.1/deploy-precise-code-intel-worker.sh)

### Update prometheus_targets.yml

```diff
-    - lsif-server:3186
-    - lsif-server:3187
+    - precise-code-intel-api-server:3186
+    - precise-code-intel-bundle-manager:3187
```

### Update image tags to 3.15.1

Please change *all sourcegraph/<service>* image tags to `3.15.1. This includes all images you previously had as `:3.14.2` AND all `sourcegraph/<service>` images:

```
index.docker.io/sourcegraph/grafana:3.15.1
index.docker.io/sourcegraph/prometheus:3.15.1
index.docker.io/sourcegraph/redis-cache:3.15.1
index.docker.io/sourcegraph/redis-store:3.15.1
index.docker.io/sourcegraph/pgsql:3.15.1
```

The following _images_ have been renamed AND use Sourcegraph versions now (their container names and shell script names remain the same for now):

```diff
- index.docker.io/sourcegraph/syntect_server:c0297a1@sha256:333abb45cfaae9c9d37e576c3853843b00eca33a40a7c71f6b93211ed96528df
+ index.docker.io/sourcegraph/syntax-highlighter:3.15.1

- index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200318141948-0b140b7@sha256:b022fd7e4884a71786acae32e0ec8baf785c18350ebf5d574d52335a346364f9
+ index.docker.io/sourcegraph/search-indexer:3.15.1

- index.docker.io/sourcegraph/zoekt-webserver:0.0.20200318141342-0b140b7@sha256:0d0fbce55b51ec7bdd37927539f50459cd0f207b7cf219ca5122d07792012fb1
+ index.docker.io/sourcegraph/indexed-searcher:3.15.1
```

## v3.13.2 → v3.14.2 changes

### Update image tags

Change 3.13.2 → 3.14.2 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.14.2     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.14.2     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.14.2 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.14.2    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.14.2  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.14.2 |
| replacer          | index.docker.io/sourcegraph/replacer:3.14.2     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.14.2 |
| searcher          | index.docker.io/sourcegraph/searcher:3.14.2     |
| symbols           | index.docker.io/sourcegraph/symbols:3.14.2      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| grafana           | index.docker.io/sourcegraph/grafana:10.0.12@sha256:2cde7e16fa56e81237fb05e228018015385f6498c4642d4ae073799a02b2b68c |
| prometheus        | index.docker.io/sourcegraph/prometheus:10.0.10@sha256:4d73d99f3b18d8e4e09f63671c51eabcba956e357ab33d682de8cf1540e3b863 |
| zoekt-indexserver | index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200318141948-0b140b7@sha256:b022fd7e4884a71786acae32e0ec8baf785c18350ebf5d574d52335a346364f9 |
| zoekt-webserver   | index.docker.io/sourcegraph/zoekt-webserver:0.0.20200318141342-0b140b7@sha256:0d0fbce55b51ec7bdd37927539f50459cd0f207b7cf219ca5122d07792012fb1 |

## v3.12.5 → v3.13.2 changes

### Confirm file permissions

Confirm that `redis-store-disk` has the correct file permissions:

```
sudo chown -R 999:1000 ~/sourcegraph-docker/redis-store-disk/ ~/sourcegraph-docker/redis-cache-disk/
```

### Update image tags

Change 3.12.5 → 3.13.2 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.13.2     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.13.2     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.13.2 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.13.2    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.13.2  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.13.2 |
| replacer          | index.docker.io/sourcegraph/replacer:3.13.2     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.13.2 |
| searcher          | index.docker.io/sourcegraph/searcher:3.13.2     |
| symbols           | index.docker.io/sourcegraph/symbols:3.13.2      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| grafana           | index.docker.io/sourcegraph/grafana:10.0.10@sha256:a6f9816346c3e38478f4b855eeee199fc91a4f69311f5dd57760bf74c3234715 |
| prometheus        | index.docker.io/sourcegraph/prometheus:10.0.8@sha256:75efaada5a335cda9895f337d8f31b6abb7a082ef3092b7bb24bf31fb78eafe6 |
| redis-cache       | index.docker.io/sourcegraph/redis-cache:20-02-03_da9d71ca@sha256:7820219195ab3e8fdae5875cd690fed1b2a01fd1063bd94210c0e9d529c38e56 |
| redis-store       | index.docker.io/sourcegraph/redis-store:20-01-30_c903717e@sha256:e8467a8279832207559bdfbc4a89b68916ecd5b44ab5cf7620c995461c005168 |
| syntect-server    | index.docker.io/sourcegraph/syntect_server:c0297a1@sha256:333abb45cfaae9c9d37e576c3853843b00eca33a40a7c71f6b93211ed96528df |
| zoekt-indexserver | index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200302121716-13dbd22@sha256:91643d83223bb72f4aa2b5031ceb774c8e604a227c58ed54375bd341f25e2ef3 |
| zoekt-webserver   | index.docker.io/sourcegraph/zoekt-webserver:0.0.20200302121635-13dbd22@sha256:0183bd676fe1ba774edcca29f042d8d3594e833e08b6d603af98f74c575eaf69 |

## v3.12.2 → v3.12.5 changes

### Confirm file permissions

Confirm that the `replacer-disk` has the correct file permissions:

```
sudo chown 100:101 ~/sourcegraph-docker/replacer-disk
```

### Update image tags

Change 3.12.2 → 3.12.5 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.12.5     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.12.5     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.12.5 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.12.5    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.12.5  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.12.5 |
| replacer          | index.docker.io/sourcegraph/replacer:3.12.5     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.12.5 |
| searcher          | index.docker.io/sourcegraph/searcher:3.12.5     |
| symbols           | index.docker.io/sourcegraph/symbols:3.12.5      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| zoekt-indexserver | index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200124185115-83b89a5@sha256:efd1fb37fc62bfab963f12e95f69778b0e2e6a253caed5be9025840072ea85b5 |
| zoekt-webserver   | index.docker.io/sourcegraph/zoekt-webserver:0.0.20200124185328-83b89a5@sha256:cde27ee7db0fe6c293a8c9df47b529fb01b5a898e6cbeea4c18d80fe218563db |

## v3.12.1 → v3.12.2 changes

### Update image tags

Change 3.12.1 → v3.12.2 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.12.2     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.12.2     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.12.2 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.12.2    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.12.2  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.12.2 |
| replacer          | index.docker.io/sourcegraph/replacer:3.12.2     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.12.2 |
| searcher          | index.docker.io/sourcegraph/searcher:3.12.2     |
| symbols           | index.docker.io/sourcegraph/symbols:3.12.2      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| prometheus        | index.docker.io/sourcegraph/prometheus:10.0.7@sha256:22d54f27c7df8733a06c7ae8c2e851b61b1ed42f1f5621d493ef58ebd8d815e0 |

## v3.10.4 → v3.12.1 changes

### Management console removal

- Please remove the `management-console` container entirely from your deployment ([details](https://docs.sourcegraph.com/admin/migration/3_11#migration-notes-for-sourcegraph-3-11)).
- Please remove the `- management-console:6060` entry from your `prometheus/prometheus_targets.yml` file.

### If you use `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE`

If you are making use of `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE` environment variables please:

1. Simply copy all properties from `CRITICAL_CONFIG_FILE` and paste them into your `SITE_CONFIG_FILE`.
2. Delete and remove the `CRITICAL_CONFIG_FILE`, as it will no longer be used (the two are now just one).

### Update image tags

Change 3.10.4 → v3.12.1 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.12.1     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.12.1     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.12.1 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.12.1    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.12.1  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.12.1 |
| replacer          | index.docker.io/sourcegraph/replacer:3.12.1     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.12.1 |
| searcher          | index.docker.io/sourcegraph/searcher:3.12.1     |
| symbols           | index.docker.io/sourcegraph/symbols:3.12.1      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| grafana           | index.docker.io/sourcegraph/grafana:10.0.9@sha256:0132e5602030145803753468497a2d17640164b9c34df4ce2532dd93e4b1f6fc |
| prometheus        | index.docker.io/sourcegraph/prometheus:10.0.6@sha256:f681ceb9400f0d546601cbf827ac9c3db16acd37e810da0860cf05d4f42305d1 |
| syntect-server    | index.docker.io/sourcegraph/syntect_server:2b5a3fb@sha256:ef5529cafdc68d5a21edea472ee8ad966878b173044aa5c3db93bc3d84765b1f |
| zoekt-indexserver | index.docker.io/sourcegraph/zoekt-indexserver:0.0.20191204145522-b470e5f@sha256:84e9de8be269277d6e4711a61d0c9675a44d409a4bf7f7dd1b90a22175095fa7 |
| zoekt-webserver   | index.docker.io/sourcegraph/zoekt-webserver:0.0.20191204145231-b470e5f@sha256:fc3bfa69fc60b7a049a6646b71e45896cfae8adf3484602d140965c3781463a0 |
| pgsql (no change if using external Postgres) | index.docker.io/sourcegraph/postgres-11.4:19-11-14_b084311b@sha256:072481559d559cfd9a53ad77c3688b5cf583117457fd452ae238a20405923297 |
| redis-cache (no change if using external Redis) | index.docker.io/sourcegraph/redis-cache:19-04-16_6891de82@sha256:4cbfac8af0abb673899250d4fd859cc477d6426de519e9deb71e454e18322499 |
| redis-store (no change if using external Redis) | index.docker.io/sourcegraph/redis-store:19-04-16_6891de821@sha256:56426d601ce1f6d63088fea1cefa61f69a2e809c7d90fc1d157cca63cf81b277 |

## v3.10.4 → v3.12.5 changes

### Confirm file permissions

Confirm that the `replacer-disk` has the correct file permissions:

```
sudo chown 100:101 ~/sourcegraph-docker/replacer-disk
```

### Management console removal

- Please remove the `management-console` container entirely from your deployment ([details](https://docs.sourcegraph.com/admin/migration/3_11#migration-notes-for-sourcegraph-3-11)).
- Please remove the `- management-console:6060` entry from your `prometheus/prometheus_targets.yml` file.

### If you use `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE`

If you are making use of `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE` environment variables please:

1. Simply copy all properties from `CRITICAL_CONFIG_FILE` and paste them into your `SITE_CONFIG_FILE`.
2. Delete and remove the `CRITICAL_CONFIG_FILE`, as it will no longer be used (the two are now just one).

### Update image tags

Change 3.10.4 → v3.12.5 for the following containers:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| frontend          | index.docker.io/sourcegraph/frontend:3.12.5     |
| frontend-internal | index.docker.io/sourcegraph/frontend:3.12.5     |
| github-proxy      | index.docker.io/sourcegraph/github-proxy:3.12.5 |
| gitserver         | index.docker.io/sourcegraph/gitserver:3.12.5    |
| lsif-server       | index.docker.io/sourcegraph/lsif-server:3.12.5  |
| query-runner      | index.docker.io/sourcegraph/query-runner:3.12.5 |
| replacer          | index.docker.io/sourcegraph/replacer:3.12.5     |
| repo-updater      | index.docker.io/sourcegraph/repo-updater:3.12.5 |
| searcher          | index.docker.io/sourcegraph/searcher:3.12.5     |
| symbols           | index.docker.io/sourcegraph/symbols:3.12.5      |

Also change the follow which are not versioned alongside Sourcegraph currently:

| Container         | New image                                       |
|-------------------|-------------------------------------------------|
| grafana           | index.docker.io/sourcegraph/grafana:10.0.9@sha256:0132e5602030145803753468497a2d17640164b9c34df4ce2532dd93e4b1f6fc |
| prometheus        | index.docker.io/sourcegraph/prometheus:10.0.7@sha256:22d54f27c7df8733a06c7ae8c2e851b61b1ed42f1f5621d493ef58ebd8d815e0 |
| syntect-server    | index.docker.io/sourcegraph/syntect_server:2b5a3fb@sha256:ef5529cafdc68d5a21edea472ee8ad966878b173044aa5c3db93bc3d84765b1f |
| zoekt-indexserver | index.docker.io/sourcegraph/zoekt-indexserver:0.0.20200124185115-83b89a5@sha256:efd1fb37fc62bfab963f12e95f69778b0e2e6a253caed5be9025840072ea85b5 |
| zoekt-webserver   | index.docker.io/sourcegraph/zoekt-webserver:0.0.20200124185328-83b89a5@sha256:cde27ee7db0fe6c293a8c9df47b529fb01b5a898e6cbeea4c18d80fe218563db |
| pgsql (no change if using external Postgres) | index.docker.io/sourcegraph/postgres-11.4:19-11-14_b084311b@sha256:072481559d559cfd9a53ad77c3688b5cf583117457fd452ae238a20405923297 |
| redis-cache (no change if using external Redis) | index.docker.io/sourcegraph/redis-cache:19-04-16_6891de82@sha256:4cbfac8af0abb673899250d4fd859cc477d6426de519e9deb71e454e18322499 |
| redis-store (no change if using external Redis) | index.docker.io/sourcegraph/redis-store:19-04-16_6891de821@sha256:56426d601ce1f6d63088fea1cefa61f69a2e809c7d90fc1d157cca63cf81b277 |
