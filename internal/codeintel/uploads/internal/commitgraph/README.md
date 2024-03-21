# commitgraph

The problem this module is solving: Given a repository id and a commit hash, find me the set of index uploads most relevant to that commit.

We can't just look for uploads at the given commit, because there are various reasons for why a commit might not have associated indexes uploaded

- It is a monorepo moving at high commit velocity relative to indexing speed
- It's not covered by any indexing policy. For example, some customers only index commits on the `main` branch, or on release tags.
- The upload is in a queued, processing, or errored/failed state.
- The index was deleted, by a retention policy or an explicit user action.

To solve this problem we build an annotated copy of the git commit graph in Postgres, so that we can return so called "visibleUploads" quickly.

## The annotated commit graph

A fully annotated commit graph contains a set of visible uploads for every commit. We can never see multiple visible uploads from the same indexer _and_ root at the same commit.

Conceptually:

`commitgraph : Map<Commit, Map<(Indexer, Root), (UploadId, Distance)>>`

Here's what an example entry for a commit `deadbeef` might look like for our conceptual model:

```go
commitgraph = {
  "deadbeef" => {
    ("scip-go", "backend/") => (UploadId(23), 0),
    ("scip-go", "worker/") => (UploadId(24), 1),
    ("scip-typescript", "frontend/") => (UploadId(33), 1),
  }
}
```

We'll look at a couple of examples with increasing complexity to understand what constitutes visibility.

### Visibility rules

Our examples will be drawings of commit graphs.

We will use `I1`..`In` to denote indexer+root combinations or "index keys". For example `I1` could correspond to indexes created by `"scip-go"` in the `"backend/"` root directory.

We will use single uppercase letters in boxes with rounded corners to denote commits.
Colored boxes mark commits with uploads, and black boxes those with no uploads.
A commit with multiple uploads will have additional colored outlines.

Boxes with sharp corners record the set of visible uploads for a commit. An entry like `I1 : A(3)` means this commit sees an upload for `I1` made at commit `A` at a depth of 3.

![commitgraph_0.png](https://storage.googleapis.com/sourcegraph-assets/dev-docs/commitgraph/commitgraph_0.png)

**Rule 1: If an index key is uploaded at a particular commit, that commit will consider it visible at depth 0**

For our first example we'll start with a graph of a single commit B, and indexes for both `I1` and `I2` have been uploaded for `B`.
According to our first rule both `I1` and `I2` are visible to B at depth 0.

**Rule 2: An upload is visible to a commit at depth N + 1 if it's visible to its parent at depth N**

![commitgraph_1.png](https://storage.googleapis.com/sourcegraph-assets/dev-docs/commitgraph/commitgraph_1.png)

Because there are no uploads at `A` and no uploads in `A`'s ancestors it does not have any visible uploads.
There are no uploads for `C`, but because there are uploads at `B` and `C` is its child, it has visibility onto both uploads at `B` at depth 1.

**Rule 3: Uploads for the same index key shadow uploads at greater depth**

![commitgraph_2.png](https://storage.googleapis.com/sourcegraph-assets/dev-docs/commitgraph/commitgraph_2.png)

In this example `C` now has an upload for `I1`, which means it sees the upload at depth 0, rather than the one from `B` at depth 1.

`E` sees the upload for `I2` at `D` at a distance of 1, rather than `B` because the upload at `B` has a distance of 2.
The notion of shadowing can be a bit unintuitive here, as `D` does _not_ need to be on the path from `B` to `E` in order to shadow `B`.

In practice, this means that if a codebase uses merge commits and also indexes commits not on the `main` branch, it is possible to fall back to uploads on commits that aren't on the `main` branch.

_Implementation note: If there are two uploads at the same depth we pick the smaller upload_id. This choice is arbitrary but deterministic._

### Linked commits

Because storing the full map of visible uploads per commit can be expensive, we store _links_ to ancestors were possible.
This is purely an optimization to reduce the amount of storage we use.

A commit is stored as a link iff:

- It does not have associated uploads
- It is not a merge commit
- It does not have a child that is a merge commit

For these commits we store the first ancestor commit that isn't a link, as well as the depth to that ancestor.
The visible uploads for those commits can then be computed by retrieving the visible uploads for the linked commit and increasing all their depths by the depth between the linked commits.

![commitgraph_3.png](https://storage.googleapis.com/sourcegraph-assets/dev-docs/commitgraph/commitgraph_3.png)

In this example both `B` and `C` are stored as links. `E` is not stored as a link, because its child `F` is a merge commit.

### Potential drawbacks

Purely relying on "commit depth" can cause outdated uploads to appear visible when using merge commits. For example:

![commitgraph_drawback.png](https://storage.googleapis.com/sourcegraph-assets/dev-docs/commitgraph/commitgraph_drawback.png)

`B` could be a documentation commit that was opened and forgotten for a while. Since then 1000 commits with various changes and uploads have happened.
When `B` is now merged back without rebasing, it creates a "portal" into the past that makes the upload at `A` shadow the one at `C`.
There's no immediate easy solution for this, but it's worth considering as a culprit if we're seeing outdated uploads used for code navigation.

## Architecture & operations

At a high level the annotated commit graph is kept up-to-date by a worker called `commitGraphUpdater`, that runs periodically. We maintain a table `lsif_dirty_repositories` that tracks what repositories have had new indexes uploaded or old ones deleted.
When a repository is marked as dirty we update its annotated commit graph and remove its dirty flag.

### Loading data

For a repository marked as dirty we fetch the following data before computing the annotated commit graph:

- The full non-annotated commit graph from git server. All commits and their relationships
- Metadata for all uploaded indexes (indexer name, commit, root directory)

### Storage

- Commits and their visible uploads are stored in `lsif_nearest_uploads`
  - The visibility maps are stored as `jsonb` objects
- Linked commits are stored in `lsif_nearest_uploads_links`

These tables are updated via a diffing mechanism. We create temporary tables and insert the full graph into them.
We then run a query to compare the actual table and the temporary table and insert/delete/update individual rows.

### TODO: Queries for commits not yet in the annotated commit graph

When information is requested for an unknown commit in a repository we ask git server for a fragment of the commit graph that includes the commit and "graft" it onto our existing graph
