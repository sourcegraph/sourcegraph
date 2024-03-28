# buildchecker [![buildchecker](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker.yml/badge.svg)](https://github.com/sourcegraph/sourcegraph/actions/workflows/buildchecker.yml)

`buildchecker` is designed to respond to periods of consecutive build failures on a Buildkite pipeline.
Owned by the [DevInfra team](https://handbook.sourcegraph.com/departments/engineering/teams/devinfra/).

More documentation for Sourcegraph teammates is available in the[CI incidents playbook](https://handbook.sourcegraph.com/departments/product-engineering/engineering/process/incidents/playbooks/ci#scenarios).

Buildchecker is deployed as a [GitHub Action](../../.github/workflows/buildchecker.yml).

## Usage

Available commands:

- [`buildchecker check`](#check)
- [`buildchecker history`](#history)

### Check

Checks for a series of build failures that exceed the configured threshold, locks the target branch, and posts various updates to Slack.

```sh
go run ./dev/buildchecker/ check # directly
./dev/buildchecker/run-check.sh  # using wrapper script
```

Also see the [`buildchecker` GitHub Action workflow](../../.github/workflows/buildchecker.yml) where `buildchecker check` is run on an automated basis.

### History

Writes aggregated historical data, including the builds it finds, to a few files.

```sh
go run ./dev/buildchecker \
  -buildkite.token=$BUILDKITE_TOKEN \
  -builds.write-to=".tmp/builds.json" \
  -csv.write-to=".tmp/" \
  -failures.timeout=999 \
  -created.from="2021-08-01" \
  history
```

To load builds from a file instead of fetching from Buildkite, use `-builds.load-from="$FILE"`.

You can also send metrics to Honeycomb with `-honeycomb.dataset` and `-honeycomb.token`:

```sh
go run ./dev/buildchecker \
  -builds.load-from=".tmp/builds.json" \
  -failures.timeout=999 \
  -created.from="2021-08-01" \
  -honeycomb.dataset="buildkite-history" \
  -honeycomb.token=$HONEYCOMB_TOKEN \
  history
```

## Tokens

### Buildkite API token

Required for all `buildchecker` commands, except for `buildchecker history -load-from`.

1. Go over [your personal settings](https://buildkite.com/user/api-access-tokens)
2. Create a new token with the following permissions:

- check `sourcegraph` organization
- `read_builds`
- `read_pipelines`

## Development

- `branch_test.go` contains integration tests against the GitHub API. Normally runs against recordings in `testdata` - to update `testdata`, run the tests with the `-update` flag.
- All other tests are strictly unit tests.
Hello World
