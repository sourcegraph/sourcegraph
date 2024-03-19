# Deploying Sourcegraph executors using Terraform on AWS or GCP

[Terraform modules](https://learn.hashicorp.com/tutorials/terraform/module-use?in=terraform/modules) are provided to
provision machines running executors on [AWS](https://sourcegraph.com/github.com/sourcegraph/terraform-aws-executors)
and [Google Cloud](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors).

## Basic Definition

The following is the minimum required definition to deploy an executor.

```terraform
module "executors" {
  source = "sourcegraph/executors/<aws | google>"

  # Find the latest version matching your Sourcegraph version here:
  # - https://github.com/sourcegraph/terraform-google-executors/tags
  # - https://github.com/sourcegraph/terraform-aws-executors/tags
  version = "<version>"

  # AWS specific
  availability_zone = "<availability zone to provision resource in AWS>"
  # Google specific
  region            = "<region to provision in GCP>"
  zone              = "<zone to provision resource in GCP>"

  executor_sourcegraph_external_url            = "<external url>"
  executor_sourcegraph_executor_proxy_password = "<shared secret>"
              
  # Either:
  executor_queue_name                          = "<codeintel | batches>"
  # Or:
  executor_queue_names                         = "codeintel,batches"
                
  executor_instance_tag                        = "<tag to filter in stackdriver monitoring>"
  executor_metrics_environment_label           = "<label to filter custom metrics>"
}
```

| Variable                                        | Description                                                                                                                                                                                                                                                           |
|------------------------------------------------ |-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `availability_zone`                             | The **AWS** availability zone to create the instance in                                                                                                                                                                                                               |
| `region`                                        | The **Google** region to provision the executor resources in.                                                                                                                                                                                                         |
| `zone`                                          | The **Google** zone to provision the executor resources in.                                                                                                                                                                                                           |
| `executor_sourcegraph_external_url`             | The public URL of your Sourcegraph instance. This corresponds to the `externalURL` value in your Sourcegraph instance’s site configuration and must be resolvable from the provisioned executor compute resources.                                                    |
| `executor_sourcegraph_executor_proxy_password`  | The access token corresponding to the `executors.accessToken` in your Sourcegraph instance's site configuration.                                                                                                                                                      |
| `executor_queue_name`                           | The single queue from which the executor should pull jobs - [`codeintel`](../../code_navigation/explanations/auto_indexing.md) or [`batches`](../../batch_changes/explanations/server_side.md). Either this or `executor_queue_names` must be set.                    |
| `executor_queue_names`                          | The multiple queues from which the executor should pull jobs - one or more of [`codeintel`](../../code_navigation/explanations/auto_indexing.md) and [`batches`](../../batch_changes/explanations/server_side.md). Either this or `executor_queue_name` must be set.  |
| `executor_instance_tag`                         | A label tag to add to all the executors; can be used for filtering out the right instances in stackdriver monitoring                                                                                                                                                  |
| `executor_metrics_environment_label`            | The value for environment by which to filter the custom metrics.                                                                                                                                                                                                      |

See the Terraform Modules for additional configurations.

- [AWS](https://sourcegraph.com/github.com/sourcegraph/terraform-aws-executors/-/blob/modules/executors/variables.tf)
- [Google](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors/-/blob/modules/executors/variables.tf)

## Terraform Version

Terraform modules `4.2.x` and above allow Terraform from `1.1.x` to `< 2.x` to be used.

If using a Terraform module `4.1.x` or below, use [tfenv](https://sourcegraph.com/github.com/tfutils/tfenv) to install Terraform
1.1+.

```shell
tfenv install 1.1.9
tfenv use 1.1.9
```

## Permissions

In order to provision executor resources, specific permissions must be granted.

### AWS

Access to get and create in the following resources.

- Auto Scaling
- CloudWatch Logs
- EBS (EC2)
- EC2 (Elastic Compute Cloud)
- IAM (Identity & Access Management)
- VPC (Virtual Private Cloud)

### Google

Ensure the [IAM API](https://console.cloud.google.com/apis/api/iam.googleapis.com/overview) is enabled.

- appengine.applications.get
- clientauthconfig.brands.*
- clientauthconfig.clients.*
- cloudasset.assets.searchAllResources
- cloudnotifications.activities.list
- cloudtrace.insights.get
- cloudtrace.insights.list
- cloudtrace.stats.get
- cloudtrace.tasks.*
- cloudtrace.traces.list
- compute.addresses.*
- compute.autoscalers.*
- compute.disks.*
- compute.firewalls.*
- compute.globalOperations.get
- compute.instanceGroupManagers.*
- compute.instanceGroups.create
- compute.instances.*
- compute.instanceTemplates.*
- compute.networks.*
- compute.regionOperations.get
- compute.subnetworks.*
- compute.zoneOperations.get
- compute.zones.get
- container.clusters.list
- iam.roles.*
- iam.serviceAccountKeys.*
- logging.logEntries.list
- logging.privateLogEntries.list
- monitoring.timeSeries.list
- oauthconfig.testusers.update
- oauthconfig.verification.update
- orgpolicy.policy.get
- resourcemanager.projects.*
- secretmanager.locations.list
- secretmanager.secrets.*
- secretmanager.versions.*

## Supported Regions

### AWS

- `us-east-1`
- `us-east-2`
- `us-west-1`
- `us-west-2`
- `eu-west-1`
- `eu-west-2`
- `eu-west-3`
- `eu-north-1`
- `eu-south-1`
- `eu-central-1`
- `ap-northeast-1`
- `ap-northeast-2`
- `ap-southeast-1`
- `ap-southeast-2`
- `ap-southeast-3`
- `ap-east-1`
- `ap-south-1`
- `sa-east-1`
- `me-south-1`
- `af-south-1`
- `ca-central-1`

### Google

All regions are supported.

## Examples

### Single Executor

The following examples provision a single executor to pull from the `codeintel` queue.

- [AWS example](https://sourcegraph.com/github.com/sourcegraph/terraform-aws-executors/-/tree/examples/single-executor)
- [Google example](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors/-/tree/examples/single-executor)
                    
### Multiple Executors

The following examples provision two executors, one to pull from the `codeintel` queue and the other for the `batches`
queue.

- [AWS example](https://sourcegraph.com/github.com/sourcegraph/terraform-aws-executors/-/tree/examples/multiple-executors)
- [Google example](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors/-/tree/examples/multiple-executors)

### Step-by-step Guide

The following is a step-by-step guide on provisioning a single `codeintel` executor on GCP.

#### Provision

1. [Install Terraform](#terraform-version).
2. Install the [`gcloud CLI`](https://cloud.google.com/sdk/docs/install)
3. Run `gcloud auth application-default login`
4. Set up your Sourcegraph instance's Site configuration for executors
    1. Click on your profile picture in the top right corner
    2. Select **Site admin**
    3. Example the **Configuration** section
    4. Select **Site configuration**
    5. Set the following,
        - `"externalURL": "<URL>"`
            - A URL that is accessible from the GCP VM (e.g. a public URL such as `https://sourcegraph.example.com`)
        - `"executors.accessToken": "<new long secret>"`
            - Can be generated by running `cat /dev/random | base64 | head -c 20`
            - The secret will be as displayed `REDACTED` once it's saved. 
        - `"codeIntelAutoIndexing.enabled": true`
            - *This is only for `codeintel` executors.*
5. Download
   the [example files](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors/-/blob/examples/single-executor)
6. Change the following in `providers.tf`
    - `project` to the GCP project to provision the executor in
    - `region` to the GCP region to provision the executor in
    - `zone` to the GCP zone to provision the executor in
7. Change the following in `main.tf`
    - `executor_sourcegraph_external_url` to the URL configured in your instance's **Site configuration**
    - `executor_sourcegraph_executor_proxy_password` to the access token configured in your instance's **Site
      configuration**
8. Run `terraform init` to download the Sourcegraph executor modules.
9. Run `terraform plan` to preview the changes that will occur to your GCP infrastructure.
10. Run `terraform apply` and enter "yes" after reviewing the proposed changes to create the executor VM
    - Ensure `terraform apply` exited with code 0 and did not print any errors
11. Go back to the site admin page, expand **Executors**, click **Instances**, and check to see if your executor shows
    up in the list with a green dot 🟢

#### Validation

The following can be done to troubleshoot or double-check that the executor has been properly provisioned.

Ensure the executor is listed in the Compute Engine. Either go to **Compute Engine** in the GCP Console for your project
or run the following command.

```shell
$ gcloud compute instances list
NAME                                          ZONE           MACHINE_TYPE   PREEMPTIBLE  INTERNAL_IP  EXTERNAL_IP    STATUS
sourcegraph-executor-h0rv                     us-central1-c  n1-standard-4               10.0.1.16                   RUNNING
sourcegraph-executors-docker-registry-mirror  us-central1-c  n1-standard-2               10.0.1.2                    RUNNING
```

You can ssh into to the instance to ensure the service is running. You can open an ssh connection either via the GCP
Console or by running the following command.

```shell
gcloud compute ssh sourcegraph-executor-h0rv
```

Then run the following command to check if the service is running.

```shell
you@sourcegraph-executor-h0rv:~$ systemctl status executor
🟢 executor.service - User code executor
     Loaded: loaded (/etc/systemd/system/executor.service; enabled; vendor preset: enabled)
     Active: active (running) since Thu 2021-11-18 02:28:48 UTC; 19s ago
```

To check the logs, you can either query the **Log Explorer** in the GCP Console or by running the following command
while connected to the instance.

```shell
you@sourcegraph-executor-h0rv:~$ journalctl -u executor | less
Nov 18 02:31:01 sourcegraph-executor-h0rv executor[2465]: t=2021-11-18T02:31:01+0000 lvl=dbug msg="TRACE internal" host=... path=/.executors/queue/codeintel/dequeue code=204 duration=92.131237ms
Nov 18 02:31:01 sourcegraph-executor-h0rv executor[2465]: t=2021-11-18T02:31:01+0000 lvl=dbug msg="TRACE internal" host=... path=/.executors/queue/codeintel/canceled code=200 duration=90.630467ms
Nov 18 02:31:02 sourcegraph-executor-h0rv executor[2465]: t=2021-11-18T02:31:02+0000 lvl=dbug msg="TRACE internal" host=... path=/.executors/queue/codeintel/dequeue code=204 duration=91.269106ms
Nov 18 02:31:02 sourcegraph-executor-h0rv executor[2465]: t=2021-11-18T02:31:02+0000 lvl=dbug msg="TRACE internal" host=... path=/.executors/queue/codeintel/canceled code=200 duration=161.469685ms
```

Ensure the `EXECUTOR_FRONTEND_URL` and `EXECUTOR_FRONTEND_PASSWORD` in `/etc/systemd/system/executor.env` are correct

```
cat /etc/systemd/system/executor.env
```

Ensure the VM can hit your `externalURL`:

```shell
you@sourcegraph-executor-h0rv:~$ curl <your externalURL here>
<a href="/sign-in?returnTo=%2F">Found</a>
```

#### Configure Auto-indexing

1. Go to the **Site admin** page
2. Expand **Code graph**,
3. Select **Configuration**
4. Click **Create new policy**, and fill in:
    - Name: `TEST`
    - Click *add a repository pattern*
    - Repository pattern #1: set this to an existing repository on your Sourcegraph instance (
      e.g. `github.com/gorilla/mux`)
    - Type: `HEAD`
    - Retention: Disabled
    - Auto-indexing: Enabled
5. Expand **Code graph**
6. Select **Auto-indexing**, and check to see if an indexing job has appeared. If nothing is there:
    - Try clicking **Enqueue**
    - Try setting a higher update frequency: `PRECISE_CODE_INTEL_AUTO_INDEXING_TASK_INTERVAL=10s`
    - Try setting a lower delay: `PRECISE_CODE_INTEL_AUTO_INDEXING_REPOSITORY_PROCESS_DELAY=10s`
7. Once you have a completed indexing job, click **Uploads** and check to see that an index has been uploaded.
8. Once the index has been uploaded, you should see the **`PRECISE`** badge in the hover popover! 🎉

## Auto-scaling

> NOTE: Auto scaling is currently not supported
> when [downloading and running executor binaries yourself](deploy_executors_binary.md),
> and on managed instances when using self-hosted executors, since it requires deployment adjustments.

Auto-scaling of executor instances can help to increase concurrency of jobs, without paying for unused resources. With auto-scaling, you can scale down to 0 instances when no workload exist and scale up as far as you like and your cloud provider can support. Auto-scaling needs to be configured separately.

Auto-scaling makes use of the auto-scaling capabilities of the respective cloud provider (**AutoScalingGroups** on AWS and **Instance Groups** on GCP). Sourcegraph's `worker` service publishes a scaling metric (that is, the number of jobs in queue) to the cloud providers. Then, based on that reported value, the auto-scalers add and remove compute resources to match the required amount of compute. The autoscaler will attempt to hold 1 instance running per each [`executor_jobs_per_instance_scaling`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub.com/sourcegraph/terraform-.*-executors%24+variable+%22executor_jobs_per_instance_scaling%22&patternType=literal) items in queue.

For example, if `executor_jobs_per_instance_scaling` is set to `20` and the queue size is currently `400`, then `20`instances would be determined as required to handle the load. You might want to tweak this number based on the [machine type](https://sourcegraph.com/search?q=context:global+repo:%5Egithub.com/sourcegraph/terraform-.*-executors%24+variable+%22machine_type%22+-f:docker-mirror&patternType=literal), [concurrency per machine](https://sourcegraph.com/search?q=context:global+repo:%5Egithub.com/sourcegraph/terraform-.*-executors%24+variable+%22maximum_num_jobs%22&patternType=literal) and desired processing speed.

With the Terraform variables [`executor_min_replicas`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub.com/sourcegraph/terraform-.*-executors%24+variable+%22executor_min_replicas%22&patternType=literal) and [`executor_max_replicas`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub.com/sourcegraph/terraform-.*-executors%24+variable+%22executor_max_replicas%22&patternType=literal) in the Terraform modules linked to above, you can configure the minimum and maximum number of compute machines to be run at a given time.

For auto-scaling to work, two things must be true:

1. `executor_min_replicas` must be `>= 0` and `executor_max_replicas` must be `> executor_min_replicas`.
2. The Sourcegraph instance (its `worker` service, specifically) needs to publish scaling metrics to the used cloud
   provider.

For the latter to work, the Sourcegraph instance needs to be configured with the correct credentials that allow it to access the cloud provider.

The `credentials` submodule in both the [AWS](https://sourcegraph.com/github.com/sourcegraph/terraform-aws-executors/-/tree/modules/credentials) and [Google](https://sourcegraph.com/github.com/sourcegraph/terraform-google-executors/-/tree/modules/credentials) executor modules exists for that purpose. When used, the `credentials` module sets up the credentials on the cloud provider and returns them in the Terraform outputs.

Here's an example of how one would configure auto-scaling.

```terraform
module "executors" {
  source  = "sourcegraph/executors/<aws | google>"
  version = "<version>"

  # Basic configuration...

  # Auto-scaling
  executor_min_replicas              = 0 # Spin down when not in use
  executor_max_replicas              = 30
  executor_jobs_per_instance_scaling = 20
}

module "my-credentials" {
  source  = "sourcegraph/executors/<aws | google>//modules/credentials"
  version = "<version>"

  # AWS
  availability_zone = "<availability zone to provision resource in AWS>" # Removed in 4.2
  # Google
  zone              = "<zone to provision resource in GCP>" # Removed in 4.1

  resource_prefix = "<optional prefix to added to created resources>"
}

# AWS
output "metric_writer_access_key_id" {
  value = module.my-credentials.metric_writer_access_key_id
}

output "metric_writer_secret_key" {
  value     = module.my-credentials.metric_writer_secret_key
  sensitive = true
}

# Google
output "metric_writer_credentials_file" {
  value     = module.my-credentials.metric_writer_credentials_file
  sensitive = true
}
```

After running `terraform apply`, the outputs are retrieved by running the following commands.

```shell
# AWS
$ terraform output metric_writer_access_key_id
$ terraform output metric_writer_secret_key

# Google
$ terraform output metric_writer_credentials_file
```

These outputs are used to configure the Sourcegraph instance (see below).

### AWS

The AWS EC2 auto-scaling groups configured by the Sourcegraph Terraform module respond to changes in metric values written to **CloudWatch**. The target Sourcegraph instance is expected to continuously write these values.

To write the scaling metric to **CloudWatch**, the `worker` service must have defined the following environment variables.

| Environment Variable                    | Description                                              |
| --------------------------------------- | -------------------------------------------------------- |
| `EXECUTOR_METRIC_ENVIRONMENT_LABEL`     | Same value as `executor_metrics_environment_label`       |
| `EXECUTOR_METRIC_AWS_NAMESPACE`         | Must be set to `sourcegraph-executor`                    |
| `EXECUTOR_METRIC_AWS_REGION`            | The target AWS region                                    |
| `EXECUTOR_METRIC_AWS_ACCESS_KEY_ID`     | The value of the output of `metric_writer_access_key_id` |
| `EXECUTOR_METRIC_AWS_SECRET_ACCESS_KEY` | The value of the output of `metric_writer_secret_key`    |

### Google

The Google Compute Engine auto-scaling groups configured by the Sourcegraph Terraform module respond to changes in metric values written to Cloud Monitoring. The target Sourcegraph instance is expected to continuously write these values.

To write the scaling metric to **Cloud Monitoring**, the `worker` service must have defined the following environment variables.

| Environment Variable                | Description                                        |
| ----------------------------------- | -------------------------------------------------- |
| `EXECUTOR_METRIC_ENVIRONMENT_LABEL` | Same value as `executor_metrics_environment_label` |
| `EXECUTOR_METRIC_GCP_PROJECT_ID`    | The GCP Project ID                                 |

Then either one of the following environment variables must be set.

| Environment Variable                                          | Description                                                                                       |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `EXECUTOR_METRIC_GOOGLE_APPLICATION_CREDENTIALS_FILE_CONTENT` | The **base64-decoded** output of `metric_writer_credentials_file`                                 |
| `EXECUTOR_METRIC_GOOGLE_APPLICATION_CREDENTIALS_FILE`         | The path to the file containing the **base64-decoded** output of `metric_writer_credentials_file` |

### Testing auto scaling

Once the environment variables have been set and the worker service has been restarted, you should be able to find the scaling metrics in your cloud providers dashboards.

To test if the metric is correctly reported into the Cloud provider:

- On AWS, this can be found in the **CloudWatch** metrics section. Under **All metrics**, select the namespace `sourcegraph-executor` and then the metric `environment`, `queueName`. Make sure there are entries returned.
- On Google Cloud, this can be found in the **Metrics explorer**. Select **Resource type: Global** and then **Metric: `custom/executors/queue/size`**. You should see values reported here. `0` is also an indicator that it works correct.

Next, you can test whether the number of executors rises and shrinks as load spikes occur. Keep in mind that auto-scaling is not a real-time operation on most cloud providers and usually takes a short moment and can have some delays between the metric going down and the desired machine count adjusting.

## Upgrading executors

Upgrading executors is relatively uninvolved. Simply follow the instructions below.
Also, check the [changelog](https://github.com/sourcegraph/sourcegraph/blob/main/CHANGELOG.md) for any Executors related breaking changes or new features or flags that you might want to configure. See [Executors maintenance](deploy_executors.md#Maintaining-and-upgrading-executors) for version compatability.

### **Step 1:** Update the source version of the terraform modules

> NOTE: Keep in mind that only one minor version bumps are guaranteed to be disruption-free.

```diff
module "executors" {
  source = "sourcegraph/executors/<aws | google>"

  # Find the latest version matching your Sourcegraph version here:
  # - https://github.com/sourcegraph/terraform-google-executors/tags
  # - https://github.com/sourcegraph/terraform-aws-executors/tags
-  version = "4.0.0"
+  version = "4.1.0"

  # AWS specific
  availability_zone = "<availability zone to provision resource in AWS>"
  # Google specific
  region            = "<region to provision in GCP>"
  zone              = "<zone to provision resource in GCP>"

  executor_sourcegraph_external_url            = "<external url>"
  executor_sourcegraph_executor_proxy_password = "<shared secret>"
  executor_queue_name                          = "<codeintel | batches>"
  executor_instance_tag                        = "<tag to filter in stackdriver monitoring>"
  executor_metrics_environment_label           = "<label to filter custom metrics>"
}
```

### **Step 2:** Reapply the terraform configuration

Simply reapply the terraform configuration and executors will be ready to go again.

```bash
terraform apply
```
