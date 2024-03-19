# Deploying Sourcegraph executors on Kubernetes (docker-in-docker)

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta and might change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

[Kubernetes manifests](https://github.com/sourcegraph/deploy-sourcegraph-k8s) are provided to deploy Sourcegraph Executors on a running Kubernetes cluster. If you are deploying Sourcegraph with helm, charts are available [here](https://github.com/sourcegraph/deploy-sourcegraph-helm).

## Deployment

Executors on kubernetes machines require privileged access to a container runtime daemon in order to operate correctly. In order to ensure maximum capability across Kubernetes versions and container runtimes, a [Docker in Docker](https://www.docker.com/blog/docker-can-now-run-within-docker/) sidecar is deployed with each executor pod to avoid accessing the host container runtime directly.

### Step-by-step Guide

Ensure you have the following tools installed:

- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl)
- [Helm](https://helm.sh/) if you're installing Sourcegraph with `helm`.

#### Deployment via Kustomize

Please refer to the [Sourcegraph Kustomize docs](https://docs.sourcegraph.com/admin/deploy/kubernetes/kustomize) for the latest instructions.

To include Executors dind, see [configure Sourcegraph with Kustomize](https://docs.sourcegraph.com/admin/deploy/kubernetes/configure) on how to specify the component.

#### Deployment via Helm

Please refer to the [Sourcegraph Helm docs](https://docs.sourcegraph.com/admin/deploy/kubernetes/helm#quickstart) for the latest instructions.

To specifically deploy Executors,
1. Create an overrides file, `override.yaml`, with any other customizations you may require.
   1. See [details on configurations](https://docs.sourcegraph.com/admin/deploy/kubernetes/helm#configuration).
2. Run the following command:
    ```bash
    helm upgrade --install --values ./override.yaml --version <your Sourcegraph Version> sg-executor sourcegraph/sourcegraph-executor-dind
    ```
3. Confirm executors are working by checking the _Executors_ page under **Site admin > Executors > Instances** .

## Note

Executors deployed in kubernetes do not use [Firecracker](index.md#how-it-works), meaning they require [privileged access](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/) to the docker daemon running in a sidecar alongside the executor pod.

If you have security concerns, consider deploying via [terraform](deploy_executors_terraform.md) or [installing the binary](deploy_executors_binary.md) directly.
