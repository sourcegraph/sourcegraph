# Reference Repositories

Sourcegraph provides reference repositories with branches corresponding to the version of Sourcegraph you wish to deploy for each supported deployment type. The reference repository contains everything you need to spin up and configure your instance depending on your deployment type, which also assists in your upgrade process going forward.

## List

| **Deployment type**       | **Link to reference repository**                         |
|:--------------------------|:---------------------------------------------------------|
| Kubernetes                | https://github.com/deploy-sourcegraph-k8s                |
| Helm                      | https://github.com/sourcegraph/deploy-sourcegraph-helm   |
| Docker and Docker Compose | https://github.com/sourcegraph/deploy-sourcegraph-docker |

> WARNING: [deploy-sourcegrap](https://github.com/deploy-sourcegrap) has been deprecated

## Create a private copy

### Step 1: Create an empty repository

Follow the [official GitHub docs](https://docs.github.com/en/repositories/creating-and-managing-repositories/creating-a-new-repository) on creating a new **empty** repository.

### Step 2: Set the environment variables

Export the following environment variables for the next steps.

- `SG_DEPLOY_REPO_NAME`: name of the deployment repository
  - `deploy-sourcegraph-k8s` for Kubernetes with Kustomize deployment
  - `deploy-sourcegraph-docker` for Docker and Docker Compose deployment
- `DEPLOY_GITHUB_USERNAME`: the account name that is hosting the empty repository created in step 1 
- `SG_PRIVATE_DEPLOY_REPO_NAME`: default to the same name as $SG_DEPLOY_REPO_NAME
- `SG_DEPLOY_VERSION`: latest version number of Sourcegraph

Update the environment variables in the command below before running it in your terminal:

```bash
export SG_DEPLOY_GITHUB_USERNAME="YOUR_USERNAME"
export SG_DEPLOY_REPO_NAME="deploy-sourcegraph-k8s"
export SG_PRIVATE_DEPLOY_REPO_NAME="$SG_DEPLOY_REPO_NAME"
export SG_DEPLOY_VERSION="v4.5.0"
```

### Step 3: Create remote and local copies

Once the required environment variables are exported, run the following commands in the same terminal:

```bash
git clone --bare https://github.com/sourcegraph/$SG_DEPLOY_REPO_NAME
cd $DEPLOY_REPO.git
git push --mirror https://github.com/SG_DEPLOY_GITHUB_USERNAME/$SG_PRIVATE_DEPLOY_REPO_NAME.git
cd ..
rm -rf $DEPLOY_REPO.git
git clone https://github.com/SG_DEPLOY_GITHUB_USERNAME/$SG_PRIVATE_DEPLOY_REPO_NAME.git
```

### Step 4: Create a release branch

Create a `release` branch to track all of your customizations to Sourcegraph. This branch will be used to [upgrade Sourcegraph](../updates.md) and [install your Sourcegraph instance](./index.md#installation).

```bash
cd $SG_PRIVATE_DEPLOY_REPO_NAME
git checkout $SG_DEPLOY_VERSION -b release-$SG_DEPLOY_VERSION
```

You can now deploy using your private copy of the repository you've just created. Please follow the installation and configuration docs for your specific deployment type for next steps.

## Update your private copy

Before you can upgrade Sourcegraph, you will first update your private copy with the upstream branch, and then merge the upstream release tag for the next minor version into your release branch. 

In the following example, the release branch is being upgraded to v4.5.1.

```bash
export YOUR_RELEASE_BRANCH=release-$SG_DEPLOY_VERSION
# first, checkout the release branch
git checkout $YOUR_RELEASE_BRANCH
# fetch updates
git fetch upstream
# merge the upstream release tag into your release branch
git checkout $YOUR_RELEASE_BRANCH
git merge v4.5.1
```

A [standard upgrade](../updates.md#standard-upgrades) occurs between two minor versions of Sourcegraph. If you are looking to jump forward several versions, you must perform a [multi-version upgrade](../updates.md#multi-version-upgrades) instead.

