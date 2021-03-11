# Sourcegraph architecture overview

This document provides a high level overview of Sourcegraph's architecture so you can understand how our systems fit together.

## Code syncing

At its core, Sourcegraph maintains a persistent cache of all the code that is connected to it. It is persistent, because this data is critical for Sourcegraph to function, but it is ultimately a cache because the code host is the source of truth and our cache is eventually consistent.

- [gitserver](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/gitserver/README.md) is the sharded service that stores the code and makes it accessible to other Sourcegraph services.
- [repo-updater](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/repo-updater/README.md) is the singleton service that is responsible for ensuring all the code in gitserver is as up-to-date as possible while respecting code host rate limits. It is also responsible for syncing code repository metadata from the code host that is stored in the `repo` table of our Postgres database.

If you want to learn more about how code is synchronized, read [Life of a repository](life-of-a-repository.md).

## Search

Devs can search across all the code that is connected to their Sourcegraph instance.

By default, Sourcegraph uses [zoekt](https://github.com/sourcegraph/zoekt) to create a trigram index of the default branch of every repository so that searches are fast. This trigram index is the reason why Sourcegraph search is more powerful and faster than what is usually provided by code hosts.

- [zoekt-indexserver](https://sourcegraph.com/github.com/sourcegraph/zoekt/-/tree/cmd/zoekt-sourcegraph-indexserver)
- [zoekt-webserver](https://sourcegraph.com/github.com/sourcegraph/zoekt/-/tree/cmd/zoekt-webserver)

Sourcegraph also has a fast search path for code that isn't indexed yet, or for code that will never be indexed (for example: code that is not on a default branch). Indexing every branch of every repository isn't a pragmatic use of resources for most customers, so this decision balances optimizing the common case (searching all default branches) with space savings (not indexing everything).

- [searcher](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/searcher/README.md) implements the non-indexed search.

Syntax highlighting for any code view, including search results, is provided by [Syntect server](https://sourcegraph.com/github.com/sourcegraph/syntect_server).

If you want to learn more about search:

- [Code search product documentation](../../../code_search/index.md)
- [Life of a search query](life-of-a-search-query.md)
- [Search pagination](search-pagination.md)

## Code intelligence

Code intelligence surfaces data (for example: doc comments for a symbol) and actions (for example: go to definition, find references) based on our semantic understanding of code (unlike search, which is completely text based).

By default, Sourcegraph provides imprecise [search-based code intelligence](../../../code_intelligence/explanations/search_based_code_intelligence.md). This reuses all the architecture that makes search fast, but it can result in false positives (for example: finding two definitions for a symbol, or references that aren't actually references), or false negatives (for example: not able to find the definition or all references). This is the default because it works with no extra configuration and is pretty good for many use cases and languages. We support a lot of languages this way because it only requires writing a few regular expressions.

With some setup, customers can enable [precise code intelligence](../../../code_intelligence/explanations/precise_code_intelligence.md). Repositories add a step to their build pipeline that computes the index for that revision of code and uploads it to Sourcegraph. We have to write language specific indexers, so adding precise code intel support for new languages is a non-trivial task.

If you want to learn more about code intelligence:

- [Code intelligence product documentation](../../../code_intelligence/index.md)
- [Code intelligence developer documentation](../codeintel/index.md)
- [Available indexers](../../../code_intelligence/references/indexers.md)

## Batch changes

Batch changes (formerly known as [campaigns](../../../batch_changes/references/name-change.md)) create and manage large scale code changes across projects, repositories, and code hosts.

To create a batch change, users write a [batch spec](../../../batch_changes/references/batch_spec_yaml_reference.md), which is a YAML file that specifies the changes that should be performed, and the repositories that they should be performed upon — either through a Sourcegraph search, or by declaring them directly. This spec is then executed by [src-cli](#src-cli) on the user's machine (or in CI, or some other environment controlled by the user), which results in [changeset specs](../../../batch_changes/explanations/introduction_to_batch_changes.md#changeset-spec) that are sent to Sourcegraph. These changeset specs are then applied by Sourcegraph to create one or more changesets per repository. (A changeset is a pull request or merge request, depending on the code host.)

Once created, changesets are monitored by Sourcegraph, and their current review and CI status can be viewed on the batch change's page, providing a single pane of glass view of all the changesets created as part of the batch change. The batch change can be updated at any time by re-applying the original batch spec: this will transparently add or remove changesets in repositories that now match or don't match the original search as needed.

If you want to learn more about batch changes:

- [Batch changes product documentation](../../../batch_changes/index.md)
- [Batch changes design principles](../../../batch_changes/explanations/batch_changes_design.md)
- [Batch changes developer documentation](../batch_changes/index.md)
- [How `src` executes a batch spec](../../../batch_changes/explanations/how_src_executes_a_batch_spec.md)

## Code insights

Code insights surface higher-level, aggregated information to leaders in engineering organizations in dashboards.
For example, code insights can track the number of matches of a search query over time, the number of code intelligence diagnostic warnings in a code base, usage of different programming languages, or even data from external services, like test coverage from Codecov.
Sample use cases for this are for tracking migrations, usage of libraries across an organization, tech debt, code base health, and much more.

Code insights are currently feature-flagged - set `"experimentalFeatures": { "codeInsights": true }` in your user settings to enable them.

Code insights currently work through [**extensions**](#sourcegraph-extension-api).
A code insight extension can register a _view provider_ that contributes a graph to either the repository/directory page, the [search homepage](https://sourcegraph.com/search), or the [global "Insights" dashboard](https://sourcegraph.com/insights) reachable from the navbar.
It is called on-demand on the client (the browser) to return the data needed for the chart.
_How_ that extension produces the data is up to the extension - it can run search queries, query code intelligence data or analyze Git data using the Sourcegraph GraphQL API, or it can query an external service using its public API, e.g. Codecov.

To enable a code insight, install one of the [code insights extensions](https://sourcegraph.com/extensions?query=category%3A%22Insights%22).
The extension can then be configured in your [user settings](https://sourcegraph.com/user/settings) according to the examples in the extension README.
Just like other extensions, it's also possible to install and configure them organization-wide.

Because of code insights currently being run on-demand in the client, the performance of code insights is bound to the performance of the underlying data source.
For example, search queries are relatively fast as long as the scope doesn't include many repositories, but performance degrades when trying to include a lot of repositories.
We're actively working on removing this limitation. 

If you want to learn more about code insights:

- [Code insights team page](https://about.sourcegraph.com/handbook/engineering/code-insights)
- [Code insights product document (PD)](https://docs.google.com/document/d/1d34gCpt_rUOMAun8phcjNsFofGaaA_N_8znmgaugdKw/edit)
- [Original code insights RFC](https://docs.google.com/document/d/1EHzor6I1GhVVIpl70mH-c10b1tNEl_p1xRMJ9qHQfoc/edit)

## Code monitoring

Code monitoring allows users to get notified of changes to their codebase.

Users can view, edit and create code monitors through the code monitoring UI (`/code-monitoring`). A code monitor comprises a **trigger**, and one or more **actions**.

The **trigger** watches for new data and if there is new data we call this an event. For now, the only supported trigger is a search query of `type:diff` or `type:commit`, run every five minutes by the Go backend with an automatically added `after:` parameter narrowing down the diffs/commits that should be searched. The monitor's configured actions are run when this query returns a non-zero number of results.

The **actions** are run in response to a trigger event. For now, the only supported action is an email notification to the primary email address of the code monitor's owner. In order for this to work, [`email.address` and `email.smtp` must be configured in site configuration](https://docs.sourcegraph.com/admin/observability/alerting#email). Code monitoring actions will be extended in the future to support webhooks.

If you want to learn more about code monitoring:

- [Code monitoring documentation](https://docs.sourcegraph.com/code_monitoring)

## Browser extensions

The Sourcegraph browser extensions bring the features of Sourcegraph directly into the UI of code hosts such as GitHub, GitLab and Bitbucket.

With the Sourcegraph browser extension installed, users get Sourcegraph features (including code intelligence and Sourcegraph extensions) on their code host while browsing code, viewing diffs, or reviewing pull requests.

This lets users get value from Sourcegraph without leaving their existing workflows on their code host, while also giving them a convenient way to jump into Sourcegraph at any time (by using the `Open in Sourcegraph` button on any repository or file). The browser extension also adds an address bar search shortcut, allowing you to search on Sourcegraph directly from the browser address bar.

If you want to learn more about browser extensions:

- [Sourcegraph browser extensions documentation](../../../integration/browser_extension.md)
- [Overview of code host integrations](../web/code_host_integrations.md)

## Native integrations (for code hosts)

Native integrations bring Sourcegraph features directly into the UI of code hosts, in a similar way to the browser extension.

Instead of requiring a browser extension, native integrations inject a script by extending the code host directly (for example, using the code host's plugin architecture). The advantage is that Sourcegraph can be enabled for all users of a code host instance, without any action required from each user.

If you want to learn more about native integrations:

- [Overview of code host integrations](../web/code_host_integrations.md)

## Sourcegraph extension API

The Sourcegraph extension API allows developers to write extensions that extend the functionality of Sourcegraph.

Extensions that use the API can add elements and interactions to the Sourcegraph UI, such as:

- adding action buttons in the toolbar
- decorating specific lines of code in a file
- contributing hover tooltip information on specific tokens in a file
- decorating files in directory listings

Some core features of Sourcegraph, like displaying code intelligence hover tooltips, are implemented using the extension API.

If you want to learn more about our extension API:

- [Sourcegraph extension architecture](sourcegraph-extensions.md)

## src-cli

src-cli, or `src`, is a command line tool that users can run locally to interact with Sourcegraph.

src-cli is written in Go, and distributed as a standalone binary for Windows, macOS, and Linux. Its features include [running searches](../../../cli/references/search.md), managing Sourcegraph, and [executing batch changes](../../../batch_changes/quickstart.md#create-the-batch-change). src-cli is an integral part of the [batch changes product](#batch-changes).

Note that src-cli is not contained within the Sourcegraph monorepo, and has its own release cadence.

If you want to learn more about src-cli:

- [src-cli repository](https://github.com/sourcegraph/src-cli)
- [src-cli documentation](../../../cli/index.md)

## Editor extensions

Sourcegraph editor extensions will bring Sourcegraph features like search, code intelligence, and Sourcegraph extensions into your IDE. (Switching between Sourcegraph and an IDE when viewing a file is separately powered by Sourcegraph extensions.) 

The editor extension is still in the exploratory phase of determining priority and scope. For more information: 

- [PD19: IDE Extension (Research & Exploration)](https://docs.google.com/document/d/1LpShKInGJo0BBDnRQW4yz4_CjhjK_FYcwse4LYXGImE/edit#)

## Deployment

Sourcegraph is deployable via three supported methods:

- [Kubernetes](../../../admin/install/kubernetes/index.md) is intended for all medium to large scale production deployments that require fault tolerance and high availibility. For advanced users only with significant kubernetes experience required. This deployment method is developed in [`deploy-sourcegraph`](https://github.com/sourcegraph/deploy-sourcegraph-docker).
- [Docker-Compose](../../../admin/install/docker-compose/index.md) is intended to be used for small to medium production deployments, with some customization available. Easy to setup with basic infrastructure and docker knowledge required. A variation on this is the [pure-Docker option](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/README.md). Both of these deployment methods are developed in [`deploy-sourcegraph-docker`](https://github.com/sourcegraph/deploy-sourcegraph-docker).
- [Server](../../../admin/install/docker/index.md) for small environments on a single server. Easiest and quickest to setup with a single command. Little infrastructure knowledge is required. This deployment method is developed in [`cmd/server`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/server).

The [resource estimator](https://docs.sourcegraph.com/admin/install/resource_estimator) can guide you on the requirements for each deployment type.

## Observability

Observability encapsulates the monitoring and debugging of Sourcegraph deployments.
Sourcegraph is designed, and ships with, a number of observability tools and capabilities out-of-the box to enable visibility into the health and state of a Sourcegraph deployment.

Monitoring includes [metrics and dashboards](../../../admin/observability/metrics.md), [alerting](../../../admin/observability/alerting.md), and [health checking](../../../admin/observability/health_checks.md) capabilities.
Learn more about monitoring in the [monitoring architecture overview](https://about.sourcegraph.com/handbook/engineering/observability/monitoring_architecture).

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
- [Observability at Sourcegraph](https://about.sourcegraph.com/handbook/engineering/observability)

## Diagram

You can click on each component to jump to its respective code repository or subtree.

<object data="/dev/background-information/architecture/architecture.svg" type="image/svg+xml" style="width:100%; height: 100%">
</object>

Note that almost every service has a link back to the frontend, from which it gathers configuration updates.
These edges are omitted for clarity.

## Other resources

- [Life of a ping](life-of-a-ping.md)
