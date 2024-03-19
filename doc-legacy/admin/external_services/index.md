# Using external services with Sourcegraph

Sourcegraph by default provides versions of services it needs to operate, including:

- A [PostgreSQL](https://www.postgresql.org/) instance for storing long-term information, such as user information when using Sourcegraph's built-in authentication provider instead of an external one.
- A second PostgreSQL instance for storing large-volume precise code graph data.
- A [Redis](https://redis.io/) instance for storing short-term information such as user sessions.
- A second Redis instance for storing cache data.
- A `sourcegraph/blobstore` instance that serves as a local S3-compatible object storage to hold user uploads before they can be processed. _This data is for temporary storage and content will be automatically deleted once processed._
- A [Jaeger](https://www.jaegertracing.io/) instance for end-to-end distributed tracing. 

Your Sourcegraph instance can be configured to use an external or managed version of these services:

- Using a managed version of PostgreSQL can make backups and recovery easier to manage and perform.
- Using a managed object storage service may decrease your hosting costs as persistent volumes are often more expensive than object storage space.

## External services guides

See the following guides to use an external or managed version of each service type.

- See [Using your own PostgreSQL server](./postgres.md) to replace the bundled PostgreSQL instances.
- See [Using your own Redis server](./redis.md) to replace the bundled Redis instances.
- See [Using a managed object storage service (S3 or GCS)](./object_storage.md) to replace the bundled blobstore instance.
- See [Using an external Jaeger instance](../observability/tracing.md#Use-an-external-Jaeger-instance) to replace the bundled Jaeger instance.

> NOTE: Using Sourcegraph with an external service is a [paid feature](https://sourcegraph.com/pricing). [Contact us](https://sourcegraph.com/contact/sales) to get a trial license.

## Cloud alternatives

- Amazon Web Services: [AWS RDS for PostgreSQL](https://aws.amazon.com/rds/), [Amazon ElastiCache](https://aws.amazon.com/elasticache/redis/), and [S3](https://aws.amazon.com/s3/) for storing user uploads.
- Google Cloud: [Cloud SQL for PostgreSQL](https://cloud.google.com/sql/docs/postgres/), [Cloud Memorystore](https://cloud.google.com/memorystore/), and [Cloud Storage](https://cloud.google.com/storage) for storing user uploads.
- Digital Ocean: [Digital Ocean Managed Databases](https://www.digitalocean.com/products/managed-databases/) for [Postgres](https://www.digitalocean.com/products/managed-databases-postgresql/), [Redis](https://www.digitalocean.com/products/managed-databases-redis/), and [Spaces](https://www.digitalocean.com/products/spaces/) for storing user uploads.
