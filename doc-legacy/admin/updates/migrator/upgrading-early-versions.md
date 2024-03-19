# Upgrading Early Versions

This document provides advise on multiversion upgrades starting from `v4.0.0` and earlier.

Reminders:
- Except in the case of the `up` command, always run `migrator` commands with the most recent release image.
- Always refer to the [upgrade notes](../index.md#upgrades-index) for the versions you'll pass over in the case of a multiversion upgrade.

## Before v4.0.0

The `upgrade` `migrator` command is introduced in `v4.0.0`, just remember to use the latest version of the `migrator` image released during a multiversion upgrade and you'll have access to the most up to date `migtrator` commands.

## Before v3.37.0

In `v3.37.0` the `migrator` service was introduced. Docker-compose and Kubernetes instances hoping to upgrade from versions before the introduction of migrator will need to create the `migrator` manifest in order to run `migrator` commands.
- In Docker-compose add the [migrator service](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml#LL3C1-L53C35) to the top of your `docker-compose.yaml` file and follow the [multiversion upgrade proceedure](../../deploy/docker-compose/upgrade.md#multi-version-upgrades).
- In kubernetes deployments you'll need to create the [`migrator.Job.yaml` job manifest](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/migrator/migrator.Job.yaml) and use it as described in the [multiversion upgrade procedure](../../deploy/kubernetes/upgrade.md#multi-version-upgrades).
> Note: *It may be more effective in some cases to `git checkout` a later version of Sourcegraph to get access to `migrator` manifests and invoke migrator this was before checking back out your `release` branch and proceeding with the upgrade.*

## Before v3.27.0

In version `3.27` `pgsql` and `codeintel-db` databases were upgraded from Postgres 11 to Postgres 12. **If upgrading from 3.26 or before to 3.27 or later**, the `pgsql` and `codeintel-db` databases must have their Postgres version upgraded. If this step is not performed, then the following upgrade procedure will fail fast (and leave all existing data untouched).
  - If using an external database, follow the [upgrading external PostgreSQL instances](../../postgres.md#upgrading-external-postgresql-instances) guide.
  - Otherwise, perform the following steps from the [upgrading internal Postgres instances](../../postgres.md#upgrading-internal-postgresql-instances) guide.

The following procedures decribe how to upgrade the Postgres version:

### Docker-compose postgres version upgrade

1. It's assumed that your fork of `deploy-sourcegraph-docker` is up to date with your instance's current version. Pull the upstream changes for `v3.27.0` and resolve any git merge conflicts. We need to temporarily boot the containers defined at this specific version to rewrite existing data to the new Postgres 12 format.
2. Run `docker-compose up pgsql` to launch new Postgres 12 containers and rewrite the old Postgres 11 data. This may take a while, but streaming container logs should show progress.
3. Wait until the database container is accepting connections. Once ready, run the command `docker exec pgsql -- psql -U sg -c 'REINDEX database sg;'` to repair indexes that were silently invalidated by the previous data rewrite step. **If you skip this step**, then some data may become inaccessible under normal operation, the following steps are not guaranteed to work, and **data loss will occur**.
4. Follow the same steps for the `codeintel-db`:
   - Run `docker-compose up codeintel-db` to launch Postgres 12.
   - Run `docker exec codeintel-db -- pgsql -U sg -c 'REINDEX database sg;'` to reindex the database.

### Kubernetes postgres version upgrade

1. It's assumed that your fork of `deploy-sourcegraph` is up to date with your instance's current version. Pull upstream changes for `v3.27.0` and resolve any git merge conflicts. We need to temporarily boot the containers defined in this specific version to rewrite existing data to the new Postgres 12 format.
2. Run `kubectl apply -l deploy=sourcegraph -f base/pgsql` to launch a new Postgres 12 container and rewrite the Postgres 11 data. This may take a while, but streaming container logs should show progress. 
> **NOTE**: *The Postgres migration requires enough capacity in its attached volume to accommodate an additional copy of the data currently on disk. Resize the volume now if necessary—the container will fail to start if there is not enough free disk space.*
3. Wait until the database container is accepting connections. Once ready, run the command `kubectl exec pgsql -- psql -U sg -c 'REINDEX database sg;'` issue a reindex command to Postgres to repair indexes that were silently invalidated by previous data rewrite step. **If you skip this step**, then some data may become inaccessible under normal operation, following steps are not guaranteed to work, and **data loss will occur**.
4. Follow the same steps for the `codeintel-db`:
   - Run `kubectl apply -l deploy=sourcegraph -f base/codeintel-db` to launch Postgres 12.
   - Run `kubectl exec codeintel-db -- psql -U sg -c 'REINDEX database sg;'` to issue a reindex command to Postgres,
5. Leave these versions of the databases running while the subsequent migration steps are performed. If `codeinsights-db` is a container new to your instance, now is a good time to start it as well.
