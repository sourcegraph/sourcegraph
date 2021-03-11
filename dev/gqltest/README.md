# GraphQL integration tests

This directory contains API-based integration tests in the form of standard Go tests. It is called gqltest since most of our API is GraphQL. However, the test suite has been extended to test other endpoints such as streaming search.

## How to set up credentials

Tests use environment variables to accept credentials of different external services involved, suggest to use [direnv](https://direnv.net/) to presistent those credentials for your convenience. Here is a comperhensive example `.envrc` file (you're free to use any other means, e.g. `.profile` or `.bashrc`):

```sh
# Your GitHub personal access token, this token needs to have scope to access private
# repositories of "sgtest" organization. If you haven't joined "sgtest" organization,
# please post a message on #dev-chat to ask for an invite.
export GITHUB_TOKEN=<REDACTED>

# Please go to https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/5q5lnpirajegt7uifngeabrak4
# and copy relevant credentials to here.
export AWS_ACCESS_KEY_ID=<REDACTED>
export AWS_SECRET_ACCESS_KEY=<REDACTED>
export AWS_CODE_COMMIT_USERNAME=<REDACTED>
export AWS_CODE_COMMIT_PASSWORD=<REDACTED>

export BITBUCKET_SERVER_URL=<REDACTED>
export BITBUCKET_SERVER_TOKEN=<REDACTED>
export BITBUCKET_SERVER_USERNAME=<REDACTED>
```

You need to run `direnv allow` after editing the `.envrc` file.

Alternatively you can use the 1password CLI tool:

```sh
# dev-private token for ghe.sgdev.org
op get item bw4nttlfqve3rc6xqzbqq7l7pm | jq -r '.. | select(.t? == "token name: dev-private") | @sh "export GITHUB_TOKEN=\(.v)"'
# AWS and Bitbucket tokens
op get item 5q5lnpirajegt7uifngeabrak4 | jq -r '.details.sections[] | .fields[] | @sh "export \(.t)=\(.v)"
```

## How to run tests

GraphQL-based integration tests are running against a live Sourcegraph instance, the eaiset way to make one is by booting up a single Docker container:

```sh
# For easier testing, run Sourcegraph instance without volume,
# so it always starts from a clean state.
docker run --publish 7080:7080 --rm sourcegraph/server:insiders
```

Once the the instance is live (look for the log line `✱ Sourcegraph is ready at: http://127.0.0.1:7080`), you can open another terminal tab to run these tests under this directory (`dev/gqltest`):

```sh
→ go test -long
2020/07/17 14:17:32 Site admin has been created: gqltest-admin
PASS
ok  	github.com/sourcegraph/sourcegraph/dev/gqltest	31.521s
```

### Testing against local dev instance

It is not required to boot up a single Docker container to run these tests, which means it's also possible to run these tests against any Sourcegraph instance, for example, your local dev instance:

```sh
go test -long -base-url "http://localhost:3080" -email "joe@sourcegraph.com" -username "joe" -password "<REDACTED>"
```

Generally, you're able to repeatedly run these tests regardless of any failures because tests are written in the way that cleans up and restores to the previous state. It is aware of if the instance has been initialized, so you can focus on debugging tests.

Because we're using the standard Go test framework, you are able to just run a single or subset of these tests:

```sh
→ go test -long -run TestSearch
2020/07/17 14:20:59 Site admin authenticated: gqltest-admin
PASS
ok  	github.com/sourcegraph/sourcegraph/dev/gqltest	3.073s
```

## How to add new tests

Adding new tests to this test suite is as easy as adding a Go test, here are some general rules to follow:

- Use `gqltest-` prefix for entity name, and be as specific as possible for easier debugging, e.g. `gqltest-org-user-1`.
- Restore the previous state regardless of failures, including:
  - Delete new users created during the test.
  - Delete external service created during the test.
  - Although, sometimes you would not want to delete an entity so you could login and inspect the failure state.
- Prefix your branch name with `master-dry-run/` will run integration tests in CI on your pull request.
