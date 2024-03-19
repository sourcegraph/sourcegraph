# Deploying Sourcegraph executors natively on Kubernetes

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta and might change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

> NOTE: This feature is available in Sourcegraph 5.2.5 and later.

[Kubernetes manifests](https://github.com/sourcegraph/deploy-sourcegraph-k8s) are provided to deploy Sourcegraph Executors on a running Kubernetes cluster. If you are deploying Sourcegraph with helm, charts are available [here](https://github.com/sourcegraph/deploy-sourcegraph-helm).

## Requirements

### Feature flag
To instruct Sourcegraph to use Kubernetes-deployed executors, you will need to enable the `native-ssbc-execution` [feature flag](./native_execution.md#enable).

### RBAC Roles

Executors interact with the Kubernetes API to handle jobs. The following are the RBAC Roles needed to run Executors on
Kubernetes.

| API Groups | Resources          | Verbs                     | Reason                                                                                    |
|------------|--------------------|---------------------------|-------------------------------------------------------------------------------------------|
| `batch`    | `jobs`             | `create`, `delete`        | Executors create Job pods to run processes. Once Jobs are completed, they are cleaned up. |
|            | `pods`, `pods/log` | `get`, `list`, `watch`    | Executors need to look up and steam logs from the Job Pods.                               |

<!-- 

Additional RBAC Roles are needed for single pod + pvc executors. Hidden for now until 5.2.

| API Groups | Resources                | Verbs                     | Reason                                                                                    |
|------------|--------------------------|---------------------------|-------------------------------------------------------------------------------------------|
|            | `secrets`                | `create`, `delete`        | Executors need to create a token secret used for by each pod.                             |
|            | `persistentvolumeclaims` | `create`, `delete`        | When using PVC instead of `emptyDir` for Jobs, Executors need the ability to create PVCs. |


-->

See
the [example Role YAML](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/cmd/executor/kubernetes/batches/executor-batches.Role.yml)
for details.

### Docker Image

The Executor Docker image is available on Docker Hub
at [`sourcegraph/executor-kubernetes`](https://hub.docker.com/r/sourcegraph/executor-kubernetes/tags).

### Environment Variables

The following are Environment Variables that are specific to the Kubernetes runtime. These environment variables can be
set on the Executor `Deployment` and will configure the `Job`s that it spawns.

| Name                                                         | Default Value     | Description                                                                                                                                                                                            |
|--------------------------------------------------------------|:------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| EXECUTOR_KUBERNETES_CONFIG_PATH                              | N/A               | The path to the Kubernetes configuration file. If not specified, the in cluster config is used.                                                                                                        |
| EXECUTOR_KUBERNETES_NODE_NAME                                | N/A               | The name of the Kubernetes Node to create Jobs in. If not specified, the Pods are created in the first available node.                                                                                 |
| EXECUTOR_KUBERNETES_NODE_SELECTOR                            | N/A               | A comma separated list of values to use as a node selector for Kubernetes Jobs. e.g. `foo=bar,app=my-app`                                                                                              |
| EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS | N/A               | The JSON encoded required affinity match expressions for Kubernetes Jobs. e.g. `[{"key": "foo", "operator": "In", "values": ["bar"]}]`                                                                 |
| EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS      | N/A               | The JSON encoded required affinity match fields for Kubernetes Jobs. e.g. `[{"key": "foo", "operator": "In", "values": ["bar"]}]`                                                                      |
| EXECUTOR_KUBERNETES_POD_AFFINITY                             | N/A               | The JSON encoded pod affinity for Kubernetes Jobs. e.g. [{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}]      |
| EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY                        | N/A               | The JSON encoded pod anti-affinity for Kubernetes Jobs. e.g. [{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}] |
| EXECUTOR_KUBERNETES_NODE_TOLERATIONS                         | N/A               | The JSON encoded tolerations for Kubernetes Jobs. e.g. [{"key": "foo", "operator": "Equal", "value": "bar", "effect": "NoSchedule"}]                                                                   |
| EXECUTOR_KUBERNETES_NAMESPACE                                | `default`         | The namespace to create the Jobs in.                                                                                                                                                                   |
| EXECUTOR_KUBERNETES_PERSISTENCE_VOLUME_NAME                  | `sg-executor-pvc` | The name of the Executor Persistence Volume. Must match the `PersistentVolumeClaim` configured for the instance.                                                                                       |
| EXECUTOR_KUBERNETES_RESOURCE_LIMIT_CPU                       | N/A               | The maximum CPU resource for Kubernetes Jobs.                                                                                                                                                          |
| EXECUTOR_KUBERNETES_RESOURCE_LIMIT_MEMORY                    | `12Gi`            | The maximum memory resource for Kubernetes Jobs.                                                                                                                                                       |
| EXECUTOR_KUBERNETES_RESOURCE_REQUEST_CPU                     | N/A               | The minimum CPU resource for Kubernetes Jobs.                                                                                                                                                          |
| EXECUTOR_KUBERNETES_RESOURCE_REQUEST_MEMORY                  | `12Gi`            | The minimum memory resource for Kubernetes Jobs.                                                                                                                                                       |
| KUBERNETES_JOB_DEADLINE                                      | `1200`            | The number of seconds after which a Kubernetes job will be terminated.                                                                                                                                 |
| KUBERNETES_RUN_AS_USER                                       | N/A               | The user ID to run Kubernetes jobs as.                                                                                                                                                                 |
| KUBERNETES_RUN_AS_GROUP                                      | N/A               | The group ID to run Kubernetes jobs as.                                                                                                                                                                |
| KUBERNETES_FS_GROUP                                          | `1000`            | The group ID to run all containers in the Kubernetes jobs as.                                                                                                                                          |
| KUBERNETES_KEEP_JOBS                                         | `false`           | If true, Kubernetes jobs will not be deleted after they complete. Useful for debugging.                                                                                                                |
| KUBERNETES_JOB_ANNOTATIONS                                   | N/A               | The JSON encoded annotations to add to the Kubernetes Jobs. e.g. `{"foo": "bar", "faz": "baz"}`                                                                                                        |
| KUBERNETES_JOB_POD_ANNOTATIONS                               | N/A               | The JSON encoded annotations to add to the Kubernetes Job Pods. e.g. `{"foo": "bar", "faz": "baz"}`                                                                                                    |
| KUBERNETES_IMAGE_PULL_SECRETS                                | N/A               | The names of Kubernetes image pull secrets to use for pulling images. e.g. my-secret,my-other-secret                                                                                                   |
> Note: `EXECUTOR_KUBERNETES_NAMESPACE` should be set to either "default" or the specific namespace where your Executor is deployed.

<!--

Additional Environment Variables are needed for single pod + pvc executors. Hidden for now until 5.2 (some of these may be removed by then).

| Name                                    | Default Value                        | Description                                                                                                            |
|-----------------------------------------|:-------------------------------------|------------------------------------------------------------------------------------------------------------------------|
| KUBERNETES_SINGLE_JOB_POD               | `false`                              | Determine if a single Job Pod should be used to process a workspace.                                                   |
| KUBERNETES_JOB_VOLUME_TYPE              | `emptyDir`                           | Determines the type of volume to use with the single job. Options are 'emptyDir' and 'pvc'.                            |
| KUBERNETES_JOB_VOLUME_SIZE              | `5Gi`                                | Determines the size of the job volume.                                                                                 |
| KUBERNETES_ADDITIONAL_JOB_VOLUMES       | N/A                                  | Additional volumes to associate with the Jobs. e.g. `[{"name": "my-volume", "configMap": {"name": "cluster-volume"}}]` |
| KUBERNETES_ADDITIONAL_JOB_VOLUME_MOUNTS | N/A                                  | Volumes to mount to the Jobs. e.g. `[{"name":"my-volume", "mountPath":"/foo/bar"}]`                                    |
| KUBERNETES_SINGLE_JOB_STEP_IMAGE        | `sourcegraph/batcheshelper:insiders` | The image to use for intermediate steps in the single job. Defaults to `sourcegraph/batcheshelper:latest`.               |

-->

See other possible Environment Variables [here](./deploy_executors_binary.md#step-2-setup-environment-variables).

> Note: `executor.frontendUrl` must be set in the Site configuration for Executors to work correctly.

### Job Scheduling

> Note: Kubernetes has a max of 110 pods per node. If you run into this limit, you can lower the number of Job Pods running on a node by setting the environment variable `EXECUTOR_MAXIMUM_NUM_JOBS`.

Executors deployed on Kubernetes require Jobs to be scheduled on the same Node as the Executor. This is to ensure that
Jobs are able to access the same Persistence Volume as the Executor.

To ensure that Jobs are scheduled on the same Node as the Executor, the following environment variables can be set,

- `EXECUTOR_KUBERNETES_NODE_NAME`
- `EXECUTOR_KUBERNETES_NODE_SELECTOR`
- `EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS`
- `EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS`

#### Node Name

Using the [Downward API](https://kubernetes.io/docs/concepts/workloads/pods/downward-api/#downwardapi-fieldRef), the 
property `spec.nodeName` can be used to set the `EXECUTOR_KUBERNETES_NODE_NAME` environment variable.

```yaml
    - name: EXECUTOR_KUBERNETES_NODE_NAME
      valueFrom:
        fieldRef:
          fieldPath: spec.nodeName
```

This ensures that the Job is scheduled on the same Node as the Executor.

However, if the node does not have enough resources to run the Job, the Job will not be scheduled.

### Firewall Rules

The following are Firewall rules that are _highly recommended_ when running Executors on Kubernetes in a Cloud
Environment.

- Disable access to internal resources.
- Disable access to `5.2.5.254` (AWS / GCP Instance Metadata Service).

### Batch Changes

To run [Batch Changes](../../batch_changes/index.md) on
Kubernetes, [Native Server-Side Batch Changes](./native_execution.md) must be enabled.

### Example Configuration YAML

See
the [local development YAMLs](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/cmd/executor/kubernetes)
for an example of how to configure the Executor in Kubernetes.

## Deployment

Executors on Kubernetes require specific RBAC rules to be configured in order to operate correctly.
See [RBAC Roles](#rbac-roles) for more information.

### Step-by-step Guide

Ensure you have the following tools installed:

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [Helm](https://helm.sh/) if you're installing Sourcegraph with `helm`.

#### Deployment via Kustomize

Please refer to the [Sourcegraph Kustomize docs](https://docs.sourcegraph.com/admin/deploy/kubernetes/kustomize) for the latest instructions.

To include Native Kubernetes Executors, see [configure Sourcegraph with Kustomize](https://docs.sourcegraph.com/admin/deploy/kubernetes/configure) on how to specify the component (`components/executors/k8s`).

#### Deployment via Helm

Please refer to the [Sourcegraph Helm docs](https://docs.sourcegraph.com/admin/deploy/kubernetes/helm#quickstart) for the latest instructions.

To specifically deploy Executors,
1. Create an overrides file, `override.yaml`, with any other customizations you may require.
    1. See [details on configurations](https://docs.sourcegraph.com/admin/deploy/kubernetes/helm#configuration).
2. Run the following command:
    ```bash
    helm upgrade --install --values ./override.yaml --version <your Sourcegraph Version> sg-executor sourcegraph/sourcegraph-executor-k8s
    ```
3. Confirm executors are working by checking the _Executors_ page under **Site admin > Executors > Instances** .

## Note

Executors deployed on Kubernetes do not use [Firecracker](./index.md#how-it-works).

If you have security concerns, consider deploying via [terraform](deploy_executors_terraform.md)
or [installing the binary](deploy_executors_binary.md) directly.
