# Sourcegraph Machine Images

We aim to improve the overall deployment experience for our users through customized machine images.

All Sourcegraph image instances are deployed into a single K3s server cluster, running on a single node with an embedded SQLite Database. It allows us to package all the Sourcegraph services with necessary components into one single launcher image so that you can spin up a Sourcegraph instance with just a few clicks in less than 10 minutes.

This deployment method is highly recommended for customers who do not wish to spend too much time on looking for the right configurations and maintenance, while still having full control over their instances. The Sourcegraph image instances also provide high-availability and flexibility in resource usage, with the capability for scaling and making additional customizations easy whenever your needs have changed, by simply adjusting the worker/agent nodes, while still being on a single node environment. See the official K3s docs to learn more about [the architecture of a K3s server](https://docs.k3s.io/architecture). 

Most importantly, everything we use to build and publish the images can be found in our [public deployment repository](https://sourcegraph.com/github.com/sourcegraph/deploy) so that you can oversee our image creation and development process. You are also welcome to check out and follow our progress and updates there.

Our deployment and release process is also documented in the [deployment docs](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/doc/development.md). 

All Sourcegraph machine images are free to download, and we strongly encourage you to spin up a Sourcegraph AMI instance to experiment with. They are currently available in the following formats:

<div class="getting-started">
  <a class="btn btn-primary text-center" href="aws-ami"><span>AWS AMIs</span></a>
  <a class="btn btn-primary text-center" href="azure"><span>Azure Images</span></a>
  <a class="btn btn-primary text-center" href="gce"><span>Google Compute Images</span></a>
</div>

## Sourcegraph Machine Image Instance Overview

- Self-hosted
- Single node
- Preconfigured according to your business size
- Deployed with our [Helm Charts](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-helm) to a K3s, a highly available lightweight Kubernetes distribution cluster, using the containerd run time with built-in ingress, load balancer provider, and local storage provisioner ([Click here for the full K3s dependency list](https://sourcegraph.com/github.com/k3s-io/k3s#what-is-this))
- Easy to maintain and configure
- Supports all Sourcegraph features
- Ability to perform upgrades easily with or without SSH access

### Sourcegraph AWS AMI Instances

All AMI instances are currently pinned with a Sourcegraph version that the instance is launched with to ensure restarting the instance will not cause upgrades accidentally. The version number is saved into a text file on both the root (file path: /home/ec2-user/.sourcegraph-version) and data volumes (file path: /mnt/data/.sourcegraph-version) where it will be read by the [reboot script](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/reboot.sh) on each reboot. Upgrades will only happen on reboot if the version numbers from both volumes are different.

Detailed deployment and upgrade instructions can be found in our [AWS AMIs docs](https://docs.sourcegraph.com/admin/deploy/aws-ami). 

Unique AMI IDs can be found in our [release page](https://github.com/sourcegraph/deploy/releases).

### What is a Sourcegraph AMI Instance?

- Self-hosted
- Single node
- Preconfigured according to your business size
- Deployed with our [Helm Charts](https://sourcegraph.com/github.com/sourcegraph/deploy-sourcegraph-helm) to a K3s, a highly available lightweight Kubernetes distribution cluster, using the containerd run time with built-in ingress, load balancer provider, and local storage provisioner ([Click here for the full K3s dependency list](https://sourcegraph.com/github.com/k3s-io/k3s#what-is-this))
- Easy to maintain and configure
- Supports all Sourcegraph features
- Ability to perform upgrades easily with or without SSH access

#### Creation procress

Our AWS AMIs are all based on the HVM method, which provides us with the ability to create a Sourcegraph instance directly on the virtual machine using the verified Amazon Linux 2 Kernel 5.10 x86_64 HVM as the base image. The following steps are performed during the AMI creation process through our [install script](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/install.sh):

1. De-escalate to `ec2-user` to make sure tasks are performed by non-root user
1. Install Git
1. Clone the [deployment repository](https://github.com/sourcegraph/deploy)
1. Data volumes are formatted and labeled for the AMI instance to mount later
1. [Add configurations to the kernel](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/install.sh?L64-73) that would otherwise limit Sourcegraph search performance
1. [Adjust the local firewall settings](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/install.sh?L78-84) to ensure K3s cluster networking/DNS can pass through
1. Link the ephemeral kubelet/pod storage to our data disk
1. Link the persistent volume pod storage to our data disk
1. Link the K3s's embedded database to our data disk
1. Install K3s on root volume
1. Correct permission of the K3s kube config file located in `/etc/rancher/k3s/k3s.yaml`
1. Install Helm on root volume
1. Download Sourcegraph Helm Charts on root volume
1. Deploy Sourcegraph using the local Helm Charts
1. Save the version number to both root and data volumes
1. Add a cronjob to run the [reboot script](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/reboot.sh) on each reboot
1. K3s is stopped and disabled
1. The instance will then be stopped to create an AMI using the attached volumes

#### Data volumes

Each AWS AMI comes with two Amazon EBS volumes, one is for root, and the other one is for data:

- The root volume contains all the files inside the sourcegraph/deploy repository that are used to build that image and deploy the Sourcegraph instance that lives inside that specific image
  - File path to the deployment repo sourcegraph/deploy: `/home/ec2-user/deploy/`
  - A local copy of the helm charts that were used to create the AMI: `/home/ec2-user/deploy/install/sourcegraph-charts.tar`
  - Version number of the AMI Instance: `/mnt/data/.sourcegraph-version`
  - Create a copy of the kube config file from `/etc/rancher/k3s/k3s.yaml` allows you to manage the k3s cluster from outside the cluster
- The data volume is where all your Sourcegraph data will be stored after the instance has been launched. The K3s embedded SQLite database is also mounted onto that volume to make back-up, upgrade, and recovery of the volumes easier.
  - Data of your cluster are stored in the mounted path: `/mnt/data`
  - Version number of the deployment on disk: `/mnt/data/.sourcegraph-version`

Sourcegraph does not have access to your cluster and data.

#### Network and Security

<img class="screenshot w-100" src="https://user-images.githubusercontent.com/68532117/195904844-9257c7cd-f9b2-4d15-9c7f-a2d66a42c5df.png" alt="ami-diagram"/>

- K3s exposes the kubelet API to the Kubernetes control plane node through a websocket tunnel in order to eliminate the need to expose extra ports on the worker nodes.
- No ingress requirements which allow cluster owners to restrict inbound traffic to only traffic within their network
- Built-in certificate rotation with the expiration date of 12 months [docs](https://docs.k3s.io/advanced)
- Enable custom certificates through [etcdctl](https://docs.k3s.io/advanced#using-etcdctl) as recommended by K3s.
- Ability to launch an AMI instance on EC2 with custom encryption [docs](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIEncryption.html#AMI-encryption-launch).

Please refer to the official [CIS hardening guide](https://docs.k3s.io/security/hardening-guide) by K3s for more details and general tips on how to improve security of your cluster.

> NOTE: See [Sourcegraph Vulnerability Management Policy](https://handbook.sourcegraph.com/departments/engineering/dev/policies/vulnerability-management-policy/#vulnerability-service-level-agreements) to learn more about our vulnerability and patching policy as well as the current [vulnerability service level agreements](https://handbook.sourcegraph.com/departments/engineering/dev/policies/vulnerability-management-policy/#vulnerability-service-level-agreements). 


## Additional resources

- [sourcegraph/deploy](https://sourcegraph.com/github.com/sourcegraph/deploy)
- [Scripts used for building the AWS AMIs](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/install/install.sh)
- [Sourcegraph machine images deployment and release process](https://sourcegraph.com/github.com/sourcegraph/deploy@v4.0.1/-/blob/doc/development.md)
