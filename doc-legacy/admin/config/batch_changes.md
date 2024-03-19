# Batch Changes site admin configuration reference

Batch Changes is generally configured through the same [site configuration](site_config.md) and [code host configuration](../external_service/index.md) as the rest of Sourcegraph. However, Batch Changes features may require specific configuration, and those are documented here.

## Access control

<span class="badge badge-note">Sourcegraph 5.0+</span>

Batch Changes is [RBAC-enabled](../../admin/access_control/index.md) <span class="badge badge-beta">Beta</span>. By default, all users have full read and write access for Batch Changes, but this can be restricted by changing the default role permissions, or by creating new custom roles.

### Enable organization members to administer

<span class="badge badge-note">Sourcegraph 5.0.5+</span>

By default, only a batch change's author or a site admin can administer (apply, close, rename, etc.) a batch change. However, admins can use [organizations](../../admin/organizations.md) to facilitate closer collaboration and shared administrative control over batch changes by enabling the `orgs.allMembersBatchChangesAdmin` setting for an organization. When enabled, members of the organization will be able to administer all batch changes created in that organization's namespace. Batch changes created in other namespaces (user or organization) will still be restricted to the author and site admins.

## Rollout windows

By default, Sourcegraph attempts to reconcile (create, update, or close) changesets as quickly as the rate limits on the code host allow. This can result in CI systems being overwhelmed if hundreds or thousands of changesets are being handled as part of a single batch change.

Configuring rollout windows allows changesets to be reconciled at a slower or faster rate based on the time of day and/or the day of the week. These windows are applied to changesets across all code hosts, but they only affect the rate at which changesets are created/published, updated, or closed, as well as some other internal operations like importing and detaching. Bulk operations to publish changesets also respect the rollout window; however, bulk commenting, merging, and closing will happen all at once.

