# Troubleshooting Executors
This page compiles a list of common troubleshooting steps found during development and administration of executors.

## Checking for issues with an executor instance
To debug problems you might face with an executor instance, you can apply the following steps.

First, prepare the instance:
1. `ssh` into the host VM (see [Connecting to cloud provider executor instances](#connecting-to-cloud-provider-executor-instances))
1. `sudo su` to become the `root` user
1. `systemctl stop executor` to stop the `executor` service
1. `export $(cat /etc/systemd/system/executor.env | xargs)` to load the executor environment into your shell

### Validating the executor configuration
You can now run `executor validate`, which will inform you about any configuration issues. Fix any reported issues before proceeding.

### Creating a debug Firecracker VM
The next step is to create a temporary Firecracker VM for debugging purposes.

> NOTE: if the host VM is provisioned with the [Sourcegraph terraform modules](./deploy_executors_terraform.md), the VMs may be configured to stop automatically. Refer to [Disabling the auto-deletion of Executor VMs](#disabling-the-auto-deletion-of-executor-vms) for information to prevent this.

Run one of the following commands `executor test-vm` to generate a test firecracker VM:
```shell
# Test if a firecracker VM can be started
executor test-vm

# Test if a firecracker VM can be started and if a repository can be cloned into the VM's workspace
executor test-vm [--repo=github.com/sourcegraph/sourcegraph --revision=main]
```

The command will output a line like:
```
Success! Connect to the VM using
$ ignite attach executor-test-vm-0160f53f-e765-4481-a81e-aa3c704d07bd
```
Execute the generated `ignite attach <vm>` command to gain a shell to the Firecracker VM.

## Disabling the auto-deletion of Executor VMs
> NOTE: These instructions are for users using the VMs deployed via the [Terraform Modules](./deploy_executors_terraform.md)

The Executor host VMs are configured to automatically tear themselves down once all jobs in the queue are completed. While this is desired behaviour under regular circumstances, it complicates debugging issues in the executor configuration or connections. To prevent the VMs from automatically stopping:
1. `ssh` into the VM
1. `sudo su` to become the `root` user
1. Remove (or rename) the `/shutdown_executor.sh` file

The VM should now persist after all jobs are satisfied.

## Recreating a Firecracker VM 
If a server-side batch change fails unexpectedly, it's possible to recreate the generated Firecracker VM from the batch change execution.

> NOTE: if the host VM is provisioned with the [Sourcegraph terraform modules](./deploy_executors_terraform.md), the VMs may be configured to stop automatically. Refer to [Disabling the auto-deletion of Executor VMs](#disabling-the-auto-deletion-of-executor-vms) for information to prevent this.

1. Navigate to the failed execution page of the Batch Change
1. Select a failed Workspace on the left and click the `Diagnostics` link on the right pane
1. In the modal, expand the `Setup` step by clicking the text or the expansion arrow on the right
1. Copy the command from the  final step of `Setup` starting with `ignite run` 

1. `ssh` into the host VM
1. `sudo su` to become the `root` user
1. `systemctl stop executor` to stop the `executor` service
1. `export $(cat /etc/systemd/system/executor.env | xargs)` to load the executor environment into your shell
1. Paste in the command copied from the batch change. You may need to remove the `--copy-files` and `--volumes` directives as those volumes and files may not exist on the VM any longer. Surround the `--kernel-args` arguments in quotes as well
1. Execute the command and wait for the VM to start
1. Run `ignite ps` to list all currently running VMs
1. Run `ignite attach <vm id>` to get a shell to the running VM

## List preferred Linux distros
An ARM64 (x86_64) linux distro must be used due to the machine type of the VM. You may list available ARM64 distros with the following command, depending on your cloud provider:

### GCP    
```shell
gcloud compute images list --filter='(family~amd)'
```
    
### AWS
```shell
aws ec2 describe-instances --filters architecture=x86_64
```

## Configure the log level of executors
The log level of executors are set using the environment variable `SRC_LOG_LEVEL`. The following values are allowed:
* `dbug`
* `info`
* `warn` (default)
* `error`
* `crit`

Update or set this value in the shell profile or environment file of the instance, then run `executor run` to restart the instance. 

## Problems with the Docker mirror instance
Verify that the Docker mirror instance is functioning properly by testing the following:
    
### Mirror is reachable from the executor instance
Run the following command on the executor instance to determine whether it responds properly:
```shell
# If EXECUTOR_DOCKER_REGISTRY_MIRROR_URL is set to a custom URL, replace the base endpoint with its value
curl http://localhost:5000/v2/_catalog
```
    
### Registry is mounted in the file system
Verify that the registry is mounted under the expected path in the file system by running:
```shell
# This directory should always be mounted
ls /mnt/registry
   
# If jobs have been processed, the following path should exist
ls /mnt/registry/docker/registry/v2/repositories/<public repository name>
```

## Connecting to cloud provider executor instances
The following commands allow you to SSH into an executor instance, depending on your cloud platform of choice.
    
### GCP
Find the name of an executor instance with
```shell
# optionally provide the --project flag
gcloud compute instances list --filter="name~executor" --format="get(name)"
```

Then, using the name of an instance, run
```shell
# optionally provide the --project flag
# use an identity-aware proxy tunnel with --tunnel-through-iap
gcloud compute ssh ${INSTANCE_NAME}
```
    
Alternatively, you may navigate to the compute instance in the GCP web console, where you will be able to connect with SSH in-browser.

### AWS
In order to connect to an EC2 instance using SSH, you must have [specified a key pair](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html) when the instance was launched. If you have not done so, you can connect to your instance through the web console instead.  

Assuming you have specified the key pair, first run
```shell
chmod 400 path/to/key.pem 
```
    
Find the public DNS value of your instance either through the web console or by using `aws ec2 describe-instances`, then run
```shell
ssh -i "path/to/key.pem" root@${INSTANCE_PUBLIC_DNS}
```

## Misconfigured environment variables
This section lists some common mistakes with environment variables. Some of these will be exposed by running `executor validate` on the executor instance.

| Env var                                                                                                                                                                 | Common mistakes                                                                                                            |
|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------------------------------|
| `EXECUTOR_FRONTEND_URL`                                                                                                                                                 | No protocol included (e.g. `https://`                                                                                      |
| `EXECUTOR_FRONTEND_PASSWORD`                                                                                                                                            | Not set in `executor.accessToken` in the site config                                                                       |
| `EXECUTOR_QUEUE_NAME`                                                                                                                                                   | Value doesn't match one of [`codeintel`, `batches`], or neither of `EXECUTOR_QUEUE_NAME` and `EXECUTOR_QUEUE_NAMES` is set |
| `EXECUTOR_QUEUE_NAMES`                                                                                                                                                  | Value doesn't match one of [`codeintel`, `batches`]                                                                        |
| <ul><li>`EXECUTOR_MAXIMUM_RUNTIME_PER_JOB`</li><li>`EXECUTOR_MAX_ACTIVE_TIME`</li><li>`EXECUTOR_QUEUE_POLL_INTERVAL`</li><li>`EXECUTOR_CLEANUP_TASK_INTERVAL`</li></ul> | Value format can't be parsed by `time.ParseDuration`                                                                       |
| <ul><li>`EXECUTOR_JOB_MEMORY`</li><li>`EXECUTOR_JOB_NUM_CPUS`</li></ul>                                                                                                 | Value format not recognized by virtual machine or Docker                                                                   |
| `EXECUTOR_FIRECRACKER_DISK_SPACE`                                                                                                                                       | Value format not recognized by virtual machine                                                                             |
| `EXECUTOR_DOCKER_REGISTRY_MIRROR_URL`                                                                                                                                   | Wrong IP or port specified                                                                                                 |
| `EXECUTOR_DOCKER_HOST_MOUNT_PATH`                                                                                                                                       | Workspace does not exist at provided mount path                                                                            |
| `EXECUTOR_VM_STARTUP_SCRIPT_PATH`                                                                                                                                       | Script does not exist at provided file path                                                                                |
| <ul><li>`EXECUTOR_FIRECRACKER_IMAGE`</li><li>`EXECUTOR_FIRECRACKER_KERNEL_IMAGE`</li><li>`EXECUTOR_FIRECRACKER_SANDBOX_IMAGE`</li></ul>                                 | Image does not exist for provided repository, name, or tag                                                                 |
| <ul><li>`NODE_EXPORTER_URL`</li><li>`DOCKER_REGISTRY_NODE_EXPORTER_URL`</li></ul>                                                                                       | `/metrics` path is included or wrong IP or port specified                                                                  |
| `SRC_LOG_LEVEL`                                                                                                                                                         | not set to one of [`dbug`, `info`, `warn`, `error`, `crit`]                                                                |

## Verify Firecracker support    
The VM instance must [support KVM](./deploy_executors.md#firecracker-requirements). In effect, this means the instance must meet certain requirements depending on the Cloud provider in use.
    
### GCP
Nested virtualization must be enabled on the machine.
1. SSH into the executor instance (see [Connecting to cloud provider executor instances](#connecting-to-cloud-provider-executor-instances))
1. Run the following command. If it outputs anything other than `0`, nested virtualization is enabled:
    ```shell
    grep -cw vmx /proc/cpuinfo
    ```
    
### AWS
Verify that the machine type in use is of type `.metal` (e.g. `M5.metal`).
    
## Why use `iptables` 
`iptables` provides network isolation, security, and regulated access for Firecracker VMs. It implements NAT of private IP addresses for each VM, and allows forwarding only specific ports to VMs. It also blocks all other traffic, and prevents IP spoofing. 
    
### Allowed traffic
| Description                                          | Purpose                  | Relevant rules                                                                                                                                                                           |
|------------------------------------------------------|--------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| DNS traffic                                          | DNS resolution           | `iptables -A CNI-ADMIN -p udp --dport 53 -j ACCEPT`                                                                                                                                      |
| Host to guest, established connections guest to host | SSH access               | `iptables -A INPUT -d 10.61.0.0/16 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT`                                                                                                 |
| From guest to gateway                                | Outbound internet access | <ul><li>`iptables -A CNI-ADMIN -s 10.61.0.1/32 -d 10.61.0.0/16 -j ACCEPT`</li><li>`iptables -A CNI-ADMIN -d 10.61.0.0/16 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT`</li></ul> |
    
### Blocked traffic
| Description         | Purpose                                                              | Relevant rules                                                |
|---------------------|----------------------------------------------------------------------|---------------------------------------------------------------|
| Guest to host       | Block outbound traffic (e.g. other executors or the Docker registry) | `iptables -A INPUT -s 10.61.0.0/16 -j DROP`                   |
| Guest to guest      | Block outbound traffic to other Firecracker VMs                      | `iptables -A INPUT -s 10.61.0.0/16 -d 10.61.0.0/16 -j DROP`   |
| Guest to link-local | Block Cloud provider resources such as instance metadata             | `iptables -A INPUT -s 10.61.0.0/16 -d 169.254.0.0/16 -j DROP` |

## Kubernetes Job Scheduling

There are a few environment variables available that can be used to determine which node an Executor Job Pod will be 
scheduled in. The Job Pods need to be scheduled in the same node as the Executor Pod (in order to mount the 
Persistence Volume Claim).

The following environment variables can be used to determine where the Job Pods will be scheduled.

| Name                                                         | Default Value | Description                                                                                                                                                                                            |
|--------------------------------------------------------------|:--------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| EXECUTOR_KUBERNETES_NODE_NAME                                | N/A           | The name of the Kubernetes Node to create Jobs in. If not specified, the Pods are created in the first available node.                                                                                 |
| EXECUTOR_KUBERNETES_NODE_SELECTOR                            | N/A           | A comma separated list of values to use as a node selector for Kubernetes Jobs. e.g. `foo=bar,app=my-app`                                                                                              |
| EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_EXPRESSIONS | N/A           | The JSON encoded required affinity match expressions for Kubernetes Jobs. e.g. `[{"key": "foo", "operator": "In", "values": ["bar"]}]`                                                                 |
| EXECUTOR_KUBERNETES_NODE_REQUIRED_AFFINITY_MATCH_FIELDS      | N/A           | The JSON encoded required affinity match fields for Kubernetes Jobs. e.g. `[{"key": "foo", "operator": "In", "values": ["bar"]}]`                                                                      |
| EXECUTOR_KUBERNETES_POD_AFFINITY                             | N/A           | The JSON encoded pod affinity for Kubernetes Jobs. e.g. [{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}]      |
| EXECUTOR_KUBERNETES_POD_ANTI_AFFINITY                        | N/A           | The JSON encoded pod anti-affinity for Kubernetes Jobs. e.g. [{"labelSelector": {"matchExpressions": [{"key": "foo", "operator": "In", "values": ["bar"]}]}, "topologyKey": "kubernetes.io/hostname"}] |

### Scheduling Errors

If you encounter the following errors,

```text
deleted by scheduler: pod could not be scheduled
```

or

```text
unexpected end of watch
```

Add/update the environment variable `SRC_LOG_LEVEL` to `dbug` to start receiving debug logs. The specific debug logs 
that may help troubleshoot the errors is `Watching pod`

The `Watching pod` debug logs contain `conditions` that may describe _why_ a Job Pod is not being scheduled correctly. 
For example,

```json
{
  "conditions": {
    "condition[0]": {
      "type": "PodScheduled",
      "status": "False",
      "reason": "Unschedulable",
      "message": "0/1 nodes are available: 1 node(s) didn't match pod affinity rules. preemption: 0/1 nodes are available: 1 Preemption is not helpful for scheduling."
    }
  }
}
```

Tells us that the Pod cannot be scheduled because the pod affinity rules (`EXECUTOR_KUBERNETES_POD_AFFINITY`) we 
configured do not match any nodes.

In this case, the `EXECUTOR_KUBERNETES_POD_AFFINITY` needs to be modified to correctly target the node.
