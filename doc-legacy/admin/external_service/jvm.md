# JVM dependencies

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change or be removed in the future. We've released it as an experimental feature to provide a preview of functionality we're working on.
</p>
</aside>

Site admins can sync JVM dependencies from any Maven repository, including Maven Central, Sonatype, or Artifactory, to their Sourcegraph instance so that users can search and navigate the repositories.

To add JVM dependencies to Sourcegraph you need to setup a JVM dependencies code host:

1. As *site admin*: go to **Site admin > Site configuration** and enable the experimental feature by adding: `{"experimentalFeatures": {"jvmPackages": "enabled"} }`
1. As *site admin*: go to **Site admin > Manage code hosts**
1. Select **JVM Dependencies**.
1. [Configure the connection](#configuration) by following the instructions above the text field. Additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Repository syncing

There are two ways to sync JVM dependency repositories.

* **Indexing** (recommended): run [`scip-java`](https://sourcegraph.github.io/scip-java/) against your JVM codebase and upload the generated index to Sourcegraph using the [src-cli](https://github.com/sourcegraph/src-cli) command `src code-intel upload`. This is usually setup to run in a CI pipeline. Sourcegraph automatically synchronizes JVM dependency repositories based on the dependencies that are discovered by `scip-java`.
* **Code host configuration**: manually list dependencies in the `"dependencies"` section of the [JSON configuration](#configuration) when creating the JVM dependency code host. This method can be useful to verify that the credentials are picked up correctly without having to upload an index.

## Credentials

Sourcegraph uses [Coursier](https://get-coursier.io/) to resolve JVM dependencies.
Use the `"credentials"` section of the JSON configuration to provide usernames and passwords to access your Maven repository. See the Coursier documentation about [inline credentials](https://get-coursier.io/docs/other-credentials#inline) to learn more about how to format the `"credentials"` configuration, or the example displayed at the bottom of the page.

## Rate limiting

By default, requests to the JVM dependency code host will be rate-limited
based on a default internal limit. ([source](https://github.com/sourcegraph/sourcegraph/blob/main/schema/jvm-packages.schema.json))

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

JVM dependencies code host connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/jvm-packages.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/jvm) to see rendered content.</div>
