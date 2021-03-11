# `src orgs delete`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-id` | The ID of the organization to delete. |  |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src orgs delete':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -id string
    	The ID of the organization to delete.
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  Delete an organization by ID:

    	$ src orgs delete -id=VXNlcjox

  Delete an organization by name:

    	$ src orgs delete -id=$(src orgs get -f='{{.ID}}' -name=abc-org)

  Delete all organizations that match the query

    	$ src orgs list -f='{{.ID}}' -query=abc-org | xargs -n 1 -I ORGID src orgs delete -id=ORGID



```
	