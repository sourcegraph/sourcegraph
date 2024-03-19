# Managing search contexts with the API

Learn how to manage [search contexts](../../code_search/how-to/search_contexts.md) on private Sourcegraph instances with the API. If you haven't used the API before, learn more about [the GraphQL API and how to use it](index.md).

## Prerequisites

* Search contexts and search context management are [enabled in global settings](../../code_search/explanations/features.md#search-contexts).

### Permissions and visibility overview

To read and write search contexts through the API you will need appropriate permissions. The permissions are determined based on the individual search context's namespace and its visibility (private or public).

**Read** permissions (view contents, use in searches):

* **Public** search contexts are available to all users
  * On Sourcegraph.com, unauthenticated visitors can also see public search contexts
* **Private** search contexts
  * With user namespace: only available to the user
  * With organization namespace: only available to users in the organization
  * With global (instance-level) namespace: only available to site-admins

**Write** permissions (create, update, delete):

* Site-admins have write access to all public search contexts and all global (instance-level) search contexts
* A regular user has write access to its search contexts and its organization's search contexts

## Create a context

Below is a GraphQL query that creates a new search context. To populate the `searchContext.namespace` property, you will have to query the API beforehand to retrieve the user or organization ID.

If `searchContext.namespace` is not specified or `null` then the context is created in the global (instance-level) namespace.
To specify search context repositories you will need their ids. Similar to the `namespace` property you will need to retrieve them from the API before creating the context.

```gql
mutation CreateSearchContext(
  $searchContext: SearchContextInput!
  $repositories: [SearchContextRepositoryRevisionsInput!]!
) {
  createSearchContext(searchContext: $searchContext, repositories: $repositories) {
    id
    spec
  }
}
```

Example variables:

```json
{
  "searchContext": {
    "name": "MySearchContext",
    "description": "A description of my search context",
    "namespace": "user-id",
    "public": true
  },
  "repositories": [
    {
      "repositoryID": "repo-id",
      "revisions": ["main", "branch-1"]
  	}
  ]
}
```

## Read a single context

Below is a GraphQL query that fetches a single search context by ID.

```gql
query ReadSearchContext($id: ID!) {
  node(id: $id) {
    ... on SearchContext {
      id
      spec
    }
  }
}
```

Example variables:

```json
{ "id": "search-context-id" }
```

## List available contexts

Below is a GraphQL query that fetches all available search contexts and allows filtering by multiple parameters. The `namespaces` array allows filtering by one or multiple namespace ids (user or organization id).
To include global (instance-level) contexts you can specify `null` as one of the ids. If the `namespaces` array is omitted or empty, then no filtering by namespace is applied and all available contexts are returned.
The `query` parameter allows filtering by search context spec.

```gql
query ListSearchContexts(
  $first: Int!
  $after: String
  $query: String
  $namespaces: [ID]
  $orderBy: SearchContextsOrderBy
  $descending: Boolean
) {
  searchContexts(
    first: $first
    after: $after
    query: $query
    namespaces: $namespaces
    orderBy: $orderBy
    descending: $descending
  ) {
    nodes {
      id
      spec
    }
    pageInfo {
      hasNextPage
      endCursor
    }
    totalCount
  }
}
```

Example variables:

Query with these variables will return the first 50 search contexts ordered by search context `updatedAt` timestamp ascending. The results will be filtered to include only
global contexts (`null`), `organization1` contexts (`organization1-id`), and `user1` contexts (`user1-id`).

```json
{
  "first": 50,
  "namespaces": [null, "organization1-id", "user1-id"],
  "orderBy": "SEARCH_CONTEXT_UPDATED_AT",
}
```


## Update a context

Below is a GraphQL query that updates an existing search context. You cannot update a search context namespace.
You have to provide the full search context and all repositories with revisions you want to keep on each update.

```gql
mutation UpdateSearchContext(
  $id: ID!
  $searchContext: SearchContextEditInput!
  $repositories: [SearchContextRepositoryRevisionsInput!]!
) {
  updateSearchContext(id: $id, searchContext: $searchContext, repositories: $repositories) {
    id
    spec
  }
}
```

Example variables:

```json
{
  "id": "search-context-id-to-update",
  "searchContext": {
    "name": "MyUpdatedSearchContext",
    "description": "An updated description of my search context",
    "public": false
  },
  "repositories": [
    {
      "repositoryID": "repo-id",
      "revisions": ["main", "branch-1", "branch-2"]
  	}
  ]
}
```

## Delete a context

Below is a GraphQL query that deletes a search context by ID.

```gql
mutation DeleteSearchContext($id: ID!) {
  deleteSearchContext(id: $id) {
    alwaysNil
  }
}
```

Example variables:

```json
{ "id": "search-context-id-to-delete" }
```
