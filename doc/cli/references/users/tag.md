# `src users tag`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-remove` | Remove the tag. (default: add the tag | `false` |
| `-tag` | The tag to set on the user. (required) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-id` | The ID of the user to tag. (required) |  |


## Usage

```
Usage of 'src users tag':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -remove
    	Remove the tag. (default: add the tag
  -tag string
    	The tag to set on the user. (required)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-id string
    	The ID of the user to tag. (required)

Examples:

  Add a tag "foo" to a user:

    	$ src users tag -user-id=$(src users get -f '{{.ID}}' -username=alice) -tag=foo

  Remove a tag "foo" to a user:

    	$ src users tag -user-id=$(src users get -f '{{.ID}}' -username=alice) -remove -tag=foo

Related examples:

  List all users with the "foo" tag:

    	$ src users list -tag=foo



```
	