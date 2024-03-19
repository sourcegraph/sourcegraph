# Creating multiple changesets in large repositories

<style>
.markdown-body h2 { margin-top: 50px; }
.markdown-body pre.chroma { font-size: 0.75em; }
</style>

<aside class="beta">
<p>
<span class="badge badge-beta">beta</span>This feature is in beta and might change in the future.</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

## Overview

Batch changes can produce a lot of changes in a single repository. In order to make reviewing and merging the changes easier, it can be helpful to split the changes up into multiple changesets.

That can be done by using [`transformChanges`](../references/batch_spec_yaml_reference.md#transformchanges) in the batch spec to group the changes produced in one single repository by directory and create a changeset for each group.

> NOTE: In some monorepos it makes more sense to run the batch spec [`steps`][steps] _per project_. Take a look at "[Creating changesets per project in monorepos](./creating_changesets_per_project_in_monorepos.md)" to find out how to use the [`workspaces`][workspaces] property to do that.

## Using `transformChanges`

The following batch spec uses the `transformChanges` property to create up to 4 changesets in a single repository by grouping the changes made in different directories:

```yaml
name: hello-world
description: Add Hello World to READMEs

# Find all repositories that contain a README.md file.
on:
  - repositoriesMatchingQuery: file:README.md

# In each repository, run this command. Each repository's resulting diff is captured.
steps:
  - run: IFS=$'\n'; echo Hello World | tee -a $(find -name README.md)
    container: alpine:3

# Transform the changes produced in each repository.
transformChanges:
  # Group the file diffs by directory and produce one additional changeset per group.
  # Changes that haven't been grouped will be be in the standard changeset.
  group:
    - directory: client
      branch: hello-world-client # will replace the `branch` in the `changesetTemplate`
    - directory: docker-images
      # Optional: only apply the rule in this repository
      repository: github.com/sourcegraph/sourcegraph
      branch: hello-world-infra
    - directory: monitoring
      repository: github.com/sourcegraph/sourcegraph
      branch: hello-world-monitoring

# Describe the changeset (e.g., GitHub pull request) you want for each repository.
changesetTemplate:
  title: Hello World
  body: My first batch change!
  branch: hello-world # This branch is the default branch and will be
                      # overwritten for each additional changeset.
  commit:
    message: Append Hello World to all README.md files
  published: false # Do not publish any changes to the code hosts yet
```

This batch spec will produce up to 4 changesets in the `github.com/sourcegraph/sourcegraph` repository:

1. a changeset with the changes in the `client` directory
1. a changeset with the changes in `docker-images`
1. a changeset with the changes in `monitoring`
1. a changeset with the changes in the other directories.

Since code hosts and git don't allow creating multiple, _different_ changesets on the same branch, it is **required** to specify a unique `branch` for each `directory` that will be used for the additional changesets. That `branch` will overwrite the default branch specified in `changesetTemplate`.

In case no changes have been made in a `directory` specified in a `group`, no additional changeset will be produced.

If the optional `repository` property is specified only the changes in that repository will be grouped.

See the [batch spec YAML reference on `transformChanges`](../references/batch_spec_yaml_reference.md#transformchanges) for more details.

<!-- References for easier reading of text above: -->

[steps]: ../references/batch_spec_yaml_reference.md#steps
[workspaces]: ../references/batch_spec_yaml_reference.md#workspaces
