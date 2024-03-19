# Administration troubleshooting

### Docker Toolbox on Windows: `New state of 'nil' is invalid`

If you are using Docker Toolbox on Windows to run Sourcegraph, you may see an error in the `frontend` log output:

```bash
frontend |
frontend |
frontend |
frontend |     New state of 'nil' is invalid.
```

After this error, no more `frontend` log output is printed.

This problem is caused by [docker/toolbox#695](https://github.com/docker/toolbox/issues/695#issuecomment-356218801) in Docker Toolbox on Windows. To work around it, set the environment variable `LOGO=false`, as in:

```bash
docker container run -e LOGO=false ... sourcegraph/server
```

See [sourcegraph/sourcegraph#398](https://github.com/sourcegraph/sourcegraph/issues/398) for more information.

> WARNING: Running Sourcegraph on Docker Toolbox for Windows is not supported for production deployments.

### Submitting a metrics dump

If you encounter performance or instability issues with Sourcegraph, we may ask you to submit a metrics dump to us. This allows us to inspect the performance and health of various parts of your Sourcegraph instance in the past and can often be the most effective way for us to identify the cause of your issue.

The metrics dump includes non-sensitive aggregate statistics of Sourcegraph like CPU & memory usage, number of successful and error requests between Sourcegraph services, and more. It does NOT contain sensitive information like code, repository names, user names, etc.

In the process of troubleshooting, we may also ask that you perform other tasks from the command line to help us assist you. Our [command line generator](https://sourcegraph.github.io/support-generator/) is a tool we've created to help users generate commands for various functions (i.e. retrieving logs from specific containers, port forwarding, checking dirty databases, etc.).

#### Docker Compose deployments

To create a metrics dump from a docker-compose deployment, follow these steps:

* Open a shell to the running `prometheus` container:

```sh
docker exec -it prometheus /bin/sh
```

* Inside the container bash shell trigger the creation of a Prometheus snapshot:  

```sh
wget --post-data "" http://localhost:9090/api/v1/admin/tsdb/snapshot
```

* Find the created snapshot's name:

```sh
ls /prometheus/snapshots/
```

* Tar up the created snapshot

```sh
cd /prometheus/snapshots && tar -czvf /tmp/sourcegraph-metrics-dump.tgz <snapshot-name>
```

* Switch back to local shell (`exit`) and copy the metrics dump file to the host machine:

```sh
docker cp prometheus:/tmp/sourcegraph-metrics-dump.tgz sourcegraph-metrics-dump.tgz
```

Please then upload the `sourcegraph-metrics-dump.tgz` file to Sourcegraph support so we can inspect it.

#### Single-container Sourcegraph deployments

To create a metrics dump from a [single-container `sourcegraph/server` deployment](deploy/docker-single-container/index.md), follow these steps:

* Open a shell to the running container:
    1. Run `docker ps` to get the name of the Sourcegraph server container.
    1. Run `docker exec -it <container name> /bin/bash` to open a bash shell.
* Inside the container bash shell trigger the creation of a Prometheus snapshot:  

```sh
wget --post-data "" http://localhost:9090/api/v1/admin/tsdb/snapshot
```

* Tar up the created snapshot

```sh
cd ~/.sourcegraph/data/prometheus/snapshots && tar -czvf /tmp/sourcegraph-metrics-dump.tgz <snapshot-name>
```

* If needed, you can download the metrics dump to your local machine (current directory) using `scp`:

```sh
scp -r username@hostname:/tmp/sourcegraph-metrics-dump.tgz .
```

Please then upload the `sourcegraph-metrics-dump.tgz` for Sourcegraph support to access it. If desired, we can send you a shared private Google Drive folder for the upload as it can sometimes be a few gigabytes.

### Kubernetes deployments

If you're using the [Kubernetes cluster deployment option](https://github.com/sourcegraph/deploy-sourcegraph),  
you can create a metrics dump as follows:

* Open a shell to the running container
    1. Run `kubectl get pods` to get the name of the Prometheus pod.
    1. Run `kubectl exec -it <pod-name> -- /bin/bash`.

* Inside the container bash shell trigger the creation of a Prometheus snapshot:  

```sh
wget --post-data "" http://localhost:9090/api/v1/admin/tsdb/snapshot
```

* Tar up the created snapshot

```sh
cd /prometheus/snapshots && tar -czvf /tmp/sourcegraph-metrics-dump.tgz <snapshot-name>
```

* Switch back to local shell and copy the metrics dump file over:

```sh
kubectl cp <podname>:/tmp/sourcegraph-metrics-dump.tgz /tmp/sourcegraph-metrics-dump.tgz
```

Again please then upload the `sourcegraph-metrics-dump.tgz` for Sourcegraph support to access it.

### Generating pprof profiles

Please follow [these instructions](pprof.md) to generate pprof profiles.

### Sourcegraph is not returning results from a repository unless "repo:" is included

If you can get repository results when you explicitly include `repo:{your repository}` in your search, but don't see any results from that repository when you don't, there are a few possible causes: 

- The repository is a fork repository (excluded from search results by default) and `fork:yes` is not specified in the search query.
- The repository is an archived repository (excluded from search results by default) and `archived:yes` is not specified in the search query.
- There is an issue indexing the repository: check the logs of repo-updater and/or search-indexer.
- The search index is unavailable for some reason: try the search query `repo:<the_repository> index:only`. If it returns no results, the repository has not been indexed.

### Sourcegraph is making unauthorized requests to the git server

This is normal and happens whenever git is used over HTTP. To avoid unnecessarily sending a password over HTTP, git first
makes a request without the password included. If a 401 Unauthorized is returned, git sends the request with the password.

More information can be found [here](https://confluence.atlassian.com/bitbucketserverkb/two-401-responses-for-every-git-opperation-938854756.html).

If this behaviour is undesired, the `gitURLType` in the [external service configuration](https://docs.sourcegraph.com/admin/external_service/github#configuration)
should be set to `ssh` instead of `http`. This will also require [ssh keys to be set up](https://docs.sourcegraph.com/admin/repo/auth#repositories-that-need-http-s-or-ssh-authentication).
