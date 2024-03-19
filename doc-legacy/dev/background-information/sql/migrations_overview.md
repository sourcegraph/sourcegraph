# Migrations overview

There are two types of migrations supported in a Sourcegraph application: in-band migrations and out-of-the-band migrations.

## In-band migrations

The process of in-band migrations evolved from the original [golang-migrate/migrate](https://github.com/golang-migrate/migrate), which uses version-based sequential migrations.

There are three logical databases for a Sourcergaph application, "frontend" (main database), "codeintel" (only for code intelligence data), and "codeinsights" (only for code insights data). They are logical databases because the Sourcegraph application may be configured to use any combination of DSNs for them and does not require them to be physically isolated. For example, one may choose to:

* Configure all three databases in a single database within a single PostgreSQL instance for local development, using prefixes for tables to identify target databases (e.g. "codeintel_lsif").
* Configure all three databases in a single PostgreSQL instance for the simplicity of a normal Sourcegraph deployment.
* Configure only the "codeintel" database to live on separate PostgreSQL instances, but keep "frontend" and "codeinsights" in the same PostgreSQL instance. Due to their intensive usage of code intelligence features.
* Configure all three databases in separate PostgreSQL instances for large deployments (e.g. Sourcegraph.com).

All databases are assumed to be created under the "public" schema regardless of how they’re being configured.

Migrations are executed alongside but not coupled with application upgrades. In a continuous deployment setup, the very first thing that happens in an upgrade process is to run schema change migrations, and only proceed application upgrades if all of the migrations are succeeded. Previously, migrations are executed by the application itself, and had the problem of when the migrations failed, the application would then refuse to start, thus very vulnerable to have production incidents.

Both Sourcegraph Cloud and on-prem instances share the same schema change process through the "[migrator](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/migrator)" service, regardless of the deployment models (Kubernetes, Docker Compose, Single-Docker container).

All migrations of three databases are stored in the top-level directory ["migrations"](https://github.com/sourcegraph/sourcegraph/tree/main/migrations), and each database has its own subdirectory for migrations targeted at them respectively. New migrations should be created by the internal dev tool [sg](https://docs.sourcegraph.com/dev/background-information/sg), by giving it the target database and a short description as the migration name:

```
$ sg migration add -db=frontend int64-ids-user-pending-permissions
Migration files created
 Up query file: ~/migrations/frontend/1648524019_int64-ids-user-pending-permissions/up.sql
 Down query file: ~/migrations/frontend/1648524019_int64-ids-user-pending-permissions/down.sql
 Metadata file: ~/migrations/frontend/1648524019_int64-ids-user-pending-permissions/metadata.yaml
```

Each migration is another subdirectory containing three files:

* `up.sql` - contains SQL statements to "upgrade" the database schema ([example](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648524019_int64-ids-user-pending-permissions/up.sql)).
* `down.sql` - contains SQL statements to "downgrade" the database schema (i.e. revert schema changes) whenever possible ([example](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648524019_int64-ids-user-pending-permissions/down.sql)).
* `metadata.yaml` - contains the metadata of the migration with following fields ([example](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648524019_int64-ids-user-pending-permissions/metadata.yaml)):
    * `name` - the name of the migrations, which is given to the `sg migration add` command.
    * `parents` - the parent migrations that the current one is created from (i.e. should only execute the current migration if all of the parents are successfully executed). This is to solve the problem of multiple engineers adding migrations at the same time and having conflicting migration versions, by moving the dependency resolution from the implicit migration version to be the explicit reference. Therefore, having multiple migrations pointing to the same parent is expected by design, here a simplified scenario:
        1. Two engineers created new migrations independently branch off [A](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648195639_squashed_migrations_unprivileged/metadata.yaml).
        2. Now we have migrations [B](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648524019_int64-ids-user-pending-permissions/metadata.yaml) and [C](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1648628900_rename_localclone_worker_table/metadata.yaml) both have A as the parent.
        3. Another migration [D](https://github.com/sourcegraph/sourcegraph/blob/main/migrations/frontend/1649159359_batch_spec_resolution_user_id/metadata.yaml) is created, and the `sg migration add` command recognizes its parent should be B and C.
    * `privileged` - indicates whether the migration must be run by a privileged user (i.e. super user). As of now, only [squash migrations are marked with a `true` value](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+privileged:+true+f:metadata.yaml&patternType=standard&sm=0&groupBy=path).
    * `nonIdempotent` - indicates whether the migration is not possible to run repeatedly and create incompatible side effects. As of now, only [squash migrations are marked with a `true` value](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+nonIdempotent:+true&patternType=standard&sm=0&groupBy=path).
    * `createIndexConcurrently` - indicates whether the migration uses the semantic of `CREATE INDEX CONCURRENTLY` with [caveats](https://github.com/sourcegraph/sourcegraph/blob/daf10fe1d0f921013fe3f14f6c4aaee754fc75cf/dev/sg/internal/migration/add.go#L23-L26).

The execution of every migration is always wrapped in a single PostgreSQL transaction. If something needs to happen across the boundary of a PostgreSQL transaction, then make them two migrations.

Due to the nature of in-band migrations being executed in sequence, it could take quite a bit of time to run through all of them in a fresh installation. Therefore, squashing migrations is used as a technique to have a cumulated version of the database scheme up to the point of the oldest supported version of Sourcegraph to be upgraded from. For example, if the oldest supported version is 3.20, then all migrations created prior to 3.20 are squashed and users must upgrade to 3.20 first (to perform all necessary migrations) before jump start to the latest version (e.g. 3.11 -> 3.20 -> 4.2).

### Privileged vs. unprivileged

In the majority of the Sourcegraph installations, a privileged (super) user of the PostgreSQL instance is given. Unfortunately, that is not always the case.

While an unprivileged user in a correct database setup is able to perform all regular migrations, operations like creating PostgreSQL extensions can only be performed by a privileged user. As of now, only [squash migrations require a privileged user to run](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+f:migrations/+privileged:+true+lang:miniyaml&patternType=standard&sm=0&groupBy=path).

It is impossible to perform migrations that require a privileged user automatically if the user that is given to the Sourcegraph application is an unprivileged user. Therefore, [manual intervention is always required](https://docs.sourcegraph.com/admin/how-to/privileged_migrations) by system admins in such scenarios.

### `sg migration`

The `sg migration` subcommand is primarily designed to work with in-band migrations, it provides [a variety of subcommands and their options](https://docs.sourcegraph.com/dev/background-information/sg/reference#sg-migration) that work across all deployment models.

One notable feature it provides is the [schema drift detection](https://docs.sourcegraph.com/dev/background-information/sg/reference#sg-migration-drift), which in most cases, is able to generate the SQL statements that can help system admins to fix the drift.

## Out-of-band migrations

While in-band migrations are working great for both DDL and DML, some data-oriented migrations are just too complex to be able to fit into SQL statements, or even impossible to be expressed with SQL that have to involve code logic from the application. This is why we also support [out-of-band migrations](https://docs.sourcegraph.com/dev/background-information/oobmigrations) to complement those needs.

It is notable that out-of-band migrations should only be used for data-oriented migrations (DML), any type of schema changes (DDL) should be done by in-band migrations.

## Sourcegraph Cloud

Sourcegraph Cloud is a multi-single tenant architecture, aka. managed instances SaaS offering. The schema change process on Sourcegraph Cloud is no different from other deployment models. However, it does have challenges and constraints on the way that the infrastructure is being set up, due to security and compliance.

### IaC model and GitOps

Sourcegraph uses Terraform as the infrastructure automation tool to practice Infrastructure as Code (IaC), all customers' infrastructure definitions are stored in a single Git repository. Each customer maintains its own Terraform module, Google Cloud Platform (GCP) project, Cloud SQL PostgreSQL instance and has its own directory in the Git repository.

Terraform Cloud is being used as Terraform state backends for all managed instances on Sourcegraph Cloud.

Changes are applied through Terraform Cloud after being merged into the `main` branch:

1. Make changes to the Terraform module
2. Do `terraform plan` 
3. Submit pull request for review
4. Get approval and merge
5. Terraform Cloud does the `terraform apply` automatically

### Cloud SQL connectivity

Managed instance use the Kubernetes deployment model and services run on Google Kubernetes Engine (GKE), the Cloud SQL proxy is used to allow service pods to connect to it.

If a third-party service needs to connect to the Cloud SQL instance without Cloud SQL proxy, the source IP must be explicitly allowlisted, and [SSL must be turned on](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/PostgreSQL.Concepts.General.SSL.html) for all incoming connections.

For every Cloud SQL instance in a customer GCP project (e.g. `src-747bc765eb31a4873e4b`):

* Has a main database whose name follows the format of `src-{twenty random characters}`, e.g. `src-875fd645c21bebf8`.
* Has a dedicated service account as the primary (super) user used by the Sourcegraph application through Cloud SQL proxy, whose name following the format of `{Cloud SQL instance ID}@{project ID}.iam`, e.g. `src-875fd645c21bebf8@src-747bc765eb31a4873e4b.iam`.

### Upgrade process

The [Sourcegraph Cloud release upgrade process](https://handbook.sourcegraph.com/departments/cloud/technical-docs/#release-process) is documented in the handbook, in a nutshell, staged rollout strategy is used:

1. Upgrade internal instances
2. Upgrade trial instances
3. Upgrade customer instances

There are also few unconventional managed instances where pinned and hotpatch versions are deployed for various reasons.

When performing an upgrade, the short-lived "migrator" service is run first to ensure database migrations are successful, only then actual application services are upgraded.

### Ad-hoc operations

As of now, there is no control panel for ad-hoc operations made to the Cloud SQL instances, special tooling is provided to establish a Cloud SQL proxy connection tunnel locally, and devs can connect to them as if they were local PostgreSQL instances.

No peer review nor audit log is available. The only available information is who connected to which Cloud SQL instance at when due to fact all connections are made through GCP IAP tunneling.

### Monitoring and alerting

Native GCP features are used for essentially monitoring and alerting to Slack for CPU, memory and health checks.

## Related readings

* [Sourcegraph Blog - Broken database migrations: How we finally fixed an embarrassing problem](https://sourcegraph.com/blog/introducing-migrator-service)
* [RFC 469: Decouple migrations from code deployments](https://docs.google.com/document/d/1_wqvNbEGhMzu45CHZNrLQYys7gl0AtVpLluGp4cbQKk/edit?usp=sharing)
* [RFC 697: Multiple version upgrades](https://docs.google.com/document/d/1eNiwzWZmfpvzFhs9IDcM7Sbchv_Hm8Uj-6bI-3TDlvg/edit?usp=sharing)
* [Database Schema Migrations: A Few Lessons Learned](https://danwatt.org/2021/03/database-schema-migrations-a-few-lessons-learned/)
