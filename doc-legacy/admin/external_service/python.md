# Python dependencies

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change or be removed in the future. We've released it as an experimental feature to provide a preview of functionality we're working on.
</p>
</aside>

Site admins can sync Python packages from any Python package mirror, including open source code from pypi.org or a private mirror such as [Nexus](https://www.sonatype.com/products/nexus-repository), to their Sourcegraph instance so that users can search and navigate the repositories.

To add Python dependencies to Sourcegraph you need to setup a Python dependencies code host:

1. As *site admin*: go to **Site admin > Global settings** and enable the experimental feature by adding: `{"experimentalFeatures": {"pythonPackages": "enabled"} }`
1. As *site admin*: go to **Site admin > Manage code hosts**
1. Select **Python Dependencies**.
1. [Configure the connection](#configuration) by following the instructions above the text field. Additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

## Repository syncing

There are currently two ways to sync Python dependency repositories.

* **Code host configuration**: manually list dependencies in the `"dependencies"` section of the JSON configuration when creating the Python dependency code host. This method can be useful to verify that the credentials are picked up correctly without having to run a dependencies search.

Sourcegraph tries to find each dependency repository in all configured `"urls"` until it's found. This means you can configure a public mirror first and fallback to a private one second (e.g. `"urls": ["https://pypi.org", "https://admin:foobar@nexus.yourcorp.com"]`).

## Credentials

Each entry in the `"urls"` array can contain basic auth if needed (e.g. `https://user:password@nexus.yourcorp.com`).

## Rate limiting

By default, requests to the Python package mirrors will be rate-limited based on a default internal limit. ([source](https://github.com/sourcegraph/sourcegraph/blob/main/schema/python-packages.schema.json))

```json
"rateLimit": {
  "enabled": true,
  "requestsPerHour": 57600
}
```

where the `requestsPerHour` field is set based on your requirements.

**Not recommended**: Rate-limiting can be turned off entirely as well.
This increases the risk of overloading the proxy.

```json
"rateLimit": {
  "enabled": false
}
```

## Configuration

Python dependencies code host connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/python-packages.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/integration/python) to see rendered content.</div>
