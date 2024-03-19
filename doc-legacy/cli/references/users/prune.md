# `src users prune`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-days` | Days threshold on which to remove users, must be 60 days or greater and defaults to this value  | `60` |
| `-display-users` | display table of users to be deleted by prune | `false` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-force` | skips user confirmation step allowing programmatic use | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-remove-admin` | prune admin accounts | `false` |
| `-remove-null-users` | removes users with no last active value | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |


## Usage

```
Usage of 'src users prune':
  -days int
    	Days threshold on which to remove users, must be 60 days or greater and defaults to this value  (default 60)
  -display-users
    	display table of users to be deleted by prune
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -force
    	skips user confirmation step allowing programmatic use
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -remove-admin
    	prune admin accounts
  -remove-null-users
    	removes users with no last active value
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)

This command removes users from a Sourcegraph instance who have been inactive for 60 or more days. Admin accounts are omitted by default.
	
Examples:

	$ src users prune -days 182
	
	$ src users prune -remove-admin -remove-null-users


```
	