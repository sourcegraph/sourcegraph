# Deploying Executors Binary Offline

When running in an air-gap environment, the executor binary can be deployed with this guide.

## Initial Dependencies

Executors
require [initial dependencies](https://docs.sourcegraph.com/admin/executors/deploy_executors_binary#dependencies) to
be installed on the host machine. The minimum dependencies (when not using [Firecracker](index.md#firecracker)
Isolation) are:

- [Docker](https://docs.docker.com/engine/install/binaries/#install-daemon-and-client-binaries-on-linux)
- [Git](https://git-scm.com/download/linux)

## Install Binary

1. Download the executor binary version that matches your deployed Sourcegraph Version (e.g. `v4.1.0`) from a machine
   with internet access
      ```shell
      curl -sfLo executor https://storage.googleapis.com/sourcegraph-artifacts/executor/${SOURCEGRAPH_VERSION}/linux-amd64/executor
      ```
2. Copy the `executor` binary to the offline host machine
3. Set the binary as executable: `chmod +x executor`
4. Move the binary to a location in your `$PATH` (e.g. `/usr/local/bin`)

## Configure Docker

`executor` requires the ability to connect to a Docker Registry to pull Docker Images. The offline host machine needs to
be able to connect to an internal Docker Registry (e.g. JFrog Artifactory) to be able to
pull the images.

## Environment Variables

See [deploy executors binary](./deploy_executors_binary.md#step-2-setup-environment-variables) for a list of environment
variables that are configurable.

## Batch Changes

Batch Changes requires either `src-cli` to be installed on the host machine or
for [Native Execution](native_execution.md) to be enabled.

### src-cli

Executors requires the `src-cli` to be installed on the host machine, if not
using [Native Execution](native_execution.md) for Batch Changes. To install `src-cli`:

1. Download the `src-cli` binary [version](https://github.com/sourcegraph/src-cli/releases) that matches your deployed
   Sourcegraph Version (e.g. `v4.1.0`) from a machine with internet access.
2. Copy the `src` binary to the offline host machine
3. Extract the binary from the archive
      ```shell
      $ tar -zxcf src-cli_${VERSION}_linux_amd64.tar.gz
      ```
4. Set the binary as executable by running `chmod +x src`
5. Move the binary to a location in your `$PATH` (e.g. `/usr/local/bin`)
6. Confirm `src` is installed by running `src`
      ```shell
      $ src version
      Current version: 4.1.0
      ```

### Native Execution

See [Native Execution](native_execution.md) for details on how to enable Native Execution. Ensure the
image `sourcegraph/batcheshelper` is available in the internal Docker Registry.

## Auto Indexing

Auto Indexing requires images to be available in the internal Docker Registry. The images for languages can be found in 
the [Code Navigation](../../code_navigation/index.md) page.

Once the images are available in the internal Docker Registry, the `executor` can be configured to use the images by 
updating `codeIntelAutoIndexing.indexerMap` in the **Site configuration**. For example,

```json
"codeIntelAutoIndexing.indexerMap": {
  "go": "my.company/lsif-go:custom",
}
```

## Firecracker Setup

See [Firecracker details](./firecracker.md) to determine if firecracker fits your use case. If you are using
Firecracker, you will need to install additional dependencies.

If you are not using Firecracker, ensure the environment variable `EXECUTOR_USE_FIRECRACKER` is set to `false`.

### Initial Dependencies

Executors running Firecracker Isolation
require [initial dependencies](https://docs.sourcegraph.com/admin/executors/deploy_executors_binary#dependencies) to
be installed on the host machine.

- `dmsetup`
- `losetup`
- `mkfs.ext4`
- `strings`
  - If not already installed (part of `binutils`)

### Install CNI

In order for `ignite` to function properly, CNI Plugins must be installed. To install CNI Plugins:

1. Download
   the [CNI Plugins](https://github.com/containernetworking/plugins/releases/download/v0.9.1/cni-plugins-linux-amd64-v0.9.1.tgz)
   and [CNI Isolation](https://github.com/AkihiroSuda/cni-isolation/releases/download/v0.0.4/cni-isolation-amd64.tgz)
   archives on a machine with internet access
2. Copy the archives to the offline host machine
3. Create the `/opt/cni/bin` directory
      ```shell
      $ mkdir -p /opt/cni/bin
      ```
4. Extract the archives to the `/opt/cni/bin` directory
      ```shell
      $ tar -zxcf cni-plugins-linux-amd64-v0.9.1.tgz -C /opt/cni/bin
      $ tar -zxcf cni-isolation-amd64.tgz -C /opt/cni/bin
      ```

### Install Ignite

Executors use `ignite` to spawn Firecracker VMs to run code in isolation. To install `ignite`:

1. Download [`ignite`](https://github.com/sourcegraph/ignite/releases/download/v0.10.5/ignite-amd64) on a machine with
   internet access
2. Copy `ignite-amd64` to the offline host machine
3. Set the binary as executable by running `chmod +x ignite-amd64`
4. Move the binary to a location in your `$PATH` (e.g. `/usr/local/bin`)
5. Confirm `ignite` is installed by running `ignite`
      ```shell
      $ ignite version
      Ignite version: version.Info{Major:"0", Minor:"8", GitVersion:"v0.10.0", GitCommit:"...", GitTreeState:"clean", BuildDate:"...", GoVersion:"...", Compiler:"gc", Platform:"linux/amd64"}
      Firecracker version: v0.22.4
      Runtime: containerd
      ```

### Install IPTables

IPTables prevent Firecracker from talking on Private IPv4 Address (
see [Firecracker details](./firecracker.md#known-caveats)). To install IPTables, the `executor` binary has a command to
install IPTables rules:

```shell
$ executor install iptables-rules
```

### Install Images

`ignite` requires three Docker Images to be made available on the offline host machine. To install the images, the
offline host machine needs to be able to connect to an internal Docker Registry (e.g. JFrog Artifactory) to be able to
pull the images.

#### Executor VM Image

To install the `executor-vm` image (ensure the version of the image matches your deployment version), import the image using `ignite`.

```shell
$ ignite image import --runtime docker <docker repository image for sourcegraph/executor-vm:your-version>
```  

If you are using a custom image instead of the Sourcegraph image, you will need to set the environment variable 
`EXECUTOR_FIRECRACKER_IMAGE` to match the image name. 

#### Sandbox Image

To install the Firecracker sandbox image, import the image using `docker`.

```shell
$ docker pull <docker repository image for sourcegraph/ignite:v0.10.5>
```

> Note: Check the [version](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/cmd/executor/internal/config/consts.go?L15) against the version of executors being installed.

If you are using a custom image instead of the Sourcegraph image, you will need to set the environment variable
`EXECUTOR_FIRECRACKER_SANDBOX_IMAGE` to match the image name.

#### Kernel Image

To install the Firecracker Kernel image, import the image (`sourcegraph/ignite-kernel:5.10.135-amd64`) using `ignite`.

```shell
$ ignite kernel import --runtime docker <docker repository image for sourcegraph/ignite-kernel:5.10.135-amd64>
```

> Note: Check the [version](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@main/-/blob/cmd/executor/internal/config/consts.go?L18) against the version of executors being installed.

If you are using a custom image instead of the Sourcegraph image, you will need to set the environment variable
`EXECUTOR_FIRECRACKER_KERNEL_IMAGE` to match the image name.

## Validation

Once the `executor` binary is installed and dependencies are met, you can validate the installation by running:

```shell
$ executor validate
```
