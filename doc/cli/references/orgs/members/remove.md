# `src orgs members remove`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-org-id` | ID of organization from which to remove member. (required) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-id` | ID of user to remove as member. (required) |  |


## Usage

```
Usage of 'src orgs members remove':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -org-id string
    	ID of organization from which to remove member. (required)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-id string
    	ID of user to remove as member. (required)

Examples:

  Remove a member (alice) from an organization (abc-org):

    	$ src orgs members remove -org-id=$(src org get -f '{{.ID}}' -name=abc-org) -user-id=$(src users get -f '{{.ID}}' -username=alice)


```
	