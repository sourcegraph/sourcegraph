# `src extensions copy`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-current-user` | The current user |  |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-extension-id` | The <extID> in https://sourcegraph.com/extensions/<extID> (e.g. sourcegraph/java) |  |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |


## Usage

```
Usage of 'src extensions copy':
  -current-user string
    	The current user
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -extension-id string
    	The <extID> in https://sourcegraph.com/extensions/<extID> (e.g. sourcegraph/java)
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)

Copy an extension from Sourcegraph.com to your private registry.


```
	