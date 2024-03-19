# Index a TypeScript or JavaScript repository

Instructions for creating an index for JavaScript and TypeScript projects and uploading it to Sourcegraph.

We will use [`scip-typescript`](https://github.com/sourcegraph/scip-typescript) to create the index and the [Sourcegraph CLI](https://github.com/sourcegraph/src-cli) to upload it to Sourcegraph.

## Indexing in CI using scip-typescript directly

In this approach, you will directly install `scip-typescript` and `src-cli` in CI. This is particularly useful if you are already using some other Docker image for your build.

Here is an example using GitHub Actions to create and upload an index for a TypeScript project.

```yaml
jobs:
  create-index-and-upload:
    # prevent forks of this repo from uploading lsif indexes
    if: github.repository == '<insert your repo name>'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Install dependencies
        run: npm install
      - name: Install scip-typescript
        run: npm install -g @sourcegraph/scip-typescript
      - name: Generate index
        uses: scip-typescript index
      - name: Install src-cli
        run: |
          curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
          chmod +x /usr/local/bin/src
      - name: Upload index
        run: src code-intel upload -github-token='${{ secrets.GITHUB_TOKEN }}' -no-progress
        env:
          SRC_ENDPOINT: https://sourcegraph.com/
```

> NOTE: `src-cli` ignores index upload failures by default to avoid disrupting CI pipelines with non-critical errors.

On CI providers other than GitHub Actions, you may need to explicitly install [Node.js](https://nodejs.org/) as a first step. See the [`scip-typescript` README](https://github.com/sourcegraph/scip-typescript) for the list of supported Node.js versions.

Examples:

- [lodash/lodash](https://github.com/sourcegraph-codeintel-showcase/lodash/blob/master/.github/workflows/lsif.yml) (JavaScript)

### Optional scip-typescript flags

The exact `scip-typescript` invocation will vary based on your configuration. For example:

- If you are indexing a JavaScript project instead of TypeScript, add the `--infer-tsconfig` flag.
  ```sh
  scip-typescript index --infer-tsconfig
  ```
- If you are indexing a project using Yarn workspaces, add the `--yarn-workspaces` flag.
  ```sh
  scip-typescript index --yarn-workspaces
  ```
- If you are indexing a project using Pnpm workspaces, add the `--pnpm-workspaces` flag.
  ```sh
  scip-typescript index --pnpm-workspaces
  ```

## Indexing in CI using the scip-typescript Docker image

Sourcegraph provides a Docker image for `sourcegraph/scip-typescript`, which bundles `src-cli` for convenience.

Here is an example using the `scip-typescript` Docker image with GitHub Actions to index a TypeScript project.

```yaml
jobs:
  create-and-upload-index:
    # prevent forks of this repo from uploading lsif indexes
    if: github.repository == '<insert your repo name>'
    runs-on: ubuntu-latest
    container: sourcegraph/scip-typescript:latest
    steps:
      - uses: actions/checkout@v3
      - name: Install dependencies
        run: npm install
      - name: Generate index
        run: scip-typescript index
      - name: Upload index
        run: src code-intel upload -github-token=${{ secrets.GITHUB_TOKEN }} -no-progress
        env:
          SRC_ENDPOINT: https://sourcegraph.com/
```

If you are indexing a JavaScript codebase or a project using Yarn workspaces, tweak the `scip-typescript` invocation as documented in the [Optional scip-typescript flags](#optional-scip-typescript-flags) section.

## One-off indexing using scip-typescript locally

Creating one-off indexes and uploading them is valuable as a proof of concept, but indexes are not kept up to date.

The steps here are similar to those in the previous GitHub Actions example.

1. Install `scip-typescript`.
   ```sh
   npm install -g @sourcegraph/scip-typescript
   ```
2. Install the Sourcegraph CLI.
   ```
   curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/local/bin/src
   chmod +x /usr/local/bin/src
   ```
   The exact invocation may change depending on the OS and architecture. See the [`src-cli` README](https://github.com/sourcegraph/src-cli#installation) for details.
3. `cd` into your project's root (which contains `package.json`/`tsconfig.json`) and run the following:
   ```sh
   # Enable (1) type-checking code used from external packages and (2) cross-repo navigation
   # by installing dependencies first with npm or yarn
   npm install
   scip-typescript index # for TypeScript projects
   ```
   If you are indexing a JavaScript codebase or a project using Yarn workspaces, tweak the `scip-typescript` invocation as documented in the [Optional scip-typescript flags](#optional-scip-typescript-flags) section.
4. Upload the data to a Sourcegraph instance.
   ```
   # for private instances
   SRC_ENDPOINT=<your sourcegraph endpoint> src code-intel upload
   # for public instances
   src code-intel upload -github-token=<your github token>
   ```
   The upload command will provide a URL you can visit to see the upload status. Once the upload has finished processing, you can visit the repo and enjoy precise code navigation!
