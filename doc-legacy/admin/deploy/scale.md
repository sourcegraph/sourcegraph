
<style>
  table {
    vertical-align: text-top;
    border: none!important;
    margin-top: 2rem;
    margin-bottom: 2rem;
}
  td, th, tr {
    vertical-align: text-top;
    border: none!important;
    background-color: transparent!important;
  }
  .my-2, hr {
    margin-top: 2rem;
    margin-bottom: 2rem;
  }
  h3 {
    margin-top: 2rem;
  }
</style>

# Scaling Overview for Services

This page provides a comprehensive overview of how each Sourcegraph service scales.
In order to support the growth of your instance usage, it is recommended to scale Sourcegraph based on the scaling factors suggested below.

Grafana should be the first stop you make if you plan to expand on one of the scaling factors or when you are experiencing a system performance issue. 

Please use the following scaling guideline for services that are using more than 80% of their assigned resources. 

Scaling is unnecessary if your resource usage is kept below 80%.

For example, if you plan to add 100% more engaged users, and the resource usage for all services is currently at around 70%, we’d recommend using this documentation as a reference to adjust the resources that list “Number of active users” as one of their scaling factors. You can also use the output from the Resource Estimator as references alternatively.

> NOTE: For assistance when scaling and tuning Sourcegraph, [contact us](https://sourcegraph.com/contact/). We're happy to help!

---

## Components Overview

Here is a list of components you can find in a typical Sourcegraph deployment:

### Core Components

|                                                     |                                                                                                                                                                                          |
| :-------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [`frontend`](scale.md#frontend)                     | Serves the web application, extensions, and graphQL services. Almost every service has a link back to the frontend, from which it gathers configuration updates.                         |
| [`gitserver`](scale.md#gitserver)                   | Mirrors repositories from their code host. All other Sourcegraph services talk to gitserver when they need data from git.                                                                |
| [`precise-code-intel`](scale.md#precise-code-intel) | Converts LSIF upload file into Postgres data. The entire index must be read into memory to be correlated.                                                                                |
| [`repo-updater`](scale.md#repo-updater)             | Tracks the state of repositories. It is responsible for automatically scheduling updates using gitserver and for synchronizing metadata between code hosts and external services.        |
| [`searcher`](scale.md#searcher)                     | Provides on-demand unindexed search for repositories. It fetches archives from gitserver and searches them with regexp.                                                                  |
| [`symbols`](scale.md#symbols)                       | Indexes symbols in repositories using Ctags.                                                                                                                                             |
| [`syntect-server`](scale.md#syntect-server)         | An HTTP server that exposes the Rust Syntect syntax highlighting library for use by other services.                                                                                      |
| [`worker`](scale.md#worker)                         | Runs a collection of background jobs periodically in response to internal requests and external events. It is currently janitorial and commit based.                                     |
| [`zoekt-indexserver`](scale.md#zoekt-indexserver)   | Indexes all enabled repositories on Sourcegraph and keeps the indexes up to date. Lives inside the indexed-search pod in a Kubernetes deployment.                                        |
| [`zoekt-webserver`](scale.md#zoekt-webserver)       | Runs searches from indexes stored in memory and disk. The indexes are persisted to disk to avoid re-indexing on startup. Lives inside the indexed-search pod in a Kubernetes deployment. |

### External Services

A list of services that can be externalized. See our docs on [Using external services with Sourcegraph](../external_service/index.md) for detailed instruction.

|                                               |                                                                                                                                                                         |
| :-------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [`codeinsights-db`](scale.md#codeinsights-db) | A PostgreSQL instance for storing code insights data.                                                                                                                   |
| [`codeintel-db`](scale.md#codeintel-db)       | A PostgreSQL instance for storing large-volume code graph data.                                                                                                         |
| [`jaeger`](scale.md#jeager)                   | A Jaeger instance for end-to-end distributed tracing.                                                                                                                   |
| [`blobstore`](scale.md#blobstore)             | A blobstore instance that serves as a local S3-compatible object storage to hold user uploads for code-intel before they can be processed.                              |
| [`pgsql`](scale.md#pgsql)                     | A PostgreSQL instance for storing long-term information, such as user information when using Sourcegraph’s built-in authentication provider instead of an external one. |
| [`redis-cache`](scale.md#redis-cache)         | A Redis instance for storing cache data.                                                                                                                                |
| [`redis-store`](scale.md#redis-store)         | A Redis instance for storing short-term information such as user sessions.                                                                                              |

### Monitoring Tools

Sourcegraph provides a number of tools to monitor the health and usage of your deployment. See our [Observability docs](../observability/index.md) for more information.
You can also learn more about the architecture of Sourcegraph’s monitoring stack in [Sourcegraph monitoring architecture](https://handbook.sourcegraph.com/departments/engineering/dev/tools/observability/monitoring_architecture)

|                                     |                                                                                                                                      |
| :---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| [`cadvisor`](scale.md#cadvisor)     | A custom cAdvisor instance that exports container monitoring metrics scraped by Prometheus and visualized in Grafana.                |
| [`grafana`](scale.md#grafana)       | A Grafana instance that displays data from Prometheus and Jaeger. It is shipped with customized dashboards for Sourcegraph services. |
| [`prometheus`](scale.md#prometheus) | A customized Prometheus instance for collecting high-level and low-cardinality, metrics across services.                             |

---

## Scaling Guideline

> This section provides you with a high-level overview of how each Sourcegraph service works with resources, with a list of scaling factors and basic guideline. 

### cAdvisor

```
A cAdvisor instance.
It exports container monitoring metrics scraped by Prometheus and visualized in Grafana.
```

| Resources   |                                                                                                                                                                             |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Scaling is not necessary as it is designed to be a small footprint service                                                                                                  |
| `Factors`   | Its primary traffic are the requests coming from Prometheus                                                                                                                 |
| `Guideline` | Read [the list of known issues](https://docs.sourcegraph.com/dev/background-information/observability/cadvisor#known-issues) that can cause performance issues for cAdvisor |

---

### codeinsights-db

```
A PostgreSQL instance for storing code insights data.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |


| CPU           |                                                     |
| :------------ | :-------------------------------------------------- |
| `Overview`    | Executes queries                                    |
| ``Factors``   | Number of active users                              |
|               | Number of repositories                              |
|               | Number of insight series defined                    |
|               | Number of matches per insight series                |
|               | Compression ratio of insight data                   |
| ``Guideline`` | Keep the total memory larger than the largest index |

| Memory      |                                                     |
| :---------- | :-------------------------------------------------- |
| `Overview`  | Process Indexes                                     |
| `Factors`   | Number of active users                              |
|             | Size of all repositories                            |
|             | Number of repositories                              |
|             | Number of insight series defined                    |
|             | Number of matches per insight series                |
|             | Compression ratio of insight data                   |
| `Guideline` | Keep the total memory larger than the largest index |

| Storage     |                                                                            |
| :---------- | :------------------------------------------------------------------------- |
| `Overview`  | Stores code insights data                                                  |
| `Factors`   | Number of insight series defined                                           |
|             | Number of matches per insight series                                       |
|             | Compression ratio of insight data                                          |
| `Guideline` | Depends entirely on usage and the specific Insights that are being created |
| `Type`      | Persistent Volumes for Kubernetes                                          |
|             | Persistent SSD for Docker Compose                                          |

> WARNING: The database must be configured properly to consume resources effectively and efficiently for better performance and to avoid regular usage from overwhelming built-in utilities (like autovacuum for example). For example, a Postgres database running out of memory indicates that it is currently misconfigured and the amount of memory each worker can utilize should be reduced. See our [Postgres database configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf) for more information.

---

### codeintel-db

```
A PostgreSQL instance for storing large-volume code graph data.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                                                                   |
| :---------- | :------------------------------------------------------------------------------------------------ |
| `Overview`  | Executes queries                                                                                  |
| `Factors`   | Number of active users                                                                            |
|             | Frequency with which the instance runs precise code navigation queries                            |
| `Guideline` | The default value should work for all deployments. Please refer to the note below for more detail |

| Memory      |                                                                                                                           |
| :---------- | :------------------------------------------------------------------------------------------------------------------------ |
| `Overview`  | Process LSIF indexes                                                                                                      |
| `Factors`   | Number of active users                                                                                                    |
|             | Frequency with which the instance runs precise code navigation queries                                                    |
|             | Total size of repositories indexed by Rockskip                                                                            |
| `Guideline` | The database must be configured properly to consume resources effectively and efficiently. See note below for more detail |
|             | The amount of memory each Postgres worker can utilize must be adjusted according to the memory assigned to the database   |
|             | Increase the memory assigned to each worker proportionally when database queries are slow                                 |

| Storage     |                                                                              |
| :---------- | :--------------------------------------------------------------------------- |
| `Overview`  | Stores processed upload data                                                 |
| `Factors`   | Number and size of precise code graph data uploads                           |
|             | Indexer used                                                                 |
| `Guideline` | The index size and processed size are currently based on indexer used        |
|             | Requires about 4 times of the total size of repositories indexed by Rockskip |
|             | SCIP provides a more stable approximation of index size -> processed         |
| `Type`      | Persistent Volumes for Kubernetes                                            |
|             | Persistent SSD for Docker Compose                                            |

> WARNING: The database must be configured properly to consume resources effectively and efficiently for better performance and to avoid regular usage from overwhelming built-in utilities (like autovacuum for example). For example, a Postgres database running out of memory indicates that it is currently misconfigured and the amount of memory each worker can utilize should be reduced. See our [Postgres database configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf) for more information.

---

### frontend

```
Serves the Sourcegraph web application, extensions, and graphQL API services. 
```

| Replica     |                                                                                                   |
| :---------- | :------------------------------------------------------------------------------------------------ |
| `Overview`  | Almost every service has a link back to the frontend, from which it gathers configuration updates |
| `Factors`   | Number of active users                                                                            |
|             | Number of services connected                                                                      |
| `Guideline` | More engaged users = more replicas                                                                |

| CPU         |                                                                                                     |
| :---------- | :-------------------------------------------------------------------------------------------------- |
| `Overview`  | At least one goroutine is dispatched per HTTP request. It is also used to serve Code Insight series |
| `Factors`   | Number of active users                                                                              |
|             | Number of user actions performed                                                                    |
|             | Number of insight series defined                                                                    |
| `Guideline` | More engaged users = more replicas                                                                  |

| Memory      |                                                   |
| :---------- | :------------------------------------------------ |
| `Overview`  | Aggregates results before serving them to clients |
| `Factors`   | Number of active users                            |
|             | Number of repositories                            |
| `Guideline` | More engaged users = more Memory                  |

| Storage     |      |
| :---------- | :--- |
| `Overview`  | -    |
| `Factors`   | -    |
| `Guideline` | -    |
| `Type`      | None |

---

### gitserver

```
Mirrors repositories from their code host. 
Other Sourcegraph services communicate with gitserver when they need data from git.
```

| Replica     |                                                                        |
| :---------- | :--------------------------------------------------------------------- |
| `Overview`  | Handles requests from other Sourcegraph services for git information   |
| `Factors`   | Size of all repositories                                               |
| `Guideline` | When the total size of repositories is too large to fit in one replica |

| CPU         |                                                                                      |
| :---------- | :----------------------------------------------------------------------------------- |
| `Overview`  | Runs git commands concurrently                                                       |
| `Factors`   | Number of active users                                                               |
|             | Size of all repositories                                                             |
|             | Size of the largest repository                                                       |
| `Guideline` | Depends on the amount of git commands need to perform --more git commands = more CPU |

| Memory      |                                                                                                |
| :---------- | :--------------------------------------------------------------------------------------------- |
| `Overview`  | Data associate with the running git commands                                                   |
| `Factors`   | Size of all repositories                                                                       |
|             | Size of the largest repository                                                                 |
| `Guideline` | Depends on the git commands to be executed --the more or larger the git commands = more memory |

| Storage     |                                                                                                                      |
| :---------- | :------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Repositories are cloned to disk                                                                                      |
| `Factors`   | Size of all repositories                                                                                             |
| `Guideline` | Greater than 20% free space accounting for the size of all repositories on disk, including soft-deleted repositories |
|             | It can also be customized via a variable named SRC_REPOS_DESIRED_PERCENT_FREE                                        |
|             | Update disk size of indexserver per adjustments made for gitserver disk size                                         |
| `Type`      | Persistent Volumes for Kubernetes                                                                                    |
|             | Persistent SSD for Docker Compose                                                                                    |

---

### grafana

```
A Grafana instance that displays data from Prometheus and Jaeger. 
It is shipped with customized dashboards for Sourcegraph services.
```

| Resources   |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | Not designed to be a high-traffic service                   |
| `Factors`   | Number of Site Admins                                       |
| `Guideline` | The default setup should be sufficient for most deployments |

---

### jeager

```
A Jaeger instance for end-to-end distributed tracing
```

| Resources   |                                                                                                        |
| :---------- | :----------------------------------------------------------------------------------------------------- |
| `Overview`  | A debugging tool that is not designed to be a high-traffic service                                     |
| `Factors`   | Number of Site Admins                                                                                  |
| `Guideline` | Memory depends on the size of buffers, like the number of traces and the size of the queue for example |

> NOTE: The jaeger service does not have to be enabled for Sourcegraph work, however, the ability to troubleshoot the system will be disabled.

---

### blobstore

```
A blobstore instance that serves as local S3-compatible object storage. It
holds files such as search jobs results and index uploads for precise code
navigation.

```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments |
| `Factors`   | -                                                           |
| `Guideline` | -                                                           |

| Memory      |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments |
| `Factors`   | -                                                           |
| `Guideline` | -                                                           |

| Storage     |                                                   |
| :---------- | :------------------------------------------------ |
| `Overview`  | A temporary storage location for the LSIF uploads |
| `Factors`   | Size of the largest LSIF index                    |
| `Guideline` | Equal to the size of the largest LSIF index file  |
| `Type`      | Persistent Volumes for Kubernetes                 |
|             | Persistent SSD for Docker Compose                 |

---

### pgsql

```
The main database. A PostgreSQL instance.
Data stored include repo lists, user data, worker queue , and site-config files etc.
Data for code-insights related to running queries are also stored here.
Basically anything not related to code-intel.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                                                                                                                                                         |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Executes queries                                                                                                                                                                        |
| `Factors`   | The default setup should be sufficient for most deployments                                                                                                                             |
| `Guideline` | The database must be configured properly following our [Postgres configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf)to use the assigned resources efficiently |

| Memory      |                                                                                                                                                                                         |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Linear to the concurrent number of API requests proxied                                                                                                                                 |
| `Factors`   | The default setup should be sufficient for most deployments                                                                                                                             |
| `Guideline` | The database must be configured properly following our [Postgres configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf)to use the assigned resources efficiently |

| Storage     |                                                                                                                                                                                         |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | The Postgres instance will use memory by bringing OS pages into resident memory where it will control its own allocations                                                               |
| `Factors`   | Size of all repositories                                                                                                                                                                |
|             | Number of insight queries                                                                                                                                                               |
| `Guideline` | Starts at default as the value grows depending on the number of active users and activity                                                                                               |
|             | The database must be configured properly following our [Postgres configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf)to use the assigned resources efficiently |
| `Type`      | Persistent Volumes for Kubernetes                                                                                                                                                       |
|             | Persistent SSD for Docker Compose                                                                                                                                                       |

> WARNING: The database must be configured properly to consume resources effectively and efficiently for better performance and to avoid regular usage from overwhelming built-in utilities (like autovacuum for example). For example, a Postgres database running out of memory indicates that it is currently misconfigured and the amount of memory each worker can utilize should be reduced. See our [Postgres database configuration guide](https://docs.sourcegraph.com/admin/config/postgres-conf) for more information.

---

###  precise-code-intel

```
Handles conversion of uploaded code graph data bundles.
It converts LSIF upload file into Postgres data.
```

| Replica     |                                                                                               |
| :---------- | :-------------------------------------------------------------------------------------------- |
| `Overview`  | Process uploads queue                                                                         |
| `Factors`   | Number of jobs in the upload queue                                                            |
| `Guideline` | When there is a large queue backlog to increase the throughput at which uploads are processed |

| CPU         |                                                                                                                                   |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | This service is I/O bound: reading from blobstore/GCS/S3 and writing to pgsql/codeintel-db. Correlation has been fairly optimized |
| `Factors`   | Number of jobs in the upload queue                                                                                                |
| `Guideline` | Upload jobs may finish faster if the CPU is increased, but having it at a reasonable minimum should be the ideal target here      |

| MEM         |                                                                                                                     |
| :---------- | :------------------------------------------------------------------------------------------------------------------ |
| `Overview`  | The entire LSIF index file must be read into memory to be correlated, and causes uploads to fail when out of memory |
| `Factors`   | Size of the largest LSIF index                                                                                      |
| `Guideline` | The entire index must be read into memory to be correlated                                                          |
|             | Add memory when the uploaded index is too large to be processed without OOMing                                      |
|             | Requires two times of the size of the largest LSIF index times upload_concurrency in memory                         |

| Storage     |      |
| :---------- | :--- |
| `Overview`  | -    |
| `Factors`   | -    |
| `Guideline` | -    |
| `Type`      | None |

---

### prometheus

```
A customized Prometheus instance for collecting high-level and low-cardinality, metrics across services.
It currently bundles Alertmanager as well as integrations to the Sourcegraph web application.
```

| Resources   |                                                                                                                                                                                                        |
| :---------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments                                                                                                                                            |
| `Factors`   | Number of active users                                                                                                                                                                                 |
| `Guideline` | For Kubernetes deployments, please follow the [instruction here](kubernetes/configure.md#filtering-cadvisor-metrics) to prevent Prometheus from scraping metrics outside of your Sourcegraph namespace |

---

### redis-cache

```
A Redis instance for storing cache data for frontend.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments |
| `Factors`   | -                                                           |
| `Guideline` | -                                                           |

| Memory      |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments |
| `Factors`   | -                                                           |
| `Guideline` | -                                                           |

| Storage     |                                                 |
| :---------- | :---------------------------------------------- |
| `Overview`  | A temporary storage location for cache data     |
| `Factors`   | Size of all repositories                        |
| `Guideline` | Adjust based on the size of cloned repositories |
|             | Depends on the size of the API response body    |
| `Type`      | Ephemeral storage for Kubernetes                |
|             | Persistent SSD for Docker Compose               |

---

### redis-store

```
A Redis instance for storing short-term information such as user sessions.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                             |
| :---------- | :---------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments |
| `Factors`   | -                                                           |
| `Guideline` | -                                                           |

| Memory      |                                                                                                                  |
| :---------- | :--------------------------------------------------------------------------------------------------------------- |
| `Overview`  | The default setup should be sufficient for most deployments                                                      |
| `Factors`   | Number of active users                                                                                           |
| `Guideline` | Increase memory based on the number of active user sessions (including both anonymous users and signed-in users) |
|             | Each anonymous session is counted independently                                                                  |

| Storage     |                                                                                                           |
| :---------- | :-------------------------------------------------------------------------------------------------------- |
| `Overview`  | Stores user sessions                                                                                      |
| `Factors`   | Size of all repositories                                                                                  |
| `Guideline` | TIncrease based on the number of active user sessions (including both anonymous users and signed-in users |
|             | each anonymous session is counted independently                                                           |
| `Type`      | Ephemeral storage for Kubernetes                                                                          |
|             | Persistent SSD for Docker Compose                                                                         |

---

### repo-updater

```
Repo-updater tracks the state of repositories.
It is responsible for automatically scheduling updates using gitserver. 
It is also responsible for synchronizing metadata between code hosts and external services.
Services that desire updates or fetch must communicate with repo-updater instead of gitserver.
```

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                                                          |
| :---------- | :--------------------------------------------------------------------------------------- |
| `Overview`  | Most operations are not CPU bound                                                        |
| `Factors`   | Most of the syncing jobs are related more to internal and code host-specific rate limits |
| `Guideline` | -                                                                                        |

| Memory      |                                                                                                                                                                                                                                   |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | The queue of repositories that need to be updated is stored in memory. It uses an in-memory queue and is mostly network intensive as it makes API calls and processes and writes those newly available data to the pgsql database |
| `Factors`   | Number of repositories                                                                                                                                                                                                            |
| `Guideline` | This service is safe to restart at any time. The existing in-memory update queue is reset upon restart                                                                                                                            |
|             | Not memory intensive                                                                                                                                                                                                              |

| Storage     |                                                                |
| :---------- | :------------------------------------------------------------- |
| `Overview`  | A stateless service that directly writes to the pgsql database |
| `Factors`   | -                                                              |
| `Guideline` | -                                                              |
| `Type`      | None                                                           |

---

### searcher

```
Provides on-demand unindexed search for repositories. 
It relies on the OS file page cache to speed up future searches
```

| Replica     |                                                                     |
| :---------- | :------------------------------------------------------------------ |
| `Overview`  | Depending on the number of concurrent requests for unindexed search |
| `Factors`   | Number of active users                                              |
| `Guideline` | More engaged users = more replicas                                  |


| CPU         |                                                                                                |
| :---------- | :--------------------------------------------------------------------------------------------- |
| `Overview`  | Searcher is IO and CPU bound. It fetches archives from gitserver and searches them with regexp |
| `Factors`   | Number of active users                                                                         |
| `Guideline` | More engaged users = more CPU                                                                  |

| Memory      |                                                                       |
| :---------- | :-------------------------------------------------------------------- |
| `Overview`  | Searcher fetches archives from gitserver and stores them on disk      |
| `Factors`   | Number of active users                                                |
| `Guideline` | Not a memory-intensive service as the search results are streamed out |
|             | Memory usage is based on the number of concurrent search requests     |
|             | More memory will be useful around page cache                          |

| Storage     |                                                                                |
| :---------- | :----------------------------------------------------------------------------- |
| `Overview`  | Searcher primarily uses disk space to cache archives for unindexed search      |
| `Factors`   | Size of the largest repository                                                 |
| `Guideline` | Requires enough disk space to store the largest repository                     |
|             | The most important thing is to ensure fast IO for storage                      |
|             | Add more disks or replicas if you have lots of unindexed searches              |
|             | More disk space will help speed up future caches                               |
| `Type`      | Ephemeral storage for Kubernetes deployments                                   |
|             | The request size of the ephemeral storage is used as a limit for the zip cache |
|             | Non-persistent SSD for Docker Compose                                          |

> NOTE: For example, if you search all branches on all repositories, that translates into lots of concurrent unindexed requests. 

---

### symbols

```
The backend for symbols operations.
Indexes symbols in repositories using Ctags.
```

| Replica     |                                                                             |
| :---------- | :-------------------------------------------------------------------------- |
| `Overview`  | Process unindexed search                                                    |
| `Factors`   | Number of active users                                                      |
| `Guideline` | More requests for distinct commits in different repositories = more replica |


| CPU         |                                                                                        |
| :---------- | :------------------------------------------------------------------------------------- |
| `Overview`  | Runs Ctags over code, stores symbol data in SQLite (or codeintel-db if using Rockskip) |
| `Factors`   | Size of all repositories                                                               |
| `Guideline` | Scale with the size of repositories                                                    |


| Memory      |                                                                     |
| :---------- | :------------------------------------------------------------------ |
| `Overview`  | Stores symbol data in SQLite and/or Postgres if Rockskip is enabled |
| `Factors`   | Size of all repositories                                            |
| `Guideline` | Scale with the size of repositories                                 |

| Storage     |                                                                                                                      |
| :---------- | :------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Saves SQLite DBs as files on disk in LRU fashion and copies an old one to a new file when a user visits a new commit |
| `Factors`   | Size of the largest repository                                                                                       |
| `Guideline` | At least 20% more than the size of your largest repository                                                           |
|             | Using SSD is highly preferred                                                                                        |
| `Type`      | Ephemeral storage for Kubernetes deployments                                                                         |
|             | Non-persistent SSD for Docker Compose                                                                                |

> NOTE: If Rockskip is enabled, the symbols for repositories indexed by Rockskip are stored in codeintel-db instead.

---

### syntect-server

```
An HTTP server that exposes the Rust Syntect syntax highlighting library to other services.
```

| Replica     |                                                                                                                                                                                      |
| :---------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | The constant overhead applies per worker process. Having a large number of processes is not necessarily useful (consumes more memory while idling) if there aren't many active users |
| `Factors`   | Number of active users                                                                                                                                                               |
| `Guideline` | More users = more CPU and replicas                                                                                                                                                   |
|             | Add replica when syntax highlighting queries take a long duration because of existing traffic                                                                                        |

| CPU         |                                                                                                                                                                                                                                                                  |
| :---------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | There are a number of worker processes, each with some number of threads. When a syntax highlighting request comes in, it is dispatched to a process, which further sends it to the worker thread. When there are no requests, CPU consumption should be minimal |
| `Factors`   | Number of active users                                                                                                                                                                                                                                           |
| `Guideline` | More users = more CPU and replicas                                                                                                                                                                                                                               |

| Memory      |                                                                                                                                                     |
| :---------- | :-------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Overview`  | A lot of the highlighting themes and compiled grammars are loaded into memory at start up                                                           |
| `Factors`   | Number of active users                                                                                                                              |
| `Guideline` | There is additional memory consumption on receiving requests (< 25 MB), although, that's generally much smaller than the constant overhead (1-2 GB) |

| Storage     |      |
| :---------- | :--- |
| `Overview`  | -    |
| `Factors`   | -    |
| `Guideline` | -    |
| `Type`      | None |

---

### worker

```
Runs a collection of background jobs periodically in response to internal requests and external events.
```

> NOTE: See the docs on [Worker services](https://docs.sourcegraph.com/admin/workers#worker-jobs) for a list of worker jobs.

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Singleton                                               |
| `Factors`   | -                                                       |
| `Guideline` | A Singleton service should not have more than 1 replica |

| CPU         |                                                |
| :---------- | :--------------------------------------------- |
| `Overview`  | One goroutine per worker                       |
| `Factors`   | Number of repositories                         |
|             | Size of all repositories                       |
|             | Size of the largest repository                 |
|             | Number of insight series defined               |
| `Guideline` | Rate limited / concurrency limited worker pool |

| Memory      |                                                                |
| :---------- | :------------------------------------------------------------- |
| `Overview`  | Abulating search results to generate Code Insights time series |
| `Factors`   | Number of repositories                                         |
|             | Number of insight series defined                               |
| `Guideline` | Grow with the number of repositories and code-insight users    |

| Storage     |      |
| :---------- | :--- |
| `Overview`  | -    |
| `Factors`   | -    |
| `Guideline` | -    |
| `Type`      | None |

---

### zoekt-indexserver

```
Indexes all enabled repositories on Sourcegraph.
It also keeps the indexes up to date.
Lives inside the indexed-search pod in a Kubernetes deployment.
The main guideline for scaling a zoekt-indexserver is the size of your largest repository.
```

> NOTE: As indexserver currently only indexes one repository at a time, having more CPU and memory are not as important here than in webserver.

| Replica     |                                                       |
| :---------- | :---------------------------------------------------- |
| `Overview`  | Parallel with zoekt-webserver                         |
| `Factors`   | Size of the largest repository                        |
| `Guideline` | Scales with zoekt-webserver                           |
|             | Replicas number must be parallel with zoekt-webserver |

| CPU         |                                                                                              |
| :---------- | :------------------------------------------------------------------------------------------- |
| `Overview`  | Indexes one repository at a time                                                             |
| `Factors`   | Size of the largest repository                                                               |
| `Guideline` | More CPU results in lower lag between a new commit and the time it takes to index for search |

| Memory      |                                                                                                                   |
| :---------- | :---------------------------------------------------------------------------------------------------------------- |
| `Overview`  | Scans for symbols                                                                                                 |
| `Factors`   | Size of the largest repository                                                                                    |
| `Guideline` | In general the amount of RAM used will max out at 100MB * number of CPUS * constant factor (around 5 in practice) |

| Storage     |                                                                       |
| :---------- | :-------------------------------------------------------------------- |
| `Overview`  | Stores index. Storage is shared with zoekt-webserver                  |
| `Factors`   | Size of all repositories                                              |
| `Guideline` | 50% of the gitserver disk size                                        |
|             | Disk IO is important as it constantly reads from disk during searches |
|             | Scale with zoekt-webserver                                            |
| `Type`      | Persistent Volumes for Kubernetes                                     |
|             | Persistent SSD for Docker Compose                                     |

> WARNING: We recommend providing zoekt-indexserver with more resources when trying to add a lot of new repositories that requires indexing from a new external service, and then scale down once indexing is completed.

---

### zoekt-webserver

```
Runs searches from indexes stored in memory and disk.
It serves and processes data from zoekt-indexserver.
The indexes are persisted to disk to avoid re-indexing on startup.
Lives inside the indexed-search pod in a Kubernetes deployment.
```

> NOTE: Adding more CPU and memory helps speed up searches.

| Replica     |                                                         |
| :---------- | :------------------------------------------------------ |
| `Overview`  | Parallel with zoekt-indexserver                         |
| `Factors`   | Number of repositories                                  |
|             | Size of all repositories                                |
| `Guideline` | More repositories = more CPU and replicas               |
|             | Replicas number must be parallel with zoekt-indexserver |

| CPU         |                                              |
| :---------- | :------------------------------------------- |
| `Overview`  | Goroutines are dispatched per search query   |
| `Factors`   | Number of repositories                       |
|             | Number of active users                       |
| `Guideline` | Scales with the number of search requests    |
|             | More search requests = more CPU and replicas |

| Memory      |                                         |
| :---------- | :-------------------------------------- |
| `Overview`  | Parts of the index are stored in memory |
| `Factors`   | Number of repositories                  |
|             | Size of all repositories                |
| `Guideline` | Scales with the number of repositories  |
|             | More repositories = more memory         |

| Storage     |                                                                       |
| :---------- | :-------------------------------------------------------------------- |
| `Overview`  | Stores index. Storage is shared with zoekt-indexserver.               |
| `Factors`   | Size of all repositories                                              |
| `Guideline` | 50% of the gitserver disk size                                        |
|             | Disk IO is important as it constantly reads from disk during searches |
|             | Scale with zoekt-indexserver                                          |
| `Type`      | Persistent Volumes for Kubernetes                                     |
|             | Persistent SSD for Docker Compose                                     |

> WARNING: Check the peak bursts rather than the average over time when monitoring CPU usage for zoekt as it depends on when the searches happen.

---
