# Life of a search query

This document describes how our backend systems serve search results to clients. There are multiple kinds of searches (e.g. text, repository, file, symbol, diff, commit), but this document will focus on text searches.

## Clients

There are a few ways to perform a search with Sourcegraph:

1. Typing a query into the search bar of the Sourcegraph web application.
2. Typing a query into your browser's location bar after configuring a [browser search engine shortcut](https://docs.sourcegraph.com/integration/browser_extension/how-tos/browser_search_engine).
3. Using the [src CLI command](https://github.com/sourcegraph/src-cli).

Clients use either the [Streaming API](../../../api/stream_api/index.md) or the [search query](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+%5Cbsearch%5C%28+file:schema.graphql&patternType=regexp) in our GraphQL API. Both are exposed in our [frontend](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/frontend) service.

## Frontend

The frontend implements the Streaming API [here](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+func+%28h+*streamHandler%29+ServeHTTP). The Streaming API is used by the browser. Historically we served results via [GraphQL](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+func+%28r+*schemaResolver%29+Search%28) and there are still many clients who use this API. Internally Sourcegraph search is streaming based.

First, the frontend takes the query and [creates a plan of jobs](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+func+NewPlanJob&patternType=literal) to execute concurrently. A job is a specific query against a backend. For example [here we convert a Sourcegraph query into a Zoekt query](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+func+querytozoektquery&patternType=literal) for our indexed search backend. The comments in the job creation function are worth reading for more details. Additionally you can experiment with the debug command at [`./dev/internal/cmd/search-plan`](https://github.com/sourcegraph/sourcegraph/blob/main/dev/internal/cmd/search-plan/search-plan.go) to understand the jobs that are generated.

Most Sourcegraph queries can be directly translated into Zoekt queries without consulting our database of repositories. However, not all repositories or revisions are indexed. So we need to work out what isn't indexed by Zoekt so we can do unindexed queries against Searcher. So the frontend [determines which repository@revision combinations are indexed by Zoekt](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+zoektIndexedRepos%28+file:indexed_search.go&patternType=literal) by [consulting an in-memory cache](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+type+cachedSearcher). See [RepoSubsetTextSearch](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+type+RepoSubsetTextSearch&patternType=literal) for how searcher is queried.

## Zoekt (indexed search)

zoekt-webserver [serves search requests](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/zoekt%24+serveSearchErr%28&patternType=literal) by [iterating through matches in the index](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/zoekt%24+func+%28d+*indexData%29+Search&patternType=literal). It watches the index directory and loads/unloads index files as they come and go.

To decide what to index [zoekt-sourcegraph-indexserver](https://sourcegraph.com/github.com/sourcegraph/zoekt/-/tree/cmd/zoekt-sourcegraph-indexserver) sends an [HTTP Get request to the frontend internal API](https://sourcegraph.com/search?q=context:global+r:github.com/sourcegraph/+-file:%28doc%7Ctest%7Cspec%29+%22/repos/index%22+fork:yes&patternType=regexp) at most once per minute to fetch the list of repository names to index. For each repository the indexserver will compare what Sourcegraph wants indexed (commit, configuration, etc.) to what is already indexed on disk and will start an index job for anything that is missing.

What we index in a repository is affected by admin configuration (branches, large file allow list). For each repository the indexserver [asks the frontend for configuration](https://sourcegraph.com/search?q=context:global+r:github.com/sourcegraph/+-file:%28doc%7Ctest%7Cspec%29+%22/search/configuration%22+fork:yes&patternType=regexp).
It uses [git shallow clones](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/zoekt%24+-file:_test+--depth%3D1&patternType=literal) via [gitserver](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+-file:%28test%7Cspec%7Cdoc%29+GitUploadPack&patternType=literal) to fetch the contents of the branches to index. It then calls out to `zoekt-git-index` which creates 1 or more shards containing the indexes used by `zoekt-webserver`.

## Searcher (non-indexed search)

Searcher is a horizontally scalable stateless service that performs non-indexed code search. Each request is a search on a single repository (the frontend searches multiple repositories by sending one concurrent request per repository). To serve a [search request](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:search/search.go+s.streamSearch), it first [fetches a zip archive of the repo at the desired commit](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:searcher+GetZipFileWithRetry&patternType=literal) from [gitserver](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+FetchTar:+file:searcher/shared/shared.go) and then [iterates through the files in the archive](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+readerGrep+Find&patternType=regexp&case=yes) to perform the actual search.

Searcher will [offload work](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:searcher+file:hybrid.go) to Zoekt to speed up response times. It will ask gitserver for a diff of what has changed between the indexed commit and the current request. Using that information it only needs to search a subset of changed files, the rest can be searched by zoekt.
