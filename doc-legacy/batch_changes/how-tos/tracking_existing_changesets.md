# Tracking existing changesets

Batch Changes allow you not only to [publish changesets](publishing_changesets.md) but also to **import and track changesets** that already exist on different code hosts. That allows you to get an overview of the status of multiple changesets, with the ability to filter and drill down into the details of a specific changeset.

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/tracking_existing_changesets_overview.png" class="screenshot center">

## Requirements

- Sourcegraph instance with repositories in it. See the "[Quickstart](../../index.md#quick-install)" guide on how to setup a Sourcegraph instance.
- A [global service account token](../how-tos/configuring_credentials.md#global-service-account-tokens) for Batch Changes (**a personal access token cannot currently be used for importing changesets**)

## Importing changesets into a batch change

To track existing changesets in a batch change you add them to the [batch spec](../explanations/introduction_to_batch_changes.md#batch-spec) under the `importChangesets` property and apply the batch spec.

The following example batch spec tracks multiple existing changesets in different repositories on different code hosts:

```yaml
name: track-important-milestone
description: Track all changesets related to our important milestone

importChangesets:
- repository: github.com/sourcegraph/sourcegraph
  externalIDs: [15397, 15590, 15597, 15583, 15806, 15798]
- repository: github.com/sourcegraph/src-cli
  externalIDs: [378, 373, 374, 369, 368, 361, 380]
- repository: bitbucket.sgdev.org/SOUR/vegeta
  externalIDs: [8]
- repository: gitlab.sgdev.org/sourcegraph/src-cli
  externalIDs: [113, 119]
```

See "[Creating a batch change](creating_a_batch_change.md)" on how to create a batch change from the batch spec.

> NOTE: You can combine the tracking of existing changesets and creating new ones by adding `importChangesets:` to your batch specs that have `on:`, `steps:` and `changesetTemplate:` properties.

Once you've created the batch change you'll see the existing changeset show up in the list of changesets. The batch change will track the changeset's status and include it in the overall batch change progress (in the same way as if it had been created by the batch change):

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/tracking_existing_changesets_burndown_chart.png" class="screenshot center">
