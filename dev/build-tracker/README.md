# BUILD TRACKER

Build Tracker is a server that listens for build events from Buildkite and stores them in memory and sends notifications about builds if they've failed.

The server currently listens for two events:

- `build.finished`
- `job.finished`

For each `job.finished` event that is received, the corresponding `build` is updated with the job that has finished. On receipt of a `build.finished` event, the server will determine if the build has failed by going through all the contained jobs of the build. If one or more jobs have indeed failed, a notification will be sent over slack.

## Deployment infrastructure

Build Tracker is deployed in the Buildkite kubernetes cluster of the Sourcegraph CI project on GCP. For more information on the deployment see [infrastructure](https://github.com/sourcegraph/infrastructure/tree/main/buildkite/kubernetes)

## Build

Execute the `build.sh` script which will build the docker container and push it to correct GCP registry. Once the image has been pushed the pod needs to be restarted so that it can pick up the new image!

## Test

To run the tests execute `go test .`

### Notification testing

To test the notifications that get sent over slack you can pass the flag `-RunSlackIntegrationTest` as part of your test invocation, with some required configuration:

```sh
export SLACK_TOKEN='my valid token'
export BUILDKITE_WEBHOOK_TOKEN='optional'
export GITHUB_TOKEN='optional'
go test . -RunSlackIntegrationTest
```

You can enable Slack client debugging by exporting the following environment variable `BUILD_TRACKER_SLACK_DEBUG=1`
Hello World
