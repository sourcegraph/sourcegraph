# Executor

The executor service polls the public frontend API for work to perform. The executor will pull a job from a particular queue (configured via the envvar `EXECUTOR_QUEUE_NAME`), then performs the job by running a sequence of docker and src-cli commands. This service is horizontally scalable.

Since executors and Sourcegraph are separate deployments, our agreement is to support 1 minor version divergence for now. See this example for more details:

| **Sourcegraph version** | **Executor version** | **Ok** |
| ----------------------- | -------------------- | ------ |
| 3.43.0                  | 3.43.\*              | ✅     |
| 3.43.3                  | 3.43.\*              | ✅     |
| 3.43.0                  | 3.44.\*              | ✅     |
| 3.43.0                  | 3.42.\*              | ✅     |
| 3.43.0                  | 3.41.\*              | 🚫     |
| 3.43.0                  | 3.45.\*              | 🚫     |

See the [executor queue](../frontend/internal/executorqueue/README.md) for a complete list of queues.

## Building and releasing

Building and releasing is handled automatically by the CI pipeline.

### Binary

The executor binary is simply built with `bazel build //cmd/executor:executor`.

For publishing it, see `bazel run //cmd/executor:binary.push`:

- In every scenario, the binary will be uploaded to `gcs://sourcegraph-artifacts/executors/$GIT_COMMIT/`.
- If the current branch is `main` when this target is run, it will also be copied over to `gcs://sourcegraph-artifacts/executors/latest`.
- If the env var `EXECUTOR_IS_TAGGED_RELEASE` is set to true, it will also be copied over to `gcs://sourcegraph-artifacts/executors/$BUILDKITE_TAG`.

### VM image

The VM Image is built with `packer`, but it also uses an OCI image as a base for Firecracker, `//docker-images/executor-vm:image_tarball` which it depends on. That OCI image is a _legacy_ image, see `docker-images/executor-vm/README.md`.

Because we're producing an AMI for both AWS and GCP, there are two steps involved:

- `bazel run //cmd/executor/vm-image:ami.build` creates the AMI and names it according to the CI runtype.
- `bazel run //cmd/executor/vm-image:ami.push` takes the AMIs from above and publish them (adjust perms, naming).

While `gcloud` is provided by Bazel, AWS cli is expected to be available on the host running Bazel.

Building AMIs on GCP is rather quick, but it's notoriously slow on AWS (about 20m) so we use [target-determinator](https://github.com/bazel-contrib/target-determinator) to detect when to rebuild the image. See [ci-should-rebuild.sh](./ci-should-rebuild.sh), which is used by the pipeline generator to skip building it if we detect that nothing changed since the parent commit.

### Docker Mirror

As with the VM image, we're producing an AMI for both AWS and GCP, there are two steps involved:

- `bazel run //cmd/executor/docker-mirror:ami.build` creates the AMI and names it according to the CI runtype.
- `bazel run //cmd/executor/docker-mirror:ami.push` takes the AMIs from above and publish them (adjust perms, naming).

While `gcloud` is provided by Bazel, AWS cli is expected to be available on the host running Bazel.
Hello World
