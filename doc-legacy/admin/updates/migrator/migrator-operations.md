# `migrator` operations

This document provides a list of the `migrator` commands available to admins for manual invocation, as well as instructions migrator's operation in specific deployment environments.

> NOTE: Admins should always use the latest `migrator` image release, even against older Sourcegraph instances. This is especially true for commands such as `downgrade`, `drift`, `run-out-of-band-migrations`, and `upgrade`.
> 
> The exception to this rule is the default `migrator` command `up`. `up` should always be run using the `migrator` image version corresponding with the Sourcegraph version being deployed. In most deployments this is the default migrator command used on Sourcegraph startup.

## Environment specific operations

To run a `migrator` command, follow the guide for your Sourcegraph distribution type:

- Kubernetes
  - [Kustomize](#kubernetes-kustomize)
  - [Helm](#kubernetes-helm)
- [Docker-compose](#docker-compose)
- [Docker Single-node](#single-node)
- [Local development](#local-development)

## Overview

The `migrator` is a Sourcegraph service whose purpose is to managed the state of `pgsql`, `codeintel-db`, and `codeinsights-db` database schemas.

Whenever a `docker-compose` or `kubernetes` based deployment is started `migrator` is run with the default `up` command. In kubernetes as an [`initContainer`](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/base/frontend/sourcegraph-frontend.Deployment.yaml#L30-L40), and in [docker-compose](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml#L14-L53) as a `depends_on` of the `frontend`. `up` runs all necessary schema migrations in the case of a [standard upgrade](../index.md#upgrade-types), and then exits. The `frontend` service will not start until `migrator` exits successfully. 

`migrator` can also be used like a command line tool, accepting *command* arguments with *flags*. The most common usage of `migrator` this way is to perform [multiversion upgrades](../index.md#upgrade-types) with the [`upgrade`](#upgrade) command. Many other commands are also available.

Some notes on `migrator`:
- The `migrator` service was introduced in `v3.37.0`.
- `migrator`'s `upgrade` command was introduced in `v4.0.0` and is intended to be used to upgrade versions as old as `3.20.x`. **Always check your deployment types [upgrade notes](../index.md#upgrades-index) before a multiversion upgrade.**
- Commands such as `downgrade`, `drift`, `run-out-of-band-migrations`, and `upgrade`, all work against Sourcegraph versions as old as `v3.20.x`.

## Migrator environment variables

Migrator uses environemt variables to target the correct database instances. By default these values are configured to target Sourcegraphs locally deployed databases. These values may be adjusted to connect migrator to externally managed databases.

Manifest loactions:
- [Docker-compose](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml#L20C4-L43)
- [Kustomize](https://github.com/sourcegraph/deploy-sourcegraph-k8s/blob/main/components/utils/migrator/resources/sourcegraph-frontend.ConfigMap.yaml#L10-L28)
- [Legacy](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/migrator/migrator.Job.yaml#L23C13-L59)
- [Helm](https://github.com/sourcegraph/deploy-sourcegraph-helm/blob/main/charts/sourcegraph-migrator/values.yaml#L39-L79)

Example default environment variables:
```yaml
PGHOST: "pgsql"
PGPORT: "5432"
PGUSER: "sg"
PGPASSWORD: "sg"
PGDATABASE: "sg"
PGSSLMODE: "disable"
CODEINTEL_PGHOST: "codeintel-db"
CODEINTEL_PGPORT: "5432"
CODEINTEL_PGUSER: "sg"
CODEINTEL_PGPASSWORD: "sg"
CODEINTEL_PGDATABASE: "sg"
CODEINTEL_PGSSLMODE: "disable"
CODEINSIGHTS_PGHOST: "codeinsights-db"
CODEINSIGHTS_PGPORT: "5432"
CODEINSIGHTS_PGUSER: "postgres"
CODEINSIGHTS_PGPASSWORD: "password"
CODEINSIGHTS_PGDATABASE: "postgres"
CODEINSIGHTS_PGSSLMODE: "disable"
```

## Commands

The `migrator` service exposes the following commands:

### upgrade

The `upgrade` command performs database schema migrations and out-of-band migrations to rewrite existing instance data in-place into the shaped expected by a given target Sourcegraph version. This command is used by site-administrators to perform [multi-version upgrades](../index.md#upgrade-types).

```sh
upgrade \
    --from=<current version> --to=<target version> \
    [--dry-run=false] \
    [--disable-animation=false] \
    [--skip-version-check=false] [--skip-drift-check=false] \
    [--unprivileged-only=false] [--noop-privileged=false] [--privileged-hash=<hash>] \
    [--ignore-migrator-update=false]
```

**Required arguments**:

- `--from`: The current Sourcegraph release version (*without the patch*; e.g., `v3.36`)
- `--to`: The target Sorucegraph release version (*without the patch*; e.g., `v4.0`)

**Optional arguments**:

- `--dry-run`: Print the steps of the upgrade but do not execute them.
- `--disable-animation`: Print plain log messages instead of an animated progress bar.
- `--skip-version-check`: Skip comparing the current instance version against `--from`.
- `--skip-drift-check`: Skip comparing the database schema shape against the schema defined by `--from`.
- `--unprivileged-only` and `--noop-privileged`: Controls behavior of schema migrations the presence of [privileged definitions](../../how-to/privileged_migrations.md).
- `--ignore-migrator-update`: Controls whether to hard- or soft-fail if a newer migrator version is available. It is recommended to use the latest migrator version.

**Notes**:

- Successive invocations of this command will re-attempt the last failed or attempted (but incomplete) migration. This command run as if the `--ignore-single-{dirty,pending}-log` flags supplied by the commands `up`, `upto`, and `downto` were enabled.
- This command checks that the schema of the database is in the correct state for the current version, if schema drift is detected it must be resolved before completing the upgrade. [Learn more here.](./schema-drift.md).
- Successive invocations of this command may *cause* database drift when partial progress is made. When making a subsequent upgrade attempt, invoke this command with `--skip-drift-check` ignore the failing startup check.

### drift

The `drift` command describes the current (live) database schema and compares it against the expected schema at the given version. The output of this command will include all relevant schema differences that could affect application correctness and performance. When schema drift is detected, a diff of the expected and actual Postgres object definitions will be shown, along with instructions on how to manually resolve the disparity. [Learn more here.](./schema-drift.md)

```sh
drift \
    --db=<schema> \
    [--version=<version>] \
    [--file=<path to description file>] \
    [--ignore-migrator-update=false]
```

**Required arguments**:

- `--db`: The target schema to inspect. *Ex: frontend, codeintel, codeinsights*

**Mutually exclusive arguments**:

Exactly one of `--version` and `--file` must be supplied.

- `--version`: The instance's current Sourcegraph release version *including a patch* (e.g., `v3.42.1`).
- `--file`: The filepath to a local schema description file. This is useful for airgapped instances that do not have access to the public Sourcegraph GitHub repository or the public GCS bucket where old revisions have been backfilled.
- `--ignore-migrator-update`: Controls whether to hard- or soft-fail if a newer migrator version is available. It is recommended to use the latest migrator version.

### downgrade

The `downgrade` command performs database schema migrations and (reverse-applied) out-of-band migrations to rewrite existing instance data in-place into the shaped expected by a given target Sourcegraph version.

```sh
downgrade \
    --from=<current version> --to=<target version> \
    [--dry-run=false] \
    [--disable-animation=false] \
    [--skip-version-check=false] [--skip-drift-check=false] \
    [--unprivileged-only=false] [--noop-privileged=false] [--privileged-hash=<hash>] \
    [--ignore-migrator-update=false]
```

**Required arguments**:

- `--from`: The current Sourcegraph release version (*without the patch*; e.g., `v3.36`)
- `--to`: The target Sorucegraph release version (*without the patch*; e.g., `v4.0`)

**Optional arguments**:

- `--dry-run`: Print the steps of the upgrade but do not execute them.
- `--disable-animation`: Print plain log messages instead of an animated progress bar.
- `--skip-version-check`: Skip comparing the current instance version against `--from`.
- `--skip-drift-check`: Skip comparing the database schema shape against the schema defined by `--from`.
- `--unprivileged-only` and `--noop-privileged`: Controls behavior of schema migrations the presence of [privileged definitions](../../how-to/privileged_migrations.md).
- `--ignore-migrator-update`: Controls whether to hard- or soft-fail if a newer migrator version is available. It is recommended to use the latest migrator version.

**Notes**:

- Successive invocations of this command will re-attempt the last failed or attempted (but incomplete) migration. This command run as if the `--ignore-single-{dirty,pending}-log` flags supplied by the commands `up`, `upto`, and `downto` were enabled.
- Successive invocations of this command may *cause* database drift when partial progress is made. When making a subsequent downgrade attempt, invoke this command with `--skip-drift-check` ignore the failing startup check.

### add-log

The `add-log` command adds an entry to the `migration_logs` table after a site administrator has explicitly applied the contents of a migration definition. This command may be performed by a site-administrator as part of [repairing a dirty database](../../how-to/dirty_database.md#3-add-a-migration-log-entry).

```sh
add-log \
    --db=<schema> \
    --version=<version> \
    [--up=true]
```

**Required arguments**:

- `--db`: The target schema to modify.
- `--version`: The migration identifier noted on the log entry.

**Optional arguments**:

- `--up`: The migration direction noted on the log entry.

### validate

The `validate` command validates the current state of the database (both schema and data migration progress). This command is used on Sourcegraph instance startup of database-dependent services to ensure that the migrator has been run to the expected version.

```sh
validate \
    [--db=all] \
    [--skip-out-of-band-migrations=false]
```

**Optional arguments**:

- `--db`: The target schema(s) to validate. Comma-separated values are allowed.
- `--skip-out-of-band-migrations`: Skip validation of out-of-band migrations. Validate the schema only.

**Notes**:

- If `DISABLE_CODE_INSIGHTS` is not set and the `codeinsights-db` is not available, then this command will fail with the default value for the `--db` flag. To resolve, supply `--db=frontend,codeintel` instead.

### up

The `up` command (the default behavior of the `migrator` service) applies all migrations defined at the time the `migrator` was built. This command is used on Sourcegraph instance startup to ensure the database schema is up to date prior to starting other services that depend on the database.

> WARNING: The target migration leaves of this command are defined at `migrator` **compile time** and does not accept a version argument. This is the only command where the Sourcegraph instance version and `migrator` version are expected to match.

Users should generally prefer the command [`upto`](#upto), which accepts more explicit bounds and does not depend on the migrator compilation version.

```sh
up \
    [--db=all] \
    [--skip-upgrade-validation=false] \
    [--skip-oobmigration-validation=false]
    [--ignore-single-dirty-log=false] [--ignore-single-pending-log=false] \
    [--unprivileged-only=false] [--noop-privileged=false] [--privileged-hash=<hash>]
```

**Optional arguments**:

- `--db`: The target schema(s) to modify. Comma-separated values are allowed.
- `--skip-upgrade-validation`: Skip asserting that the [standard upgrade policy](../index.md#upgrade-types) is being followed.
- `--skip-oobmigration-validation`: Skip reading the progress of out-of-band migrations to assert completion of newly deprecated migrations.
- `--ignore-single-dirty-log` and `--ignore-single-pending-log`: Re-attempt to apply the **next** migration that was marked as errored or as incomplete (respectively). See [how to troubleshoot a dirty database](../../how-to/dirty_database.md#0-attempt-re-application).
- `--unprivileged-only` and `--noop-privileged`: Controls behavior of schema migrations the presence of [privileged definitions](../../how-to/privileged_migrations.md).

**Notes**:

- If `DISABLE_CODE_INSIGHTS` is not set and the `codeinsights-db` is not available, then this command will fail with the default value for the `--db` flag. To resolve, supply `--db=frontend,codeintel` instead.

### run-out-of-band-migrations

The `run-out-of-band-migrations` command runs out-of-band migrations within the `migrator`. This command may be performed by a site-administrator as part of [repairing an unfinished migration](../../how-to/unfinished_migration.md).

```sh
run-out-of-band-migrations \
    [--id <id>]+ \
    [--apply-reverse=false] \
    [--disable-animation=false]
```

**Required arguments**:

- `--id`: The identifier(s) of the migrations to apply. Multiple flags can be supplied. If no flag is supplied, all migrations are applied.
- `--apply-reverse`: Run migrations in the reverse direction.

**Optional arguments**:

- `--disable-animation`: Print plain log messages instead of an animated progress bar.

### describe

The `describe` command outputs a dump of your database schema.

```sh
describe \
    --db=<schema> \
    --format=<json|psql> \
    [--out=stdout] \
    [--force=false] \
    [--no-color=false]
```

**Required arguments**:

- `--db`: The target schema to describe.
- `--format`: The format in which the description is output.

**Optional arguments**:

- `--out`: The target output file. If not supplied, the output is printed to stdout.
- `--force`: Overwrite the file.
- `--no-color`: Do not print ANSI color sequences.

## Environments

- Kubernetes
  - [Helm](#kubernetes-helm)
  - [Kustomize](#kubernetes-kustomize)
- [Docker-compose](#docker-compose)
- [Docker Single-node](#single-node)
- [Local development](#local-development)

Generally in production environments `migrator` is run by updating the startup **command** and **image** of `migrator` in the manifest defining migrator. Below you can find general instructions on how to manipulate `migrator`'s manifest to invoke a **command**.

### Kubernetes Kustomize

In kubernetes `migrator` is initialized as a [kubernetes job](https://kubernetes.io/docs/concepts/workloads/controllers/job/). The `job` is initialized with arguments passed to the `args:` key. Below are links to the job manifests in our kustomize and legacy deployments:
- [kustomize](https://github.com/sourcegraph/deploy-sourcegraph-k8s/blob/main/components/utils/migrator/resources/migrator.Job.yaml)
  - [*configMap*](https://github.com/sourcegraph/deploy-sourcegraph-k8s/blob/main/components/utils/migrator/resources/sourcegraph-frontend.ConfigMap.yaml)
- [legacy](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/migrator/migrator.Job.yaml#LL21C1-L23C1)
  
To run `migrator` with a specific *command*: 

1. Edit the `migrator.Job.yaml` files `args:` value to the intended command with flags, and alter the `image:` value to the latest released image of migrator (*in the case of the `up` command use your current deployment version*).

For example here the `upgrade` command:
```yaml
containers:
  - name: migrator
    image: "index.docker.io/sourcegraph/migrator:5.0.3"
    args: ["upgrade", "--from=v3.41.0", "--to=v4.5.1"]
    envFrom:
```

2. Apply the job and wait for it to complete.

```sh
# Delete previous job (if one exists)
kubectl delete -f configure/migrator/migrator.Job.yaml --ignore-not-found=true

# Apply the manifest and wait for the operation to complete before continuing
kubectl apply -f configure/migrator/migrator.Job.yaml
```

3. Check the logs from execution
   
```sh
$ kubectl logs job.batch/migrator -f
```

### Kubernetes Helm

Running migrator operations in helm takes advantage of the [sourcegraph-migrator helm charts](https://github.com/sourcegraph/deploy-sourcegraph-helm/tree/main/charts/sourcegraph-migrator). You can learn more about general operations and find some examples in the repo where the charts are defined.

Generally the migrator is run via the `helm upgrade`, for example:

```
$ helm upgrade --install -n sourcegraph --set "migrator.args={drift,--db=frontend,--version=v3.39.1}" sourcegraph-migrator sourcegraph/sourcegraph-migrator --version 4.4.2
```

In the example above the `drift` operation is run with flags `--db` and `--version`. The migrator is run using image version `v4.4.2`.

Arguments are set with the `--set "migrator.args={operation-arg,flag-arg-1,flag-arg-2}` portion of the command. Just like you would run commands in terminal, these are the args you are telling the migrator to run on initialization.

In the most general form running operations follows this template:

```
$ helm upgrade --install -n <your namespace> --set "migrator.args={<arg1>,<arg2>,<arg3>}" sourcegraph-migrator sourcegraph/sourcegraph-migrator --version <migrator image version> 
```

> NOTE: You can troubleshoot a migrators operations with the command `kubectl -n sourcegraph logs -l job=migrator -f`. This will show you logs from the migrator jobs operation steps.

### Docker-compose

In Docker-compose `migrator` is initialized as a [container](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml#L14-L53) in the `docker-compose.yaml` file.
  
To run `migrator` with a specific *command*: 

1. Edit the `docker-compose.yaml` file `command:` value to the intended command with flags, and alter the `image:` value to the latest released image of migrator (*in the case of the `up` command use your current deployment version*).

For example here the `upgrade` command:
```yaml
migrator:
  container_name: migrator
  image: 'index.docker.io/sourcegraph/migrator:5.0.3'
  cpus: 0.5
  mem_limit: '500m'
  command: ['upgrade', '--from=v3.41.0', '--to=v4.5.1']
  environment:
```

2. Apply the job and wait for it to complete.

```sh
$ docker-compose up -d migrator
```

3. The log output of `migrator` can be checked with the command
```sh
$ docker logs migrator
```

> Note: Remember to set the `command:` back to `up` and `image:` back to your deployment version if you are starting Souregraph again.

### Single-node

Run the following commands on your Docker host.

> NOTE: These values will work for a standard docker-compose deployment of Sourcegraph. If you've customized your deployment (e.g., using an external database service), you will have to modify the environment variables accordingly.

```sh
export MIGRATOR_SOURCEGRAPH_VERSION="..."

docker run \
  --rm \
  --name migrator_$MIGRATOR_SOURCEGRAPH_VERSION \
  -e PGHOST='pgsql' \
  -e PGPORT='5432' \
  -e PGUSER='sg' \
  -e PGPASSWORD='sg' \
  -e PGDATABASE='sg' \
  -e PGSSLMODE='disable' \
  -e CODEINTEL_PGHOST='codeintel-db' \
  -e CODEINTEL_PGPORT='5432' \
  -e CODEINTEL_PGUSER='sg' \
  -e CODEINTEL_PGPASSWORD='sg' \
  -e CODEINTEL_PGDATABASE='sg' \
  -e CODEINTEL_PGSSLMODE='disable' \
  -e CODEINSIGHTS_PGHOST='codeinsights-db' \
  -e CODEINSIGHTS_PGPORT='5432' \
  -e CODEINSIGHTS_PGUSER='postgres' \
  -e CODEINSIGHTS_PGPASSWORD='password' \
  -e CODEINSIGHTS_PGDATABASE='postgres' \
  -e CODEINSIGHTS_PGSSLMODE='disable' \
  --network=docker-compose_sourcegraph \
  sourcegraph/migrator:$MIGRATOR_SOURCEGRAPH_VERSION \
  # Optional (defaults to `["up", "-db", "all"]`)
  "add" "quoted" "arguments"
```

Observe the output of the `migrator` container via:

```sh
$ docker logs migrator_$SOURCEGRAPH_VERSION
```

The log output of the `migrator` should include `INFO`-level logs and successfully terminate with `migrator exited with code 0`. If you see an error message or any of the databases have been flagged as "dirty", please follow ["How to troubleshoot a dirty database"](../../../admin/how-to/dirty_database.md). A dirty database will not affect your ability to use Sourcegraph however it will need to be resolved to upgrade further. If you are unable to resolve the issues, contact support at <mailto:support@sourcegraph.com> for further assistance. Otherwise, you are now safe to upgrade Sourcegraph.

### Local development

To run the migrator locally, simply run `go run ./cmd/migrator`.

Many of the commands detailed above are also available via `sg`. Replace `migrator` with `sg migration ...`. There are a few command registered on the `migrator` but not on `sg` (e.g., `upgrade` and `downgrade`), as local environments are a bit of a different beast than environments performing upgrades only along tagged releases.
