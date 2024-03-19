# Sourcegraph architecture overview

This document provides a high level overview of Sourcegraph's architecture so you can understand how our systems fit together.

The **"Dependencies"** sections give a short, high-level overview of dependencies on other architecture components and the most important aspects of _how_ they are used.

## Diagram

You can click on each component to jump to its respective code repository or subtree. <a href="./architecture.svg" target="_blank">Open in new tab</a>

<!--
Auto-generated from ./doc/dev/background-information/architecture/architecture.dot
Run cd ./doc/dev/background-information/architecture && ./generate.sh to update the .svg
-->
<object data="./architecture/architecture.svg" type="image/svg+xml" width="1023" height="1113" style="width:100%; height: auto">
</object>

Note that almost every service has a link back to the frontend, from which it gathers configuration updates.
These edges are omitted for clarity.

## Repository syncing

At its core, Sourcegraph maintains a persistent cache of all repositories that are connected to it. It is persistent, because this data is critical for Sourcegraph to function, but it is ultimately a cache because the code host is the source of truth and our cache is eventually consistent.

- [gitserver](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/gitserver/README.md) is the sharded service that stores repositories and makes them accessible to other Sourcegraph services.
- [repo-updater](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/repo-updater/README.md) is the singleton service that is responsible for ensuring all repositories in gitserver are as up-to-date as possible while respecting code host rate limits. It is also responsible for syncing repository metadata from the code host that is stored in the `repo` table of our Postgres database.

If you want to learn more about how repositories are synchronized, read [Life of a repository](life-of-a-repository.md).

## Permission syncing

Repository permissions are by default being mirrored from code hosts to Sourcegraph, it builds the foundation of Sourcegraph authorization for repositories to ensure users see consistent content as on code hosts. Currently, the background permissions syncer resides in the [repo-updater](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/repo-updater/README.md).

If you want to learn more about how repository permissions are synchronized in the background, read about [permission syncing](../../../admin/permissions/syncing.md).

## Settings cascade

<small>Last updated: 2021-08-12</small>

Sourcegraph offers the flexibility of customizing settings by users. The settings of a single user is generally the result of merging user settings, organization settings and global settings. Each of these are referred to as a _settings subject_, which are part of the _settings cascade_. They are all exposed over GraphQL.

## Search

Devs can search across all the code that is connected to their Sourcegraph instance.

