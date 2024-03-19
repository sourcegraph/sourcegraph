# Using your own PostgreSQL server

You can use your own PostgreSQL v12+ server with Sourcegraph if you wish. For example, you may prefer this if you already have existing backup infrastructure around your own PostgreSQL server, wish to use Amazon RDS, etc.

Please review [the PostgreSQL](../postgres.md) documentation for a complete list of requirements.

> NOTE: As of version 3.39.0, codeinsights-db no longer relies on the internal TimescaleDB and can be externalized.


## General recommendations

If you choose to set up your own PostgreSQL server, please note **we strongly recommend each database to be set up in different servers and/or hosts**. We suggest either:

1. Deploy _codeintel-db_ alongside the other Sourcegraph containers, i.e. not as a managed PostgreSQL instance.
2. Deploy a separate PostgreSQL instance. The primary reason to not use the same Postgres instance for this data is because code graph data can take up a significant of space (given the amount of indexed repositories is large) and the performance of the database may impact the performance of the general application database. You'll most likely want to be able to scale their resources independently.

We also recommend having backups for the _codeintel-db_ as a best practice. The reason behind this recommendation is that _codeintel-db_ data is uploaded via CI systems. If data is lost, Sourcegraph cannot automatically rebuild it from the repositories, which means you'd have to wait until it is re-uploaded from your CI systems.

## Instructions

The addition of `PG*` environment variables to your Sourcegraph deployment files will instruct Sourcegraph to target an external PostgreSQL server. To externalize the _frontend database_, use the following standard `PG*` variables:

- `PGHOST`
- `PGPORT`
- `PGUSER`
- `PGPASSWORD`
- `PGDATABASE`
- `PGSSLMODE`

To externalize the _code navigation database_, use the following prefixed `CODEINTEL_PG*` variables:

- `CODEINTEL_PGHOST`
- `CODEINTEL_PGPORT`
- `CODEINTEL_PGUSER`
- `CODEINTEL_PGPASSWORD`
- `CODEINTEL_PGDATABASE`
- `CODEINTEL_PGSSLMODE`

> NOTE: ⚠️ If you have configured both the frontend (pgsql) and code navigation (codeintel-db) databases with the same values, the Sourcegraph instance will refuse to start. Each database should either be configured to point to distinct hosts (recommended), or configured to point to distinct databases on the same host.

### sourcegraph/server

Add the following to your `docker run` command:

<!--
  DO NOT CHANGE THIS TO A CODEBLOCK.
  We want line breaks for readability, but backslashes to escape them do not work cross-platform.
  This uses line breaks that are rendered but not copy-pasted to the clipboard.
-->

<pre class="pre-wrap start-sourcegraph-command"><code>docker run [...]<span class="virtual-br"></span> -e PGHOST=psql1.mycompany.org<span class="virtual-br"></span> -e PGUSER=sourcegraph<span class="virtual-br"></span> -e PGPASSWORD=secret<span class="virtual-br"></span> -e PGDATABASE=sourcegraph<span class="virtual-br"></span> -e PGSSLMODE=require<span class="virtual-br"> -e CODEINTEL_PGHOST=psql2.mycompany.org<span class="virtual-br"></span> -e CODEINTEL_PGUSER=sourcegraph<span class="virtual-br"></span> -e CODEINTEL_PGPASSWORD=secret<span class="virtual-br"></span> -e CODEINTEL_PGDATABASE=sourcegraph-codeintel<span class="virtual-br"></span> -e CODEINTEL_PGSSLMODE=require<span class="virtual-br"></span> sourcegraph/server:5.2.5</code></pre>

### Docker Compose

