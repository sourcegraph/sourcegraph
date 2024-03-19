# Installation Guide - Amazon Elastic Kubernetes Service (EKS)

This section is aimed at providing high-level guidance on deploying Sourcegraph using a Kustomize overlay on Amazon Elastic Kubernetes Service (EKS). 

## Overview

The installation instructions below will guide you through deploying Sourcegraph on Elastic Kubernetes Service (EKS) with our quick-start overlay.

The overlay will:

- Deploy a Sourcegraph instance without RBAC resources 
- Configure Ingress to use [AWS Load Balancer Controller](https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html) to expose Sourcegraph publicly on your domain
- Configure the Storage Class to use [AWS EBS CSI driver](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html) (installed as adds-on)

## Prerequisites

-  A EKS cluster (>=1.19) with the following addons enabled:
   - [AWS Load Balancer Controller](https://docs.aws.amazon.com/eks/latest/userguide/aws-load-balancer-controller.html)
   - [AWS EBS CSI driver](https://docs.aws.amazon.com/eks/latest/userguide/managing-ebs-csi.html)
- Minimum Kubernetes version: [v1.19](https://kubernetes.io/blog/2020/08/26/kubernetes-release-1.19-accentuate-the-paw-sitive/) with [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) v1.19 or later
- [Kustomize](https://kustomize.io/) (built into `kubectl` in version >= 1.14)

## Quick Start

You must complete **all** the prerequisites listed above before installing Sourcegraph with following steps.

### Step 1: Deploy Sourcegraph

Deploy Sourcegraph main app without the monitoring stacks to your cluster:

```bash
$ kubectl apply --prune -l deploy=sourcegraph -k https://github.com/sourcegraph/deploy-sourcegraph-k8s/examples/aws
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

Once the load balancer is ready, you can access your new Sourcegraph instance at the returned IP address in your browser via HTTP. Accessing the IP address with HTTPS will return errors because TLS must be enabled first.

It might take about 10 minutes for the load balancer to be fully ready. In the meantime, you can access Sourcegraph using the port forward method as described below.

#### Port forward

Forward the remote port so that you can access Sourcegraph without network configuration temporarily.

```bash
kubectl port-forward svc/sourcegraph-frontend 3080:30080 -n ns-sourcegraph
```

You should now be able to access your new Sourcegraph instance at http://localhost:3080  🎉

### Further configuration

The steps above have guided you to deploy Sourcegraph using the [quick-start/aws/eks](https://github.com/sourcegraph/deploy-sourcegraph-k8s/tree/master/examples/aws) overlay preconfigured by us.

If you would like to make other configurations to your existing instance, you can create a new overlay using its kustomization.yaml file shown below and build on top of it. For example, you can upgrade your instance from size XS to L, or add the monitoring stacks.

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
  # Use resources for a size-XS instance
  - ../../components/sizes/xs
  # Apply configurations for AWS EKS storage class and ALB
  - ../../components/clusters/aws/eks-ebs
```

#### Enable TLS

Once you have created a new overlay using the kustomization file from our quick-start overlay for AWS EKS, we strongly recommend that you:
- create a DNS A record for your Sourcegraph instance domain
- enable TLS is highly recommended. 

If you would like to enable TLS with your own certificate, please read the [TLS configuration guide](../configure.md#tls) for detailed instructions. 

##### AWS-managed certificate

In order to use a managed certificate from [AWS Certificate Manager](https://docs.aws.amazon.com/acm/latest/userguide/acm-overview.html) to enable TLS:

Step 1: Add the `aws/mange-cert` component to your overlay:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: ns-sourcegraph
resources:
  - ../../base/sourcegraph
  - ../../base/monitoring
components:
  - ../../components/resources/namespace
  - ../../components/sizes/xs
  - ../../components/clusters/aws/eks-ebs
  - ../../components/clusters/aws/managed-cert
```

Step 2: Set the `AWS_MANAGED_CERT_ARN` variable with the `ARN of your AWS-managed TLS certificate` under the [BUILD CONFIGURATIONS](index.md#buildconfig-yaml) section:

```yaml
# instances/$INSTANCE_NAME/buildConfig.yaml
kind: SourcegraphBuildConfig
metadata:
  name: sourcegraph-kustomize-config
data:
  # ARN of the AWS-managed TLS certificate
  AWS_MANAGED_CERT_ARN: arn:aws:acm:us-west-2:xxxxx:certificate/xxxxxxx
```
