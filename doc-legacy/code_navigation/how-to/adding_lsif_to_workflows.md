# Adding precise code navigation to CI/CD workflows

## [Language-specific guides](index.md)

We are working on creating language specific guides for use with different indexers, so make sure to check for the documentation for your language! If there isn't a guide for your language, this general guide will help you through the precise code navigation setup process.

> NOTE: First make sure to complete the [how-to guides on indexing](../how-to/index.md).

## Benefits of CI integration

Setting up a source code indexing job in your CI build provides you with fast code navigation that gives you more control on when source code gets indexed, and ensures accuracy of your code navigation by keeping in sync with changes in your repository. Due to the large number of CI frameworks and languages we may not have specific documentation for your use case. Feel free to [file an issue](https://github.com/sourcegraph/sourcegraph/issues/new) if you're having difficulties and we can help troubleshoot your setup.

## Index

- [Using indexer containers](#using-indexer-containers)
- [CI from scratch](#ci-from-scratch)
  - [GitHub Action Examples](#github-action-examples)
  - [Circle CI Examples](#circle-ci-examples)
  - [Travis CI Examples](#travis-ci-examples)
- [Recommended upload frequency](#recommended-upload-frequency)
- [Uploading indexes to Sourcegraph.com](#uploading-indexes-to-sourcegraph-com)

## Using indexer containers

We're working on creating containers that bundle indexers and the [Sourcegraph CLI (`src`)](https://github.com/sourcegraph/src-cli). All the languages in our [language-specific guides](index.md) are supported, and this section provides a general guide to using those containers. We've pinned the containers to the `latest` tag in these samples so they stay up to date, but make sure you pin them before going live to ensure reproducibility.

### Basic usage

Some indexers will work out of the box by just running them in your project root. Here's an example using GitHub actions and Go:

```yaml
jobs:
  lsif-go:
    # this line will prevent forks of this repo from uploading lsif indexes
    if: github.repository == '<insert your repo name>'
    runs-on: ubuntu-latest
    container: sourcegraph/lsif-go:latest
    steps:
      - uses: actions/checkout@v1
      - name: Generate index
        run: lsif-go
      - name: Upload index
        # this will upload to Sourcegraph.com, you may need to substitute a different command
        # by default, we ignore failures to avoid disrupting CI pipelines with non-critical errors.
        run: src code-intel upload -github-token=${{ secrets.GITHUB_TOKEN }} -ignore-upload-failure
```

### Sub-projects

If your repository contains multiple code projects in different folders, your CI framework likely provides a "working directory" option so that you can do something like:

```yaml
jobs:
  lsif-go:
    # this line will prevent forks of this repo from uploading lsif indexes
    if: github.repository == '<insert your repo name>'
    runs-on: ubuntu-latest
    container: sourcegraph/lsif-go:latest
    steps:
      - uses: actions/checkout@v1
      - name: Generate index
        working-directory: backend/
        run: lsif-go
      - name: Upload index
        # note that the upload command also needs to happen in the same directory!
        working-directory: backend/
        # this will upload to Sourcegraph.com, you may need to substitute a different command
        # by default, we ignore failures to avoid disrupting CI pipelines with non-critical errors.
        run: src code-intel upload -github-token=${{ secrets.GITHUB_TOKEN }} -ignore-upload-failure
```

### Custom build environments

Depending on which language you're using, you may need to perform some pre-indexing steps in a custom build environment. You have a couple options for this:

- Add the indexer and [Sourcegraph CLI (`src`)](https://github.com/sourcegraph/src-cli) to your environment. To make this easier, you could either directly build a Docker image from our indexer container, or use its Dockerfile as inspiration for getting the tools installed in your environment.
- Do the pre-indexing steps in your environment, and then make those artifacts available to our container.

This second step is easy in GitHub actions because our container can be used as an action. Here's an example for a TypeScript project:

```yaml
jobs:
  scip-typescript:
    # this line will prevent forks of this repo from uploading lsif indexes
    if: github.repository == '<insert your repo name>'
    runs-on: ubuntu-latest
    container: my-awesome-container
    steps:
      - uses: actions/checkout@v1
      - name: Install dependencies
        run: <install dependencies>
      - name: Generate index
        uses: docker://sourcegraph/scip-typescript:latest
        with:
          args: scip-typescript index
      - name: Upload index
        uses: docker://sourcegraph/src-cli:latest
        with:
          # this will upload to Sourcegraph.com, you may need to substitute a different command
          # by default, we ignore failures to avoid disrupting CI pipelines with non-critical errors.
          args: lsif upload -github-token=${{ secrets.GITHUB_TOKEN }} -ignore-upload-failure
```


### GitHub Action Examples
The following projects have example GitHub Action workflows to generate and upload indexes.

- [elastic/kibana](https://github.com/sourcegraph-codeintel-showcase/kibana/blob/7ed559df0e2036487ae6d606e9ffa29d90d49e38/.github/workflows/lsif.yml)
- [golang/go](https://github.com/sourcegraph-codeintel-showcase/go/blob/f40606b1241b0ca4802d7b00a763241b03404eea/.github/workflows/lsif.yml)
- [kubernetes/kubernetes](https://github.com/sourcegraph-codeintel-showcase/kubernetes/blob/359b6469d85cc7cd4f6634e50651633eefeaea4e/.github/workflows/lsif.yml)
- [lodash/lodash](https://github.com/sourcegraph-codeintel-showcase/lodash/blob/b90ea221bd1b1e036f2dfcd199a2327883f9451f/.github/workflows/lsif.yml)
- [moby/moby](https://github.com/sourcegraph-codeintel-showcase/moby/blob/380429abb05846de773d5aa07de052f40c9e8208/.github/workflows/lsif.yml)
- [ReactiveX/IxJS](https://github.com/sourcegraph-codeintel-showcase/IxJS/blob/e53d323314043afb016b6deceaeb068d8d23c303/.github/workflows/lsif.yml)


### Circle CI Examples
Certain frameworks like Circle CI may require you to explicitly cache artifacts between jobs. In CircleCI this might look like the following:

```yaml
version: 2.1

jobs:
  install-deps:
    docker:
      - image: my-awesome-container
    steps:
      - checkout
      - <install dependencies>
      - save_cache:
          paths:
            - node_modules
          key: dependencies

jobs:
  scip-typescript:
    docker:
      - image: sourcegraph/scip-typescript:latest
    steps:
      - checkout
      - restore_cache:
          keys:
            - dependencies
      - run: scip-typescript index
        # this will upload to Sourcegraph.com, you may need to substitute a different command
        # by default, we ignore failures to avoid disrupting CI pipelines with non-critical errors.
      - run: src code-intel upload -github-token=<<parameters.github-token>> -ignore-upload-failure

workflows:
  scip-typescript:
    jobs:
      - install-deps
      - scip-typescript:
          requires:
            - install-deps
```

The following projects have example CircleCI configurations to generate and upload indexes.

- [angular/angular](https://github.com/sourcegraph-codeintel-showcase/angular/blob/f06eec98cadab2ff7a1cef2a03ba7c42015eb399/.circleci/config.yml)
- [facebook/jest](https://github.com/sourcegraph-codeintel-showcase/jest/blob/b781fa2b6683f04324edbc4b41552a94f97cd479/.circleci/config.yml)
- [facebook/react](https://github.com/sourcegraph-codeintel-showcase/react/blob/e488420f686b88803cfb1bb09bbc4d3991db8c55/.circleci/config.yml)
- [grafana](https://github.com/sourcegraph-codeintel-showcase/grafana/blob/664a694955ea40575a1cffe9db47a7adf4d3c2bb/.circleci/config.yml)
- [helm](https://github.com/sourcegraph-codeintel-showcase/helm/blob/62c38f152d0802719aad1ec4c1c281f01dc75173/.circleci/config.yml)
- [prometheus/prometheus](https://github.com/sourcegraph-codeintel-showcase/prometheus/blob/a0a8a249fff9d1c6ce4c097ccc4f5e120c723c51/.circleci/config.yml)
- [ReactiveX/rxjs](https://github.com/sourcegraph-codeintel-showcase/rxjs/blob/c9d3c1a76a68273863fc59075a71b4cc43c06114/.circleci/config.yml)

### Travis CI Examples
The following projects have example Travis CI configurations to generate and upload indexes.

- [aws/aws-sdk-go](https://github.com/sourcegraph-codeintel-showcase/aws-sdk-go/blob/92f67a061fcdd46d6a418b28838b10b6ac63a880/.travis.yml)
- [etcd-io/etcd](https://github.com/sourcegraph-codeintel-showcase/etcd/blob/eae726706fe8ebf7e08b45ba29a70388595db31b/.travis.yml)
- [expressjs/express](https://github.com/sourcegraph-codeintel-showcase/express/blob/bd1ae153f19656183257ed223d518aeb9f5091ec/.travis.yml)
- [hugo/hugo](https://github.com/sourcegraph-codeintel-showcase/hugo/blob/6704b7c125d7b21ccf2048d7bff0f1ffe2b0867d/.travis.yml)
- [Microsoft/TypeScript](https://github.com/sourcegraph-codeintel-showcase/TypeScript/blob/f37f1dee1b3e63b12df2935590c8707a5ec3993b/.travis.yml)
- [moment/moment](https://github.com/sourcegraph-codeintel-showcase/moment/blob/eedccdc2c07fb5abe931b427d50f5b3c3f44ac95/.travis.yml)
- [sindresorhus/got](https://github.com/sourcegraph-codeintel-showcase/got/blob/164d55a029512cea7f245de870cbb1eaba114734/.travis.yml)

## CI from scratch

If you're indexing a language we haven't documented yet in our [language-specific guides](./index.md#language-specific-guides), follow the instructions in this section. We want to have containers available for every language with a robust indexer, so please also consider [filing an issue](https://github.com/sourcegraph/sourcegraph/issues/new) to let us know we're missing one.

### Set up your CI machines

Your CI machines will need two command-line tools installed. Depending on your build system setup, you can do this as part of the CI step, or you can add it directly to your CI machines for use by the build.

1. The [Sourcegraph CLI (`src`)](https://github.com/sourcegraph/src-cli).
1. The [indexer](../references/indexers.md) for your language.

### Add steps to your CI

1. **Generate an index** for a project within your repository by running the indexer in the project directory (consult your indexer's docs).
1. **[Upload that generated index](../how-to/index.md)** to your Sourcegraph instance.

## Recommended upload frequency

We recommend that you start with a CI job that runs on every commit (including branches).

If you see too much load on your CI, your Sourcegraph instance, or a rapid decrease in free disk space on your Sourcegraph instance, you can instead index only the default branch, or set up a periodic job (e.g. daily) in CI that indexes your default branch.

With periodic jobs, you should still receive precise code navigation on non-indexed commits on lines that are unchanged since the nearest indexed commit. This requires that the indexed commit be a direct ancestor or descendant no more than [100 commits](https://github.com/sourcegraph/sourcegraph/blob/e7803474dbac8021e93ae2af930269045aece079/lsif/src/shared/constants.ts#L25) away. If your commit frequency is too high and your index frequency is too low, you may find commits with no precise code navigation at all. In this case, we recommend you try to increase your index frequency if possible.

## Uploading indexes to Sourcegraph.com

Indexes can be uploaded to a self-hosted Sourcegraph instance or to [Sourcegraph.com](https://sourcegraph.com). Using the [Sourcegraph.com](https://sourcegraph.com) endpoint will surface code navigation for your public repositories directly on GitHub via the [Sourcegraph browser extension](https://docs.sourcegraph.com/integration/browser_extension) and at `https://sourcegraph.com/github.com/<your-username>/<your-repo>`.

Using the [Sourcegraph.com](https://sourcegraph.com) endpoint is free and your index is treated as User-Generated Content (you own it, as covered in our [Sourcegraph.com terms of service](https://sourcegraph.com/terms-dotcom#3-proprietary-rights-and-licenses)). If you run into trouble, or a situation arises where you need all of your index expunged, please reach out to us at [support@sourcegraph.com](mailto:support@sourcegraph.com).
