# Code Graph Data uploads

<p class="subtitle">How Code Graph data is uploaded and used.</p>

<style>
img.screenshot {
  display: block;
  margin: 1em auto;
  max-width: 600px;
  margin-bottom: 0.5em;
  border: 1px solid lightgrey;
  border-radius: 10px;
}

img.terminal-screenshot {
  max-width: 800px;
}
</style>

[Code graph indexers](../references/indexers.md) analyze source code and generate an index file, which is subsequently [uploaded to a Sourcegraph instance](../how-to/index_other_languages.md#4-upload-lsif-data) using [Sourcegraph CLI](../../cli/index.md) for processing. Once processed, this data becomes available for [precise code navigation queries](precise_code_navigation.md).

## Lifecycle of an upload

Uploaded index files are processed asynchronously from a queue. Each upload has an attached `state` that can change as work associated with that data is performed. The following diagram shows the possible transition paths from one `state` of an upload to another.

![Upload state diagram](./diagrams/upload-states.svg)

The typical sequence for a successful upload is: `UPLOADING_INDEX`, `QUEUED_FOR_PROCESSING`, `PROCESSING`, and `COMPLETED`.

In some cases, the processing of an index file may fail due to issues such as malformed input or transient network errors. When this happens, an upload enters the `PROCESSING_ERRORED` state. Such error uploads may undergo multiple retry attempts before moving into a permanent error state.

At any point, an uploaded record may be deleted. This can happen due to various reasons, such as being replaced by a newer upload, due to the age of the upload record, or by explicit deletion initiated by the user. When deleting a record that could be used for code navigation queries, it transitions first into the `DELETING` state. This temporary state allows Sourcegraph to manage the set of Code Graph uploads smoothly.

Changing the state of an upload to or from `COMPLETED` requires updating the [repository commit graph](#repository-commit-graph). This process can be computationally expensive for the worker service or Postgres database.

## Lifecycle of an upload (via UI)

After successfully uploading an index file, the Sourcegraph CLI will provide a URL on the target instance to track the progress of that upload.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/sg-3.34/uploads/src-lsif-upload.gif" class="screenshot terminal-screenshot" alt="Uploading a code graph index via the Sourcegraph CLI">

You can view the data uploads for a specific repository by navigating to the Code Graph page on the target repository's index page.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/sg-3.33/repository-page.png" class="screenshot" alt="Repository index page">

Alternatively, website administrators of a Sourcegraph instance can access a global view of Code Graph data uploads across all repositories from the **Site Admin** page.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/indexes-list.png" class="screenshot" alt="Global list of code graphd data uploads across all repositories">

## Repository commit graph

Sourcegraph maintains a mapping from a commit of a repository to the set of upload records that can resolve a query for that commit. When an upload record transitions to or from the `COMPLETED` state, the set of eligible uploads changes, and this mapping must be recalculated.

Upon a state change in an upload, we flag the repository as needing an update. Subsequently, the worker service updates the commit graph and asynchronously clears the flag for that repository.

When an upload changes state, the repository is flagged as requiring an update status. Then the [`worker` service](https://docs.sourcegraph.com/admin/workers#codeintel-commitgraph)
will update the commit graph and unset the flag for that repository asynchronously.

While this flag is set, the repository's commit graph is considered `stale`. This means there may be some upload records in a `COMPLETED` state that aren't yet used to resolve code navigation queries.

The state of a repository's commit graph can be seen in the code graph data page on the target repository's index page.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/stale-commit-graph.png" class="screenshot" alt="Stale repository commit graph notice">

Once the commit graph has updated (and no subsequent changes to that repository's uploads have occurred), the repository commit graph is no longer considered `stale`.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/code-intelligence/renamed/fresh-commit-graph.png" class="screenshot" alt="Up-to-date repository commit graph notice">
