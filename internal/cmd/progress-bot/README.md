# Progress bot

## Running

```console
Usage of ./progress-bot:
  -channel string
        Slack channel to post message to (default "progress-bot-test")
  -dry
        If true, print out the JSON payload that would be sent to the Slack API
  -since duration
        Report new changelog entries since this period (default 24h0m0s)
```

## Deployment

The `progress-bot` is deployed with [this GitHub action](../../../.github/workflows/progress.yml).

In order to deploy a new version, first run `docker build -t sourcegraph/progress-bot .` and then `docker push sourcegraph/progress-bot`.
Hello World
