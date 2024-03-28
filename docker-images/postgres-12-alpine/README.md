# Postgres-12 Alpine

This image provides a Postresql server for the _main_ database for Sourcegraph.

## Building and testing

This image comes in two flavours, the old alpine image, and the hardened one, built with Wolfi and Bazel.

### Alpine

As per [RFC 793]() Sourcegraph 5.1.0 is the last release we ship this variant to customers.

- Build: `./docker-images/postgres-12-alpine/build.sh`
- Test: N/A

### Hardened

Please note that migrating from the Alpine image, this require a manual step to reindex the database. See the 5.1 upgrade documentation for details.

- Build: `bazel build //docker-images/postgres-12-alpine:image_tarball`
- Test: `bazel build //docker-images/postgres-12-alpine:image_test`
Hello World
