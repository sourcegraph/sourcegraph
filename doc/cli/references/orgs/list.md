# `src orgs list`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}} ({{.DisplayName}})" or "{{.|json}}") | `{{.Name}}` |
| `-first` | Returns the first n organizations from the list. (use -1 for unlimited) | `1000` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-query` | Returns organizations whose names match the query. (e.g. "alice") |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src orgs list':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}} ({{.DisplayName}})" or "{{.|json}}") (default "{{.Name}}")
  -first int
    	Returns the first n organizations from the list. (use -1 for unlimited) (default 1000)
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -query string
    	Returns organizations whose names match the query. (e.g. "alice")
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  List organizations:

    	$ src orgs list

  List *all* organizations (may be slow!):

    	$ src orgs list -first='-1'

  List organizations whose names match the query:

    	$ src orgs list -query='myquery'



```
	