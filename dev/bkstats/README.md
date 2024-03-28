# Bkstats

A crude script to compute simple "red time" statistics from our Buildkite pipelines and post them to Slack.
Owned by the [DevInfra team](https://handbook.sourcegraph.com/departments/engineering/teams/devinfra/).

"Red time" is defined to be the duration from the _end of a failed build_ to the _end of the first subsequent green build_.

For more detailed analyses over longer periods of time, check out [`buildchecker history`](../buildchecker/README.md#history).

## Usage

```sh
$ go run main.go -buildkite.token $BUILDKITE_API_TOKEN -date 2021-10-22 -buildkite.pipeline sourcegraph
# ...
On 2021-10-22, the pipeline was red for 1h8m32.856s
```

### Buildkite API token

1. Go over [your personal settings](https://buildkite.com/user/api-access-tokens)
2. Create a new token with the following permissions:

- Check `sourcegraph` organization
- `read_builds`
- `read_pipelines`
Hello World
