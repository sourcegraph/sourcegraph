# Outgoing webhooks

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> <strong>This feature is currently in beta.</strong>
</p>
</aside>

<span class="badge badge-note">Sourcegraph 5.0+</span>

Outgoing webhooks can be configured on a Sourcegraph instance in order to send Sourcegraph events to external tools and services. This allows for deeper integrations between Sourcegraph and other applications.

Currently, webhooks are only implemented for events related to [Batch Changes](../../../batch_changes/index.md). They also cannot yet be scoped to specific entities, meaning that they will be triggered for all events of the specified type across Sourcegraph. Expanded support for more event types and scoped events is planned for the future. Please [let us know](mailto:feedback@sourcegraph.com) what types of events you would like to see implemented next, or if you have any other feedback!

> WARNING: Outgoing webhooks have the potential to send sensitive information about your repositories and code to other untrusted services. When configuring outgoing webhooks, be sure to only send events to trusted service URLs and to use the shared secret to verify any requests received.

## Adding an outgoing webhook

1. Navigate to **Site Admin > Configuration > Ougoing webhooks**
   ![Outgoing webhooks page](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/outgoing-webhooks-page.png)
1. Click **+ Create webhook**
   ![Adding an outgoing webhook](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/adding-outgoing-webhook.png)
1. Fill out the form:
   1. **URL**: URL endpoint of the external service that Sourcegraph should send webhook events to.
   1. **Secret**: An arbitrary secret to share between Sourcegraph and the external service. A default value is provided, but you are free to change it.
   1. **Event types**: The types of [events](#supported-event-types) that will trigger a webhook event. Currently, only events related to Batch Changes are supported.
1. Click **Create**

The outgoing webhook will now be created and active. To view or edit its details, or to see the log of event requests that have been sent for it, click the **Edit** button on the outgoing webhook's row.
![Created webhook](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/webhooks/outgoing-webhook-details.png)

## Supported event types

### Batch change

- **batch_change:apply** - Triggered when a batch spec is applied to a batch change.
- **batch_change:close** - Triggered when a batch change is closed.
- **batch_change:delete** - Triggered when a batch change is deleted.

#### Example payload

The batch change webhook event payload mirrors the [GraphQL API](../../../api/graphql/index.md) `BatchChange` type and contains the following fields:

```json
{
  // The unique ID for the batch change.
  "id": "QmF0Y2hDaGFuZ2U6MTcz",
  // The ID of the namespace where this batch change is defined.
  "namespace_id": "VXNlcjox",
  // The name of the batch change.
  "name": "hello-world",
  // The description of the batch change (as Markdown).
  "description": "Add Hello World to READMEs",
  // The state of the batch change on Sourcegraph.
  "state": "OPEN",
  // The ID of the user who created this batch change.
  "creator_user_id": "VXNlcjox",
  // The ID of the user who last applied a spec to this batch change.
  "last_applier_user_id": "VXNlcjox",
  // The URL path on Sourcegraph for this batch change.
  "url": "/users/my-username/batch-changes/hello-world",
  // The date and time when the batch change was created.
  "created_at": "2023-03-19T05:41:24Z",
  // The date and time when the batch change was last updated.
  "updated_at": "2023-03-19T05:43:04Z",
  // The date and time when the batch change was last updated with a new spec.
  "last_applied_at": "2023-03-19T05:43:04Z",
  // The date and time when the batch change was closed, or null if it's still open.
  "closed_at": null
}
```

### Changeset

- **changeset:close** - Triggered when a changeset is closed or merged by Sourcegraph.
- **changeset:publish** - Triggered when a changeset is successfully published to the code host.
- **changeset:publish_error** - Triggered when an attempt to publish a changeset to the code host fails.
- **changeset:update** - Triggered when a changeset is updated on the code host by Sourcegraph.
- **changeset:update_error** - Triggered when an attempt to update a changeset on the code host fails.

#### Example payload

The changeset webhook event payload mirrors the [GraphQL API](../../../api/graphql/index.md) `ExternalChangeset` type and contains the following fields:

```json
{
  // The unique ID for the changeset.
  "id": "Q2hhbmdlc2V0OjI4MA==",
  // The external ID that uniquely identifies this ExternalChangeset on the code host (e.g. the pull request number). Note that this is only available after the changeset has been published.
  "external_id": "204",
  // The IDs of the batch changes that this changeset is associated with.
  "batch_change_ids": [
    "QmF0Y2hDaGFuZ2U6MTcz"
  ],
  // The ID (on Sourcegraph) of the repository that this changeset is associated with.
  "repository_id": "UmVwb3NpdG9yeToxNQ==",
  // The date and time when the batch change was created.
  "created_at": "2023-03-19T05:41:24Z",
  // The date and time when the batch change was last updated.
  "title": "Hello World",
  // The body of the changese (as Markdown).
  "body": "My first batch change!",
  // The username of the author of the changeset. Note that this is only available after the changeset has been published and is not available on some code hosts.
  "author_name": "my-username",
  // The email of the author of the changeset. Note that this is only available after the changeset has been published and is not available on most code hosts.
  "author_email": "me@myorganization.com",
  // The state of the changeset on Sourcegraph.
  "state": "OPEN",
  // Any labels attached to the changeset on the code host.
  "labels": ["bug"],
  // The external URL of the changeset on the code host. Note that this is only available after the changeset has been published.
  "external_url": "https://github.com/my-org/my-repo/pull/204",
  // If the changeset was opened from a fork, this is the namespace of the fork on the code host.
  "fork_namespace": "fork-username",
  // If the changeset was opened from a fork, this is the name of the fork repository. Note that this is only available after the changeset has been published.
  "fork_name": "my-repo-fork",
  // The review state of this changeset on the code host. Note that this is only available after the changeset has been published.
  "review_state": "CHANGES_REQUESTED",
  // The check state of this changeset on the code host. Note that this is only available after the changeset has been published.
  "check_state": "PASSED",
  // Any error that occurred when publishing or updating the changeset.
  "error": null,
  // Any error that occured during the last sync of the changeset by Sourcegraph.
  "syncer_error": null,
  // The ID of the batch change that produced this changeset.
  "owning_batch_change_id": "QmF0Y2hDaGFuZ2U6MTcz"
}
