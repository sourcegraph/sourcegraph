# `src config get`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | Format for the output, using the syntax of Go package text/template. (e.g. "{{.|json}}") | `{{.|jsonIndent}}` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-subject` | The ID of the settings subject whose settings to get. (default: authenticated user) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src config get':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	Format for the output, using the syntax of Go package text/template. (e.g. "{{.|json}}") (default "{{.|jsonIndent}}")
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -subject string
    	The ID of the settings subject whose settings to get. (default: authenticated user)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  Get settings for the current user (authenticated by the src CLI's access token, if any):

    	$ src config get

  Get settings for the user with username alice:

    	$ src config get -subject=$(src users get -f '{{.ID}}' -username=alice)

  Get settings for the organization named abc-org:

    	$ src config get -subject=$(src orgs get -f '{{.ID}}' -name=abc-org)



```
	