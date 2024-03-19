# Deploying Sourcegraph executors using Docker Compose

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta and might change in the future.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

A [docker-compose file](https://github.com/sourcegraph/deploy-sourcegraph-docker/blob/master/docker-compose/executors/executor.docker-compose.yaml) is provided to deploy executors standlone, or alongside your existing Sourcegraph deployment.

## Requirements

Privileged containers are required to run executors in docker-compose. This is because executors require access to the docker daemon running on the host.

## Deployment

### Prerequisites

  - Install [Docker Compose](https://docs.docker.com/compose/) on the server
  - Minimum Docker [v20.10.0](https://docs.docker.com/engine/release-notes/#20100) and Docker Compose [v1.29.0](https://docs.docker.com/compose/release-notes/#1290)
  - Docker Swarm mode is **not** supported
  - Clone the [deploy-sourcegraph-docker](https://github.com/sourcegraph/deploy-sourcegraph-docker)
  - Edit the `deploy-sourcegraph-docker/docker-compose/executors/executor.docker-compose.yaml` and update the environment variables
  - Follow the instructions in the `README.md` for more specific deployment instructions.

## Note

Executors deployed via docker-compose do not use [Firecracker](index.md#how-it-works), meaning they require [privileged access](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities) to the docker daemon running on the host.

If you have security concerns, consider deploying via [terraform](deploy_executors_terraform.md) or [installing the binary](deploy_executors_binary.md) directly.

