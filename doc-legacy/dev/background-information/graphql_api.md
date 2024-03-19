# Developing the Sourcegraph GraphQL API

## Internal comments vs external documentation

Comments starting with `#!` in [schema.graphql](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/frontend/graphqlbackend/schema.graphql) will be stripped out before the GraphQL documentation is generated. `#!` is useful for implementation details and security notes that shouldn't be displayed to API users.

## GraphQL schema evolution

When changing the GraphQL schema, try to make changes that cause the API to ["grow"](https://youtu.be/oyLBGkS5ICk?t=19m49s), such as:

- Providing more: adding a new field to a return type, marking a field in a return type as non-nullable (e.g. `String!`)
- Requiring less: marking a parameter (or field in an input object) as nullable

Avoid "shrinking" the API with changes such as:

- Removing fields in a return type
- Marking fields in a return type as nullable
- Marking a parameter as non-nullable

## Changing the return type of a field

Because most queries do not mention return types directly, it's often possible to change the return type of a field as long as the new return type is a superclass of the old one. This breaks when clients use `__typename`, for example to determine the union variant of a value, so use caution when changing this.

As an example of changing a return type, imagine this query that deletes a user by ID:

```graphql
mutation {
  deleteUser(id: 1) {
    alwaysNil
  }
}
```

Let's say we want to return the `User` that was deleted by `deleteUser`, but it's currently returning `EmptyResponse`. We could change the return type to `User` and temporarily add the `alwaysNil` field to `User` while clients migrate:

```diff
 type Mutation {
-  deleteUser(id: ID!): EmptyResponse
+  deleteUser(id: ID!): User!
 }

 type EmptyResponse {
   alwaysNil: String
 }

 type User {
   ...
   id: ID!
+  # For compatibility with old clients only.
+  alwaysNil: String
 }
```

When the new type is not a superclass of the old one (e.g. `String` -> `User`), a breaking change is required.

## Making breaking changes

Try to avoid making breaking changes, but if you have to, be sure to give clients time to migrate. This usually involves splitting the change into 2 parts. For example, if you want to rename a field in the return type of a query:

- Add a new field with a different name, deprecate the old field, and communicate the deprecation in the changelog
- Give clients time to migrate (2 release cycles is a common guideline), remove the old deprecated field, and communicate the removal in the changelog

## Guides

- [How to add a GraphQL query](../how-to/add_graphql_query.md)
