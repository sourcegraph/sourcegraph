# Deployment Lag Notifer

https://github.com/sourcegraph/sourcegraph/issues/32878

The code for this is largely inspired by [Deployment Notifier](../deployment-notifer/README.md).

## Usage

Run `make`. The message (as would be sent to slack) should be printed out to the console.

If you would like to actually post the message, either configure the `-slack-webhook-url` flag or set the `SLACK_WEBHOOK_URL` environment variable and then run `make prod`.

## Flags

- `-dry-run` Print to stdout instead of sending to Slack
- `-env` (Default: Cloud) Environment to check against. Options: [`cloud`, `k8s`]
- `-slack-webhook-url` (env var: `SLACK_WEBHOOK_URL`) Slack webhook URL to post to. To get a webhook URL, add a configuration [here](https://sourcegraph.slack.com/apps/A0F7XDUAZ-incoming-webhooks?tab=settings&next_id=0):
- `-num-commits` (Default: `30`) Number of commits the deployed environment is allowed to differ from tip (Default: 30)
- `-allowed-age` (Default: `2.5h`) The age that the deployed version is allowed to drift from the current tip of `sourcegraph/sourcegraph@main`. The format should be provided in (`time.Duration`)[https://pkg.go.dev/time#ParseDuration] string format (e.g. `1h`, `20m`, `3h2m1s`) .

**NOTE:** Alerts will trigger when _either_ `num-commits` or `allowed-age` is exceeded.

## How it works

This is inspired by `sg live cloud`. It is run on a [fixed schedule]() to check that code deployed to Cloud is recent. If it detects that the deployed version on Cloud differs by more than an allowed number of commits from the tip of `sourcegraph/sourcegraph@main`, an alert will be sent to a Slack channel.
Hello World
