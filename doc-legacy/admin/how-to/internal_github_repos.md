# How to import internal organization GitHub repositories to Sourcegraph

This document will walk you through the steps of adding internal organization GitHub repositories to Sourcegraph without manually having to maintain an explicit list.

## Prerequisites

This document assumes that you have:

* Site-admin level permissions on your Sourcegraph instance.
* Access to your Sourcegraph deployment.
* Internal Github repositories in your organization.

## Steps to import internal GitHub repositories
1. Using the [repositoryQuery](https://docs.sourcegraph.com/admin/external_service/github#repositoryQuery) configuration option, pass the `org` flag to specify the name of the organization the internal repositories belong to and;
2. Add `is : internal` to the same line.

For example:

``` 
"repositoryQuery": [
    "org:$name is:internal"
  ],
```

### How to check that you now have internal repositories
Confirm that you have cloned repositories in your instance by accessing your GraphQL console on `$your_sourcegraph_url/api/console` and running the following query;

```
query{
  externalServices{
    nodes{
      kind
      displayName
      repoCount
    }
  }
}
```
## Further resources

* [Sourcegraph GraphQL API](https://docs.sourcegraph.com/api/graphql)
