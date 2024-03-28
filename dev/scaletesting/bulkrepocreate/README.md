# Bulkrepocreate

A CLI tool to generate blank repositories (containing a single commit with a README.md) in bulk on a given GHE instance.

## Usage

`go run ./dev/scaletesting/bulkrepocreate [flags...]`

Flags:

- Authenticating:
  - `github.token`: GHE Token to create the repositories (required).
  - `github.url`: Base URL to the GHE instance (ex: `https://ghe.sgdev.org`) (required).
  - `github.org`: Existing organization to create the repositories in (required).
  - `github.login`: Login of a user account on that GHE instance with write permissions on the given organization repos.
  - `github.password`: Password of the above mentioned user account.
  - `insecure`: Allow invalid/self-signed TLS certificates.
- Generation parameters:
  - `count`: Number of repositories to create (default: `100`).
  - `prefix`: Prefix to use when naming the repos, i.e using `foobar` as prefix will create repos named `foobar0000001`, `foobar0000002`, ... (default: `repo`)
  - `retry`: Number of times to retry pushind (can be tedious at high concurrency)
- Resuming work:
  - `resume`: sqlite database name to create or resume from (default `state.db`)

## FAQ

> Why require the organization to exist in the first place?

_Creating organization on the fly requires having an GHE admin account, which is inconvenient to share. You can create one with a single click in the UI._

> Why require the login and password for the user, isn't the token enough?

_The script will run `git push` which requires to authenticate as the user pushing the repos. Pushing over HTTPS is much simpler as it means it's not required to upload your public key on the GHE user account before running this script._

> Can I `ctrl-c` the script as we have a `-resume` flag?

No. The script is made to handle errors from third parties, it's not handling anything else.
Hello World
