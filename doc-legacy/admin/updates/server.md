# Single-image Sourcegraph Upgrade Notes

This page lists the changes that are relevant for upgrading Sourcegraph on a **single-node Sourcegraph instance**.

For upgrade procedures or general info about sourcegraph versioning see the links below:
- [Single Container Upgrade Procedures](../deploy/docker-single-container/index.md#upgrade)
- [General Upgrade Info](./index.md)
- [Product changelog](../../../CHANGELOG.md)

> ***Attention:** These notes may contain relevant information about the infrastructure update such as resource requirement changes or versions of depencies (Docker, externalized databases).*
>
> ***If the notes indicate a patch release exists, target the highest one.***

<!-- GENERATE UPGRADE GUIDE ON RELEASE (release tooling uses this to add entries) -->

## Unreleased

<!-- Add changes changes to this section before release. -->

## v5.2.4 ➔ v5.2.5

#### Notes:

## v5.2.3 ➔ v5.2.4

#### Notes:

## v5.2.2 ➔ v5.2.3

#### Notes:

## v5.2.1 ➔ v5.2.2

#### Notes:

## v5.2.0 ➔ v5.2.1

#### Notes:

## v5.1.9 ➔ v5.2.0

#### Notes:

## v5.1.8 ➔ v5.1.9

#### Notes:

## v5.1.7 ➔ v5.1.8

#### Notes:

## v5.1.6 ➔ v5.1.7

#### Notes:

## v5.1.5 ➔ v5.1.6

#### Notes:

## v5.1.4 ➔ v5.1.5

#### Notes:

## v5.1.3 ➔ v5.1.4

#### Notes:

## v5.1.2 ➔ v5.1.3

#### Notes:

## v5.1.1 ➔ v5.1.2

#### Notes:

## v5.1.0 ➔ v5.1.1

#### Notes:

## v5.0.6 ➔ v5.1.0

#### Notes:

#### Notes:

- The Docker Single Container Deployment image has switched to a Wolfi-based container image. Upon upgrading, Sourcegraph will need to re-index the entire database. All users **must** read through the [5.1 upgrade guide](../migration/5_1.md) _before_ upgrading.

## v5.0.5 ➔ v5.0.6

#### Notes:

## v5.0.4 ➔ v5.0.5

#### Notes:

## v5.0.3 ➔ v5.0.4

#### Notes:

## v5.0.2 ➔ v5.0.3

#### Notes:

## v5.0.1 ➔ v5.0.2

#### Notes:

## v5.0.0 ➔ v5.0.1

#### Notes:

## v4.5.1 ➔ v5.0.0

#### Notes:

## v4.5.0 ➔ v4.5.1

#### Notes:

## v4.4.2 ➔ v4.5.0

#### Notes:

- This release introduces a background job that will convert all LSIF data into SCIP. **This migration is irreversible** and a rollback from this version may result in loss of precise code intelligence data. Please see the [migration notes](../how-to/lsif_scip_migration.md) for more details.

## v4.4.1 ➔ v4.4.2

#### Notes:

## v4.3 ➔ v4.4.1

_No notes._

## v4.2 ➔ v4.3.1

_No notes._

## v4.1 ➔ v4.2.1

_No notes._

## v4.0 ➔ v4.1.3

_No notes._

## v3.43 ➔ v4.0

**Patch releases**:

- `v4.0.1`

## v3.42 ➔ v3.43

**Patch releases**:

- `v3.43.1`
- `v3.43.2`

## v3.41 ➔ v3.42

**Patch releases**:

- `v3.42.1`
- `v3.42.2`

## v3.40 ➔ v3.41

No upgrade notes.

## v3.39 ➔ v3.40

**Patch releases**:

- `v3.40.1`
- `v3.40.2`

## v3.38 ➔ v3.39

**Patched releases**:

- `v3.39.1`

## v3.37 ➔ v3.38

**Patch releases**:

- `v3.38.1`

## v3.36 ➔ v3.37

No upgrade notes.

## v3.35 ➔ v3.36

No upgrade notes.

## v3.34 ➔ v3.35

**Patch releases**:

- `v3.35.1`

**Notes**:

- There is a [known issue](../../code_insights/how-tos/Troubleshooting.md#oob-migration-has-made-progress-but-is-stuck-before-reaching-100) with the Code Insights out-of-band settings migration not reaching 100% complete when encountering deleted users or organizations.

## v3.33 ➔ v3.34

No upgrade notes.

## v3.32 ➔ v3.33

No upgrade notes.

## v3.31 ➔ v3.32

No upgrade notes.

## v3.30 ➔ v3.31

> WARNING: **This upgrade must originate from `v3.30.3`.**

**Notes**:

- The **built-in** main Postgres (`pgsql`) and codeintel (`codeintel-db`) databases have switched to an alpine-based Docker image. Upon upgrading, Sourcegraph will need to re-index the entire database. All users that use our bundled (built-in) database instances **must** read through the [3.31 upgrade guide](../migration/3_31.md) _before_ upgrading.

## v3.29 ➔ v3.30

> WARNING: **If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2** please follow [this migration guide](../migration/3_30.md).

**Patch releases**:

- `v3.30.1`
- `v3.30.2`
- `v3.30.3`

## v3.28 ➔ v3.29

No upgrade notes.

## v3.27 ➔ v3.28

No upgrade notes.

## v3.26 ➔ v3.27

> WARNING: Sourcegraph 3.27 now requires **Postgres 12+**.

**Notes**:

- If you are using an external database, [upgrade your database](https://docs.sourcegraph.com/admin/postgres#upgrading-external-postgresql-instances) to Postgres 12 or above prior to upgrading Sourcegraph. If you are using the embedded database, [prepare your data for migration](https://docs.sourcegraph.com/admin/postgres#upgrading-single-node-docker-deployments) prior to upgrading Sourcegraph.

## v3.25 ➔ v3.26

No upgrade notes.

## v3.24 ➔ v3.25

No upgrade notes.

**Notes**:

- Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and and connecting to it using SSL/TLS check whether the certificate is up to date.
  - AWS RDS customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) for steps to rotate your certificate.

## v3.23 ➔ v3.24

No upgrade notes.
## v3.22 ➔ v3.23

No upgrade notes.
## v3.21 ➔ v3.22

> WARNING: **This upgrade must originate from `v3.20.1`.**

No upgrade notes.
## v3.20 ➔ v3.21

> WARNING: **This upgrade must originate from `v3.17.2`** due to a [patched](https://github.com/sourcegraph/sourcegraph/pull/11633) [bug](https://github.com/sourcegraph/sourcegraph/issues/11618) in our release.

**Notes**:

- This release introduces a second database instance, `codeintel-db`. If you have configured Sourcegraph with an external database, then update the `CODEINTEL_PG*` environment variables to point to a new external database as described in the [external database documentation](../external_services/postgres.md). Again, these must not point to the same database or the Sourcegraph instance will refuse to start.
- **Turn off database secrets encryption**. In Sourcegraph version 3.20, we would automatically generate a secret key file (`/var/lib/sourcegraph/token`) inside the container for encrypting secrets stored in the database. However, it is not yet ready for general use and format of the secret key file might change. Therefore, it is best to delete the secret key file (`/var/lib/sourcegraph/token`) and turn off the database secrets encryption.

## v3.19 ➔ v3.20

No upgrade notes.

## v3.18 ➔ v3.19

No upgrade notes.

## v3.17 ➔ v3.18

No upgrade notes.

## v3.16 ➔ v3.17

**Patch releases**:

- `v3.17.2`

## v3.15 ➔ v3.16

No upgrade notes.
## v3.14 ➔ v3.15

No upgrade notes.
