# `src batch validate`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-allow-unsupported` | Allow unsupported code hosts. | `false` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | The batch spec file to read, or - to read from standard input. |  |
| `-force-override-ignore` | Do not ignore repositories that have a .batchignore file. | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |


## Usage

```
Usage of 'src batch validate':
  -allow-unsupported
    	Allow unsupported code hosts.
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
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)

'src batch validate' validates the given batch spec.

Usage:

    src batch validate [-f] FILE

Examples:

    $ src batch validate batch.spec.yaml

    $ src batch validate -f batch.spec.yaml



```
	