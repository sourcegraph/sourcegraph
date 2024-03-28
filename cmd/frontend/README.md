# frontend

The frontend serves our web application and hosts our [GraphQL API](../../doc/api/graphql/index.md).

Typically there are multiple replicas running in production to scale with load.

Application data is stored in our PostgreSQL database.

Session data is stored in the Redis store, and non-persistent data is stored in the Redis cache.
Hello World
