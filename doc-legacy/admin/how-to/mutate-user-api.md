# Admin Level GraphQL Mutations


## How to update or remove users with the GraphQL API console

This document walk you through the steps of using mutation queries to update or remove users via the GraphQL API console. 

## Prerequisites

* This document assumes that you have site-admin level permissions on your Sourcegraph instance
* Assumes that you have set up your query token [via the steps here](https://docs.sourcegraph.com/api/graphql#quickstart)

## Update a user

First, query the user's ID by using their email address or user name

Example:

```graphql
{
  user(email: "someone@gmail.com") {
    id
  }
}
```

```graphql
{
  user(username: "username") {    
  id
  }
}
```

Once you've got a user's ID use the `updateUser` mutation query, to change user metadata. An example of updating a user's username can be found below:

```graphql
mutation {
  updateUser(user: "REDACTED", username: "doombot") {
    username
  } 
}
```
> NOTE: `REDACTED` is a placeholder for a user ID

This query will return the username after altering the user's username data as seen below:
```graphql
{
  "data": {
    "updateUser": {
      "username": "doombot"
    }
  }
}
```
Learn more about the options available with the `updateUser` query in the graphQL API consule Documentation Explorer.

## Remove a user

### There are two different options for removing a user:

**Option A) Deleting a user:** the user and *all* associated data is marked as deleted in the DB and never served again. You could undo this by running DB commands manually.

**Option B) Nuking a user:** the user and *all* associated data is deleted forever. *Note: You cannot undo this and this is considered the less safe option.*

First, query the user's ID by using their email address or user name, as seen [above](#update-a-user).

Example:

```graphql
{
  user(email: "someone@gmail.com") {
    id
  }
}
```

```graphql
{
  user(username: "username") {    
  id
  }
}
```

Next, plug the user ID into either Option A. or B. to delete the account.

Option A) example:

```graphql
mutation {
  deleteUser(user: "THE_USER_ID") {
    alwaysNil
  }
}
```

Option B) example, include `hard: true`:

```graphql
mutation {
  deleteUser(user: "THE_USER_ID" hard: true) {
    alwaysNil
  }
}
```

**Optional step:** Recheck the delete worked by running the query from step #1. again. If the results are “user not found:...” then it worked. 

Example:

```graphql
{
  "errors": [
    {
      "message": "user not found: [someone@gmail.com]",
      "path": [
        "user"
      ]
    }
  ],
  "data": {
    "user": null
  }
}
```

## How to Import your code host and repositories into Sourcegraph using GraphQL
Sourcegraph site admins can use graphql API to import a code host as well as repositories into their Sourcegraph instance using `mutation.AddExternalService`.
This is as below:

```
mutation {
  addExternalService(
    input: {input: {kind: $codehost_type, displayName: "$example_name", config: "{\"url\":\"https://example.com\",\"token\":\"xxxxxxxxxxx\",\"repos\":[\"<owner>/<reponame>]"}){
    id
    nextSyncAt
  }
}
```
You can also use the documentation explorer on the right-hand side of the `API console` page to explore the fields available as well as use `Ctrl`+ `Spacebar` on Mac to bring up suggestions.

## Further resources

* [Sourcegraph - User data deletion](https://docs.sourcegraph.com/admin/user_data_deletion)
* [Sourcegraph GraphQL API](https://docs.sourcegraph.com/api/graphql)
