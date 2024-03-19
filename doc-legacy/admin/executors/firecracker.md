# Firecracker

[Executors](./index.md), by design, are services that run arbitrary code supplied by a user. The executor jobs produced by precise code intelligence [auto-indexing](../../code_navigation/explanations/auto_indexing.md) and [server-side batch changes](../../batch_changes/explanations/server_side.md) are built to invoke _templated_ execution plans, where some parts of execution may invoke code configured by a Sourcegraph administrator or user. Generating a precise index requires invoking an indexer for that language. Batch changes are configured to run arbitrary tooling over the contents of a repository.

Because Sourcegraph has access to your code and credentials to external tools, we've designed executors to be able to run separately from the Sourcegraph instance (on a raw compute node) with the minimum API surface area and user code exposed to the job required to meet its objective. This effectively reduces the blast radius of a misconfiguration or insecure configuration.

Jobs handled by the executor are defined as a series of Docker (or Docker-like) image invocations sharing the same filesystem workspace initialized with the contents of the repository being indexed or modified. When the executor process is running on a raw compute node (specifically, not using the Kubernetes runner in-cluster), the executor can create a [Firecracker "MicroVM"](https://firecracker-microvm.github.io/) for every job and run the containers in a _completely isolated_ manner. The Docker (or Docker-like) daemon running in a MicroVM is isolated from the host, hence each job is also isolated from one another.

See [this architecture diagram](./index.md#firecracker) detailing Firecracker isolation.

## When to use

Using Firecracker is our **most secure** isolation method, but it is not a **necessary** isolation method. Most users will be fine running containers on the executor host, or deploying bundled executors via Kubernetes jobs. Firecracker isolation was created when our design constrains included multi-tenant environments, and is likely overkill for any on-premise Sourcegraph instance administrated by a single company.

For companies with very high security consciousness, Firecracker isolation is still an option. See the [How to use](firecracker.md#how-to-use) for installation instructions and deployment caveats.

> Note: Firecracker relies on some specific kernel extensions to run, which are only available on some classes of compute on Cloud providers such as AWS and GCP.

## How to use

To use Firecracker, the host machine has to support KVM. When deploying on an AWS, that means the compute node must run on a [metal instance (`.metal`)](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-types.html). When deploying on GCP, that means the compute node must have [nested virtualization enabled](https://cloud.google.com/compute/docs/instances/nested-virtualization/enabling).

See [deploying Executors binary](./deploy_executors_binary.md) for additional information on configuring Linux Machines.

### AWS Bare Metal

AWS Bare Metal provides an application with direct access to the processor and memory of the underlying server. This
allows the application to use the host hardware and kernel directly, no virtualization layer is present.

Using bare metal is expensive to run on. Ideally, Executors on bare metal should be used when there are a lot of Jobs to
run - this will offset the cost of running a bare metal instance. To get the best performance to cost ratio, it is
recommended to fine tune the number of CPUs, the Disk Space allocated, and the memory for each Firecracker VM.

Executor can be fined tuned with the following environment variables,

| Environment Variable              | Description                                                                                                                                               |
|-----------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|
| `EXECUTOR_FIRECRACKER_DISK_SPACE` | How much disk space to allocate to each virtual machine. (default value: "20G")                                                                           |
| `EXECUTOR_JOB_NUM_CPUS`           | How many CPUs to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs). (default value: "4")     |
| `EXECUTOR_JOB_MEMORY`             | How much memory to allocate to each virtual machine or container. A value of zero sets no resource bound (in Docker, but not VMs). (default value: "12G") |

## Known caveats

We [configure iptables](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/executor/internal/run/install.go?L229-255) to prevent Firecracker-isolated processes from talking on [Private IPv4 Addresses](https://en.wikipedia.org/wiki/Private_network#Private_IPv4_addresses) (providing network-level isolation). They can talk to DNS and Sourcegraph only, which prevents users from talking to a 10.x.x.x, 172.x.x.x, or 192.168.x.x range IP.
