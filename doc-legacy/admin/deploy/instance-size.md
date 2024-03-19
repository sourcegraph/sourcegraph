---
title: Instance Size
---

# Instance Size

When sizing your Sourcegraph deployment, it is important to take into account the number of users and repositories in your environment. Below is our sizing chart with our sizing recommendations for different size environments.

## Size chart

If you fall between two sizes, choose the larger of the two. For examples:

1. If you have 8,000 users with 80,000 repositories, your instance size would be **L**. 
2. If you have 1,000 users with 80,000 repositories, your instance size would still be **L**. 

|                  | **XS**        | **S**          | **M**          | **L**          | **XL**         |
|:-----------------|:-------------:|:--------------:|:--------------:|:--------------:|:--------------:|
| **Users**        | Up to 500     | Up to 1,000    | Up to 5,000    | Up to 10,000   | Up to 20,000   |
| **Repositories** | Up to 5,000   | Up to 10,000   | Up to 50,000   | Up to 100,000  | Up to 250,000  |
| **vCPU**         | 8             | 16             | 32             | 48             | 96             |
| **Memory (GB)**  | 32            | 64             | 128            | 192            | 384            |
| **SSD Required** | Yes           | Yes            | Yes            | Yes            | Yes            |

## Instance type

### Single Node

We recommend the following instance type for the cloud providera listed below.

|                  | **XS**        | **S**          | **M**          | **L**          | **XL**         |
|:-----------------|:-------------:|:--------------:|:--------------:|:--------------:|:--------------:|
| **AWS**          | m6a.2xlarge   | m6a.4xlarge    | m6a.8xlarge    | m6a.12xlarge   | m6a.24xlarge   |
| **Azure**        | D8_v3         | D16_v3         | D32_v3         | D48_v3         | D64_v3         |
| **GCP**          | n2-standard-8 | n2-standard-16 | n2-standard-32 | n2-standard-48 | n2-standard-96 |


### Kubernetes

> WARNING: If you intend to set this up as a production instance, we recommend you create the cluster in a VPC
> or other secure network that restricts unauthenticated access from the public Internet. You can later expose the
> necessary ports via an
> [Internet Gateway](http://docs.aws.amazon.com/AmazonVPC/latest/UserGuide/VPC_Internet_Gateway.html) or equivalent
> mechanism. Note that SG must expose port 443 for outbound traffic to codehosts and to enable [telemetry](https://docs.sourcegraph.com/admin/pings) with
> Sourcegraph.com. Additionally port 22 may be opened to enable git SSH cloning by Sourcegraph. Take care to secure your cluster in a manner that meets your
> organization's security requirements.

Follow the instructions linked in the table below to provision a Kubernetes cluster for the
infrastructure provider of your choice, using the recommended node and list types in the
table.

| **Provider**                       | **Node type**                   | **Boot/ephemeral disk size** |
|------------------------------------|---------------------------------|------------------------------|
|[Amazon EKS (better than plain EC2)](kubernetes/eks.md)| m5.4xlarge | 100 GB (SSD preferred) |
|[AWS EC2](https://kubernetes.io/docs/getting-started-guides/aws/)| m5.4xlarge |  100 GB (SSD preferred) |
|[Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs/quickstart)| n2-standard-16 | 100 GB (default) |
|[Azure](kubernetes/azure.md)| D16 v3 | 100 GB (SSD preferred) |
|[Other](https://kubernetes.io/docs/setup/production-environment/turnkey-solutions/)| 16 vCPU, 60 GiB memory per node | 100 GB (SSD preferred) |

<span class="virtual-br"></span>

> NOTE: Sourcegraph can run on any Kubernetes cluster, so if your infrastructure provider is not
> listed, see the "Other" row. Pull requests to add rows for more infrastructure providers are
> welcome!

<span class="virtual-br"></span>

> WARNING: If you are deploying on Azure, you **must** ensure that [your cluster is created with support for CSI storage drivers](https://docs.microsoft.com/en-us/azure/aks/csi-storage-drivers). This **can not** be enabled after the fact.

## Resources

Please refer to our [resource estimator](https://docs.sourcegraph.com/admin/deploy/resource_estimator) for more information regarding resources allocation for your Sourcegraph deployment.
