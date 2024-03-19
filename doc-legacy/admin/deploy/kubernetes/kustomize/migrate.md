# Migration Docs for Kustomize

The old method of deploying Sourcegraph with custom scripts has been deprecated. Instead, the new setup uses Kustomize, a Kubernetes-native tool, for configurations. This guide explains how to migrate from the old setup ([deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph)) to the new one ([deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s)).

>NOTE: Both the old custom scripts and Kustomize only create manifests for deployment and don’t change any existing resources in an active cluster.

## Why migrate?

Here are the benefits of the new base cluster with the new Kustomize setup compared to the old one:

- Improved security defaults:
  * Runs in non-privileged mode
  * Uses non-root users
  * Does not require RBAC resources
- Streamlined resource allocation process:
  * Allocates resources based on the size of the instance
  * Optimized through load testing
  * The searcher and symbols use StatefulSets and do not require ephemeral storage
- Utilizes the Kubernetes-native tool Kustomize:
  * Built into kubectl
  * No additional scripting required
  * More extensible and composable
  * Highly reusable that enables creation of multiple instances with the same base resources and components
- Effortless configurations:
  * A comprehensive list of components pre-configured for different use cases
  * Designed to work seamlessly with Sourcegraph’s design and functionality
  * Prevents merge conflicts during upgrades

---

## Migration process

The migration process for transitioning from the Kustomize setup in [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) to [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s) involves the steps shown below. 

The goal of this migration process is to create a new overlay that will generate similiar resources as the current cluster, ensuring a smooth deployment process without disrupting existing resources. 

## Step 1: Upgrade current instance with the old repository

