# Updating base images in Dockerfiles

<style>
.markdown-body pre.chroma {
  font-size: 0.75em;
}
</style>

<p class="lead">
Create a batch change to update Dockerfiles in every one of your repositories.
</p>

### Introduction

Batch Changes allow us to update the base images used in our Dockerfiles, across many repositories, in just a few commands.

This tutorial shows you how to create [a batch spec](../explanations/introduction_to_batch_changes.md#batch-spec) that

1. finds `Dockerfile`s that make use of `google/dart:2.x` base images and
2. changes those `Dockerfiles` to use `google/dart:2.10`

The batch spec and instructions here can [easily be adapted to update other base images](#updating-other-base-images).

<img src="https://sourcegraphstatic.com/docs/images/batch_changes/update_base_images_in_dockerfiles_teaser.png" class="screenshot center">

### Prerequisites

We recommend using the latest version of Sourcegraph when working with Batch Changes and that you have a basic understanding of how to create batch specs and run them. See the following documents for more information:

1. ["Quickstart"](../quickstart.md)
1. ["Introduction to Batch Changes"](../explanations/introduction_to_batch_changes.md)

### Create the batch spec

Save the following batch spec YAML as `update-dart-base-images-2-10.batch.yaml`:

```yaml
name: update-dart-base-images-2-10
description: This batch change updates `google/dart:2.*` base images in Dockerfiles to `google/dart:2.10.2`.

on:
  # Find all repositories that contain Dockerfiles with `FROM google/dart:2.*` as base images.
  # The regexp used here matches images
  #
  #   google/dart:2.MINOR.PATCH
  #   google/dart:2.MINOR.PATCH-dev.DEVMINOR.DEVPATCH
  #
  #   google/dart-runtime:2.MINOR.PATCH
  #   google/dart-runtime:2.MINOR.PATCH-dev.DEVMINOR.DEVPATCH
  #
  # where the tag is < 2.10. Feel free to adjust it to your requirements.
  - repositoriesMatchingQuery: ^FROM google\/dart(-runtime)?:2\.[0-9]\.?\d?(-dev\.\d\.\d)? file:Dockerfile  patternType:regexp

# In each repository
steps:
  # find all Dockerfiles and replace the old image tags with our desired ones:
  - run: |
      find . -name Dockerfile -type f |\
      xargs sed\
        -i\
        --regexp-extended\
        's/FROM google\/dart(-runtime)?:2\.[[:digit:]]\.?[[:digit:]]?(-dev\.?[[:digit:]]?\.?[[:digit:]]?)?/FROM google\/dart:2\.10/g'
    container: alpine:3

# Describe the changeset (e.g., GitHub pull request) you want for each repository.
changesetTemplate:
  title: Update google/dart base images in Dockerfiles to 2.10.2
  body: This updates google/dart base images used in Dockerfiles to version 2.10.2
  branch: batch-changes/google-dart-2-10-2-base-images # Push the commit to this branch.
  commit:
    message: Update google/dart base images in Dockerfiles to 2.10.2
  published: false
```

### Create the batch change

1. In your terminal, run this command:

    <pre>src batch preview -f update-dart-base-images-2-10.batch.yaml</pre>
1. Wait for it to run and compute the changes for each repository.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/update_base_images_in_dockerfiles_wait_run.png" class="screenshot">
1. Open the preview URL that the command printed out.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/update_base_images_in_dockerfiles_click_url.png" class="screenshot">
1. Examine the preview. Confirm that the changesets are the ones you intended to track. If not, edit the batch spec and then rerun the command above.
    <img src="https://sourcegraphstatic.com/docs/images/batch_changes/update_base_images_in_dockerfiles_preview.png" class="screenshot">
1. Click the **Apply** button to create the batch change.
1. Feel free to then publish the changesets (i.e. create pull requests and merge requests) by [modifying the `published` attribute in the batch spec](../references/batch_spec_yaml_reference.md#changesettemplate-published) and re-running the `src batch preview` command.

### Updating other base images

In order to update other base images in `Dockerfile`s you can either extend the batch spec or create another batch spec in which you change the `repositoriesMatchingQuery` and the `steps.run` properties.

You can keep using regexp-based search and `sed`, or you can use [structural search](../../code_search/reference/structural.md) combined with [comby](https://comby.dev) to update base images.

For example, here's how you would update `alpine` base images from `alpine:3.9`, `alpine:3.10`, `alpine:3.11` to `alpine:3.12`:

```yaml
# [...]

on:
  - repositoriesMatchingQuery: FROM alpine:3.:[minor~[91]+[0-1]?] file:Dockerfile patternType:structural
  - repositoriesMatchingQuery: FROM alpine:3.:[minor~[91]+[0-1]?]@sha256::[hash~[a-f0-9]+] file:Dockerfile patternType:structural

steps:
  - run: |
      comby \
        -in-place \
        'FROM alpine:3.:[minor~[91]+[0-1]?]@sha256::[hash~[a-f0-9]+]' \
        'FROM alpine:3.12@ sha256:c0e9560cda118f9ec63ddefb4a173a2b2a0347082d7dff7dc14272e7841a5b5a' \
        Dockerfile
    container: comby/comby
  - run: comby -in-place 'alpine:3.:[minor~[91]+[0-1]?]' 'FROM alpine:3.12' Dockerfile
    container: comby/comby

# [...]
```

And here's how you could update `golang` base images to use Go 1.15:

```yaml
# [...]
on:
  - repositoriesMatchingQuery: FROM golang:1.1:[minor~[0-4]]-alpine file:Dockerfile patternType:structural
  - repositoriesMatchingQuery: FROM golang:1.1:[minor~[0-4]]-alpine@sha256::[hash~[a-f0-9]+] file:Dockerfile patternType:structural

steps:
  - run: |
      comby \
        -in-place \
        'FROM golang::[version]-alpine@sha256::[hash~[a-f0-9]+]' \
        'FROM golang:1.15-alpine@sha256:df0119b970c8e5e9f0f5c40f6b55edddf616bab2b911927ebc3b361c469ea29c' \
        Dockerfile
    container: comby/comby
  - run: comby -in-place 'FROM golang::[version]-alpine' 'FROM golang:1.15-alpine' Dockerfile
    container: comby/comby
```
