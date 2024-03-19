<style>

.note p:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(../../../code_monitoring/file-icon.svg);
  margin-right: 0.2em;
  margin-bottom: -0.29em;
}

.warning p:before {
  content: '⚠️';
  display: inline-block;
  background-size: contain;
  background-repeat: no-repeat;
  margin-right: 0.2em;
  margin-bottom: -0.29em;
}

</style>

# Configure Sourcegraph with Kustomize

This guide will demonstrate how to customize a Kubernetes deployment (**non-Helm**) using Kustomize components.

<div class="getting-started">
  <a class="btn text-center" href="./index">Installation</a>
  <a class="btn text-center" href="./kustomize">Introduction</a>
  <a class="btn text-center btn-primary" href="#">★ Configuration</a>
  <a class="btn text-center" href="./operations">Maintenance</a>
</div>

## Overview

To ensure optimal performance and functionality of your Sourcegraph deployment, please only include components listed in the [kustomization.template.yaml file](kustomize/index.md#kustomization-yaml) for your instance overlay. These components include settings that have been specifically designed and tested for Sourcegraph and do not require any additional configuration changes.

The order of components listed in the [kustomization.template.yaml file](kustomize/index.md#kustomization-yaml) is important and should be maintained. The components are listed in a specific order to ensure proper dependency management and compatibility between components. Reordering components can introduce conflicts or prevent components from interacting as expected. Only modify the component order if explicitly instructed to do so by the documentation. Otherwise, leave the component order as-is to avoid issues.

Following these guidelines will help you create a seamless deployment and avoid conflicts.

> NOTE: All commands in this guide should be run from the root of the reference repository.

## Base cluster

The base resources in Sourcegraph include the services that make up the main Sourcegraph apps as well as the monitoring services ([tracing services](#tracing) and [cAdvisor](#deploy-cadvisor) are not included). These services are configured to run as non-root users without privileges, ensuring a secure deployment:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  resources:
    # Deploy Sourcegraph main stack
    - ../../base/sourcegraph
    # Deploy Sourcegraph monitoring stack
    - ../../base/monitoring
```

To enable cluster metrics monitoring, you will need to [deploy cAdvisor](#deploy-cadvisor), which is a container resource usage monitoring service. cAdvisor is configured for Sourcegraph and can be deployed using one of the provided components. This component contains RBAC resources and must be run with privileges to ensure that it has the necessary permissions to access the container metrics.

### RBAC

Sourcegraph has removed Role-Based Access Control (RBAC) resources from the default base cluster for the Kustomize deployment. This means that [service discovery](#service-discovery) is not enabled by default, and the endpoints for each service replica must be manually added to the frontend ConfigMap. When using the [size components](#instance-size-based-resources) included in the [kustomization file built for Sourcegraph](kustomize/index.md#kustomization-yaml), service endpoints are automatically added to the ConfigMap.

### Non-Privileged

By default, all Sourcegraph services are deployed in a non-root and non-privileged mode, as defined in the [base](kustomize/index.md#base) cluster.

### Privileged

To deploy a High Availability (HA) configured Sourcegraph instance to an RBAC-enabled cluster, you can include the [privileged component](#privileged) and the [privileged component for monitoring](#deploy-cadvisor) in your component list. This provides redundant, monitored instances of each service running as root with persistent storage, and adding the [service discovery component](#service-discovery) enables automated failover and recovery by detecting backend service endpoints.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  resources:
    - ../../base/sourcegraph # Deploy Sourcegraph main stack
    - ../../base/monitoring # Deploy Sourcegraph monitoring stack
  components:
    # Add resources for cadvisor
    # NOTE: cadvisor includes RBAC resources and must be run with privileges
    - ../../components/monitoring/cadvisor
    # Run Sourcegraph main stack with privilege and root
    # NOTE: This adds RBAC resources to the monitoring stack
    - ../../components/privileged
    # Run monitoring services with privilege and root
    # It also allows Prometheus to talk to the Kubernetes API for service discovery
    - ../../components/monitoring/privileged
    # Enable service discovery by adding RBACs
    # IMPORTANT: Include as the last component
    - ../../components/enable/service-discovery
```

### Service discovery

RBAC must be enabled in your cluster for the frontend to communicate with other services through the Kubernetes API. To enable service discovery for the frontend service, Include the following component as the **last** component in your `kustomization.yaml` file:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/...
    # IMPORTANT: Include this as the last component
    - ../../components/enable/service-discovery
```

This will allow the frontend service to discover endpoints for each service replica and communicate with them through the Kubernetes API. Note that this component should only be added if RBAC is enabled in your cluster.

### Embeddings Service
By default, the Embeddings service which is used to handle embeddings searches is disabled. To enable it the following must be commented out. By default, the Embeddings service stores indexes in `blobstore`. Use the [embeddings-backend](./configure.md#external-embeddings-object-storage) patch to configure an external object store.
```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
components:
  - ../../components/remove/embeddings # -- Disable Embeddings service by default
```

---

## Monitoring stack

The monitoring stack for Sourcegraph, similar to the main stack, does not include RBAC (Role-Based Access Control) resources by default. As a result, some dashboards may not display any data unless cAdvisor is deployed separately with privileged access.

To deploy the monitoring stack, add the monitoring resources to the resources-list:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  resources:
    # Deploy Sourcegraph main stack
    - ../../base/sourcegraph
    # Deploy Sourcegraph monitoring stack
    - ../../base/monitoring
```

If RBAC is enabled in your cluster, it is highly recommended to [deploy cAdvisor](#deploy-cadvisor) with privileged access to your cluster. With privileged access, cAdvisor will have the necessary permissions to gather and display detailed information about the resources used by your Sourcegraph instance. It's considered a key component for monitoring and troubleshooting. See [Deploy cAdvisor](#deploy-cadvisor) below for more information.

### Deploy cAdvisor

cAdvisor requires a service account and certain permissions to access and gather information about the Kubernetes cluster in order to display key metrics such as resource usage and performance data. Removing the service account and privileged access for cAdvisor could impede its ability to collect this information, resulting in missing data on Grafana dashboards and potentially impacting visibility and monitoring capabilities for the cluster and its pods. This could negatively impact the level of monitoring and visibility into the cluster's state that cAdvisor is able to provide.

To deploy cAdvisor with privileged access, include the following:

- [monitoring base resources](#monitoring-stack)
- [monitoring/privileged component](#monitoring-stack)
- [privileged component](#privileged)
- [cadvisor component](#deploy-cadvisor)

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  resources:
    - ../../base/sourcegraph # Deploy Sourcegraph main stack
    - ../../base/monitoring # Deploy Sourcegraph monitoring stack
  components:
    # Add resources for cadvisor
    # NOTE: cadvisor includes RBAC resources and must be run with privileges
    - ../../components/monitoring/cadvisor
    # Run Sourcegraph main stack with privilege and root
    # NOTE: This adds RBAC resources to the monitoring stack
    - ../../components/privileged
    # Run monitoring services with privilege and root
    # It also allows Prometheus to talk to the Kubernetes API for service discovery
    - ../../components/monitoring/privileged
    # Enable service discovery by adding RBACs
    # IMPORTANT: Include as the last component
    - ../../components/enable/service-discovery
```

### Prometheus targets

Skip this configuration if you are running Prometheus with RBAC permissions.

Because Sourcegraph is running without RBAC permissions in Kubernetes, it cannot auto-discover metrics endpoints to scrape metrics. As a result, Prometheus (the metrics scraper) requires a static target list (list of endpoints to scrape) provided in a ConfigMap.

The default target list ConfigMap only contains endpoints for the `default` and `ns-sourcegraph` namespaces, where Sourcegraph may be installed. If you have installed Sourcegraph in a different namespace, the provided target list will be missing those endpoints.

To accommodate this, you must update the namespace within the target list. This ensures Prometheus has the correct list of endpoints to scrape metrics from.

**Step 1**: Create a subdirectory called `patches` within the directory of your overlay:

```bash
$ mkdir -p instances/$INSTANCE_NAME/patches
```

**Step 2**: Set the `SG_NAMESPACE` value to your namespace in your terminal:

```bash
$ export SG_NAMESPACE=your_namespace
```

**Step 3**: Replace the value of `$SG_NAMESPACE` in the `prometheus.ConfigMap.yaml` file with your Sourcegraph namespace value set in the previous step using [envsubst](https://www.gnu.org/software/gettext/manual/html_node/envsubst-Invocation.html), and the save the output of the substitution as a new file in the `patches` directory created in step 1:

```bash
envsubst < base/monitoring/prometheus/prometheus.ConfigMap.yaml > instances/$INSTANCE_NAME/patches/prometheus.ConfigMap.yaml
```

**Step 4**: Add the new patch file to your overlay under `patches`:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  patches:
    - patch: patches/prometheus.ConfigMap.yaml
```

Following these steps will allow Prometheus to successfully scrape metrics from a Sourcegraph installation in namespaces other than `default` and `ns-sourcegraph` without RBAC Kubernetes permissions, by using a pre-defined static target list of endpoints.

---

## Tracing

Sourcegraph exports traces in OpenTelemetry format. The OpenTelemetry collector, which must be configured as part of the deployment using the [otel component](#deploy-opentelemetry-collector), [collects and exports traces](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/docker-images/opentelemetry-collector/configs/logging.yaml).

By default, Sourcegraph supports exporting traces to multiple backends including Jaeger.

### Deploy OpenTelemetry Collector

Include the `otel` component to deploy OpenTelemetry Collector:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # Deploy OpenTelemetry Collector
    - ../../components/monitoring/otel
```

Learn more about Sourcegraph's integrations with the [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/) in our [OpenTelemetry documentation](../../observability/opentelemetry.md).

### Deploy OpenTelemetry Collector with Jaeger as tracing backend

If you do not have an external backend available for the OpenTelemetry Collector to export tracing data to, you can deploy the Collector with the Jaeger backend to store and view traces using the `tracing component` as described below.

#### Enable the bundled Jaeger deployment

**Step 1**: Include the `tracing` component to deploy both OpenTelemetry and Jaeger. The component also configures the following services:

- `otel-collector` to export to this Jaeger instance
- `grafana` to get metrics from this Jaeger instance

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # Deploy OpenTelemetry Collector and Jaeger
    - ../../components/monitoring/tracing
```

**Step 2**: In your Site configuration, add the following to:

- sends Sourcegraph traces to OpenTelemetry Collector
- send traces from OpenTelemerty to Jaeger

```json
{
  "observability.client": {
    "openTelemetry": {
      "endpoint": "/-/debug/otlp"
    }
  },
  "observability.tracing": {
    "type": "opentelemetry",
    "urlTemplate": "{{ .ExternalURL }}/-/debug/jaeger/trace/{{ .TraceID }}"
  }
}
```

### Configure a tracing backend

Follow these steps to add configure OpenTelementry to use a different backend:

1. Create a subdirectory called 'patches' within the directory of your overlay
2. Copy and paste the [base/otel-collector/otel-collector.ConfigMap.yaml file](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s@master/-/tree/base/otel-collector/otel-collector.ConfigMap.yaml) to the new [patches subdirectory](kustomize/index.md#patches-directory)
3. In the copied file, make the necessary changes to the `exporters` and `service` blocks to connect to your backend based on the documentation linked above
4. Include the following content in your overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/otel-collector/backend
  patches:
    - patch: patches/otel-collector.ConfigMap.yaml
```

The component will update the `command` for the `otel-collector` container to `"--config=/etc/otel-collector/conf/config.yaml"`, which is now pointing to the mounted config.

Please refer to [OpenTelemetry](../../observability/opentelemetry.md) for detailed descriptions on how to configure your backend of choice.

---

## Namespace

Recommended namespace: `ns-sourcegraph`

Some resources and components (e.g. [Prometheus](#prometheus-targets)) are pre-configured to work with the `default` and `ns-sourcegraph` namespaces only and require additional configurations when running in a different namespace. Please refer to the [Prometheus](#prometheus-targets) section for more details.

### Set namespace

To set a namespace for all your Sourcegraph resources, update the `namespace` field to an existing namespace in your cluster:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  namespace: ns-sourcegraph
```

This will set namespace to the `namespace` value (`ns-sourcegraph` in this example) for all your Sourcegraph resources.

> NOTE: This step assumes that the namespace already exists in your cluster. If the namespace does not exist, you will need to create one before applying the overlay.

### Create a namespace

To create a new namespace, include the [utils/namespace](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/resources/namespace) component in your overlay.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  namespace: ns-sourcegraph
  components:
    - ../../components/resources/namespace
```

This component will create a new namespace using the  `namespace` value (`ns-sourcegraph` in this example).

---

## Resources

Properly allocating resources is crucial for ensuring optimal performance of your Sourcegraph instance. To ensure this, it is recommended to use one of the provided [sizes components](#instance-size-based-resources) for resource allocation, specifically designed for your [instance size](../instance-size.md). These components have been tested and optimized based on load test results, and are designed to work seamlessly with Sourcegraph's design and functionality.

### Instance-size-based resources

To allocate resources based on your [instance size](../instance-size.md):

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # Include ONE of the sizes component based on your instance size
    - ../../components/sizes/xs
    - ../../components/sizes/s
    - ../../components/sizes/m
    - ../../components/sizes/l
    - ../../components/sizes/xl
```

### Adjust storage sizes

You can adjust storage size for different services in the *STORAGE SIZES* section with [patches](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/patches/).

Here is an example on how to adjust the storage sizes for different services:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/sizes/l
  # [STORAGE SIZES]
  patches:
    # `pgsql` to 500Gi
    - target:
        kind: PersistentVolumeClaim
        name: pgsql
      patch: |-
        - op: replace
          path: /spec/resources/requests/storage
          value: 500Gi
    # `redis-store` & `redis-cache` to 200Gi
    - target:
        kind: PersistentVolumeClaim
        name: redis-store|redis-cache
      patch: |-
        - op: replace
          path: /spec/resources/requests/storage
          value: 200Gi
    # `gitserver` & `indexed-search` to 1000Gi
    - target:
        kind: StatefulSet
        name: gitserver|indexed-search
      patch: |-
        - op: replace
          path: /spec/volumeClaimTemplates/0/spec/resources/requests/storage
          value: 1000Gi
```

### Custom resources allocation

> WARNING: Only available in version 4.5.0 or above

In cases where adjusting resource allocation (e.g. replica count, resource limits, etc.) is necessary, it is important to follow the instructions provided below.

> NOTE: The built-in [replica transformer](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/replicas/) is currently not supported unless being used as a component.

**Step 1**: Create a copy of the `components/custom/resources` directory inside your overlay directory `instances/$INSTANCE_NAME`

Rename it to `custom-resources`:

```bash
# rename the directory from 'custom/resources' to 'custom-resources'
$ cp -R components/custom/resources instances/$INSTANCE_NAME/custom-resources
```

**Step 2**: In the newly copied `instances/$INSTANCE_NAME/custom-resources/kustomization.yaml` file:

1. uncomment the patches for the services you would like to adjust resources for, and then;
2. update the resource values accordingly

For example, the following patches update the resources for:

- gitserver
  - increase replica count to 2
  - adjust resources limits and requests
- pgsql
  - increase storage size to 500Gi

```yaml
# instances/$INSTANCE_NAME/custom-resources/kustomization.yaml
  patches:
    - patch: |-
        apiVersion: apps/v1
        kind: StatefulSet
        metadata:
          name: gitserver
        spec:
          replicas: 2
          template:
            spec:
              containers:
                - name: gitserver
                  resources:
                    limits:
                      cpu: "8"
                      memory: 32G
                    requests:
                      cpu: "1"
                      memory: 2G
    - patch: |-
        apiVersion: v1
        kind: PersistentVolumeClaim
        metadata:
          name: pgsql
        spec:
          resources:
            requests:
              storage: 500Gi
```

> WARNING: Please make sure the patches for services you are not adjusting resources for are left uncommented. **Do not remove** `patches/update-endpoints.yaml`.

**Step 3**: In the `instances/$INSTANCE_NAME/kustomization.yaml` file, add the `custom-resources` directory created in step 1 to your components-list under the *Resource Allocation* section:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - custom-resources
```

> WARNING: If *service-discovery* is not enabled for the sourcegraph-frontend service, the `instances/$INSTANCE_NAME/custom-resources/endpoint-update.yaml` file within the patches subdirectory is responsible for updating the relevant variables for the frontend to generate the endpoint addresses for each service replica. It should not be removed at any point.

### Remove securityContext

The `remove/security-context` component removes all the `securityContext` configurations pre-defined in base.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/remove/security-context
```

### Remove DaemonSets

If you do not have permission to deploy DaemonSets, you can include the `remove/daemonset` component to remove all services with DaemonSets resources (e.g. node-exporter) from the [monitoring resources](#monitoring-stack):

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  resources:
    # Deploy Sourcegraph main stack
    - ../../base/sourcegraph
    # Deploy Sourcegraph monitoring stack
    - ../../base/monitoring
  components:
    # Make sure the cAdvisor and otel are excluded from the components list
    # As they both include daemonset
    # - ../../components/monitoring/cadvisor
    # - ../../components/monitoring/otel
    # component to remove all daemonsets from the monitoring stack
    - ../../components/remove/daemonset
```

> NOTE: If `monitoring` is not included under resources, adding the `remove/daemonset component` would result in errors because there will be no daemonsets to remove.

---

## Storage class

A storage class is required for all persistent volume claims by default. It must be configured and created before deploying Sourcegraph to your cluster. See [the official documentation](https://kubernetes.io/docs/tasks/administer-cluster/change-pv-reclaim-policy/#changing-the-reclaim-policy-of-a-persistentvolume) for more information about configuring persistent volumes.

### Google Cloud Platform


**Step 1**: Read and follow the [official documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver) for enabling the persistent disk CSI driver on a [new](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver#enabling_the_on_a_new_cluster) or [existing](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver#enabling_the_on_an_existing_cluster) cluster.

**Step 2**: Include the GCP storage class component to the `kustomization.yaml` file for your Kustomize overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/gcp
```

The component takes care of creating a new storage class named `sourcegraph` with the following configurations:

- Provisioner: pd.csi.storage.gke.io
- SSD: types: pd-ssd

It also updates the storage class name for all resources to `sourcegraph`.

[Additional documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver) for more information.

### Amazon Web Services

**Step 1**: Follow the [official instructions](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html) to deploy the [Amazon Elastic Block Store (Amazon EBS) Container Storage Interface (CSI) driver](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html).

**Step 2**: Include one of the AWS storage class components in your overlay: [storage-class/aws/eks](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/storage-class/aws/eks) or [storage-class/aws/ebs](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/storage-class/aws/ebs)

   * The [storage-class/aws/ebs-csi](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/storage-class/aws/eks) component is configured with the `ebs.csi.aws.com` storage class provisioner for clusters with self-managed Amazon EBS Container Storage Interface driver installed
   * The [storage-class/aws/aws-ebs](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/storage-class/aws/ebs) component is configured with the `kubernetes.io/aws-ebs` storage class provisioner for clusters with the [AWS EBS CSI driver installed as Amazon EKS add-on](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html)


```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # Set provisioner to `kubernetes.io/aws-ebs`
    - ../../components/storage-class/aws/aws-ebs
    # Set provisioner to `ebs.csi.aws.com`
    - ../../components/storage-class/aws/ebs-csi
```

[Additional documentation](https://docs.aws.amazon.com/eks/latest/userguide/ebs-csi.html) for more information.

### Azure

> WARNING: If you are deploying on Azure, you **must** ensure that your cluster is created with support for CSI storage drivers [(link)](https://docs.microsoft.com/en-us/azure/aks/csi-storage-drivers). This **can not** be enabled after the fact

**Step 1**: Follow the [official instructions](https://docs.microsoft.com/en-us/azure/aks/csi-storage-drivers) to deploy the [Container Storage Interface (CSI) drivers](https://learn.microsoft.com/en-us/azure/aks/csi-storage-drivers).

**Step 2**: Include the azure storage class component to the `kustomization.yaml` file for your Kustomize overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/azure
```

This component creates a new storage class named `sourcegraph` in your cluster with the following configurations:

- provisioner: disk.csi.azure.com
- parameters.storageaccounttype: Premium_LRS
  - This configures SSDs and is highly recommended.
  - **A Premium VM is required.**

[Additional documentation](https://docs.microsoft.com/en-us/azure/aks/csi-storage-drivers) for more information.

### k3s

Configure to use the default storage class `local-path` in a k3s cluster:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/k3s
```

### Trident

If you are using Trident as your storage orchestrator, you must have [fsType](https://docs.netapp.com/us-en/trident/trident-reference/objects.html#storage-pool-selection-attributes) defined in your storageClass for it to respect the volume ownership required by Sourcegraph. When [fsType](https://docs.netapp.com/us-en/trident/trident-reference/objects.html#storage-pool-selection-attributes) is not set, all the files within the cluster will be owned by user `99 (NOBODY)`, resulting in permission issues for all Sourcegraph databases.

Add one of the available `storage-class/trident/$FSTYPE` components to the `kustomization.yaml` file for your Kustomize overlay based on your fsType:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # -- fsType: ext3
    - ../../components/storage-class/trident/ext3
    # -- fsType: ext4
    - ../../components/storage-class/trident/ext4
    # -- fsType: xfs
    - ../../components/storage-class/trident/xfs
```

### Other cloud providers

To use an **existing** storage class provided by other cloud providers:

**Step 1**: Include the `storage-class/name-update` component to your overlay:

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/name-update
  ```

**Step 2**: Enter the value of your existing storage class name in your [buildConfig.yaml file](kustomize/index.md#buildconfig-yaml) using the `STORAGECLASS_NAME` config key

Example, add `STORAGECLASS_NAME=sourcegraph` if `sourcegraph` is the name for the existing storage class:

  ```yaml
  # instances/$INSTANCE_NAME/buildConfig.yaml
  kind: SourcegraphBuildConfig
  metadata:
    name: sourcegraph-kustomize-config
  data:
    STORAGECLASS_NAME: sourcegraph # [ACTION] Set storage class name here
  ```

  The `storage-class/name-update` component updates the `storageClassName` field for all associated resources to the `STORAGECLASS_NAME` value set in step 2.

### Update storageClassName

To updates the `storageClassName` field for all associated resources:

**Step 1**: Include the `storage-class/name-update` component to your overlay:

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/name-update
  ```

**Step 2**: Enter the value of your existing storage class name in your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file using the `STORAGECLASS_NAME` config key

Example, set `STORAGECLASS_NAME=sourcegraph` if `sourcegraph` is the name for the existing storage class:

  ```yaml
  # instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    STORAGECLASS_NAME: sourcegraph # [ACTION] Set storage class name here
  ```

  The `storage-class/name-update` component updates the `storageClassName` field for all associated resources to the `STORAGECLASS_NAME` value set in step 2.

### Create a custom storage class

To create a custom storage class:

**Step 1**: Include the `storage-class/cloud` component to your overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/storage-class/cloud
```

Update the following variables in your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file. Replace them with the correct values according to the instructions provided by your cloud provider:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    # [ACTION] Set values below
    STORAGECLASS_NAME: STORAGECLASS_NAME
    STORAGECLASS_PROVISIONER: STORAGECLASS_PROVISIONER
    STORAGECLASS_PARAM_TYPE: STORAGECLASS_PARAM_TYPE
```

> IMPORTANT: Make sure to create the storage class in your cluster before deploying Sourcegraph

---

## Network access

To allow external users to access the main web server, you need to configure it to be reachable over the network.

We recommend using the [ingress-nginx](https://kubernetes.github.io/ingress-nginx/) [ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) controller for production environments.

### Ingress controller

To utilize the `sourcegraph-frontend` ingress, you'll need to install a NGINX ingress controller (ingress-nginx) in your cluster. Follow the official instructions at https://kubernetes.github.io/ingress-nginx/deploy/ to install the ingress-nginx controller in your cluster.

Alternatively, you can build the manifests using one of our pre-configured ingress-nginx-controller overlays:

```bash
# Build manifests for AWS
$ kubectl kustomize examples/ingress-controller/aws -o ingress-controller.yaml
# Build manifests for other cloud providers
$ kubectl kustomize examples/ingress-controller/cloud -o ingress-controller.yaml
# Deploy to cluster after reviewing the manifests in ingress-controller.yaml
$ kubectl apply -f ingress-controller.yaml
```

Check the external address by running the following command and look for the `LoadBalancer` entry:

```bash
$ kubectl -n ingress-nginx get svc
```

Verify that the ingress-nginx IP is accessible. If you are having trouble accessing Sourcegraph, see [Troubleshooting ingress-nginx](https://kubernetes.github.io/ingress-nginx/troubleshooting/) for further assistance. The namespace of the ingress-controller is ingress-nginx.

Once you have completed the installation process for Sourcegraph, run the following command to check if an IP address has been assigned to your ingress resource. This IP address or the configured URL can then be used to access Sourcegraph in your browser.

```bash
$ kubectl get ingress sourcegraph-frontend

NAME                   CLASS    HOSTS             ADDRESS     PORTS     AGE
sourcegraph-frontend   <none>   sourcegraph.com   8.8.8.8     80, 443   1d
```

### TLS

To ensure secure communication, it is recommended to enable Transport Layer Security (TLS) and properly configure a certificate on your Ingress. This can be done by utilizing managed certificate solutions provided by cloud providers or by manually configuring a certificate.

To manually configure a certificate via [TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets), follow these steps:

**Step 1**: Move the `tls.crt` and `tls.key` files to the root of your overlay directory (e.g. `instances/$INSTANCE_NAME`).

**Step 2**: Include the following lines in your overlay to generate secrets with the provided files:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml > [SECRETS GENERATOR]
  secretGenerator:
  - name: sourcegraph-frontend-tls
    behavior: create
    files:
    - tls.crt
    - tls.key
```

This will create a new Secret resource named sourcegraph-frontend-tls that contains the encoded cert and key:

```yaml
# cluster.yaml - output file after running build
  apiVersion: v1
  kind: Secret
  type: kubernetes.io/tls
  metadata:
    name: sourcegraph-frontend-tls-99dh8g92m5
    namespace: $YOUR_NAMESPACE
  data:
    tls.crt: |
      LS...FUlRJRklDQVRFLS0tLS0=
    tls.key: |
      LS...SSVZBVEUgS0VZLS0tLS0=
# the data is abbreviated in this example
```

**Step 3**: Configure the TLS settings of your Ingress by adding the following variables to your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file:

- **TLS_HOST**: your domain name
- **TLS_INGRESS_CLASS_NAME**: ingress class name required by your cluster-issuer
- **TLS_CLUSTER_ISSUER**: name of the cluster-issuer

Example:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    # [ACTION] Set values below
    TLS_HOST: sourcegraph.company.com
    TLS_INGRESS_CLASS_NAME: example-ingress-class-name
    TLS_CLUSTER_ISSUER: letsencrypt
```

Step 4: Include the `tls` component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network/tls
```

### TLS with Let’s Encrypt

Alternatively, you can configure [cert-manager with Let’s Encrypt](https://cert-manager.io/docs/configuration/acme/) in your cluster. Then, follow the steps listed above for configuring TLS certificate via TLS Secrets manually. However, when adding the variables to the [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file, set **TLS_CLUSTER_ISSUER=letsencrypt** to include the cert-manager with Let's Encrypt.

### TLS secret name

If the name of your secret for TLS is not `sourcegraph-frontend-tls`, you can replace it using the `$TLS_SECRET_NAME` config key:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    # [ACTION] Set values below
    TLS_SECRET_NAME: sourcegraph-tls
```

Then, include the `tls-secretname` component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network/tls-secretname
```

---

## Ingress

Configuration options for ingress installed for sourcegraph-frontend.

### AWS ALB

Component to configure Ingress to use [AWS Load Balancer Controller](https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html) to expose Sourcegraph publicly by updating annotation to `kubernetes.io/ingress.class: alb` in frontend ingress.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/ingress/alb
```

### GKE

Component to configure network access for GKE clusters with HTTP load balancing enabled.

It also adds a [BackendConfig CRD](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#create_backendconfig). This is necessary to instruct the GCP load balancer on how to perform health checks on our deployment.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/ingress/gke
```

### k3s

Component to configure Ingress to use the default HTTP reverse proxy and load balancer `traefik` in k3s clusters.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/ingress/k3s
```

### Hostname

To configure the hostname for your Sourcegraph ingress, follow these steps:

**Step 1**: In your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file, include the `HOST_DOMAIN` variable and set it to your desired hostname, for example:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    # [ACTION] Set values below
    HOST_DOMAIN: sourcegraph.company.com
```

**Step 2**: Include the hostname component in your components.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/ingress/hostname
```

This will configure the hostname for the ingress resource, allowing external users to access Sourcegraph using the specified hostname.

### ClusterIP

The Sourcegraph frontend service is configured as a [ClusterIP](https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types) by default, which allows it to be accessed within the Kubernetes cluster using a stable IP address provided by Kubernetes. If you want to make the frontend service accessible from outside the cluster, you can use the [network/nodeport](#nodeport) or [network/loadbalancer](#loadbalancer) components.

### NodePort

The `network/nodeport` component creates a frontend service of [type NodePort](https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport), making it accessible by using the IP address of any node in the cluster, along with the specified nodePort value (30080).

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network/nodeport/30080
```

### LoadBalancer

The `network/loadbalancer` component sets the type of the frontend service as [LoadBalancer](https://kubernetes.io/docs/concepts/services-networking/service/#loadbalancer), which provisions a load balancer and makes the service accessible from outside the cluster.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
  - ../../components/network/loadbalancer
```

### Annotations

To configure ingress-nginx annotations for the Sourcegraph frontend ingress:

**Step 1**: Create a subdirectory called 'patches' within the directory of your overlay

```bash
$ mkdir -p instances/$INSTANCE_NAME/patches
```

**Step 2**: Copy the `frontend-ingress-annotations.yaml` patch file from the components/patches directory to the new [patches subdirectory](kustomize/index.md#patches-directory)

```bash
$ cp components/patches/frontend-ingress-annotations.yaml instances/$INSTANCE_NAME/patches/frontend-ingress-annotations.yaml
```

**Step 3**: Add the additional annotations at the end of the new patch file

**Step 4**: Include the patch file in your overlay under `patches`:

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/...
  # ...
  patches:
    - patch: patches/frontend-ingress.annotations.yaml
  ```

This will add the annotations specified in your copy of the [frontend-ingress-annotations.yaml](https://github.com/sourcegraph/deploy-sourcegraph-k8s/blob/master/base/sourcegraph/frontend/sourcegraph-frontend.Ingress.yaml) file to the sourcegraph-frontend ingress resource. For more information on [ingress-nginx annotations](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/annotations/), refer to the [NGINX Configuration documentation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/).

### NetworkPolicy

To configure network policy for your Sourcegraph installation, you will need to follow these steps:

1. Create a namespace for your Sourcegraph deployment as described in the [namespace section](#namespace).

2. Include the network-policy component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network-policy
```

3. Apply the network-policy component to your cluster. This will create a NetworkPolicy resource that only allows traffic between pods in the namespace labeled with name: sourcegraph-prod

4. If you need to allow traffic to external services or ingress traffic from the outside to the frontend, you will need to augment the example NetworkPolicy. You can check out this [collection](https://github.com/ahmetb/kubernetes-network-policy-recipes) of NetworkPolicies to get started.

> NOTE: You should check with your cluster administrator to ensure that NetworkPolicy is supported in your cluster.

---

## Network rule

Add a network rule that allows incoming traffic on port 30080 (HTTP) to at least one node. Note that this configuration does not include support for Transport Layer Security (TLS).

### Google Cloud Platform Firewall

- Expose the necessary ports.

```bash
$ gcloud compute --project=$PROJECT firewall-rules create sourcegraph-frontend-http --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:30080
```

- Include the nodeport component to change the type of the `sourcegraph-frontend` service from `ClusterIP` to `NodePort` with the `nodeport` component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network/nodeport/30080
```

- Directly applying this change to a running service [will fail](https://github.com/kubernetes/kubernetes/issues/42282). You must first delete the old service before redeploying a new one (with a few seconds of downtime):

```bash
$ kubectl delete svc sourcegraph-frontend
```

- Find a node name.

```bash
$ kubectl get pods -l app=sourcegraph-frontend -o=custom-columns=NODE:.spec.nodeName
```

- Get the EXTERNAL-IP address (will be ephemeral unless you [make it static](https://cloud.google.com/compute/docs/ip-addresses/reserve-static-external-ip-address#promote_ephemeral_ip)).

```bash
$ kubectl get node $NODE -o wide
```

Learn more about [Google Cloud Platform Firewall rules](https://cloud.google.com/compute/docs/vpc/using-firewalls).

### AWS Security Group

Sourcegraph should now be accessible at `$EXTERNAL_ADDR:30080`, where `$EXTERNAL_ADDR` is the address of _any_ node in the cluster.

Learn more about [AWS Security Group rules](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_SecurityGroups.html).

### Rancher Kubernetes Engine

If your [Rancher Kubernetes Engine (RKE)](https://rancher.com/docs/rke/latest/en/) cluster is configured to use [NodePort](https://docs.ranchermanager.rancher.io/v2.0-v2.4/how-to-guides/new-user-guides/migrate-from-v1.6-v2.x/expose-services#nodeport), include the [network/nodeport/custom component](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/network/nodeport/custom) to change the port type for `sourcegraph-frontend` service from `ClusterIP` to `NodePort` to use `nodePort: 30080`:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
components:
  - ../../components/network/nodeport/30080
```

> NOTE: Check with your upstream admin for the correct nodePort value.

---

## Service mesh

There are a few [known issues](troubleshoot.md#service-mesh) when running Sourcegraph with service mesh. We recommend including the `network/envoy` component in your components list to bypass the issue where Envoy, the proxy used by Istio, breaks Sourcegraph search function by dropping proxied trailers for [HTTP/1](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/core/v3/protocol.proto#config-core-v3-http1protocoloptions) requests.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/network/envoy
```

---

## Environment variables

### Frontend

To update the environment variables for the **sourcegraph-frontend** service, add the new environment variables to the end of the *FRONTEND ENV VARS* section at the bottom of your [kustomization file](kustomize/index.md#kustomizationyaml). For example:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
components:
  - ../../components/...
configMapGenerator:
  - name: sourcegraph-frontend-env
    behavior: merge
    literals:
      - DEPLOY_TYPE=kustomize
      - NEW_ENV_VAR=NEW_VALUE
```

These values will be automatically merged with the environment variables currently listed in the ConfigMap for frontend.

> WARNING: You must restart frontend for the updated values to be activated

### Gitserver

You can update environment variables for **gitserver** with `patches`:

For example, to add new environment variables `SRC_ENABLE_GC_AUTO` and `SRC_ENABLE_SG_MAINTENANCE`:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
patches:
  - target:
      name: gitserver
      kind: StatefulSet
    patch: |-
      - op: add
        path: /spec/template/spec/containers/0/env/-
        value:
          name: SRC_ENABLE_GC_AUTO
          value: "true"
      - op: add
        path: /spec/template/spec/containers/0/env/-
        value:
          name: SRC_ENABLE_SG_MAINTENANCE
          value: "false"
```

### Searcher

You can update environment variables for **searcher** with `patches`.

For example, to update the value for `SEARCHER_CACHE_SIZE_MB`:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  patches:
    - target:
        name: searcher
        kind: StatefulSet|Deployment
      patch: |-
        - op: replace
          path: /spec/template/spec/containers/0/env/0
          value:
            name: SEARCHER_CACHE_SIZE_MB
            value: "50000"
```

### Symbols

You can update environment variables for **searcher** with `patches`.

For example, to update the value for `SYMBOLS_CACHE_SIZE_MB`:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  patches:
    - target:
        name: symbols
        kind: StatefulSet|Deployment
      patch: |-
        - op: replace
          path: /spec/template/spec/containers/0/env/0
          value:
            name: SYMBOLS_CACHE_SIZE_MB
            value: "50000"
```

---

## External services

You can use an external or managed version of PostgreSQL and Redis with your Sourcegraph instance. For detailed information as well as the requirements for each service, please see our docs on [using external services with Sourcegraph](../../external_services/index.md).

### External Postgres

For optimal performance and resilience, it is recommended to use an external database when deploying Sourcegraph. For more information on database requirements, please refer to the [Postgres guide](../../postgres.md).

To connect Sourcegraph to an existing PostgreSQL instance, add the relevant environment variables ([such as PGHOST, PGPORT, PGUSER, etc.](http://www.postgresql.org/docs/current/static/libpq-envars.html)) to the frontend ConfigMap by adding the new environment variables to the end of the *FRONTEND ENV VARS* section at the bottom of your [kustomization file](kustomize/index.md#kustomizationyaml). For example:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/...
  configMapGenerator:
    - name: sourcegraph-frontend-env
      behavior: merge
      literals:
        - DEPLOY_TYPE=kustomize
        - PGHOST=NEW_PGHOST
        - PGPORT=NEW_PGPORT
```

> WARNING: You must restart frontend for the updated values to be activated

### External Redis

Sourcegraph supports specifying an external Redis server with these environment variables:

- **REDIS_CACHE_ENDPOINT**=[redis-cache:6379](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24++REDIS_CACHE_ENDPOINT+AND+REDIS_STORE_ENDPOINT+-file:doc+file:internal&patternType=literal) for caching information.
- **REDIS_STORE_ENDPOINT**=[redis-store:6379](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24++REDIS_CACHE_ENDPOINT+AND+REDIS_STORE_ENDPOINT+-file:doc+file:internal&patternType=literal) for storing information (session data and job queues).

When using an external Redis server, the corresponding environment variable must also be added to the following services:

<!-- Use ./dev/depgraph/depgraph summary internal/redispool to generate this -->

- `sourcegraph-frontend`
- `repo-updater`
- `gitserver`
- `searcher`
- `symbols`
- `worker`

**Step 1**: Include the `services/redis` component in your components:

This adds the new environment variables for redis to the services listed above.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/services/redis
```

**Step 2**: Set the following variable in your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    # [ACTION] Set values below
    REDIS_CACHE_ENDPOINT: REDIS_CACHE_DSN
    REDIS_STORE_ENDPOINT: REDIS_STORE_DSN
```

> WARNING: You must restart frontend for the updated values to be activated

### External Embeddings Object Storage

Sourcegraph supports specifying an external Object Store for embeddings indexes.

**Step 1**: Create a subdirectory called 'patches' within the directory of your overlay

```bash
$ mkdir instances/$INSTANCE_NAME/patches
```

**Step 2**: Copy the `embeddings-backend.yaml` patch file from the components/patches directory to the new [patches subdirectory](kustomize/index.md#patches-directory)

```bash
$ cp components/patches/embeddings-backend.yaml instances/$INSTANCE_NAME/patches/embeddings-backend.yaml
```

**Step 3**: Configure the external object store [backend](../../../cody/core-concepts/embeddings/manage-embeddings.md#store-embedding-indexes)

**Step 4**: Include the patch file in your overlay under `patches`:

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/...
  # ...
  patches:
    - patch: patches/embeddings-backend.yaml
  ```
---

## SSH for cloning

Sourcegraph will clone repositories using SSH credentials when the `id_rsa` and `known_hosts` files are mounted at `/home/sourcegraph/.ssh` (or `/root/.ssh` when the cluster is run by root users) in the `gitserver` deployment.

**WARNING:** Do not commit the actual `id_rsa` and `known_hosts` files to any public repository.

To mount the files through Kustomize:

**Step 1:** Copy the required files to the `configs` folder at the same level as your overylay's kustomization.yaml file

**Step 2:** Include the following in your overlay to [generate secrets](https://kubernetes.io/docs/tasks/configmap-secret/managing-secret-using-kustomize/) that base64 encoded the values in those files

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml > [SECRETS GENERATOR]
  secretGenerator:
  - name: gitserver-ssh
    files:
    - configs/id_rsa
    - configs/known_hosts
  ```

**Step 3:** Include the following component to mount the [secret as a volume](https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets-as-files-from-a-pod) in [gitserver.StatefulSet.yaml](https://github.com/sourcegraph/deploy-sourcegraph-k8s/blob/master/base/sourcegraph/gitserver/gitserver.StatefulSet.yaml).

  ```yaml
  # instances/$INSTANCE_NAME/kustomization.yaml
  components:
    # Enable SSH to clone repositories as non-root user (default)
    - ../../components/enable/ssh/non-root
    # Enable SSH to clone repositories as root user
    - ../../components/enable/ssh/root
  ```

> NOTE: If you are running Sourcegraph with privileges using our [privileged component](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/privileged), you are most likely running Sourcegraph with `root` access and should use the [ssh/root component](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-k8s/-/tree/components/enable/ssh/root) to enable SSH cloning.

**Step 4:** Update code host configuration

Update your [code host configuration file](../../external_service/index.md#full-code-host-docs) to enable ssh cloning. For example, set [gitURLType](../../external_service/github.md#gitURLType) to `ssh` for [GitHub](../../external_service/github.md). See the [external service docs](../../../admin/external_service/index.md) for the correct setting for your code host.

---

## Openshift

### Arbitrary users

Our Postgres databases can only be run with the UIDs defined by the upstream images, for example, [UID 70 and GID 70](https://sourcegraph.com/github.com/docker-library/postgres@master/-/blob/12/alpine/Dockerfile?L9-15#L4)--this could cause permission issues for clusters that run pods with arbitrary users. This can be resolved with the `utils/uid` component created based on one of the solutions suggested by Postgres on their [official docker page](https://hub.docker.com/_/postgres).

The `utils/uid` component bind-mount `/etc/passwd` as read-only through hostpath so that you can run the containers with valid users on your host.

```yaml
  components:
    - ../../components/utils/uid
```

---

## Add license key

Sourcegraph's Kubernetes deployment [requires an Enterprise license key](https://sourcegraph.com/pricing).

Once you have a license key, add it to your [site configuration](https://docs.sourcegraph.com/admin/patches/site_config).

---

## Filtering cAdvisor metrics

cAdvisor can pick up metrics for services unrelated to the Sourcegraph deployment running on the same nodes
([Learn more](../../../dev/background-information/observability/cadvisor.md#identifying-containers)) **when running with privileges**. To work around this:

1\. Create a subdirectory called 'patches' within the directory of your overlay

```bash
$ mkdir instances/$INSTANCE_NAME/patches
```

2\. Create a copy of the `prometheus.ConfigMap.yaml` file in the new [patches subdirectory](kustomize/index.md#patches-directory)

```bash
mv base/monitoring/prometheus/rbacs/prometheus.ConfigMap.yaml instances/$INSTANCE_NAME/patches/prometheus.ConfigMap.yaml
```

3\. In the copied ConfigMap file, add the following under `metric_relabel_configs` for the `kubernetes-pods` job:

```yaml
# instances/$INSTANCE_NAME/patches/prometheus.ConfigMap.yaml
  - job_name: 'kubernetes-pods'
    metric_relabel_configs:
      - source_labels: [container_label_io_kubernetes_pod_namespace]
        regex: ^$|ns-sourcegraph # ACTION: replace ns-sourcegraph with your namespace
        action: keep
```

Replace `ns-sourcegraph` with your namespace, e.g. `regex: ^$|your_namespace`.

4\. Add the patches to your overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  patches:
    - patch: patches/prometheus.ConfigMap.yaml
```

This will cause Prometheus to drop all metrics *from cAdvisor* that are not services running in the specified namespace.

---

## Private registry

**Step 1:** To update all image names with your private registry, e.g. `index.docker.io/sourcegraph/service_name` to `your.private.registry.com/sourcegraph/service_name`, include the `private-registry` component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/enable/private-registry
```

**Step 2:** Set the `PRIVATE_REGISTRY` variable in your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file. For example:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    PRIVATE_REGISTRY: your.private.registry.com # -- Replace 'your.private.registry.com'
```

### Add imagePullSecrets

To add `imagePullSecrets` to all resources:

**Step 1:** Include the `imagepullsecrets` component.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/resources/imagepullsecrets
```

**Step 2:** Set the `IMAGE_PULL_SECRET_NAME` variable in your [buildConfig.yaml](kustomize/index.md#buildconfig-yaml) file.

For example:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
  data:
    IMAGE_PULL_SECRET_NAME: YOUR_SECRET_NAME
```

**Alternative:** You can add the following patch under `patches`, and replace `YOUR_SECRET_NAME` with the name of your secret.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
patches:
  - patch: |-
      - op: add
        path: /spec/template/spec/imagePullSecrets
        value:
          name: YOUR_SECRET_NAME
    target:
      group: apps
      kind: StatefulSet|Deployment|DaemonSet
      version: v1
```

---

## Multi-version upgrade

In order to perform a [multi-version upgrade](../../updates/index.md#multi-version-upgrades), all pods must be scaled down to 0 except databases, which can be handled by including the `utils/multi-version-upgrade` component:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/utils/multi-version-upgrade
```

After completing the multi-version-upgrade process, exclude the component to allow the pods to scale back to their original number as defined in your overlay.

---

## Migrate from Privileged to Non-privileged

To migrate an existing deployment from root to non-root environment, you must first transfer ownership of all data directories to the specified non-root users for each service using the utils/migrate-to-nonprivileged component.

After transferring ownerships, you can redeploy the instance with non-privileged configurations.

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
  components:
    - ../../components/utils/migrate-to-nonprivileged
```

---

## Outbound Traffic

When working with an [Internet Gateway](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_Internet_Gateway.html) or VPC it may be necessary to expose ports for outbound network traffic. Sourcegraph must open port 443 for outbound traffic to code hosts, and to enable [telemetry](https://docs.sourcegraph.com/admin/pings) with Sourcegraph.com. Port 22 must also be opened to enable git SSH cloning by Sourcegraph. In addition, please make sure to apply other required changes to secure your cluster in a manner that meets your organization's security requirements.

---

## Troubleshooting

See the [Troubleshooting docs](troubleshoot.md).
