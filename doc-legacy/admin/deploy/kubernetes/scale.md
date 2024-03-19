# Scaling Sourcegraph on Kubernetes

Sourcegraph can scale to accommodate large codebases and many users. 

Increase resources according to the [Scaling Overview per Service](../scale.md) if you notice slower search or navigation.

## Cluster resource guidelines

For production environments, we recommend allocate resources based on your [instance size](../instance-size.md). See our [resource estimator](../resource_estimator.md) for estimates.

---

## Improving performance with a large number of repositories

Here is a simplified list of the key parameters to tune when scaling Sourcegraph to many repositories:

- `sourcegraph-frontend` CPU/memory resource allocations
- `searcher` replica count
- `indexedSearch` replica count and CPU/memory resource allocations
- `gitserver` replica count
- `symbols` replica count and CPU/memory resource allocations
- `gitMaxConcurrentClones`, because `git clone` and `git fetch` operations are IO and CPU-intensive
- `repoListUpdateInterval` (in minutes), because each interval triggers `git fetch` operations for all repositories

Notes:

- If your change requires restarting `gitserver` pods and they are rescheduled to other nodes, they may go offline briefly (showing a `Multi-Attach` error). This is due to volume detach/reattach. [Contact us](https://sourcegraph.com/contact/) for mitigation steps depending on your cloud provider.
- See the docs to understand each service's role:
  - [Sourcegraph Architecture Overview](../../../dev/background-information/architecture/index.md)
  - [Scaling Overview per Service](../scale.md)

---

## Improving performance with large monorepos

Here is a simplified list of key parameters to tune when scaling Sourcegraph to large monorepos:

- `sourcegraph-frontend` CPU/memory resource allocations
- `searcher` CPU/memory resource allocations (allocate enough memory to hold all non-binary files in your repositories)
- `indexedSearch` CPU/memory resource allocations (for the `zoekt-indexserver` pod, allocate enough memory to hold all non-binary files in your largest repository; for the `zoekt-webserver` pod, allocate enough memory to hold ~2.7x the size of all non-binary files in your repositories)
- `symbols` CPU/memory resource allocations
- `gitserver` CPU/memory resource allocations (allocate enough memory to hold your Git packed bare repositories)

---

## Configuring faster disk I/O for caches

Many parts of Sourcegraph's infrastructure benefit from using SSDs for caches. This is especially
important for search performance. By default, disk caches will use the
Kubernetes [hostPath](https://kubernetes.io/docs/concepts/storage/volumes/#hostpath) and will be the
same IO speed as the underlying node's disk. Even if the node's default disk is a SSD, however, it
is likely network-mounted rather than local.
