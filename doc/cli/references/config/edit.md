# `src config edit`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-overwrite` | Overwrite the entire settings with the value given in -value (not just a single property). | `false` |
| `-property` | The name of the settings property to set. |  |
| `-subject` | The ID of the settings subject whose settings to edit. (default: authenticated user) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-value` | The value for the settings property (when used with -property). |  |
| `-value-file` | Read the value from this file instead of from the -value command-line option. |  |


## Usage

```
Usage of 'src config edit':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -overwrite
    	Overwrite the entire settings with the value given in -value (not just a single property).
  -property string
    	The name of the settings property to set.
  -subject string
    	The ID of the settings subject whose settings to edit. (default: authenticated user)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -value string
    	The value for the settings property (when used with -property).
  -value-file string
    	Read the value from this file instead of from the -value command-line option.

Examples:

  Edit settings property for the current user (authenticated by the src CLI's access token, if any):

    	$ src config edit -property motd -value '["Hello!"]'

  Overwrite all settings settings for the current user:

    	$ src config edit -overwrite -value '{"motd":["Hello!"]}'

  Overwrite all settings settings for the current user with the file contents:

    	$ src config edit -overwrite -value-file myconfig.json

  Edit a settings property for the user with username alice:

    	$ src config edit -subject=$(src users get -f '{{.ID}}' -username=alice) -property motd -value '["Hello!"]'

  Overwrite all settings settings for the organization named abc-org:

    	$ src config edit -subject=$(src orgs get -f '{{.ID}}' -name=abc-org) -overwrite -value '{"motd":["Hello!"]}'

  Change global settings:

    	$ src config edit -subject=$(echo 'query { site { id } }' | src api | jq .data.site.id --raw-output)


```
	