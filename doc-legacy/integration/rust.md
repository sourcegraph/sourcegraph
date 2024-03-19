# Rust dependencies integration with Sourcegraph

You can use Sourcegraph with Rust dependencies from any Cargo repository, including crates.io or an internal Artifactory.

This integration makes it possible to search and navigate through the source code of published Rust crates (for example, [`serde@1.0.158`](https://sourcegraph.com/crates/serde@v1.0.158)).

Feature | Supported?
------- | ----------
[Repository syncing](#repository-syncing) | ✅
[Repository permissions](#repository-syncing) | ❌
[Multiple Cargo repositories code hosts](#multiple-rust-dependencies-code-hosts) | ❌

## Setup

See the "[Rust dependencies](../admin/external_service/rust.md)" documentation.

## Repository syncing

Site admins can [add Rust dependencies to Sourcegraph](../admin/external_service/rust.md#repository-syncing).

## Repository permissions

⚠ Rust dependencies are visible by all users of the Sourcegraph instance.

## Multiple Rust dependencies code hosts

⚠️ It's only possible to create one Rust dependency code host for each Sourcegraph instance.

See the issue [sourcegraph#32461](https://github.com/sourcegraph/sourcegraph/issues/32461) for more details about this limitation.
