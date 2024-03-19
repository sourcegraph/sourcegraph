# Sourcegraph 4.2.1+ blobstore update notes

In Sourcegraph versions 4.2.1+ and 4.3+, `minio` object storage has been replaced with `sourcegraph/blobstore`. **No migration is required for most deployment types.**

## About the change

Sourcegraph is committed to only distributing open-source software that is permitted by other industry leaders, and prohibits the use of all software licenses [not allowed at Google](https://opensource.google/documentation/reference/thirdparty/licenses#banned).

As a result, Sourcegraph v4.2.1+ and v4.3+ no longer use or distribute _any_ minio or AGPL-licensed software.

If your Sourcegraph instance is already configured to use [external object storage](../external_services/object_storage.md), or you use `DISABLE_MINIO=true` in `sourcegraph/server` deployments, then this change should not affect you (there would already be no Minio software running as part of Sourcegraph). It is also safe to remove any minio specific env variables from your deployment.

**If you are on running a proxy with the `NO_PROXY` env variable, you'll need to make sure that minio is removed from this list, and blobstore is added.**

If you have any questions or concerns, please reach out to us via support@sourcegraph.com and we'd be happy to help.

## If you use Kubernetes + Helm

To update a Helm deployment, simply replace `minio` with `blobstore` in your Helm override YAML file (if present). For example:

```diff
-minio:
+blobstore:
  enabled: false # Disable deployment of the built-in object storage
```

## If you use `sourcegraph/server`

We are aware of an issue in `sourcegraph/server` which will be fixed in v4.3 (scheduled to be released Dec 15th.)

You may initially encounter an error like:

```
failed to create bucket: operation error S3: CreateBucket, retry quota exceeded, 0 available, 5 requested
```

If you do encounter this, you may try simply running the container again (the 2nd time the file permissions are usually fixed.)

To avoid this error during upgrade, please manually correct the permissions of the data directory. For example if you use the standard `--volume` commands like `--volume ~/.sourcegraph/config:/etc/sourcegraph`, then you may run:

```
mkdir -p ~/.sourcegraph/data/blobstore && sudo chown -R 100:101 ~/.sourcegraph/data/blobstore
```

After this the container should start without error.
