---
title: Install Sourcegraph via Shell Script
---

# Install Sourcegraph via Shell Script

Following these docs will launch a pre-configured single node Sourcegraph instance via shell script.

---

## Supported Distros
The script can be run on the following distros. 

- Debian
- Ubuntu
- Fedora
- RHEL
- Amazon Linux 2

As it is impossible to test all possible deployment environments it is possible albeit unlikely to encounter an error. We highly suggest using one of our
curated [Machine Images](../machine-images/index.md) if possible.

## Instance Size Chart

Determine the instance type required to support the number of users and repositories you have using this table. If you fall between two sizes, choose the larger of the two.

For example, if you have 8,000 users with 80,000 repositories, your instance size would be **L**. If you have 1,000 users with 80,000 repositories, you should go with size **M**.

|                            | **XS**     | **S**       | **M**        | **L**        | **XL**       |
|----------------------------|------------|-------------|--------------|--------------|--------------|
| **Users**                  | _<=_ 500   | _<=_ 1,000  | _<=_ 5,000   | _<=_ 10,000  | _<=_ 20,000  |
| **Repositories**           | _<=_ 5,000 | _<=_ 10,000 | _<=_ 50,000  | _<=_ 100,000 | _<=_ 250,000 |
| **Recommended vCPU / RAM** | 8 / 32 GiB | 16 / 64 GiB | 32 / 128 GiB | 48 / 192 GiB | 96 / 384 GiB |
| **Minimum vCPU / RAM**     | 8 / 32 GiB | 8 / 32 GiB  | 16 / 64 GiB  | 32 / 128 GiB | 48 / 192 GiB |


## Machine Prerequisites
Before running the install script please ensure ensure the target machine meets the following prerequisites.

1. Sufficient CPU and RAM according to the [instance size chart](#instance-size-chart)
2. A root disk size with `50 GiB` free space for ephemeral data.
3. A secondary disk of size `250 GiB` **minimum**. Adjust to your specific needs.
4. A non-root user with `sudo` permissions to execute the script

<span class="badge badge-critical">IMPORTANT</span> For optimal performance both disks should be backed by SSDs or equivalently fast storage.

## Deploy Sourcegraph

The script accepts the following parameters:
#### Required

- `-d <device>`

    The data disk where Sourcegraph persistent data should be stored e.g. `-d /dev/sdb`

    The script will format the disk if it has not been formatted already.
    If you choose to format and mount the disk yourself it must be mounted to `/mnt/data`
    MUST be a separate disk than e.g. the base OS for upgrades.

#### Optional
- `-s <size>`

    The Instance Size to use. Defaults to `XS`
- `-v <version>`

    The Sourcegraph version to deploy. Defaults to the latest version

```sh
curl -sfL https://raw.githubusercontent.com/sourcegraph/deploy/main/install/scripts/k3s/install.sh | bash -s -- <args>
```

> NOTE: Please allow up to ~5 minutes for Sourcegraph to initialize

## Networking
Sourcegraph will be available on ports 80 and 443 (using a self signed certificate).

We recommend deploying your own reverse proxy to terminate TLS connections with a properly signed certificate.

## Upgrade
- [Changelog](https://docs.sourcegraph.com/CHANGELOG)
- [Update policy](https://docs.sourcegraph.com/admin/updates#update-policy)
- [Update notes](https://docs.sourcegraph.com/admin/updates/kubernetes)
- [Multi-version upgrade procedure](https://docs.sourcegraph.com/admin/updates/kubernetes#multi-version-upgrade-procedure)

<span class="badge badge-critical">IMPORTANT</span> **Back up your volumes before each upgrade**

1. Shutdown the current machine running the Sourcegraph instance
2. Detach the data disk
3. Launch a new machine with the same specification as the current instance
  - Use the existing data disk in place of creating a new one
4. Run the install script targeting the data disk.
5. Wait for Sourcegraph to start.

## Downgrade
Please refer to the upgrade procedure above if you wish to rollback your instance.

## Storage and Backups
We strongly recommend taking snaphots of the data disk on an automatic, scheduled basis.