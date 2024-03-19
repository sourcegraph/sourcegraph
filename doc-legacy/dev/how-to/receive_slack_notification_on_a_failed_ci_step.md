# How to receive a Slack notification if a specific CI step failed

This guide documents how to make a specific step send a custom notification on Slack if it failed.

> NOTE: This is especially useful when monitoring a flaky step, because it can be used in conjunction with soft failures to avoid blocking teammates builds while still being notifying the step owners if the step failed. See [How to make allow a CI step to fail without breaking a build and still being notified](./ci_soft_failure_and_still_notify.md).

## Why don't we use a standard buildkite notification?

While Buildkite provides a [mechanism](https://buildkite.com/docs/pipelines/notifications) for setting up Slack notifications, the [available conditionals](https://buildkite.com/docs/pipelines/conditionals#variable-and-syntax-reference-variables) do not allow to express a predicate such as "notify if and only if this step failed" without writing some bash code that requires to be familiar with the technical details of the CI. 

That particular code has been abstracted in a [Buildkite plugin](https://github.com/sourcegraph/step-slack-notify-buildkite-plugin) that we can use for that purpose. And as a bonus, they are more useful than the traditional Buildkite notifications, as they include a direct link to the failing step. 

> _On Slack, in the `#jh-bot-testing` channel:_

> 🐪 is thursty 🌊 cc @teammate

> 👉 [View logs](#) 👈 [sourcegraph/sourcegraph: Build XXXXX](#) 🔴

## How to write a step that notify on failure

In the [CI pipeline generator](../background-information/ci/development.md), when defining a step you can use the `buildkite.SlackStepNotify(config)` function to define what needs to be cached and under which key to store it.

> NOTE: You don't need to provide `buildkite.SlackStepNotifyConfigPayload.SlackTokenEnvVarName`, a default value will be injected. 

```diff
--- a/dev/ci/internal/ci/operations.go
+++ b/dev/ci/internal/ci/operations.go
pipeline.AddStep(":camel: Crossing the desert",
+ bk.SlackStepNotify(&bk.SlackStepNotifyConfigPayload{
+   Message:              "Camel are thursty :water_wave: cc <@teamate>", // Can also reference a Slack user group
+   ChannelName:          "jh-bot-testing",
+   Conditions:           bk.SlackStepNotifyPayloadConditions{Failed: true, Branches: []string{"main"}},
+ }),
  bk.Cmd("some command involving a camel, like a perl script")
)
```
