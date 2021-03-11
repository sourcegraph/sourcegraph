# Refactor Go code using Comby

<style>
.markdown-body pre.chroma {
  font-size: 0.75em;
}
</style>

### Introduction

This batch change uses Sourcegraph's [structural search](../../code_search/reference/structural.md) and [Comby](https://comby.dev/) to rewrite Go statements from

```go
fmt.Sprintf("%d", number)
```

to

```go
strconv.Itoa(number)
```

The statements are semantically equivalent, but the latter is clearer.

Since the replacements could require importing the `strconv` package, it uses [`goimports`](https://godoc.org/golang.org/x/tools/cmd/goimports) to update the list of imported packages in all `*.go` files.

### Prerequisites

We recommend that use the latest version of Sourcegraph when working with Batch Changes and that you have a basic understanding of how to create batch specs and run them. See the following documents for more information:

1. ["Quickstart"](../quickstart.md)
1. ["Introduction to Batch Changes"](../explanations/introduction_to_batch_changes.md)

### Create the batch spec

Save the following batch spec YAML as `sprintf-to-itoa.batch.yaml`:

```yaml
name: sprintf-to-itoa
description: |
  This batch change uses [Comby](https://comby.dev) to replace `fmt.Sprintf` calls
  in Go code with the equivalent but clearer `strconv.Iota` call.

on:
  - repositoriesMatchingQuery: lang:go fmt.Sprintf("%d", :[v]) patterntype:structural

steps:
  - run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' .go -matcher .go -exclude-dir .,vendor
    container: comby/comby
  - run: goimports -w $(find . -type f -name '*.go' -not -path "./vendor/*")
    container: unibeautify/goimports

changesetTemplate:
  title: Replace fmt.Sprintf with equivalent strconv.Itoa
  body: This batch change replaces `fmt.Sprintf` with `strconv.Itoa`
  branch: batch-changes/sprintf-to-itoa
  commit:
    message: Replacing fmt.Sprintf with strconv.Iota
  published: false
```

### Create the batch change

1. In your terminal, run this command:

    <pre>src batch preview -f sprintf-to-itoa.batch.yaml</pre>
1. Wait for it to run and compute the changes for each repository.
1. Open the preview URL that the command printed out.
1. Examine the preview. Confirm that the changesets are the ones you intended to track. If not, edit the batch spec and then rerun the command above.
1. Click the **Apply spec** button to create the batch change.
1. Feel free to then publish the changesets (i.e. create pull requests and merge requests) by [modifying the `published` attribute in the batch spec](../references/batch_spec_yaml_reference.md#changesettemplate-published) and re-running the `src batch preview` command.
