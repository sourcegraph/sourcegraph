# msp-example

This service is an example service testing and demonstrating how to build Managed Services Platform services: [go/msp](https://handbook.sourcegraph.com/departments/engineering/teams/core-services/managed-services/platform/)

## Pushing a new version

```sh
bazel run //cmd/msp-example:candidate_push --stamp -- --tag insiders --repository us.gcr.io/sourcegraph-dev/msp-example
```
Hello World
