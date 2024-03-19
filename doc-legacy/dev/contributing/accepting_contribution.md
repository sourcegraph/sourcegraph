# How to accept an external contribution

This page outlines how to accept a contribution to the [Sourcegraph repository](https://github.com/sourcegraph/sourcegraph).

## CLA-bot

1. If the `cla-bot` check fails, ensure that that contributor signed the [CLA](https://docs.google.com/spreadsheets/d/1_iBZh9PJi-05vTnlQ3GVeeRe8H3Wq1_FZ49aYrsHGLQ/edit?usp=sharing) by filling out the [form](https://forms.gle/YnmetmopXNxFxsDUA). You may have to wait up to 30 minutes for the form response to be synchronized to the [contributors list](https://github.com/sourcegraph/clabot-config).
   1. A sync can also be manually triggered from [the `sync` workflow](https://github.com/sourcegraph/clabot-config/actions/workflows/sync.yml).
2. Comment on the pull request: `@cla-bot check`.
3. The `verification/cla-signed` workflow should become green. 🎉

## Buildkite

To request a [Buildkite build](../background-information/ci/index.md#buildkite-pipelines) for a pull request from a fork, a build must be manually requested after reviewing the contributor's changes. A successful Buildkite build is required for a pull request to be merged.

> WARNING: Builds do not happen automatically for forks for security reasons—Buildkite build runs have access to a variety of secrets used in testing. When reviewing, ensure that there are no unexpected usages of secrets or attempts to expose secrets in logs or external services.

### Request a build directly

Once changes have been reviewed, a build can be requested directly for a commit with [the `sg` CLI](../background-information/sg/index.md):

```sh
sg ci build --commit $COMMIT
```

### Check out and request a build

To check out a pull request's code locally, use [the `gh` CLI](https://cli.github.com/):

```sh
gh pr checkout $NUMBER
```

Alternatively, it is also possible to check out the branch without having to re-clone the forked repo by running the following—make sure that the created branch name exactly matches their branch name, otherwise Buildkite will not match the created build to their branch:

```sh
git fetch git@github.com:$THEIR_USERNAME/sourcegraph $THEIR_BRANCH:$THEIR_BRANCH
```

Then, use [the `sg` CLI](../background-information/sg/index.md) to request a build after reviewing the code:

```sh
sg ci build
```
