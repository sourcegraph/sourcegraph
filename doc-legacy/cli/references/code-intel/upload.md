# `src code-intel upload`


## Flags

| Name | Description | Default Value |
|------|-------------|---------------|
| `-associated-index-id` | ID of the associated index record for this upload. For internal use only. | `-1` |
| `-commit` | The 40-character hash of the commit. Defaults to the currently checked-out commit. |  |
| `-file` | The path to the LSIF dump file. |  |
| `-github-token` | A GitHub access token with 'public_repo' scope that Sourcegraph uses to verify you have access to the repository. |  |
| `-gitlab-token` | A GitLab access token with 'read_api' scope that Sourcegraph uses to verify you have access to the repository. |  |
| `-ignore-upload-failure` | Exit with status code zero on upload failure. | `false` |
| `-indexer` | The name of the indexer that generated the dump. This will override the 'toolInfo.name' field in the metadata vertex of the LSIF dump file. This must be supplied if the indexer does not set this field (in which case the upload will fail with an explicit message). |  |
| `-indexerVersion` | The version of the indexer that generated the dump. This will override the 'toolInfo.version' field in the metadata vertex of the LSIF dump file. This must be supplied if the indexer does not set this field (in which case the upload will fail with an explicit message). |  |
| `-insecure-skip-verify` | Skip validation of TLS certificates against trusted chains | `false` |
| `-json` | Output relevant state in JSON on success. | `false` |
| `-max-concurrency` | The maximum number of concurrent uploads. Only relevant for multipart uploads. Defaults to all parts concurrently. | `-1` |
| `-max-payload-size` | The maximum upload size (in megabytes). Indexes exceeding this limit will be uploaded over multiple HTTP requests. | `100` |
| `-no-progress` | Do not display progress updates. | `false` |
| `-open` | Open the LSIF upload page in your browser. | `false` |
| `-repo` | The name of the repository (e.g. github.com/gorilla/mux). By default, derived from the origin remote. |  |
| `-root` | The path in the repository that matches the LSIF projectRoot (e.g. cmd/project1). Defaults to the directory where the dump file is located. |  |
| `-skip-scip` | Skip converting LSIF index to SCIP if the instance supports it; this option should only used for debugging | `false` |
| `-trace` | -trace=0 shows no logs; -trace=1 shows requests and response metadata; -trace=2 shows headers, -trace=3 shows response body | `0` |
| `-upload-route` | The path of the upload route. For internal use only. | `/.api/lsif/upload` |


## Usage

```
Usage of 'src code-intel upload':
  -associated-index-id int
    	ID of the associated index record for this upload. For internal use only. (default -1)
  -commit string
    	The 40-character hash of the commit. Defaults to the currently checked-out commit.
  -file string
    	The path to the LSIF dump file.
  -github-token string
    	A GitHub access token with 'public_repo' scope that Sourcegraph uses to verify you have access to the repository.
  -gitlab-token string
    	A GitLab access token with 'read_api' scope that Sourcegraph uses to verify you have access to the repository.
  -ignore-upload-failure
    	Exit with status code zero on upload failure.
  -indexer string
    	The name of the indexer that generated the dump. This will override the 'toolInfo.name' field in the metadata vertex of the LSIF dump file. This must be supplied if the indexer does not set this field (in which case the upload will fail with an explicit message).
  -indexerVersion string
    	The version of the indexer that generated the dump. This will override the 'toolInfo.version' field in the metadata vertex of the LSIF dump file. This must be supplied if the indexer does not set this field (in which case the upload will fail with an explicit message).
  -insecure-skip-verify
    	Skip validation of TLS certificates against trusted chains
  -json
    	Output relevant state in JSON on success.
  -max-concurrency int
    	The maximum number of concurrent uploads. Only relevant for multipart uploads. Defaults to all parts concurrently. (default -1)
  -max-payload-size int
    	The maximum upload size (in megabytes). Indexes exceeding this limit will be uploaded over multiple HTTP requests. (default 100)
  -no-progress
    	Do not display progress updates.
  -open
    	Open the LSIF upload page in your browser.
  -repo string
    	The name of the repository (e.g. github.com/gorilla/mux). By default, derived from the origin remote.
  -root string
    	The path in the repository that matches the LSIF projectRoot (e.g. cmd/project1). Defaults to the directory where the dump file is located.
  -skip-scip
    	Skip converting LSIF index to SCIP if the instance supports it; this option should only used for debugging
  -trace int
    	-trace=0 shows no logs; -trace=1 shows requests and response metadata; -trace=2 shows headers, -trace=3 shows response body
  -upload-route string
    	The path of the upload route. For internal use only. (default "/.api/lsif/upload")

Examples:
  Before running any of these, first use src auth to authenticate.
  Alternately, use the SRC_ACCESS_TOKEN environment variable for
  individual src-cli invocations. 

  If run from within the project itself, src-cli will infer various
  flags based on git metadata.

        $ src code-intel upload # uploads ./index.scip

  If src-cli is invoked outside the project root, or if you're using
  a version control system other than git, specify flags explicitly:

    	$ src code-intel upload -root='' -repo=FOO -commit=BAR -file=index.scip

  Upload a SCIP index for a subproject:

    	$ src code-intel upload -root=cmd/

  Upload a SCIP index when lsif.enforceAuth is enabled in site settings:

    	$ src code-intel upload -github-token=BAZ, or
    	$ src code-intel upload -gitlab-token=BAZ

  For any of these commands, an LSIF index (default name: dump.lsif) can be
  used instead of a SCIP index (default name: index.scip).


```
	