# Dataloaders

Dataloaders are a pattern for efficiently batching and caching data fetches in GraphQL resolvers. They help avoid the N+1 problem where a GraphQL query results in N + 1 database queries.

## Why dataloaders?

- Improve GraphQL query performance by batching loads and caching data
- Avoid N+1 queries and overfetching
- Manage caching and batching logic in a simple, reusable way

## Usage

1. Import the dataloader package:

```go
import "github.com/sourcegraph/sourcegraph/internal/dataloaders"
```

1. Crete a dataloader for a specific data type:

```go
const userLoader := dataloader.NewUserLoader(db)
```

1. Use the dataloader in resolvers:

```go
func (r *queryResolver) GetUser(ctx context.Context, id int) (*User, error) {
  // get the loader from context
  return userLoader.Load(ctx, id)
}
```

The user loader will batch and cache loads behind the scenes.

#### Contributing

Add new dataloader instances to the loader package to avoid duplication across resolvers.
