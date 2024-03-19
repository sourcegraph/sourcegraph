# Bitbucket Cloud

Site admins can sync Git repositories hosted on [Bitbucket Cloud](https://bitbucket.org) with Sourcegraph so that users can search and navigate the repositories.

To connect Bitbucket Cloud to Sourcegraph:

1. Go to **Site admin > Manage code hosts > Add repositories**.
2. Select **Bitbucket.org**.
3. Configure the connection to Bitbucket Cloud using the action buttons above the text field. Additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
4. Press **Add repositories**.

## Repository syncing

Currently, all repositories belonging to the user configured will be synced.

In addition, there is one more field for configuring which repositories are mirrored:

- [`teams`](bitbucket_cloud.md#configuration)<br>A list of teams (workspaces) that the configured user has access to whose repositories should be synced.
- [`exclude`](bitbucket_cloud.md#configuration)<br>A list of repositories to exclude, which takes precedence over the `teams` field.

## Configuration options

Bitbucket Cloud code host connections can be configured with either a username and app password combination, or with workspace access tokens.

### Username and app password

1. Visit your [Bitbucket account settings page](https://bitbucket.org/account/settings).
2. Navigate to **App passwords**.
3. Select **Create app password**.
4. Give your app password a label.
5. Select the `Projects: Read` permission. `Repositories: Read` should automatically be selected.
6. Press **Create**.

Use the newly created app password and your username to configure the Bitbucket Cloud connection:

```json
{
  "url": "https://bitbucket.org",
  "username": "USERNAME",
  "appPassword": "<PASSWORD>",
  // ... other settings
}
```

### Workspace access token

1. Visit the Bitbucket Cloud workspace settings page of the workspace you want to create an access token for.
2. Navigate to **Security > Access tokens**.
3. Press **Create workspace access token**.
4. Give your access token a name.
5. Select the `Projects: Read` permission. `Repositories: Read` should automatically be selected.
6. Press **Create**.

Use the newly created access token to configure the Bitbucket Cloud connection:

```json
{
  "url": "https://bitbucket.org",
  "accessToken": "ACCESS_TOKEN",
  // ... other settings
}
```

### HTTPS cloning

Sourcegraph clones repositories from your Bitbucket Cloud via HTTP(S), using the [`username`](bitbucket_cloud.md#configuration) and [`appPassword`](bitbucket_cloud.md#configuration) required fields you provide in the configuration.

## Rate limits

Read about how Bitbucket Cloud applies rate limits [here](https://support.atlassian.com/bitbucket-cloud/docs/api-request-limits/).

When Sourcegraph encounters rate limits on Bitbucket Cloud, it will retry the request with exponential back-off, until 5 minutes have passed. If the connection is still being rate limited after 5 minutes, the request will be dropped.

### Internal rate limits

See [Internal rate limits](./rate_limits.md#internal-rate-limits)

## User authentication

To configure Bitbucket Cloud as an authentication provider (which will enable sign-in via Bitbucket Cloud), see the
[authentication documentation](../auth/index.md#bitbucket-cloud).

## Repository permissions

Prerequisite: [Add Bitbucket Cloud as an authentication provider](#user-authentication).

Then, add or edit a Bitbucket Cloud connection as described above and include the `authorization` field:

```json
{
  // The URL used to set up the Bitbucket Cloud authentication provider must match this URL.
  "url": "https://bitbucket.com",
  "username": "horsten",
  "appPassword": "$APP_PASSWORD",
  // ...
  "authorization": {}
}
```

> NOTE: It can take some time to complete full cycle of repository permissions sync if you have a large number of users or repositories. [See sync duration time](../permissions/syncing.md#sync-duration) for more information.

## Configuration

Bitbucket Cloud connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/bitbucket_cloud.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/bitbucket_cloud) to see rendered content.</div>

## Webhooks

Using the `webhooks` property on the external service has been deprecated.

Please consult [this page](../config/webhooks/incoming.md) in order to configure webhooks.
