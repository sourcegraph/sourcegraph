# How to create a search context with the GraphQL API

This document will take you through how to create a search context for your user with Sourcegraph's GraphQL API.

## Prerequisites

* This document assumes that you have a private Sourcegraph instance
* Assumes you are creating a Private search context with a user namespace which is only available to the user
* For more information on available permissions and ways to alter the following examples, please see [Managing search contexts with api - permissions and visibility overview](https://docs.sourcegraph.com/api/graphql/managing-search-contexts-with-api#permissions-and-visibility-overview)

## Steps to create


Step 1: Add to global configuration (must be site-admin):


```json
{
    "experimentalFeatures": {
      "showSearchContext": true
  }
}
```

Step 2: Make sure you have added code hosts: [Add repositories (from code hosts) to Sourcegraph](https://docs.sourcegraph.com/admin/repo/add)



Step 3: Follow the steps to [Generate an access token for GraphQL](https://docs.sourcegraph.com/api/graphql#quickstart) if you already haven't



Step 4: Navigate to the API console on your instance, replacing sourcegraph.example with the correct string for your instance URL.

* Example: `https://sourcegraph.example.com/api/console`



Step 5: Query your user namespace id and save the value

* The name: will be your Sourcegraph instance login name
Example:

```json
query {
  namespaceByName(name: "my_login_name") {
    id
  }
}
```

Step 6: Query your desired repo id and save the value.

* It should be whatever the URL is for that repo. 
Example: 

```json
query {
  repository(name: "github.com/org_name/repo_name") {
    id
  }
}
```

Step 7: Take the values from steps 5 and 6 and put them into the example variables from our docs here:

* [Managing search contexts with API - Create a context](https://docs.sourcegraph.com/api/graphql/managing-search-contexts-with-api#create-a-context)


Run this with no changes:

```json
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

Then in the Query Variables section on the bottom of the GraphQL API page, use this variables example, changing at least the name and description:

```json
{
  "searchContext": {
    "name": "MySearchContext",
    "description": "A description of my search context",
    "namespace": "user-id-from-step-5",
    "public": true
  },
  "repositories": [
    {
      "repositoryID": "repo-id-from-step-6",
      "revisions": ["main", "branch-1"]
  	}
  ]
}
```


Step 8: Run the query, that should create a search context and the output will look something like:


```json
{
  "data": {
    "createSearchContext": {
      "id": "V2VhcmNoQ29udGV4dDoiQGdpc2VsbGUvTXlTZWFyY2hDb250ZXh0MiI=",
      "spec": "@my_login_name/MySearchContext"
    }
  }
}
```



Step 9: Go to the main search page and you should see the new Search context as part of the search bar!

## Further resources

* [Using and creating search contexts](https://docs.sourcegraph.com/code_search/how-to/search_contexts)
* [Sourcegraph - Administration Config](https://docs.sourcegraph.com/admin/config)
