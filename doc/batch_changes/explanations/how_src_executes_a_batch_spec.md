# How `src` executes a batch spec

This document is meant to help with debugging and troubleshooting the writing and execution of batch specs with [Sourcegraph CLI `src`](../../cli/index.md).

It explains what happens under the hood when a user uses applies or previews a batch spec by running `src batch apply` or `src batch preview`.

## Overview

`src batch apply` and `src batch preview` execute a batch spec the same way:

1. [Parse the batch spec](#parse-the-batch-spec)
1. [Resolve the namespace](#resolving-namespace)
1. [Prepare container images](#preparing-container-images)
1. [Resolve repositories](#resolving-repositories)
1. [Executing steps](#executing-steps)
1. [Sending changeset specs](#sending-changeset-specs)
1. [Sending batch spec](#sending-the-batch-spec)
1. [Preview or apply the batch spec](#preview-or-apply-the-batch-spec)

The difference is the last step: `src batch apply` _applies_ the batch spec where the `src batch preview` only prints a URL at which you can preview what would happen if you apply it.

The rest of the document explains each step in more detail.

## Parse the batch spec

`src` reads in, parses and validates the batch spec YAML specified with the `-f` flag.

It validates the batch spec against its [schema](https://github.com/sourcegraph/src-cli/blob/main/schema/batch_spec.schema.json) and does some semantic checks to make sure that, for example, `changesetTemplate` is specified if `steps` are specified, or that no feature is used that's not supported by the Sourcegraph instance.

## Resolving namespace

`src` resolves the given namespace in which to apply/preview the batch spec by sending a GraphQL request to the Sourcegraph instance to fetch the ID for the given namespace name.

If no namespace is specified with `-namespace` (or `-n`) then the currently authenticated user is used as the namespace. See "[Connect to Sourcegraph](../../cli/quickstart.md#connect-to-sourcegraph)" in the CLI docs for details on how to authenticate.

## Preparing container images

If the batch spec contains `steps`, then for each step `src` checks its `container` image to see whether it's already available locally.

To do that it runs `docker image inspect --format {{.Id}} -- <container-image-name>` to get the specific image ID for the container image.

If that fails with a "No such image" error, `src` tries to pull the image by running `docker image pull <container-image-name>` and then running `docker image inspect --format {{.Id}} -- <container-image-name>` again.

## Resolving repositories

`src` resolves each entry in the batch spec's `on` property to produce a _unique list of repositories (!)_ in which to execute the batch spec's `steps`.

With an `on` property like this

```yaml
on:
  - repositoriesMatchingQuery: lang:go fmt.Sprintf("%d", :[v]) patterntype:structural -file:vendor
  - repositoriesMatchingQuery: repohasfile:README.md
  - repository: github.com/sourcegraph/sourcegraph
  - repository: github.com/sourcegraph/automation-testing
    branch: thorstens-test-branch
```

`src` will do the following:

1. For each `repositoriesMatchingQuery` it will:
	1. Send a request to the Sourcegraph API to execute the search query.
	1. Collect each result's _repository_: the ID, the name, the default branch and the current revision of the default branch. If the search result _is a repository result_ (i.e. a search query of `type:repo` only produces repositories) that's used. If it's a file match the file match's repository is used.
	1. _Optional_: if the results are file matches, then their path in the repository is also saved, so that they can be used in the `steps` with [templating](../references/batch_spec_templating.md).
1. For each `repository` without a `branch` it will:
	1. Send a request to the Sourcegraph API to get the repository's ID, name, its default branch and the current revision of the default branch.
1. For each `repository` _with_ a `branch` it will:
	1. Send a request to the Sourcegraph API to get the repository's ID, and name and the current revision of the specified `branch`.
1. It then creates a _unique_ list of all repositories yielded by the previous three steps by going through all repositories and comparing them, skipping those where no current revision of a branch could be resolved, checking whether they're on a supported code host. If they are on unsupported code hosts and no `-allow-unsupported` flag is given, then a warning is printed and the repositories are not added to the list.

## Executing steps

If a batch spec contains `steps` then `src` executes the steps _locally_, on the machine on which `src` is run, for _each repository yielded by the previous "[Resolving repositories](#resolving-repositories)" step_.

If `-clear-cache` is _not_ set and it previously executed _the same `steps`_ for the _same repository_ at the _same revision of the base branch_, it will try to use cached results instead of re-executing the steps.

The following is what `src` does _for each repository_:

### 1. Download archive and prepare

1. Download archive of repository. What it does is equivalent to:

    ```
    curl -L -v -X GET -H 'Accept: application/zip' \
      -H 'Authorization: token <THE_SRC_TOKEN>' \
      'http://sourcegraph.example.com/github.com/my-org/my-repo@refs/heads/master/-/raw' \
      --output ~/tmp/my-repo.zip
    ```
2. Unzip archive into the workspace. Where the workspace lives depends on the workspace mode, which can be controlled by the `-workspace` flag. The two modes are:
  * _Bind_ mount mode (the default everywhere except Intel macOS), this will be somewhere on the filesystem, e.g. `~/.cache/sourcegraph/batch-changes` (see `src batch preview -h` for the default value of cache directory, overwrite with `-cache`)
  * _Volume_ mount mode (the default on Intel macOS): a Docker volume will be created using `docker volume create` and attached to all running containers, then removed before `src` exits
3. `cd` into the workspace, which now contains the unzipped archive
4. In the workspace, create a git repository:
	- Configure `git` to not use local configuration (see [the code for explanations on what each variable does](https://github.com/sourcegraph/src-cli/blob/54fedaf3bfcf21ad3a8d89d9d2d361c8c6da6441/internal/batches/git.go#L13-L26)):

    ```
    export GIT_CONFIG_NOSYSTEM=1 \
           GIT_CONFIG=/dev/null \
           GIT_AUTHOR_NAME=Sourcegraph \
           GIT_AUTHOR_EMAIL=batch-changes@sourcegraph.com \
           GIT_COMMITTER_NAME=Sourcegraph \
           GIT_COMMITTER_EMAIL=batch-changes@sourcegraph.com
    ```
  - Run `git init`
  - Run `git config --local user.name Sourcegraph`
  - Run `git config --local user.email batch-changes@sourcegraph.com`
  - Run `git add --force --all`
  - Run `git commit --quiet --all -m sourcegraph-batch-changes`

### 2. Run the steps

For each step in the batch spec `steps`:

1. Probe container image (the `container` property of the step) to see whether it has `/bin/sh` or `/bin/bash`
2. Write the step's `run` command to a temp file on the host, e.g. `/tmp-script`
3. Run `chmod 644 /tmp-script`
4. Run the Docker container. The exact command will depend on the workspace mode:
  * _Bind_:

      ```
      docker run --rm --init --workdir /work \
        --mount type=bind,source=/unzipped-archive-locally,target=/work \
        --mount type=bind,source=/tmp-script,target=/tmp-file-in-container \
        --entrypoint /bin/bash -- <IMAGE> /tmp-file-in-container
      ```
  * _Volume_:

      ```
      docker run --rm --init --workdir /work \
        --mount type=volume,source=temporary-docker-volume-id,target=/work \
        --mount type=bind,source=/tmp-script,target=/tmp-file-in-container \
        --entrypoint /bin/bash -- <IMAGE> /tmp-file-in-container
      ```
5. Add all produced changes to the git index: `git add --all`

### 3. Create final diff

In the workspace:

1. Create a diff by running: `git diff --cached --no-prefix --binary`

### 4. Saving a changeset spec

`src` adds the produced diff to the local cache, so that re-executing the same steps in the same repository can be skipped if the base branch has not changed.

`src` then creates a changeset spec from:
- the diff
- information about the repository in which the changes have been made (the name and ID of the repository, the revision of its base branch)
- the `changesetTemplate`

A changeset spec is a description of what the changeset should look like.

## Importing changesets

If the batch spec contains [`importChangesets`](../references/batch_spec_yaml_reference.md#importchangesets) then `src` goes through the list of `importChangesets` and for each entry it:

1. Resolves the repository name, trying to get to get an ID, base branch, and revision for the given repository name.
1. Parses the `externalIDs`, checking that they're valid strings or numbers.
1. For each external ID it saves a changeset spec that describes that a changeset with the given external ID, in the given repository, should be imported and tracked in the batch change.

## Sending changeset specs

The previous two steps, "[Executing steps](#executing-steps)" and "[Importing changesets](#importing-changesets)", can produce changeset specs, each one describing either a changeset to create or to import.

These changeset specs are now uploaded to the connected Sourcegraph instance, one request per changeset spec.

Each request yields an ID that uniquely identifies the changeset spec on the Sourcegraph instance. These IDs are used for the next step.

## Sending the batch spec

The IDs of the changeset specs that were created in the previous step, "[Sending changeset specs](#sending-changeset-specs)", are collected into a list and used for the next request with which `src` uploads the batch spec to the connected Sourcegraph instance.

`src` _creates_ the batch spec on the Sourcegraph instance, together with the changeset spec IDs, so that the batch spec fully describes the desired state of a batch change: its name, its description, and which changesets should be created or imported from which repository on which code host.

That request yields an ID that uniquely identifies this expanded version of the batch spec.

## Preview or apply the batch spec

If `src batch apply` was used, then the ID of the batch change is then used to send another request to the Sourcegraph instance, to _apply_ the batch spec.

If `src batch preview` was used to execute and create the batch spec, then a URL is printed, pointing to a preview page on the Sourcegraph instance on which we can see what _would_ happen if we were to apply the batch spec.
