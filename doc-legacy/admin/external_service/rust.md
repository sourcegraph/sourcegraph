# Rust dependencies

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change or be removed in the future. We've released it as an experimental feature to provide a preview of functionality we're working on.
</p>
</aside>

Site admins can sync Rust dependencies from any Cargo repository, including crates.io or an internal Artifactory, to their Sourcegraph instance so that users can search and navigate their dependencies.

To add Rust dependencies to Sourcegraph you need to setup a Rust dependencies code host:

1. As *site admin*: go to **Site admin > Global settings** and enable the experimental feature by adding: `{"experimentalFeatures": {"rustPackages": "enabled"} }`
1. As *site admin*: go to **Site admin > Manage code hosts**
1. Select **Rust Dependencies**.
1. [Configure the connection](#configuration) by following the instructions above the text field. Additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Repository syncing

There are two ways to sync Rust dependency repositories.

* **Indexing** (recommended): run [`scip-rust`](https://sourcegraph.github.io/scip-rust/) against your Rust codebase and upload the generated index to Sourcegraph using the [src-cli](https://github.com/sourcegraph/src-cli) command `src code-intel upload`. This is usually setup to run in a CI pipeline. Sourcegraph automatically synchronizes Rust dependency repositories based on the dependencies that are discovered by `scip-rust`.
* **Code host configuration**: manually list dependencies in the `"dependencies"` section of the [JSON configuration](#configuration) when creating the Rust dependency code host. This method can be useful to verify that the credentials are picked up correctly without having to upload an index.

## Credentials

The `"repository"` field in the [configuration](#configuration) section is automatically redacted and can optionally include the username and password of an internal [Artifactory Cargo](https://www.jfrog.com/confluence/display/JFROG/Cargo+Repositories) repository.

## Rate limiting

By default, requests to the Cargo repository is 8 request per second.

To manually set the value, add the following to your code host configuration:

```json
"rateLimit": {
  "enabled": true,
  "requestsPerHour": 600
}
```

where the `requestsPerHour` field is set based on your requirements.

**Not recommended**: Rate-limiting can be turned off entirely as well.
This increases the risk of overloading the code host.

```json
"rateLimit": {
  "enabled": false
}
```

## Configuration

Rust dependencies code host connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/rust-packages.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/integration/rust) to see rendered content.</div>
