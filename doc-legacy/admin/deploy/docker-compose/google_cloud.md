# Install Sourcegraph on Google Cloud

> ⚠️ We recommend new users use our [GCE machine image](../machine-images/gce.md) or [script-install](../single-node/script.md) instructions, which are easier and offer more flexibility when configuring Sourcegraph. Existing customers can reach out to our Customer Engineering team support@sourcegraph.com if they wish to migrate to these deployment models.

---

This guide will take you through how to deploy Sourcegraph with [Docker Compose](https://docs.docker.com/compose/) to a single node running on Google Cloud.

## Configure

Click **Create Instance** in your [Google Cloud Compute Engine Console](https://console.cloud.google.com/compute/instances) to create a new VM instance, then configure the instance following the instructions below for each section:

#### Machine configuration
1. Select an appropriate machine type using our [resource estimator](../resource_estimator.md) as reference
  
#### Boot disk
1. Click **CHANGE** to update the boot disk:
   * `Operating System`: Ubuntu
   * `Version`: Ubuntu 18.04 LTS (x86/64, amd64 bionic image)
   * `Boot disk type`: SSD persistent disk
   * `Size (GB)`: Use default

#### Firewall
1. Check box to **Allow HTTP traffic**
1. Check box to **Allow HTTPS traffic**

#### Advanced options > Disks

1. Expand the **Advanced options** section and the **Disks** section within to add an additional disk to store data from the Sourcegraph Docker instance.

1. Click **+ ADD NEW DISK** to setup the new disk with the following settings:
  * `Name`: "sourcegraph-docker-disk" (or something similarly descriptive)
  * `Description`: "Disk for storing Docker data for Sourcegraph" (or something similarly descriptive)
  * `Disk source type`: Blank disk
  * `Disk type`: SSD persistent disk
  * `Size`: `250GB` minimum
      * Sourcegraph needs at least as much space as all your repositories combined take up
      * Allocating as much disk space as you can upfront minimize the need for [resizing this disk](https://cloud.google.com/compute/docs/disks/add-persistent-disk#resize_pd) later on
  * `(optional, recommended) Snapshot schedule`: The most straightfoward way of automatically backing Sourcegraph's data is to set up a [snapshot schedule](https://cloud.google.com/compute/docs/disks/scheduled-snapshots) for this disk. We strongly recommend that you take the time to do so here.
  * `Attachment settings - Mode`: Read/write
  * `Attachment settings - Deletion rule`: Keep disk

#### Advanced options > Management

1. Expand the **Advanced options** section and the **Management** section within

2. Copy and paste the *Startup script* below into the **Automation** field

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
# IMPORTANT: DO NOT MAKE ANY CHANGES FROM THIS POINT ONWARD
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
if [ "${device_fs}" == "" ] ## only format the volume if it isn't already formatted
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
sudo cat "${DOCKER_DAEMON_CONFIG_FILE}" | sudo jq --arg DATA_ROOT "${DOCKER_DATA_ROOT}" '.["data-root"]=$DATA_ROOT' > "${tmp_config}"
sudo cat "${tmp_config}" > "${DOCKER_DAEMON_CONFIG_FILE}"
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

1. Click **CREATE** to create your VM with Sourcegraph installed
2. Navigate to the public IP address assigned to your instance to visit your newly created Sourcegraph instance

It may take a few minutes for the instance to finish initializing before Sourcegraph becomes accessible. 

You can monitor the setup process by SSHing into the instance to run the following diagnostic commands:

```bash
# Follow the status of the startup script
tail -c +0 -f /var/log/syslog | grep startup-script
# Once installation is completed, check the health of the "sourcegraph-frontend" container
docker ps --filter="name=sourcegraph-frontend-0"
```

> NOTE: If you have configured a DNS entry for the IP, please ensure to update `externalURL` in your Sourcegraph instance's Site Configuration to reflect that

---

## Upgrade

Please refer to the [Docker Compose upgrade docs](upgrade.md) for detailed instructions on updating your Sourcegraph instance.

---

## Storage and Backups

Data is persisted within a [Docker volume](https://docs.docker.com/storage/volumes/) as defined in the [deployment repository](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml). The startup script configures Docker using a [daemon configuration file](https://docs.docker.com/engine/reference/commandline/dockerd/#daemon-configuration-file) to store all the data on the attached data volume, which is mounted at `/mnt/docker-data`, where volumes are stored within `/mnt/docker-data/volumes`.

The most straightforward method to backup the data is to [snapshot the entire `/mnt/docker-data` volume](https://cloud.google.com/compute/docs/disks/create-snapshots) automatically using a [snapshot schedule](https://cloud.google.com/compute/docs/disks/scheduled-snapshots). You can also [set up a snapshot snapshot schedule](https://cloud.google.com/compute/docs/disks/scheduled-snapshots) after your instance has been created.

<span class="badge badge-note">RECOMMENDED</span> Using an external Postgres service such as [AWS RDS for PostgreSQL](https://aws.amazon.com/rds/) takes care of backing up all the user data for you. If the Sourcegraph instance ever dies or gets destroyed, creating a fresh new instance connected to the old external Postgres service will get Sourcegraph back to its previous state.

---

## Other resources

[HTTP and HTTPS/SSL configuration](../../../admin/http_https_configuration.md#sourcegraph-via-docker-compose-caddy-2)
[Site Administration Quickstart](../../../admin/how-to/site-admin-quickstart.md)
