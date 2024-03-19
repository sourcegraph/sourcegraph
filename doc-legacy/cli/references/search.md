# `src search`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-display` | Limit the number of results that are displayed. Only supported together with stream flag. Statistics continue to report all results. | `-1` |
| `-dump-requests` | Log GraphQL requests and responses to stdout | `false` |
| `-explain-json` | Explain the JSON output schema and exit. | `false` |
| `-get-curl` | Print the curl command for executing this query and exit (WARNING: includes printing your access token!) | `false` |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-json` | Whether or not to output results as JSON. | `false` |
| `-less` | Pipe output to 'less -R' (only if stdout is terminal, and not json flag). | `true` |
| `-stream` | Consume results as stream. Streaming search only supports a subset of flags and parameters: trace, insecure-skip-verify, display, json. | `false` |
| `-trace` | Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing | `false` |
| `-user-agent-telemetry` | Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph | `true` |


## Usage

```
Usage of 'src search':
  -display int
    	Limit the number of results that are displayed. Only supported together with stream flag. Statistics continue to report all results. (default -1)
  -dump-requests
    	Log GraphQL requests and responses to stdout
  -explain-json
    	Explain the JSON output schema and exit.
  -get-curl
    	Print the curl command for executing this query and exit (WARNING: includes printing your access token!)
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -json
    	Whether or not to output results as JSON.
  -less
    	Pipe output to 'less -R' (only if stdout is terminal, and not json flag). (default true)
  -stream
    	Consume results as stream. Streaming search only supports a subset of flags and parameters: trace, insecure-skip-verify, display, json.
  -trace
    	Log the trace ID for requests. See https://docs.sourcegraph.com/admin/observability/tracing
  -user-agent-telemetry
    	Include the operating system and architecture in the User-Agent sent with requests to Sourcegraph (default true)

Examples:

  Perform a search and get results:

    	$ src search 'repogroup:sample error'

  Perform a search and get results as JSON:

    	$ src search -json 'repogroup:sample error'

Other tips:

  Make 'type:diff' searches have colored diffs by installing https://colordiff.org
    - Ubuntu/Debian: $ sudo apt-get install colordiff
    - Mac OS:        $ brew install colordiff
    - Windows:       $ npm install -g colordiff

  Disable color output by setting NO_COLOR=t (see https://no-color.org).

  Force color output on (not on by default when piped to other programs) by setting COLOR=t

  Query syntax: https://docs.sourcegraph.com/code_search/reference/queries

  Be careful with search strings including negation: a search with an initial
  negated term may be parsed as a flag rather than as a search string. You can
  use -- to ensure that src parses this correctly, eg:

    	$ src search -- '-repo:github.com/foo/bar error'


```
	