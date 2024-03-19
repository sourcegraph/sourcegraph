# How to cache CI artefacts

This guide documents how to cache build artefacts in order to speed up build times in Sourcegraph's [Buildkite CI pipelines](../background-information/ci/index.md#buildkite-pipelines).

> NOTE: Before getting started, we recommend familiarize yourself with [Pipeline development](../background-information/ci/development.md) and [Buildkite infrastructure](../background-information/ci/development.md#buildkite-infrastructure).

## The need for caching

Because [Buildkite agents are stateless](../background-information/ci/development.md#buildkite-infrastructure) and start with a blank slate, this means that all dependencies and binaries have to rebuild on each job. This is the price to pay for complete isolation from one job to the other.

A common strategy to address this problem of having to rebuild everything is to store objects that are commonly reused accross jobs and to download them again rather than rebuilding everything from scratch.

## What and when to cache?

In order to determine what we can cache and when to do it, we need to make sure to cover the following questions:

1. Will it be faster to download a cached version rather than building it?
1. Can we come up with an identifier that represents accurately what version of the cache is needed, before building anything?
   - Ex: the checksum of `pnpm-lock.yaml` accurately tells us which version of the dependencies cache needs to be downloaded if it exists.
1. Is what is being cached, not the end result of that test?
   - If it were the case, it means we would be caching the result of the test, which is defeating the purpose of a continuous integration process.

## How to write a step that caches an artefact?

In the [CI pipeline generator](../background-information/ci/development.md), when defining a step you can use the `buildkite.Cache()` function to define what needs to be cached and under which key to store it.

For example: we want to cache the `node_modules` folder to avoid dowloading again all dependencies for the front-end.

```go
pipeline.AddStep("...",
  bk.Cache(&buildkite.CacheOptions{
    ID:          "node_modules_pnpm",
    Key:         "cache-node_modules-pnpm-{{ checksum 'pnpm-lock.yaml' }}",
    RestoreKeys: []string{"cache-node_modules-pnpm-{{ checksum 'pnpm-lock.yaml' }}"},
    Paths:       []string{"node_modules", ".pnpm/cache"},
  }),
  bk.Cmd("..."),
```

The important part here are:

- The `Key` which defines how the cached version is named, so we can find it again on ulterior builds.
  - It includes a `{{ checksum 'pnpm-lock.yaml' }}` segment in its name, which means that any changes in the `pnpm-lock.yaml` will be reflected in the key name.
  - It means that if the `pnpm-lock.yaml` checksum would change because dependencies have changed, it should use a different version of the cached dependencies. If those were to not be present on the cache, it will simply rebuild them and upload the result to the cache.
- The `RestoreKeys` lists the keys we can use to know if there is a cached version or not available. 99% of the time, that's the same exact thing as the `Key`.
- The `Paths` lists the path to the files that needs to be cached. They **must be within the repository**, not outside.

Please note that from the perspective of the commands ran in that step, it's not possible to know if the cache was hit or missed. So the code using what has been cached must be able to understand it on its own (example, checking for the presence of folders defined in the `Paths` keys). Most build systems will handle that on their own, but if you were to cache test data you'll probably have to handle that yourself.

## How to make sure that caching is faster?

When a build is finished and successful, you will find an annotation with a link to the trace of that build on HoneyComb. You can compare the timings of your build before and after adding the cache (with a cache hit of course, otherwise you would compare no cache with a cache miss, which will often be longer because there is time spent uploading the cache result).

## How to purge the cache from a given key?

If you accidently cached incorrect results, the simplest way to purge it is to use `gsutil`:

```
# List everything
gsutil ls -al gs://sourcegraph_buildkite_cache/sourcegraph/sourcegraph/
# Purge the key you want to delete
gsutil rm gs://sourcegraph_buildkite_cache/sourcegraph/sourcegraph/[MY-KEY].tar.gz
```

## When is the cache purged?

Cached artefacts expire automatically after 30 days, as mandated by an object lifecycle policy on the bucket.

## How to enable caching for a new Buildkite pipeline?

> NOTE: These instructions assume that the new pipeline is defined in a `pipeline.yaml` file directly instead of being generated.

While the [Cache Buildkite Plugin](https://github.com/sourcegraph/cache-buildkite-plugin) takes care of the caching itself, new pipelines require some bootstrapping to ensure required dependencies are installed and configured.

1. Add `.buildkite/hooks/pre-command` to the root of your repository if it does not exist yet. This is a [Buildkite lifecycle hook](https://buildkite.com/docs/agent/v3/hooks#job-lifecycle-hooks) that runs before every build command. Then add the following snippet to this file:
    ```
    #!/usr/bin/env bash

    set -e

    # awscli is needed for Cache Buildkite Plugin
    asdf install awscli

    # set the buildkite cache access keys
    AWS_CONFIG_DIR_PATH="/buildkite/.aws"
    mkdir -p "$AWS_CONFIG_DIR_PATH"
    AWS_CONFIG_FILE="$AWS_CONFIG_DIR_PATH/config"
    export AWS_CONFIG_FILE
    AWS_SHARED_CREDENTIALS_FILE="$AWS_CONFIG_DIR_PATH/credentials"
    export AWS_SHARED_CREDENTIALS_FILE
    aws configure set aws_access_key_id "$BUILDKITE_HMAC_KEY" --profile buildkite
    aws configure set aws_secret_access_key "$BUILDKITE_HMAC_SECRET" --profile buildkite
    ```

   > NOTE: for `asdf install awscli` to succeed, a `.tool-versions` file containing the dependency and the desired version number must be present. You may replace `asdf` with any other installation method.

   #

   > NOTE: the environment variables `$BUILDKITE_HMAC_KEY` and `$BUILDKITE_HMAC_SECRET` are set on the Buildkite agent already.

1. Add the following snippet to the top of `pipeline.yaml`:
    ```
    s3-settings: &s3-settings
      backend: s3
      s3:
        bucket: sourcegraph_buildkite_cache
        endpoint: https://storage.googleapis.com
        profile: buildkite
        region: us-central1
    ```


1. For each build step where you would like to cache artifacts, define what to cache and how it should invalidate. Decide what the key should be as described in [What and when to cache?](#what-and-when-to-cache). Generally, consider these cache types:

  * **Long-lived** caches. An example is a project's dependencies, which do not change frequently and may consume a significant portion of the build time when pulled from repositories. In this case, using the checksum of the dependency list (such as `go.mod` or `requirements.txt`) as a cache key will cause the cache to be recreated whenever the dependencies are updated. You may also hash [a directory instead of a file](https://github.com/sourcegraph/cache-buildkite-plugin#hashing-checksum-against-directory). **It is important to set the `paths` to the dependency directory to ensure only those files are cached**. This snippet shows an example configuration:
      ```
      plugins:
        - https://github.com/sourcegraph/cache-buildkite-plugin.git#master:
          id: <fitting ID> # e.g. go-mod
          key: "{{ id }}-{{ git.branch }}-{{ checksum /path/to/dependency-file }}"
          restore-keys:
            - "{{ id }}-{{ git.branch }}-{{ checksum /path/to/dependency-file }}"
            - "{{ id }}-{{ git.branch }}-"
            - "{{ id }}-"
          compress: true
          compress-program: pigz
          paths:
            - "/path/to/dependencies"
          <<: *s3-settings
      ```

  * **Short-lived** caches. These contain artifacts of a project that are frequently changed, such as application code. These caches can be useful for e.g. rerunning a build on a network timeout. A cache key should be used that is unique across multiple builds. A good default is the build's git commit SHA. The following snippet demonstrates how to do this:
      ```
      plugins:
         - https://github.com/sourcegraph/cache-buildkite-plugin.git#master:
           id: <fitting ID> # e.g. project name
           key: "{{ id }}-{{ git.branch }}-{{ git.commit }}"
           restore-keys:
             - "{{ id }}-{{ git.branch }}-{{ git.commit }}"
             - "{{ id }}-{{ git.branch }}-"
             - "{{ id }}-"
           compress: true
           compress-program: pigz
           <<: *s3-settings
      ```

   Add every plugin definition to the relevant steps in your `pipeline.yaml`. An example of a valid pipeline step with a plugin configured can be found [here](https://sourcegraph.com/github.com/sourcegraph/image-updater-pipeline/-/blob/.buildkite/image-updater/pipeline.yaml?L25).
