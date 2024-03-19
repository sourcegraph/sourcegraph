# npm dependencies

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change or be removed in the future. We've released it as an experimental feature to provide a preview of functionality we're working on.
</p>
</aside>

Site admins can sync npm packages from any npm registry, including open source code from npmjs.com or a private registry such as Verdaccio, to their Sourcegraph instance so that users can search and navigate the repositories.

To add npm dependencies to Sourcegraph you need to setup an npm dependencies code host:

1. As *site admin*: go to **Site admin > Global settings** and enable the experimental feature by adding: `{"experimentalFeatures": {"npmPackages": "enabled"} }`
1. As *site admin*: go to **Site admin > Manage code hosts**
1. Select **npm Dependencies**.
1. [Configure the connection](#configuration) by following the instructions above the text field. Additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Repository syncing

There are three ways to sync npm dependency repositories.

* **SCIP** (recommended): run [`scip-typescript`](https://github.com/sourcegraph/scip-typescript) on your JavaScript/TypeScript codebase and upload the generated index to Sourcegraph using the [src-cli](https://github.com/sourcegraph/src-cli) command `src code-intel upload`. Sourcegraph automatically synchronizes npm dependency repositories based on the dependencies that are discovered by `scip-typescript`.
* **Code host configuration**: manually list dependencies in the `"dependencies"` section of the JSON configuration when creating the npm dependency code host. This method can be useful to verify that the credentials are picked up correctly without having to upload an index.

## Credentials

Use the `"credentials"` section of the JSON configuration to provide an access token for your private npm registry. See the [official npm documentation](https://docs.npmjs.com/about-access-tokens) for more details about how to create, list and view npm access tokens.

## Rate limiting

By default, requests to the npm registry will be rate-limited based on a default [internal limit](https://github.com/sourcegraph/sourcegraph/blob/main/schema/npm-packages.schema.json) which complies with the [documented acceptable use policy](https://docs.npmjs.com/policies/open-source-terms#acceptable-use) of registry.npmjs.org (i.e. max 5 million requests per month).

```json
"rateLimit": {
  "enabled": true,
  "requestsPerHour": 3000
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

npm dependencies code host connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/npm-packages.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/integration/npm) to see rendered content.</div>
