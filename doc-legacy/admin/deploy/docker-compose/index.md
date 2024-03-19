# Sourcegraph with Docker Compose

Setting up Docker applications with [multiple containers](https://www.docker.com/resources/what-container) like Sourcegraph using Docker Compose allows us to start all the applications with a single command. It also makes configuring the applications easier through updating the docker-compose.yaml and docker-compose.override.yaml files. Please see the [official Docker Compose docs](https://docs.docker.com/compose/) to learn more about Docker Compose.

This guide will take you through how to install Sourcegraph with Docker Compose on a server, which could be the local machine, a server on a local network, or cloud-hosted server. You can also follow one of the available *cloud-specific guides* listed below to prepare and install Sourcegraph on a supported cloud environment:

<div class="getting-started">
  <a class="btn btn-secondary text-center" href="aws">AWS</a>
  <a class="btn btn-secondary text-center" href="azure">Azure</a>
  <a class="btn btn-secondary text-center" href="digitalocean">DigitalOcean</a>
  <a class="btn btn-secondary text-center" href="google_cloud">Google Cloud</a>
</div>

## Prerequisites

  - Install [Docker Compose](https://docs.docker.com/compose/) on the server 
    - Minimum Docker [v20.10.0](https://docs.docker.com/engine/release-notes/#20100) and Docker Compose [v1.29.0](https://docs.docker.com/compose/release-notes/#1290)
    - Docker Swarm mode is **not** supported
  - Check the [resource estimator](../resource_estimator.md) for resource requirements
  - Obtain a [Sourcegraph license](https://sourcegraph.com/pricing/)
    - License is required for instances with **more than 10 users**
  - <span class="badge badge-beta">optional</span> Configure ingress firewall rules to enable secure access to the server

>WARNING: Running Sourcegraph on Windows or `ARM`/`ARM64` images is not supported for production deployments.

---

## Installation Steps

A step by step guide to install Sourcegraph with Docker Compose.

### Overview

 1. <span class="badge badge-note">RECOMMENDED</span> [Fork the deployment repository](#step-1-fork-the-deployment-repository)
 2. [Customize the instance](#step-2-configure-the-instance)
 3. [Clone the release branch](#step-3-clone-the-release-branch)
 4. [Build and start the Sourcegraph containers](#step-4-start-sourcegraph)

>NOTE: This guide is not limited to GitHub users. You can create a copy of the [deployment repository](https://github.com/sourcegraph/deploy-sourcegraph-docker/) in any code host. 

### Step 1: Fork the deployment repository

[`sourcegraph/deploy-sourcegraph-docker`](https://github.com/sourcegraph/deploy-sourcegraph-docker/) is the deployment repository for Docker Compose---it contains everything you need to install and configure a Sourcegraph Docker Compose instance. 

<span class="badge badge-note">RECOMMENDED</span> We **strongly recommend** you to deploy Sourcegraph using your own fork (or private copy) of the deployment repository as this allows you to track customizations made to the [Sourcegraph docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) easily. It also makes upgrading your instance easier in the future.

> NOTE: When forking the repository, make sure the box labeled, "Copy the master branch only", is unchecked. Checking this box will prevent the repository tags from being copied and will result in an error in a later step. 


> WARNING: In GitHub, the forks of public repositories are also public. Create a private copy following the [official docs on duplicating a repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/duplicating-a-repository) is strongly recommended if you plan to store secrets (SSL certificates, external Postgres credentials, etc.) within the repository. However, the preferable approach would be to use a Secrets Management Service. 


#### Create a public or private copy of the deployment repository

Use the GitHub GUI to [create a public fork](https://docs.github.com/en/get-started/quickstart/fork-a-repo#forking-a-repository) of the [sourcegraph/deploy-sourcegraph-docker](https://github.com/sourcegraph/deploy-sourcegraph-docker/) deployment repository

<details>
  <summary>**Or click here** for detailed instruction on creating a *private copy*</summary>

##### Using a private copy of the deployment repository

1\. Create an [empty private repository](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository), for example `<you/private-repository>` in GitHub.

2\. Bare clone the deployment repository. 

```bash
  git clone --bare https://github.com/sourcegraph/deploy-sourcegraph-docker/
```

3\. Navigate to the bare clone and mirror push it to your private repository.

```bash
  cd deploy-sourcegraph-docker.git
  git push --mirror https://github.com/<you/private-repository>.git
```

4\. Remove your local bare clone. 

```bash
  cd ..
  rm -rf deploy-sourcegraph-docker.git
```

5\. Private repository clone URL

If you are deploying using our start up scripts, please check with your code host on how to generate a URL for cloning private repository
For example, GitHub users can include their personal access token to clone repositories they have access to using the following URL:

```bash
# Please make sure to discard the token after the deployment for security purpose
https://<PERSONAL-ACCESS-TOKEN>@github.com/<USERNAME>/<REPO>.git
```

</details>

#### Configure your deployment repository

Continue with the following steps *after* you have created a public or private copy of the [deployment repository](https://github.com/sourcegraph/deploy-sourcegraph-docker/):

1\. Clone the publicly forked (or privately cloned) repository to your local machine. 

```bash
  git clone https://github.com/<you/private-repository>.git 
```

2\. Add the deployment repository maintained by Sourcegraph as the remote `upstream`.

  - This is to keep your clone synced with the upstream repository.

```bash
  git remote add upstream https://github.com/sourcegraph/deploy-sourcegraph-docker
```

3\. Create a new branch called `release` off the latest version of Sourcegraph

  - This branch will be used to [upgrade Sourcegraph](upgrade.md) and install your Sourcegraph instance.
  - It also allows us to track all of the customizations made to your Sourcegraph instance. 

```bash
  # Specify the version you want to install
  export SOURCEGRAPH_VERSION="v5.2.5"
  # Check out the selected version for use, in a new branch called "release"
  git checkout $SOURCEGRAPH_VERSION -b release
```

### Step 2: Configure the Instance

You can find the default [docker-compose.yaml](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/docker-compose.yaml) file inside the deployment repository.

If you would like to make changes to the default configurations, we highly recommend you create a new file called `docker-compose.override.yaml` in the same directory where the default docker-compose.yaml file is located, and make your customizations inside the [docker-compose.override.yaml](configuration.md#what-is-an-override-file) file.

- Here is a list of customizations you can make using an override file:
  - Add replicas
  - Adjust resources
  - Connect to an external database
  - Disable a service
  - Expose debug port
  - Git SSH configuration
  - Update or add new environment variables
  - Enable the Embeddings service
  - And more!

Please make sure to commit any changes to your `release` branch.

For detailed instructions on how to configure the instance using an override file, please refer to the [configuration docs](configuration.md).

> NOTE: Using an override file to customize your Sourcegraph instance is highly recommended as it is the best way to prevent merge conflicts during upgrades.

### Step 3: Clone the release branch

Now that you have customized your instance and published the changes to your code host, you will need to clone the newly configured `release` branch onto the production server: 

```bash
  git clone --branch release https://github.com/<you/private-repository>.git 
```

> NOTE: The `docker-compose.yaml` file currently depends on configuration files that live in the repository, so you must have the entire repository cloned onto your server.

### Step 4: Start Sourcegraph

On the production server, run the following command inside the [./docker-compose](https://github.com/sourcegraph/deploy-sourcegraph-docker/tree/master/docker-compose) directory to build and start Sourcegraph:

```bash
  # Go to the docker-compose configuration directory
  cd docker-compose
  # Start Sourcegraph with Docker Compose
  docker-compose up
  # OR you can start Sourcegraph with Docker Compose in a detached mode
  docker-compose up -d
```

To check if the server is ready, the `sourcegraph-frontend-0` service must be displayed as healthy:

```bash
  # Check the health status for sourcegraph-frontend-0
  docker ps --filter="name=sourcegraph-frontend-0"
```

Once the server is ready, navigate to the `sourcegraph-frontend-0` hostname or IP address on port `80`.  

---

## Additional Information

- [Upgrade](upgrade.md)
- [Management Operations](operations.md)
- [HTTP and HTTPS/SSL configuration](../../../admin/http_https_configuration.md#sourcegraph-via-docker-compose-caddy-2)
- [Site Administration Quickstart](../../../admin/how-to/site-admin-quickstart.md)
