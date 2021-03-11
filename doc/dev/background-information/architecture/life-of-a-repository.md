# Life of a repository

This document describes how our backend systems clone and update repositories from a code host.

## High level

1. An admin configures a [code host configuration](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24%40v3.14.0+file:%5Eschema/%28aws%7Cbit%7Cgit%7Cother%29.*schema%5C.json%24&patternType=literal).
2. `repo-updater` periodically [syncs](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/syncer.go#L101) all repository metadata from configured code hosts.
  1. We [poll](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/syncer.go#L354:18) the code host's API based on the configuration.
  2. We [add/update/remove](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/syncer.go#L142-147) entries in our [`repo` table](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/frontend/db/schema.md#table-public-repo).
3. All repositories in our `repo` table are in a [scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/scheduler.go#L82-95) on `repo-updater` which ensures they are cloned and updated on [`gitserver`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/gitserver/server/server.go#L385:18).

Our guiding principle is to ensure all repositories configured by a site administrator are cloned and up to date. However, we need to avoid overloading a code host with API and Git requests.

>NOTE: Sourcegraph.com is different since it isn't feasible to maintain a clone of all open source repositories. It works via on-demand requests from users.

>NOTE: There is one other way repositories are fetched. A new commit may not be on Sourcegraph yet, but a user is browsing it via our browser extension. `gitserver` supports a ["EnsureRevision"](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/gitserver/server/server.go#L645) use-case which will do a "git fetch" for the missing revision.

## Services

`repo-updater` is responsible for communicating with code host APIs and co-ordinating the state we synchronise from them. It is a singleton service. It is responsible for maintaining the `repo` table which other services read. It is also responsible for scheduling clones/fetches on `gitserver`. It is also responsible for anything which communicates with a code host API. So our campaigns and background permissions syncers also live in `repo-updater`.

`gitserver` is a scaleable stateful service which clones git repositories and can run git commands against them. All data maintained on this service is from cloning an upstream repository. We shard the set of repositories across the gitserver replicas. The main RPC gitserver supports is `exec` which returns the output of the specified git command.

>NOTE: The name `repo-updater` does not accurately capture what the service does. This is a historical artifact. We have not updated it due to the unneccessary operational burden it would put on our customers.

## Discovery

Before we can clone a repository, we first must discover that it exists. This is configured by a site administrator setting code host configuration. Typically a code host will have an API as well as git endpoints. A code host configuration typically will specify how to communicate with the API and which repositories to ask the API for. For example:

``` json
{
  "url": "https://github.com",
  "token": "deadbeaf",
  "repositoryQuery": ["affiliated"],
}
```

This is a GitHub code host configuration for `github.com` using the private access token `deadbeaf`. It will ask GitHub for all affiliated repositories. Follow [`GithubSource.listRepositoryQuery`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/github.go#L612) to find the actual API call we do.

Discovering the repositories for each codehost/configuration is abstracted in the [`Sources interface`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/sources.go#L82:1).

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

We keep a list of all repositories on Sourcegraph in the [`repo` table](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/frontend/db/schema.md#table-public-repo). This is to provide a code host independent list of repositories on Sourcegraph that we can quickly query. `repo-updater` will periodically sync each code host connection in the background. It compares the list of repos configured with those in our `repo` table and ensures that thet are consistent. See [`Syncer.SyncExternalServices`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.25.0/-/blob/internal/repos/syncer.go#L166) for details.

## Git Update Scheduler

We can't clone all repositories concurrently due to resource constraints in Sourcegraph and on the code host. So `repo-updater` has an [update scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/scheduler.go). Cloning and fetching are treated in the same way, but priority is given to newly discovered repositories.

The scheduler is divided into two parts:

- [`updateQueue`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/scheduler.go#L392:6) is a priority queue of repositories to clone/fetch on `gitserver`.
- [`schedule`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/scheduler.go#L567:6) which places repositories onto the `updateQueue` when it thinks it should be updated. This is what paces out updates for a repository. It contains heuristics such that recently updated repositories are more frequently checked.

Repositories can also placed onto the `updateQueue` if we receive a webhook indicating the repository has changed. (We don't by default setup webhooks when integrating into a code host). When a user directly visits a repository on Sourcegraph we also enqueue it for update.

The [update scheduler](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/cmd/repo-updater/repos/scheduler.go#L165:27) has [`conf.GitMaxConcurrentClones`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@v3.14.0/-/blob/schema/site.schema.json#L235-240) workers processing the `updateQueue` and issuing git clone/fetch commands.

>NOTE: gitserver also enforces `GitMaxConcurrentClones` per shard. So it is possible to have `GitMaxConcurrentClones * GITSERVER_REPLICA_COUNT` clone/fetch running, although uncommon.
