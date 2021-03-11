# `src users create`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-email` | The new user's email address. (required) |  |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-reset-password-url` | Print the reset password URL to manually send to the new user. | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-username` | The new user's username. (required) |  |


## Usage

```
Usage of 'src users create':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -email string
    	The new user's email address. (required)
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -reset-password-url
    	Print the reset password URL to manually send to the new user.
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -username string
    	The new user's username. (required)

Examples:

  Create a user account:

    	$ src users create -username=alice -email=alice@example.com



```
	