1. Add/modify the following environment variables to all of the `sourcegraph-frontend-*` services, the `sourcegraph-frontend-internal` service, and the `migrator` service (for Sourcegraph versions 3.37+) in [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/3.37/docker-compose/docker-compose.yaml):

    ```
    sourcegraph-frontend-0:
      # ...
      environment:
        # ...
        - 'PGHOST=psql1.mycompany.org'
        - 'PGUSER=sourcegraph'
        - 'PGPASSWORD=secret'
        - 'PGDATABASE=sourcegraph'
        - 'PGSSLMODE=require'
        - 'CODEINTEL_PGHOST=psql2.mycompany.org'
        - 'CODEINTEL_PGUSER=sourcegraph'
        - 'CODEINTEL_PGPASSWORD=secret'
        - 'CODEINTEL_PGDATABASE=sourcegraph-codeintel'
        - 'CODEINTEL_PGSSLMODE=require'
      # ...
    ```

    See ["Environment variables in Compose"](https://docs.docker.com/compose/environment-variables/) for other ways to pass these environment variables to the relevant services (including from the command line, a `.env` file, etc.).

1. Comment out / remove the internal `pgsql` and `codeintel-db` services in [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/3.37/docker-compose/docker-compose.yaml) since Sourcegraph is using the external one now.

    ```
    # # Description: PostgreSQL database for various data.
    # #
    # # Disk: 128GB / persistent SSD
    # # Ports exposed to other Sourcegraph services: 5432/TCP 9187/TCP
    # # Ports exposed to the public internet: none
    # #
    # pgsql:
    # container_name: pgsql
    # image: 'index.docker.io/sourcegraph/postgres-11.4:19-11-14_b084311b@sha256:072481559d559cfd9a53ad77c3688b5cf583117457fd452ae238a20405923297'
    # cpus: 4
    # mem_limit: '2g'
    # healthcheck:
    #    test: '/liveness.sh'
    #    interval: 10s
    #    timeout: 1s
    #    retries: 3
    #    start_period: 15s
    # volumes:
    #    - 'pgsql:/data/'
    # networks:
    #     - sourcegraph
    # restart: always

    # # Description: PostgreSQL database for code navigation data.
    # #
    # # Disk: 128GB / persistent SSD
    # # Ports exposed to other Sourcegraph services: 5432/TCP 9187/TCP
    # # Ports exposed to the public internet: none
    # #
    # codeintel-db:
    #   container_name: codeintel-db
    #   image: 'index.docker.io/sourcegraph/codeintel-db@sha256:63090799b34b3115a387d96fe2227a37999d432b774a1d9b7966b8c5d81b56ad'
    #   cpus: 4
    #   mem_limit: '2g'
    #   healthcheck:
    #     test: '/liveness.sh'
    #     interval: 10s
    #     timeout: 1s
    #     retries: 3
    #     start_period: 15s
    #   volumes:
    #     - 'codeintel-db:/data/'
    #   networks:
    #     - sourcegraph
    #   restart: always
    ```

### Kubernetes

Update the `PG*` and `CODEINTEL_PG*` environment variables in the `sourcegraph-frontend` deployment YAML file to point to the external frontend (`pgsql`) and code navigation (`codeintel-db`) PostgreSQL instances, respectively. Again, these must not point to the same database or the Sourcegraph instance will refuse to start.

You are then free to remove the now unused `pgsql` and `codeintel-db` services and deployments from your cluster.

### Version requirements

Please refer to our [Postgres](https://docs.sourcegraph.com/admin/postgres) documentation to learn about version requirements.

### Caveats

> NOTE: If your PostgreSQL server does not support SSL, set `PGSSLMODE=disable` instead of `PGSSLMODE=require`. Note that this is potentially insecure.

Most standard PostgreSQL environment variables may be specified (`PGPORT`, etc). See http://www.postgresql.org/docs/current/static/libpq-envars.html for a full list.

> NOTE: On Mac/Windows, if trying to connect to a PostgreSQL server on the same host machine, remember that Sourcegraph is running inside a Docker container inside of the Docker virtual machine. You may need to specify your actual machine IP address and not `localhost` or `127.0.0.1` as that refers to the Docker VM itself.

----

## Usage with AWS RDS IAM Auth

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> 
This method is experimental and may have performance implication. Please reach out to your account team for support.
</p>
</aside>

For AWS RDS for Postgres, you have the option to use IAM database authentication to avoid using static database credentials. Learn more from [AWS documentation](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html).

In order to enable IAM Auth, you first need to:

- enabled [IAM authentication on the RDS instance](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.Enabling.html)
- created [the database account using IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html#UsingWithRDS.IAMDBAuth.DBAccounts.PostgreSQL)
- created [IAM policy for IAM database access](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.IAMPolicy.html)
- created IAM roles for your compute workload and grant the role with the above policy
- ensured your compute resources can assume those IAM roles
  - For EKS (k8s deployment), use [IAM roles for service accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
  - For EC2 (docker-compose deployment), use [IAM roles for Amazon EC2](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html)

For [every services that require postgres database connection](https://github.com/sourcegraph/sourcegraph/blob/main/lib/servicecatalog/service-catalog.yaml), ensure below environment variables are configured:

- `PG_CONNECTION_UPDATER=EC2_ROLE_CREDENTIALS`
- `PGSSLMODE=require`
- `PGHOST=<>`
- `PGPORT=<>`
- `PGUSER=<>` - this should be the database accounts created above
- `PGDATABASE=<>`
- `CODEINTEL_PGSSLMODE=require`
- `CODEINTEL_PGPORT=<>`
- `CODEINTEL_PGUSER=<>` this should be the database accounts created above
- `CODEINTEL_PGDATABASE=<>`
- `CODEINSIGHTS_PGSSLMODE=require`
- `CODEINSIGHTS_PGHOST=<>`
- `CODEINSIGHTS_PGPORT=<>`
- `CODEINSIGHTS_PGUSER=<>` this should be the database accounts created above
- `CODEINSIGHTS_PGDATABASE=<>`

## Usage with PgBouncer

[PgBouncer] is a lightweight connections pooler for PostgreSQL. It allows more clients to connect with the PostgreSQL database without running into connection limits.

When [PgBouncer] is used, we need to include `statement_cache_mode=describe` in the PostgreSQL connection url. This can be done by configuring the `PGDATASOURCE` and `CODEINSIGHTS_PGDATASOURCE` environment variables to `postgres://username:password@pgbouncer.mycompany.com:5432/sg?statement_cache_mode=describe`

### sourcegraph/server

Add the following to your `docker run` command:

<pre class="pre-wrap start-sourcegraph-command"><code>docker run [...]<span class="virtual-br"></span> -e PGDATASOURCE="postgres://username:password@sourcegraph-pgbouncer.mycompany.com:5432/sg?statement_cache_mode=describe"<span class="virtual-br"></span> -e CODEINSIGHTS_PGDATASOURCE="postgres://username:password@sourcegraph-codeintel-pgbouncer.mycompany.com:5432/sg?statement_cache_mode=describe"<span class="virtual-br"></span> sourcegraph/server:5.2.5</code></pre>

### Docker Compose

1. Add/modify the following environment variables to all of the `sourcegraph-frontend-*` services, the `sourcegraph-frontend-internal` service, and the `migrator` service (for Sourcegraph versions 3.37+) in [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/3.37/docker-compose/docker-compose.yaml):

    ```yml
    sourcegraph-frontend-0:
      # ...
      environment:
        # ...
        - 'PGDATASOURCE=postgres://username:password@sourcegraph-pgbouncer.mycompany.com:5432/sg?statement_cache_mode=describe'
        - 'CODEINSIGHTS_PGDATASOURCE=postgres://username:password@sourcegraph-codeintel-pgbouncer.mycompany.com:5432/sg?statement_cache_mode=describe'
      # ...
    ```

    See ["Environment variables in Compose"](https://docs.docker.com/compose/environment-variables/) for other ways to pass these environment variables to the relevant services (including from the command line, a `.env` file, etc.).

1. Comment out / remove the internal `pgsql` and `codeintel-db` services in [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/3.37/docker-compose/docker-compose.yaml) since Sourcegraph is using the external one now.

    ```yml
    # # Description: PostgreSQL database for various data.
    # #
    # # Disk: 128GB / persistent SSD
    # # Ports exposed to other Sourcegraph services: 5432/TCP 9187/TCP
    # # Ports exposed to the public internet: none
    # #
    # pgsql:
    # container_name: pgsql
    # image: 'index.docker.io/sourcegraph/postgres-11.4:19-11-14_b084311b@sha256:072481559d559cfd9a53ad77c3688b5cf583117457fd452ae238a20405923297'
    # cpus: 4
    # mem_limit: '2g'
    # healthcheck:
    #    test: '/liveness.sh'
    #    interval: 10s
    #    timeout: 1s
    #    retries: 3
    #    start_period: 15s
    # volumes:
    #    - 'pgsql:/data/'
    # networks:
    #     - sourcegraph
    # restart: always

    # # Description: PostgreSQL database for code navigation data.
    # #
    # # Disk: 128GB / persistent SSD
    # # Ports exposed to other Sourcegraph services: 5432/TCP 9187/TCP
    # # Ports exposed to the public internet: none
    # #
    # codeintel-db:
    #   container_name: codeintel-db
    #   image: 'index.docker.io/sourcegraph/codeintel-db@sha256:63090799b34b3115a387d96fe2227a37999d432b774a1d9b7966b8c5d81b56ad'
    #   cpus: 4
    #   mem_limit: '2g'
    #   healthcheck:
    #     test: '/liveness.sh'
    #     interval: 10s
    #     timeout: 1s
    #     retries: 3
    #     start_period: 15s
    #   volumes:
    #     - 'codeintel-db:/data/'
    #   networks:
    #     - sourcegraph
    #   restart: always
    ```

### Kubernetes

Create a new [Secret](https://kubernetes.io/docs/concepts/configuration/secret/) to store the [PgBouncer] credentials.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sourcegraph-pgbouncer-credentials
data:
  # notes: secrets data has to be base64-encoded
  password: ""
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sourcegraph-codeintel-pgbouncer-credentials
data:
  # notes: secrets data has to be base64-encoded
  password: ""
```

Update the environment variables in the `sourcegraph-frontend` deployment YAML.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sourcegraph-frontend
spec:
  template:
    spec:
      containers:
      - name: frontend
        env:
        - name: PGDATABASE
          value: sg
        - name: PGHOST
          value: sourcegraph-pgbouncer
        - name: PGPORT
          value: "5432"
        - name: PGSSLMODE
          value: disable
        - name: PGUSER
          value: sg
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: sourcegraph-pgbouncer-credentials
              key: password
        - name: PGDATASOURCE
          value: postgres://$(PGUSER):$(PGPASSWORD)@$(PGHOST):$(PGPORT)/$(PGDATABASE)?statement_cache_mode=describe
        - name: CODEINTEL_PGDATABASE
          value: sg-codeintel
        - name: CODEINTEL_PGHOST
          value: sourcegraph-codeintel-pgbouncer.mycompany.com
        - name: CODEINTEL_PGPORT
          value: "5432"
        - name: CODEINTEL_PGSSLMODE
          value: disable
        - name: CODEINTEL_PGUSER
          value: sg
        - name: CODEINTEL_PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: sourcegraph-codeintel-pgbouncer-credentials
              key: password
        - name: CODEINSIGHTS_PGDATASOURCE
          value: postgres://$(CODEINTEL_PGUSER):$(CODEINTEL_PGPASSWORD)@$(CODEINTEL_PGHOST):$(CODEINTEL_PGPORT)/$(CODEINTEL_PGDATABASE)?statement_cache_mode=describe
```

----

## Postgres Permissions and Database Migrations

There is a tight coupling between the respective database service accounts for the Frontend DB, CodeIntel DB and Sourcegraph [database migrations](../../dev/background-information/sql/migrations.md). 

By default, the migrations that Sourcegraph runs expect `SUPERUSER` permissions. Sourcegraph migrations contain SQL that enable extensions and modify roles.

> NOTE: On AWS RDS, you will need to perform the operations below using the `rds_superuser` role because RDS does not grant SUPERUSER privileges to user database accounts.

This may not be acceptable in all environments. At minimum we expect that the `PGUSER` and `CODEINTEL_PGUSER` have the `ALL` permissions on `PGDATABASE` and `CODEINTEL_PGDATABASE` respectively.

`ALL` privileges on the [Database object](https://www.postgresql.org/docs/current/sql-grant.html) include:
 * `SELECT`
 * `INSERT`
 * `UPDATE`
 * `DELETE`
 * `TRUNCATE`
 * `REFERENCES`
 * `TRIGGER`
 * `CREATE`
 * `CONNECT`
 * `TEMPORARY`
 * `EXECUTE`
 * `USAGE`

<!--
When https://github.com/sourcegraph/deploy-sourcegraph/pull/4058 is merged, 
we will want to make sure these instructions are updated to reflect any additional
actions necessary to accomodate the migrator.
-->

----

### Using restricted permissions for pgsql (frontend DB)

> NOTE: For AWS RDS, refer to the note from this [section](#postgres-permissions-and-database-migrations).

Sourcegraph requires some initial setup that requires `SUPERUSER` permissions. A database administrator needs to perform the necessary actions on behalf of Sourcegraph migrations as `SUPERUSER`. 

Update these variables to match your deployment of the Sourcegraph _frontend_ database following [the guidance from the instructions section](#instructions). This database is called `pgsql` in the Docker Compose and Kubernetes deployments.

```bash
PGHOST=psql
PGUSER=sourcegraph
PGPASSWORD=secret
PGDATABASE=sourcegraph
```

The SQL script below is intended to be run from by a database administrator with `SUPERUSER` priviledges against the Frontend Database. It creates a database, user, and configures necesasry permissions for use by the Sourcegraph _frontend_ services.

```sql
# Create the application database
CREATE DATABASE $PGDATABASE;

# Create the application service user
CREATE USER $PGUSER with encrypted password '$PGPASSWORD';

# Give the application service permissions to the application database
GRANT ALL PRIVILEGES ON DATABASE $PGDATABASE to $PGUSER;

# Select the application database
\c $PGDATABASE;

# Install necessary extensions
CREATE extension citext; 
CREATE extension hstore; 
CREATE extension pg_stat_statements;
CREATE extension pg_trgm;
CREATE extension pgcrypto; 
CREATE extension intarray;
```

After the database is configured, Sourcegraph will attempt to run migrations. There are a few migrations that may fail as they attempt to run actions that require `SUPERUSER` permissions. 

These failures must be interpreted by the database administrator and resolved using guidance from [How to Troubleshoot a Dirty Database](https://docs.sourcegraph.com/admin/how-to/dirty_database). Generally-speaking this will involve looking up the migration source code and manually applying the necessary SQL code.

**Initial Schema Creation**

The first migration fails since it attempts to add `COMMENT`s to installed extensions. You may see the following error message:

```
failed to run migration for schema "frontend": failed upgrade migration 1528395834: ERROR: current transaction is aborted, commands ignored until end of transaction block (SQLSTATE 25P02)
```

In this case, locate the UP [migration 1528395834](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1528395834_squashed_migrations.up.sql) and apply all SQL after the final `COMMENT ON EXTENSION` command following the [dirty database procedure](https://docs.sourcegraph.com/admin/how-to/dirty_database).

**Dropping the `sg_service` role**

The `sg_service` database role is a legacy role that should be removed from all Sourcegraph installations at this time. Migration `remove_sg_service_role` attempts to enforce this with a `DROP ROLE` command. The `PGUSER` does not have permissions to perform this action, therefore the migration fails. You can safely skip this migration.

----

### Using restricted permissions for CodeIntel DB

> NOTE: For AWS RDS, refer to the note from this [section](#postgres-permissions-and-database-migrations).

CodeIntel requires some initial setup that requires `SUPERUSER` permissions. A database administrator needs to perform the necessary actions on behalf of Sourcegraph migrations as `SUPERUSER`.

```bash
CODEINTEL_PGHOST=psql2
CODEINTEL_PGUSER=sourcegraph
CODEINTEL_PGPASSWORD=secret
CODEINTEL_PGDATABASE=sourcegraph-codeintel
CODEINTEL_PGSSLMODE=require
```

The SQL script below is intended to be run from by a database administrator with `SUPERUSER` priviledges against the CodeIntel Database. It creates a database, user, and configures necesasry permissions for use by the Sourcegraph _frontend_ services.

```sql
# Create the CodeIntel database
CREATE DATABASE $CODEINTEL_PGDATABASE;

# Create the CodeIntel service user
CREATE USER $CODEINTEL_PGUSER with encrypted password '$CODEINTEL_PGPASSWORD';

# Give the CodeIntel  permissions to the application database
GRANT ALL PRIVILEGES ON DATABASE $CODEINTEL_PGDATABASE to $CODEINTEL_PGUSER;

# Select the application database
\c $CODEINTEL_PGDATABASE;

# Install necessary extensions
CREATE extension citext; 
CREATE extension hstore; 
CREATE extension pg_stat_statements;
CREATE extension pg_trgm;
CREATE extension pgcrypto; 
CREATE extension intarray;
```
After the database is configured, Sourcegraph will attempt to run migrations, this time using the CodeIntel DB. There are a few migrations that may fail as they attempt to run actions that require `SUPERUSER` permissions. 

These failures must be intepreted by the database administrator and resolved using guidance from [How to Troubleshoot a Dirty Database](https://docs.sourcegraph.com/admin/how-to/dirty_database). Generally-speaking this will involve looking up the migration source code and manually applying the necessary SQL code. The `codeintel_schema_migrations` table should be consulted for dirty migrations in this case.

**Initial CodeIntel schema creation**

Like the failure in the Sourcegraph DB (pgsql) migrations, the CodeIntel initial migration attempts to `COMMENT` on an extension. Resolve this in a similar manner by executing the SQL in the `1000000015_squashed_migrations.up` migration after the `COMMENT` SQL statement.

The following error is a nudge to check the `codeintel_schema_migrations` table in `$CODEINTEL_PGDATABASE`.

```
Failed to connect to codeintel database: 1 error occurred:
	* dirty database: schema is marked as dirty but no migrator instance appears to be running

The target schema is marked as dirty and no other migration operation is seen running on this schema. The last migration operation over this schema has failed (or, at least, the migrator instance issuing that migration has died). Please contact support@sourcegraph.com for further assistance.
```

[pgbouncer]: https://www.pgbouncer.org/
