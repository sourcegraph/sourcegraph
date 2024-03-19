# Search configuration

See "[Code search overview](../code_search/index.md)" for general information about Sourcegraph's code search.

## Indexed search

Sourcegraph indexes the code on the default branch of each repository. This speeds up searches that hit many repositories at once. Not all files in a repository branch are indexed, we skip files that are [larger than 1 MB](../code_search/explanations/search_details.md) and binary files. To view which files are skipped during indexing, visit the repository settings page and click on indexing.

For large deployments we recommend horizontally scaling indexed search. You can do this by [adjusting the number of replicas](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/configure.md#configure-indexed-search-replica-count). Sourcegraph shards repository indexes across replicas. When the replica count changes Sourcegraph will slowly rebalance indexes to ensure availability of existing indexes.

The resource requirements for indexed search vary considerably based on the text contents of your repositories, but a good estimate is that the node should have enough memory to hold the entire text contents of the default branch of each repository.

### Scaling considerations

Zoekt is Sourcegraph's indexing engine.

When processing a repository, Zoekt splits the index it creates across
one or more files on disk. These files are called "shards."

Zoekt uses [memory maps](https://en.wikipedia.org/wiki/Memory-mapped_file) to load all shards into memory to evaluate search queries. In most deployments, the total size of the search index (all the shards on disk) is much larger than the total amount of RAM available to Zoekt. Amongst other benefits, using [memory maps](https://en.wikipedia.org/wiki/Memory-mapped_file) allows Zoekt to:

- Leverage [demand paging](https://en.wikipedia.org/wiki/Demand_paging): only read the shard file from the disk if Zoekt tries to read that portion of the file (and it isn't already in RAM)
- Leverage the kernel's [page cache](https://en.wikipedia.org/wiki/Page_cache): keep the most frequently accessed pages in RAM and evict them when the system is under memory pressure

#### Consideration: RAM versus Disk

As stated above, Zoekt uses RAM as a cache for accessing shards while
evaluating a search query.

- The more RAM you give Zoekt, the more shards it can hold in RAM *before* it has to access the disk.

- The less RAM you give Zoekt, the more often it will have to access the disk to read the data it needs from its shards. If Zoekt has to access the disk more often, this negatively impacts search performance, increases disk utilization, etc.

Tuning Zoekt's resource requirements is a balance between:

- The amount of RAM you are willing to allocate
- The amount of disk i/o resources that you have available in your environment
- The impact on search performance that you find acceptable

#### Consideration: Available memory maps

As stated above, Zoekt uses [memory maps](https://en.wikipedia.org/wiki/Memory-mapped_file) to load all of its shards into memory.

There is a [limit to the number of memory maps a process](https://www.kernel.org/doc/Documentation/sysctl/vm.txt) can create on Linux. On most systems, the default limit is 65536 maps. Processes, including Zoekt, will be terminated if they attempt to allocate more memory maps than that limit.

**This limit provides a ceiling for the number of shards a Zoekt instance can store.** When capacity planning, you can estimate the
amount of Zoekt instances (including some slack for growth) you'll need via the following rule of thumb:

```text
(number of repositories) / (60% * (memory map limit))
```

So, assuming that you have 300,000 repositories and a memory map limit of 65,536, this results in:

```text
(300,000) / (.6 * 65,536) = 7.62 => 8 instances
```

Sourcegraph's monitoring system also includes an [alert for this
scenario and mitigation steps](https://docs.sourcegraph.com/admin/observability/alerts#zoekt-memory-map-areas-percentage-used).
