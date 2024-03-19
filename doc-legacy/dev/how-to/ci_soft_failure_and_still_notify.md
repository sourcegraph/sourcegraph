# How to allow a CI step to fail without breaking the build and still receive a notification.

Sometimes, it's not clearcut if a CI step is flaky or not, especially when the root cause for the failures is external to the system (like a third party website failing to answer requests). It means that the step is in gray area, where you typically want to keep running it so you can further observe and understand its behaviour, but you don't want to disrupt teammates workflow either.

That's what the _soft fail_ attribute is for, it will allow a step to fail without failing the build that contains that step. But this create another problem, as the owner of that step, you now have to actively monitor builds to see when they failed, which is not very practical.

Therefore a good solution for that is to also enable custom step notifications, so you can choose to get notified in the way you want when that particular step is failing. [How to receive a Slack notification if a specific CI step failed](./receive_slack_notification_on_a_failed_ci_step.md) covers it, but here we're focusing on showing how to do both.

## Editing your step to make it soft failing

In the [CI pipeline generator](../background-information/ci/development.md), you'll find the code that declare all steps, usually located in [ci/operations.go](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/ci/internal/ci/operations.go)

A good way to find all of them is the following search query:

<div class="embed">
  <iframe src="https://sourcegraph.com/embed/notebooks/Tm90ZWJvb2s6MTQwMA=="
    style="width:100%;height:720px" frameborder="0" sandbox="allow-scripts allow-same-origin allow-popups">
  </iframe>
</div>

Let's use as an example the following:

```diff
--- a/dev/ci/internal/ci/operations.go
+++ b/dev/ci/internal/ci/operations.go
func addJetBrainsUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":vitest::java: Test (client/jetbrains)",
		withPnpmCache(),
		bk.Cmd("pnpm install --fetch-timeout 60000"),
		bk.Cmd("pnpm generate"),
		bk.Cmd("pnpm --filter @sourcegraph/jetbrains run build"),
+   bk.SoftFail(1, 2),
	)
}
```

The `bk.SoftFail` function will make that step soft fail if and only if the exit code for that step is equal to `1` or `2`.

## Editing your step so is also sends a notification on failures

Now we want to add a custom notification as well:

> NOTE: You don't need to provide `buildkite.SlackStepNotifyConfigPayload.SlackTokenEnvVarName`, a default value will be injected.

```diff
--- a/dev/ci/internal/ci/operations.go
+++ b/dev/ci/internal/ci/operations.go
func addJetBrainsUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":vitest::java: Test (client/jetbrains)",
		withPnpmCache(),
+   bk.SlackStepNotify(&bk.SlackStepNotifyConfigPayload{
+     Message:              "JetBrains Unit tests failed, cc <@integrations-eng>",
+     ChannelName:          "integrations-internal",
+     Conditions:           bk.SlackStepNotifyPayloadConditions{
+       Failed: true,
+       Branches: []string{"main"},
+     },
+   }),
    bk.Cmd("pnpm install --fetch-timeout 60000"),
    bk.Cmd("pnpm generate"),
    bk.Cmd("pnpm --filter @sourcegraph/jetbrains run build"),
+   bk.SoftFail(1, 2),
+
	)
}
```

And that's it!

> NOTE: If you want to test your changes before merging your code, so you can see how the notification will look like, you can modify the step as following:

```diff
--- a/dev/ci/internal/ci/operations.go
+++ b/dev/ci/internal/ci/operations.go
unc addJetBrainsUnitTests(pipeline *bk.Pipeline) {
	pipeline.AddStep(":vitest::java: Test (client/jetbrains)",
		withPnpmCache(),
    bk.SlackStepNotify(&bk.SlackStepNotifyConfigPayload{
      Message:              "JetBrains Unit tests failed, cc <@integrations-eng>",
      ChannelName:          "integrations-internal",
      Conditions:           bk.SlackStepNotifyPayloadConditions{
        Failed: true,
+       // Branches: []string{"main"}, commenting so it triggers on your branch, before it gets merged.
-       Branches: []string{"main"},
      },
    }),
-   bk.Cmd("pnpm install --fetch-timeout 60000"),
-   bk.Cmd("pnpm generate"),
-   bk.Cmd("pnpm --filter @sourcegraph/jetbrains run build"),
+   bk.Cmd("please-fail-lol"),
-   bk.SoftFail(1, 2),
+   bk.SoftFail(127), // 127 is the exit code when a command isn't found, see the line above.
	)
}
```
