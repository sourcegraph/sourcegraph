# Pre-load repositories already cloned to disk

> NOTE: This page describes how to use a local copy of a repository to speed up the initial clone that Sourcegraph must do. This does not replace [adding a code host connection](add.md). You still must add the code host connection in order for Sourcegraph to recognize that this repository exists and to view it.
>
>Sourcegraph's [`src-expose` tool](../external_service/non-git.md) can be used to serve local repositories without a dedicated code host.

You can use repositories that are already cloned to disk on the host machine to speed up Sourcegraph's cloning. This is useful for very large repositories on which cloning exceeds the resources available to the Docker container. This is not intended for individual users who want to set up a personal Sourcegraph instance just for searching code on their own local disk; we recommend either using a CLI tool such as ripgrep instead, or simply connecting Sourcegraph to your code host with a limited set of repositories.

The steps documented here are intended for [single-container Sourcegraph instances](../deploy/docker-single-container/index.md). The general process also applies for other deployment methods, with some differences:

- [Docker Compose](../deploy/docker-compose/index.md): you need to perform these steps on the relevant [Docker Compose volumes](../deploy/docker-compose/index.md#manage-storage).
- [Kubernetes](../deploy/kubernetes/index.md): you need to perform these steps on the underlying node hosting the `gitserver` pod, or on the persistent volume used by the `gitserver` deployment.

> WARNING: For [single-container Sourcegraph instances](../deploy/docker-single-container/index.md), Sourcegraph will alter the contents and structure of files under `/var/opt/sourcegraph` (Sourcegraph’s data volume inside the container), so do not mount repositories in use by other processes under that directory.

If you're using the default `--volume $HOME/.sourcegraph/data:/var/opt/sourcegraph` argument to run the `sourcegraph/server` Docker image, and the repository you want to add is named `github.com/my/repo`, then follow these steps:

1.  Ensure that the added repository is included in [a code host configuration on Sourcegraph](../external_service/index.md).

2.  Stop Sourcegraph if it is running. This is to ensure it doesn't interfere with the clone.

3.  On the host machine, ensure that a bare Git clone of the repository exists at `$HOME/.sourcegraph/repos/github.com/my/repo/.git`.

    To create a new clone given its clone URL:

    ```
    git clone --mirror YOUR-REPOSITORY-CLONE-URL $HOME/.sourcegraph/repos/github.com/my/repo/.git
    ```

    Or, as an optimization, you can reuse an existing local clone to avoid needing to fetch all the repository data again:

    ```
    git clone --mirror --reference PATH-TO-YOUR-EXISTING-LOCAL-CLONE --dissociate YOUR-REPOSITORY-CLONE-URL $HOME/.sourcegraph/repos/github.com/my/repo/.git
    ```

If this repository exists on a code host that Sourcegraph integrates with, then use that code host's configuration (as described in the [code host documentation](../external_service/index.md)). After updating the code host configuration, if you used the correct repository path, Sourcegraph will detect and reuse the existing clone. (For example, if you're working with a repository on GitHub.com, ensure that the repository path name you used is of the form `github.com/my/repo`.)
