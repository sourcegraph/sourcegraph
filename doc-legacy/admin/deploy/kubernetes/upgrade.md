# Updating Sourcegraph with Kubernetes

This document describes the process to update a **Kubernetes Kustomize** or **Kubernetes Legacy** Sourcegraph instance. If you are unfamiliar with Sourcegraph versioning or releases see our [general concepts documentation](../../updates/index.md).

This guide is **not for use with Helm**. Please refer to the [Upgrading Sourcegraph with Helm docs](helm.md#upgrading-sourcegraph) for Helm deployments.


> ***⚠️ Attention:*** 
>   - ***Always consult the [release notes](../../updates/kubernetes.md) for the versions your upgrade will pass over and end on.***
> 
>   - *This guide assumes you have created a `release` branch following the [reference repositories docs](../repositories.md)*
> 
>   - ***please see our [cautionary note](../../updates/index.md#best-practices) on upgrades**, if you have any concerns about running a multiversion upgrade, please reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com) for advisement.*

## Standard upgrades

A [standard upgrade](../../updates/index.md#upgrade-types) occurs between a Sourcegraph version and the minor or major version released immediately after it. If you would like to jump forward several versions, you must perform a [multi-version upgrade](#multi-version-upgrades) instead.

### Upgrade with Kubernetes Kustomize

The following procedure is for performing a **standard upgrade** with Sourcegraph instances version **`v4.5.0` and above**, which have [**migrated**](../kubernetes/kustomize/migrate.md) to the [deploy-sourcegraph-k8s repo](https://github.com/sourcegraph/deploy-sourcegraph-k8s).

**Step 1**: Create a backup copy of the deployment configuration file

Make a duplicate of the current `cluster.yaml` deployment configuration file that was used to deploy the current Sourcegraph instance.

If the Sourcegraph upgrade fails, you can redeploy using the current `cluster.yaml` file to roll back and restore the instance to its previous state before the failed upgrade.

---

**Step 2**: Merge the new version of Sourcegraph into your release branch.

  ```sh
  cd $DEPLOY_SOURCEGRAPH_FORK
  # get updates
  git fetch upstream
  # to merge the upstream release tag into your release branch.
  git checkout release
  # Choose which version you want to deploy from https://github.com/sourcegraph/deploy-sourcegraph-k8s/tags
  git merge $NEW_VERSION
  ```

---

**Step 3**: Build new manifests with Kustomize

Generate a new set of manifests locally using your current overlay `instances/$INSTANCE_NAME` (e.g. INSTANCE_NAME=my-sourcegraph) without applying to the cluster.

  ```sh
  $ kubectl kustomize instances/my-sourcegraph -o cluster.yaml
  ```

Review the generated manifests to ensure they match your intended configuration and have the images for the `$NEW_VERSION` version.

  ```sh
  $ less cluster.yaml
  ```

---

**Step 4**:  Deploy the generated manifests

Apply the new manifests from the ouput file `cluster.yaml` to your cluster:

  ```sh
  $ kubectl apply --prune -l deploy=sourcegraph -f cluster.yaml
  ```

---

**Step 5**: Monitor the status of the deployment to determine its success.

  ```sh
  $ kubectl get pods -o wide --watch
  ```

---

### Upgrade with Legacy Kubernetes

The following procedure is for performing a **standard upgrade** with Sourcegraph instances in versions **prior to `v4.5.0`**, or which **have not** [**migrated**](../kubernetes/kustomize/migrate.md) and still use [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph).

**Step 1**: Merge the new version of Sourcegraph into your release branch.

  ```sh
  cd $DEPLOY_SOURCEGRAPH_FORK
  # get updates
  git fetch upstream
  # to merge the upstream release tag into your release branch.
  git checkout release
  # Choose which version you want to deploy from https://github.com/sourcegraph/deploy-sourcegraph/tags
  git merge $NEW_VERSION
  ```

---

**Step 2**: Update your install script `kubectl-apply-all.sh`

By default, the install script `kubectl-apply-all.sh` applies our base manifests using [`kubectl apply` command](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#apply) with a variety of arguments specific to the layout of the [deploy-sourcegraph reference repository](https://github.com/sourcegraph/deploy-sourcegraph).

If you have specific commands that should be run whenever you apply your manifests, you should modify this script accordingly. 

For example, if you use [overlays to make changes to the manifests](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/overlays), you should modify this script to apply the manifests from the `generated cluster` directory instead.

---

**Step 3**: Apply the updates to your cluster.

  ```sh
  $ ./kubectl-apply-all.sh
  ```

**Step 4**: Monitor the status of the deployment to determine its success.

  ```sh
  $ kubectl get pods -o wide --watch
  ```

---

## Multi-version upgrades

If you are upgrading to **Sourcegraph 5.1 or later**, we encourage you to perform an [**automatic multi-version upgrade**](../../updates/automatic.md). The following procedure has been automated, but is still applicable should errors occur in an automated upgrade. 

---

> **⚠️ Attention:** please see our [cautionary note](../../updates/index.md#best-practices) on upgrades, if you have any concerns about running a multiversion upgrade, please reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com) for advisement.

To perform a **manual** multi-version upgrade on a Sourcegraph instance running on our **kubernetes** repo follow the procedure below:

1. **Check Upgrade Readiness**:
   - Check the [upgrade notes](../../updates/kubernetes.md#kubernetes-upgrade-notes) for the version range you're passing through.
   - Check the `Site Admin > Updates` page to determine [upgrade readiness](../../updates/index.md#upgrade-readiness).

2. **Disable Connections to the Database**:
   - The following services must have their replicas scaled to 0:
     - Deployments (e.g., `kubectl scale deployment <name> --replicas=0`)
       - precise-code-intel-worker
       - repo-updater
       - searcher
       - sourcegraph-frontend
       - sourcegraph-frontend-internal
       - symbols
       - worker
     - Stateful sets (e.g., `kubectl scale sts <name> --replicas=0`):
       - gitserver
       - indexed-search

    *convenience command:*
    ```sh
    $ kubectl scale deployment precise-code-intel-worker repo-updater searcher sourcegraph-frontend sourcegraph-frontend-internal symbols worker --replicas=0 && kubectl scale sts gitserver indexed-search --replicas=0
    ```

3. **Run Migrator with the `upgrade` command**:
   - The following procedure describes running migrator in brief, for more detailed instructions and available command flags see our [migrator docs](../../updates/migrator/migrator-operations.md#kubernetes-kustomize).
     1. In the `configure/migrator/migrator.Job.yaml` manifest ([kustomize](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/migrator/migrator.Job.yaml) or [legacy](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/migrator/migrator.Job.yaml)): 
        - set the `image:` to the **latest** release of `migrator`
        - set the `args:` value to `upgrade`. **Example:**
     ```yaml
           - name: migrator
        image: "index.docker.io/sourcegraph/migrator:5.0.3"
        args: ["upgrade", "--from=v3.41.0", "--to=v4.5.1"]
        env:
     ```
      > *Note:* 
      >  - *Always use the latest image version of migrator for migrator commands, except the startup command `up`*
      >  - *You may add the `--dry-run` flag to the `command:` to test things out before altering the dbs*
     2. Run the following commands to schedule the migrator job with the upgrade command and monitor its progress:
     ```sh
     # To ensure no previous job invocations will conflict with our current invocation
     kubectl delete -f configure/migrator/migrator.Job.yaml
     # Start the migrator job
     kubectl apply -f configure/migrator/migrator.Job.yaml
     # Stream the migrator's stdout logs for progress
     kubectl logs job.batch/migrator -f
     ```
     **Example:**
     ```sh
     $ kubectl -n sourcegraph apply -f ./migrator.Job.yaml
     job.batch/migrator created
     $ kubectl -n sourcegraph get jobs
     NAME       COMPLETIONS   DURATION   AGE
     migrator   0/1           9s         9s
     $ kubectl -n sourcegraph logs job/migrator
     ❗️ An error was returned when detecting the terminal size and capabilities:
     
        GetWinsize: inappropriate ioctl for device
     
        Execution will continue, but please report this, along with your operating
        system, terminal, and any other details, to:
          https://github.com/sourcegraph/sourcegraph/issues/new
     
     ✱ Sourcegraph migrator 4.5.1
     👉 Migrating to v3.43 (step 1 of 3)
     👉 Running schema migrations
     ✅ Schema migrations complete
     👉 Running out of band migrations [1 2 4 5 7 13 14 15 16]
     ✅ Out of band migrations complete
     👉 Migrating to v4.3 (step 2 of 3)
     👉 Running schema migrations
     ✅ Schema migrations complete
     👉 Running out of band migrations [17 18]
     ✅ Out of band migrations complete
     👉 Migrating to v4.5 (step 3 of 3)
     👉 Running schema migrations
     ✅ Schema migrations complete
     $ kubectl -n sourcegraph get jobs
     NAME       COMPLETIONS   DURATION   AGE
     migrator   1/1           9s         35s
     ```

4. **Pull and merge upstream changes**: 
     - Follow the [standard legacy upgrade procedure](#upgrade-with-legacy-kubernetes) to pull and merge upstream changes from the version you are upgrading to to your `release` branch.

5. **Scale your replicas back up and apply new manifests**: 
     - The [legacy kubernetes upgrade procedure](#upgrade-with-legacy-kubernetes) describes this step in more detail.
       - Ensure that the replica counts adjusted in the previous steps are turned back up.
       - Run `./kubectl-apply-all.sh` to deploy the new pods to the Kubernetes cluster.
       - Monitor the status of the deployment via `kubectl get pods -o wide --watch`.

---

### Using MVU to Migrate to Kustomize

Due to limitations with the Kustomize deployment method introduced in Sourcegraph `v4.5.0`, multi-version upgrades (e.g. `v4.2.0` -> `v5.0.3`), migrations to `deploy-sourcegraph-k8s` should be conducted seperately from a full upgrade. 

Admins upgrading a Sourcegraph instance older than `v4.5.0` and migrating from our [legacy kubernetes](https://github.com/sourcegraph/deploy-sourcegraph) offering to our new [kustomize manifests](https://github.com/sourcegraph/deploy-sourcegraph-k8s) should upgrade to `v4.5.0` perform the `migrate` [procedure](../kubernetes/kustomize/migrate.md) and then perfom the remaining upgrade to bring Sourcegraph up to the desired version.

## Rollback

## Rollback

You can rollback by resetting your `release` branch to the old state before redeploying the instance.

If you are rolling back more than a single version, then you must also [rollback your database](../../how-to/rollback_database.md), as database migrations (which may have run at some point during the upgrade) are guaranteed to be compatible with one previous minor version.

### Rollback with Kustomize

**For Sourcegraph version 4.5.0 and above, which have [migrated](../kubernetes/kustomize/migrate.md) to [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s).**

For instances deployed using the [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s) repository:

  ```sh
  # Re-generate manifests
  kubectl kustomize instances/$YOUR_INSTANCE -o cluster-rollback.yaml
  # Review manifests
  less cluster-rollback.yaml
  # Re-deploy
  kubectl apply --prune -l deploy=sourcegraph -f cluster-rollback.yaml
  ```

### Rollback without Kustomize

**For Sourcegraph version prior to 4.5.0 using our legacy [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) manifests.**

For instances deployed using the old [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) repository:

  ```sh
  $ ./kubectl-apply-all.sh
  ```

### Rollback with `migrator downgrade`

For rolling back a multiversion upgrade use the `migrator` [downgrade](../../updates/migrator/migrator-operations.md#downgrade) command. Learn mor in our [downgrade docs](../../updates/migrator/downgrading.md).

---

## Database migrations

In some situations, administrators may wish to migrate their databases before upgrading the rest of the system to reduce downtime. Sourcegraph guarantees database backward compatibility to the most recent minor point release so the database can safely be upgraded before the application code.

To execute the database migrations independently, follow the [Kubernetes instructions on how to manually run database migrations](../../updates/migrator/migrator-operations.md). Running the `up` (default) command on the `migrator` of the *version you are upgrading to* will apply all migrations required by the next version of Sourcegraph.

## Improving update reliability and latency with node selectors

Some of the services that comprise Sourcegraph require more resources than others, especially if the
default CPU or memory allocations have been overridden. During an update when many services restart,
you may observe that the more resource-hungry pods (e.g., `gitserver`, `indexed-search`) fail to
restart, because no single node has enough available CPU or memory to accommodate them. This may be
especially true if the cluster is heterogeneous (i.e., not all nodes have the same amount of
CPU/memory).

If this happens, do the following:

- Use `kubectl drain $NODE` to drain a node of existing pods, so it has enough allocation for the larger
  service.
- Run `watch kubectl get pods -o wide` and wait until the node has been drained. Run `kubectl get pods` to check that all pods except for the resource-hungry one(s) have been assigned to a node.
- Run `kubectl uncordon $NODE` to enable the larger pod(s) to be scheduled on the drained node.

Note that the need to run the above steps can be prevented altogether with [node
selectors](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector), which
tell Kubernetes to assign certain pods to specific nodes.

---

## High-availability updates

Sourcegraph is designed to be a high-availability (HA) service, but upgrades by default require a 10m downtime
window. If you need zero-downtime upgrades, please contact us. Services employ health checks to test the health
of newly updated components before switching live traffic over to them by default. HA-enabling features include
the following:

- Replication: nearly all of the critical services within Sourcegraph are replicated. If a single instance of a
  service fails, that instance is restarted and removed from operation until it comes online again.
- Updates are applied in a rolling fashion to each service such that a subset of instances are updated first while
  traffic continues to flow to the old instances. Once the health check determines the set of new instances is
  healthy, traffic is directed to the new set and the old set is terminated. By default, some database operations
  may fail during this time as migrations occur so a scheduled 10m downtime window is required.
- Each service includes a health check that detects whether the service is in a healthy state. This check is specific to
  the service. These are used to check the health of new instances after an update and during regular operation to
  determine if an instance goes down.
- Database migrations are handled automatically on update when they are necessary.

---

## Troubleshooting

See the [troubleshooting page](troubleshoot.md).
