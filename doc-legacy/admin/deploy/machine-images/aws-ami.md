<style>
  .screenshot {
      min-width: 100%
  }
</style>

# Sourcegraph AWS AMI instances

Sourcegraph [Amazon Machine Images (AMIs)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instances-and-amis.html) allow you to quickly deploy a production-ready Sourcegraph instance tuned to your organization's scale in just a few clicks.

Following these docs will provision the following resources:

- An EC2 node running Sourcegraph
- A root EBS volume with 50GB of storage
- An additional EBS volume with 500GB of storage for storing code and search indices

### Instance size chart

Select an AMI according and instance type to the number of users and repositories you have using this table. If you fall between two sizes, choose the larger of the two.

For example, if you have 8,000 users with 80,000 repositories, your instance size would be **L**. If you have 1,000 users with 80,000 repositories, you should still go with size **M**.

|                  | **XS**     | **S**       | **M**       | **L**       | **XL**      |
|------------------|------------|-------------|-------------|-------------|-------------|
| **Users**        | _<=_ 500   | _<=_ 1,000  | _<=_ 5,000  | _<=_ 10,000 | _<=_ 20,000 |
| **Repositories** | _<=_ 5,000 | _<=_ 10,000 | _<=_ 50,000 | _<=_ 100,000| _<=_ 250,000|
| **Recommended Type**  |                                               m6a.2xlarge                                                |                     m6a.4xlarge                      |                     m6a.8xlarge                      |                                              m6a.12xlarge                                               |                                               m6a.24xlarge                                               |
| **Minimum Type**      |                                               m6a.2xlarge                                                |                     m6a.2xlarge                      |                     m6a.4xlarge                      |                                               m6a.8xlarge                                               |                                               m6a.12xlarge                                               |
| **AMIs List** | [size-XS AMIs](https://console.aws.amazon.com/ec2/v2/home#Images:visibility=public-images;imageName=Sourcegraph-XS) | [size-S AMIs](https://console.aws.amazon.com/ec2/v2/home#Images:visibility=public-images;imageName=Sourcegraph-S) | [size-M AMIs](https://console.aws.amazon.com/ec2/v2/home#Images:visibility=public-images;imageName=Sourcegraph-M) | [size-L AMIs](https://console.aws.amazon.com/ec2/v2/home#Images:visibility=public-images;imageName=Sourcegraph-L) | [size-XL AMIs](https://console.aws.amazon.com/ec2/v2/home#Images:visibility=public-images;imageName=Sourcegraph-XL) |

Click [here](https://github.com/sourcegraph/deploy#amazon-ec2-amis) to see the completed list of AMI IDs published in each region.

<span class="badge badge-critical">IMPORTANT</span> The default AMI user name is **ec2-user**.

> NOTE: AMIs are optimized for the specific set of resources provided by the instance type, please ensure you use the correct AMI for the associated EC2 instance type. You can [resize your EC2 instance anytime](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-resize.html), but your Sourcegraph AMI must match accordingly. If needed, follow the [upgrade steps](#upgrade) to switch to the correct AMI image that is optimized for your EC2 instance type.


---

## Deploy Sourcegraph

1. In the [instance size chart](#instance-size-chart), click the link for the AMI that matches your deployment size.
2. Choose **Launch instance from AMI**.
3. Name your instance.
4. Select an **instance type** according to [the sizing chart](#instance-size-chart).
5. **Key pair (login)**: Select or create a new Key Pair for connecting to your instance securely (this may be required in the event you need support).
6. **Network settings**: 
   - Under "Auto-assign public IP" select "Enable".
   - Select a **Security Group** for the instance, or create one with the following rules:
     - Allow SSH from Anywhere (port range 22, source 0.0.0.0/0, ::/0)
     - Allow HTTPS from the internet (port range 443, source 0.0.0.0/0, ::/0)
     - Allow HTTP traffic from the internet (port range 80, source 0.0.0.0/0, ::/0)
   - **NOTE**: If you do not wish to have HTTP/HTTPS exposed to the public internet, you may later choose to remove these rules so that all traffic routes through your AWS load balancer.
7. **Configure storage**:
   - Root Volume: 50GB
   - EBS Volume: 500GB - this should be at least 25-50% *more* than the size of all your repositories on disk (you may check your GitHub/BitBucket/GitLab instance's disk usage.)
8. Click **Launch instance**, and navigate to the public IP address in your browser. (Look for the IPv4 Public IP value in your EC2
   instance page under the Description panel.)

Once the instance has started, please allow ~5 minutes for Sourcegraph to initialize. During this time you may observe a `404 page not found` response.

To configure SSL, and lock down the instance from the public internet, see the [networking](#networking) section.

> NOTE: If you cannot access the Sourcegraph homepage after 10 minutes, please try reboot your instance.

### Executors
Executors are supported using [native kubernetes executors](../../../admin/executors/deploy_executors_kubernetes.md).

Executors support [auto-indexing](../../../code_navigation/explanations/auto_indexing.md) and [server-side batch changes](../../../batch_changes/explanations/server_side.md).

To enable executors you must do the following:
1. Connect to the AMI instance using `ssh`
2. Run `cd /home/ec2-user/deploy/install/`
3. Replace the placeholder `executor.frontendPassword` in `override.yaml`
4. Run the following command to update the executor
```
helm upgrade -i -f ./override.yaml --version "$(cat /home/ec2-user/.sourcegraph-version)" executor ./sourcegraph-executor-k8s-charts.tgz
```
5. Add the following to the site-admin config using the password you chose previously
```
"executors.accessToken": "<exector.frontendPassword>",
"executors.frontendURL": "http://sourcegraph-frontend:30080",
"codeIntelAutoIndexing.enabled": true
```
6. Check `Site-Admin > Executors > Instances` to verify the executor connected successfully. If it does not appear try reboot the instance

To use server-side batch changes you will need to enable the `native-ssbc-execution` [feature flag](../../../admin/executors/native_execution.md#enable).

---

## Networking

We suggest using an AWS Application Load Balancer (ALB) to manage HTTPS connections to Sourcegraph. This makes managing SSL certificates easy.

### Creating an AWS Load Balancer

> NOTE: You must own a domain name before you can proceed with the following steps.

1. Request a certificate for the domain name in [AWS Certificate Manager](https://aws.amazon.com/certificate-manager/)
2. Create a [target group](https://console.aws.amazon.com/ec2#TargetGroups) for `HTTPS Port 443` that links to the instance's `Port 443`
3. Create a new subnet inside the instance VPC
4. Create a new Application Load Balancer via [AWS Load Balancers](https://console.aws.amazon.com/ec2#LoadBalancers)

#### Step 1: Request certificate

<img class="screenshot" src="https://user-images.githubusercontent.com/68532117/192369850-e90d1078-7ad6-4624-acc1-db093ef4d642.png">

Open the [AWS Certificate Manager console](https://console.aws.amazon.com/acm) to **Request a certificate**:

- **Domain names**: Fully qualified domain name: your domain
- **Select validation method**: DNS validation—recommended
 
After the certificate has been created, you will need to attach the `CNAME name` and `CNAME values` to your DNS.

Follow the steps below to attach the CNAME to your DNS if your DNS is hosted in [AWS route 53](https://console.aws.amazon.com/route53):

1. Click **Create record in route 53** in the certificate dashboard
2. Select the DNS you would like to attach the certificate to
3. Click **Create records** once you have verified the information is correct
4. Wait ~30 mins before the validation is completed

#### Step 2: Create a target group

1. Click **Create a target group** on your [EC2 Target groups dashboard](https://console.aws.amazon.com/ec2#TargetGroups)
   - Choose a target type: Instance
   - Target group name: name
   - Protocol: HTTPS
   - Port: 443
   - VPC: Select the VPC where your instance is located
   - Protocol version: HTTP2
   - Health checks: Use Default
2. Click **Include as pending below**

#### Step 3: Create subnets

Click **Create subnet** in your VPC subnets dashboard:

- **VPC ID**: Selected the VPC that the instance is in
- **Subnet name**: name the subnet
- **Availability Zone**: select an availability zone that is different from the current zone
- Click **Create subnet**

#### Step 4: Create an Application Load Balancer

1. Open your [EC2 Load Balancers dashboard](https://console.aws.amazon.com/ec2#LoadBalancers) to **Create Load Balancer**. 
2. Choose **Application Load Balancer** as the Load balancer types using the following configurations:

- **Basic configuration**
  - Load balancer name: name
  - Scheme: Internet-facing
  - IP address type: IPv4
- **Network mapping**
  - VPC: Selected the VPC that the instance is in
  - Mapping: Select two subnets associated with the selected VPC
  - Security groups
  - Security groups: Make sure only the security group associated with the instance is selected
- **Listeners and routing**
  - Protocol: HTTPS
  - Port: 443
  - Default action: Select the HTTPS target group created for the instance

### Securing your instance

[Configure user authentication](../../../admin/auth/index.md) (SSO, SAML, OpenID Connect, etc.) to give users of your Sourcegraph instance access to it.

Now that your instance is confirmed to be working, and you have HTTPS working through an Amazon load balancer, you may choose to secure your Sourcegraph instance further by modifying the security group/firewall rules to prevent access from the public internet. You can do this by modifying the security group/firewall rules.

---

## Upgrade

> WARNING: This upgrade process works with **Sourcegraph AWS AMI instances only**. Do not use these if you deployed Sourcegraph through other means.

Please take time to review the following before proceeding with the upgrades:

- [Changelog](https://docs.sourcegraph.com/CHANGELOG)
- [Update policy](https://docs.sourcegraph.com/admin/updates#update-policy)
- [Update notes](https://docs.sourcegraph.com/admin/updates/kubernetes)
- [Multi-version upgrade procedure](https://docs.sourcegraph.com/admin/updates/kubernetes#multi-version-upgrade-procedure)

<span class="badge badge-critical">IMPORTANT</span> **Back up your volumes before each upgrade**

#### Step 1: Stop the current instance

1. Stop your current Sourcegraph AMI instance
   - Go to the ECS console for your instance
   - Click Instance State to Stop Instance
2. Detach the non-root data volume (Device name: /dev/sdb/)
   - Go to the Storage section in your instance console
   - Find the volume with the device name **/dev/sdb**
   - Select the volume, then click Actions to **Detach Volume**
   - Give the volume a name for identification purposes
3. Make a note of the VPC name

#### Step 2: Launch a new instance

1. Launch a new Sourcegraph instance from an AMI with the latest version of Sourcegraph
2. Name the instance
3. Select the appropriate **instance type**
4. Under **Key Pair**
  - Select the **Key Pair** used by the old instance
5. Under **Network settings**
   - Select the **Security Group** used by the old instance
6. Under **Configure storage**
 - Remove the **second** EBS volume
7. After reviewing the settings, click **Launch Instance**
8. Attach the detached volume to the new instance
   - Go to the Volumes section in your ECS Console
   - Select the volume you've detached earlier
   - Click **Actions > Attach Volume**
9. On the `Attach volume` page:
  - **Instance**: select the new Sourcegraph AMI instance
  - **Device name**: /dev/sdb
10. **Reboot** the new instance

You can terminate the stopped Sourcegraph AMI instance once you have confirmed the new instance is up and running.

## Downgrade

Please refer to the upgrade procedure above if you wish to rollback your instance. 

---

## Storage and Backups

We strongly recommend you taking [snapshots of the entire EBS volume](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-creating-snapshot.html) on an [automatic, scheduled basis](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/snapshot-lifecycle.html).

## Additional resources

- [Increase the size of an Amazon EBS volume on an EC2 instance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/modify-ebs-volume-on-instance.html)
- [Change the instance type](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-resize.html)
