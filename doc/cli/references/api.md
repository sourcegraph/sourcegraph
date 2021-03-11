# `src api`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-query` | GraphQL query to execute, e.g. 'query { currentUser { username } }' (stdin otherwise) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-vars` | GraphQL query variables to include as JSON string, e.g. '{"var": "val", "var2": "val2"}' |  |


## Usage

```
Usage of 'src api':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -query string
    	GraphQL query to execute, e.g. 'query { currentUser { username } }' (stdin otherwise)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -vars string
    	GraphQL query variables to include as JSON string, e.g. '{"var": "val", "var2": "val2"}'

Exit codes:

  0: Success
  1: General failures (connection issues, invalid HTTP response, etc.)
  2: GraphQL error response

Examples:

  Run queries (identical behavior):

    	$ echo 'query { currentUser { username } }' | src api
    	$ src api -query='query { currentUser { username } }'

  Specify query variables:

    	$ echo '<query>' | src api 'var1=val1' 'var2=val2'

  Searching for "Router" and getting result count:

    	$ echo 'query($query: String!) { search(query: $query) { results { resultCount } } }' | src api 'query=Router'

  Get the curl command for a query (just add '-get-curl' in the flags section):

    	$ src api -get-curl -query='query { currentUser { username } }'


```
	