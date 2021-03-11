# `src orgs get`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-f` | Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}} ({{.DisplayName}})") | `{{.|json}}` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-name` | Look up organization by name. (e.g. "abc-org") |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src orgs get':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -f string
    	Format for the output, using the syntax of Go package text/template. (e.g. "{{.ID}}: {{.Name}} ({{.DisplayName}})") (default "{{.|json}}")
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -name string
    	Look up organization by name. (e.g. "abc-org")
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  Get organization named abc-org:

    	$ src orgs get -name=abc-org

  List usernames of members of organization named abc-org (replace '.Username' with '.ID' to list user IDs):

    	$ src orgs get -f '{{range $i,$ := .Members.Nodes}}{{if ne $i 0}}{{"\n"}}{{end}}{{.Username}}{{end}}' -name=abc-org



```
	