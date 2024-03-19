# Creating changesets per project in monorepos

<style>
.markdown-body h2 { margin-top: 50px; }
.markdown-body pre.chroma { font-size: 0.75em; }
</style>

## Overview

Large repositories often contain multiple projects, making them so-called monorepos. It can make sense to run the batch spec [`steps`][steps] separately in each project and create one changeset per project.

That can be done by using [`workspaces`][workspaces] in the batch specs in two steps:

1. Define the project locations with the `workspaces` property
2. Produce unique `changesetTemplate.branch` names

## 1. Define project locations with `workspaces`

Let's say we have a repository containing multiple TypeScript projects in which we want to update TypeScript by running the following command:

```
npm update typescript
```

The repository has the following directory and file structure:

```
README.md
project1/package.json
project1/src/...
project2/package.json
project2/src/...
examples/project3/package.json
examples/project3/src/...
```

The location of the `package.json` files tell us that the TypeScript projects are in `project1`, `project2` and `examples/project3`. In each of these we to run the `npm update` command and produce an individual changeset per project.

The [`workspaces`][workspaces] property in batch specs allows us to do that:

```yaml
name: update-typescript-monorepo
description: This batch change updates the TypeScript dependency to the latest version

on:
  - repositoriesMatchingQuery: our-large-monorepo

workspaces:
  - rootAtLocationOf: package.json
    in: github.com/our-org/our-large-monorepo

steps:
  - run: npm update typescript
    container: node:14

# [...]
```

The `workspaces` property here defines that in `github.com/our-org/our-large-monorepo` different `workspaces` exist and contain a `package.json` at their root.

When executed with `src batch [apply|preview]` this would produce up to 3 changesets in `github.com/our-org/our-large-monorepo`, one for each project.

## 2. Produce unique `changesetTemplate.branch` names

Since changesets are uniquely identified by their repository and branch, we **must** ensure that multiple changesets in the same repository will different branches.

To do that, we make use of [templating][templating] in the [`changesetTemplate.branch`][branch] field

```yaml
# [...]
changesetTemplate:
  title: Update TypeScript
  body: This updates TypeScript to the latest version
  published: false
  commit:
    message: Update TypeScript
  # Templating and helper functions allow us to get the `path` in which
  # the `steps` executed and turn that into a branch name:
  branch: batch-changes/update-typescript-${{ replace steps.path "/" "-" }}
```

The `steps.path` [templating variable][templating] contains the path in which the `steps` were executed, relative to the root of the repository.

With the file and directory structure above, that means we'd end up with the following branch names:

- `batch-changes/update-typescript-project1`
- `batch-changes/update-typescript-project2`
- `batch-changes/update-typescript-examples-project3`

And with that, we're done and ready to produce multiple changesets in a single repository, with the full batch spec looking like this:

```yaml
name: update-typescript-monorepo
description: This batch change updates the TypeScript dependency to the latest version

on:
  - repository: github.com/sourcegraph/automation-testing

workspaces:
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/automation-testing

steps:
  - run: npm update typescript
    container: node:14

changesetTemplate:
  title: Update TypeScript
  body: This updates TypeScript to the latest version
  branch: batch-changes/update-typescript-${{ replace steps.path "/" "-" }}
  commit:
    message: Update TypeScript
  published: false
```

Now we only need to run `src batch [apply|preview]` to execute our batch spec.

## Dynamic discovery of workspaces

The `workspace` property leverages Sourcegraph search to find the location of the defined workspaces in the repositories yielded by the [`on`][on] property of the batch spec.

That has the advantage that it's _dynamic_: whenever `src batch [apply|preview]` is re-executed, Sourcegraph search is used again to find workspaces, automatically picking up new ones and removing workspaces that no longer exist.

## Only downloading workspace data in large repositories

If the repository containing the workspaces is really large and it's not feasible to download to make it available for the `steps` execution, the [`workspaces.onlyFetchWorkspaces`][onlyFetchWorkspaces] field can be set to `true` to only download the workspaces, without the rest of the repository.

## Learn more

To learn more about `workspaces`, take a look at its entry in the "[batch spec YAML reference][workspaces]".

<!-- References for easier reading of text above: -->

[cli]: ../cli/index.md
[steps]: ../references/batch_spec_yaml_reference.md#steps
[workspaces]: ../references/batch_spec_yaml_reference.md#workspaces
[onlyfetchworkspace]: ../references/batch_spec_yaml_reference.md#workspaces-onlyfetchworkspace
[on]: ../references/batch_spec_yaml_reference.md#on
[branch]: ../references/batch_spec_yaml_reference.md#changesettemplate-branch
[templating]: ../references/batch_spec_templating.md
