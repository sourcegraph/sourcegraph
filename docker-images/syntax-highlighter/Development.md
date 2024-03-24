# Developing Syntax Highlighter

- [Building](#building)
- [Testing](#testing)
  - [Running tests](#running-tests)
  - [Updating snapshots](#updating-snapshots)
- [Formatting](#formatting)
- [Debugging](#debugging)
- [Modifying configuration](#modifying-configuration)
  - [Adding dependencies](#adding-dependencies)
  - [Updating Rust version](#updating-rust-version)
  - [Other configuration changes](#other-configuration-changes)

## Building

```
cargo build --workspace
# OR
bazel build //docker-images/syntax-highlighter/...
```

Both commands should be equivalent.

## Testing

### Running tests

```bash
cargo test --workspace
# OR
bazel test //docker-images/syntax-highlighter/...
```

Both commands above should be equivalent.

### Updating snapshots

```bash
cargo insta test
cargo insta review
```

At the moment, this is not supported using Bazel.

## Debugging

TODO

## Formatting

```bash
cargo fmt --all
# OR
bazel run @rules_rust//:rustfmt
```

Both commands above should be equivalent.

## Modifying configuration

### Adding dependencies

For dependencies that are shared across crates,
prefer adding them in the `Cargo.toml` file at the workspace root,
and access them using `workspace = true`.

For dependencies in a specific crate,
add them to the appropriate `Cargo.toml`.

After updating `Cargo.toml`, run:

```bash
CARGO_BAZEL_REPIN=1 bazel sync --only=crates_index
```

WARNING: The `bazel sync` invocation may take several minutes to complete.

### Updating Rust version

Update the `rust-toolchain` file and the `WORKSPACE` file at the root.

### Other configuration changes

Most of Bazel's configuration is inferred from `Cargo.toml` files.
However, [gazelle](https://github.com/bazelbuild/bazel-gazelle) does not support Rust
at the moment, so the `BUILD.bazel` files are hand-written
and may need modifications for crate-specific changes.

For toolchain changes, the `WORKSPACE` file may also need changes.
