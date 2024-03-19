
# Management Operations

> ⚠️ We recommend new users use our [machine image](../machine-images/index.md) or [script-install](../single-node/script.md) instructions, which are easier and offer more flexibility when configuring Sourcegraph. Existing customers can reach out to our Customer Engineering team support@sourcegraph.com if they wish to migrate to these deployment models.

---

The Sourcegraph Docker Compose yaml file uses [Docker volumes](https://docs.docker.com/storage/volumes/) to store its data. These volumes are stored at `/var/lib/docker/volumes` by [default on Linux](https://docs.docker.com/storage/#choose-the-right-type-of-mount).

Guides for managing cloud storage and backups are available in our cloud-specific installation guides:

- [Storage and backups for Amazon Web Services](./aws.md#storage-and-backups)
- [Storage and backups for Azure](./aws.md#storage-and-backups)
- [Storage and backups for Google Cloud](./google_cloud.md#storage-and-backups)
- [Storage and backups for Digital Ocean](./digitalocean.md#storage-and-backups)

## Access the database

The following command allows a user to shell into the Sourcegraph database container and run `psql` to interact with the container's postgres database:

```bash
docker exec -it pgsql psql -U sg #access pgsql container and run psql
docker exec -it codeintel-db psql -U sg #access codeintel-db container and run psql
```
## Database Migrations

The `frontend` container in the `docker-compose.yaml` file will automatically run on startup and migrate the databases if any changes are required, however administrators may wish to migrate their databases before upgrading the rest of the system when working with large databases. Sourcegraph guarantees database backward compatibility to the most recent minor point release so the database can safely be upgraded before the application code.

To execute the database migrations independently, follow the [docker-compose instructions on how to manually run database migrations](../../updates/migrator/migrator-operations.md#docker-compose). Running the `up` (default) command on the `migrator` of the *version you are upgrading to* will apply all migrations required by the next version of Sourcegraph.

## Backup and restore

The following instructions are specific to backing up and restoring the Sourcegraph databases in a Docker Compose deployment. These do not apply to other deployment types.

> WARNING: **Only core data will be backed up**.
>
> These instructions will only back up core data including user accounts, configuration, repository-metadata, etc. Other data will be regenerated automatically:
>
> - Repositories will be re-cloned
> - Search indexes will be rebuilt from scratch
>
> The above may take a while if you have a lot of repositories. In the meantime, searches may be slow or return incomplete results. This process rarely takes longer than 6 hours and is usually **much** faster.

### Back up Sourcegraph databases

These instructions will back up the primary `sourcegraph` database and the [codeintel](../../../code_navigation/index.md) database.

1\. `ssh` from your local machine into the machine hosting the `sourcegraph` deployment

2\. `cd` to the `deploy-sourcegraph-docker/docker-compose` directory on the host

3\. Verify the deployment is running:

```bash
docker-compose ps
          Name                         Command                       State                                                           Ports
-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
caddy                       caddy run --config /etc/ca ...   Up                      2019/tcp, 0.0.0.0:443->443/tcp, 0.0.0.0:80->80/tcp
cadvisor                    /usr/bin/cadvisor -logtost ...   Up (health: starting)   8080/tcp
codeinsights-db             docker-entrypoint.sh postgres    Up                      5432/tcp
codeintel-db                /postgres.sh                     Up (healthy)            5432/tcp
gitserver-0                 /sbin/tini -- /usr/local/b ...   Up
grafana                     /entry.sh                        Up                      3000/tcp, 0.0.0.0:3370->3370/tcp
jaeger                      /go/bin/all-in-one-linux - ...   Up                      0.0.0.0:14250->14250/tcp, 14268/tcp, 0.0.0.0:16686->16686/tcp, 5775/udp, 0.0.0.0:5778->5778/tcp,
                                                                                     0.0.0.0:6831->6831/tcp, 6831/udp, 0.0.0.0:6832->6832/tcp, 6832/udp
blobstore                   /usr/bin/docker-entrypoint ...   Up (healthy)            9000/tcp
pgsql                       /postgres.sh                     Up (healthy)            5432/tcp
precise-code-intel-worker   /sbin/tini -- /usr/local/b ...   Up (health: starting)   3188/tcp
prometheus                  /bin/prom-wrapper                Up                      0.0.0.0:9090->9090/tcp
query-runner                /sbin/tini -- /usr/local/b ...   Up
redis-cache                 /sbin/tini -- redis-server ...   Up                      6379/tcp
redis-store                 /sbin/tini -- redis-server ...   Up                      6379/tcp
repo-updater                /sbin/tini -- /usr/local/b ...   Up
searcher-0                  /sbin/tini -- /usr/local/b ...   Up (healthy)
symbols-0                   /sbin/tini -- /usr/local/b ...   Up (healthy)            3184/tcp
syntect-server              sh -c /http-server-stabili ...   Up (healthy)            9238/tcp
worker                      /sbin/tini -- /usr/local/b ...   Up                      3189/tcp
zoekt-indexserver-0         /sbin/tini -- zoekt-source ...   Up
zoekt-webserver-0           /sbin/tini -- /bin/sh -c z ...   Up (healthy)
```
4\. Stop the deployment, and restart the databases service only to ensure there are no other connections during backup and restore.

```bash
docker-compose down
docker-compose -f db-only-migrate.docker-compose.yaml up -d
```

5\. Generate the database dumps

```bash
docker exec pgsql sh -c 'pg_dump -C --clean --if-exists --username sg sg' > sourcegraph_db.out
docker exec codeintel-db -c 'pg_dump -C --clean --if-exists --username sg sg' > codeintel_db.out
```

6\. Ensure the `sourcegraph_db.out` and `codeintel_db.out` files are moved to a safe and secure location. 

### Restore Sourcegraph databases into a new environment

The following instructions apply **only if you are restoring your databases into a new deployment** of Sourcegraph ie: a new virtual machine. If you are restoring a previously running environment, see the instructions for [restoring a previously running deployment](#restoring-sourcegraph-databases-into-an-existing-environment)

1\. Copy the database dump files into the `deploy-sourcegraph-docker/docker-compose` directory. 

2\. Start the database services

```bash
docker-compose -f db-only-migrate.docker-compose.yaml up -d
```

3\. Copy the database files into the containers

```bash
docker cp sourcegraph_db.out pgsql:/tmp/sourcegraph_db.out
docker cp codeintel_db.out codeintel-db:/tmp/codeintel_db.out
```

4\. Restore the databases

```bash
docker exec pgsql sh -c 'psql -v ERROR_ON_STOP=1 --username sg -f /tmp/sourcegraph_db.out sg'
docker exec codeintel-db sh -c 'psql -v ERROR_ON_STOP=1 --username sg -f /tmp/condeintel_db.out sg'
```

5\. Start the remaining Sourcegraph services

```bash
docker-compose up -d
```

6\. Verify the deployment has started 

```bash 
docker-compose ps
          Name                         Command                       State                                                           Ports
-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
caddy                       caddy run --config /etc/ca ...   Up                      2019/tcp, 0.0.0.0:443->443/tcp, 0.0.0.0:80->80/tcp
cadvisor                    /usr/bin/cadvisor -logtost ...   Up (health: starting)   8080/tcp
codeinsights-db             docker-entrypoint.sh postgres    Up                      5432/tcp
codeintel-db                /postgres.sh                     Up (healthy)            5432/tcp
gitserver-0                 /sbin/tini -- /usr/local/b ...   Up
grafana                     /entry.sh                        Up                      3000/tcp, 0.0.0.0:3370->3370/tcp
jaeger                      /go/bin/all-in-one-linux - ...   Up                      0.0.0.0:14250->14250/tcp, 14268/tcp, 0.0.0.0:16686->16686/tcp, 5775/udp, 0.0.0.0:5778->5778/tcp,
                                                                                     0.0.0.0:6831->6831/tcp, 6831/udp, 0.0.0.0:6832->6832/tcp, 6832/udp
blobstore                   /usr/bin/docker-entrypoint ...   Up (healthy)            9000/tcp
pgsql                       /postgres.sh                     Up (healthy)            5432/tcp
precise-code-intel-worker   /sbin/tini -- /usr/local/b ...   Up (health: starting)   3188/tcp
prometheus                  /bin/prom-wrapper                Up                      0.0.0.0:9090->9090/tcp
query-runner                /sbin/tini -- /usr/local/b ...   Up
redis-cache                 /sbin/tini -- redis-server ...   Up                      6379/tcp
redis-store                 /sbin/tini -- redis-server ...   Up                      6379/tcp
repo-updater                /sbin/tini -- /usr/local/b ...   Up
searcher-0                  /sbin/tini -- /usr/local/b ...   Up (healthy)
symbols-0                   /sbin/tini -- /usr/local/b ...   Up (healthy)            3184/tcp
syntect-server              sh -c /http-server-stabili ...   Up (healthy)            9238/tcp
worker                      /sbin/tini -- /usr/local/b ...   Up                      3189/tcp
zoekt-indexserver-0         /sbin/tini -- zoekt-source ...   Up
zoekt-webserver-0           /sbin/tini -- /bin/sh -c z ...   Up (healthy)> docker-compose ps
```

7\. Browse to your Sourcegraph deployment, login and verify your existing configuration has been restored

### Restore Sourcegraph databases into an existing environment

1\. `cd` to the `deploy-sourcegraph-docker/docker-compose` and stop the previous deployment and remove any existing volumes
```bash
docker-compose down
docker volume rm docker-compose_pgsql
docker volume rm docker-compose_codeintel-db
```

2\. Start the databases services only

```bash
docker-compose -f db-only-migrate.docker-compose.yaml up -d
```

3\. Copy the database files into the containers

```bash
docker cp sourcegraph_db.out pgsql:/tmp/sourcegraph_db.out
docker cp codeintel_db.out codeintel-db:/tmp/codeintel_db.out
```

4\. Restore the databases

```bash
docker exec pgsql sh -c 'psql -v ERROR_ON_STOP=1 --username sg -f /tmp/sourcegraph_db.out sg'
docker exec codeintel-db sh -c 'psql -v ERROR_ON_STOP=1 --username sg -f /tmp/condeintel_db.out sg'
```

5\. Start the remaining Sourcegraph services

```bash
docker-compose up -d
```

6\. Verify the deployment has started 

```bash 
docker-compose ps
          Name                         Command                       State                                                           Ports
-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
caddy                       caddy run --config /etc/ca ...   Up                      2019/tcp, 0.0.0.0:443->443/tcp, 0.0.0.0:80->80/tcp
cadvisor                    /usr/bin/cadvisor -logtost ...   Up (health: starting)   8080/tcp
codeinsights-db             docker-entrypoint.sh postgres    Up                      5432/tcp
codeintel-db                /postgres.sh                     Up (healthy)            5432/tcp
gitserver-0                 /sbin/tini -- /usr/local/b ...   Up
grafana                     /entry.sh                        Up                      3000/tcp, 0.0.0.0:3370->3370/tcp
jaeger                      /go/bin/all-in-one-linux - ...   Up                      0.0.0.0:14250->14250/tcp, 14268/tcp, 0.0.0.0:16686->16686/tcp, 5775/udp, 0.0.0.0:5778->5778/tcp,
                                                                                     0.0.0.0:6831->6831/tcp, 6831/udp, 0.0.0.0:6832->6832/tcp, 6832/udp
blobstore                   /usr/bin/docker-entrypoint ...   Up (healthy)            9000/tcp
pgsql                       /postgres.sh                     Up (healthy)            5432/tcp
precise-code-intel-worker   /sbin/tini -- /usr/local/b ...   Up (health: starting)   3188/tcp
prometheus                  /bin/prom-wrapper                Up                      0.0.0.0:9090->9090/tcp
query-runner                /sbin/tini -- /usr/local/b ...   Up
redis-cache                 /sbin/tini -- redis-server ...   Up                      6379/tcp
redis-store                 /sbin/tini -- redis-server ...   Up                      6379/tcp
repo-updater                /sbin/tini -- /usr/local/b ...   Up
searcher-0                  /sbin/tini -- /usr/local/b ...   Up (healthy)
symbols-0                   /sbin/tini -- /usr/local/b ...   Up (healthy)            3184/tcp
syntect-server              sh -c /http-server-stabili ...   Up (healthy)            9238/tcp
worker                      /sbin/tini -- /usr/local/b ...   Up                      3189/tcp
zoekt-indexserver-0         /sbin/tini -- zoekt-source ...   Up
zoekt-webserver-0           /sbin/tini -- /bin/sh -c z ...   Up (healthy)> docker-compose ps
```

7\. Browse to your Sourcegraph deployment, login and verify your existing configuration has been restored

## Monitoring

You can monitor the health of a deployment in several ways:

- Using [Sourcegraph's built-in observability suite](../../observability/index.md), which includes dashboards and alerting for Sourcegraph services.
- Using [`docker ps`](https://docs.docker.com/engine/reference/commandline/ps/) to check on the status of containers within the deployment (any tooling designed to work with Docker containers and/or Docker Compose will work too).
  - This requires direct access to your instance's host machine.

## OpenTelemetry Collector

Learn more about Sourcegraph's integrations with the [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) in our [OpenTelemetry documentation](../../observability/opentelemetry.md).

### Configure a tracing backend

[Tracing](../../observability/tracing.md) export can be configured via the [OpenTelemetry collector](../../observability/opentelemetry.md) deployed by default in all Sourcegraph docker-compose deployments.
To get started, edit the mounted configuration file in `otel-collector/config.yaml` based on the [OpenTelemetry collector configuration guidance](../../observability/opentelemetry.md) and edit your `docker-compose.yaml` file to have the `otel-collector` service use the mounted configuration:

```yaml
services:
  # ...
  otel-collector:
    # ...
    command: ['--config', '/etc/otel-collector/config.yaml']
    volumes:
      - '../otel-collector/config.yaml:/etc/otel-collector/config.yaml'
```

#### Enable the bundled Jaeger deployment

Alternatively, you can use the `jaeger` overlay to easily deploy Sourcegraph with some default configuration that exports traces to a standalone Jaeger instance:

```sh
docker-compose \
    -f docker-compose/docker-compose.yaml \
    -f docker-compose/jaeger/docker-compose.yaml \
    up
```

Once a tracing backend has been set up, refer to the [tracing guidance](../../observability/tracing.md) for more details.
