# How to address common monorepo performance problems

This document is intended as an explanation of common issues faced by Sourcegraph instances syncing monorepos. Sourcegraph search relies on the retrieval and indexing of git repos. For very large repos, more computational resources are required to perform the necessary operations. As such tuning Sourcegraph's services is often the resolution to bugs and performance issues covered in this how-to.

_This document is targeted at docker-compose and kubernetes deployments, where services can be isolated for individual tuning_

The following bullets provide a general guidline to which service may require more resources:

* `sourcegraph-frontend` CPU/memory resource allocations
* `searcher` CPU/memory resource allocations (allocate enough memory to hold all non-binary files in your repositories)
* `indexedSearch` CPU/memory resource allocations (for the `zoekt-indexserver` pod, allocate enough memory to hold all non-binary files in your largest repository; for the `zoekt-webserver` pod, allocate enough memory to hold ~2.7x the size of all non-binary files in your repositories)
* `symbols` CPU/memory resource allocations
* `gitserver` CPU/memory resource allocations (allocate enough memory to hold your Git packed bare repositories)

## Symbols sidebar - Processing symbols

![Screen Shot 2021-11-15 at 12 35 07 AM](https://user-images.githubusercontent.com/13024338/141749036-95759cbe-abd5-4d78-91eb-618423d2f66c.png)

If you are regularly seeing the `Processing symbols is taking longer than expected. Try again in a while` warning in your sidebar, its likely that your symbols and/or gitserver services are underprovisioned and need more CPU/mem resources.

The [symbols sidebar](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/repo/RepoRevisionSidebarSymbols.tsx?L42) is dependent on the symbols and gitserver services. Upon opening the symbols sidebar, a search query is made to the GraphQL API to retrieve the symbols associated with the current git commit. You can read more about the [symbol search behavior and performance](../../code_navigation/explanations/features.md#symbol-search-behavior-and-performance).

To address this concern, allocate more resources to the symbols service (to provide more processing power for indexing operations) and allocate more resources to the gitserver (to provide for the extra load associated with responding to fetch requests from symbols, and speed up sending the large repo).

Here's an example of a diff to improve symbols performance in a k8s deployment:

```diff
          name: debug
        resources:
          limits:
-           cpu: "3"
-           memory: 6G
          requests:
-           cpu: 500m
-           memory: 5G

          name: debug
        resources:
          limits:
+           cpu: "4"
+           memory: 16G
          requests:
+           cpu: "1"
+           memory: 8G
```

_Learn more about managing resources in [docker-compose](../deploy/docker-compose/index.md#operations) and [kubernetes](../deploy/kubernetes/operations.md)_

## Slow hover tooltip results

Hovering over a symbol results in a query for the definition. If the symbol is defined in a repo that has precise code navigation, then Sourcegraph should respond with results quickly. Otherwise, the definition query will have the same performance characteristics as above in [symbols sidebar](#symbols-sidebar---processing-symbols) because it uses a `type:symbol` search.

## Slow history tab and git blame results

![Screen Shot 2021-11-15 at 1 10 16 AM](https://user-images.githubusercontent.com/13024338/141754063-2080c7c6-b5be-43c1-b9db-386e916d2968.png)

Selecting the Show History button while viewing a file initiates a request to [fetch commits](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Eclient/web/src/repo/RepoRevisionSidebarCommits%5C.tsx+function+fetchCommits%28&patternType=literal) for the file. This request is ultimately resolved by gitserver using functionality similar to git log. To improve performance allocate gitserver more CPU.

## Common alerts

The following alerts are common to instances underprovisioned in relation to their monorepos, [learn more about alerts](https://docs.sourcegraph.com/admin/observability/alerts):

- frontend: 20s+ 99th percentile code-intel successful search request duration over 5m
- frontend: 15s+ 90th percentile code-intel successful search request duration over 5m
- zoekt-webserver: 5% Indexed search request errors every 5m by code for 5m0s
- symbols: 25+ current fetch queue size
