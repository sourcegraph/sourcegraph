# Explicit permissions API

Sourcegraph's GraphQL API allows users to explicitly set repository permissions. This is an alternative to other mechanisms, which involve directly talking to the code host.

<span class="virtual-br"></span>

## Permissions mechanisms in parallel

<span class="badge badge-note">Up to Sourcegraph 5.0</span>

> NOTE: Up to version 5.0, if the Explicit permissions API is enabled, all the other repository permissions mechanisms are disabled. If you're using Sourcegraph with multiple code hosts, it's not possible to use the explicit permissions API for one code host and use other mechanisms for getting permissions for others.

<span class="badge badge-experimental">Experimental</span>
<span class="badge badge-note">Sourcegraph 5.0+</span>

If you want to use explicit permissions API alongside synced permissions, read section [permission mechanisms in parallel here](index.md#permissions-mechanisms-in-parallel). 
## Recommendations

We only recommend to use explicit permissions API in cases, where the other methods are not possible or effective. 
E.g. if a code host does not support permission syncing/webhooks or if it would take an unreasonable amount of resources/time to sync permissions from the code host.

It's also a good idea to use explicit permissions API if the source of truth for the codehost permissions is already defined in some external system, e.g. LDAP group membership.
In that case, it might be less resource intensive to sync the permissions from external source of truth directly via a periodically running routine.

## SLA

Sourcegraph SLA is, that **p95 of write requests to the explicit permissions API will be resolved within 10 seconds**.

Sourcegraph does not provide SLA for how fresh the permissions are, since the data is provided as is to the API.

## Disadvantages

It is important to note, that when using explicit permissions API, the permissions are written to the database as provided, without further verification that such permissions do exist on the code host side.

Keeping the permissions in sync and fresh is the responsibility of the site admins.

## Configuration

To enable the permissions API, add the following to the [site configuration](../config/site_config.md):

```json
"permissions.userMapping": {
    "enabled": true,
    "bindID": "username"
}
```

The `bindID` value specifies how to uniquely identify users when setting permissions:

- `username`: You can [set permissions](#setting-repository-permissions-for-users) for users by specifying their Sourcegraph usernames. Using usernames is **preferred**, as usernames are required to be unique for each user.
- `email`: You can [set permissions](#setting-repository-permissions-for-users) for users by specifying their email addresses (which must be verified emails associated with their Sourcegraph user account). This method can lead to unexpected results if there are multiple Sourcegraph user accounts with the same verified email address.

After you enable the permissions API, you must [set permissions](#setting-repository-permissions-for-users) to allow users to view repositories (site admins bypass all permissions checks and can always view all repositories).

> NOTE: If you were previously using [permissions syncing](./syncing.md), e.g. syncing permissions from Github, then those permissions are used as the initial state after enabling explicit permissions. Otherwise, the initial state is for all repositories to have an empty set of authorized users, so users will not be able to view any repositories.

<span class="virtual-br"></span>

## Setting a repository as unrestricted

Sometimes it can be useful to mark a repository as `unrestricted`, meaning that it is available to all Sourcegraph users. This can be done with the `setRepositoryPermissionsUnrestricted` mutation. Marking a repository as unrestricted will disregard any previously set explicit or synced permissions. Setting `unrestricted` back to `false` will restore the previous behaviour.

For example:

```graphql
mutation {
  setRepositoryPermissionsUnrestricted(repositories: ["<repo ID>", "<repo ID>", "<repo ID>"], unrestricted: true)
}
```

## Setting repository permissions for users

Setting the permissions for a user can be accomplished with 2 [GraphQL API](../../api/graphql.md) calls.

First, obtain the ID of the repository from its name:

```graphql
query {
  repository(name: "github.com/owner/repo") {
    id
  }
}
```

Next, set the list of users allowed to view the repository:

```graphql
mutation {
  setRepositoryPermissionsForUsers(
    repository: "<repo ID>",
    userPermissions: [
      { bindID: "alice" },
      { bindID: "bob" },
    ]) {
    alwaysNil
  }
}
```

Now, only the users specified in the `userPermissions` parameter will be allowed to view the repository. Sourcegraph automatically enforces these permissions for all operations. (Site admins bypass all permissions checks by default. See the [Site administrators](./index.md#site-administrators) section)

You can call `setRepositoryPermissionsForUsers` repeatedly to set permissions for each repository, and whenever you want to change the list of authorized users.

### Listing a user's authorized repositories

You may query the set of repositories visible to a particular user with the `authorizedUserRepositories` [GraphQL API](../../api/graphql.md) query, which accepts a `username` or `email` parameter to specify the user:

```graphql
query {
  authorizedUserRepositories(username: "alice", first: 100) {
    nodes {
      name
    }
    totalCount
  }
}
```
