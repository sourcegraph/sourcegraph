# Docker Single Container Deployment

The Docker Single Container deployment type is a way to very quickly get an instance of Sourcegraph set up locally to experiment with many of its features. However, it is **not recommended** for a production instance, and **has limitations** depending on the OS you are deploying to, as well as the associated resources. See the [troubleshooting section](#troubleshooting) for additional information.

[Code Insights](../../../code_insights/index.md) is not supported in Single Container deployments. To try Code Insights you must deploy using [Docker Compose](../docker-compose/index.md) or [Kubernetes](../kubernetes/index.md). [Tracing](../../observability/tracing.md) is disabled by default, and if you intend to enable it, you will have to deploy and configure the [OpenTelemetry Collector](../../observability/opentelemetry.md). The Single Container deployment does not ship with this service included. It is strongly recommended to use one of the aforementioned deployment methods if tracing support is a requirement. 

## Installation

It takes less than a minute to run and install Sourcegraph using Docker:

<!--
  DO NOT CHANGE THIS TO A CODEBLOCK.
  We want line breaks for readability, but backslashes to escape them do not work cross-platform.
  This uses line breaks that are rendered but not copy-pasted to the clipboard.
-->

<pre class="pre-wrap start-sourcegraph-command"><code>docker run<span class="virtual-br"></span> --publish 7080:7080 --publish 127.0.0.1:3370:3370 --rm<span class="virtual-br"></span> --volume ~/.sourcegraph/config:/etc/sourcegraph<span class="virtual-br"></span> --volume ~/.sourcegraph/data:/var/opt/sourcegraph<span class="virtual-br"></span> sourcegraph/server:5.2.5</code></pre>

Once the server is ready (logo is displayed in the terminal), navigate to the hostname or IP address on port `7080`.  Create the admin account, then you'll be guided through setting up Sourcegraph for code searching and navigation.

For next steps and further configuration options, review the high-level configuration items below, or visit the [detailed configuration documentation](../../config/index.md).

> WARNING: **We do not recommend using this method for a production instance.** If deploying a production instance, see [our recommendations](../index.md) for how to choose a deployment type that suits your needs. We recommend [Docker Compose](../docker-compose/index.md) for most initial production deployments.

## Configuration

### Configure exposed Sourcegraph port

Change the `docker` `--publish` argument to make it listen on the specific interface and port on your host machine. For example, `docker run ... --publish 0.0.0.0:80:7080 ...` would make it accessible on port 80 of your machine. For more information, see "[Publish or expose port](https://docs.docker.com/engine/reference/commandline/run/#publish-or-expose-port--p---expose)" in the Docker documentation.

The other option is to deploy and run Sourcegraph on a cloud provider. For an example, see the [cloud installation section](#cloud-installation-guides).

### Git configuration and authentication

For single-container environments, upon the Sourcegraph Docker image container start, it copies all files from `/etc/sourcegraph/{ssh,gitconfig,netrc}` into its own `$HOME` directory, via the `--volume /mnt/sourcegraph/config:/etc/sourcegraph` in the `docker run` command.

For example, to mount a `.gitconfig`, create a file `/mnt/sourcegraph/config/gitconfig` on your host containing your configuration:

```
# example .gitconfig

[url "example.url.com:"]
  insteadOf = "ssh://example.url.com"
```

Alternatively you can create a new Docker image which inherits from Sourcegraph and then mutates the environment:

```dockerfile
FROM sourcegraph/server:5.2.5

COPY gitconfig /etc/gitconfig
COPY ssh /root/.ssh
RUN	find /root/.ssh -type f -exec chmod 600 '{}' ';'
RUN	find /root/.ssh -type d -exec chmod 700 '{}' ';'
```

This approach can also be used for `sourcegraph/gitserver` images in cluster environments.

Learn more about Git [configuration](../../repo/git_config.md) and [authentication](../../repo/auth.md).

#### SSH authentication (config, keys, `known_hosts`)

The container consults its own file system (in the standard locations) for SSH configuration, private keys, and `known_hosts`. Upon container start, it copies all files from `/etc/sourcegraph/ssh` into its own `$HOME/.ssh` directory.

To provide SSH authentication configuration to the container, assuming you're using the default `--volume $HOME/.sourcegraph/config:/etc/sourcegraph`, follow these steps:

1. Create files at `$HOME/.sourcegraph/config/ssh/config`, `$HOME/.sourcegraph/config/ssh/known_hosts`, etc., on the host machine as desired to configure SSH.
1. Start (or restart) the container.

To configure the container to use the same SSH as your user account on the host machine, you can also run `cp -R $HOME/.ssh $HOME/.sourcegraph/config/ssh`.

#### HTTP(S) authentication via netrc

The easiest way to specify HTTP(S) authentication for repositories is to include the username and password in the clone URL itself, such as `https://user:password@example.com/my/repo`. These credentials won't be displayed to non-admin users.

Otherwise, the container consults the `$HOME/.netrc` files on its own file system for HTTP(S) authentication. The `.netrc` file is a standard way to specify authentication used to connect to external hosts.

To provide HTTP(S) authentication, assuming you're using the default `--volume $HOME/.sourcegraph/config:/etc/sourcegraph`, follow these steps:

1. Create a file at `$HOME/.sourcegraph/config/netrc` on the host machine that contains lines of the form `machine example.com login alice password mypassword` (replacing `example.com`, `alice`, and `mypassword` with the actual values).
1. Start (or restart) the container.

### Expose debug port

This is required to [collect debug data](../../pprof.md).

The docker run command for single-container Sourcegraph needs an additional publish flag to expose the debug port:

```sh
$ docker run --publish 7080:7080 --publish 127.0.0.1:3370:3370 --publish 127.0.0.1:6060:6060 --rm --volume ~/.sourcegraph/config:/etc/sourcegraph --volume ~/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:5.2.5
```

If Sourcegraph is deployed to a remote server, then access via an SSH tunnel using a tool
such as [sshuttle](https://github.com/sshuttle/sshuttle) is required to establish a secure connection.
To access the remote server using `sshuttle` from your local machine:

```sh
$ sshuttle -r user@host 0/0
```

### Environment variables

Add the following to your docker run command:

```sh
$ docker run [...]
-e (YOUR CODE)
sourcegraph/server:5.2.5
```

## Operation

### Access the database

> NOTE: To execute an SQL query against the database without first creating an interactive session (as below), append `--command "SELECT * FROM users;"` to the `docker container exec` command.

Get the Docker container ID for Sourcegraph:

```sh
$ docker ps
CONTAINER ID        IMAGE
d039ec989761        sourcegraph/server:VERSION
```

Open a PostgreSQL interactive terminal:

```sh
$ docker container exec -it d039ec989761 psql -U postgres sourcegraph
```

Run your SQL query:

```sql
SELECT * FROM users;
```

## Upgrade

### Standard upgrades

A [standard upgrade](../../updates/index.md#standard-upgrades) occurs between two minor versions of Sourcegraph. If you are looking to jump forward several versions, you must perform a [multi-version upgrade](#multi-version-upgrades) instead.

**Before upgrading:**

- Read our [update policy](../../updates/index.md#update-policy) to learn about Sourcegraph updates.
- Find the relevant entry for your update in the [update notes for single-container Sourcegraph with Docker](../../updates/server.md).

To update, just use the newer `sourcegraph/server:N.N.N` Docker image (where `N.N.N` is a patch or single minor release away your current version) in place of the older one, using the same Docker volumes. Your server's data will be migrated automatically if needed. You can always find the version number details of the latest release via the [changelog](https://docs.sourcegraph.com/CHANGELOG).

### Multi-version upgrades

A [multi-version upgrade](../../updates/index.md#multi-version-upgrades) is a downtime-incurring upgrade from version 3.20 or later to any future version. Multi-version upgrades will run both schema and data migrations to ensure the data available from the instance remains available post-upgrade.

> NOTE: It is highly recommended to **take an up-to-date snapshot of your databases** prior to starting a multi-version upgrade. The upgrade process aggressively mutates the shape and contents of your database, and undiscovered errors in the migration process or unexpected environmental differences may cause an unusable instance or data loss.
>
> We recommend performing the entire upgrade procedure on an idle clone of the production instance and switch traffic over on success, if possible. This may be low-effort for installations with a canary environment or a blue/green deployment strategy.
>
> **If you do not feel confident running this process solo**, contact customer support team to help guide you thorough the process.

**Before performing a multi-version upgrade**:

- Read our [update policy](../../updates/index.md#update-policy) to learn about Sourcegraph updates.
- Find the entries that apply to the version range you're passing through in the [update notes for Sourcegraph with Docker Single Container](../../updates/server.md#multi-version-upgrade-procedure).

To perform a multi-version upgrade on a Sourcegraph instance running on Docker Single Container:

1. Stop the running Sourcegraph container via `docker stop [CONTAINER]`.
1. Start a temporary Postgres container on top of the Postgres data directory used by the old `sourcegraph/server` image. This Postgres instance will be used by the following upgrade migration. If using an [external database](../../external_services/postgres.md), the database is already accessible from the `migrator` so no action is needed. Otherwise, start the new Postgres container by following the steps [described below](#running-temporary-postgres-containers).
2. Follow the instructions on [how to run the migrator job in Docker](../../updates/migrator/migrator-operations.md#docker-compose) to perform the upgrade migratiohn. For specific documentation on the `upgrade` command, see the [command documentation](../../updates/migrator/migrator-operations.md#upgrade). The following specific steps are an easy way to run the upgrade command:

<p />

```sh
$ docker run \
  --rm \
  --name migrator_${SG_VERSION} \
  -e PGHOST='pgsql' \
  -e PGPORT='5432' \
  -e PGUSER='sg' \
  -e PGPASSWORD='sg' \
  -e PGDATABASE='sourcegraph' \
  -e PGSSLMODE='disable' \
  -e CODEINTEL_PGHOST='pgsql' \
  -e CODEINTEL_PGPORT='5432' \
  -e CODEINTEL_PGUSER='sg' \
  -e CODEINTEL_PGPASSWORD='sg' \
  -e CODEINTEL_PGDATABASE='sourcegraph-codeintel' \
  -e CODEINTEL_PGSSLMODE='disable' \
  -e CODEINSIGHTS_PGHOST='pgsql' \
  -e CODEINSIGHTS_PGPORT='5432' \
  -e CODEINSIGHTS_PGUSER='postgres' \
  -e CODEINSIGHTS_PGPASSWORD='password' \
  -e CODEINSIGHTS_PGDATABASE='postgres' \
  -e CODEINSIGHTS_PGSSLMODE='disable' \
  -e CODEINTEL_PG_ALLOW_SINGLE_DB=true \
  sourcegraph/migrator:v${SG_VERSION} \
  upgrade --from=${CURRENT_SG_VERSION} --to=${SG_VERSION}
```

It is recommended to also add the `--dry-run` flag on a trial invocation to detect if there are any issues with database connection, schema drift, or mismatched versions that need to be addressed.

After this container exits successfully, the remaining infrastructure can now be updated. All temporary containers can be stopped, and the Docker invocation for your `sourcegraph/server` container can be updated to use the new target version.

#### Running temporary Postgres containers

Mounting a Postgres container on top of the data directory used by `sourcegraph/server` will allow us to access and migrate the data in-place without having running services interfere.

Let `${PATH}` be the directory mounted into `/var/opt/sourcegraph` of your instance. This mount contains the Postgres data directory inside of the container.

For example, `${PATH}` is `~/.sourcegraph/data` in `-v ~/.sourcegraph/data:/var/opt/sourcegraph`.

```sh
$ docker run --rm -it \
  -v ${PATH}/postgresql:/data/pgdata-${PG_VERSION} \
  -u 70 \
  -p 5432:5432 \
  --entrypoint bash \
  sourcegraph/postgres-${PG_VERSION_TAG}:${SG_VERSION} \
  -c 'echo "host all all 0.0.0.0/0 trust" >> /data/pgdata-${PG_VERSION}/pg_hba.conf && postgres -c l listen_addresses="*" -D /data/pgdata-${PG_VERSION}'
```

The version of this Postgres container is dependent on the version of the instance prior to upgrade.

| `${SG_VERSION}`     | `${PG_VERSION}` | `${PG_VERSION_TAG}` |
| ------------------- | --------------- | ------------------- |
| `3.20.X` - `3.29.X` | `12`            | `12.6`              |
| `3.30.X` - `3.37.X` | `12`            | `12.6-alpine`       |
| `3.38.X` -          | `12`            | `12-alpine`         |

## Troubleshooting

If you get stuck or need help, [file an issue](https://github.com/sourcegraph/sourcegraph/issues/new?&title=Improve+Sourcegraph+quickstart+guide), [tweet (@sourcegraph)](https://twitter.com/sourcegraph) or [email](mailto:support@sourcegraph.com?subject=Sourcegraph%20quickstart%20guide).

### Mac Computers with Apple silicon

On Mac computers with Apple silicon, you’ll need to add an extra `--platform linux/amd64` argument to your Docker command for correctly running and installing Sourcegraph. 

### File system performance on Docker for Mac

There is a [known issue](https://github.com/docker/for-mac/issues/77) in Docker for Mac that causes slower than expected file system performance on volume mounts, which impacts the performance of search and cloning.

To achieve better performance, you can do any of the following:

- For better clone performance, clone the repository on your host machine and then [add it to Sourcegraph Server](../../repo/add.md#add-repositories-already-cloned-to-disk).
- Try adding the `:delegated` suffix the data volume mount. [Learn more](https://github.com/docker/for-mac/issues/1592).
  ```sh
  --volume ~/.sourcegraph/data:/var/opt/sourcegraph:delegated
  ```

### Testing Sourcegraph on Windows

Sourcegraph can be **tested** on Windows 10 using roughly the same steps provided above, but data will not be retained after server restarts ([this is due to a limitation of Docker on Windows](https://github.com/docker/for-win/issues/39#issuecomment-371942845)).

1. [Install Docker for Windows](https://docs.docker.com/docker-for-windows/install/)
2. Using a command prompt, follow the same [installation steps provided above](#install-sourcegraph-with-docker) but remove the `--volume` arguments. For example by pasting this:

<pre class="pre-wrap"><code>docker run<span class="virtual-br"></span> --publish 7080:7080 --publish 127.0.0.1:3370:3370 --rm<span class="virtual-br"></span> sourcegraph/server:5.2.5</code></pre>

### Low resource environments

To test Sourcegraph in a low resource environment you may want to disable some of the observability tools (Prometheus and Grafana).

Add `-e DISABLE_OBSERVABILITY=true` to your docker run command.

### Starting in Postgres restore mode

In order to restore a Postgres backup, you need to start on an empty database and prevent all other Sourcegraph services from starting.
You can do this by adding `-e PGRESTORE=true` to your `docker run` command. This will start only the Postgres system and allow you to perform a restore. Once it is done, remove that parameter from your docker command.

The database is only accessible from within the container. To perform a restore you will need to copy the required files to the container and then execute the restore commands from within the container using `docker exec`.

You can find examples of this procedure for `docker-compose` in our [docker-compose migration docs](../docker-compose/migrate.md).

### Special instructions for RHEL, Fedora, CentOS and others

If you run Docker on an OS such as RHEL, Fedora, or CentOS with SELinux enabled, sVirt doesn't allow the Docker process to access `~/.sourcegraph/config` and `~/.sourcegraph/data`. In that case, you will see the following message:

`Failed to setup nginx:failed to generate nginx configuration to /etc/sourcegraph: open /etc/sourcegraph/nginx.conf: permission denied`.

To fix this, run:

`mkdir -p ~/.sourcegraph/config ~/.sourcegraph/data && chcon -R -t svirt_sandbox_file_t ~/.sourcegraph/config ~/.sourcegraph/data`

## Reference

### Cloud installation guides

Cloud specific Sourcegraph installation guides for AWS, Google Cloud and Digital Ocean.

- [Install Sourcegraph with Docker on AWS](aws.md)
- [Install Sourcegraph with Docker on Google Cloud](google_cloud.md)
- [Install Sourcegraph with Docker on DigitalOcean](digitalocean.md)

### Insiders build

To test new development builds of Sourcegraph (triggered by commits to `main`), change the tag to `insiders` in the `docker run` command.

> WARNING: `insiders` builds may be unstable, so back up Sourcegraph's data and config (usually `~/.sourcegraph`) beforehand.

```sh
$ docker run --publish 7080:7080 --rm --volume ~/.sourcegraph/config:/etc/sourcegraph --volume ~/.sourcegraph/data:/var/opt/sourcegraph sourcegraph/server:insiders
```

To keep this up to date, run `docker pull sourcegraph/server:insiders` to pull in the latest image, and restart the container to access new changes.
