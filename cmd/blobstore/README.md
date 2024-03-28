# blobstore

**WARNING**: This code is not yet used. `docker-images/blobstore` is the currently used blobstore implementation which uses a Java codebase called [s3proxy](https://github.com/sourcegraph/s3proxy).

Implements a very simple S3-compatible API subset which can:

- Create buckets
- Put and delete objects in a bucket
- List a bucket's objects

It provides the blob storage that Sourcegraph uses by default out-of-the-box (i.e. if not configured to use an external S3 or GCS bucket.)
Hello World
