# batcheshelper

`batcheshelper` is a helper script for Sourcegraph Batch Changes that is run in a workspace container before and after a
step is executed.

It is designed to replace Sourcegraph's `src` CLI for use in Executors.

## Usage

```shell
batcheshelper <pre|post> <step index> [OPTIONS]
OPTIONS:
  -input string
        The input JSON file for the workspace execution. Defaults to "input.json". (default "input.json")
  -previousStepPath string
        The path to the previous steps result file. Defaults to current working directory.
  -workspaceFiles string
        The path to the workspace files. Defaults to "/job/workspace-files". (default "/job/workspace-files")
```

### Arguments

| Argument | Placement | Description                       | Example Value         |
| -------- | --------- | --------------------------------- | --------------------- |
| Mode     | First     | The mode to run the script in.    | `pre` or `post`       |
| Step     | Second    | The step that is being processed. | `0`, `1`, `2`, etc... |

### Options

| Flag                | Default Value          | Description                                                                  |
| ------------------- | ---------------------- | ---------------------------------------------------------------------------- |
| `-input`            | `input.json`           | The path to the input file. Defaults to "input.json". (default "input.json") |
| `-previousStepPath` | N/A                    | The path to the previous step's result file.                                 |
| `-workspaceFiles`   | `/job/workspace-files` | The path to the workspace files.                                             |

## Modes

There are two modes that this script can run in: `pre` and `post`.

### pre

The `pre` mode prepares the workspace for the Batch Change step to be executed. This includes,

- Setting up Environment Variables
- Evaluating step condition
- Writing File Mounts
- Writing Workspace Files

#### Example Command

```shell
batcheshelper pre 0
```

### post

The `post` mode determines the changes that were made to the workspace by the Batch Change step. The mode will,

- Generate a `git` diff
- Write the Batch Change step logs to a result file
- Generate a Cache Key

#### Example Command

```shell
batcheshelper pre 0
```

## Build

When running on Kubernetes, the image can be built with the following command.

```shell
IMAGE=sourcegraph/batcheshelper:insiders ./build.sh
```

When running the following `sg` commands, `batcheshelper` will automatically be built on any changes to the binary.

- `batcheshelper-builder`
- `batches`
Hello World
