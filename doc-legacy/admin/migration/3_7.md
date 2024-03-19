# Migrating to Sourcegraph 3.7.2+

Sourcegraph 3.7.2+ includes much faster indexed symbol search ([on large searches, up to 20x faster](https://docs.google.com/spreadsheets/d/1oPzePjD8YLrnppLm3nk46h48_Cxipz4_QqRMBYaIOYQ/edit?usp=sharing)). However, there are some aspects you should be aware of when upgrading an existing Sourcegraph instance:

Upgrading and downgrading is safe, reindexing will occur in the background seamlessly with no downtime or harm to search performance.

**Please read this document in full before upgrading to 3.7.**

## Increased disk space requirements

With indexed symbol search comes an **increase in the required disk space. Please ensure you have enough free space before upgrading.**

Run the command below for your deployment to determine how much disk space the indexed search indexes are taking currently. Then, multiply the number you get times 1.3 to determine how much free space you need before upgrading.

For example, in the below examples we see 126 GiB is currently in use. Multiplying 126 GiB * 1.3 gives us 163.8 GiB (the amount we should ensure is free before upgrading).

<details>
<summary><strong>Single-container Docker deployment</strong></summary>
Run the following on the host machine:

```bash
$ du -sh ~/.sourcegraph/data/zoekt/index/
126G	/Users/jane/.sourcegraph/data
```
</details>

<details>
<summary><strong>Kubernetes cluster deployment</strong></summary>
Run the following, but replace the value of `$POD_NAME` with your `indexed-search` pod name from `kubectl get pods`:

```
$ POD_NAME='indexed-search-974c74498-6jngm' kubectl --namespace=prod exec -it $POD_NAME -c zoekt-indexserver -- du -sh /data/index
126G	/data/index
```
</details>

<details>
<summary><strong>Pure-Docker cluster deployment</strong></summary>
Run the following against the `zoekt-shared-disk` directory on the host machine:

```bash
$ du -sh ~/sourcegraph-docker/zoekt-shared-disk/
126G	/home/ec2-user/sourcegraph-docker/zoekt-shared-disk/
```
</details>

## Background indexing

Sourcegraph will reindex all repositories in the background seamlessly. In the meantime, it will serve searches just as fast from the old search index.

This process happens at a rate of about 1,400 repositories/hr, depending on repository size and available resources.

**Until this process has completed, search performance will be the same as the prior version.**

If you're eager or want to confirm, here's how to check the process has finished:

<details>
<summary><strong>Single-container Docker deployment</strong></summary>
The following command ran on the host machine shows how many repositories have been reindexed:

```bash
$ ls ~/.sourcegraph/data/zoekt/index/*_v16* | wc -l
      12583
```

When it is equal to the number of repositories on your instance, the process has finished!
</details>

<details>
<summary><strong>Kubernetes cluster deployment</strong></summary>
The following command will show how many repositories have been reindexed. Replace the value of `$POD_NAME` with your `indexed-search` pod name from `kubectl get pods`:

```bash
$ kubectl --namespace=prod exec -it indexed-search-974c74498-6jngm -c zoekt-indexserver -- sh -c 'ls /data/index/*_v16* | wc -l'
12583
```
  
When it is equal to the number of repositories on your instance, the process has finished!
</details>

<details>
<summary><strong>Pure-Docker cluster deployment</strong></summary>
The following command ran on the host machine against the `zoekt-shared-disk` directory will show how many repositories have been reindexed.

```bash
$ ls ~/sourcegraph-docker/zoekt-shared-disk/*_v16* | wc -l
12583
```
  
When it is equal to the number of repositories on your instance, the process has finished!
</details>

## Downgrading

As guaranteed by our compatibility promise, it is always safe to downgrade to a previous minor version. e.g. 3.7.2 to 3.6.x. There will be no downtime, and search speed will not be impacted.

Please do *not* downgrade or upgrade to 3.7.0 or 3.7.1, though, as those versions will incur a reindex and search performance will be harmed in the meantime.

## Memory and CPU requirements (no substantial change compared to v3.5)

- In v3.7, the `indexed-search` / `zoekt-indexserver` container will use 28% more memory on average compared to v3.6. However, please take note that v3.6 reduced memory consumption of the same container by about 41% -- so the net change from v3.5 -> v3.7 is still less memory usage overall.
- CPU usage may increase depending on the amount of symbol queries your users run now that it is much faster. We suggest not changing any CPU resources and instead checking resource usage after the upgrade and reindexing has finished.
