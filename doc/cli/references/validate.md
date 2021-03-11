# `src validate`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-context` | Comma-separated list of key=value pairs to add to the script execution context |  |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-secrets` | Path to a file containing key=value lines. The key value pairs will be added to the script context |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src validate validate':
  -context string
    	Comma-separated list of key=value pairs to add to the script execution context
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -secrets string
    	Path to a file containing key=value lines. The key value pairs will be added to the script context
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
'src validate' is a tool that validates a Sourcegraph instance.

EXPERIMENTAL: 'validate' is an experimental command in the 'src' tool.

Usage:

	src validate [options] src-validate.yml
or
    cat src-validate.yml | src validate [options]

Please visit https://docs.sourcegraph.com/admin/validation for documentation of the validate command.


```
	