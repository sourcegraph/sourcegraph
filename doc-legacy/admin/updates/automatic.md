# Automatic multi-version upgrades

From **Sourcegraph 5.1 and later**, multi-version upgrades can be performed **automatically** as if they were a standard upgrade for the same deployment type. Automatic multi-version upgrades take the following general form:

1. Determine if your instance is ready to Upgrade:
   1. Check your Sourcegraph instances `Site admin > Updates` page. ([more info](./index.md#upgrade-readiness))
   2. Consult upgrade notes for your deployment type accross the range of your upgrade. ([Kubernetes](./kubernetes.md), [Docker-compose](./docker_compose.md), [Server](./server.md))
2. Merge the latest Sourcegraph release into your deployment manifests. 
3. With upstream changes to your manifests merged, start the new instance.

The upgrade magic now happens when the new version is booted. In more detail, starting a new `frontend` container will:

1. Detect that a previous version of Sourcegraph was/is currently running.
2. Plan and persist database schema migrations based on the previously running version, the new target version, and the database state.
3. Start a new internal server that sends poison pills to disconnect old services from the databases and prevents new services from connecting before the upgrade completes.
4. Start a status server in place of the `frontend` service's usual primary exposed port
5. Runs the migration plan (performing the same steps as `migration upgrade ...` )
6. Shuts down the internal and status servers and continues to boot normally

## Viewing progress

During an automatic multi-version upgrade, we'll attempt to boot a status server in the frontend container that is running (or blocking) on an active upgrade attempt. If there is an upgrade failure that affects the frontend, this status page will not be available and the `frontend` container logs should be viewed. Optimistically, the status server will also be unreachable in the case that an upgrade performs quickly enough that there's no time for the status server to start.

In the case that there's a migration failure, or an unfinished out-of-band migration that needs to be complete, the status server will be served instead of the normal Sourcegraph React app. The following screenshots show an upgrade from Sourcegraph v3.37.1 to Sourcegraph 5.0, in which the `frontend` schema is applying (or waiting to apply) a set of schema migrations, the `codeintel` schema has a pair of schema migration failures, and a single unfinished out-of-band migration is still actively being performed to completion.

![An example in-progress upgrade with schema migrations queued for application](https://storage.googleapis.com/sourcegraph-assets/docs/images/upgrades/5.1/queued.png)
![An example in-progress upgrade with a few schema migration failures](https://storage.googleapis.com/sourcegraph-assets/docs/images/upgrades/5.1/failed.png)
![An example in-progress upgrade with unfinished out-of-band migrations](https://storage.googleapis.com/sourcegraph-assets/docs/images/upgrades/5.1/oobmigrations.png)

## Pre v5.0.0 Automatic Multiversion Upgrades

Whether or not a Sourcegraph instance will perform an automatic upgrade is determined by the state of the `versions` table in the `pgsql` (also called `frontend`) database. After `v5.0.0` a column exists on `versions` called `auto_upgrade` set either true of false, this controls whether or not automatic upgrades will be attempted by a Sourcegraph instance. 

For versions prior to v5.0.0 the `SRC_AUTOUPGRADE` environment variable may be set on the `sourcegraph-frontend`, and `migrator` service manifests. To enable automatic upgrades. Ex:
```yaml
  sourcegraph-frontend-0:
    container_name: sourcegraph-frontend-0
    image: 'index.docker.io/sourcegraph/frontend:4.5.1@sha256:22bb1203a6d8ac9bab442dcfef867efc216181026c3d6fc62415ef1a3f063139'
    cpus: 4
    mem_limit: '8g'
    environment:
      - SRC_AUTOUPGRADE=true
      - DEPLOY_TYPE=docker-compose
      - 'OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317'
      - PGHOST=pgsql
      - CODEINTEL_PGHOST=codeintel-db
      - CODEINSIGHTS_PGDATASOURCE=postgres://postgres:password@codeinsights-db:5432/postgres
      - 'SRC_GIT_SERVERS=gitserver-0:3178'
      - 'SRC_SYNTECT_SERVER=http://syntect-server:9238'
```

## Drift and Automatic Upgrades

Multiversion upgrades should be performed only when no drift is detected in your database, and by default if database drifts are detected the migrator `upgrade` command will not perform an upgrade. In some cases however drift may be expected, in such cases the automatic multiversion upgrade drift check may be bypassed by setting the `SRC_AUTOUPGRADE_IGNORE_DRIFT` environment variable on `sourcegraph-frontend` and `migrator` services. 
