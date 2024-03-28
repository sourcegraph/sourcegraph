# scip-syntax

<!--toc:start-->
- [scip-syntax](#scip-syntax)
  - [Usage](#usage)
    - [Indexing](#indexing)
    - [Evaluation](#evaluation)
  - [Development](#development)
    - [Running tests](#running-tests)
      - [Generating local reference SCIP files](#generating-local-reference-scip-files)
    - [Build the CLI for local testing](#build-the-cli-for-local-testing)
    - [Run the locally built CLI](#run-the-locally-built-cli)
<!--toc:end-->

A command line tool that uses other scip-* crates to either

- produce a SCIP file containing global and local symbols discovered during analysis.
- evaluate one SCIP file against another, producing precision/recall summary

## Usage

### Indexing

Index a list of files:

```bash
scip-syntax index --language java --out ./index.scip file1.java file2.java ...
```

Index a folder recursively:

```bash
scip-syntax index --language java --out ./index.scip --workspace <some-folder>
```
### Evaluation

```bash
scip-syntax evaluate --candidate index-tree-sitter.scip --ground-truth index.scip
```

## Development

This is a standard Rust CLI project, with a single runnable entry point - the CLI itself.

The CI uses Bazel for building and testing,
but Cargo usage is also supported for convenience.

### Running tests

#### Generating local reference SCIP files

To run the tests locally (in particular evaluation tests) without Bazel, you first need to produce the reference SCIP files
by using SCIP indexers.

If you run tests through Bazel, then you don't need to do anything extra - the
SCIP generation is wired as part of `integration_test` target.

The setup for tests is required to work with both Bazel and Cargo, and so here's how we do it:

1. The reference SCIP files are generated entirely by Bazel (see below)
2. Each testdata language folder contains a symlink named `index.scip` that
   points to a stable location where Bazel puts binary artifacts.

   e.g.

   ```
   testdata/java/index.scip -> ../../../../../../bazel-bin/docker-images/syntax-highlighter/crates/scip-syntax/index-java.scip
   ```
3. The code is written in such a way to fallback to a local `index.scip` file unless
   Bazel-specific environment variables are set up.

To generate reference SCIP files locally, it's recommended to run the `integration_test` target
once:

```
bazel test //docker-images/syntax-highlighter/crates/scip-syntax:integration_test
```

Or if you prefer, run individual tasks:

```
bazel build //docker-images/syntax-highlighter/crates/scip-syntax:java_groundtruth_scip
```

After that you can use `cargo test`, or continue to run tests through Bazel (which is what
CI does).


```bash
cargo test
```

```bash
bazel test //docker-images/syntax-highlighter/crates/scip-syntax:all
```

We use [Insta](https://insta.rs/) for snapshot testing.
If you're changing the output produced by the CLI,
run `cargo test` and then `cargo insta review`
to accept/reject changes in snapshots.

### Build the CLI for local testing

```bash
cargo build
```

```bash
bazel build //docker-images/syntax-highlighter/crates/scip-syntax
```

### Run the locally built CLI

```bash
cargo run -- index --language java --out ./index.scip file1.java file2.java ...
```

```bash
bazel run //docker-images/syntax-highlighter/crates/scip-syntax -- index --language java --out ./index.scip file1.java file2.java ...
```
Hello World
