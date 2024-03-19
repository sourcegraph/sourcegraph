# Site configuration

Site configuration defines how various Sourcegraph features behave. See the [full reference](#reference) below for a comprehensive list of site configuration options.

## Configuration overview

[Go here](index.md) for an overview of configuring Sourcegraph.

## View and edit site configuration

Site admins can view and edit site configuration on a Sourcegraph instance:

1. Go to **User menu > Site admin**.
1. Open the **Configuration** page. (The URL is `https://sourcegraph.example.com/site-admin/configuration`.)

## Reference

All site configuration options and their default values are shown below.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/config/site.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/config/site_config) to see rendered content.</div>

#### Known bugs

The following site configuration options require the server to be restarted for the changes to take effect:

```
auth.providers
externalURL
insights.query.worker.concurrency
insights.commit.indexer.interval
permissions.syncUsersMaxConcurrency
```

## Editing your site configuration if you cannot access the web UI

If you are having trouble accessing the web UI, you can make edits to your site configuration by editing the configuration directly.

### Sourcegraph with Docker Compose and single-server Sourcegraph with Docker

Set `FRONTEND_CONTAINER` to:

- [Docker Compose](../deploy/docker-compose/index.md): the `sourcegraph-frontend` container
- [Single-container](../deploy/docker-single-container/index.md): the `sourcegraph/server` container

```sh
docker exec -it --user=root $FRONTEND_CONTAINER sh -c 'apk add --no-cache && nano /home/sourcegraph/site-config.json'
```

Or if you prefer using a Vim editor:

```sh
docker exec -it $FRONTEND_CONTAINER sh -c 'vi ~/site-config.json'
```

### Sourcegraph with Kubernetes

For [Kubernetes](../deploy/kubernetes/index.md) deployments:

```sh
kubectl exec -it $FRONTEND_POD -- sh -c 'apk add --no-cache nano && nano ~/site-config.json'
```

Or if you prefer using a Vim editor:

```sh
kubectl exec -it $FRONTEND_POD -- sh -c 'vi ~/site-config.json'
```

Then simply save your changes (type <kbd>ctrl+x</kbd> and <kbd>y</kbd> to exit `nano` and save your changes). Your changes will be applied immediately in the same way as if you had made them through the web UI.

## If you are still encountering issues

You can check the container logs to see if you have made any typos or mistakes in editing the configuration file. If you are still encountering problems, you can save the default site configuration that comes with Sourcegraph (below) or contact support@sourcegraph.com with any questions you have.

```json
{
	// The externally accessible URL for Sourcegraph (i.e., what you type into your browser)
	// This is required to be configured for Sourcegraph to work correctly.
	// "externalURL": "https://sourcegraph.example.com",

	// The authentication provider to use for identifying and signing in users.
	// Only one entry is supported.
	//
	// The builtin auth provider with signup disallowed (shown below) means that
	// after the initial site admin signs in, all other users must be invited.
	//
	// Other providers are documented at https://docs.sourcegraph.com/admin/auth.
	"auth.providers": [
		{
			"type": "builtin",
			"allowSignup": false
		}
	],
}
```
