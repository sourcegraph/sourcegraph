# Deployment Notifier

Deployment Notifier is a tool that analyzes changes in order to guess which pull requests have been deployed in a given environment.
It is meant to be included in deploymement pipeline to automatically notify teammates that their changes have been deployed.

## Usage

In a deployment repository (such as `sourcegraph/deploy-sourcegraph-*`) you can run the following command to post GitHub comments and Slack notifications.

```sh
deployment-notifier -environment $MY_ENV -slack.token=$SLACK_TOKEN -slack.webhook=$SLACK_WEBHOOK
```

### Flags

- `-github.token` (defaults to `$GITHUB_TOKEN`)
- `-environment` (default to the only valid option `production`)
- `-dry` (optional) do not post on Slack or GitHub, just print out what would be posted.
- `-slack.token` Slack Token used to find the matching Slack handle for pull request authors.
- `-slack.webhook` Slack webhook URL to post the notifications on.
- `-honeycomb.token` Honeycomb API token that is used to upload deployment traces.

## How it works

Deployment notifier works as following:

1. Inspect the of diff the deployment manifests' current commit (supposed to be a merge commit, e.g. run on a release or deploy branch)
2. Compute the deployed applications and take note of the old commit new and Sourcegraph commit for each of them.
3. For each set of old and new commits, find all PRs that happened after the old Sourcegraph commit, up to the new Sourcegraph commit.
4. Post a comment in each of those PRs with the list of applications computed in 2.
5. Post a Slack message pinging all pull request authors.

## Contributing

### Local usage

To avoid spamming comments, Deployment Notifier comes with a few flags to help:

- `-dry` prints a summary of what would be posted on the standard output.

### Testing

The tests uses recorded responses from GitHub, to update the cassettes, uses the `-update` flag when running `go test`. Make sure
that the `GITHUB_TOKEN` environment is defined when doing so.
Hello World
