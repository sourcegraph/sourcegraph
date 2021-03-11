# GitLab

Site admins can sync Git repositories hosted on [GitLab](https://gitlab.com) (GitLab.com and GitLab CE/EE) with Sourcegraph so that users can search and navigate the repositories.

To connect GitLab to Sourcegraph:

1. Depending on whether you are a site admin or user:
    1. *Site admin*: Go to **Site admin > Manage repositories > Add repositories**
    1. *User*: Go to **Settings > Manage repositories**.
1. Select **GitLab**.
1. Configure the connection to GitLab using the action buttons above the text field, and additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration).
1. Press **Add repositories**.

**NOTE** That adding code hosts as a user is currently in private beta.

## Supported versions

- GitLab.com
- GitLab CE/EE (v10.0 and newer)

## Repository syncing

There are three fields for configuring which projects are mirrored/synchronized:

- [`projects`](gitlab.md#configuration)<br>A list of projects in `{"name": "group/name"}` or `{"id": id}` format.
- [`projectQuery`](gitlab.md#configuration)<br>A list of strings with one pre-defined option (`none`), and/or an URL path and query that targets a GitLab API endpoint returning a list of projects.
- [`exclude`](gitlab.md#configuration)<br>A list of projects to exclude which takes precedence over the `projects`, and `projectQuery` fields. It has the same format as `projects`.

### Troubleshooting

You can test your access token's permissions by running a cURL command against the GitLab API. This is the same API and the same project list used by Sourcegraph.

Replace `$ACCESS_TOKEN` with the access token you are providing to Sourcegraph, and `$GITLAB_HOSTNAME` with your GitLab hostname:

```
curl -H 'Private-Token: $ACCESS_TOKEN' -XGET 'https://$GITLAB_HOSTNAME/api/v4/projects'
```

## Repository permissions

By default, all Sourcegraph users can view all repositories. To configure Sourcegraph to use
GitLab's per-user repository permissions, see "[Repository
permissions](../repo/permissions.md#gitlab)".

## User authentication

To configure GitLab as an authentication provider (which will enable sign-in via GitLab), see the
[authentication documentation](../auth/index.md#gitlab).

## Internal rate limits

Internal rate limiting can be configured to limit the rate at which requests are made from Sourcegraph to GitLab.

If enabled, the default rate is set at 36,000 per hour (10 per second) which can be configured via the `requestsPerHour` field (see below). If rate limiting is configured more than once for the same code host instance, the most restrictive limit will be used.

**NOTE** Internal rate limiting is only currently applied when synchronising changesets in [batch changes](../../batch_changes/index.md).

## Configuration

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/gitlab.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/gitlab) to see rendered content.</div>

## Native integration

To provide out-of-the-box code intelligence and navigation features to your users on GitLab, you will need to [configure your GitLab instance](https://docs.gitlab.com/ee/integration/sourcegraph.html). If you are using an HTTPS connection to GitLab, you will need to [configure HTTPS](https://docs.sourcegraph.com/admin/http_https_configuration) for your Sourcegraph instance.

The Sourcegraph instance's site admin must [update the `corsOrigin` site config property](../config/site_config.md) to allow the GitLab instance to communicate with the Sourcegraph instance. For example:

```json
{
  // ...
  "corsOrigin":
    "https://my-gitlab.example.com"
  // ...
}
```

The site admin should also set `alerts.codeHostIntegrationMessaging` in [global settings](../config/settings.md#editing-global-settings-for-site-admins) to ensure informational content for users in the Sourcegraph webapp references the native integration and not the browser extension.

```json
{
  // ...
  "alerts.codeHostIntegrationMessaging": "native-integration"
  // ...
}
```

## Access token scopes

Sourcegraph requires an access token with `api` permissions (and `sudo`, if you are using an `external` identity provider type). These permissions are required for the following reasons:

We are actively collaborating with GitLab to improve our integration (e.g. the [Sourcegraph GitLab native integration](https://docs.gitlab.com/ee/integration/sourcegraph.html) and [better APIs for querying repository permissions](https://gitlab.com/gitlab-org/gitlab/issues/20532)).

| Request Type | Required GitLab scope | Sourcegraph usage |
|--------------|-----------------------|-------------------|
| [`GET /projects`](https://docs.gitlab.com/ee/api/projects.html#list-all-projects) | `api` | (1) For repository discovery when specifying `projectQuery` in code host configuration; (2) If using an `external` identity provider type, also used as a test query to ensure token is `sudo` (`sudo` not required otherwise). |
| [`GET /users`](https://docs.gitlab.com/ee/api/users.html#list-users) | `read_user` or `api` | If you are using an `external` identity provider type, used to discover user accounts. |
| [`GET /users/:id`](https://docs.gitlab.com/ee/api/users.html#single-user) | `read_user` or `api` | If using GitLab OAuth, used to fetch user metadata during the OAuth sign in process. |
| [`GET /projects/:id`](https://docs.gitlab.com/ee/api/projects.html#get-single-project) | `api` | (1) If using GitLab OAuth and repository permissions, used to determine if a user has access to a given _project_; (2) Used to query repository metadata (e.g. description) for display on Sourcegraph. |
| [`GET /projects/:id/repository/tree`](https://docs.gitlab.com/ee/api/repositories.html#list-repository-tree) | `api` | If using GitLab OAuth and repository permissions, used to verify a given user has access to the file contents of a repository within a project (i.e. does not merely have `Guest` permissions). |
| Batch changes requests | `api`, `read_repository`, `write_repository` | [Batch changes](../../batch_changes/index.md) require write access to push commits and create, update and close merge requests on GitLab repositories. See "[Code host interactions in batch changes](../../batch_changes/explanations/permissions_in_batch_changes.md#code-host-interactions-in-batch-changes)" for details. |

## Webhooks

The `webhooks` setting allows specifying the webhook secrets necessary to authenticate incoming webhook requests to `/.api/gitlab-webhooks`.

```json
"webhooks": [
  {"secret": "verylongrandomsecret"}
]
```

Using webhooks is highly recommended when using [batch changes](../../batch_changes/index.md), since they speed up the syncing of pull request data between GitLab and Sourcegraph and make it more efficient.

To set up webhooks:

1. In Sourcegraph, go to **Site admin > Manage repositories** and edit the GitLab configuration.
1. Add the `"webhooks"` property to the configuration (you can generate a secret with `openssl rand -hex 32`):<br /> `"webhooks": [{"secret": "verylongrandomsecret"}]`
1. Click **Update repositories**.
1. Copy the webhook URL displayed below the **Update repositories** button.
1. On GitLab, go to your project, and then **Settings > Webhooks** (or **Settings > Integration** on older GitLab versions that don't have the **Webhooks** option).
1. Fill in the webhook form:
   * **URL**: the URL you copied above from Sourcegraph.
   * **Secret token**: the secret token you configured Sourcegraph to use above.
   * **Trigger**: select **Merge request events** and **Pipeline events**.
   * **Enable SSL verification**: ensure this is enabled if you have configured SSL with a valid certificate in your Sourcegraph instance.
1. Click **Add webhook**.
1. Confirm that the new webhook is listed below **Project Hooks**.

Done! Sourcegraph will now receive webhook events from GitLab and use them to sync merge request events, used by [batch changes](../../batch_changes/index.md), faster and more efficiently.
