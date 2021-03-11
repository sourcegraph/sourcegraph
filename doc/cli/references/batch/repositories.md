# `src batch repositories`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | The batch spec file to read. |  |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src batch repositories':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	The batch spec file to read.
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

'src batch repositories' works out the repositories that a batch spec would
apply to.

Usage:

    src batch repositories -f FILE

Examples:

    $ src batch repositories -f batch.spec.yaml



```
	