By default, Sourcegraph uses [zoekt](https://github.com/sourcegraph/zoekt) to create a trigram index of the default branch of every repository so that searches are fast. This trigram index is the reason why Sourcegraph search is more powerful and faster than what is usually provided by code hosts.

- [zoekt-indexserver](https://sourcegraph.com/github.com/sourcegraph/zoekt/-/tree/cmd/zoekt-sourcegraph-indexserver)
- [zoekt-webserver](https://sourcegraph.com/github.com/sourcegraph/zoekt/-/tree/cmd/zoekt-webserver)

Sourcegraph also has a fast search path for code that isn't indexed yet, or for code that will never be indexed (for example: code that is not on a default branch). Indexing every branch of every repository isn't a pragmatic use of resources for most customers, so this decision balances optimizing the common case (searching all default branches) with space savings (not indexing everything).

- [searcher](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/searcher/README.md) implements the non-indexed search.

Syntax highlighting for any code view, including search results, is provided by [Syntect server](https://github.com/sourcegraph/sourcegraph/tree/main/docker-images/syntax-highlighter).

If you want to learn more about search:

- [Code search product documentation](../../../code_search/index.md)
- [Life of a search query](life-of-a-search-query.md)
- [Indexed ranking](indexed-ranking.md)

## Code navigation

Code navigation surfaces data (for example: doc comments for a symbol) and actions (for example: go to definition, find references) based on our semantic understanding of code (unlike search, which is completely text based).

By default, Sourcegraph provides imprecise [search-based code navigation](../../../code_navigation/explanations/search_based_code_navigation.md). This reuses all the architecture that makes search fast, but it can result in false positives (for example: finding two definitions for a symbol, or references that aren't actually references), or false negatives (for example: not able to find the definition or all references). This is the default because it works with no extra configuration and is pretty good for many use cases and languages. We support a lot of languages this way because it only requires writing a few regular expressions.

With some setup, customers can enable [precise code navigation](../../../code_navigation/explanations/precise_code_navigation.md). Repositories add a step to their build pipeline that computes the index for that revision of code and uploads it to Sourcegraph. We have to write language specific indexers, so adding precise code navigation support for new languages is a non-trivial task.

If you want to learn more about code navigation:

- [Code navigation product documentation](../../../code_navigation/index.md)
- [Code navigation developer documentation](../codeintel/index.md)
- [Available indexers](../../../code_navigation/references/indexers.md)

### Dependencies

<small>Last updated: 2021-07-05</small>

- [Search](#search)
  - Symbol search is used for basic code navigation
- [Sourcegraph extension API](#sourcegraph-extension-api)
  - Hover and definition providers
- [Native integrations (for code hosts)](#native-integrations-for-code-hosts)
  - UI of hover tooltips on code hosts

## Batch Changes

Batch Changes (formerly known as [campaigns](../../../batch_changes/references/name-change.md)) creates and manages large scale code changes across projects, repositories, and code hosts.

To create a batch change, users write a [batch spec](../../../batch_changes/references/batch_spec_yaml_reference.md), which is a YAML file that specifies the changes that should be performed, and the repositories that they should be performed upon — either through a Sourcegraph search, or by declaring them directly. This spec is then executed by [src-cli](#src-cli) on the user's machine (or in CI, or some other environment controlled by the user), which results in [changeset specs](../../../batch_changes/explanations/introduction_to_batch_changes.md#changeset-spec) that are sent to Sourcegraph. These changeset specs are then applied by Sourcegraph to create one or more changesets per repository. (A changeset is a pull request or merge request, depending on the code host.)

Once created, changesets are monitored by Sourcegraph, and their current review and CI status can be viewed on the batch change's page, providing a single pane of glass view of all the changesets created as part of the batch change. The batch change can be updated at any time by re-applying the original batch spec: this will transparently add or remove changesets in repositories that now match or don't match the original search as needed.

If you want to learn more about batch changes:

- [Batch Changes product documentation](../../../batch_changes/index.md)
- [Batch Changes design principles](../../../batch_changes/explanations/batch_changes_design.md)
- [Batch Changes developer documentation](../batch_changes/index.md)
- [How `src` executes a batch spec](../../../batch_changes/explanations/how_src_executes_a_batch_spec.md)

### Dependencies

<small>Last updated: 2021-07-05</small>

- [src-cli](#src-cli)
  - Batch changes are currently executed client-side through the `src` CLI
- [Search](#search)
  - Repositories in which batch specs need to be executed are resolved through the search API

## Code insights

Code insights surface higher-level, aggregated information to leaders in engineering organizations in dashboards.
For example, code insights can track the number of matches of a search query over time, the number of code navigation diagnostic warnings in a code base or usage of different programming languages.
Sample use cases for this are for tracking migrations, usage of libraries across an organization, tech debt, code base health, and much more.

Code Insights are persisted in a separate databased called `codeinsights-db`. The web application interacts with the backend through a [GraphQL API](../../../api/graphql/managing-code-insights-with-api.md).

Code Insights makes use of data from the `frontend` database for repository metadata, as well as repository permissions to filter time series data.

Code Insights can either generate data in the background, or just-in-time when viewing charts. This decision is currently enforced in the product, depending on the type and scope of the insight.
For code insights being run just-in-time in the client, the performance of code insights is bound to the performance of the underlying data source.
These insights are relatively fast as long as the scope doesn't include many repositories (or large monorepos), but performance degrades when trying to include a lot of repositories. Insights
that are processed in the background are rate limited and will perform approximately 28,000 queries per hour when fully saturated on default settings.

There is also a feature flag left over from the original development of the early stage product that we retained in case a customer who doesn't purchase it ever has a justified need to disable insights. You can set `"experimentalFeatures": { "codeInsights": false }` in your settings to disable insights.

If you want to learn more about code insights:

- [Code insights team page](https://handbook.sourcegraph.com/engineering/code-graph/code-insights#code-insights-team)
- [Code insights product document (PD)](https://docs.google.com/document/d/1d34gCpt_rUOMAun8phcjNsFofGaaA_N_8znmgaugdKw/edit)
- [Original code insights RFC](https://docs.google.com/document/d/1EHzor6I1GhVVIpl70mH-c10b1tNEl_p1xRMJ9qHQfoc/edit)

### Dependencies

<small>Last updated: 2021-08-12</small>

- [Search](#search)
  - GraphQL API for text search, in particular `search()`, `matchCount`, `stats.languages`
  - Query syntax: Code insights "construct" search queries programmatically
  - Exhaustive search (with `count:all`/`count:999999` operator)
  - Historical search (= unindexed search, currently)
  - Commit search to find historical commits to search over
- [Repository Syncing](#repository-syncing)
  - The code insights backend has direct dependencies on `gitserver` and `repo-updater`
- [Permission syncing](#permission-syncing)
  - The code insights backend depends on synced repository permissions for access control.
- [Settings cascade](#settings-cascade)
  - Insights and dashboard configuration is currently stored in user, organization and global settings. This will change in the future and is planned to be moved to the database.
  - Insights contributed by extensions are configured through settings (this will stay the same).
- Future: [Batch Changes](#batch-changes)
  - "Create a batch change from a code insight" flow
- Future: [Code monitoring](#code-monitoring)
  - "Create a code monitor from a code insight" flow

## Code monitoring

Code monitoring allows users to get notified of changes to their codebase.

Users can view, edit and create code monitors through the code monitoring UI (`/code-monitoring`). A code monitor comprises a **trigger**, and one or more **actions**.

The **trigger** watches for new data and if there is new data we call this an event. For now, the only supported trigger is a search query of `type:diff` or `type:commit`, run every five minutes by the Go backend with an automatically added `after:` parameter narrowing down the diffs/commits that should be searched. The monitor's configured actions are run when this query returns a non-zero number of results.

The **actions** are run in response to a trigger event. For now, the only supported action is an email notification to the primary email address of the code monitor's owner. In order for this to work, [`email.address` and `email.smtp` must be configured in site configuration](https://docs.sourcegraph.com/admin/observability/alerting#email). Code monitoring actions will be extended in the future to support webhooks.

If you want to learn more about code monitoring:

- [Code monitoring documentation](https://docs.sourcegraph.com/code_monitoring)

### Dependencies

<small>Last updated: 2021-07-05</small>

- [Search](#search)
  - Diff and commit search triggers

## Browser extensions

The Sourcegraph browser extensions bring the features of Sourcegraph directly into the UI of code hosts such as GitHub, GitLab and Bitbucket.

With the Sourcegraph browser extension installed, users get Sourcegraph features (including code navigation) on their code host while browsing code, viewing diffs, or reviewing pull requests.

This lets users get value from Sourcegraph without leaving their existing workflows on their code host, while also giving them a convenient way to jump into Sourcegraph at any time (by using the `Open in Sourcegraph` button on any repository or file). The browser extension also adds an address bar search shortcut, allowing you to search on Sourcegraph directly from the browser address bar.

If you want to learn more about browser extensions:

- [Sourcegraph browser extensions documentation](../../../integration/browser_extension.md)
- [Overview of code host integrations](../web/code_host_integrations.md)

## Native integrations (for code hosts)

Native integrations bring Sourcegraph features directly into the UI of code hosts, in a similar way to the browser extension.

Instead of requiring a browser extension, native integrations inject a script by extending the code host directly (for example, using the code host's plugin architecture). The advantage is that Sourcegraph can be enabled for all users of a code host instance, without any action required from each user.

If you want to learn more about native integrations:

- [Overview of code host integrations](../web/code_host_integrations.md)

### Dependencies

<small>Last updated: 2021-07-05</small>

- [Repository Syncing](#repository-syncing)
  - Uses the GraphQL API to resolve repositories and revisions on code hosts

### Dependencies

<small>Last updated: 2021-08-12</small>

- [Search](#search)
  - Query transformer API hooks into search in the web app
- [Settings cascade](#settings-cascade)
  - Which extensions are enabled and configuration for extensions are stored in settings. Extensions may also change settings.

## src-cli

src-cli, or `src`, is a command line tool that users can run locally to interact with Sourcegraph.

src-cli is written in Go, and distributed as a standalone binary for Windows, macOS, and Linux. Its features include [running searches](../../../cli/references/search.md), managing Sourcegraph, and [executing batch changes](../../../batch_changes/quickstart.md#create-the-batch-change). src-cli is an integral part of the [batch changes product](#batch-changes).

Note that src-cli is not contained within the Sourcegraph monorepo, and has its own release cadence.

If you want to learn more about src-cli:

- [src-cli repository](https://github.com/sourcegraph/src-cli)
- [src-cli documentation](../../../cli/index.md)

### Dependencies

<small>Last updated: 2021-07-05</small>

- [Search](#search)
  - GraphQL API
- [Batch Changes](#batch-changes)
  - GraphQL API

## Editor extensions

Sourcegraph editor extensions will bring Sourcegraph features like search, and code navigation into your IDE. (Switching between Sourcegraph and an IDE when viewing a file is separately powered by Sourcegraph extensions.)

The editor extension is still in the exploratory phase of determining priority and scope. For more information:

- [PD19: IDE Extension (Research & Exploration)](https://docs.google.com/document/d/1LpShKInGJo0BBDnRQW4yz4_CjhjK_FYcwse4LYXGImE/edit#)

## Deployment

Sourcegraph is deployable via three supported methods:

- [Kubernetes](../../../admin/deploy/kubernetes/index.md) is intended for all medium to large scale production deployments that require fault tolerance and high availibility. For advanced users only with significant kubernetes experience required. This deployment method is developed in [`deploy-sourcegraph`](https://github.com/sourcegraph/deploy-sourcegraph).
- [Docker Compose](../../../admin/deploy/docker-compose/index.md) is intended to be used for small to medium production deployments, with some customization available. Easy to setup with basic infrastructure and docker knowledge required. A variation on this is the [pure-Docker option](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/README.md). Both of these deployment methods are developed in [`deploy-sourcegraph-docker`](https://github.com/sourcegraph/deploy-sourcegraph-docker).
- [Docker Single Container](../../../admin/deploy/docker-single-container/index.md) for small environments on a single server. Easiest and quickest to setup with a single command. Little infrastructure knowledge is required. This deployment method is developed in [`cmd/server`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/server).

The [resource estimator](https://docs.sourcegraph.com/admin/deploy/resource_estimator can guide you on the requirements for each deployment type.

## Observability

Observability encapsulates the monitoring and debugging of Sourcegraph deployments.
Sourcegraph is designed, and ships with, a number of observability tools and capabilities out-of-the box to enable visibility into the health and state of a Sourcegraph deployment.

Monitoring includes [metrics and dashboards](../../../admin/observability/metrics.md), [alerting](../../../admin/observability/alerting.md), and [health checking](../../../admin/observability/health_checks.md) capabilities.
Learn more about monitoring in the [monitoring architecture overview](https://handbook.sourcegraph.com/engineering/observability/monitoring_architecture).

- [grafana](../observability/grafana.md) is the frontend for service metrics, and ships with customized dashboards for Sourcegraph services.
- [prometheus](../observability/prometheus.md) handles scraping of service metrics, and ships with recording rules, alert rules, and alerting capabilities.
- [cadvisor](../observability/cadvisor.md) provides per-container performance metrics (scraped by Prometheus) in most Sourcegraph environments.
- [Health checks are provided by each Sourcegraph service](../../../admin/observability/health_checks.md).

Debugging includes [tracing](../../../admin/observability/tracing.md) and [logging](../../../admin/observability/logs.md).

- [jaeger](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/jaeger-all-in-one) is the distributed tracing service used by Sourcegraph.
- [jaeger-agent](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/docker-images/jaeger-agent) is a sidecar used in Kubernetes deployments for collecting traces from services.
- [Logs are provided by each Sourcegraph service](../../../admin/observability/logs.md).

If you want to learn more about observability:

- [Observability for site administrators](../../../admin/observability/index.md)
- [Observability developer documentation](../observability/index.md)
- [Observability at Sourcegraph](https://handbook.sourcegraph.com/engineering/observability)

## Other resources

- [Life of a ping](life-of-a-ping.md)
