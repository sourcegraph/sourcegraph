# Configuration

> ⚠️ We recommend new users use our [machine image](../../index.md) or [script-install](../single-node/script.md) instructions, which are easier and offer more flexibility when configuring Sourcegraph. Existing customers can reach out to our Customer Engineering team support@sourcegraph.com if they wish to migrate to these deployment models.

---

You can find the default [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) file inside the deployment repository.

If you would like to make changes to the default configurations, we highly recommend you to create a new file called `docker-compose.override.yaml` in the same directory where the base file ([docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml)) is located, and make your customizations inside the `docker-compose.override.yaml` file.

>WARNING: For configuration of Sourcegraph, see Sourcegraph's [configuration](../../config/index.md) docs.

## What is an override file?

Docker Compose allows you to customize configuration settings using an override file called `docker-compose.override.yaml`, which allows customizations to persist through upgrades without needing to manage merge conflicts as changes are not made directly to the base `docker-compose.yaml` file.

When you run the `docker-compose up` command, the override file will be automatically merged over the base [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) file.

The [official Docker Compose docs](https://docs.docker.com/compose/extends/) provide details about override files.

## Examples

In order to make changes to the configuration settings defined in the base file [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml), create an empty `docker-compose.override.yaml` file in the same directory as the [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) file, using the same version number, and then add the customizations under the `services` field.

### Adjust resources

Note that you will only need to list the fragments that you would like to change from the base file.

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  gitserver-0:
    cpus: 8
    mem_limit: '26g'
```

### Add replica endpoints

When adding a new replica for `gitserver`, `searcher`, `symbols`, and `indexed-search`, you must list the endpoints for each replica individually in order for frontend to communicate with them.

To do that, add or modify the environment variables to all of the sourcegraph-frontend-* services and the sourcegraph-frontend-internal service in the [Docker Compose YAML file](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml).

#### version older than 4.5.0

The following configuration in a docker-compose.override.yaml file shows how to list the endpoints for each replica service individually when the replica count for gitserver, searcher, symbols, and indexed-search has been increased to 2. This is done by using the environment variables specified for each service:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  sourcegraph-frontend-0:
    environment:
      #  List all replica endpoints for gitserver
      - 'SRC_GIT_SERVERS=gitserver-0:3178 gitserver-1:3178'
      #  List all replica endpoints for indexed-search/zoekt-webserver
      - 'INDEXED_SEARCH_SERVERS=zoekt-webserver-0:6070 zoekt-webserver-1:6070'
      #  List all replica endpoints for searcher
      - 'SEARCHER_URL=http://searcher-0:3181 http://searcher-1:3181'
      #  List all replica endpoints for symbols
      - 'SYMBOLS_URL=http://symbols-0:3184 http://symbols-1:3184'
```

The above configuration uses the environment variables SRC_GIT_SERVERS, INDEXED_SEARCH_SERVERS, SEARCHER_URL, and SYMBOLS_URL to specify the individual endpoints for each replica service. This is done by listing the hostname and port number for each replica, separated by a space.

#### version 4.5.0 or above

In version 4.5.0 or above of Sourcegraph, it is possible to update the environment variables in the docker-compose.override.yaml file to automatically generate the endpoints based on the number of replicas provided. This eliminates the need to list each replica endpoint individually as in the previous example.

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  sourcegraph-frontend-0:
    environment:
      #  To generate replica endpoints for gitserver
      - 'SRC_GIT_SERVERS=2'
      #  To generate replica endpoints for indexed-search/zoekt-webserver
      - 'INDEXED_SEARCH_SERVERS=2'
      #  To generate replica endpoints for searcher
      - 'SEARCHER_URL=1'
      #  To generate replica endpoints for symbols
      - 'SYMBOLS_URL=1'
```

In the above example, the value of the environment variables `SRC_GIT_SERVERS`, `INDEXED_SEARCH_SERVERS`, `SEARCHER_URL`, and `SYMBOLS_URL` are set to the number of replicas for each respective service. This allows Sourcegraph to automatically generate the endpoints for each replica, eliminating the need to list them individually. This can be a useful feature when working with large numbers of replicas.

### Create multiple gitserver shards

Split gitserver across multiple shards:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
# Adjust resources for gitserver-0
# And then create an anchor to share with the replica
  gitserver-0: &gitserver
    cpus: 8
    mem_limit: '26g'
# Create a new service called gitserver-1,
# which is an extension of gitserver-0
  gitserver-1:
  # Extend the original gitserver-0 to get the image values etc
    extends:
      file: docker-compose.yaml
      service: gitserver-0
    # Use the new resources values from gitserver-0 above
    <<: *gitserver
    # Since this is an extension of the original gitserver-0,
    # we will have to rename the container name to gitserver-1
    container_name: gitserver-1
    # Assign it to a new volume which we will create below in the volumes section
    volumes:
      - 'gitserver-1:/data/repos'
    # Assign a new host name so it doesn't use the gitserver-0 one
    hostname: gitserver-1
# Add the new replica to other related services as environment
  sourcegraph-frontend-0: &frontend
    cpus: 6
    mem_limit: '6g'
    # Set the following environment variables to generate the replica endpoints
    environment: &env_gitserver
      - 'SRC_GIT_SERVERS=2'
    # IMPORTANT: For version below 4.3.1, you must list the endpoints individually
      # - &env_gitserver 'SRC_GIT_SERVERS=gitserver-0:3178 gitserver-1:3178'
# Use the same override values as sourcegraph-frontend-0 above
  sourcegraph-frontend-internal:
    <<: *frontend
# Add the updated environment for gitserver from frontend to worker using anchor
  worker:
    environment:
      - *env_gitserver
# Add a new volume assigned to the new gitserver replica
volumes:
  gitserver-1:
```

### Disable a service

You can "disable services" by assigning them to one or more [profiles](https://docs.docker.com/compose/profiles/), so that when running the `docker compose up` command, services assigned to profiles will not be started unless explicitly specified in the command (e.g., `docker compose --profile disabled up`).

For example, when you need to disable the internal codeintel-db in order to use an external database, you can assign `codeintel-db` to a profile called `disabled`:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  codeintel-db:
    profiles:
      - disabled
```

### Enable tracing

Tracing should be enabled in the `docker-compose.yaml` file by default.

If not, you can enable it by setting the environment variable to `SAMPLING_STRATEGIES_FILE=/etc/jaeger/sampling_strategies.json` in the `jaeger` container:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  jaeger:
    environment:
      - 'SAMPLING_STRATEGIES_FILE=/etc/jaeger/sampling_strategies.json'
```

### Enabling Embeddings service

The Embeddings service handles searching embeddings for Cody context. It can be enabled using the [override file](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-docker/-/blob/docker-compose/embeddings/embeddings.docker-compose.yaml)

#### Configuring the Embeddings service
By default the Embeddings service uses the `blobstore` service for storing embeddings indexes.
To configure an external [object store](./../../../cody/core-concepts/embeddings/manage-embeddings.md#store-embedding-indexes) the override file can modified by setting environment variables. These variables **must** be set on both the `worker` and `embeddings` services.

### Git configuration

#### Git SSH configuration

Provide your `gitserver` instance with your SSH / Git configuration (e.g. `.ssh/config`, `.ssh/id_rsa`, `.ssh/id_rsa.pub`, and `.ssh/known_hosts`. You can also provide other files like `.netrc`, `.gitconfig`, etc. if needed) by mounting a directory that contains this configuration into the `gitserver` container.

For example, in the `gitserver-0` container configuration in your `docker-compose.yaml` file or `docker-compose.override.yaml`, add the volume listed in the following example, while replacing `~/path/on/host/` with the path on the host machine to the `.ssh` directory:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  gitserver-0:
    volumes:
      - 'gitserver-0:/data/repos'
      - '~/path/on/host/.ssh:/home/sourcegraph/.ssh'
```

> WARNING: The permissions on your SSH / Git configuration must be set to be readable by the user in the `gitserver` container. For example, run `chmod -v -R 600 ~/path/to/.ssh` in the folder on the host machine.

#### Git HTTP(S) authentication

The easiest way to specify HTTP(S) authentication for repositories is to include the username and password in the clone URL itself, such as `https://user:password@example.com/my/repo`. These credentials won't be displayed to non-admin users.

Otherwise, follow the previous steps for mounting SSH configuration to mount a host directory containing the desired `.netrc` file to `/home/sourcegraph/` in the `gitserver` container.

### Expose debug port

To [generate pprof profiling data](../../pprof.md), you must configure your deployment to expose port 6060 on one of your frontend containers, for example:

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  sourcegraph-frontend-0:
    ports:
      - '0.0.0.0:6060:6060'
```

For specific ports that can be exposed, see the [debug ports section](../../pprof.md#debug-ports) of Sourcegraphs's [generate pprof profiling data](../../pprof.md) docs.

### Set environment variables

Add/modify the environment variables to all of the sourcegraph-frontend-* services and the sourcegraph-frontend-internal service in the [Docker Compose YAML file](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml):

```yaml
# docker-compose.override.yaml
version: '2.4'
services:
  sourcegraph-frontend-0:
    environment:
      - (YOUR CODE)
```

See ["Environment variables in Compose"](https://docs.docker.com/compose/environment-variables/) for other ways to pass these environment variables to the relevant services (including from the command line, a .env file, etc.).


### Use an external database

The Docker Compose configuration has its own internal PostgreSQL and Redis databases.

You can alternatively configure Sourcegraph to [use external services](../../external_services/index.md).
