# `src users delete`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-id` | The ID of the user to delete. |  |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src users delete':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -id string
    	The ID of the user to delete.
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  Delete a user account by ID:

    	$ src users delete -id=VXNlcjox

  Delete a user account by username:

    	$ src users delete -id=$(src users get -f='{{.ID}}' -username=alice)

  Delete all user accounts that match the query:

    	$ src users list -f='{{.ID}}' -query=alice | xargs -n 1 -I USERID src users delete -id=USERID



```
	