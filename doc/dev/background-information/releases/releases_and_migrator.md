
# Current v5.2 Upgrade/Migrator Design<a id="current-v52-upgrademigrator-design"></a>

_This document is attached to_ [_RFC 850 WIP: Improving The Upgrade Experience_](https://docs.google.com/document/d/1ne8ai60iQnZfaYuB7QLDMIWgU5188Vn4_HBeUQ3GASY/edit) _as a summary of the current design of our upgrade and database schema management design. It aims to provide general information about the relationship of our migrator service to our deployment types and migrators dependencies during the release process._


## Migrator Functionality<a id="migrator-functionality"></a>

Overall the migrator code and its relevant artifacts have two core purposes –

1. Migrator constructs migration plans given version ranges and a table of migrations which have been successfully applied (each schema has a table to track applied migrations within that schema). This logic is supported by a variety of files generated during releases and depends on the parent/child metadata of migrations generated via the sg migration tool.

2. Migrator manages out of band migrations. These are data migrations that must be run within specific schema boundaries. Running OOB migrations at/after the deprecated version is unsupported. Migrator ensures that the necessary OOB migrations are run at [stopping points](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/oobmigration/upgrade.go?L32-42) in a multiversion upgrade – learn more [here](https://about.sourcegraph.com/blog/introducing-migrator-service).


### CLI design<a id="cli-design"></a>

Migrator is designed as a CLI tool – taking various commands to alter the state of the database and apply necessary schema changes. This design was initially implemented as a tool for TS team members to assist in multiversion upgrades and because it could easily be included over multiple deployment methods as a containerized process run separately from the frontend during startup ([which originally caused issues](https://about.sourcegraph.com/blog/introducing-migrator-service)). This replaced earlier `go/migrate` based strategies which ran in the frontend on startup. While the migrator can operate as a CLI tool, it’s containerized as if it was another application which allows it to be run as an initContainer (in Kubernetes deployments) or as a dependent service (for Docker Compose deployments) to ensure that the necessary migrations are applied before startup of the application proper.


### Important Commands<a id="important-commands"></a>

The most important migrator commands are `up` and `upgrade/downgrade`:

- `Up` : This ensures that, for a given version of the migrator, every migration defined at that build has been successfully applied in the connected database, this is specifically important to ensure _patch version migrations are run_. **_This must be run with the target version migrator image/build_**_._ If migrator and frontend pods are deployed in version lockstep, this ensures that ALL migrations required by the frontend will be successfully applied prior to boot. This is a syntactic sugar over a more internal `upto` command. 

- `Upgrade`: This runs all migrations defined between two minor versions of Sourcegraph and requires that other services which may access the database are brought down. Before running, the database is checked for and schema drifts in order to prevent a failure while attempting a migration.

In general, the `up` command can be thought of as a **standard** upgrade (rolling upgrades with no downtime) while the `upgrade` command is what enables **multiversion** upgrades. In part, `up` was designed to maintain our previous upgrade policy and is thus run as an initContainer (or initContainer-like mechanism) of the frontend, i.e. between two versions of Sourcegraph, the antecedent sourcegraph services will continue to work after the consequent version’s migrations have been applied. ![](https://lh7-us.googleusercontent.com/1kOLQBZ5kjF0Ll_3AYl4L2kDiyOJh3Fedy5tupap5w3abfGnUDGhz205T0oy8tlG4d7byqIk0Qmi4Kx-FiikyXwi6ZRnZpkh2IfU8x0EeqFLFWC0E-VQThGYCIeO4Qa0HnBSg9CcpxN4ysegeeD-d_0)


## Current Startup Dependency<a id="current-startup-dependency"></a>

All of our deployment types currently utilize migrator during startup. The frontend service won’t start until migrator has been run with the default `up` command. The frontend service will also validate the expected schema (and OOB migration progress), and die on startup if this validation pass fails. This ensures that the expected migrations for the version in question have been run.

In docker-compose (_see diagram)_, this is accomplished via a chain of `depends_on` clauses in the `docker-compose.yaml `([link](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-docker/-/blob/docker-compose/docker-compose.yaml?L217-223)).

**For our k8s based deployments (including the AMIs) migrator is run as an** `initContainer` **within the frontend utilizing the** `up` **command on the given pods startup.**

- [Helm Ex](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-helm/-/blob/charts/sourcegraph/templates/frontend/sourcegraph-frontend.Deployment.yaml?L49-76)

- [Kustomize Ex](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/blob/base/sourcegraph/frontend/sourcegraph-frontend.Deployment.yaml?L30-48)


## Auto-upgrade<a id="auto-upgrade"></a>

Migrator has been incrementally improved over the last year in an attempt to get closer and closer to auto-upgrades. After migrator v5.0.0 logic was added to the database to attempt an automatic upgrade to the latest version of Sourcegraph on the startup of the frontend.

For more information about how this works see the [docs](https://docs.sourcegraph.com/admin/updates/automatic#automatic-multi-version-upgrades). Some notable points:

- The upgrade operations in this case are [triggered](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cf85b5803d32a91425f243930a4f50364625bcd2/-/blob/cmd/frontend/internal/cli/serve_cmd.go?L94-96) and [run](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cf85b5803d32a91425f243930a4f50364625bcd2/-/blob/cmd/frontend/internal/cli/autoupgrade.go?L37-145) by the `frontend` container.

- Migrator [looks for](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cf85b5803d32a91425f243930a4f50364625bcd2/-/blob/internal/database/migration/cliutil/up.go?L125-128) the existence of the env var `SRC_AUTOUPGRADE=true` on services `sourcegraph-frontend`, `sourcegraph-frontend-internal`, and `migrator`. Otherwise it [looks in the frontend db](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cf85b5803d32a91425f243930a4f50364625bcd2/-/blob/internal/database/migration/cliutil/up.go?L120-123) for the value of the `autoupgrade` column. These checks are performed with either the `up` or `upgrade` commands defined on the migrator.

- The internal connections package to the DB now uses a special [sentinel value](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/dbconn/connect.go?L31-37) to make connection attempts sleep if migrations are in progress.

- A limited frontend is [served](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@cf85b5803d32a91425f243930a4f50364625bcd2/-/blob/cmd/frontend/internal/cli/autoupgrade.go?L78-88) by the frontend during an autoupgrade, displaying progress of the upgrade and any drift encountered. 


## [Migrator Release Artifacts](#migrator-release-artifacts)<a id="migrator-release-artifacts"></a>

During the release of migrator we construct and build some artifacts used by migrator to support its operations. Different artifacts must be generated depending on the release type –

- **Major**

  - [lastMinorVersionInMajorRelease](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/oobmigration/version.go?L84-87): Used to evaluate what oobmigrations must run, must be updated every major release. This essentially tells us when a minor version becomes a major version. _It may be useful elsewhere at some point._

- **Minor**

  - [maxVersionString](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@5851edf/-/blob/internal/database/migration/shared/data/cmd/generator/consts.go?L12I): Defined in `consts.go` this string is used to tell migrator the latest **minor** version targetable for MVU and oobmigrations. [If not updated](https://github.com/sourcegraph/sourcegraph/issues/55048) multiversion upgrades cannot target the latest release. _Note this is used to determine how many versions should be included in the_ `stitched-migration-graph.json` _file._

  - [_Stitched-migration.json_](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v5.2.0/-/blob/internal/database/migration/shared/data/stitched-migration-graph.json): Used by multiversion upgrades to unsquash migrations. Generated during release [_here_](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/release/src/release.ts?L1101-1110). Learn more below.

- **Patch**

* [Git\_versions](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/migrator/generate.sh?L69-79): Defined in `generate.sh` this string array contains versions of Sourcegraph whose schemas should be embedded in migrator during a migrator build to enable drift detection without having to pull them directly from GitHub or, for older versions, from a pre-prepared GCS bucket (this is necessary in air gapped environments). This should be kept up to date with `maxVersionString`. [Learn more](https://github.com/sourcegraph/sourcegraph/issues/49813).

* [Squashed.sql](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v5.2.0/-/blob/migrations/frontend/squashed.sql): for each database we [generate](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/gen.sh?L18-20) a new `squashed.sql` file. It is [used](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v5.2.0/-/blob/internal/database/migration/drift/util_search.go?L12-31) to help suggest fixes for certain types of database drift. For example for a missing database column [this search](https://sourcegraph.com/search?patternType=regexp\&q=repo%3A%5Egithub%5C.com%2Fsourcegraph%2Fsourcegraph%24%40v5.0.6+file%3A%5Emigrations%2Ffrontend%2Fsquashed%5C.sql%24+%28%5E%7C%5Cb%29CREATE%5CsTABLE%5Csexternal_service_sync_jobs%28%24%7C%5Cb%29+OR+%28%5E%7C%5Cb%29ALTER%5CsTABLE%5CsONLY%5Csexternal_service_sync_jobs%28%24%7C%5Cb%29\&groupBy=path) is used to suggest a definition.

* [Schema descriptions](https://raw.githubusercontent.com/sourcegraph/sourcegraph/v5.2.0/internal/database/schema.json): schema descriptions are [embedded](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v5.2.0/-/blob/cmd/migrator/generate.sh?L64-66) in migrator on each new build as a reference for the expected schema state during drift checking.


### Stitched Migration JSON and Squashing Migrations<a id="stitched-migration-json-and-squashing-migrations"></a>

### ![](https://lh7-us.googleusercontent.com/NxIb4K9GdvnujQak49q5phSihj24aYHRPyCHvNlXT_TmaG2HBUjRhiXW7v_z9pno1Q6ArmSKtPFUjYvtNjxdNmust9qT6Tb_IK1GMhSu8P4_TWpHmDldJwrJID_MYzfXN1e9pVYn-QuLMmW3Nxdgn8g)

_^^ Generated with_ `sg migrations visualize –db frontend` _on v5.2.0_


#### Squashing Migrations<a id="squashing-migrations"></a>

[Periodically we squash](https://github.com/sourcegraph/sourcegraph/pull/41819) all migrations from two versions behind the current version into a single migration using `sg migration squash.`This reduces the time required to initialize a new database. This means that the migrator image is built with a set of definitions embedded that doesn’t reflect the definition set in older versions. For multiversion upgrades this presents a problem. To get around this, on minor releases we generate a `stitched-migration-graph.json` file. Reference links: [Bazel](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/release/src/util.ts?L293-327), [Embed](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/migration/shared/embed.go?L15-22), [Generator](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/migration/shared/data/cmd/generator/main.go) 


#### Stitched Migration Graph `stitched-migrations-graph.json` [stitches](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v5.2.0/-/blob/internal/database/migration/stitch/stitch.go) (can think of this as unsquashing) historic migrations using git magic, enabling the migrator to have a reference of older migrations. This serves a few purposes:<a id="stitched-migration-graphstitched-migrations-graphjson-stitches-can-think-of-this-as-unsquashing-historic-migrations-using-git-magic-enabling-the-migrator-to-have-a-reference-of-older-migrations-this-serves-a-few-purposes"></a>

1. When jumping across multiple versions we do not have access to a full record of migration definitions on migrator disk because some migrations will likely have been squashed. Therefore we need a way to ensure we don’t miss migrations on a squash boundary. _Note we can’t just re-apply the root migration after a squash because some schema state that's already represented in the root migration. This means the squashed root migration isn’t always idempotent._ 

2. During a multiversion upgrade migrator must schedule out of band migrations to be started at some version and completed before upgrading past some later version. Migrator needs access to the unsquashed migration definitions to know which migrations must have run at the time the oob migration is triggered.

In standard/`up` upgrades `stitched-migrations.json` isn’t necessary. This is because `up` determines migrations to run by [comparing migrations](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/database/migration/definition/definition.go?L214-244) listed as already run in the relevant db’s` migration_logs` table directly to those migration definitions [embedded](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/migrations/embed.go) in the migrator disk at build time for the current version, and running any which haven’t been run. We never squash away the previous minor version of Sourcegraph, in this way we can guarantee the `migration_logs`  table migrations always has migrations in common with the migration definitions on disk.


### Database Drifts<a id="database-drifts"></a>

Database drift is any difference between the expected state of the database at a given version and the actual state of the database. How does it come to be? We’ve observed drift in customer databases for a number of reasons, both due to admin tampering and problems in our own release process:

- **A backup/restore process that didn’t include all objects**: This notably happened to Reddit causing their database to have no indexes, primary keys, or unique constraints defined. 

- **Modifying the database explicitly**: These are untracked manual changes to the database schema, observed occasionally in the wild, and in our dotcom deployment. 

- **Migration failures:** that occur during multi-version upgrade or the `up` command will cause the set of _successfully applied migrations_ to be between two versions, where drift is well-defined.

- **Site Admins Error:** Errors in git-ops like deploying to production on the wrong version of Sourcegraph manifests have introduced drift. Another source is the incorrect procedure in downgrading.

- **Historic Bugs**: We, at one point, [too eagerly backfilled records that we should’ve instead applied](https://github.com/sourcegraph/sourcegraph/pull/55650). This bug was the result of changes being backported to the metadata definition of a migrations parent migrations, violating assumptions made during the generation of the `stitched-migration-graph.json`.

Database drift existing at the time of a migration can cause migrations to fail when they try to reference some table property that is not in the expected state. Not to mention the application may not behave as expected if drift is present. Migrator includes a `drift` command intended to help admins and CS team members to diagnose and resolve drift in customer instances. Multiversion upgrades in particular check for drift before starting unless the `--skip-drift-check` argument is supplied.
