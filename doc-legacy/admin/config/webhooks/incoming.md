# Incoming webhooks

Incoming webhooks can be configured on a Sourcegraph instance in order to receive webhook events from code hosts. This allows Sourcegraph to react more quickly to events that occur outside the instance instead of polling for changes.

Webhooks are currently implemented to speed up two types of external events:

* Keeping batch changes changeset details up to date
* Keeping code on Sourcegraph fresh by responding to new code being pushed to a repository

See the table below for code host compatibility:

Code host | [Batch changes](../../../batch_changes/index.md) | Code push | User permissions
--------- | :-: | :-: | :-:
GitHub | 🟢 | 🟢 | 🟢
GitLab | 🟢 | 🟢 | 🔴
Bitbucket Server / Datacenter | 🟢 | 🟢 | 🔴
Bitbucket Cloud | 🟢 | 🟢 | 🔴
Azure DevOps | 🟢 | 🔴 | 🔴

To receive webhooks both Sourcegraph and the code host need to be configured. To configure Sourcegraph, [add an incoming webhook](#adding-an-incoming-webhook). Then [configure webhooks on your code host](#configuring-webhooks-on-the-code-host)

## Adding an incoming webhook

Before adding an incoming webhook you should ensure that you have at least one [code host connection](../../external_services/index.md) configured.

The incoming webhook will be configured to accept events from a specific code host connection based on its type and URN.

1. Navigate to **Site Admin > Configuration > Incoming webhooks**
   ![Incoming webhooks page](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/incoming-webhooks-page.png)
2. Click **+ Create webhook**
   ![Adding an incoming webhook](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/adding-webhook.png)
3. Fill out the form:
   1. **Webhook name**: Descriptive name for the webhook.
   1. **Code host type**: Select from the dropdown. This will be filtered based on code host connections added on your instance.
   1. **Code host URN**: The URN for the code host. Again, this will be filtered by code host connections added on your instance.
   1. **Secret**: An arbitrary shared secret between Sourcegraph and the code host. A default value is provided, but you are free to change it.
       > NOTE: Secrets are not supported for Bitbucket cloud
4. Click **Create**

The incoming webhook will now be created, and you will be redirected to a page showing more details.
![Created webhook](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/webhook-created.png)

Use the unique URL present on the details page to configure [the webhook on your code host](#configuring-webhooks-on-the-code-host).

## Configuring webhooks on the code host

The instructions for setting up webhooks on the code host are specific to each code host type.

### GitHub

#### Batch changes

1. Copy the webhook URL displayed after adding the incoming webhook as mentioned [above](#adding-an-incoming-webhook)
1. On GitHub, go to the settings page of your organization. From there, click **Settings**, then **Webhooks**, then **Add webhook**.
1. Fill in the webhook form:
   * **Payload URL**: the URL you copied above from Sourcegraph.
   * **Content type**: this must be set to `application/json`.
   * **Secret**: the secret token you configured Sourcegraph to use above.
   * **Which events**: select **Let me select individual events**, and then enable:
     - Issue comments
     - Pull requests
     - Pull request reviews
     - Pull request review comments
     - Check runs
     - Check suites
     - Statuses
   * **Active**: ensure this is enabled.
1. Click **Add webhook**.
1. Confirm that the new webhook is listed.

Done! Sourcegraph will now receive webhook events from GitHub and use them to sync pull request events, used by [batch changes](../../../batch_changes/index.md), faster and more efficiently.

#### Code push

Follow the same steps as above, but ensure you include the `push` event under **Let me select individual events**

#### Repository permissions

Follow the same steps as above, but ensure you include the following events under **Let me select individual events**:
- `Collaborator add, remove, or changed`
- `Memberships`
- `Organizations`
- `Repositories`
- `Teams`

When one of these events occur, a permissions sync will trigger for the relevant user or repository.

> NOTE: Permission changes can take a few seconds to reflect on GitHub. To prevent syncing permissions before the change reflects on GitHub, the permissions sync will only occur 10 seconds after the relevant event is received.

### GitLab

#### Batch changes

1. Copy the webhook URL displayed after adding the incoming webhook as mentioned [above](#adding-an-incoming-webhook)
1. On GitLab, go to your project, and then **Settings > Webhooks** (or **Settings > Integration** on older GitLab versions that don't have the **Webhooks** option).
1. Fill in the webhook form:
   * **URL**: the URL you copied above from Sourcegraph.
   * **Secret token**: the secret token you configured Sourcegraph to use above.
   * **Trigger**: select **Merge request events** and **Pipeline events**.
   * **Enable SSL verification**: ensure this is enabled if you have configured SSL with a valid certificate in your Sourcegraph instance.
1. Click **Add webhook**.
1. Confirm that the new webhook is listed below **Project Hooks**.

Done! Sourcegraph will now receive webhook events from GitLab and use them to sync merge request events, used by [batch changes](../../../batch_changes/index.md), faster and more efficiently.

**NOTE:** We currently do not support [system webhooks](https://docs.gitlab.com/ee/administration/system_hooks.html) as these provide a different set of payloads.

#### Code push

Follow the same steps as above, but ensure you include the `Push events` trigger.

### Bitbucket server

#### Batch changes

The [Sourcegraph Bitbucket Server plugin](../../../integration/bitbucket_server.md#sourcegraph-bitbucket-server-plugin) enables the Bitbucket Server / Bitbucket Data Center instance to send webhooks to Sourcegraph.

1. Install the [Sourcegraph Bitbucket Server plugin](../../../integration/bitbucket_server.md#sourcegraph-bitbucket-server-plugin) on your Bitbucket Server / Bitbucket Data Center instance.
1. On your Bitbucket Server / Bitbucket Data Center instance, go to **Administration > Add-ons > Sourcegraph**
1. Fill in the **Add a webhook** form
   * **Name**: A unique name representing your Sourcegraph instance.
   * **Scope**: `global`.
   * **Endpoint**: The URL found after creating an incoming webhook.
   * **Events**: `repo:build_status`, `pr:activity:status`, `pr:activity:event`, `pr:activity:rescope`, `pr:activity:merge`, `pr:activity:comment`, `pr:activity:reviewers`, `pr:participant:status`
   * **Secret**: The secret you configured when creating the incoming webhook.
1. Confirm that the new webhook is listed under **All webhooks** with a timestamp in the **Last successful** column.

Done! Sourcegraph will now receive webhook events from Bitbucket Server / Bitbucket Data Center and use them to sync pull request events, used by [batch changes](../../../batch_changes/index.md), faster and more efficiently.

#### Code push

Follow the same steps as above, but ensure you tick the `Push` option. If asked for a specific event, use `repo:refs_changed`.

### Bitbucket cloud

#### Batch changes

> NOTE: Experimental webhook support for Bitbucket Cloud was added in Sourcegraph 3.40. Please <a href="https://sourcegraph.com/contact">contact us</a> with any issues found while using webhooks.

1. On Bitbucket Cloud, go to each repository, and then **Repository settings > Webhooks**.
1. Click **Add webhook**.
1. Fill in the webhook form:
   * **Title**: Any title.
   * **URL**: The URL found after creating an incoming webhook.
   * **Triggers**: Select **Build status created** and **Build status updated** under **Repository**, and every item under **Pull request**.
1. Click **Save**.
1. Confirm that the new webhook is listed below **Repository hooks**.

Done! Sourcegraph will now receive webhook events from Bitbucket Cloud and use them to sync pull request events, used by [batch changes](../../../batch_changes/index.md), faster and more efficiently.

#### Code push

Follow the same steps as above, but ensure you tick the `Push` option.

### Azure DevOps

#### Batch changes

> NOTE: Experimental webhook support for Azure DevOps was added in Sourcegraph 5.0, and does not currently support secrets. Please <a href="https://sourcegraph.com/contact">contact us</a> with any issues found while using webhooks.

1. On Azure DevOps, go to each project, and then **Project settings > General > Service hooks**.
2. Click **Create subscription**.
   ![](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/webhook-step-2.png)
3. Select **Web Hooks** and click **Next** .
   ![](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/webhook-step-3.png)
4. From the **Trigger on this type of event** drop-down, choose: **Pull request updated**.
   ![](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/webhook-step-4.png)
5. Set the filters how you like, or leave them at the default: **[Any]** and click **Next**.
6. Fill in the webhook form:
   - **URL**: The URL found after creating an incoming webhook.
   - Leave the rest of the fields on the default values.
7. Click **Test** to verify the webhook works. Then click **Finish**.
8. Repeat the steps above, this time choosing **Pull request merged** as your event type.

Done! Sourcegraph will now receive webhook events from Azure DevOps and use them to sync pull request events, used by [batch changes](../../../batch_changes/index.md), faster and more efficiently.

## Webhook logging

Sourcegraph can track incoming webhooks from code hosts to more easily debug issues with webhook delivery. These webhooks can be viewed in two places depending on how they were added:

1. Via **Site Admin > Configuration > Incoming webhooks**
   ![Webhook logs](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/webhook-logs.png)
2. **Deprecated** Via code host connection: **Site Admin > Batch Changes > Incoming webhooks**
   ![Legacy webhook logs](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/webhook-logs-legacy.png)

By default, sites without [database encryption](../encryption.md) enabled will retain three days of webhook logs. Sites with encryption will not retain webhook logs by default, as webhooks may include sensitive information; these sites can enable webhook logging and optionally configure encryption for them by using the settings below.

### Enabling webhook logging

Webhook logging is controlled by the `webhook.logging` site configuration
option. This option is an object with the following keys:

| Key         | Type      | Default                                                                                                               | Description                                                 |
|-------------|-----------|-----------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------|
| `enabled`   | `boolean` | If `true`, incoming webhooks will be stored.                                                                          | `true` if no site encryption is enabled; `false` otherwise. |
| `retention` | `string`  | The length of time to retain the webhooks, expressed as a valid [Go duration](https://pkg.go.dev/time#ParseDuration). | `72h`                                                       |

#### Examples

To disable webhook logging:

```json
{
  "webhook.logging": {"enabled": false}
}
```

To retain webhook logs for one day:

```json
{
  "webhook.logging": {
    "enabled": false,
    "retention": "24h"
  }
}
```

### Encrypting webhook logs

Webhook logs can be encrypted by specifying a `webhookLogKey` in the [on-disk database encryption site configuration](../encryption.md).

## Deprecation notice

As of Sourcegraph 4.3.0 webhooks added via code host configuration are deprecated and support will be removed in release 5.1.0.

This includes any webhooks pointed at URLs starting with the following:

* `.api/github-webhooks`
* `.api/gitlab-webhooks`
* `.api/bitbucket-server-webhooks`
* `.api/bitbucket-cloud-webhooks`

In order to continue using webhooks you need to follow the steps below to [add an incoming webhook](#adding-an-incoming-webhook) and then update the webhook configured on your code host with the new webhook url which will look something like `https://sourcegraph-instance/.api/webhooks/{UUID}`
