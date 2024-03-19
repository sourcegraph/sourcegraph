# Installation Guide - Google Kubernetes Engine (GKE)

This section is aimed at providing high-level guidance on deploying Sourcegraph using a Kustomize overlay on GKE. 

## Overview

The installation guide below will walk you through deploying Sourcegraph on Google Kubernetes Engine (GKE) using our GKE example overlay.

The GKE overlay will:

- Deploy a Sourcegraph instance without RBAC resources 
- Create [BackendConfig](https://cloud.google.com/kubernetes-engine/docs/how-to/ingress-configuration#create_backendconfig) CRD. This is necessary to instruct the GCP load balancer on how to perform health checks on our deployment.
- Configure ingress to use [Container-native load balancing](https://cloud.google.com/kubernetes-engine/docs/how-to/container-native-load-balancing) to expose Sourcegraph publicly on a domain of your choosing and
- Create Storage Class to use [Compute Engine persistent disk](https://cloud.google.com/kubernetes-engine/docs/how-to/persistent-volumes/gce-pd-csi-driver).

## Prerequisites

- A running GKE cluster with the following configurations:
  - **Enable HTTP load balancing** in Networking
  - **SSD persistent disk** as book disk type 
- Minimum Kubernetes version: [v1.19](https://kubernetes.io/blog/2020/08/26/kubernetes-release-1.19-accentuate-the-paw-sitive/) with [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) v1.19 or later
- [Kustomize](https://kustomize.io/) (built into `kubectl` in version >= 1.14)

## Quick Start

You must complete **all** the prerequisites listed above before installing Sourcegraph with following steps.

### Step 1: Deploy Sourcegraph

Deploy Sourcegraph to your cluster:

```bash
$ kubectl apply --prune -l deploy=sourcegraph -k https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/gke
```

Monitor the deployment status to make sure everything is up and running:

```bash
kubectl get pods -n ns-sourcegraph -o wide --watch
```

### Step 2: Access Sourcegraph in Browser

To check the status of the load balancer and obtain its IP:

```bash
$ kubectl describe ingress sourcegraph-frontend -n ns-sourcegraph
```

From you output, look for the IP address of the load balancer, which is listed under `Address`.

```bash
# Sample output:
Name:             sourcegraph-frontend
Namespace:        default
Address:          12.345.678.0
```

Once the load balancer is ready, you can access your new Sourcegraph instance at the returned IP address in your browser via HTTP. Accessing the IP address with HTTPS returns errors because TLS must be enabled first.

It might take about 10 minutes for the load balancer to be fully ready. In the meantime, you can access Sourcegraph using the port forward method as described below.

#### Port forward

Forward the remote port so that you can access Sourcegraph without network configuration temporarily.

```bash
kubectl port-forward svc/sourcegraph-frontend 3080:30080 -n ns-sourcegraph
```

You should now be able to access your new Sourcegraph instance at http://localhost:3080  🎉

### Further configuration

The steps above have guided you to deploy Sourcegraph using the [examples/gke](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tree/master/examples/gke) overlay preconfigured by us.

If you would like to make other configurations to your existing instance, you can create a new overlay using its kustomization.yaml file shown below and build on top of it. For example, you can upgrade your instance from size XS to L, or add cAdvisor.

```yaml
# overlays/$INSTANCE_NAME/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ns-sourcegraph
resources:
  # Deploy Sourcegraph main stack
  - ../../base/sourcegraph
  # Deploy Sourcegraph monitoring stack
  - ../../base/monitoring
components:
  # Create a namespace for ns-sourcegraph
  - ../../components/resources/namespace
  # Use resources for a size-XS instance
  - ../../components/sizes/xs
  # Apply configurations for GKE
  - ../../components/clusters/gke/configure
```

#### Enable TLS

Once you have created a new overlay using the kustomization file from our examples overlay for gke, we strongly recommend you to: 
- create a DNS A record for your Sourcegraph instance domain
- enable TLS is highly recommended. 

If you would like to enable TLS with your own certificate, please read the [TLS configuration guide](../configure.md#tls) for detailed instructions.

##### Google-managed certificate

In order to use [Google-managed SSL certificates](https://cloud.google.com/kubernetes-engine/docs/how-to/managed-certs) to enable TLS:

Step 1: Add the `gke mange-cert` component to your overlay:

```yaml
# instances/$INSTANCE_NAME/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ns-sourcegraph
resources:
  - ../../base/sourcegraph
  - ../../base/monitoring
components:
  - ../../components/resources/namespace
  - ../../components/sizes/xs
  - ../../components/clusters/gke/configure
  - ../../components/clusters/gke/managed-cert
```

Step 2: Set the `GKE_MANAGED_CERT_NAME` variable with your Google-managed certificate name under the [BUILD CONFIGURATIONS](index.md#buildconfig-yaml) section:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
kind: SourcegraphBuildConfig
metadata:
  name: sourcegraph-kustomize-config
data:
  GKE_MANAGED_CERT_NAME: your-managed-cert-name
```

### Remote Build

You can also make a copy of this remote kustomization file locally and build on top of it.

```bash
$ kustomize create --resources https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/gke
```

You can also add remote components, but not components that required additional input in buildConfig.yaml.

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ns-sourcegraph
resources:
  - https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/gke
components:
  # Add components here
  - https://github.com/sourcegraph/deploy-sourcegraph-k8s/components/monitoring/cadvisor
  - https://github.com/sourcegraph/deploy-sourcegraph-k8s/components/monitoring/tracing

```
