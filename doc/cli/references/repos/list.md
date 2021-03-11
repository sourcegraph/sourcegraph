# `src repos list`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-cloned` | Include cloned repositories. | `true` |
| `-descending` | Whether or not results should be in descending order. | `false` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}}") or "{{.|json}}") | `{{.Name}}` |
| `-first` | Returns the first n repositories from the list. (use -1 for unlimited) | `1000` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-indexed` | Include repositories that have a text search index. | `true` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-names-without-host` | Whether or not repository names should be printed without the hostname (or other first path component). If set, -f is ignored. | `false` |
| `-not-cloned` | Include repositories that are not yet cloned and for which cloning is not in progress. | `true` |
| `-not-indexed` | Include repositories that do not have a text search index. | `true` |
| `-order-by` | How to order the results; possible choices are: "name", "created-at" | `name` |
| `-query` | Returns repositories whose names match the query. (e.g. "myorg/") |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src repos list':
  -cloned
    	Include cloned repositories. (default true)
  -descending
    	Whether or not results should be in descending order.
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}}") or "{{.|json}}") (default "{{.Name}}")
  -first int
    	Returns the first n repositories from the list. (use -1 for unlimited) (default 1000)
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -indexed
    	Include repositories that have a text search index. (default true)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -names-without-host
    	Whether or not repository names should be printed without the hostname (or other first path component). If set, -f is ignored.
  -not-cloned
    	Include repositories that are not yet cloned and for which cloning is not in progress. (default true)
  -not-indexed
    	Include repositories that do not have a text search index. (default true)
  -order-by string
    	How to order the results; possible choices are: "name", "created-at" (default "name")
  -query string
    	Returns repositories whose names match the query. (e.g. "myorg/")
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  List repositories:

    	$ src repos list

  Print JSON description of repositories list:

    	$ src repos list -f '{{.|json}}'

  List *all* repositories (may be slow!):

    	$ src repos list -first='-1'

  List repositories whose names match the query:

    	$ src repos list -query='myquery'



```
	