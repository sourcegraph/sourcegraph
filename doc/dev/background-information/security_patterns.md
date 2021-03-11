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
    - Be aware that any backend failure has potential to indicate unauthorized information about private resources. Therefore, halt the process as soon as we identify an unauthorized request, and behave as if the resource does not exist at all.
- At the database layer:
    - The database is our source of truth for authorization, especially for repository and batch changes permissions. Enforcing authorization at this layer is absolutely necessary.
