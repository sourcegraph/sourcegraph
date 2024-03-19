# Deploying Sourcegraph executors on linux machines

## Installation

> Note: See [offline installation guide](deploy_executors_binary_offline.md) for instructions on how to install executors in an air-gapped environment.

The following steps will guide you through the process of installing executors on a linux machine.

### Dependencies

In order to run executors on your machine, a few things need to be set up correctly before proceeding.

- Executors only support linux-based machine with amd64 processors
- Docker has to be installed on the machine (`curl -fsSL https://get.docker.com | sh`)
- Git has to be installed at a version `>= v2.26`
- The ability to run commands as `root` on the host machine and configure networking routes

If [Firecracker isolation will be used](index.md#how-it-works): _(recommended)_

- The host has to support KVM (for AWS that means a [metal instance](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html), on GCP that means [enabling nested virtualization](https://cloud.google.com/compute/docs/instances/nested-virtualization/enabling))
- The following additional dependencies need to be installed:
  - `dmsetup`
  - `losetup`
  - `mkfs.ext4`
  - `iptables`
  - `strings` (part of binutils)
  - `systemd` (optional)

### **Step 0:** Confirm that virtualization is enabled (if using Firecracker)

KVM (virtualization) support is required for [our sandboxing model](index.md#how-it-works) with Firecracker. The following command checks whether virtualization is enabled on the machine (it should print something):

```bash
$ lscpu | grep Virtualization

Virtualization:                  VT-x
Virtualization type:             full
```

On Ubuntu-based distributions, you can also use the tool `kvm-ok` available in the `cpu-checker` package to reliably validate KVM support on your host:

```bash
# Install cpu-checker
$ apt-get update && apt-get install -y cpu-checker

# Check for KVM support
$ kvm-ok
INFO: /dev/kvm exists
KVM acceleration can be used
```

### **Step 1:** Download the latest executor binary

Below are the download links for the *latest* release of executors:

**Note:** Executors need to match the version of Sourcegraph they're running against. Latest will usually only work for you when
you run the latest version of Sourcegraph.

* [`linux-amd64/executor`](https://storage.googleapis.com/sourcegraph-artifacts/executor/latest/linux-amd64/executor)
* [`linux-amd64/executor_SHA256SUM`](https://storage.googleapis.com/sourcegraph-artifacts/executor/latest/linux-amd64/executor_SHA256SUM)
* [`Build info`](https://storage.googleapis.com/sourcegraph-artifacts/executor/latest/info.txt)

Download and setup the `executor` binary:

```bash
# Plug in the version of your Sourcegraph instance, like v4.1.0.
# Before Sourcegraph 3.43.0, tagged releases of executors are not available and you should default to using "latest" instead.
# Using latest is NOT recommended, because it might be incompatible with your Sourcegraph version.
curl -sfLo executor https://storage.googleapis.com/sourcegraph-artifacts/executor/${SOURCEGRAPH_VERSION}/linux-amd64/executor
chmod +x executor
# Assuming /usr/local/bin is in $PATH.
mv executor /usr/local/bin
```

### **Step 2:** Setup environment variables

The executor is configured through environment variables. Those need to be passed to it when you run it (including for `install`, `validate` and `test-vm`), so add these to your shell profile, or an environment file. Only `EXECUTOR_FRONTEND_URL`, `EXECUTOR_FRONTEND_PASSWORD` and `EXECUTOR_QUEUE_NAME` **or** `EXECUTOR_QUEUE_NAMES` are _required_.

| Env var                                  | Description                                                                                                                                                                                                                        | Example value                              |
| ---------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------ |
| `EXECUTOR_FRONTEND_URL`                  | The external URL of the Sourcegraph instance. **required**                                                                                                                                                                         | `http://sourcegraph.example.com`           |
| `EXECUTOR_FRONTEND_PASSWORD`             | The shared secret configured in the Sourcegraph instance site config under `executors.accessToken`. **required**                                                                                                                   | `our-shared-secret`                        |
| `EXECUTOR_QUEUE_NAME`                    | The name of a single queue to pull jobs from. Possible values: `batches` and `codeintel`. **required: either this or `EXECUTOR_QUEUE_NAMES`**                                                                                      | `batches`                                  |
| `EXECUTOR_QUEUE_NAMES`                   | The names of multiple queues to pull jobs from, comma-separated. Possible values: `batches` and `codeintel`. **required: either this or `EXECUTOR_QUEUE_NAME`**                                                                    | `batches,codeintel`                        |
| `EXECUTOR_USE_FIRECRACKER`               | Whether to isolate jobs in virtual machines. Requires ignite and firecracker. Linux hosts only. Kubernetes is not supported. (default value: "true" when OS is Linux and not on Kubernetes)                                        | `true`                                     |
| `EXECUTOR_MAXIMUM_NUM_JOBS`              | Number of virtual machines or containers that can be running at once. (default value: "1")                                                                                                                                         | `1`                                        |
| `EXECUTOR_MAXIMUM_RUNTIME_PER_JOB`       | The maximum wall time that can be spent on a single job. (default value: "30m")                                                                                                                                                    | `30m`                                      |
| `EXECUTOR_JOB_MEMORY`                    | How much memory to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs). (default value: "12G")                                                                          | `12G`                                      |
| `EXECUTOR_JOB_NUM_CPUS`                  | How many CPUs to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs). (default value: "4")                                                                              | `4`                                        |
| `EXECUTOR_DOCKER_REGISTRY_MIRROR_URL`    | The address of a docker registry mirror to use in firecracker VMs.                                                                                                                                                                 | `http://10.0.0.2:5000`                     |
| `EXECUTOR_FIRECRACKER_DISK_SPACE`        | How much disk space to allocate to each virtual machine. (default value: "20G")                                                                                                                                                    | `20G`                                      |
| `EXECUTOR_FIRECRACKER_BANDWIDTH_EGRESS`  | How much bandwidth to allow for egress packets to the VM in bytes/s. (default value: "524288000")                                                                                                                                  | `524288000`                                |
| `EXECUTOR_FIRECRACKER_BANDWIDTH_INGRESS` | How much bandwidth to allow for ingress packets to the VM in bytes/s. (default value: "524288000")                                                                                                                                 | `524288000`                                |
| `EXECUTOR_KEEP_WORKSPACES`               | Whether to skip deletion of workspaces after a job completes (or fails). Note that when Firecracker is enabled that the workspace is initially copied into the VM, so modifications will not be observed. (default value: "false") | `true`                                     |
| `EXECUTOR_MAX_ACTIVE_TIME`               | The maximum time that can be spent by the worker dequeueing records to be handled. (default value: "0")                                                                                                                            | `100m`                                     |
| `EXECUTOR_NUM_TOTAL_JOBS`                | The maximum number of jobs that will be dequeued by the worker. (default value: "0")                                                                                                                                               | `100`                                      |
| `EXECUTOR_DOCKER_HOST_MOUNT_PATH`        | The target workspace as it resides on the Docker host (used to enable Docker-in-Docker).                                                                                                                                           | `/workspaces`                              |
| `EXECUTOR_QUEUE_POLL_INTERVAL`           | Interval between dequeue requests. (default value: "1s")                                                                                                                                                                           | `1s`                                       |
| `EXECUTOR_CLEANUP_TASK_INTERVAL`         | The frequency with which to run periodic cleanup tasks. (default value: "1m")                                                                                                                                                      | `1m`                                       |
| `EXECUTOR_VM_PREFIX`                     | A name prefix for virtual machines controlled by this instance. (default value: "executor")                                                                                                                                        | `executor`                                 |
| `EXECUTOR_VM_STARTUP_SCRIPT_PATH`        | A path to a file on the host that is loaded into a fresh virtual machine and executed on startup.                                                                                                                                  | `/vm-startup.sh`                           |
| `NODE_EXPORTER_URL`                      | The URL of the node_exporter instance, without the /metrics path.                                                                                                                                                                  | `http://127.0.0.1:9000`                    |
| `EXECUTOR_FIRECRACKER_IMAGE`             | The base image to use for virtual machines.                                                                                                                                                                                        | `sourcegraph/executor-vm:insiders`         |
| `EXECUTOR_FIRECRACKER_KERNEL_IMAGE`      | The base image containing the kernel binary to use for virtual machines.                                                                                                                                                           | `sourcegraph/ignite-kernel:5.10.135-amd64` |
| `EXECUTOR_FIRECRACKER_SANDBOX_IMAGE`     | The OCI image for the ignite VM sandbox.                                                                                                                                                                                           | `sourcegraph/ignite:v0.10.5`               |
| `DOCKER_REGISTRY_NODE_EXPORTER_URL`      | The URL of the Docker Registry instance's node_exporter, without the /metrics path.                                                                                                                                                | `http://localhost:9000`                    |
| `SRC_LOG_LEVEL`                          | upper log level to restrict log output to (dbug, info, warn, error, crit) (default value: "warn")                                                                                                                                  | `warn`                                     |


```bash
# Example:
export EXECUTOR_QUEUE_NAME=batches
export EXECUTOR_FRONTEND_URL=http://sourcegraph.yourcompany.com
export EXECUTOR_FRONTEND_PASSWORD=SUPER_SECRET_SHARED_TOKEN
```

### **Step 3:** Configure your machine

To be able to run workloads in isolation, a few dependencies need to be installed and configured. The executor CLI can do all of that automatically.

To run all of the required setup steps, just run the following commands as `root`:

```bash
executor install all
# OR run the following, to see how to install/configure components separately.
executor install --help
```

### **Step 4:** Validate your machine is ready to receive workloads

All set up! Before letting the executor start receiving workloads from your Sourcegraph instance, you might want to verify your setup. Run the following command:

```bash
executor validate
```

If any issues are found, correct them before proceeding.

If you use [our sandboxing model with Firecracker](index.md#how-it-works) _(recommended)_, you can also verify that the executor is able to spin up the isolation VMs properly. For that, use the following command:

```bash
# Optionally provide a repo to clone into the VMs workspace, to verify that cloning works properly as well.
executor test-vm [--repo=github.com/sourcegraph/sourcegraph --revision=main]
```

This should succeed and print a command to use to attach to the guest VM. If it is able to spin up properly, that is usually good indication that everything is set up properly. If you need to debug some issue further, this can be helpful as you can exec into the guest VM from here.

### **Step 5:** Make executor a systemd service (optional, systemd-based distributions only)

To make sure that the executor runs post-boot of the machine, you might want to wrap it in a systemd service. This is an example of how that could look like:

```bash
cat <<EOF >/etc/systemd/system/executor.service
[Unit]
Description=Sourcegraph executor
After=docker.service
BindsTo=docker.service

[Service]
ExecStart=/usr/local/bin/executor
Restart=on-failure
EnvironmentFile=/etc/systemd/system/executor.env
Environment=HOME="%h"
Environment=SRC_LOG_LEVEL=dbug

[Install]
WantedBy=multi-user.target
EOF

# Create environment file (this can also be sourced in your shell)
cat <<EOF >/etc/systemd/system/executor.env
EXECUTOR_FRONTEND_URL="https://sourcegraph.yourcompany.com"
EXECUTOR_FRONTEND_PASSWORD="SUPER_SECRET_SHARED_TOKEN"
EXECUTOR_QUEUE_NAME="batches"
EOF

systemctl enable executor
```

### **Step 6:** Start receiving workloads

If you use the systemd service, simply run `systemctl start executor`, otherwise run `executor run`. Your executor should start listening for jobs now! All done!

## Upgrading executors

Upgrading executors is relatively uninvolved. Simply follow the instructions below.
Also, check the [changelog](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/CHANGELOG.md) for any Executors related breaking changes or new features that you might want to configure.

### **Step 1:** First, grab the executor binary for the new target Sourcegraph version.

> NOTE: Keep in mind that only one minor version bumps are guaranteed to be disruption-free.

```bash
curl -sfLo executor https://storage.googleapis.com/sourcegraph-artifacts/executor/${SOURCEGRAPH_VERSION}/linux-amd64/executor
chmod +x executor
# Assuming /usr/local/bin is in $PATH.
mv executor /usr/local/bin
```

### **Step 2:** Make sure all ambient dependencies and configurations are up-to-date:

Ensure [env vars](#step-2-setup-environment-variables) has been configured. 

```bash
executor install all
# OR run the following, to see how to install/configure components separately.
executor install --help
```

### **Step 3:** Validate your machine is ready to receive workloads

All set up! Before letting the executor start receiving workloads from your Sourcegraph instance, you might want to verify your setup. Run the following command:

```bash
executor validate
```

### **Step 4:** Restart your running executor / spin up a new machine

Depending on how you set up executors, you might want to restart the systemd service, or restart/replace the machine running them, so the new binary is running.
If you use the systemd service, simply run `systemctl start executor`, otherwise run `executor run`. Your executor should start listening for jobs now and be visible under the `Executors > Instances` section of the Site Configuration.
