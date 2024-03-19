# Upgrade Sourcegraph on Docker Compose

This document describes the process to update a Docker Compose Sourcegraph instance. If you are unfamiliar with sourcegraph versioning or releases see our [general concepts documentation](../../updates/index.md).

> ***⚠️ Attention: Always consult the [release notes](../../updates/docker_compose.md) for the versions your upgrade will pass over and end on.***

### Standard upgrades

A [standard upgrade](../../updates/index.md#upgrade-types) occurs between a Sourcegraph version and the minor or major version released immediately after it. If you would like to jump forward several versions, you must perform a [multi-version upgrade](#multi-version-upgrades) instead.

If you've [configured Docker Compose with a release branch](index.md#step-1-prepare-the-deployment-repository), please merge the upstream release tag for the next minor version into your `release` branch. In the following example, the release branch is being upgraded to v3.43.2.

```sh
# first, checkout the release branch
git checkout release
# fetch updates
git fetch upstream
# merge the upstream release tag into your release branch
git checkout release
git merge v3.43.2
```

#### Address any merge conflicts you might have

For each conflict, you need to reconcile any customizations you made with the updates from the new version. Use the information you gathered earlier from the change log and changes list to interpret the merge conflict and to ensure that it doesn't over-write your customizations. You may need to update your customizations to accommodate the new version.

> NOTE: If you have made no changes or only very minimal changes to your configuration, you can also ask git to always select incoming changes in the event of merge conflicts. In the following example merges will be accepted from the upstream version v3.43.2:
>
> `git merge -X theirs v3.43.2`
>
> If you do this, make sure your configuration is correct before proceeding because it may have made changes to your docker-compose YAML file.

#### Clone the updated release branch to your server

SSH into your instance and navigate to the appropriate folder:

```sh
# AWS
cd /home/ec2-user/deploy-sourcegraph-docker/docker-compose
# Azure, Digital Ocean, Google Cloud
cd /root/deploy-sourcegraph-docker/docker-compose
```

Download all the latest docker images to your local docker daemon:

```sh
$ docker-compose pull --include-deps
```

Restart Docker Compose using the new minor version along with your customizations:

```sh
$ docker-compose up -d --remove-orphans
```

### Multi-version upgrades

If you are upgrading to **Sourcegraph 5.1 or later**, we encourage you to perform an [**automatic multi-version upgrade**](../../updates/automatic.md). The following procedure has been automated, but is still applicable should errors occur in an automated upgrade.

---

> **⚠️ Attention:** please see our [cautionary note](../../updates/index.md#best-practices) on upgrades, if you have any concerns about running a multiversion upgrade, please reach out to us at [support@sourcegraph.com](emailto:support@sourcegraph.com) for advisement.

To perform a **manual** multi-version upgrade on a Sourcegraph instance running on Docker compose follow the procedure below:

1. **Check Upgrade Readiness**:
   - Check the [upgrade notes](../../updates/docker_compose.md#docker-compose-upgrade-notes) for the version range you're passing through.
   - Check the `Site Admin > Updates` page to determine [upgrade readiness](../../updates/index.md#upgrade-readiness).

2. **Disable Connections to the Database**:
   - Run the following command in the directory containing your `docker-compose.yaml` file.
  ```sh
  $ docker-compose stop && docker-compose up -d pgsql codeintel-db codeinsights-db
  ```
3. **Run Migrator with the `upgrade` command**:
   - The following procedure describes running migrator in brief, for more detailed instructions and available command flags see our [migrator docs](../../updates/migrator/migrator-operations.md#docker-compose).
    1. Set the migrator `image:` in your `docker-compose.yaml` to the **latest** release of `migrator`. **Example:**
    ```yaml
    migrator:
      container_name: migrator
      image: 'index.docker.io/sourcegraph/migrator:5.0.4'
    ```
    > *Note: Always use the latest image version of migrator for migrator commands, except the startup command `up`*
    2. Set the migrator `command:` to `upgrade` you'll need to supply a `--to=` argument. **Example:**
    ```yaml
    command: ['upgrade', '--from=v4.1.2', '--to=v4.4.0']
    ```
    > *Note: you may add the `--dry-run` flag to the `command:` to test things out before altering the dbs*
    3. Run migrator with `docker-compose up migrator` **Example:**
    ```sh
    $ ~/deploy-sourcegraph-docker/docker-compose/ docker-compose up migrator
    codeintel-db is up-to-date
    codeinsights-db is up-to-date
    pgsql is up-to-date
    Recreating migrator ... done
    Attaching to migrator
    migrator                         | ❗️ An error was returned when detecting the terminal size and capabilities:
    migrator                         |
    migrator                         |    GetWinsize: inappropriate ioctl for device
    migrator                         |
    migrator                         |    Execution will continue, but please report this, along with your operating
    migrator                         |    system, terminal, and any other details, to:
    migrator                         |      https://github.com/sourcegraph/sourcegraph/issues/new
    migrator                         |
    migrator                         | ✱ Sourcegraph migrator 4.4.0
    migrator                         | 👉 Migrating to v4.3 (step 1 of 2)
    migrator                         | 👉 Running schema migrations
    migrator                         | ✅ Schema migrations complete
    migrator                         | 👉 Running out of band migrations [17 18]
    ✅ Out of band migrations complete
    migrator                         | 👉 Migrating to v4.4 (step 2 of 2)
    migrator                         | 👉 Running schema migrations
    migrator                         | ✅ Schema migrations complete
    migrator                         | migrator exited with code 0
    ```

4. **Pull and merge upstream changes**:
   - Follow the [standard upgrade procedure](#standard-upgrades) to pull and merge upstream changes from the version you are upgrading to to your `release` branch.
   - **⚠️ Attention:** *merging upstream changes should set the migrator `image:` version back to the release you are upgrading to, and the `command:` should be set back to `up`, this is necessary to start your instance again.*

5. **Start your containers again**:
   - run `docker-compose up -d` in the folder containing your `docker-compose.yaml` file.
   ```sh
   $ docker-compose up -d
   ```
