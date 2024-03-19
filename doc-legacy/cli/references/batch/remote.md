# `src batch remote`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-allow-unsupported` | Allow unsupported code hosts. | `false` |
| `-clear-cache` | If true, clears the execution cache and executes all steps anew. | `false` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | The name of the batch spec file to run. |  |
| `-force-override-ignore` | Do not ignore repositories that have a .batchignore file. | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-n` | Alias for -namespace. |  |
| `-namespace` | The user or organization namespace to place the batch change within. Default is the currently authenticated user. |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |


## Usage

```
Usage of 'src batch remote':
  -allow-unsupported
    	Allow unsupported code hosts.
  -clear-cache
    	If true, clears the execution cache and executes all steps anew.
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	The name of the batch spec file to run.
  -force-override-ignore
    	Do not ignore repositories that have a .batchignore file.
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -n string
    	Alias for -namespace.
  -namespace string
    	The user or organization namespace to place the batch change within. Default is the currently authenticated user.
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)
'src batch remote' runs a batch spec on the Sourcegraph instance.

Usage:

    src batch remote [-f FILE]
    src batch remote FILE

Examples:

    $ src batch remote -f batch.spec.yaml



```
	