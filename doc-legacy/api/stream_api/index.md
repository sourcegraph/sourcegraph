# Sourcegraph Stream API

> NOTE:
> The Stream API is still evolving. Although parts of it can be considered
> stable, we don't guarantee backward compatibility just yet. This means it is
> possible that fields are added, removed, or renamed. All backward incompatible changes to the
> event stream format will be documented in the [CHANGELOG](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/CHANGELOG.md).


With the Stream API you can consume search results and related metadata as
a stream of events. The Sourcegraph UI calls the Stream API for all interactive searches.
Compared to our [GraphQL API](../graphql/index.md), it offers shorter times to first results and 
supports running exhaustive searches returning a large volume of results without
putting pressure on the backend.

## Endpoint
`/.api/search/stream`

## Request

```bash
curl --header "Accept: text/event-stream" \
     --header "Authorization: token <access token>" \
     --get \
     --url "<Sourcegraph URL>/.api/search/stream" \
     --data-urlencode "q=<query>" \
     ["display=<display-limit>"]
```

| parameter | description |
| --- | --- |
| access token | [Sourcegraph access token](https://docs.sourcegraph.com/cli/how-tos/creating_an_access_token) |
| Sourcegraph URL | The URL of your Sourcegraph instance, or https://sourcegraph.com. |
| query | A Sourcegraph query string, see our [search query syntax](../../code_search/reference/queries.md) |
| display-limit | The maximum number of matches the backend returns. Defaults to -1 (no limit). If the backend finds more then display-limit results, it will keep searching and aggregating statistics, but the matches will not be returned anymore. Note that the display-limit is different from the query filter `count:` which causes the search to stop and return once we found `count:` matches. |

See [Example](#example-curl).

## Event stream format 

The API responds with a stream of events. Each event consists of exactly two
fields, event and data, one per line. Events are separated by 2 newline
characters, `\n\n`. The value of `event:` is always a string that describes the
type of the event. The value of `data:` is always JSON.

```text
event: <event-type> // event 1
data: <JSON>

event: <event-type> // event 2
data: <JSON>

...

event: done // last event
data: {}
```

> NOTE: 
> Our format is a subset of the event stream format for [server-sent
> events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#event_stream_format). However, we do not guarantee compatibility with any third-party clients written for server-sent events.

### Event-types

Events can be of the following types:

| event-type | description |
| --- | --- |
| matches | matches can be of type content, path, commit, diff, symbol and repo |
| progress | statistics such as match count, count of repositories with matches, and duration |
| filters | suggestions for additional filters to further narrow down the search |
| alert | info, warning and error messages |
| done | always the last event |

Refer to the [interface definitions of our typescript client](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/shared/src/search/stream.ts?L12) to learn about the schema of the event-types. 

## Example (curl) 

On Sourcegraph.com we can run queries without authentication.

```shellsession
$ curl --header "Accept: text/event-stream" \
     --get \
     --url "https://sourcegraph.com/.api/search/stream" \
     --data-urlencode "q=r:sourcegraph/sourcegraph doResults count:1"

event: matches
data: [{"type":"content","path":"cmd/frontend/graphqlbackend/search_results_stats_languages.go","repositoryID":42693708,"repository":"gitlab.com/rluna-open-source/code-management/sourcegraph/sourcegraph-2020","repoLastFetched":"2021-11-19T21:54:27.009309Z","branches":[""],"commit":"0725aa021040f3c864bd5043caf965e7bc1e7a51","hunks":null,"lineMatches":[{"line":"\t\tresults, err := srs.sr.doResults(ctx, args, jobs)","lineNumber":44,"offsetAndLengths":[[25,9]]}]}]

event: progress
data: {"done":false,"matchCount":1,"durationMs":39,"skipped":[{"reason":"shard-match-limit","title":"result limit hit","message":"Not all results have been returned due to hitting a match limit. Sourcegraph has limits for the number of results returned from a line, document and repository.","severity":"info","suggested":{"title":"increase limit","queryExpression":"count:1000"}}]}

event: filters
data: [{"value":"lang:go","label":"lang:go","count":3,"limitHit":false,"kind":"lang"},{"value":"repo:^gitlab\\.com/rluna-open-source/code-management/sourcegraph/sourcegraph-2020$","label":"gitlab.com/rluna-open-source/code-management/sourcegraph/sourcegraph-2020","count":3,"limitHit":true,"kind":"repo"}]

event: progress
data: {"done":true,"repositoriesCount":1,"matchCount":1,"durationMs":39,"skipped":[{"reason":"shard-match-limit","title":"result limit hit","message":"Not all results have been returned due to hitting a match limit. Sourcegraph has limits for the number of results returned from a line, document and repository.","severity":"info","suggested":{"title":"increase limit","queryExpression":"count:1000"}}]}

event: done
data: {}
```

## FAQ

### Q: How can I run an exhaustive search directly against the Stream API?

To search a pattern over all indexed repositories, add `count:all` and remove all repo filters. For example, to search all indexed repositories for the string "secret", you can run the following command

```bash
curl --header "Accept:text/event-stream" --get --url "https://sourcegraph.com/.api/search/stream" --data-urlencode "q=secret count:all"
```

If you don't want to write your own client, you can also use Sourcegraph's [src-cli](https://sourcegraph.com/github.com/sourcegraph/src-cli).

```bash
src search -stream "secret count:all"
```

### Q: Are there plans for supporting a streaming client or interface with more functionality (e.g., parallelizing multiple streaming requests or aggregating results from multiple streams)?

There are currently no plans to support additional client-side functionality to interact with a streaming endpoint. We recommend users write their own scripts or client wrappers that handle, e.g., firing multiple requests, accepting and aggregating the return values, and additional result formatting or processing.
