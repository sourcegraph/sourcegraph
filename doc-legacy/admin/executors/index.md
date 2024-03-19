# Executors

Executors are Sourcegraph's solution for running untrusted code in a secure and controllable way.

## Installation

To deploy executors to target your Sourcegraph instance, [follow our deployment guide](deploy_executors.md).

The supported deployment options are,

- [Binary](./deploy_executors_binary.md).
- [Terraform on AWS or GCP](./deploy_executors.md).
- <span class="badge badge-beta">Beta</span> [Native Kubernetes](./deploy_executors_kubernetes.md).
- <span class="badge badge-beta">Beta</span> [Docker-in-Docker on Kubernetes](./deploy_executors_dind.md).
- <span class="badge badge-beta">Beta</span> [Docker-Compose](./deploy_executors_docker.md).

## Why use executors?

Running untrusted code is a core requirement of features such as precise code navigation [auto-indexing](../../code_navigation/explanations/auto_indexing.md), and [running batch changes server-side](../../batch_changes/explanations/server_side.md).

Auto-indexing jobs, in particular, require the invocation of arbitrary and untrusted code to support the resolution of project dependencies. Invocation of post-install hooks, use of insecure [package management tools](https://github.com/golang/go/issues/29230), and package manager proxy attacks can create opportunities in which an adversary can gain unlimited use of compute or exfiltrate data. The latter outcome is particularly dangerous for on-premise installations of Sourcegraph, which is the chosen option for companies wanting to maintain strict privacy of their code property.

Instead of performing this work within the Sourcegraph instance, where code is available on disk and unprotected internal services are available over the local network, we move untrusted compute into a sandboxed environment, the _executor_, that has access only to the clone of a single repository on disk (its _workspace_) and to the public internet.

## How it works

Executor instances are capable of being deployed in a variety of ways. Each runtime vary in _how_ jobs are executed.

<!-- 
Diagrams are at: https://app.excalidraw.com/s/4Dr1S6qmmY7/6WJFG2bwdx
See handbook to where images are stored: https://handbook.sourcegraph.com/handbook/editing/handbook-images-video/#adding-images-to-google-cloud-storage
-->

### Locally with src-cli

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_src_local_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_src_local_arch.png" alt="Executors architecture - local with src-cli">
</a>

1.  User run the `src` (e.g. `src batch`) command from the command line.
2.  `src` calls the Sourcegraph API to clone a repository.
    1.  The repositories are written to a directory.
3.  A Docker Container is created for each "step."
    1.  The directory containing the repository is mounted to the container.
    2.  "Steps" are ran in sequential order.
4.  The container run a defined command against the repository.
5.  Logs from the container are sent back to `src`.
6.  At the end of processing all repositories, the result is sent to a Sourcegraph API.
    1.  e.g. Batch Changes sends a `git diff` to a Sourcegraph API (and invokes other APIs).

### Binary

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_binary_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_binary_arch.png" alt="Executors architecture - binary">
</a>

1.  The executor binary is installed to a machine.
    1.  Additional executables (e.g. Docker, `src`) are installed as well
2.  The executor instances pulls for available Jobs from a Sourcegraph API
3.  A user initiates a process that creates executor Jobs.
4.  The executor instance "dequeues" a Job.
5.  Executor calls the Sourcegraph API to clone a repository.
    1. The repositories are written to a directory.
6.  A Docker Container is created for each "step."
    1.  If the Job is `batches` (non-native execution), `src` is invoked
    2.  Docker is invoked directly for other Jobs (`codeintel` and native execution `batches`)
    3.  The directory containing the repository is mounted to the container.
    4.  "Steps" are ran in sequential order.
7.  The container run a defined command against the repository.
8.  Logs from the container are sent back to the executor.
9.  Logs are streamed from the executor to a Sourcegraph API
10.  The executor calls a Sourcegraph API to that "complete" the Job.

### Firecracker

> NOTE: [What the heck is firecracker, anyway](./firecracker.md)??

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_firecracker_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_firecracker_arch.png" alt="Executors architecture - firecracker">
</a>

1.  The executor binary is installed to a machine.
    1.  Additional executables (e.g. Docker, `src`) are installed as well
2.  The executor instances pulls for available Jobs from a Sourcegraph API
3.  A user initiates a process that creates executor Jobs.
4.  The executor instance "dequeues" a Job.
5.  Executor calls the Sourcegraph API to clone a repository.
    1.  The repositories are written to a directory.
6. `ignite` starts up a Docker container that spawns a single Firecracker VM within the Docker container.
    1. The directory containing the repository is mounted to the VM.
7. Docker Container is created in the Firecracker VM for each "step."
    1.  If the Job is `batches` (non-native execution), `src` is invoked
    2.  Docker is invoked directly for other Jobs (`codeintel` and native execution `batches`)
    3.  "Steps" are ran in sequential order.
8.  Within each Firecracker VM a single Docker container is created
9.  The container run a defined command against the repository.
10.  Logs from the container are sent back to the executor.
11.  Logs are streamed from the executor to a Sourcegraph API
12.  The executor calls a Sourcegraph API to that "complete" the Job.

### Docker

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_docker_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_docker_arch.png" alt="Executors architecture - docker">
</a>

1.  The executor image is started as a Docker container on a machine
2.  The executor pulls for available Jobs from a Sourcegraph API
3.  A user initiates a process that creates executor Jobs.
4.  The executor instance "dequeues" a Job.
5.  Executor calls the Sourcegraph API to clone a repository.
    1.  The repositories are written to a directory.
6.  A Docker Container is created for each "step."
    1.  If the Job is `batches` (non-native execution), `src` is invoked
    2.  Docker is invoked directly for other Jobs (`codeintel` and native execution `batches`)
    3.  The directory containing the repository is mounted to the container.
    4.  "Steps" are ran in sequential order.
7.  The container run a defined command against the repository.
8.  Logs from the container are sent back to the executor.
9.  Logs are streamed from the executor to a Sourcegraph API
10.  The executor calls a Sourcegraph API to that "complete" the Job.

<!--
Comment out until ready to advertise this
-->

### Native Kubernetes

<span class="badge badge-experimental">Experimental</span>

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_kubernetes_native_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_kubernetes_native_arch.png" alt="Executors architecture - native kubernetes">
</a>

1.  The executor image is started as a pod in a Kubernetes node
2.  The executor pulls for available Jobs from a Sourcegraph API
3.  A user initiates a process that creates executor Jobs.
4.  The executor instance "dequeues" a Job.
5.  Executor calls the Sourcegraph API to clone a repository.
    1.  The repositories are written to a directory.
6.  A Kubernetes Job is created for each "step."
    1.  The directory containing the repository is mounted to the container.
    2.  "Steps" are ran in sequential order.
7.  The container run a defined command against the repository.
8.  Logs from the container are sent back to the executor.
9.  Logs are streamed from the executor to a Sourcegraph API
10.  The executor calls a Sourcegraph API to that "complete" the Job.

### Native execution

Read more in [Native execution](native_execution.md).

### Docker-in-Docker Kubernetes

<span class="badge badge-experimental">Experimental</span>

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_kubernetes_dind_arch.png' target='_blank'>
  <img src="https://storage.googleapis.com/sourcegraph-assets/executor_kubernetes_dind_arch.png" alt="Executors architecture - docker in docker kubernetes">
</a>

1.  The executor image is started as a container in Kubernetes Pod
    1. The dind image is started as a sidecar container in the same Kubernetes Pod
2.  The executor pulls for available Jobs from a Sourcegraph API
3.  A user initiates a process that creates executor Jobs.
4.  The executor instance "dequeues" a Job.
5.  Executor calls the Sourcegraph API to clone a repository.
    1.  The repositories are written to a directory.
6.  A Docker Container is created for each "step."
    1.  If the Job is `batches` (non-native execution), `src` is invoked
    2.  Docker is invoked directly for other Jobs (`codeintel` and native execution `batches`)
    3.  The directory containing the repository is mounted to the container.
    4.  "Steps" are ran in sequential order.
7.  The container run a defined command against the repository.
8.  Logs from the container are sent back to the executor.
9.  Logs are streamed from the executor to a Sourcegraph API
10.  The executor calls a Sourcegraph API to that "complete" the Job.

## Deciding which deployment to use

Deciding how to deploy the executor depends on your use case. The following flowchart can help you decide which
deployment is best for you.

<a href='https://storage.googleapis.com/sourcegraph-assets/executor_deployment_tree.png'>
  <img src='https://storage.googleapis.com/sourcegraph-assets/executor_deployment_tree.png' alt='Executor Deployment Flowchart'>
</a>

<!-- 
Diagrams are at: https://app.excalidraw.com/s/4Dr1S6qmmY7/206fDJsoMVz
See handbook to where images are stored: https://handbook.sourcegraph.com/handbook/editing/handbook-images-video/#adding-images-to-google-cloud-storage
-->

## Troubleshooting
Refer to the [Troubleshooting Executors](./executors_troubleshooting.md) document for common debugging operations.

