# Sourcegraph on Kubernetes

Deploying on Kubernetes is for organizations that need highly scalable and available code search and code navigation. 

<div class="getting-started">
  <a class="btn btn-primary text-center" href="#prerequisites">★ Installation</a>
  <a class="btn text-center" href="kustomize">Introduction</a>
  <a class="btn text-center" href="configure">Configuration</a>
  <a class="btn text-center" href="operations">Maintenance</a>
</div>

Below is an overview of installing Sourcegraph on Kubernetes using Kustomize.

### Prerequisites

* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) (v1.19 or later) with [Kustomize](https://kustomize.io/) (built into [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) in version >= 1.14)
* A [Kubernetes](https://kubernetes.io/) cluster ([v1.19 or later](https://kubernetes.io/blog/2020/08/26/kubernetes-release-1.19-accentuate-the-paw-sitive/))
   - Support for Persistent Volumes with SSDs
   - You can optionally refer to our [terraform configurations](https://github.com/sourcegraph/tf-k8s-configs) for setting up clusters on:
     - [Amazon Web Services EKS](https://github.com/sourcegraph/tf-k8s-configs/tree/main/aws)
     - [Azure AKS](https://github.com/sourcegraph/tf-k8s-configs/tree/main/azure)
     - [Google Cloud Platform GKE](https://github.com/sourcegraph/tf-k8s-configs/tree/main/gcp)

>WARNING: **If your Sourcegraph version is older than `v4.5.0` or hasn't [migrated](./kustomize/migrate.md) to [`deploy-sourcegraph-k8s`](https://github.com/sourcegraph/deploy-sourcegraph-k8s), please refer to the [legacy deployment docs for Kubernetes](https://docs.sourcegraph.com/@v4.4.2/admin/deploy/kubernetes).**

### **Step 1**: Set up a release branch

Create a release branch from the default branch (or [an available tag](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tags)) in your local fork of the [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s) repository.

See the [docs on reference repository](../repositories.md) for detailed instructions on creating a local fork.

```bash
  # Recommended: replace the URL with your private fork
  $ git clone https://github.com/sourcegraph/deploy-sourcegraph-k8s.git
  $ cd deploy-sourcegraph-k8s
  $ git checkout v4.5.1 && git checkout -b release
```

### **Step 2**: Set up a directory for your instance

Create a copy of the [instances/template](kustomize/index.md#template) directory and rename it to `instances/my-sourcegraph`:

```bash
  $ cp -R instances/template instances/my-sourcegraph
```

>NOTE: In Kustomize, this directory is referred to as an [overlay](https://kubectl.docs.kubernetes.io/references/kustomize/glossary/#overlay).

### **Step 3**: Set up the configuration files

**1.** Rename the [kustomization.template.yaml](kustomize/index.md#kustomization-yaml) file in `instances/my-sourcegraph` to `kustomization.yaml`. 

- The `kustomization.yaml` file is used to configure your Sourcegraph instance. 

```bash
  $ mv instances/my-sourcegraph/kustomization.template.yaml instances/my-sourcegraph/kustomization.yaml
```

**2.** Rename the [buildConfig.template.yaml](kustomize/index.md#buildconfig-yaml) file in `instances/my-sourcegraph` to `buildConfig.yaml`.

- The `buildConfig.yaml` file is used to configure components included in your `kustomization` file if required.

```bash
  $ mv instances/my-sourcegraph/buildConfig.template.yaml instances/my-sourcegraph/buildConfig.yaml
```

### **Step 4**: Set namespace

By default, the provided `kustomization.yaml` template deploys Sourcegraph into the `ns-sourcegraph` namespace. 

If you intend to deploy Sourcegraph into a different namespace, replace `ns-sourcegraph` with the name of the existing namespace in your cluster, or set it to `default` to deploy into the default namespace.

  ```yaml
  # instances/my-sourcegraph/kustomization.yaml
  namespace: sourcegraph
  ```

### **Step 5**: Set storage class

A storage class must be created and configured before deploying Sourcegraph. SSD storage is not required but is strongly recommended for optimal performance.

#### Option 1: Create a new storage class

We recommend using a preconfigured storage class component for your cloud provider if you can create cluster-wide resources:

```yaml
# instances/my-sourcegraph/kustomization.yaml
  components:
    # Select a component that corresponds to your cluster provider
    - ../../components/storage-class/aws/aws-ebs
    - ../../components/storage-class/aws/ebs-csi
    - ../../components/storage-class/azure
    - ../../components/storage-class/gke
```

See our [configurations guide](configure.md) for the full list of available storage class components.

#### Option 2: Use an existing storage class

If you cannot create a new storage class and/or want to use an existing one with SSDs:

<details>
  <summary>Show instruction</summary>

**1.** Include the `storage-class/name-update` component under the components list

  ```yaml
  # instances/my-sourcegraph/kustomization.yaml
    components:
      # This updates storageClassName to 
      # the STORAGECLASS_NAME value from buildConfig.yaml
      - ../../components/storage-class/name-update
  ```

**2.** Input the storage class name by setting the value of `STORAGECLASS_NAME` in `buildConfig.yaml`. 

For example, set `STORAGECLASS_NAME=sourcegraph` if `sourcegraph` is the name of an existing storage class:

  ```yaml
  # instances/my-sourcegraph/buildConfig.yaml
    kind: ConfigMap
    metadata:
      name: sourcegraph-kustomize-build-config
    data:
      STORAGECLASS_NAME: sourcegraph # -- [ACTION] Update storage class name here
  ```
</details>

#### Option 3: Use default storage class

Skip this step to use the default storage class without SSD support for non-production environments. However, you must recreate the cluster with SSDs configured for production environments later.

>WARNING: Search performance will suffer tremendously without SSDs provisioned.

### **Step 6**: Build manifests with Kustomize

Generate a new set of manifests locally using the configuration applied to the `my-sourcegraph` subdirectory without applying to the cluster.

  ```bash
  $ kubectl kustomize instances/my-sourcegraph -o cluster.yaml
  ```

### **Step 7**: Review manifests

Review the generated manifests to ensure they match your intended configuration.

  ```bash
  $ less cluster.yaml
  ```

### **Step 8**: Deploy the generated manifests

Apply the manifests from the ouput file `cluster.yaml` to your cluster:

  ```bash
  $ kubectl apply --prune -l deploy=sourcegraph -f cluster.yaml
  ```

### **Step 9**: Monitor the deployment

Monitor the deployment status to ensure all components are running properly.

  ```bash
  $ kubectl get pods -A -o wide --watch
  ```

### **Step 10**: Access Sourcegraph in Browser

To verify that the deployment was successful, port-forward the frontend pod with the following command:

  ```bash
  $ kubectl port-forward svc/sourcegraph-frontend 3080:30080
  ```

Then access your new Sourcegraph instance at http://localhost:3080 to proceed to the site-admin setup step.

  ```bash
  $ open http://localhost:3080
  ```

---

## Configure

After the initial deployment, additional configuration might be required for Sourcegraph to customize your deployment to suit your specific needs.

Common configurations that are strongly recommended for all Sourcegraph deployments:

- [Enable the Sourcegraph monitoring stack](configure.md#monitoring-stack)
- [Enable tracing](configure.md#tracing)
- [Adjust resource allocations](configure.md#instance-size-based-resources)
- [Adjust storage sizes](configure.md#adjust-storage-sizes)
- [Configure ingress](configure.md#ingress)
- [Enable TLS](configure.md#tls)
- [Enable Embeddings Service](configure.md#enable-embeddings-service)

Other common configurations include:

- [Set up an external PostgreSQL Database](configure.md#external-postgres)
- [Set up SSH connection for cloning repositories](configure.md#ssh-for-cloning)

See the [configuration guide for Kustomize](configure.md) for more configuration options.

## Helm Charts

We recommend deploying Sourcegraph on Kubernetes with Kustomize due to the flexibility it provides. If your organization uses Helm to deploy on Kubernetes, please refer to the documentation for the [Sourcegraph Helm Charts](helm.md) instead.

## Learn more

- [Scaling Sourcegraph on Kubernetes](scale.md)
- Examples of deploying Sourcegraph to the cloud provider listed below:
  - [Amazon EKS](kustomize/eks.md)
  - [Google GKE](kustomize/gke.md)
- [Migration guide](kustomize/migrate.md) on migrating from [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) to [deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s)
- [Other deployment options](../index.md)
- [Troubleshooting](troubleshoot.md)
