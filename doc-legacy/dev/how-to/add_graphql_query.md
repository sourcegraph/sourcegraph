# How to add a GraphQL query

This guide documents how to add a new query to the GraphQL API. It explains what needs to be added to the Go code, as well as how to then use that query in the UI.

## GraphQL backend

Each GraphQL query usually retrieves data from a data store. In the case of Sourcegraph, this is usually Postgres. So there needs to be some mechanism in the backend that does this.

### Query function

The data query functions are split across multiple files depending on their function, which can be found in [cmd/frontend/graphqlbackend](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/cmd/frontend/graphqlbackend). To use an existing function as an example, in [cmd/frontend/graphqlbackend/feature_flags.go](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/feature_flags.go) you'll find a function named [`OrganizationFeatureFlagValue`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Ecmd/frontend/graphqlbackend/feature_flags%5C.go+OrganizationFeatureFlagValue&patternType=literal) that takes `OrgID` and `FlagName` as arguments and fetches the relevant data from the database.

The `schemaResolver` struct of each function is tied to is what allows GraphQL to link the GraphQL query to an actual operation. To create a new GraphQL query, create a function that's tied to the `schemaResolver` struct. It needs to be a public function, so in Go, it has to start with a capital letter. You can use an existing function as a guiding example (find functions by searching for [`schemaResolver`](https://sourcegraph.com/search?q=context:global+repo:github.com/sourcegraph/sourcegraph+schemaResolver&patternType=literal&case=yes)).

```go
func (r *schemaResolver) NewQuery(ctx context.Context, args *struct {
  SomeArg string
}) (bool, error) {
  // some code that fetches and returns data from the database
}
```

The GraphQL schema also needs to know about this new function and what it returns. An example GraphQL schema can be found in [`cmd/frontend/graphqlbackend/schema.graphql`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/graphqlbackend/schema.graphql), but this is [not the only GraphQL schema](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:.*%5C.graphql%24&patternType=literal) (schema files have the `.graphql` extension). To continue with the `OrganizationFeatureFlagValue` example, you'll find a entry for [`organizationFeatureFlagValue`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Ecmd/frontend/graphqlbackend/schema%5C.graphql+organizationFeatureFlagValue&patternType=literal) in the schema file. The name has to be the exact same as the Go function, except it starts with a lower case letter instead of a capital letter.

```graphql
newQuery(someArg: String!): Boolean!
```

### Resolvers hierarchy

GraphQL queries can be deeply nested. This means that a GraphQL request text specifies which fields of the schema should be returned. On the back-end this is reflected by a hierarchy of resolvers that stem from `schemaResolver`. Each method represents a nesting layer and returns the resolver for the nested structure.

```go
// cmd/frontend/graphqlbackend/graphqlbackend.go
func (r *schemaResolver) Repository(ctx context.Context, args *struct {
		Name     *string
		CloneURL *string
	}) (*RepositoryResolver, error) { ... }

// cmd/frontend/graphqlbackend/repository.go
func (r *RepositoryResolver) Commit(ctx context.Context, args *RepositoryCommitArgs) (*GitCommitResolver, error) { ... }

// cmd/frontend/graphqlbackend/git_commit.go
func (r *GitCommitResolver) Repository() *RepositoryResolver { return r.repoResolver }
```

The above hierarchy of resolvers can implement queries like the following:

```graphql
  {
    repository(name: "github.com/gorilla/mux") {
      commit(rev: "abc") {
        oid
      }
    }
  }
```

### Unions

Some GraphQL queries dispatch on the type of the nested entity:

```graphql
  {
    organizationFeatureFlagOverrides {
      namespace {
        id
      },
      targetFlag {
        ... on FeatureFlagBoolean {
          name
        },
        ... on FeatureFlagRollout {
          name
        }
      },
      value
    }
  }
```

In the case above `targetFlag` can be considered either as a `FeatureFlagBoolean` or a `FeatureFlagRollout`, and in both cases a `name` should be returned. This is the way this is implemented on the back end:

```go
// cmd/frontend/graphqlbackend/feature_flags.go
func (f *FeatureFlagOverrideResolver) TargetFlag(ctx context.Context) (*FeatureFlagResolver, error) { ... }

func (f *FeatureFlagResolver) ToFeatureFlagBoolean() (*FeatureFlagBooleanResolver, bool) { ... }
func (f *FeatureFlagResolver) ToFeatureFlagRollout() (*FeatureFlagRolloutResolver, bool) { ... }
```

Each type that composes a union has a corresponding method on the resolver with the prefix `To` that returns a resolver and a `bool` indicating whether an instance of the type is being returned.

The relevant bit of the GraphQL schema uses a `union`:

```graphql
"""
A feature flag is either a static boolean feature flag or a rollout feature flag
"""
union FeatureFlag = FeatureFlagBoolean | FeatureFlagRollout

"""
A feature flag that has a statically configured value
"""
type FeatureFlagBoolean { ... }

"""
A feature flag that is randomly evaluated to a boolean based on the rollout parameter
"""
type FeatureFlagRollout { ... }
```

## UI

The UI needs to know about the GraphQL query as well. `graphql-operations.ts` files are generated from strings tagged with `gql` in the TypeScript files. These `graphql-operations.ts` files (which are excluded from the repository) are where the TypeScript functions, argument values, and return types are defined.

Searching for [`organizationFeatureFlagValue`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Eclient/web/src/org/backend%5C.ts+organizationFeatureFlagValue&patternType=literal) will reveal a `gql` tagged string in [`client/web/src/org/backend.ts`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/client/web/src/org/backend.ts). You'll see the query is given a name, namely `OrgFeatureFlagValue` along with some parameters that are then used in the actual GraphQL function call. This is what the code generator uses to generate the TypeScript files.

So in order to add a new GraphQL function, simply add another `gql` tagged string with a similar structure. Don't forget that you can search for other [`gql` tagged strings](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5E*%5C.ts+gql%60&patternType=literal) if you need other references.

```ts
const USE_NEW_QUERY = gql`
    query UseNewQuery($someArg: String!) {
        newQuery(someArg: $someArg)
    }
`
```

`pnpm generate` or simply saving while your local instance of Sourcegraph is running will generate new `graphql-operations.ts` files with the appropriate functions and types defined.

You can now use this function in your TypeScript code. As an example of how to do this, you could perhaps look at [this](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Eclient/web/src/user/settings/codeHosts/UserAddCodeHostsPage%5C.tsx+GET_ORG_FEATURE_FLAG_VALUE&patternType=literal). Also, refer to [Working with GraphQL](../background-information/web/graphql.md)

## Links

- [Working with GraphQL](../background-information/web/graphql.md)
- [Developing the Sourcegraph GraphQL API](../background-information/graphql_api.md)
