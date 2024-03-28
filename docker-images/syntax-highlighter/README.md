# Syntax Highlighter (And Associated Crates)

Crates:

- The main `syntect_server` executable
- `crates/tree-sitter-all-languages/`: All the grammars and parsers live here to make shipping parsers with the same tree-sitter version (and associated build tooling) very easy. All new grammars and parsers should be added here.
- `crates/scip-syntax/`: Command line tool that produces a SCIP index file from the analysis performed by other `scip-*` crates in this project
- `crates/syntax-analysis/`: local navigation calculation (and some other utilities) live here.

# scip-ctags

See [queries](./docs/queries.md)

# Syntect Server

This is an HTTP server that exposes the Rust [Syntect](https://github.com/trishume/syntect) syntax highlighting library for use by other services. Send it some code, and it'll send you syntax-highlighted code in response. This service is horizontally scalable, but please give [#21942](https://github.com/sourcegraph/sourcegraph/issues/21942) and [#32359](https://github.com/sourcegraph/sourcegraph/pull/32359#issuecomment-1063310638) a read before scaling it up.

### Cargo Usage

```bash
cargo run --bin syntect_server
```

You can set the `SRC_SYNTECT_SERVER` environment var to whatever port this
connects to and test against local Sourcegraph instance.

### Docker Usage (can be used with `sg start`)

```bash
docker run --detach --name=syntax-highlighter -p 9238:9238 sourcegraph/syntax-highlighter
```

You can then e.g. `GET` http://localhost:9238/health or http://host.docker.internal:9238/health to confirm it is working.

## API

See [API](./docs/api.md)

## Configuration

By default on startup, `syntect_server` will list all file types it supports. This can be disabled by setting `QUIET=true` in the environment.

## Development

1. Use `cargo test --workspace` to run all the tests.
   To update snapshots, run `cargo insta review`.
2. Use `cargo run --bin syntect_server` to run the server locally.
3. You can change the `SRC_SYNTECT_SERVER` option in your `sg.config.yaml` to point to whatever port you're running on (usually 8000) and test against that without building the docker image.

### Testing syntect -> SCIP grammar mappings

<!-- NOTE(id: only-flag) -->

You can run a subset of tests for `syntect_scip` using the `ONLY` environment variable.
Example:

```bash
cd crates/syntax-analysis
ONLY=.java cargo test test_all_files -- --nocapture
```

## Building docker image

`./build.sh` will build your current repository checkout into a final Docker image. You **DO NOT** need to do this when you push to get it publish. But, you should do this to make sure that it is possible to build the image :smile:.

**AGAIN NOTE**: The docker image will be published automatically via CI.

## Updating Sourcegraph

Once published, the image version will need to be updated in the following locations to make Sourcegraph use it:

- [`sourcegraph/sourcegraph > cmd/server/Dockerfile`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/server/Dockerfile?subtree=true#L54:13)
- [`sourcegraph/sourcegraph > sg.config.yaml`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/sg.config.yaml?subtree=true#L206:7)

Additionally, it's worth doing a [search](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+sourcegraph/syntect_server:&patternType=literal) for other uses in case this list is stale.

## Adding languages (tree-sitter):

See [tree-sitter-all-languages](./crates/tree-sitter-all-languages/README.md)

## Adding languages (syntect -- outdated):

#### 1) Find an open-source `.tmLanguage` or `.sublime-syntax` file and send a PR to our package registry

https://github.com/sourcegraph/Packages is the package registry we use which holds all of the syntax definitions we use in syntect_server and Sourcegraph. Send a PR there by following [these steps](https://github.com/sourcegraph/Packages/blob/master/README.md#adding-a-new-language)

#### 2) Update our temporary fork of `syntect`

We use a temporary fork of `syntect` as a hack to get our `Packages` registry into the binary. Update it by creating a PR with two commits like:

- https://github.com/slimsag/syntect/commit/9976d2095e49fd91607026364466cd7b389b938e
- https://github.com/slimsag/syntect/commit/1182dd3bd7c82b6655d8466c9896a1e4f458c71e

#### 3) Update syntect_server to use the new version of `syntect`

Run the following in this directory.

```
$ cargo update -p syntect
```

## Supported languages:

Run: `cargo run --bin syntect_server` to see supported languages.
Hello World
