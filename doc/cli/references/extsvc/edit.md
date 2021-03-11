# `src extsvc edit`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-exclude-repos` | when specified, add these repositories to the exclusion list |  |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-id` | ID of the external service to edit |  |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-name` | exact name of the external service to edit |  |
| `-rename` | when specified, renames the external service |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |


## Usage

```
Usage of 'src extsvc edit':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -exclude-repos string
    	when specified, add these repositories to the exclusion list
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -id string
    	ID of the external service to edit
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -name string
    	exact name of the external service to edit
  -rename string
    	when specified, renames the external service
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing

Examples:

  Edit an external service configuration on the Sourcegraph instance:

    	$ cat new-config.json | src extsvc edit -id 'RXh0ZXJuYWxTZXJ2aWNlOjQ='
    	$ src extsvc edit -name 'My GitHub connection' new-config.json

  Edit an external service name on the Sourcegraph instance:

    	$ src extsvc edit -name 'My GitHub connection' -rename 'New name'

  Add some repositories to the exclusion list of the external service:

    	$ src extsvc edit -name 'My GitHub connection' -exclude-repos 'github.com/foo/one' 'github.com/foo/two'


```
	