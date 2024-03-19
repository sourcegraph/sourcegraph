# Life of a repository

This document describes how our backend systems clone and update repositories from a code host.

## High level

1. An admin configures a [code host configuration](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40v3.36.3+file:%5Eschema/%28aws%7Cbit%7Cgit%7Cother%29.*schema%5C.json%24&patternType=literal).
2. `repo-updater` periodically [syncs](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/syncer.go?L67) all repository metadata from configured code hosts.
1. We [poll](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/syncer.go?L447) the code host's API based on the configuration.
2. We [add/update/remove](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/syncer.go?L586) entries in our [`repo` table](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/database/schema.md#table-public-repo).
3. All repositories in our `repo` table are in a [scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/scheduler.go#L107-110) on `repo-updater` which ensures they are cloned and updated on [`gitserver`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/cmd/gitserver/server/server.go#L787).

Our guiding principle is to ensure all repositories configured by a site administrator are cloned and up to date. However, we need to avoid overloading a code host with API and Git requests.

>NOTE: Sourcegraph.com is different since it isn't feasible to maintain a clone of all open source repositories. It works via on-demand requests from users.

>NOTE: There is one other way repositories are fetched. A new commit may not be on Sourcegraph yet, but a user is browsing it via our browser extension. `gitserver` supports a ["EnsureRevision"](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/cmd/gitserver/server/server.go#L1278-1283) use-case which will do a "git fetch" for the missing revision.

## Services

### Repo Updater

`repo-updater` is a singleton service. It is responsible for:

* Communicating with code host APIs to coordinate the state we synchronize from them.
* Maintaining the `repo` table which other services read.
* Scheduling clones/fetches on `gitserver`.
* Anything which communicates with a code host API.

Our batch changes and background permissions syncers are also located in `repo-updater` as they require communication with code host APIs.

>NOTE: The name `repo-updater` does not accurately capture what the service does. This is a historical artifact. We have not updated it due to the unnecessary operational burden it would put on our customers.

### Gitserver

`gitserver` is a scaleable stateful service which clones and updates git repositories and can run git commands against them.

All data maintained on this service is from cloning an upstream repository. We shard the set of repositories across the gitserver replicas, but do not support replication. All communication with `gitserver` from other services should be done via the gitserver [client interface](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.42.2/-/blob/internal/gitserver/client.go?L167).

It is responsible for the state of the [gitserver_repos](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.42.2/-/blob/internal/database/schema.md#table-public-gitserver-repos) table. The main process which handles this is the background job that runs on each `gitserver` instance, see [SyncRepoState](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.42.2/-/blob/cmd/gitserver/server/server.go?L445).

## Discovery

Before we can clone a repository, we first must discover that it exists. This is configured by a site administrator setting code host configuration. Typically a code host will have an API as well as git endpoints. A code host configuration typically will specify how to communicate with the API and which repositories to ask the API for. For example:

``` json
{
  "url": "https://github.com",
  "token": "deadbeef",
  "repositoryQuery": ["affiliated"],
}
```

This is a GitHub code host configuration for `github.com` using the private access token `deadbeef`. It will ask GitHub for all affiliated repositories. Follow [`GithubSource.listRepositoryQuery`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/github.go#L806) to find the actual API call we do.

Discovering the repositories for each codehost/configuration is abstracted in the [`Source interface`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/sources.go#L76:1).

``` go
// A Source yields repositories to be stored and analysed by Sourcegraph.
// Successive calls to its ListRepos method may yield different results.
type Source interface {
	// ListRepos sends all the repos a source yields over the passed in channel
	// as SourceResults
	ListRepos(context.Context, chan SourceResult)
	// ExternalServices returns the ExternalServices for the Source.
	ExternalServices() ExternalServices
}
```

## Syncing

We keep a list of all repositories on Sourcegraph in the [`repo` table](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/database/schema.md#table-public-repo). This is to provide a code host independent list of repositories on Sourcegraph that we can quickly query. `repo-updater` will periodically sync each code host connection in the background. It compares the list of repos configured with those in our `repo` table and ensures that they are consistent. The syncer respects limits set in the site config for `userRepos.maxPerSite` (20000 by default) and `userRepos.maxPerUser` (2000 by default) and if either of these limits are exceeded, the code host connection will stop syncing until the limits are increased or the excess repositories are removed.

See [`Syncer.SyncExternalServices`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/syncer.go#L447) for details.

## Git Update Scheduler

We can't clone all repositories concurrently due to resource constraints in Sourcegraph and on the code host. So `repo-updater` has an [update scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/scheduler.go). Cloning and fetching are treated in the same way, but priority is given to newly discovered repositories.

The scheduler is divided into two parts:

- [`updateQueue`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/scheduler.go#L469:6) is a priority queue of repositories to clone/fetch on `gitserver`.
- [`schedule`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/scheduler.go#L651:6) which places repositories onto the `updateQueue` when it thinks it should be updated. This is what paces out updates for a repository. It contains heuristics such that recently updated repositories are more frequently checked.

Repositories can also be placed onto the `updateQueue` if we receive a webhook indicating the repository has changed. (By default, we don't set up webhooks when integrating into a code host.) When a user directly visits a repository on Sourcegraph, we also enqueue it for update.

The [update scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/repos/scheduler.go#L174) has a number of workers equal to the value of [`conf.GitMaxConcurrentClones`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/schema/site.schema.json#L596-601), which process the `updateQueue` and issue git clone/fetch commands via an RPC call to the appropriate gitserver instance. It is important to remember that the `updateQueue` only exists in memory in `repo-updater`. `gitserver` has no knowledge of the queue and only handles requests to update repositories.

See [this diagram](update-queue.svg) which shows the relationship between the scheduler and update queue.

>NOTE: gitserver also enforces `GitMaxConcurrentClones` per shard. So it is possible to have `GitMaxConcurrentClones * GITSERVER_REPLICA_COUNT` clone/fetch running, although uncommon.

## Identity Coherence

Repositories can be referenced using an internal ID that is coherent across updates, deletes, and even re-adding the original repository name to Sourcegraph after deleting. This ID refers to the primary key column [`id`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/internal/types/types.go#L33) in the [`repo` table](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.36.3/-/blob/internal/database/schema.md#table-public-repo).
