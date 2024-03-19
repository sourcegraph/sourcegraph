# `src batch preview`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-allow-unsupported` | Allow unsupported code hosts. | `false` |
| `-apply` | Ignored. | `false` |
| `-cache` | Directory for caching results and repository archives. | `~/.cache/sourcegraph/batch` |
| `-clean-archives` | If true, deletes downloaded repository archives after executing batch spec steps. Note that only the archives related to the actual repositories matched by the batch spec will be cleaned up, and clean up will not occur if src exits unexpectedly. | `true` |
| `-clear-cache` | If true, clears the execution cache and executes all steps anew. | `false` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | The batch spec file to read, or - to read from standard input. |  |
| `-force-override-ignore` | Do not ignore repositories that have a .batchignore file. | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-j` | The maximum number of parallel jobs. Default (or 0) is the number of CPU cores available to Docker. | `0` |
| `-keep-logs` | Retain logs after executing steps. | `false` |
| `-n` | Alias for -namespace. |  |
| `-namespace` | The user or organization namespace to place the batch change within. Default is the currently authenticated user. |  |
| `-run-as-root` | If true, forces all step containers to run as root. | `false` |
| `-skip-errors` | If true, errors encountered while executing steps in a repository won't stop the execution of the batch spec but only cause that repository to be skipped. | `false` |
| `-text-only` | INTERNAL USE ONLY. EXPERIMENTAL. Switches off the TUI to only print JSON lines. | `false` |
| `-timeout` | The maximum duration a single batch spec step can take. | `1h0m0s` |
| `-tmp` | Directory for storing temporary data, such as log files. Default is /tmp. Can also be set with environment variable SRC_BATCH_TMP_DIR; if both are set, this flag will be used and not the environment variable. | `/tmp` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |
| `-v` | print verbose output | `false` |
| `-workspace` | Workspace mode to use ("auto", "bind", or "volume") | `auto` |


## Usage

```
Usage of 'src batch preview':
  -allow-unsupported
    	Allow unsupported code hosts.
  -apply
    	Ignored.
  -cache string
    	Directory for caching results and repository archives. (default "~/.cache/sourcegraph/batch")
  -clean-archives
    	If true, deletes downloaded repository archives after executing batch spec steps. Note that only the archives related to the actual repositories matched by the batch spec will be cleaned up, and clean up will not occur if src exits unexpectedly. (default true)
  -clear-cache
    	If true, clears the execution cache and executes all steps anew.
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	The batch spec file to read, or - to read from standard input.
  -force-override-ignore
    	Do not ignore repositories that have a .batchignore file.
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -j int
    	The maximum number of parallel jobs. Default (or 0) is the number of CPU cores available to Docker.
  -keep-logs
    	Retain logs after executing steps.
  -n string
    	Alias for -namespace.
  -namespace string
    	The user or organization namespace to place the batch change within. Default is the currently authenticated user.
  -run-as-root
    	If true, forces all step containers to run as root.
  -skip-errors
    	If true, errors encountered while executing steps in a repository won't stop the execution of the batch spec but only cause that repository to be skipped.
  -text-only
    	INTERNAL USE ONLY. EXPERIMENTAL. Switches off the TUI to only print JSON lines.
  -timeout duration
    	The maximum duration a single batch spec step can take. (default 1h0m0s)
  -tmp string
    	Directory for storing temporary data, such as log files. Default is /tmp. Can also be set with environment variable SRC_BATCH_TMP_DIR; if both are set, this flag will be used and not the environment variable. (default "/tmp")
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)
  -v	print verbose output
  -workspace string
    	Workspace mode to use ("auto", "bind", or "volume") (default "auto")

'src batch preview' executes the steps in a batch spec and uploads it to a
Sourcegraph instance, ready to be previewed and applied.

Usage:

    src batch preview [command options] [-f FILE]
    src batch preview [command options] FILE

Examples:

    $ src batch preview -f batch.spec.yaml

    $ src batch preview batch.spec.yaml



```
	