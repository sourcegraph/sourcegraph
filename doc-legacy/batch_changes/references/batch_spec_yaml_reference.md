# batch spec YAML reference

<style>
.markdown-body h2 { margin-top: 50px; }

/* The sidebar on this page contains a lot of long identifiers without
whitespace. In order to make them more readable we increase the width of the
sidebar. */
@media (min-width: 1200px) {
  body > #page > main > #index {
    width: 35%;
  }
}

</style>

[Sourcegraph Batch Changes](../index.md) use [batch specs](../explanations/introduction_to_batch_changes.md#batch-spec) to define batch changes.

This page is a reference guide to the batch spec YAML format in which batch specs are defined. If you're new to YAML and want a short introduction, see "[Learn YAML in five minutes](https://learnxinyminutes.com/docs/yaml/)."

## `name`

The name of the batch change, which is unique among all batch changes in the namespace. A batch change's name is case-preserving.

### Examples

```yaml
name: update-go-import-statements
```

```yaml
name: update-node.js
```

## `description`

The description of the batch change. It's rendered as Markdown.

### Examples

```yaml
description: This batch change changes all `fmt.Sprintf` calls to `strconv.Iota`.
```

```yaml
description: |
  This batch change changes all imports from

  `gopkg.in/sourcegraph/sourcegraph-in-x86-asm`

  to

  `github.com/sourcegraph/sourcegraph-in-x86-asm`
```

## `on`

The set of repositories (and branches) to run the batch change on, specified as a list of search queries (that match repositories) and/or specific repositories.

### Examples

```yaml
on:
  - repositoriesMatchingQuery: lang:go fmt.Sprintf("%d", :[v]) patterntype:structural
  - repository: github.com/sourcegraph/sourcegraph
```

## `on.repositoriesMatchingQuery`

A Sourcegraph search query that matches a set of repositories (and branches). Each matched repository branch is added to the list of repositories that the batch change will be run on.

Your search query should answer the question "where do I want to run this batch change?". Search result matches for things like commits, symbols, or file owners will be ignored.

See "[Code search](../../code_search/index.md)" for more information on Sourcegraph search queries.

### Examples

```yaml
on:
  - repositoriesMatchingQuery: file:README.md -repo:github.com/sourcegraph/src-cli
```

```yaml
on:
  - repositoriesMatchingQuery: lang:typescript file:web const changesetStatsFragment
```

## `on.repository`

A specific repository (and, optionally, one or more branches) to be added to the list of repositories that the batch change will be run on.

> NOTE: Before Sourcegraph 3.35, only the last named branch would be used if multiple branches were specified, and only a single `branch` could be provided. In Sourcegraph 3.35 and later versions, all branches are used.

To match a branch other than the default, `branch` or `branches` can be used to specify one or multiple branches, respectively. Only one of `branch` or `branches` can be set.

> WARNING: If multiple branches are matched for the same repository, then [`changesetTemplate.branch`](#changesettemplate-branch) will need to have a different value for each branch.

### Examples

```yaml
on:
  - repository: github.com/sourcegraph/src-cli
```

```yaml
on:
  - repository: github.com/sourcegraph/sourcegraph
    branch: 3.19-beta
  - repository: github.com/sourcegraph/src-cli
```

In the following example, the `repositoriesMatchingQuery` returns both repositories with their default branch, but the `3.23` branch is used for `github.com/sourcegraph/sourcegraph`, since it is more specific:

```yaml
on:
  - repositoriesMatchingQuery: repo:sourcegraph\/(sourcegraph|src-cli)$
  - repository: github.com/sourcegraph/sourcegraph
    branch: 3.23
```

In this example, both the `3.19-beta` and `3.23` branches are used:

```yaml
on:
  - repositoriesMatchingQuery: repo:sourcegraph\/(sourcegraph|src-cli)$
  - repository: github.com/sourcegraph/sourcegraph
    branches:
      - 3.19-beta
      - 3.23
```


## `steps`

The sequence of commands to run (for each repository branch matched in the `on` property) to produce the batch change's changes.

### Examples

```yaml
steps:
  - run: echo "Hello World!" >> README.md
    container: alpine:3
```

```yaml
steps:
  - run: comby -in-place 'fmt.Sprintf("%d", :[v])' 'strconv.Itoa(:[v])' .go -matcher .go -exclude-dir .,vendor
    container: comby/comby
  - run: gofmt -w ./
    container: golang:1.15-alpine
```

```yaml
steps:
  - run: ./update_dependency.sh
    container: our-custom-image
    env:
      OLD_VERSION: 1.31.7
      NEW_VERSION: 1.33.0
```

## `steps.run`

The shell command to run in the container. It can also be a multi-line shell script. The working directory is the root directory of the repository checkout.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>steps.run</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `steps.container`

The Docker image used to launch the Docker container in which the shell command is run.

The image has to have either the `/bin/sh` or the `/bin/bash` shell.

It is executed using `docker` on the machine on which the [Sourcegraph CLI (`src`)](https://sourcegraph.com/github.com/sourcegraph/src-cli) is executed. If the image exists locally, that is used. Otherwise it's pulled using `docker pull`.

## `steps.env`

Environment variables to set in the environment when running this command.

These may be defined either as an [object](#environment-object) or as an [array](#environment-array).

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>steps.env</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

### Environment object

In this case, `steps.env` is an object, where the key is the name of the environment variable and the value is the value.

#### Examples

```yaml
steps:
  - run: echo $MESSAGE >> README.md
    container: alpine:3
    env:
      MESSAGE: Hello world!
```

### Environment array

In this case, `steps.env` is an array. Each array item is either:

1. An object with a single property, in which case the key is used as the environment variable name and the value the value, or
2. For src-cli execution: A string that defines an environment variable to include from the environment `src` is being run within. This is useful to define secrets that you don't want to include in the spec file, but this makes the spec dependent on your environment, means that the local execution cache will be invalidated each time the environment variable changes, and means that the batch spec file is no longer [the sole source of truth intended by the Batch Changes design](../explanations/batch_changes_design.md).
3. For server-side execution: A string that defines a secret value to expose as an environment variable. Follow [the guide on executor secrets](../../admin/executors/executor_secrets.md) to set them up. The editor will suggest available secrets. This is useful to use secret values that you don't want to include in the spec file. The execution cache will be invalidated each time the secret value changes, and means that the batch spec file is no longer [the sole source of truth intended by the Batch Changes design](../explanations/batch_changes_design.md).

#### Examples

This example is functionally the same as the [object](#environment-object) example above:

```yaml
steps:
  - run: echo $MESSAGE >> README.md
    container: alpine:3
    env:
      - MESSAGE: Hello world!
```

This example pulls in the `USER` environment variable, or for server-side uses the executor secret called `USER`, and uses it to construct the line that will be appended to `README.md`:

```yaml
steps:
  - run: echo $MESSAGE from $USER >> README.md
    container: alpine:3
    env:
      - MESSAGE: Hello world!
      - USER
```

For instance, if `USER` is set to `adam`, this would append `Hello world! from adam` to `README.md`.

## `steps.files`

Files to create on the host machine and mount into the container when running `steps.run`.

`steps.files` is an object, where the key is the name of the file _inside the container_ and the value is the content of the file.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>steps.files</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

### Examples

```yaml
steps:
  - run: cat /tmp/my-temp-file.txt >> README.md
    container: alpine:3
    files:
      /tmp/my-temp-file.txt: Hello world!
```

```yaml
steps:
  - run: cat /tmp/global-gitignore >> .gitignore
    container: alpine:3
    files:
      /tmp/global-gitignore: |
        # Vim
        *.swp
        # JetBrains/IntelliJ
        .idea
        # Emacs
        *~
        \#*\#
        /.emacs.desktop
        /.emacs.desktop.lock
        .\#*
        .dir-locals.el
```

## `steps.outputs`

Output variables that are set after the [`steps.run`](#steps-run) command has been executed. These variables are available in the global `outputs` namespace as `outputs.<name>` <a href="batch_spec_templating">template variables</a> in the `run`, `env`, and `outputs` properties of subsequent steps, and the [`changesetTemplate`](#changesettemplate). Two steps with the same output variable name will overwrite the previous contents.

### Examples

```yaml
steps:
  - run: yarn upgrade
    container: alpine:3
    outputs:
      # Set output `friendlyMessage`
      friendlyMessage:
        value: "Hello there!"
```

```yaml
steps:
  - run: echo "Hello there!" >> message.txt && cat message.txt
    container: alpine:3
    outputs:
      friendlyMessage:
        # `value` supports templating variables and can access the just-executed
        # step's stdout/stderr.
        value: "${{ step.stdout }}"
```

```yaml
steps:
  - run: echo "Hello there!"
    container: alpine:3
    outputs:
      stepOneOutput:
        value: "${{ step.stdout }}"
  - run: echo "We have access to the output here: ${{ outputs.stepOneOutput }}"
    container: alpine:3
    outputs:
      stepTwoOutput:
        value: "here too: ${{ outputs.stepOneOutput }}"
```

```yaml
steps:
  - run: cat .goreleaser.yml >&2
    container: alpine:3
    outputs:
      goreleaserConfig:
        value: "${{ step.stderr }}"
        # Specifying a `format` tells Sourcegraph CLI how to parse the value before
        # making it available as a template variable.
        format: yaml

changesetTemplate:
  # [...]
  body: |
    The `goreleaser.yml` defines the following `before.hooks`:
    ${{ outputs.goreleaserConfig.before.hooks }}
```

## `steps.outputs.<name>.value`

The value the output should be set to.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>steps.outputs.$name.value</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `steps.outputs.<name>.format`

The format of the corresponding [`steps.outputs.<name>.value`](#outputs-value). When this is set to something other than `text`, it will be parsed as the given format.

Possible values: `text`, `yaml`, `json`. Default is `text`.

## `steps.if`

Condition to check before executing the step. If the value of the `if:` attribute is `true` (boolean) or `"true"` (string) then the step is executed in the given repository (or workspace, in case [workspaces](#workspaces) are used). Otherwise the step is skipped.

As an optimization, the [Sourcegraph CLI](../../cli/index.md) tries to evaluate the condition _before_ starting to execute any `steps`. If the condition can be evaluated ahead of time and the result of the evaluation is false then the execution of the step won't be attempted for the repository, which leads to better cache utilization.

Ahead-of-time evaluation is possible if the condition contains only static data. Example: `if: ${{ eq repository.name "github.com/my-org/my-repo" }}`. The repository name is known before the execution of the steps, so evaluation succeeds and Sourcegraph CLI will not include the given step in the list of steps to execute for repositories that don't have the matching name. That in turn allows the modification of this step's `run` attribute, for example, without invalidating the cache for the repositories in which it's never executed.

<aside class="note">
<span class="badge badge-feature">Templating</span> The <code>steps.if</code> condition can make use of <a href="batch_spec_templating">templating</a>.
</aside>

### Examples

```yaml
steps:
  # `if:` is true, step always executes.
  - if: true
    run: echo "name of repository is ${{ repository.name }}" >> message.txt
    container: alpine:3
```

```yaml
steps:
  # `if:` is a static string that's not "true", step never executes.
  - if: "random string"
    run: echo "name of repository is ${{ repository.name }}" >> message.txt
    container: alpine:3
```

```yaml
steps:
  # `if:` uses templating to check for repository name and produce a "true". Only runs in github.com/sourcegraph/automation-testing
  - if: ${{ eq repository.name "github.com/sourcegraph/automation-testing" }}
    run: echo "hello from automation-testing" >> message.txt
    container: alpine:3
```

```yaml
steps:
  # `if:` uses glob pattern to match repository name and produce "true" on match.
  - if: ${{ matches repository.name "*sourcegraph-testing*" }}
    run: echo "name contains sourcegraph-testing" >> message.txt
    container: alpine:3
```

```yaml
steps:
  # First step prints to standard out and saves to outputs
  - run:  if [[ -f "go.mod" ]]; then echo "true"; else echo "false"; fi
    container: alpine:3
    outputs:
      goModExists:
        value: ${{ step.stdout }}

  # `if:` uses the just-set `outputs.goModExists` value as condition
  - if: ${{ outputs.goModExists }}
    run: go fmt ./...
    container: golang
```

```yaml
steps:
  # `if:` checks for path, in case steps are executed in workspace.
  - if: ${{ eq steps.path "sub/directory/in/repo" }}
    run: echo "hello workspace" >> workspace.txt
    container: golang
```

## `steps.mount`

<aside class="note">
<span class="badge badge-note">New</span> <code>mount</code> is a new feature. Using <code>mount</code> locally is supported in Sourcegraph 3.41 and <a href="https://sourcegraph.com/github.com/sourcegraph/src-cli">Sourcegraph CLI</a> 3.41. Using <code>mount</code> in batch changes server-side is supported in Sourcegraph 4.1 and <a href="https://sourcegraph.com/github.com/sourcegraph/src-cli">Sourcegraph CLI</a> 4.0.1. It's a <b>preview</b> of functionality we're currently exploring to make running custom scripts/binaries easier. If you have any feedback, please let us know!
</aside>

> NOTE: This feature is currently only available for <a href="https://sourcegraph.com/github.com/sourcegraph/src-cli">Sourcegraph CLI</a>.

Mounts a local path to a path in a Docker container. Mounted paths are accessible to the step's `run` command.

A `path` can point to a file or a directory. The `path` can be an absolute path or a relative path. Regardless if the 
path is absolute or relative, the path must be within the same directory as the batch spec that is being run (the batch 
spec directory is considered the "working directory"). If the batch spec is provided using standard input, the current 
directory is used as the working directory.

Individual files are restricted to a max size of 10MB. Do not include any sensitive information in files being uploaded.

### Examples

```yaml
# Mount a Python script and run the script
steps:
  run: python /tmp/some-script.py # execute the script located at the mountpoint
  container: python:latest
  mount:
    - path: ./some-script.py # or absolute path /my/local/path/some-script.py
      mountpoint: /tmp/some-script.py
```

```yaml
# Mount a binary and run the binary
steps:
  run: /tmp/some-binary # execute the binary located at the mountpoint
  container: alpine:latest
  mount:
    - path: ./some-binary # or absolute path /my/local/path/some-binary
      mountpoint: /tmp/some-binary
```

```yaml
# Mount a directory containing scripts and run a script from the directory
steps:
  run: python /tmp/scripts/some-script.py
  container: python:latest
  mount:
    - path: ./scripts # or absolute path /my/local/path/scripts
      mountpoint: /tmp/scripts
```

```yaml
# Mount a Python script and a directory with supporting files for the script and run the script
steps:
  run: python /tmp/some-script.py
  container: python:latest
  mount:
    - path: ./some-script.py # or absolute path /my/local/path/some-script.py
      mountpoint: /tmp/some-script.py
    - path: ./supporting-files # or absolute path /my/local/path/supporting-files
      mountpoint: /tmp/supporting-files
```

## `importChangesets`

An array describing which already-existing changesets should be imported from the code host into the batch change.

### Examples

```yaml
importChangesets:
  - repository: github.com/sourcegraph/sourcegraph
    externalIDs: [13323, "13343", 13342, 13380]
  - repository: github.com/sourcegraph/src-cli
    externalIDs: [260, 271]
```


## `importChangesets.repository`

The repository name as configured on your Sourcegraph instance.

## `importChangesets.externalIDs`

The changesets to import from the code host. For GitHub this is the pull request number, for GitLab this is the merge request number, and for Bitbucket Server, Bitbucket Data Center, or Bitbucket Cloud this is the pull request number.

## `changesetTemplate`

A template describing how to create (and update) changesets with the file changes produced by the command steps.

This defines what the changesets on the code hosts (pull requests on GitHub, merge requests on Gitlab, ...) will look like.

### Examples

```yaml
changesetTemplate:
  title: Replace equivalent fmt.Sprintf calls with strconv.Itoa
  body: This batch change replaces `fmt.Sprintf("%d", integer)` calls with semantically equivalent `strconv.Itoa` calls
  branch: batch-changes/sprintf-to-itoa
  commit:
    message: Replacing fmt.Sprintf with strconv.Iota
    author:
      name: Lisa Coder
      email: lisa@example.com
  published: false
```

```yaml
changesetTemplate:
  title: Update rxjs in package.json to newest version
  body: This pull request updates rxjs to the newest version, `6.6.2`.
  branch: batch-changes/update-rxjs
  commit:
    message: Update rxjs to 6.6.2
  published: true
```

```yaml
changesetTemplate:
  title: Run go fmt over all Go files
  body: Regular `go fmt` run over all our Go files.
  branch: go-fmt
  commit:
    message: Run go fmt
    author:
      name: Anna Wizard
      email: anna@example.com
  published:
    # Do not meddle in the affairs of wizards, for they are subtle and quick to anger.
    - git.istari.example/*: false
    - git.istari.example/anna/*: true
```

## `changesetTemplate.title`

The title of the changeset on the code host.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>changesetTemplate.title</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `changesetTemplate.body`

The body (description) of the changeset on the code host. If the code supports Markdown you can use it here.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>changesetTemplate.body</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `changesetTemplate.branch`

The name of the Git branch to create or update on each repository with the changes.

If multiple branches within the same repository are matched in [`on.repository`](#on-repository), then this value must be dynamic, since it is impossible to create multiple branches with the same name in the same repository. This is often most easily accomplished with the `repository.branch` template variable. For example, this will create `new-feature-3.34` and `new-feature-3.35` branches:

```yaml
on:
  - repository: github.com/sourcegraph/sourcegraph
    branches:
      - 3.34
      - 3.35

changesetTemplate:
  branch: new-feature-${{ repository.branch }}
```

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>changesetTemplate.branch</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `changesetTemplate.commit`

The Git commit to create with the changes.

## `changesetTemplate.commit.message`

The Git commit message.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>changesetTemplate.commit.message</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

## `changesetTemplate.commit.author`

The `name` and `email` of the Git commit author.

<aside class="note">
<span class="badge badge-feature">Templating</span> <code>changesetTemplate.commit.author</code> can include <a href="batch_spec_templating">template variables</a>.
</aside>

### Examples

```yaml
changesetTemplate:
  commit:
    author:
      name: Alan Turing
      email: alan.turing@example.com
```

## `changesetTemplate.published`

Whether to publish the changesets as soon as the spec is applied. This may be a boolean value (ie `true` or `false`), `'draft'`, or [an array to only publish some changesets within the batch change](#publishing-only-specific-changesets). This may also be omitted, in which case the publication state will be controlled through the Sourcegraph UI, and will default to unpublished (that is, the same as specifying `false`).

An unpublished changeset can be previewed on Sourcegraph by any person who can view the batch change, but its commit, branch, and pull request aren't created on the code host.

When `published` is set to `draft` a commit, branch, and pull request / merge request are being created on the code host **in draft mode**. This means:

- On GitHub the changeset will be a [draft pull request](https://docs.github.com/en/free-pro-team@latest/github/collaborating-with-issues-and-pull-requests/about-pull-requests#draft-pull-requests).
- On GitLab the changeset will be a merge request whose title is be prefixed with `'WIP: '` to [flag it as a draft merge request](https://docs.gitlab.com/ee/user/project/merge_requests/work_in_progress_merge_requests.html#adding-the-draft-flag-to-a-merge-request).
- On Bitbucket Server, Bitbucket Data Center, and Bitbucket Cloud draft pull requests are not supported and changesets published as `draft` won't be created.

> NOTE: Changesets that have already been published on a code host as a non-draft (`published: true`) cannot be converted into drafts. Changesets can only go from unpublished to draft to published, but not from published to draft. That also allows you to take it out of draft mode on your code host, without risking Sourcegraph to revert to draft mode.

A published changeset results in a commit, branch, and pull request being created on the code host.

### Publishing only specific changesets

To publish only specific changesets within a batch change, an array of single-element objects can be provided. For example:

```yaml
published:
  - github.com/sourcegraph/sourcegraph: true
  - github.com/sourcegraph/src-cli: false
  - github.com/sourcegraph/batchutils: draft
```
<!--- TODO --->

Each key will be matched against the repository name using [glob](https://godoc.org/github.com/gobwas/glob#Compile) syntax. The [gobwas/glob library](https://godoc.org/github.com/gobwas/glob#Compile) is used for matching, with the key operators being:

| Term | Meaning |
|------|---------|
| `*`  | Match any sequence of characters |
| `?`  | Match any single character |
| `[ab]` | Match either `a` or `b` |
| `[a-z]` | Match any character between `a` and `z`, inclusive |
| `{abc,def}` | Match either `abc` or `def` |

If multiple entries match a repository, then the last entry will be used. For example, `github.com/a/b` will _not_ be published given this configuration:

```yaml
published:
  - github.com/a/*: true
  - github.com/*: false
```

If no entries match, then the repository will not be published. To make the default true, add a wildcard entry as the first item in the array:

```yaml
published:
  - "*": true
  - github.com/*: false
```

> NOTE: The standalone `"*"` is quoted in the key to avoid ambiguity in the YAML document.

By adding a `@<branch>` at the end of a match-rule, the rule is only matched against changesets with that branch:

```yaml
published:
  - github.com/sourcegraph/src-*@my-branch: true
  - github.com/sourcegraph/src-*@my-other-branch: true
```

### Examples

To publish all changesets created by a batch change:

```yaml
changesetTemplate:
  published: true
```

To publish all changesets created by a batch change as drafts:

```yaml
changesetTemplate:
  published: draft
```

To only publish changesets within the `sourcegraph` GitHub organization:

```yaml
changesetTemplate:
  published:
    - github.com/sourcegraph/*: true
```

To publish all changesets that are not on GitLab:

```yaml
changesetTemplate:
  published:
    - "*": true
    - gitlab.com/*: false
```

To publish all changesets on GitHub as draft:

```yaml
changesetTemplate:
  published:
    - "*": true
    - github.com/*: draft
```

To publish only one of many changesets in a repository by addressing them with their branch name:

```yaml
changesetTemplate:
  published:
    - "*": true
    - github.com/sourcegraph/*@my-branch-name-1: draft
    - github.com/sourcegraph/*@my-branch-name-2: false
```

(Multiple changesets in a single repository can be produced, for example, [per project in a monorepo](../how-tos/creating_changesets_per_project_in_monorepos.md) or by [transforming large changes into multiple changesets](../how-tos/creating_multiple_changesets_in_large_repositories.md)).

## `changesetTemplate.fork`

<span class="badge badge-note">Sourcegraph 5.1+</span>

Whether or not each changeset should be created on a fork of the upstream repository in the namespace of the user publishing them (or the namespace of the service account if [global credentials](../how-tos/configuring_credentials.md#global-service-account-tokens) are used).

If the site config setting [`batchChanges.enforceForks`](../../admin/config/batch_changes.md#forks) is enabled, this value will override the setting. For example, explicitly setting `fork: false` when the site config setting is enabled will result in changesets being published directly to the target repos. If omitted, the site config setting will be used.

The name of the fork Sourcegraph creates will be prefixed with the name of the original repo's namespace in order to prevent potential repo name collisions. For example, a batch spec targeting `github.com/my-org/project` would create or use any existing fork by the name `github.com/user/my-org-project`.

> NOTE: If your code host does not support forking (e.g. Gerrit), trying to publish a changeset with `fork: true` will result in an error.

### Examples

To publish all changesets created by this batch change to forks:

```yaml
changesetTemplate:
  fork: true
```

To publish all changesets created by this batch change to the target repository:

```yaml
changesetTemplate:
  fork: false
```

## `transformChanges`

A description of how to transform the changes (diffs) produced in each repository before turning them into separate changeset specs by inserting them into the [`changesetTemplate`](#changesettemplate).

This allows the creation of multiple changeset specs (and thus changesets) in a single repository.

### Examples

```yaml
# Transform the changes produced in each repository.
transformChanges:
  # Group the file diffs by directory and produce an additional changeset per group.
  group:
    # Create a separate changeset for all changes in the top-level `go` directory
    - directory: go
      branch: my-batch-change-go # will replace the `branch` in the `changesetTemplate`

    - directory: internal/codeintel
      branch: my-batch-change-codeintel # will replace the `branch` in the `changesetTemplate`
      repository: github.com/sourcegraph/src-cli # optional: only apply the rule in this repository
```


```yaml
transformChanges:
  group:
    - directory: go/utils/time
      branch: my-batch-change-go-time

    # The *last* matching directory is used, not the most specific one,
    # so only this changeset would be opened.
    - directory: go/utils
      branch: my-batch-change-go-date
```

## `transformChanges.group`

A list of groups to define which file diffs to group together to create an additional changeset in the given repository.

The **order of the list matters**, since each file diff's filepath is matched against the `directory` of a group and the **last match** is used.

If no changes have been produced in a `directory` then no changeset will be created.

## `transformChanges.group.directory`

The name of the directory in which file diffs should be grouped together.

The name is relative to the root of the repository.

## `transformChanges.group.branch`

The branch that should be used for this additional changeset. This **overwrites the [`changesetTemplate.branch`](#changesettemplate-branch)** when creating the additional changeset.

**Important**: the branch can _not_ be nested under the [`changesetTemplate.branch`](#changesettemplate-branch), i.e. if the `changesetTemplate.branch` is `my-batch-change` then this can _not_ be `my-batch-change/my-subdirectory` since [git doesn't allow that](https://stackoverflow.com/a/22630664). Additionally branch names must be unique and cannot be used as arguments for multiple `directory` fields.

## `transformChanges.group.repository`

Optional: the file diffs matching the given directory will only be grouped in a repository with that name, as configured on your Sourcegraph instance.

## `workspaces`

The optional `workspaces` property allows users to define where projects are located in repositories and cause the [`steps`](#steps) to be executed for each project, instead of once per repository. That allows easier creation of multiple changesets in large repositories.

For each repository that's yielded by [`on`](#on), Sourcegraph search is used to get the locations of the `rootAtLocationOf` file. Each location then serves as a workspace for the execution of the `steps`, instead of the root of the repository. Use the [`workspaces.in`](#workspaces-in) property to scope the workspaces definitions. Omitting it is treated as `*`.

**Important**: Since multiple workspaces in the same repository can produce multiple changesets, it's **required** to use templating to produce a unique [`changesetTemplate.branch`](#changesettemplate-branch) for each produced changeset. See the [examples](#workspaces-examples) below.

### Examples

Defining JavaScript projects that live in a monorepo by using the location of the `package.json` file as the root for each project:

```yaml
on:
  - repository: github.com/sourcegraph/sourcegraph

workspaces:
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/sourcegraph

changesetTemplate:
  # [...]

  # Since a changeset is uniquely identified by its repository and its
  # branch we need to ensure that each changesets has a unique branch name.

  # We can use templating and helper functions get the `path` in which
  # the `steps` executed and turn that into a branch name:
  branch: my-multi-workspace-batch-change-${{ replace steps.path "/" "-" }}
```

Using templating to produce a unique branch name in repositories _with_ workspaces and repositories without workspaces:

```yaml
on:
  - repository: github.com/sourcegraph/sourcegraph
  - repository: github.com/sourcegraph/src-cli

workspaces:
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/sourcegraph

changesetTemplate:
  # [...]

  # Since the steps in `github.com/sourcegraph/src-cli` are executed in the
  # root, where path is "", we can use `join_if` to drop it from the branch name
  # if it's a blank string:
  branch: ${{ join_if "-" "my-multi-workspace-batch-change" (replace steps.path "/" "-") }}
```

Defining where Go, JavaScript, and Rust projects live in multiple repositories:

```yaml
workspaces:
  - rootAtLocationOf: go.mod
    in: github.com/sourcegraph/go-*
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/*-js
    onlyFetchWorkspace: true
  - rootAtLocationOf: Cargo.toml
    in: github.com/rusty-org/*

changesetTemplate:
  # [...]

  branch: ${{ join_if "-" "my-multi-workspace-batch-change" (replace steps.path "/" "-") }}
```

Using [`steps.outputs`](#steps-outputs) to dynamically create unique branch names:

```yaml
# [...]
on:
  # Find all repositories with a package.json file
  - repositoriesMatchingQuery: repohasfile:package.json

workspaces:
  # Define that workspaces have their root folder at the location of the
  # package.json files
  - rootAtLocationOf: package.json
    in: "*"

steps:
  # [... steps that produce changes ...]

  # Run `jq` to extract the "name" from the package.json
  - run:  jq -j .name package.json
    container: jiapantw/jq-alpine:latest
    outputs:
      # Set outputs.packageName to stdout of this step's `run` command.
      packageName:
        value: ${{ step.stdout }}

changesetTemplate:
  # [...]

  # Use `outputs` variables to create a unique branch name per changeset:
  branch: my-batch-change-${{ outputs.projectName }}
```

Create changesets only on workspaces defined within subdirectories using `if:`:

```yaml
name: test-in
description: what happens in `in`?

on:
  - repository: github.com/sourcegraph/sourcegraph

workspaces:
  - rootAtLocationOf: package.json
    in: github.com/sourcegraph/sourcegraph
    onlyFetchWorkspace: true

steps:
  - run: |
      echo Path is: ${{ steps.path }} | tee path.txt
    container: alpine:3
    # Only creates changesets in subdirectories of client containing package.json files
    if: ${{ matches steps.path "client*" }}

changesetTemplate:
  title: Test `in` 
  body: what happens in `in`?
  branch: test-in-${{ replace "/" "-" steps.path }}
  commit:
    message: Test in
```

## `workspaces.rootAtLocationOf`

The full name of the file that sits at the root of one or more workspaces in a given repository.

Sourcegraph code search is used to find the location of files with this name in the repositories returned by [`on`](#on).

For example, in a repository with the following files:

- `packages/sourcegraph-ui/package.json`
- `packages/sourcegraph-test-helper/package.json`

the workspace configuration

```yaml
workspaces:
  - rootAtLocationOf: package.json
    in: "*"
```

would create _two changesets_ in the repository, one in `packages/sourcegraph-ui` and one in `packages/sourcegraph-test-helper`.

## `workspaces.in`

The repositories in which the workspace should be discovered.

This field supports **globbing** by using [glob](https://godoc.org/github.com/gobwas/glob#Compile) syntax. See "[Publishing only specific changesets](#publishing-only-specific-changesets)" for more information on globbing.

A repository matching multiple entries results in an error.

### Examples

Match all repository names that begin with `github.com/`:

```yaml
workspaces:
  - rootAtLocationOf: go.mod
    in: github.com/*
```

Match all repository names that begin with `gitlab.com/my-javascript-org/` and end with `-plugin`:

```yaml
workspaces:
  - rootAtLocationOf: package.json
    in: gitlab.com/my-javascript-org/*-plugin
```

## `workspaces.onlyFetchWorkspace`

When set to `true`, only the folder containing the workspace is downloaded to execute the `steps`.

This field is not required and when not set the default is `false`.

Additional files — `.gitignore` and `.gitattributes` as of now — are downloaded from the location of the workspace up to the root of the repository.

For example, with the following file layout in a repository

```
.
├── a
│   ├── b
│   │   ├── [... other files in b ...]
│   │   ├── package.json
│   │   └── .gitignore
│   │
│   ├── [... other files in a ...]
│   ├── .gitattributes
│   └── .gitignore
│
├── [... other files in root ... ]
└── .gitignore
```

and this workspace configuration

```yaml
workspaces:
  - rootAtLocationOf: package.json
    in: github.com/our-our/our-large-monorepo
    onlyFetchWorkspace: true
```

then

- the `steps` will be executed in `b`
- the complete contents of `b` will be downloaded and are available to the steps
- the `.gitattributes` and `.gitignore` files in `a` will be downloaded and put in `a`, **but only those**
- the `.gitignore` files in the root will be downloaded and put in the root folder, **but only that file**

### Examples

Only download the workspaces of specific JavaScript projects in a large monorepo:

```yaml
workspaces:
  - rootAtLocationOf: package.json
    in: github.com/our-our/our-large-monorepo
    onlyFetchWorkspace: true
```
