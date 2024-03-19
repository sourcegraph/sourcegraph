# Updating sourcegraph

This page is intended as an entry point into Sourcegraph versioning, upgrades, and the `migrator` service which manages our database schemas. Here we'll cover general concepts and direct you toward relevant operations pages.

**If you are already familiar with Sourcegraph upgrades [skip to instance specific procedures](#instance-specific-procedures).**

> *Note: For product update notes, please refer to the [changelog](../../CHANGELOG.md).*

## General concepts

### Upgrading

A new Sourcegraph release consists of updated images that incorporate code changes from our [main repository](https://github.com/sourcegraph/sourcegraph). It also includes modifications to deployment manifests in our supporting deployment repositories, such as our [k8s helm repo](https://github.com/sourcegraph/deploy-sourcegraph-helm). 

When upgrading Sourcegraph, it is necessary to update and apply the deployment manifests. However, in addition to the manifest updates, it is crucial to ensure that all underlying database schemas of Sourcegraph are also updated.

### Release Schedule and versioning
Sourcegraph releases use semantic versioning, for example: `v5.0.3`
| Description   | Version |
|---------------|---------|
| major         | `5`     |
| minor         | `0`     |
| patch         | `3`     |

To learn more about our release schedule see our [handbook](https://handbook.sourcegraph.com/departments/engineering/dev/process/releases/#sourcegraph-releases). In general a new minor version of Sourcegraph is released every month, accompanied by weekly patch releases. Major releases are less frequent and represent significant changes in Sourcegraph.You can also check the [Sourcegraph blog](https://sourcegraph.com/blog) for more information about the latest release.

### Upgrade types

Sourcegraph has two upgrade types. **Standard** upgrades and **Multiversion** upgrades.

**Standard** 
- Moves Sourcegraph one version forward (`v5.0.0` to `v5.1.0`), *this is usually one minor version unless the next version released is a major version*.
- Requires minor downtime in deployments except kubernetes where rolling updates are possible.

**Multiversion**
- Moves Sourcegraph multiple versions forward (`v5.0.0` to `v5.2.0`).
- Requires downtime while the database schemas and rewritten and unfinished out-of-band migrations are applied
- We currently support jumping from version `v3.20` or later to any future version.
- **AMIs do not yet support multiversion upgrades. We hope to improve this soon.**

> *Note: Patch versions don't determine upgrade type -- you should always upgrade to the latest patch.*

| From Version | To Version | Upgrade Type | Notes                                |
|--------------|------------|--------------|--------------------------------------|
| `v5.0.0`     | `v5.1.0`   | Standard     | A minor version                       |
| `v5.0.0`     | `v5.2.0`   | Multiversion  | Two minor versions                    |
| `v3.41.0`    | `v3.42.2`  | Standard     | A minor version and patch version     |
| `v5.1.0`     | `v5.1.3`   | Standard     | Multiple patch versions     |
| `v3.43.0`    | `v4.0.0`   | Standard     | A major version change but only one absolute version |
| `v5.0.0`     | `v5.2.2`   | Multiversion  | Two minor versions and patch          |
| `v3.33.0`    | `v5.2.0`   | Multiversion  | Major and minor                       |
| `v4.5.0`     | `v5.0.0`   | Standard     | This is a major version but only one version change |
| `v4.4.2`     | `v5.0.3`   | Multiversion  | Major, minor, and patch               |

> *Note:*
> - *Our major releases do not occur on a consistent interval, so make sure to check our changelog if you aren't certain about whether a major version is multiple minor versions away from your current version. You can also reach out to our support team [support@sourcegraph.com](mailto:support@sourcegraph.com)*
> - *Sourcegraph guarantees database backward compatibility to the most recent minor version.*

### Sourcegraph databases & migrator

To facilitate the management of Sourcegraph's databases, we have created the `migrator` service. `migrator` is usually triggered automatically on Sourcegraph startup but can also be interacted with like a cli tool. Migrator's primary purpose is to manage and apply schema migrations. 
- To learn more about migrations see our [developer docs](https://docs.sourcegraph.com/dev/background-information/sql/migrations_overview). 
- For a full listing of migrator's command arguments see its [usage docs](./migrator/migrator-operations.md).

### Best Practices
> **Caution:** The upgrade process aggressively mutates the shape and contents of your database, and undiscovered errors in the migration process or unexpected environmental differences may cause an unusable instance or data loss.

It is highly recommended to:
- Take an up-to-date snapshot of your databases prior to starting a multi-version upgrade. 
- Perform the entire upgrade procedure on an idle clone of the production instance and switch traffic over on success, if possible.

## General upgrade procedure

Sourcegraph upgrades take the following general form:
1. Determine if your instance is ready to Upgrade (check upgrade notes)
2. Merge the latest Sourcegraph release into your deployment manifests
3. If updating more than a single minor version, perform an [**automatic multi-version upgrade**](./automatic.md) if targeting **Sourcegraph 5.1 or later**; [manual multi-verison upgrades](./migrator/migrator-operations.md) are required if upgrading to an earlier version, which requires shutting off the instance and invoking the `migrator` container or job to perform the database rewrite and application of unfinished out-of-band migrations
4. With upstream changes to your manifests merged, start the new instance

> Note: For more explicit steps, specific to your deployment see the operations guides linked below.

### Upgrade Readiness

Starting in v5.0.0, as an admin you are able to check instance upgrade readiness by navigating to the `Site admin > Updates` page. Here you'll be notified if your instance has any **schema drift** or unfinished **out of band migrations**.

![Screenshot 2023-05-17 at 1 37 12 PM](https://github.com/sourcegraph/sourcegraph/assets/13024338/185fc3e8-0706-4a23-b9fe-e262f9a9e4b3)

If your instance has schema drift or unfinished oob migrations you may need to address these issues before upgrading. Feel free to reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com).

- [More info on OOB migrations](https://docs.sourcegraph.com/dev/background-information/sql/migrations_overview#out-of-band-migrations)
- [More info on schema drift](https://docs.sourcegraph.com/admin/how-to/schema-drift)

## Instance Specific Procedures

### Upgrades index
- **Sourcegraph with Docker Compose**
  - [Standard Upgrade Operations](../deploy/docker-compose/upgrade.md#standard-upgrades)
  - [Multiversion Upgrade Operations](../deploy/docker-compose/upgrade.md#multi-version-upgrades)
  - [Upgrade Notes](docker_compose.md)
- **Sourcegraph with Kubernetes**
  - **Kustomize**
    - [Standard Upgrade Operations](../deploy/kubernetes/upgrade.md#standard-upgrades)
    - [Multiversion Upgrade Operations](../deploy/kubernetes/upgrade.md#multi-version-upgrades)
  - **Helm**
    - [Standard Upgrade Operations](../deploy/kubernetes/helm.md#standard-upgrades)
    - [Multiversion Upgrade Operations](../deploy/kubernetes/helm.md#multi-version-upgrades)
  - [Upgrade Notes](kubernetes.md)
- **Single-container Sourcegraph with Docker**
  - [Standard Upgrade Operations](../deploy/docker-single-container/index.md#standard-upgrades)
  - [Multiversion Upgrade Operations](../deploy/docker-single-container/index.md#multi-version-upgrades)
  - [Upgrade Notes](server.md)
- [**Pure-docker custom deployments**](pure_docker.md)

## Other helpful links
- [Migrator operations](./migrator/migrator-operations.md)
- [Upgrading Early Versions](./migrator/upgrading-early-versions.md)
- [Troubleshooting upgrades](./migrator/troubleshooting-upgrades.md)
- [Downgrading](./migrator/downgrading.md)
- [Sourcegraph 5.2 gRPC Configuration Guide](./grpc/index.md)
