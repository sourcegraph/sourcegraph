# `src repos update-metadata`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-key` | The name of the metadata key to be updated (required) |  |
| `-repo` | The ID of the repo with the metadata key to be updated (required if -repo-name is not specified) |  |
| `-repo-name` | The name of the repo to add the key-value pair metadata to (required if -repo is not specified) |  |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |
| `-value` | The new metadata value of the metadata key to be set. Defaults to null. |  |


## Usage

```
Usage of 'src repos update-metadata':
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -key string
    	The name of the metadata key to be updated (required)
  -repo string
    	The ID of the repo with the metadata key to be updated (required if -repo-name is not specified)
  -repo-name string
    	The name of the repo to add the key-value pair metadata to (required if -repo is not specified)
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)
  -value string
    	The new metadata value of the metadata key to be set. Defaults to null.

Examples:

  Update the metadata value for a metadata key on a repository:

    	$ src repos update-metadata -repo=repoID -key=my-key -value=new-value

  Omitting -value will set the value of the key to null.

  [DEPRECATED] Note that 'update-kvp' is deprecated and will be removed in future release. Use 'update-metadata' instead.


```
	