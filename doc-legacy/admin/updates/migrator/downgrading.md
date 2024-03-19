# Downgrading

This document covers the risks and methods of downgrading a Sourcegraph instance.

## General

Sourcegraph guarantees database backward compatibility to the most recent minor version. According to this policy database schemas should be compatible with application code from the previous release. This means that in principle rolling back a Sourcegraph instance after an upgrade only requires reverting changes made by the last standard upgrade and reapplying manifests.

In practice downgrading a Sourcegraph instance should always be a last option. Some versions of Sourcegraph apply out of band migrations which are irreversible and data loss may result. **We highly recommend creating a database backup before proceeding with an upgrade**.

> Note: As a percaution its advised that after any downgrade the databases are checked with the `migrator` [`drift` command](./migrator-operations.md#drift) to identify any problems reversing migrations.

If ever you are uncertain about a downgrade please reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com)

## Multiversion downgrades - `downgrade` with `migrator`

The migrator service can be run with the `downgrade` command. See the [command documentation](./migrator-operations.md#downgrade).

`downgrade` applies reverse schema migrations corresponding with the migrations performed over a version range. It also triggers reverse out of band migrations where possible. Some out of band migrations are irreversible. 

`downgrade` can be run as part of a standard upgrade rollback. To ensure out of band migrations are reversed. 

Heres an example of `downgrade` being used on a Docker-compose deployment:

*Altered `command:` value in the `docker-compose.yaml`*
```yaml
command: ['downgrade', '--from=v4.5.1', '--to=v4.0.0']
```
*Command and output*
```bash
$ ~/deploy-sourcegraph-docker/docker-compose/ v4.5.1* docker-compose up -d migrator
codeinsights-db is up-to-date
codeintel-db is up-to-date
pgsql is up-to-date
Recreating migrator ... done
$ ~/deploy-sourcegraph-docker/docker-compose/ v4.5.1* docker logs migrator
✱ Sourcegraph migrator 5.0.3
👉 Migrating to v4.3 (step 1 of 2)
👉 Running schema migrations
✅ Schema migrations complete
👉 Running out of band migrations [17 18]
✅ Out of band migrations complete
👉 Migrating to v4.0 (step 2 of 2)
👉 Running schema migrations
✅ Schema migrations complete
```

Downgrade is run using a similar to procedure to a [multiversion upgrade](../index.md#upgrades-index):
- Services that connect to the databases must be disabled before running `downgrade`
- Manifests must be reverted to the version you are downgrading to and reapplyed **after** the `downgrade` is complete.

## Rolling back a standard upgrade

### Docker-compose

Revert changes to your `release` branch and redeploy your Sourcegraph instance with `docker-compose up`

### Kubernetes

You can rollback by resetting your `release` branch to the old state before redeploying the instance.

### Rollback with Kustomize

**For Sourcegraph versions `v4.5.0` and above, which have [migrated](../../deploy/kubernetes/kustomize/migrate.md) to [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s):**

  ```bash
  # Re-generate manifests
  $ kubectl kustomize instances/$YOUR_INSTANCE -o cluster-rollback.yaml
  # Review manifests
  $ less cluster-rollback.yaml
  # Re-deploy
  $ kubectl apply --prune -l deploy=sourcegraph -f cluster-rollback.yaml
  ```

### Rollback without Kustomize

**For Sourcegraph versions prior to `v4.5.0`, or which have not migrated away from [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph):**

  ```bash
  $ ./kubectl-apply-all.sh
  ```
