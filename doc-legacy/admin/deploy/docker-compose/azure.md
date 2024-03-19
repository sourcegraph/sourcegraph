# Install Sourcegraph on Azure

> ⚠️ We recommend new users use our [machine image](../machine-images/index.md) or [script-install](../single-node/script.md) instructions, which are easier and offer more flexibility when configuring Sourcegraph. Existing customers can reach out to our Customer Engineering team support@sourcegraph.com if they wish to migrate to these deployment models.

This guide will take you through how to set up a Sourcegraph instance on an Azure virtual machine with [Docker Compose](https://docs.docker.com/compose/).

---

## Configure

In the [Azure Quickstart Center](https://portal.azure.com/?quickstart=true#view/Microsoft_Azure_Resources/QuickstartCenterBlade), click `Deploy a virtual machine` to `Create a virtual machine`, then configure the instance following the instructions below for each section:

#### Basics

* `Virtual machine name:` Give your virtual machine a name
* `Availability options:` No infrastructure redundancy required
* `Image:` Ubuntu Server 18.04 LTS - Gen2
* `VM architecture:` x64
* `Size:` Select an appropriate instance type using our [resource estimator](../resource_estimator.md) as reference
* `Authentication type:` Select one that works best for you. SSH Key is recommended.
* `Inbound port rules:` Allowed selected ports
* `Select inbound ports:` HTTP (80), HTTPS (443), SSH (22)

#### Disks

* `OS disk type:` SSD is required --Premium SSD (Recommended) or Standard SSD
* `Delete with VM:` Unchecked

#### Disks > Data disks

Click `Create and attach a new disk` to create **two** disks:

* **Disk 1** - storage for root
   - `Source type:` None (empty disk)
   - `Size:` 50GB
   - `Performance tier:` 5000 IOS (Recommended)
   - `Enable shared disk:` No
   - `Delete disk with VM:` Checked
   - `Host caching:` Read/write
   - `LUN:` 0
* **Disk 2** - storage for the Sourcegraph instance
   - `Source type:` None (empty disk)
   - `Size:` Minimum 250GB
         * Sourcegraph needs at least as much space as all your repositories combined take up
         * Allocating as much disk space as you can upfront minimize the need for [expanding your volume](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/expand-disks) in the future
   - `Performance tier:` 5000 IOS (Recommended)
   - `Enable shared disk:` No
   - `Delete disk with VM:` Unchecked
   - `Host caching:` Read/write
   - `LUN:` 1

> NOTE: Typically, the device name for the `LUN: 0` disk is `dev/sda` while the device name for the `LUN: 1` disk is `dev/sdb` --this is important to note as our startup script mounts the disks based on device names (`PERSISTENT_DISK_DEVICE_NAME`).

#### Networking

* `Inbound port rules:` Allowed selected ports
* `Select inbound ports:` HTTP (80), HTTPS (443), SSH (22)

> NOTE: If possible, replace the IP address ranges specified with the IPs from which you actually want to allow access.

#### Management

* <span class="badge badge-note">RECOMMENDED</span> Endable backup

#### Advanced

* Enable `user data`
* In the **Custom data** and **User Data** text boxes, copy and paste the [startup script](#startup-script) from below 

##### Startup script

```bash
#!/usr/bin/env bash
set -euxo pipefail
###############################################################################
# ACTION REQUIRED: REPLACE THE URL AND REVISION WITH YOUR DEPLOYMENT REPO INFO
###############################################################################
# Please read the notes below the script if you are cloning a private repository
DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL='https://github.com/sourcegraph/deploy-sourcegraph-docker.git'
DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION='v5.2.5'
##################### NO CHANGES REQUIRED BELOW THIS LINE #####################
DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT='/root/deploy-sourcegraph-docker'
DOCKER_COMPOSE_VERSION='1.29.2'
DOCKER_DAEMON_CONFIG_FILE='/etc/docker/daemon.json'
DOCKER_DATA_ROOT='/mnt/docker-data'
PERSISTENT_DISK_DEVICE_NAME='/dev/sdb'
PERSISTENT_DISK_LABEL='sourcegraph'
# Install git
sudo apt-get update -y
sudo apt-get install -y git
# Clone the deployment repository
git clone "${DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL}" "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"
cd "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"
git checkout "${DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION}"
# Format (if unformatted) and then mount the attached volume
device_fs=$(sudo lsblk "${PERSISTENT_DISK_DEVICE_NAME}" --noheadings --output fsType)
if [ "${device_fs}" == "" ]
then
    sudo mkfs.ext4 -m 0 -E lazy_itable_init=0,lazy_journal_init=0,discard "${PERSISTENT_DISK_DEVICE_NAME}"
fi
sudo e2label "${PERSISTENT_DISK_DEVICE_NAME}" "${PERSISTENT_DISK_LABEL}"
sudo mkdir -p "${DOCKER_DATA_ROOT}"
sudo mount -o discard,defaults "${PERSISTENT_DISK_DEVICE_NAME}" "${DOCKER_DATA_ROOT}"
# Mount file system by label on reboot
sudo echo "LABEL=${PERSISTENT_DISK_LABEL}  ${DOCKER_DATA_ROOT}  ext4  discard,defaults,nofail  0  2" | sudo tee -a /etc/fstab
sudo umount "${DOCKER_DATA_ROOT}"
sudo mount -a
# Install, configure, and enable Docker
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
sudo apt-get update -y
sudo apt-get install -y software-properties-common
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
sudo apt-get update -y
apt-cache policy docker-ce
apt-get install -y docker-ce docker-ce-cli containerd.io
## Enable Docker at startup
sudo systemctl enable --now docker
## Install jq for scripting
sudo apt-get update -y
sudo apt-get install -y jq
## Initialize the config file with empty json if it doesn't exist
if [ ! -f "${DOCKER_DAEMON_CONFIG_FILE}" ]
then
    mkdir -p $(dirname "${DOCKER_DAEMON_CONFIG_FILE}")
    echo '{}' >"${DOCKER_DAEMON_CONFIG_FILE}"
fi
## Point Docker storage to mounted volume
tmp_config=$(mktemp)
trap "rm -f ${tmp_config}" EXIT
sudo cat "${DOCKER_DAEMON_CONFIG_FILE}" | sudo jq --arg DATA_ROOT "${DOCKER_DATA_ROOT}" '.["data-root"]=$DATA_ROOT' >"${tmp_config}"
sudo cat "${tmp_config}" >"${DOCKER_DAEMON_CONFIG_FILE}"
## Restart Docker daemon to pick up new changes
sudo systemctl restart --now docker
# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
curl -L "https://raw.githubusercontent.com/docker/compose/${DOCKER_COMPOSE_VERSION}/contrib/completion/bash/docker-compose" -o /etc/bash_completion.d/docker-compose
# Start Sourcegraph with Docker Compose
cd "${DEPLOY_SOURCEGRAPH_DOCKER_CHECKOUT}"/docker-compose
docker-compose up -d --remove-orphans
```

> NOTE: If you're deploying a production instance, we recommend [forking the deployment configuration repository](./index.md#step-1-fork-the-deployment-repository) to track any customizations you make to the deployment config. If you do so, you'll want to update the *startup script* you pasted from above to refer to the clone URL and revision of your fork:
> 
> - `DEPLOY_SOURCEGRAPH_DOCKER_FORK_CLONE_URL`: The Git clone URL of your deployment repository. If it is a private repository, please check with your code host on how to generate a URL for cloning private repository
> - `DEPLOY_SOURCEGRAPH_DOCKER_FORK_REVISION`: The revision (branch) in your fork containing the customizations, typically "release"

---

## Deploy

1. Click **Review + create** to create the instance
  - Please review the configurations and make sure the validation has passed before creating the instance 


2. Navigate to the `public IP address` assigned to your instance to visit your newly created instance
  - Look for the `Public IP address` in your Virtual Machine dashboard under *Networking* in the *Properties* tab

>NOTE: It may take a few minutes for the instance to finish initializing before Sourcegraph becomes accessible.

You can monitor the setup process by SSHing into the instance to run the following diagnostic commands:

```bash
# Follow the status of the startup script
tail -c +0 -f /var/log/syslog | grep cloud-init
# Once installation is completed, check the health of the "sourcegraph-frontend" container
docker ps --filter="name=sourcegraph-frontend-0"
```

> NOTE: If you have configured a DNS entry for the IP, please ensure to update `externalURL` in your Sourcegraph instance's Site Configuration to reflect that

---

## Upgrade

See the [Docker Compose upgrade docs](upgrade.md).

---

## Storage and Backups

Data is persisted within a [Docker volume](https://docs.docker.com/storage/volumes/) as defined in the [deployment repository](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml). The startup script configures Docker using a [daemon configuration file](https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file) to store all the data on the attached data volume, which is mounted at `/mnt/docker-data`, where volumes are stored within `/mnt/docker-data/volumes`.

The most straightforward method to [backup the data](https://docs.microsoft.com/en-us/azure/virtual-machines/backup-and-disaster-recovery-for-azure-iaas-disks) is to [enable incremental snapshot](https://docs.microsoft.com/en-us/azure/virtual-machines/disks-incremental-snapshots?tabs=azure-cli)

<span class="badge badge-note">RECOMMENDED</span> Using an external Postgres service such as [Azure Database for PostgreSQL](https://learn.microsoft.com/en-us/azure/postgresql/) takes care of backing up all the user data for you. If the Sourcegraph instance ever dies or gets destroyed, creating a fresh new instance connected to the old external Postgres service will get Sourcegraph back to its previous state.

---

## Other resources

[HTTP and HTTPS/SSL configuration](../../../admin/http_https_configuration.md#sourcegraph-via-docker-compose-caddy-2)
[Site Administration Quickstart](../../../admin/how-to/site-admin-quickstart.md)
