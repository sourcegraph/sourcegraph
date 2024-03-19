# Security patterns

## Authorization

Authorization is the gatekeeper for securing private resources by preventing unauthorized access. Examples of private resources are: 

- Contents of a private repository
- Endpoints that can only be called by certain users (e.g. site admins)
- User settings or organization settings

Current forms of authorization in Sourcegraph include:

 - Site admin roles
 - Organization memberships
 - Batch changes permissions
 - Repository permissions
 - Same-user validation

As a standard practice, users who do not have access to a given private resource should not be aware of the existence of that private resource. When only part of a resource is restricted by authorization limitations, it is reasonable to prompt the user that some resources are not available due to insufficient permissions. However, we must not provide anything that indicates what those restricted resources might be.

### Enforce authorization

In Sourcegraph, there are two places to enforce authorization, both equally important:

- At the GraphQL layer:
    - Some endpoints are restricted to certain users (e.g. site admins or the same user).
    - Be aware that any backend failure has the potential to indicate unauthorized information about private resources. Therefore, halt the process as soon as we identify an unauthorized request, and behave as if the resource does not exist at all.
- At the database layer:
    - The database is our source of truth for authorization, especially for repository and batch changes permissions. Enforcing authorization at this layer is absolutely necessary.

### Authorization implementation

Within our Go code, such as the frontend and repo-updater services, metadata representing the current user or actor is kept within the current context. It can be accessed using `actor.FromContext(ctx)`.

#### Actors

The [`Actor`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@c764844ef9574cef5533147d756ae98e613d0ae6/-/blob/internal/actor/actor.go?L15&subtree=true) type represents the current actor. In most cases, this is the user who issued the request.

More specifically, an actor will be one of these three cases:

1. An authenticated user, in which case the `UID` field will be set to a non-zero value.
2. A guest user, in which case the `UID` field will be zero.
3. An internal actor, which indicates that the current operation was started by an internal Sourcegraph process.

#### Checking authorization

Care must be taken when checking an actor: it's possible to have a context that doesn't have an actor at all, in which case `actor.FromContext()` will return `nil`.

Code in `cmd/frontend` can take advantage of [the helper functions available in `cmd/frontend/backend`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@c764844/-/docs/cmd/frontend/backend#func) such as `CheckCurrentUserIsSiteAdmin` and `CheckSiteAdminOrSameUser`. These functions already take the details of internal and user actors into account, and are the safest way to perform authorization checks at the frontend level (for example, in GraphQL resolvers).

Below the frontend level, or in other commands, you'll need to implement more of your own authorization logic. Two `nil`-safe methods are provided on the `Actor` type: `IsAuthenticated()` and `IsInternal()`, which are always safe to call. The `UID` field must only be accessed after calling `IsAuthenticated()`.

As a general rule, internal actors should always be considered authorized, so your checks will often take this general form:

```go
func isAuthorized(ctx context.Context, uid int32) bool {
    actor := actor.FromContext(ctx)
    if actor.IsInternal() {
        return true
    }
    if !actor.IsAuthenticated() {
        return false
    }
    if actor.UID == uid {
        return true
    }

    // Get the user by ID and check admin status if necessary.
}
```

## Secret management

Secrets used by our applications are stored in GCP Secret Manager, *never* in source code unless it is mock/test data. Internal documentation on how to store/consume secrets for development can be found in [this document](https://docs.google.com/document/d/1Qm5P4KbyVMP_KyPvud0qyqUb43RK3lTFMjAeE6623Nw/edit).
