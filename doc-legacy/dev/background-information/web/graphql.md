# Working with GraphQL

The webapp and browser extension interact with our backend through a strongly typed GraphQL API.
We auto-generate TypeScript types for the schema and all queries, mutations and fragments to ensure the client uses the API correctly.

## GraphQL Client
We use [Apollo Client](https://www.apollographql.com/docs/react/) to manage data-fetching and caching within our app. It provides a set of declarative interfaces which abstract away a lot of repeated code, whilst supporting a 'stale-while-revalidate' strategy through a normalized client cache of requested data.

**Writing and running your query**

We use `gql` template strings to declare our GraphQL queries.

Each query must have a globally unique name as per the [GraphQL specification](https://spec.graphql.org/June2018/#sec-Operation-Name-Uniqueness). Typically we should name our queries similarly to how we might name a function, by describing what the query will return. For mutations, we should prefix the name with a verb like `Delete` or `Update`, this will help avoid collisions between queries and mutations.

Using each unique query, we can generate specific types so you can receive autocompletion, syntax highlighting, hover tooltips and validation in your IDE.

Once you have built your query, `graphql-codegen` will generate the correct request and response types. This process should happen automatically through local development, you can also manually trigger this by running `pnpm generate`. Since the generator is lazy and won't generate types for fields that aren't used, if you're adding a new field then you **must** write at least one query that actually uses the field before running the generator.

Using a `useQuery` hook, we can easily fire a request and handle the response correctly.

```ts
// ./MyComponent.tsx
import { useQuery, gql } from '@sourcegraph/http-client'

import { UserDisplayNameResult, UserDisplayNameVariables } from '../../graphql-operations'

export const USER_DISPLAY_NAME = gql`
  query UserDisplayName($username: String!) {
      user(username: $username) {
          id
          displayName
      }
  }
`

const MyComponent = ({ username }: { username: string }) => {
  const { data, loading, error } = useQuery<UserDisplayNameResult, UserDisplayNameVariables>(USER_DISPLAY_NAME, { variables: { username } });

  if (loading) {
    // handle loading state
  }

  if (error) {
    // handle error state
  }

  if (data) {
    // display to user
  }
}
```

Equally, it is possible to create our own wrapper hooks, when we want to modify data accordingly.

```ts
const useFullName = (variables: UserDisplayNameVariables): ApolloQueryResult<{ fullName: string }> => {
    const response = useQuery<UserDisplayNameResult, UserDisplayNameVariables>(USER_DISPLAY_NAME, { variables })

    if (response.error) {
        // Handle error
    }

    return {
        ...response,
        data: {
            fullName: `${response.data.user.firstName} ${response.data.user.lastName}`,
        },
    }
}
```

## Frequently asked questions

### How do I use the Apollo cache?
Apollo uses a normalized in-memory cache to store the results of different queries, most of this happens automatically!

Apollo generates a composite key for each identifiable object in a response. This is typically done by combining the `__typeName` with the `id` that is returned in the response. For example:

When firing this request:
```
{
  user {
    __typename
    id
    displayName
  }
}
```

Assuming `__typename === User` and `id === 1234`, the response data is added to the cache under the key:
`User:1234`.

When a different query requests similar data, Apollo will merge both responses into the cache and update both parts of the UI with the latest data. This is useful to form links between components whilst still ensuring that the components are self-contained. For example, if a mutation updated `displayName` in a different component, then these components that *query* `displayName` would automatically receive the updated value and re-render.

**All queries should return an object identifier**. If this identifier is not under the `id` field, we need to inform Apollo which field to use in order to generate the correct key. See the [docs](https://www.apollographql.com/docs/react/caching/cache-configuration/#customizing-identifier-generation-by-type) for more information on this.

### Should I always use the Apollo cache?
Equally important to knowing how to use the cache is knowing when to use the cache, or when **not to**. Coming from a setup with no client-side caching, it's easy to not think about this question until after it's bitten you.

The stale-while-revalidate strategy is very widely used these days, and with good reason, but it's not the best strategy for every type of data. By default when you hook your UI up to Apollo, a user who visits a page that can make use of data that's already in the cache will see that data right away. That data may be out-of-date, but Apollo will already be refetching it in the background. In many cases your data might not be changing, but if something **did** change, there will be a flash of old data, which can be jarring and unexpected to the user or cause significant [layout shift](https://web.dev/cls/). In certain situations, a loading spinner or a different transition from old to new data might actually be preferable.

As a general rule of thumb, we should choose to leverage the default cache behavior if:

- Having content available immediately is more valuable than having all available content be completely up-to-date
- The data we are caching is not expected to change significantly over the course of the user's visit to the site

This will typically hold true for the majority of application data. However, there are still exceptions, when agreeing with what's on the server is a higher priority. Examples might include:

- One-off tokens
- Data tied to the current date/time
- Alerts or notifications
- Shared data that is frequently impacted by other users' actions
- Data that drives the main source of interaction on the page (e.g. form buttons dynamically rendered from data in the server response)
- Data that is subject to frequent, asynchronous side-effects of other mutations or background jobs

If you find yourself wanting to bypass the cache, you can set a query [fetch policy](https://www.apollographql.com/docs/react/data/queries/#setting-a-fetch-policy) like `network-only` or `no-cache` to keep your data fresh.

Lastly, for the request data that you *do* choose to cache, it's best to make decisions about showing that stale data while Apollo is refetching your query on a case-by-case basis. There are generally four strategies you can adopt with your UI components:

1. Show just the stale data
    - Best for data that doesn't change or isn't harmful if out-of-date
2. Show the stale data *with a loading spinner*
    - Best for data that cares about being up-to-date but is not likely to cause user confusion or significant layout shift when it changes
3. Show the stale data *with a prompt to trigger a UI update when the new data is ready*
    - Best for data that cares about being up-to-date but may cause confusion or significant layout shift when it changes, such as paginated data
4. Show just a loading spinner
    - Best for data that is always changing or should always agree with the server; effectively, no caching

### How should I write tests that handle data-fetching?
Apollo lets us easily mock queries in our tests without having to actually mock out our own logic. The tests will fail if an un-mocked query fires through Apollo, so it is important to accurately build mock requests. In order to test how the UI displays a response, you can provide a mocked result. See this example:

```ts
import { render } from '@testing-library/react'

import { getDocumentNode } from '@sourcegraph/http-client'
import { MockedTestProvider } from '@sourcegraph/shared/src/testing/apollo'

import { MyComponent, USER_DISPLAY_NAME } from './MyComponent'

const mocks = [
    {
        request: {
            query: getDocumentNode(USER_DISPLAY_NAME),
            variables: {
                username: 'mock_username',
            },
        },
        result: {
            data: {
                user: {
                    displayName: 'Mock DisplayName',
                },
            },
        },
    },
]

describe('My Test', () => {
    it('works', () => {
        const { getByText } = render(
            <MockedTestProvider mocks={mocks}>
                <MyComponent />
            </MockedTestProvider>
        )
        expect(getByText('Your display name is: Mock DisplayName')).toBeVisible();
    })
})
```

### How can I run a query outside of React?
Most queries should be requested in the context of our UI and should use hooks. If there is a scenario where this is not possible, it is still possible to realise the benefits of Apollo without relying this approach. We can imperatively trigger any query using `client.query`.

```ts
import { getDocumentNode } from '@sourcegraph/http-client'

import { client } from './backend/graphql'
import {
    UserDisplayNameResult,
    UserDisplayNameVariables,
} from './graphql-operations'

const getUserDisplayName = async (username: string): Promise<UserDisplayNameResult> => {
    const { data, error } = await client.query<UserDisplayNameResult, UserDisplayNameVariables>({
        query: getDocumentNode(UserDisplayName),
        variables: { username },
    })

    if (error) {
        // handle error
    }

    return data
}
```

### I have an issue, how can I debug?
Aside from typical debugging methods, Apollo provides [Apollo Client Devtools](https://www.apollographql.com/docs/react/development-testing/developer-tooling/#apollo-client-devtools) as a browser extension to help with debugging. This extension will automatically track query requests and responses, and provide a visual representation of cached data.

### We have different ways of requesting data from GraphQL, why?
Our code isn't yet fully aligned to a single-approach, but this is something we are working towards over time.

A lot of older code uses the non-generic `queryGraphQl()` and `mutateGraphQl()` functions.
These are less type-safe, because they return schema types with _all_ fields of the schema present, no matter whether they were queried or not.

Other code uses `requestGraphQL()`, this is an improved approach which provides better types, but it doesn't scale well when requesting data across multiple areas of the application, and often leads to cross-component dependencies.

Our goal is to migrate more code to use Apollo. This should make our components more self-contained, increase perceived performance with client-side caching and make it easier to write effective tests.

## Writing a React component or function that takes an API object as input

React components are often structured to display a subfield of a query.
The best way to declare this input type is to define a _GraphQL fragment_ with the component, then using the auto-generated type for that fragment.
This ensures the parent component don't forget to query a required field and it makes it easy to hard-code stub results in tests.

```tsx
import { PersonFields } from '../graphql-operations'

export const personFields = gql`
    fragment PersonFields on Person {
        name
    }
`

export const Greeting: React.FunctionComponent<{ person: PersonFields }> = ({ person }) =>
    <div>Hello, {person.name}!</div>
```

Since the fragment is exported, parent components can use it in their queries to include the needed data:

```ts
import { personFields } from './greeting',

export const PEOPLE = gql`
    query People {
        people {
            nodes {
                ...PersonFields
            }
        }
    }
    ${personFields}
`
```

**Note**: A lot of older components still use all-fields types generated from the whole schema (as opposed to from a fragment), usually referenced from the namespace `GQL.*`.
This is less safe as fields could be missing from the actual queries and it makes testing harder as hard-coded results need to be casted to the whole type.
Some components also worked around this by redeclaring the type structure with complex `Pick<T, K>` expressions.
When you need to interface with these, consider refactoring them to use a fragment type instead.

## Pagination in GraphQL

### Best-practices for paginating resources with GraphQL

Every query thar returns more items than can be visible in the UI at any given point should implement a paginated response. This is important because some of our largest customers have vastly bigger amounts of data and we need to ensure that the interfaces become usable regardless of the size.

For our GraphQL API we follow the Relay-style connection spec ([Relay spec](https://relay.dev/graphql/connections.htm), [Apollo spec](https://www.apollographql.com/docs/react/pagination/cursor-based/#relay-style-cursor-pagination)) and cursor-based pagination.

**Cursor-based pagination** refers to a pagination API that _does not know about relative page numbers_. Instead, every API response contains a opaque _cursor_ that we can use to load the next or previous page of data. It's important that for the front-end, this _cursor_ is never introspected. It is handled as a generic `string` that is just passed from the API response directly into the next API request.

In contrast to **offset-based pagination**, this approach brings us these benefits:

- Adding a new item to the collection while someone else is paginating through the collection will avoid inconsistencies (no items are showed more than once or skipped entirely).
- We can leverage postgres indexes to find the cursor quickly and avoid having to do a sequential scan through all rows ([learn more](https://use-the-index-luke.com/no-offset)).

### Pagination UI patterns at Sourcegraph

We currently have two accepted pagination patterns:

- **Page switcher pagination**: Here, we only show one page of data (usually in the range of ~30 items) in the view at all times. The backend needs to allow pagination in both directions (forwards _and backwards_) for this to be possible. This is the preferred pagination pattern for elements that we can fetch from the postgres database.

  To implement this, you can use the following abstractions:

  - React hook for data loading: [`usePageSwitcherPagination()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/components/FilteredConnection/hooks/usePageSwitcherPagination.ts)
  - Go abstraction to build a bi-directional paginated resource: [`ConnectionResolver`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/graphqlutil/connection_resolver.go)
  - Wildcard component for pagination UI: [`<PageSwitcher />`](https://storybook.sgdev.org/?path=/story/wildcard-pageswitcher--simple)
  - Here are two example implementations: [#45705](https://github.com/sourcegraph/sourcegraph/pull/45705) for data served via Postgres, [#45070](https://github.com/sourcegraph/sourcegraph/pull/45070) for data served from git)

- **Show more pagination** (a.k.a. infinite-scrolling): Items are appended to the bottom of the list for every page that is loaded. This is suited only for resources that do not bi-directional pagination.

  We offer the following helpers for implementing a view with this pattern:

  - React hook for data loading: [`useShowMorePagination()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/components/FilteredConnection/hooks/useShowMorePagination.ts)
  - React component "Show More" UI: [`<ShowMoreButton />`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/search/panels/ShowMoreButton.tsx)

### Example query

To better understand how the cursor pattern works in GraphQL, take a look at this example query:

```graphql
query MyPaginatedQuery($first: Int, $last: Int, $after: String, $before: String) {
  myPaginatedQuery(
    first: $first
    last: $last
    before: $before
    after: $after
  ) {
    nodes {
      ...NodeFields
    }
    totalCount
    pageInfo {
      hasNextPage
      hasPreviousPage
      startCursor
      endCursor
    }
  }
}
```

When this query is run with `first: 10`, the first ten elements of the connection are returned and the result will look like this:

```jsonc
{
  "data": {
    "myPaginatedQuery": {
      "nodes": [
        // 10 nodes
      ],
      "totalCount": 100,
      "pageInfo": {
        "hasNextPage": true,
        "hasPreviousPage": false,
        "startCursor": "aabbccdd",
        "endCursor": "eeffgghh"
      }
    }
  }
}
```

You can see that this page has a next page but no previous page, which makes sense since it's the first page.
The interesting bits are the `startCursor` and `endCursor`. These encode the first or last item of the result list in a way that the backend can efficiently query the next page.

If we want to go to the next page, we can take this opaque token and set it as the `after` value of the query. The new pagination parameters would be `first: 10, after: 'eeffgghh'".

This is not something you need to manually implement in the frontend though. [`usePageSwitcherPagination()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/components/FilteredConnection/hooks/usePageSwitcherPagination.ts) and [`useShowMorePagination()`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/components/FilteredConnection/hooks/useShowMorePagination.ts) already do that for you.

