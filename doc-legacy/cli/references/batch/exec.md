# `src batch exec`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-binaryDiffs` | Whether to encode diffs as base64. | `false` |
| `-f` | The workspace execution input file to read. |  |
| `-repo` | Path of the checked out repo on disk. |  |
| `-run-as-image-user` | True to run step containers as the default image user; if false or omitted, containers are always run as root. | `false` |
| `-timeout` | The maximum duration a single batch spec step can take. | `1h0m0s` |
| `-tmp` | Directory for storing temporary data. |  |
| `-workspaceFiles` | Path of workspace files on disk. |  |


## Usage

```
Usage of 'src batch exec':
  -binaryDiffs
    	Whether to encode diffs as base64.
  -f string
    	The workspace execution input file to read.
  -repo string
    	Path of the checked out repo on disk.
  -run-as-image-user
    	True to run step containers as the default image user; if false or omitted, containers are always run as root.
  -timeout duration
    	The maximum duration a single batch spec step can take. (default 1h0m0s)
  -tmp string
    	Directory for storing temporary data.
  -workspaceFiles string
    	Path of workspace files on disk.

INTERNAL USE ONLY: 'src batch exec' executes the given raw batch spec in the given workspaces.

The input file contains a JSON dump of the WorkspacesExecutionInput struct in
github.com/sourcegraph/sourcegraph/lib/batches.

Usage:

    src batch exec -f FILE -repo DIR -workspaceFiles DIR [command options]

Examples:

    $ src batch exec -f batch-spec-with-workspaces.json



```
	