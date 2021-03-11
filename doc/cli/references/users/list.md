# `src users list`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Username}} ({{.DisplayName}})" or "{{.|json}}") | `{{.Username}}` |
| `-first` | Returns the first n users from the list. (use -1 for unlimited) | `1000` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-query` | Returns users whose names match the query. (e.g. "alice") |  |
| `-tag` | Returns users with the given tag. |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src users list':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Username}} ({{.DisplayName}})" or "{{.|json}}") (default "{{.Username}}")
  -first int
    	Returns the first n users from the list. (use -1 for unlimited) (default 1000)
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -query string
    	Returns users whose names match the query. (e.g. "alice")
  -tag string
    	Returns users with the given tag.
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  List users:

    	$ src users list

  List *all* users (may be slow!):

    	$ src users list -first='-1'

  List users whose names match the query:

    	$ src users list -query='myquery'

  List all users with the "foo" tag:

    	$ src users list -tag=foo



```
	