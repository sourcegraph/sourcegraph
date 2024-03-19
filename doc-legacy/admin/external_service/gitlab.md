# GitLab

Site admins can sync Git repositories hosted on [GitLab](https://gitlab.com) (GitLab.com and GitLab CE/EE) with Sourcegraph so that users can search and navigate the repositories.

To connect GitLab to Sourcegraph:

1. Go to **Site admin > Manage code hosts > Add code host**
2. Select **GitLab** (for GitLab.com) or **GitLab Self-Managed**.
3. Set **url** to the URL of your GitLab instance, such as https://gitlab.example.com or https://gitlab.com (for GitLab.com).
4. Create a GitLab access token using these [instructions](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html#creating-a-personal-access-token) with the `read_api` and `read_repository` scopes, and set it to be the value of the token.
5. Use the [Repository syncing documentation below](#repository-syncing) to select and add your preferred projects/repos to the configuration.
6. You can use the action buttons above the text field to add the fields, and additional fields can be added using <kbd>Cmd/Ctrl+Space</kbd> for auto-completion. See the [configuration documentation below](#configuration) for additional fields.
7. Click **Add repositories**.

Example config:
```
{
  "url": "https://gitlab.com",
  "token": "<access token>",
  "projectQuery": [
    "groups/mygroup/projects",
    "projects?membership=true&archived=no",
    "?search=<search query>",
    "?membership=true\u0026search=foo"
 ],
 "projects": [
  {
     "name": "group/name"
  },
  {
      "id": 42
  }
 ]
}
```

## Supported versions

- GitLab.com
- GitLab CE/EE (v12.0 and newer)

## Repository syncing

There are three fields for configuring which projects are mirrored/synchronized:

- [`projects`](gitlab.md#configuration)<br>A list of projects in `{"name": "group/name"}` or `{"id": id}` format. The order determines the order in which we sync project metadata and is safe to change.
- [`projectQuery`](gitlab.md#configuration)<br>A list of strings with one pre-defined option (`none`), and/or an URL path and query that targets the [GitLab Projects API endpoint](https://docs.gitlab.com/ee/api/projects.html), returning a list of projects.
- [`exclude`](gitlab.md#configuration)<br>A list of projects to exclude which takes precedence over the `projects`, and `projectQuery` fields. It has the same format as `projects`.

### Troubleshooting

You can test your access token's permissions by running a cURL command against the GitLab API. This is the same API and the same project list used by Sourcegraph.

Replace `$ACCESS_TOKEN` with the access token you are providing to Sourcegraph, and `$GITLAB_HOSTNAME` with your GitLab hostname:

```
curl -H 'Private-Token: $ACCESS_TOKEN' -XGET 'https://$GITLAB_HOSTNAME/api/v4/projects'
```

## Repository permissions

GitLab permissions can be configured in three ways:

1. Set up GitLab as an OAuth sign-on provider for Sourcegraph (recommended)
2. Use a GitLab administrator (sudo-level) personal access token in conjunction with another SSO provider
   (recommended only if the first option is not possible)
3. Assume username equivalency between Sourcegraph and GitLab (warning: this is generally unsafe and
   should only be used if you are using strictly `http-header` authentication).

> NOTE: It can take some time to complete full cycle of repository permissions sync if you have a large number of users or repositories. [See sync duration time](../permissions/syncing.md#sync-duration) for more information.

### OAuth application

Prerequisite: [Add GitLab as an authentication provider.](../auth/index.md#gitlab)

Then, [add or edit a GitLab connection](#repository-syncing) and include the `authorization` field:

```json
{
  "url": "https://gitlab.com",
  "token": "$PERSONAL_ACCESS_TOKEN",
  // ...
  "authorization": {
    "identityProvider": {
      "type": "oauth"
    }
  }
}
```

In this case, a user's OAuth token will be used to get a list of repositories that the user can access.
[Repository-centric permissions syncing](../permissions/syncing.md) will be disabled.

### Administrator (sudo-level) access token

This method requires administrator access to GitLab so that Sourcegraph can access the [admin GitLab Users API endpoint](https://docs.gitlab.com/ee/api/users.html#for-admins). For each GitLab user, this endpoint provides the user ID that comes from the authentication provider, so Sourcegraph can associate a user in its system to a user in GitLab.

Prerequisite: Add the [SAML](../auth/index.md#saml) or [OpenID Connect](../auth/index.md#openid-connect)
authentication provider you use to sign into GitLab.

Then, [add or edit a GitLab connection](#repository-syncing) using an administrator (sudo-level) personal access token, and include the `authorization` field:

```json
{
  "url": "https://gitlab.com",
  "token": "$PERSONAL_ACCESS_TOKEN",
  // ...
  "authorization": {
    "identityProvider": {
      "type": "external",
      "authProviderID": "$AUTH_PROVIDER_ID",
      "authProviderType": "$AUTH_PROVIDER_TYPE",
      "gitlabProvider": "$AUTH_PROVIDER_GITLAB_ID"
    }
  }
}
```

`$AUTH_PROVIDER_ID` and `$AUTH_PROVIDER_TYPE` identify the authentication provider to use and should
match the fields specified in the authentication provider config
(`auth.providers`). The authProviderID can be found in the `configID` field of the auth provider config.

`$AUTH_PROVIDER_GITLAB_ID` should match the `identities.provider` returned by
[the admin GitLab Users API endpoint](https://docs.gitlab.com/ee/api/users.html#for-admins).

### Username

Prerequisite: Ensure that `http-header` is the *only* authentication provider type configured for
Sourcegraph. If this is not the case, then it will be possible for users to escalate privileges,
because Sourcegraph usernames are mutable.

[Add or edit a GitLab connection](#repository-syncing) and include the `authorization` field:

```json
{
  "url": "https://gitlab.com",
  "token": "$PERSONAL_ACCESS_TOKEN",
  // ...
  "authorization": {
    "identityProvider": {
      "type": "username"
    }
  }
}
```

## User authentication

To configure GitLab as an authentication provider (which will enable sign-in via GitLab), see the
[authentication documentation](../auth/index.md#gitlab).

## Internal repositories

GitLab also has internal repositories in addition to the usual public and private repositories. Depending on how your organization structure is configured, you may want to make these internal repositories available to everyone on your Sourcegraph instance without relying on permission syncs. To mark all internal repositories as public, add the following field to the code host connection:

```json
{
  // ...
  "markInternalReposAsPublic": true
}
```

When adding this configuration option, you may also want to configure your GitLab auth provider so that it does [not sync user permissions for internal repositories](../auth/index.md#dont-sync-user-permissions-for-internal-repositories).

## Rate limits

Always include a token in a configuration for a GitLab.com URL to avoid being denied service by GitLab's [unauthenticated rate limits](https://docs.gitlab.com/ee/user/gitlab_com/index.html#gitlabcom-specific-rate-limits).

When Sourcegraph hits a rate limit imposed by GitLab, Sourcegraph waits the appropriate amount of time specified by GitLab before retrying the request. This can be several minutes in extreme cases.

### Internal rate limits

See [Internal rate limits](./rate_limits.md#internal-rate-limits).

## Configuration

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/gitlab.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/gitlab) to see rendered content.</div>

## Native integration

To provide out-of-the-box code navigation features to your users on GitLab, you will need to [configure your GitLab instance](https://docs.gitlab.com/ee/integration/sourcegraph.html). If you are using an HTTPS connection to GitLab, you will need to [configure HTTPS](https://docs.sourcegraph.com/admin/http_https_configuration) for your Sourcegraph instance.

The Sourcegraph instance's site admin must [update the `corsOrigin` site config property](../config/site_config.md) to allow the GitLab instance to communicate with the Sourcegraph instance. For example:

```json
{
  // ...
  "corsOrigin":
    "https://my-gitlab.example.com"
  // ...
}
```

## Access token scopes

Sourcegraph requires an access token with `api` permissions (and `sudo`, if you are using an `external` identity provider type). These permissions are required for the following reasons:

We are actively collaborating with GitLab to improve our integration (e.g. the [Sourcegraph GitLab native integration](https://docs.gitlab.com/ee/integration/sourcegraph.html) and [better APIs for querying repository permissions](https://gitlab.com/gitlab-org/gitlab/issues/20532)).

| Request Type | Required GitLab scope | Sourcegraph usage |
|--------------|-----------------------|-------------------|
| [`GET /projects`](https://docs.gitlab.com/ee/api/projects.html#list-all-projects) | `api` or `read_api`| (1) For repository discovery when specifying `projectQuery` in code host configuration; (2) If using an `external` identity provider type, also used as a test query to ensure token is `sudo` (`sudo` not required otherwise). |
| [`GET /users`](https://docs.gitlab.com/ee/api/users.html#list-users) | `read_user`, `api` or `read_api` | If you are using an `external` identity provider type, used to discover user accounts. |
| [`GET /users/:id`](https://docs.gitlab.com/ee/api/users.html#single-user) | `read_user`, `api` or `read_api` | If using GitLab OAuth, used to fetch user metadata during the OAuth sign in process. |
| [`GET /projects/:id`](https://docs.gitlab.com/ee/api/projects.html#get-single-project) | `api` or `read_api` | (1) If using GitLab OAuth and repository permissions, used to determine if a user has access to a given _project_; (2) Used to query repository metadata (e.g. description) for display on Sourcegraph. |
| [`GET /projects/:id/repository/tree`](https://docs.gitlab.com/ee/api/repositories.html#list-repository-tree) | `api` or `read_api` | If using GitLab OAuth and repository permissions, used to verify a given user has access to the file contents of a repository within a project (i.e. does not merely have `Guest` permissions). |
| Batch Changes requests | `api` or `read_api`, `read_repository`, `write_repository` | [Batch Changes](../../batch_changes/index.md) require write access to push commits and create, update and close merge requests on GitLab repositories. See "[Code host interactions in batch changes](../../batch_changes/explanations/permissions_in_batch_changes.md#code-host-interactions-in-batch-changes)" for details. |

## Webhooks

Using the `webhooks` property on the external service has been deprecated.

Please consult [this page](../config/webhooks/incoming.md) in order to configure webhooks.