Rollout windows are configured through the `batchChanges.rolloutWindows` [site configuration option](site_config.md). If specified, this option contains an array of rollout window objects that are used to schedule changesets. The format of these objects [is given below](#rollout-window-object).

### Behavior

When rollout windows are enabled, changesets will initially enter a **Scheduled** state when their batch change is applied. Hovering or tapping on the changeset's state icon will provide an estimate of when the changeset will be reconciled.

To restore the default behavior, you can either delete the `batchChanges.rolloutWindows` option, or set it to `null`.

Or, to put it another way:

| `batchChanges.rolloutWindows` configuration | Behavior |
|---------------------------------------------|-----------|
| Omitted, or set to `null`                   | Changesets will be reconciled as fast as the code host allows; essentially the same as setting a single `{"rate": "unlimited"}` window. |
| Set to an array (even if empty)             | Changesets will be reconciled using the rate limit in the current window using [the leaky bucket behavior described below](#leaky-bucket-rate-limiting). If no window covers the current period, then no changesets will be reconciled until a window with a non-zero [`rate`](#rate) opens. |
| Any other value                             | The configuration is invalid, and an error will appear. |

#### Leaky bucket rate limiting

Rate limiting uses the [leaky bucket algorithm](https://en.wikipedia.org/wiki/Leaky_bucket) to smooth bursts in reconciliations.

Practically speaking, this means that the given rate can be thought of more as an average than as a simple resource allocation. If there are always changesets in the queue, a rate of `10/hour` means that a changeset will be reconciled approximately every six minutes, rather than ten changesets being simultaneously reconciled at the start of each hour.

### Avoiding hitting rate limits

Keep in mind that if you configure a rollout window that is too aggressive, you risk exceeding your code hosts' API rate limits. We recommend maintaining a rate that is no faster than `5/minute`; however, you can refer to your code host's API docs if you wish to increase it beyond this recommendation:

* [GitHub](https://docs.github.com/en/graphql/overview/resource-limitations#rate-limit)
* [GitLab](https://docs.gitlab.com/ee/user/gitlab_com/index.html#gitlabcom-specific-rate-limits)
* [Bitbucket Cloud](https://support.atlassian.com/bitbucket-cloud/docs/api-request-limits/)

When using a [global service account token](../../batch_changes/how-tos/configuring_credentials.md#global-service-account-tokens) with Batch Changes, keep in mind that this token will also be used for other Batch Changes <> code host interactions, too.

You may encounter this error when publishing changesets to GitHub:

> **Failed to run operations on changeset**
>
> Creating changeset: error in GraphQL response: was submitted too quickly

In addition to their normal API rate limits, GitHub also has an internal _content creation_ limit (also called [secondary rate limit](https://docs.github.com/en/rest/guides/best-practices-for-integrators?apiVersion=2022-11-28#dealing-with-secondary-rate-limits)), which is an [intentional](https://github.com/cli/cli/issues/4801#issuecomment-1029207971) restriction on the platform to combat abuse by automated actors. At the time of writing, the specifics of this limit remain undocumented, due largely to the fact that it is dynamically determined (see [this GitHub issue](https://github.com/cli/cli/issues/4801)). However, the behavior of the limit is that it only permits a fixed number of resources to be created per minute and per hour, and exceeding this limit triggers a temporary hour-long suspension during which time no additional resources of this type can be created.

Presently, Batch Changes does not automatically work around this limit (feature request tracked [here](https://github.com/sourcegraph/sourcegraph/issues/44631). The current guidance if you do encounter this issue is to wait an hour and then try again, setting a less frequent `rolloutWindows` rate until this issue is no longer encountered.

### Rollout window object

A rollout window is a JSON object that looks as follows:

```json
{
  "rate": "10/hour",
  "days": ["saturday", "sunday"],
  "start": "06:00",
  "end": "20:00"
}
```

All fields are optional except for `rate`, and are described below in more detail. All times and days are handled in UTC.

In the event multiple windows overlap, the last defined window will be used.

#### `rate`

`rate` describes the rate at which changesets will be reconciled. This may be expressed in one of the following ways:

* The string `unlimited`, in which case no limit will be applied for this window, or
* A string in the format `N/UNIT`, where `N` is a number and `UNIT` is one of `second`, `minute`, or `hour`; for example, `10/hour` would allow 10 changesets to be reconciled per hour, or
* The number `0`, which will prevent any changesets from being reconciled when this window is active.

#### `days`

`days` is an array of strings that defines the days of the week that the window applies to. English day names are accepted in a case insensitive manner:

* `["saturday", "sunday"]` constrains the window to Saturday and Sunday.
* `["tuesday"]` constrains the window to only Tuesday.

If omitted or an empty array, all days of the week will be matched.

#### `start` and `end`

`start` and `end` define the start and end of the window on each day that is matched by [`days`](#days), or every day of the week if `days` is omitted. Values are defined as `HH:MM` in UTC.

Both `start` and `end` must be provided or omitted: providing only one is invalid.

### Examples

To rate limit changeset publication to 3 per minute between 08:00 and 16:00 UTC on weekdays, and allow unlimited changesets outside of those hours:

```json
[
  {
    "rate": "unlimited"
  },
  {
    "rate": "3/minute",
    "days": ["monday", "tuesday", "wednesday", "thursday", "friday"],
    "start": "08:00",
    "end": "16:00"
  }
]
```

To only allow changesets to be reconciled at 1 changeset per minute on (UTC) weekends:

```json
[
  {
    "rate": "1/minute",
    "days": ["saturday", "sunday"]
  }
]
```

## Incoming webhooks

<span class="badge badge-note">Sourcegraph 3.33+</span>

Sourcegraph can track incoming webhooks from code hosts to more easily debug issues with webhook delivery. Learn [how to setup webhooks and configure logging](../../admin/config/webhooks/incoming.md#webhook-logging).

## Forks

<span class="badge badge-note">Sourcegraph 3.36+</span>

Sourcegraph can be configured to push branches created by Batch Changes to a fork of the repository, rather than the repository itself, for example if users of your code host typically do not have push access to the original repository. You can enable pushing to forks globally with the `batchChanges.enforceForks` site configuration option. Users can also indicate they do or do not want to push to forks for an individual batch change by specifying the property `changesetTemplate.fork` in their batch spec. If the batch spec property is present, it will override the site configuration option. See the [batch spec YAML reference](../../batch_changes/references/batch_spec_yaml_reference.md#changesettemplate-fork) for more information.

The fork will be created in the namespace of the user publishing the changeset, or the namespace of the service account if [global service account](../../batch_changes/how-tos/configuring_credentials.md#global-service-account-tokens) is in use. The name of the fork Sourcegraph creates will be prefixed with the name of the original repo's namespace in order to prevent potential repo name collisions. For example, a batch spec targeting `github.com/my-org/project` would create or use any existing fork by the name `github.com/user/my-org-project`.

### Examples

To enable forks, update the site configuration to include:

```json
{
  "batchChanges.enforceForks": true
}
```

## Automatically delete branches on merge/close

<span class="badge badge-note">Sourcegraph 5.1+</span>

Sourcegraph can be configured to automatically delete branches created for Batch Changes changesets when changesets are merged or closed by enabling the `batchChanges.autoDeleteBranch` site configuration option.

When enabled, Batch Changes will override any setting on the repository on the code host itself and attempt to remove the source branch of the changeset when the changeset is merged or closed. This is useful for keeping repositories clean of stale branches.

Not every code host supports this in the same way; some code host APIs expose a property on the changeset which can be toggled to enable this behavior, while others require a separate API call to delete the branch after the changeset is merged/closed.

For those that support a changeset property, Batch Changes will automatically set the property to match the site config setting. The property will be updated whenever the changeset is updated, so that the settings stay in sync. Using a changeset property has the added benefit that the branch will be deleted even if the changeset is merged/closed on the code host itself, rather than through Sourcegraph.

For those that require a separate API call, Batch Changes will only be able to delete the branch if the changeset is merged/closed _using Sourcegraph_. If the changeset is merged/closed on the code host itself, Batch Changes will not be able to delete the branch.

Refer to the table below to see the levels with which each code host is supported:

Code Host | Changeset property or separate API call? | Support on merge | Support on close | Note
--------- | --------- | :-: | :-: | ----
Azure DevOps | Changeset property | ✓ | ✗ |
Bitbucket Cloud | Changeset property | ✓ | ✓ |
Bitbucket Server | API call | ✓ | ✓ |
GitHub | API call | ✓ | ✓ |
GitLab | Changeset property | ✓ | ✓ |
Gerrit | API call | ✗ | ✓ | Requires ["delete own changes" permission](https://gerrit-review.googlesource.com/Documentation/access-control.html#category_delete_own_changes) at minimum

## Commit signing for GitHub

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> <strong>This feature is currently in beta.</strong>
</p>
</aside>

<span class="badge badge-note">Sourcegraph 5.1+</span>

Sourcegraph can be configured to [sign commits pushed to GitHub](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification) using a GitHub App. Commit signing prevents tampering by unauthorized parties and provides a way to ensure that commits pushed to branches created by Batch Changes actually do come from Sourcegraph. Enabling commit signing for Batch Changes can also help pass checks in build systems or CI/CD pipelines that require that all commits are signed and verified before they can be merged.

At present, only GitHub code hosts (both Cloud and Enterprise) are supported, and only GitHub App signing is supported. Support for other code hosts and signing methods may be added in the future.

GitHub Apps are also the recommended way to [sync repositories on GitHub](../external_service/github.md#using-a-github-app). However, **they are not a replacement for [PATs](../../batch_changes/how-tos/configuring_credentials.md#personal-access-tokens) in Batch Changes**. It is **also** necessary to create a separate GitHub App for Batch Changes commit signing even if you already have an App connected for the same code host for repository syncing because the Apps require different permissions. The process for creating each type of GitHub App is almost identical.

<!-- NOTE: The instructions in the following sections closely mirror those in doc/admin/external_service/github.md. When making changes here, be sure to consider if those changes should also be made over there! -->

To create a GitHub App for commit signing and connect it to Sourcegraph:

1. Go to **Site admin > Batch Changes > Settings** on Sourcegraph.

<img alt="The Batch Changes settings page on Sourcegraph, scrolled to show commit signing integrations" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-apps-batches-list.png" class="screenshot theme-light-only" />
<img alt="The Batch Changes settings page on Sourcegraph, scrolled to show commit signing integrations" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-apps-batches-list-dark.png" class="screenshot theme-dark-only" />

2. Click **Create GitHub App** for the GitHub instance on which you want to enable commit signing.
3. Enter a name for your app (it must be unique across your GitHub instance).

    You may optionally specify an organization to register the app with. If no organization is specified, the app will be owned by the account of the user who creates it on GitHub. This is the default.

    You may also optionally set the App visibility to public. A GitHub App must be made public if you wish to install it on multiple organizations or user accounts. The default is private.

<img alt="The GitHub App creation page on Sourcegraph, with the default values filled out" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-apps-create-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App creation page on Sourcegraph, with the default values filled out" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-apps-create-batches-dark.png" class="screenshot theme-dark-only" />

4. When you click **Create GitHub App**, you will be redirected to GitHub to confirm the details of the App to be created.

<img alt="The GitHub App creation page on GitHub" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-create-on-gh-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App creation page on GitHub" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-create-on-gh-batches-dark.png" class="screenshot theme-dark-only" />

5. To complete the setup on GitHub, you will be asked to review the App permissions and select which repositories the App can access before installing it in a namespace. The default is **All repositories**. Any repositories that you choose to omit will not be able to have changesets published to them from Sourcegraph. You can change this later.

<img alt="The GitHub App installation page on GitHub" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-install-gh-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App installation page on GitHub" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-install-gh-batches-dark.png" class="screenshot theme-dark-only" />

6. Click **Install**. Once complete, you will be redirected back to Sourcegraph, where the updated commit signing integration should be listed.

<img alt="The Batch Changes settings page on Sourcegraph, scrolled to show the newly added commit signing integration" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-post-create-batches.png" class="screenshot theme-light-only" />
<img alt="The Batch Changes settings page on Sourcegraph, scrolled to show the newly added commit signing integration" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-post-create-batches-dark.png" class="screenshot theme-dark-only" />

7. (Optional) If you want to sign commits for changesets in repositories from other organization or user namespaces and your GitHub App is set to public visibility, you can create additional installations by clicking **Edit** on the App and then from the detail page clicking **Add installation**.

### Multiple installations

The initial GitHub App setup will only install the App on the organization or user account that you registered it with. If your code is spread across multiple organizations or user accounts, you will need to create additional installations for each namespace that you want Batch Changes to be able to sign commits in.

By default, Sourcegraph creates a private GitHub App, which only allows the App to be installed on the same organization or user account that it was created in. If you did not set the App to public visibility during creation, you will need to [change the visibility](https://docs.github.com/en/apps/maintaining-github-apps/modifying-a-github-app#changing-the-visibility-of-a-github-app) to public before you can install it in other namespaces. For security considerations, see [GitHub's documentation on private vs public apps](https://docs.github.com/en/apps/creating-github-apps/setting-up-a-github-app/making-a-github-app-public-or-private).

Once public, App can be installed in additional namespaces either from Sourcegraph or from GitHub.

#### Installing from Sourcegraph

1. Go to **Site admin > Batch Changes > Settings** and click **Edit** on the App you want to install in another namespace. You'll be taken to the App details page.

<img alt="The GitHub App details page on Sourcegraph" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-details-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App details page on Sourcegraph" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-details-batches-dark.png" class="screenshot theme-dark-only" />

2. Click **Add installation**. You will be redirected to GitHub to pick which other organization to install the App on and finish the installation process.

    > NOTE: Only [organization owners](https://docs.github.com/en/organizations/managing-peoples-access-to-your-organization-with-roles/roles-in-an-organization#organization-owners) can install GitHub Apps on an organization. If you are not an owner, you will need to ask an owner to install the App for you.

<img alt="The GitHub App installation page, with a list of namespaces to select from" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-multi-install-gh-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App installation page, with a list of namespaces to select from" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-multi-install-gh-batches-dark.png" class="screenshot theme-dark-only" />

3. As before, you will be asked to review the App permissions and select which repositories the App can access before installing it in a namespace. Once you click **Install** and the setup completes, you will be redirected back to Sourcegraph. You will now be able to push signed commits for repositories in this namespace.

#### Installing from GitHub

1. Go to the GitHub App page. You can get here easily from Sourcegraph by clicking **View in GitHub** for the App you want to install in another namespace.
2. Click **Configure**, or go to **App settings > Install App**, and select the organization or user account you want to install the App on.
3. As before, you will be asked to review the App permissions and select which repositories the App can access before installing it in a namespace. Once you click **Install** and the setup completes, you will be redirected back to Sourcegraph.
4. GitHub App installations will be automatically synced in the background. Return to **Site admin > Batch Changes > Settings** and click **Edit** on the App you added the new installation for. You'll be taken to the App details page. Once synced, you will see the new installation listed, and you will be able to push signed commits for repositories in this namespace.

<img alt="The GitHub App details page on Sourcegraph, scrolled to show a second new installation" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-post-multi-install-batches.png" class="screenshot theme-light-only" />
<img alt="The GitHub App details page on Sourcegraph, scrolled to show a second new installation" src="https://sourcegraphstatic.com/docs/images/administration/config/github-apps/github-app-post-multi-install-batches-dark.png" class="screenshot theme-dark-only" />

### Uninstalling an App

You can uninstall a GitHub App from a namespace or remove it altogether at any time.

To remove an installation in a single namespace, click **View in GitHub** for the installation you want to remove. If you are able to administer Apps in this namespace, you will see **Uninstall "[APP NAME]"** in the "Danger zone" at the bottom of the page. Click **Uninstall** to remove the App from this namespace. Sourcegraph will periodically sync installations in the background. It may temporarily throw errors related to the missing installation until the sync completes. You can check the GitHub App details page to confirm the installation has been removed.

To remove an App entirely, go to **Site admin > Batch Changes > Settings** and click **Remove** for the App you want to remove. You will be prompted to confirm you want to remove the App from Sourcegraph. Once removed from the Sourcegraph side, Sourcegraph will no longer communicate with your GitHub instance via the App unless explicitly reconnected. However, the App will still exist on GitHub unless manually deleted there, as well.

### GitHub App token use

Batch Changes uses the tokens from GitHub Apps in the following ways:

#### Installation access tokens

Installation access tokens are short-lived, non-refreshable tokens that give Sourcegraph access to the repositories the GitHub App has been given access to. Sourcegraph uses these tokens to read and write commits to repository branches. These tokens expire after 1 hour.

### Rejecting Unverified Commits

<span class="badge badge-note">Sourcegraph 5.2.4+</span>

Admins can configure Batch Changes to error when it creates commits that are not signed. This can be done by enabling the `batchChanges.rejectUnverifiedCommits` setting in the site configuration:

```json
{
  ...,
  "batchChanges.rejectUnverifiedCommits": true
}
```

## Custom Certificates

<span class="badge badge-note">Sourcegraph 5.1.5+</span>

If you are using a self-signed certificate for your GitHub Enterprise instance, configure `tls.external` under `experimentalFeatures`
in the **Site configuration** with your certificate(s).

```json
{
  "experimentalFeatures": {
    "tls.external": {
      "certificates": [
        "-----BEGIN CERTIFICATE-----\n..."
      ]
    }
  }
}
```

## Ownership

When a user is deleted, their Batch Changes become inaccessible in the UI but the data is not permanently deleted.
This allows recovering the Batch Changes if the user is restored.

However, if the user deletion is permanent, deleting both account and data, then the associated Batch Changes are also permanently deleted from the database. This frees storage space and removes dangling references.
