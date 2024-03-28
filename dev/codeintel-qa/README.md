# Precise code intel tester

This package provides integration and load testing utilities for precise code intel services.

## Prerequisites

Ensure that the following tools are available on your path:

- [`src`](https://github.com/sourcegraph/src-cli)
- [`scip-go`](https://github.com/sourcegraph/scip-go)

You should have environment variables that authenticate you to the `sourcegraph-dev` GCS project if you plan to upload or download index files (as we do in CI).

Set:

```sh
SOURCEGRAPH_BASE_URL=http://localhost:3080
SOURCEGRAPH_SUDO_TOKEN=<YOUR SOURCEGRAPH API ACCESS TOKEN>
```

## Testing

1. Ensure these repositories exist on your instance (in `Site Admin` -> `Manage code hosts` -> `GitHub`):

```
  "repos": [
    "go-nacelle/config",
    "go-nacelle/log",
    "go-nacelle/nacelle",
    "go-nacelle/process",
    "go-nacelle/service",
    "sourcegraph-testing/nav-test",
  ],
```

2. Download the test indexes by running the following command:

```
go run ./cmd/download
```

Alternatively, generate them by running the following command (this takes much longer):

```
go run ./cmd/clone-and-index
```

If there is previous upload or index state on the target instance, they can be cleared by running the following command:

```
go run ./cmd/clear
```

Upload the indexes to the target instance by running the following command:

```
go run ./cmd/upload
```

Then run test queries against the target instance by running the following command:

```
go run ./cmd/query
```

## Refreshing indexes

If there is a change to an indexer that needs to be tested, the indexes can be regenerated and uploaded to gcloud for future test runs.

Generate indexes by running the following command:

```
go run ./cmd/clone-and-index
```

Upload the generated indexes to GCS by running the following command:

```
go run ./cmd/upload-gcs
```

Or if you just want to test an indexer change locally, you can:

```sh
rm -rf testdata/indexes/
```

Then run the `clone-and-index` step described above.
Hello World
