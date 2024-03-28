# Airgapped Migrator Generator

For customers operating airgapped instances, we want the migrator to contains all database schema descriptions available at the
time of the release, as they won't be able to fetch them from the outside.

`airgappedgen` simply fetches all of them up to the given version and output them in a designated folder, that can then be used
to bundle them in the OCI container of the airgapped migrator.

Unless you're working on this little command, you'll never need to run it manually, as it'll be called as part of the build process
for the airgapped migrator OCI container.

See the [FAQ](#FAQ) at the end of this document.

## Usage

All the instructions below applies to local development. If you're looking into running it manually:

### Running the tool itself

```
bazel run //cmd/migrator/airgappedgen -- <version> <output-folder>
```

### Inspecting the final tarball that gets injected in the OCI container for the airgapped migrator

If you want to inspect the tarball it creates to be added as a layer in the OCI container, you need a few additional things:

- We need to stamp the build and have the right status variables:
  - Set the `VERSION` env var to the version you want to pretend you're building the tarball for.
    - `export VERSION=v5.2.5` for example.
  - Add `--stamp` and `--workspace_status_command=dev/bazel_stamp_vars.sh` to the bazel command.
- Make sure you have a GitHub token set in `$GH_TOKEN`
  - The script is making a single request to the GitHub API, to list all tags in order to get the releases it needs to get the schemas
    from. If done unauthenticated, this API call might get throttled, which might take a very long time depending the quota
    associated to your current IP. It can go as long as 1h in some cases. The tool manually sets a short timeout, but the same
    reasoning applies.
- Append `--action_env=GH_TOKEN` to your Bazel command to make it visible to Bazel.

So if we compile all the above:

```
export GH_TOKEN=<yourtoken>
export VERSION=5.2.5
bazel build //cmd/migrator/airgappedgen:tar_schema_descriptions --stamp --workspace_status_command=dev/bazel_stamp_vars.sh --action_env=GH_TOKEN
INFO: Invocation ID: a59368d9-40a9-4da8-a4a6-f0554ab9397a
WARNING: Build option --action_env has changed, discarding analysis cache (this can be expensive, see https://bazel.build/advanced/performance/iteration-speed).
INFO: Analyzed target //cmd/migrator/airgappedgen:tar_schema_descriptions (346 packages loaded, 12864 targets configured).
INFO: From GoLink cmd/migrator/airgappedgen/airgappedgen_/airgappedgen [for tool]:
ld: warning: ignoring duplicate libraries: '-lm'
INFO: Found 1 target...
Target //cmd/migrator/airgappedgen:tar_schema_descriptions up-to-date:
  bazel-bin/cmd/migrator/airgappedgen/schema_descriptions.tar
Aspect @@rules_rust//rust/private:clippy.bzl%rust_clippy_aspect of //cmd/migrator/airgappedgen:tar_schema_descriptions up-to-date (nothing to build)
INFO: Elapsed time: 27.318s, Critical Path: 25.26s
INFO: 4 processes: 1 internal, 3 darwin-sandbox.
INFO: Build completed successfully, 4 total actions
```

And we can inspect the tarball with:

```
# We got the path the tarball from the command above, alternatively, we could use a cquery.
tar tvf bazel-bin/cmd/migrator/airgappedgen/schema_descriptions.tar
drwxr-xr-x  0 0      0           0 Jan 19 15:46 schema-descriptions/
-rw-r--r--  0 0      0       21371 Jan 19 15:46 schema-descriptions/v3.29.0-internal_database_schema.codeinsights.json
-rw-r--r--  0 0      0       41304 Jan 19 15:46 schema-descriptions/v3.30.2-internal_database_schema.codeintel.json
-rw-r--r--  0 0      0      396651 Jan 19 15:46 schema-descriptions/v3.30.0-internal_database_schema.json
# (...)
```

If you're building the tarball without stamping with the `VERSION` env var, it will still work, but will produce a tarball
that only contains a README to indicate that this is dev version. If you ever stumble across this in a production deployment,
it means that something wrong happened.

```
# Build the tarball
bazel build //cmd/migrator/airgappedgen:tar_schema_descriptions
```

```
# Grab the tarball output and shows what's inside.
# (we throw away stderr for clarity, so we just see the content of the tarball and not the logs
# from building the tarball).
$ tar tvf $(bazel cquery //cmd/migrator/airgappedgen:tar_schema_descriptions --output=files 2>/dev/null)
drwxr-xr-x  0 0      0           0 Jan 19 14:32 schema-descriptions/
-rw-r--r--  0 0      0         109 Jan 19 14:32 schema-descriptions/README.md
```

# FAQ

## Why do we need this?

Airgapped customers are running migrations in a fully isolated environment, and they can't reach the internet for security reasons,
meaning that we need to provide a migrator variant that comes with everything baked in.

## Why are we fetching database schemas prior to `v3.42.0` on GCS?

For versions prior to `v3.42.0`, the repository didn't have the `*schema.json` files committed, so they're stored in a GCS bucket
instead. See `gcs_versions.json` in this folder for the file that manually lists them.

## Why do we have to list the GCS versions in `gcs_versions.json`?

We could have use `gsutil` and simply download the entire folder content in one go instead of having to specify the versions.
But having to deal with the authentication on gcloud here would have been a bit more complicated where all we're doing is a bunch of
HTTP GET requests.

## Why don't we fail the build if unstamped, instead of silently creating an invalid tarball?

This would prevent builds in CI that are not release builds to succeed, which would be really annoying to deal with.

TODO: add a release test to ensure the airgapped migrator ships with the schemas: https://github.com/sourcegraph/sourcegraph/issues/59721

## Why not write a dumb shell script for this?

We need to list all the tags available for the `sourcegraph/sourcegraph` repository, which can be done with a `curl` but that's a lot
of fragile parsing and scripting that may fail in unexpected ways. Given the stakes of this build step, better have something robust.

## Why not get the schemas through `git` commands?

Bazel actions are unaware of the Git repo they're executed in, that's the price to pay for hermeticity. If we would really want
to use git commands, we would have to clone the repo during build time, which is really slow due to the size of our monorepo.

So instead, we handle everything through HTTP Get requests and one single authenticated call to the GitHub API.

## Are the tarballs stable, i.e. idempotent?

Yes, as long as the content didn't change in the GCS bucket and nobody re-tagged a previous release, which should never happen, the tarballs are stable, that's why you see the timestamps set at unix time (epoch) 0.

## The tarball target has no inputs, i.e. `srcs = []`, how do we know when to rebuild it?

The `genrule` that creates the tarball has the `stamp = 1` attribute, so Bazel will inspect the stable status variables and will
rebuild this if it changes. And the `VERSION` is a stable attribute, so it will get rebuilt every time it changes, .i.e. on each release.
Hello World