Upgrade your current instance to the latest version of Sourcegraph (must be 4.5.0 or above) following the [standard upgrade process](../upgrade.md#standard-upgrades) for the repository ([deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph)) your instance was deployed with.

### From Privileged to Non-privileged

Sourcegraph's deployment mode changed from privileged (containers run as root) to non-privileged (containers run as non-root) as the default in the new Kustomize setup. If your instance is currently running in privileged mode and you want to upgrade to `non-privileged` mode, use the [migrate-to-nonprivileged overlay](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/overlays/migrate-to-nonprivileged) from the Sourcegraph [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) repository when following the [standard upgrade process](../upgrade.md#standard-upgrades) to perform your upgrade.
   
>NOTE: Applying the [migrate-to-nonprivileged overlay](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/overlays/migrate-to-nonprivileged) will convert your deployment to run in non-privileged mode

## Step 2: Set up a release branch for the new repository

Set up a release branch from the latest version branch in your local fork of the [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s) repository.

```bash
  # Recommended: replace the URL with your private fork
  $ git clone https://github.com/sourcegraph/deploy-sourcegraph-k8s.git
  $ cd deploy-sourcegraph-k8s
  $ git checkout v4.5.1 && git checkout -b release
```

## Step 3: Set up a directory for your instance

Create a copy of the [instances/template](index.md#template) directory and rename it to `instances/my-sourcegraph`:

```bash
  $ cp -R instances/template instances/my-sourcegraph
```

>NOTE: In Kustomize, this directory is referred to as an [overlay](https://kubectl.docs.kubernetes.io/references/kustomize/glossary/#overlay).

## Step 4: Set up the configuration files

#### kustomization.yaml

The `kustomization.yaml` file is used to configure your Sourcegraph instance. 

**1.** Rename the [kustomization.template.yaml](index.md#kustomization-yaml) file in `instances/my-sourcegraph` to `kustomization.yaml`.

- The `kustomization.yaml` file is used to configure your Sourcegraph instance. 

```bash
  $ mv instances/my-sourcegraph/kustomization.template.yaml instances/my-sourcegraph/kustomization.yaml
```

#### buildConfig.yaml

**2.** Rename the [buildConfig.template.yaml](index.md#buildconfig-yaml) file in `instances/my-sourcegraph` to `buildConfig.yaml`.

- The `buildConfig.yaml` file is used to configure components included in your `kustomization` file when required.

```bash
  $ mv instances/my-sourcegraph/buildConfig.template.yaml instances/my-sourcegraph/buildConfig.yaml
```

## Step 5: Set namespace

Replace `ns-sourcegraph` with a namespace that matches the existing namespace for your current instance. 

You may set `namespace: default` to deploy to the default namespace.

  ```yaml
  # instances/my-sourcegraph/kustomization.yaml
    namespace: ns-sourcegraph
  ```

## Step 6: Set storage class

To add the storage class name that your current instance is using for all associated resources:

6.1. Include the `storage-class/name-update` component under the components list.

  ```yaml
  # instances/my-sourcegraph/kustomization.yaml
    components:
      # This updates storageClassName to 
      # the STORAGECLASS_NAME value from buildConfig.yaml
      - ../../components/storage-class/name-update
  ```

6.2. Input the storage class name by setting the value of `STORAGECLASS_NAME` in `buildConfig.yaml`. 

For example, set `STORAGECLASS_NAME=sourcegraph` if `sourcegraph` is the name of an existing storage class:

  ```yaml
  # instances/my-sourcegraph/buildConfig.yaml
    kind: ConfigMap
    metadata:
      name: sourcegraph-kustomize-build-config
    data:
      STORAGECLASS_NAME: sourcegraph # -- [ACTION] Update storage class name here
  ```

## Step 7: Recreate overlay (OPTIONAL)

>NOTE: You may skip this step if your instance was not deployed using overlays from the [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) repository.

If your running instance was deployed using an existing overlay from the [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) repository (except migrate-to-nonprivileged):

1. Copy and paste everything (except `kustomization.yaml`) from the old overlay directory to `instances/my-sourcegraph`.
2. Manually merge the contexts from the old `kustomization.yaml` file into `instances/my-sourcegraph/kustomization.yaml`.
3. In the new `kustomization.yaml` file, replace the old base resources (`bases/deployments`, `bases/pvcs`) with the new base resources that include:
   - `buildConfig.yaml`
   - `base/sourcegraph`
   - `base/monitoring`

For example:

```diff
# instances/my-sourcegraph/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: default
resources:
-  - ../bases/deployments
-  - ../bases/pvcs
+  - buildConfig.yaml
+  - ../../base/sourcegraph
+  - ../../base/monitoring
```

## Step 8: Recreate instance resources

Follow our [configuration guide](../configure.md) to recreate your running instance with the provided components.

It is recommended to refrain from introducing any changes to the characteristics of a running Kubernetes cluster during a migration. For example, if the cluster is currently running in privileged mode with root user access, deploying the instance in non-privileized mode could cause permission errors.

Ensure the following configurations are present/ consistent for your Sourcegraph instance during migrations:
- [Storage size for all services](../configure.md#adjust-storage-sizes)
- [Permission settings](../configure.md#base-cluster) (privileged or non-privileged mode)
- [Networking](../configure.md#network-access)
- [Ingress](../configure.md#ingress)
- [Storage class](../configure.md#storage-class)
- [Storage class name](../configure.md#update-storageclassname)
- [Namespace](../configure.md#namespace)
- [cAdvisor](../configure.md#deploy-cadvisor)
- [Tracing services](../configure.md#tracing) (OpenTelemetry and Jaeger for example)

If you have previously made changes directly to the files inside [the base directory](https://github.com/sourcegraph/deploy-sourcegraph/tree/master/base), please convert these changes into [patches](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/) before adding them to your `kustomization.yaml` file as patches.

### Privileged

If your Sourcegraph instance is currently running in privileged mode, start building your overlay with the `clusters/old-base` component, which generates resources similar to the base cluster in deploy-sourcegraph.

```yaml
# instances/my-sourcegraph/kustomization.yaml
  components:
    - ../../components/clusters/old-base
```

### Non-privileged

The default cluster now runs in non-privileged mode.

If your instance was deployed using the non-privileged overlay, you can follow the [configuration guide](../configure.md) without adding the `clusters/old-base` component.

## Step 9: Build and review new manifests

`pgsql`, `codeinsights-db`, `searcher`, `symbols`, and `codeintel-db` have been changed from `Deployments` to `StatefulSets`. However, redeploying these services as StatefulSets should not affect your existing deployment as they are all configured to use the same PVCs.

### From Deployment to StatefulSet

`searcher` and `symbols` are now StatefulSet that run as headless services. If your current `searcher` and `symbols` are running as Deployment, you will need to remove their services before re-deploying them as StatefulSet:

```bash
  $ kubectl delete service/searcher
  $ kubectl delete service/symbols
```

**1.** Generate new manifests with the overlay:

```bash
  $ kubectl kustomize my-sourcegraph -o cluster.yaml
```

**2.** Review the changes to ensure that the manifests generated by your new overlay are similar to the ones currently being used by your active cluster.

[Compare the manifests](index.md#between-an-overlay-and-a-running-cluster) generated by your new overlay with the ones in your running cluster using the command below:

```bash
  $ kubectl diff -l deploy=sourcegraph -f cluster.yaml
```


## Step 10: Deploy new manifests

Once you are satisfied with the overlay output, you can now deploy the new overlay using these commands:

```bash
  # Build manifests again with overlay
  $ kubectl kustomize $PATH_TO_OVERLAY -o cluster.yaml
  # Apply manifests to cluster
  $ kubectl apply --prune -l deploy=sourcegraph -f cluster.yaml
```

> WARNING: Make sure to test the new overlay and the migration process in a non-production environment before applying it to your production cluster.
