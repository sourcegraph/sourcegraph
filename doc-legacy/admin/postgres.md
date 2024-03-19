# PostgreSQL

Sourcegraph uses several PostgreSQL databases to support various functionality. These databases are:

- `pgsql` or `primary`: responsible for user data and account information
- `codeintel-db`: provides support for lsif data and part of the code-intelligence

## Version requirements

We support any version **starting from 12**.

> NOTE: ⚠️ Version **3.26** required only Postgres 9.6. You should check the [upgrade docs for your deployment type](../admin/updates/index.md) as well as the [database upgrade docs](#upgrading-postgresql) below prior to upgrading to 3.27.

## Role requirements

The user provided to Sourcegraph must have full access to the `sg` database and be able to create the following
extensions:

```
citext
hstore
intarray
pg_stat_statements
pg_trgm
pgcrypto
```

## [Configuring PostgreSQL](./config/postgres-conf.md)

## Upgrading PostgreSQL

Sourcegraph uses PostgreSQL as its main database and this documentation describes how to upgrade PostgreSQL between major versions.

> WARNING: Upgrading the PostgreSQL database requires stopping your Sourcegraph deployment which will result in **downtime**.

<span class="virtual-br"></span>

> NOTE: If you running PostgreSQL externally, see [Upgrading external PostgreSQL instances](postgres.md#upgrading-external-postgresql-instances)

### Upgrading external PostgreSQL instances

When running an [external PostgreSQL instance](external_services/postgres.md), please do the following:

1. Back up the Postgres DB so that you can restore to the old version should anything go wrong.
2. Turn off Sourcegraph entirely (bring down all containers and pods so they cannot talk to Postgres.)
3. Upgrade Postgres to the latest version following your provider's instruction or your preferred method:
  - [AWS RDS](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_UpgradeDBInstance.PostgreSQL.html)
  - [AWS Aurora](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/USER_UpgradeDBInstance.Upgrading.html)
  - [GCP CloudSQL](https://cloud.google.com/sql/docs/postgres/db-versions)
  - [Azure DB](https://docs.microsoft.com/en-us/azure/postgresql/concepts-supported-versions#managing-updates-and-upgrades)
  - [Heroku](https://devcenter.heroku.com/articles/upgrading-heroku-postgres-databases)
  - [EnterpriseDB](https://www.enterprisedb.com/docs/en/9.6/pg/upgrading.html)
  - [Citus](http://docs.citusdata.com/en/v8.1/admin_guide/upgrading_citus.html)
  - [Aiven PostgreSQL](https://help.aiven.io/postgresql/operations/how-to-perform-a-postgresql-in-place-major-version-upgrade)
  - [Your own PostgreSQL](https://www.postgresql.org/docs/11/pgupgrade.html)
4. Turn Sourcegraph back on connecting to the now-upgraded database.

> WARNING: **Do not allow Sourcegraph to run/connect to the new Postgres database until it has been fully populated with your data.** Doing so could result in Sourcegraph trying to create e.g. a new DB schema and partially migrating. If this happens to you, restore from the backup you previously took or contact us (support@sourcegraph.com)

### Upgrading internal PostgreSQL instances

Many of Sourcegraph's [deployment methods](deploy/index.md) come with internal PostgreSQL instances that can be deployed as part of a Sourcegraph installation. This section outlines how to upgrade these databases.

#### Sourcegraph with Docker Compose

Generally, no additional steps are required to upgrade the databases shipped alongside [Sourcegraph with Docker Compose](deploy/docker-compose/index.md).

#### Sourcegraph with Kubernetes

**The upgrade process is different for [Sourcegraph Kubernetes deployments](./deploy/kubernetes/index.md)** because [by default](https://github.com/sourcegraph/sourcegraph/blob/main/docker-images/postgres-12/build.sh#L10), it uses `sourcegraph/postgres-12` which can be [customized with environment variables](https://github.com/sourcegraph/deploy-sourcegraph/blob/7edcadb/docs/configure.md#external-postgres).

If you have changed `PGUSER`, `PGDATABASE` or `PGDATA`, then the `PG*OLD` and `PG*NEW` environment variables are
required. Below are the defaults and documentation on what each variable is used for:

- `POSTGRES_PASSWORD=''`: Password of `PGUSERNEW` if it is newly created (i.e when `PGUSERNEW` didn't exist in the old
  database).
- `PGUSEROLD=sg`: A user that exists in the old database that can be used to authenticate intermediate upgrade
  operations.
- `PGUSERNEW=sg`: A user that must exist in the new database after the upgrade is done (i.e. it'll be created if it
  didn't exist already).
- `PGDATABASEOLD=sg`: A database that exists in the old database that can be used to authenticate intermediate upgrade
  operations. (e.g `psql -d`)
- `PGDATABASENEW=sg`: A database that must exist in the new database after the upgrade is done (i.e. it'll be created if
  it didn't exist already).
- `PGDATAOLD=/data/pgdata-11`: The data directory containing the files of the old PostgreSQL database to be upgraded.
- `PGDATANEW=/data/pgdata-12`: The data directory containing the upgraded PostgreSQL data files, used by the new version
  of PostgreSQL.

Additionally, the upgrade process assumes it can write to the parent directory of `PGDATAOLD`.

##### Requirements for upgrading from Postgres 11 to 12 for Kubernetes

**This guide is for the Sourcegraph 3.27 release.** The migration to Postgres 12 using the Sourcegraph provided Postgres images is largely automated.
However, you should take some steps before the migration. For the `3.27` release, these apply to the `pgsql` & `codeintel-db` deployments.

1. Inform your users prior to starting the upgrade that Sourcegraph will take downtime. This downtime scales with the
   size and resources allocated to the Postgres database instances.
1. Take snapshots of all Sourcegraph Postgres databases. You can find the backing disk in your cloud by describing the
   relevant `PersistentVolumes`.
1. Ensure that the size of the disks backing the databases are under 50% utilization. (You will need *up-to* 2x the size
   of your current database to do the migration). In most cases Kubernetes can expand the size of the volume for you,
   see the relevant kubernetes
   docs [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims).

   You can use `kubectl exec -it $DB_POD_NAME -- df -h`. By default, the database will be mounted under `/data/`.

#### Single-container Sourcegraph

When running a new version of Sourcegraph, [single-container Sourcegraph with Docker instances](./deploy/kubernetes/index.md) will check if the PostgreSQL data needs upgrading upon initialization.

See the following steps to upgrade between major postgresql versions:

**1.** Stop the `sourcegraph/server` container.

**2.** Save this script and give it executable permissions (`chmod + x`).

> NOTE: The script presumes your data is being upgraded from `11` to `12` and your Sourcegraph directory is at `~/.sourcegraph/`. Change the values in the code below if that's not the case.

```bash
#!/usr/bin/env bash

set -xeuo pipefail

export OLD=${OLD:-"11"}
export NEW=${NEW:-"12"}
export SRC_DIR=${SRC_DIR:-"$HOME/.sourcegraph"}

docker run \
  -w /tmp/upgrade \
  -v "$SRC_DIR/data/postgres-$NEW-upgrade:/tmp/upgrade" \
  -v "$SRC_DIR/data/postgresql:/var/lib/postgresql/$OLD/data" \
  -v "$SRC_DIR/data/postgresql-$NEW:/var/lib/postgresql/$NEW/data" \
  "tianon/postgres-upgrade:$OLD-to-$NEW"

mv "$SRC_DIR/data/"{postgresql,postgresql-$OLD}
mv "$SRC_DIR/data/"{postgresql-$NEW,postgresql}

curl -fsSL -o "$SRC_DIR/data/postgres-$NEW-upgrade/optimize.sh" https://raw.githubusercontent.com/sourcegraph/sourcegraph/master/cmd/server/rootfs/postgres-optimize.sh

docker run \
  --entrypoint "/bin/bash" \
  -w /tmp/upgrade \
  -v "$SRC_DIR/data/postgres-$NEW-upgrade:/tmp/upgrade" \
  -v "$SRC_DIR/data/postgresql:/var/lib/postgresql/data" \
  "postgres:$NEW" \
  -c 'chown -R postgres $PGDATA . && gosu postgres bash ./optimize.sh $PGDATA'
```

**3.** Execute the script.

**4.** Start the `sourcegraph/server` container.
