# Serving local repositories

Sourcegraph is designed to abstract away all repository management operations from the user. However, we still often get the question: "how do I load local repositories?"

The instructions below (using the Sourcegraph CLI to act as a pseudo-code host for Sourcegraph to interact with) do enable this, but they can be quite complex. The root of the challenge is that Sourcegraph assumes that code will change over time. It has been intentionally designed to save the user from the manual effort of keeping repositories up to date (i.e., re-cloning or fetching the latest changes every time you want to search) by [automatically keeping repositories updated](https://docs.sourcegraph.com/admin/repo/update_frequency), keeping the updated content indexed, and making the change history visible and explorable to users.

And, practically, Sourcegraph was built with the assumption that certain tasks, such as getting a list of repositories and running git operations like clones, would be accessible over a network rather than on the same disk (i.e., that there would be HTTP API and git endpoints).

In both the short-term and the long-term, it is far easier for users to [connect Sourcegraph to a code host](https://docs.sourcegraph.com/admin/external_service) than to try to load code from a local disk, and we strongly encourage users, where possible, to connect to their code host of choice instead of trying to follow the instructions below. 

When Sourcegraph is connected to a code host, none of that code is ever sent off of your local Sourcegraph deployment, and nobody that you haven't given access to (whether at Sourcegraph or anywhere else) has access to your code. Sourcegraph only maintains a local clone, and does all code analysis and indexing operations locally. Read more specifics about our policies and what we do collect in [our security overview](https://sourcegraph.com/security/#Sourcegraph-on-premise).

## Using the Sourcegraph CLI to serve local repositories

The [Sourcegraph CLI (`src)`](https://github.com/sourcegraph/src-cli) provides the command `src serve-git` which will recursively serve up the git repositories it finds in the current directory. It exposes the repositories over HTTP as well as an API for Sourcegraph to query.

>NOTE: If using Perforce, see the [Perforce repositories with Sourcegraph guide](../repo/perforce.md).

The most common use-case for `src serve-git` is to create git repos that do not exist on a code host for Sourcegraph. For example:

- Splitting up a monorepo into a repository per project.
- Post-processing repositories for search. eg: extracting JAR files or committing files generated in the build process.
- Using `git p4` to serve up Perforce repositories.
- Serve up local repositories to Sourcegraph while trialling it.

> WARNING: `src serve-git` is not intended to be used to serve repos from a code host which is already configured to be synced by a seperate code host config. For instance if you have a self managed GitLab code host, it is not advised to use `src serve-git` to serve repos on the gitlab server. 

## Quickstart

1. [Install Sourcegraph CLI](https://github.com/sourcegraph/src-cli#installation) (`src`).
1. Run `src serve-git` in a directory with git repositories. Ensure the address is reachable by Sourcegraph.
1. Go to **Site admin > Manage code hosts > Add repositories**
1. Select **Sourcegraph CLI Serve-Git**.
1. Configure the URL field to the address for `src serve-git`.
1. Press **Add repositories**.

**IMPORTANT:** If you are running Sourcegraph in docker and are using a Linux host machine, replace `host.docker.internal` in the above with the IP address of your actual host machine because `host.docker.internal` [does not work on Linux](https://github.com/docker/for-linux/issues/264). You should use the network-accessible IP shown by `ifconfig` (rather than 127.0.0.1 or localhost).

## Docker

src-cli publishes Docker images which can be used instead of the binary. For example to publish your current directory run:

```
docker run \
  --rm=true \
  --publish 3434:3434 \
  --volume $PWD:/data/repos:ro \
  sourcegraph/src-cli:latest serve-git /data/repos
```

To confirm this is working visit http://localhost:3434

## src-expose

Before Sourcegraph 3.19 we recommend users to still use [`src-expose`](non-git.md).
