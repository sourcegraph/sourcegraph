# Sourcegraph with Kubernetes on Amazon EKS

> WARNING: This guide applies exclusively to a Kubernetes deployment **without** Helm.
> If you have not deployed Sourcegraph yet, it is higly recommended to use Helm as it simplifies the configuration and greatly simplifies the later upgrade process. See our guidance on [using Helm to deploy to Amazon EKS](helm.md#configure-sourcegraph-on-elastic-kubernetes-service-eks).

[Amazon EKS](https://aws.amazon.com/eks/) is Amazon's managed Kubernetes offering, similar to how Google Cloud offers managed Kubernetes clusters (GKE).

If your preferred cloud provider is Amazon, we strongly recommend using EKS instead of plain EC2. By using EKS, you will not need to manage your own Kubernetes control plane (complex). Instead, Amazon will provide it for you and you will only be responsible for managing the [NodeGroups](https://docs.aws.amazon.com/eks/latest/userguide/managed-node-groups.html) and the Sourcegraph deployment running on the Kubernetes cluster.

This guide will help you create a simple Kuberentes cluster using EKS, for more information or other advanced use-cases, please check [Amazon EKS documentation](https://docs.aws.amazon.com/eks/latest/userguide/getting-started.html).

## Requirements

The easiest way to get started with Amazon EKS is using the `eksctl`. This tool will create all the necesary resources to bootstrap a simple Kuberentes cluster in AWS.
You can find an extensive guide on using `eksctl` with Amazon EKS in the [EKS Getting Starrted guide](https://docs.aws.amazon.com/eks/latest/userguide/getting-started-eksctl.html).

Before moving forward, you will need:

- `awscli`: The command line tool provided by Amazon to work with AWS resources. [Installation guide](https://docs.aws.amazon.com/cli/latest/userguide/install-cliv2.html) and [Configuration guide](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-quickstart.html#cli-configure-quickstart-config).
- `kubectl`: The command line tool used for working with Kuberentes clusters and resources. [Installation guide](https://kubernetes.io/docs/tasks/tools/).
- `eksctl`: A tool provided by Waveworks for configuring and creating EKS clusters in Amazon. [Installation guide](https://eksctl.io/introduction/#installation).
- **IAM permissions**: The user that will be performing the installation must have permissions to work with Amazon EKS IAM roles and service linked roles, AWS CloudFormation, and a VPC and related resources.

Through this guide, we will be using a number of parameters starting with `$` that you will need to replace with your desired values:

- `$REGION`: The AWS region to use for all resources in this tutorial.
- `$KEY_NAME`: The name of the SSH key-pair in AWS used to access instances.
- `$CLUSTER_NAME`: A name to be given to the Amazon EKS Cluster.
- `$NODE_TYPE`: The instance type that will be used by the cluster NodeGroup.
- `$NODE_MAX`: The maximum number of nodes in your cluster.
- `$NODE_MIN`: The minimum number of nodes in your cluster.

## Sizing the cluster

Before getting started, we need to identify the size for our initial cluster. You can use the following chart as reference:

| Users    | `$NODE_TYPE`  | `$NODE_MIN` | `$NODE_MAX` | Cost est.    |
| -------- | ------------- | ----------- | ----------- | ------------ |
| 10-500   | m6a.4xlarge   | 3           | 6           | $59-118/day  |
| 500-2000 | m6a.4xlarge   | 6           | 10          | $118-195/day |

> **Note:** You can modify these values later on to scale up/down the number of worker nodes using the `eksctl` command line. For more information please the [eksctl documentation](https://eksctl.io/)

## Create an EC2 key-pair

> **Note:** If you already have an existing keypair you can skip this step and reuse your existing key

Creating an EC2 keypair will allow you to access the Kuberentes nodes after the setup is complete, which might be required in the future to perform troublshooting or administration tasks.

To create a key-pair in use the following command:

```bash
aws ec2 create-key-pair --region $REGION --key-name $KEY_NAME
```

## Create an Amazon EKS Cluster

We will leverage `eksctl` to create our cluster, as it automates all the steps necesary to get our EKS Cluster and its NodeGroup working.

> **Note:** This command might take 10-30 minutes to finish.

```bash
eksctl create cluster \
  --name $CLUSTER_NAME \
  --region $REGION \
  --with-oidc \
  --ssh-access \
  --ssh-public-key $KEY_NAME \
  --managed \
  --node-type $NODE_TYPE --nodes-min $NODE_MIN --nodes $NODE_MIN --nodes-max $NODE_MAX
```

## Access to the cluster

Before accessing the cluster, we need to configure `kubectl` to connect to the correct cluster, we can do this with `eksctl`:

```bash
eksctl --region $REGION utils write-kubeconfig --cluster $CLUSTER_NAME
```

At this point, running `kubectl get svc` should show something like:

```bash
$ kubectl get svc
NAME         TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
kubernetes   ClusterIP   172.20.0.1   <none>        443/TCP   4m
```

**Important**: If `kubectl` commands prompt you for username/password, make sure that `kubectl version` reports a client version of v1.10+. Older versions of kubectl do not work with the authentication configuration provided by Amazon EKS.

## Deploy the Kubernetes Web UI Dashboard (optional)

See [Tutorial: Deploy the Kubernetes Dashboard](https://docs.aws.amazon.com/eks/latest/userguide/dashboard-tutorial.html).

## Deploy Sourcegraph! 🎉

Your Kubernetes cluster is now all set up and running!

Luckily, deploying Sourcegraph on your cluster is much easier and quicker than the above steps. :)

Follow our [installation documentation](index.md) to continue.
