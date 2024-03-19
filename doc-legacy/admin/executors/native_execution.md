# Native Execution

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta and might change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

> NOTE: This feature is available in Sourcegraph 5.1.0 and later.

Native Execution is an image that runs Batch Changes without
requiring [`src-cli`](https://github.com/sourcegraph/src-cli) to be installed on the Executor machine.

Native Execution is required when running Batch Changes in Kubernetes. No docker-in-docker or privileged
containers are required.

This is also useful for environments where it is difficult to install `src-cli` on the Executor machine, e.g. air-gap
environments.

## Enable

Native Execution is configured using a feature flag. To enable it,

1. Go to **Site admin**
2. Under **Configuration** select **Feature flags**
3. Select **Create feature flag**
4. Enter `native-ssbc-execution` as the **Name**
5. Select `Boolean` as the **Type**
6. Set the **Value** to `True`

## Docker Image

The Native Execution Docker image is available on Docker Hub
at [`sourcegraph/batcheshelper`](https://hub.docker.com/r/sourcegraph/batcheshelper/tags).

The default image (`sourcegraph/batcheshelper:${VERSION}`) can be overridden by updating the following in the **Site configuration** 

- `executors.batcheshelperImage`
- `executors.batcheshelperImageTag`

## Requirements

The Docker Images that execute the actual Batch Change step require `tee` to be available on the image. Without `tee`,
the output of the step cannot be captured properly for template variable rendering.
