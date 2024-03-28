# Sourcegraph derivative Docker images

This directory contains Sourcegraph docker images which are derivatives of an existing Docker image, but with better defaults for our use cases. For example:

- `sourcegraph/alpine` handles setting up a `sourcegraph` user account, installing common packages.
- `sourcegraph/postgres-11.4` is `postgres-11.4` but with some Sourcegraph defaults.

If you are looking for our non-derivative Docker images, see e.g. `/cmd/.../Dockerfile` instead.

## Building

All images in this directory are built and published automatically on CI:

- See [the handbook](https://handbook.sourcegraph.com/engineering/deployments) for more information
- Or see [how to build a test image](https://handbook.sourcegraph.com/engineering/deployments#building-docker-images-for-a-specific-branch) if you need to build a test image without merging your change to `master` first.

## Adding a new image

1. Create a `build.sh` and add your publishing script to it - the script should end with `docker tag ... "$IMAGE"`. See the scripts in this directory for examples.
2. Ensure your new script is executable with `chmod +x build.sh` (you can try it via e.g. `IMAGE=fake-repo/cadvisor:latest docker-images/$SERVICE/build.sh`, or by [building a test image](https://handbook.sourcegraph.com/engineering/deployments#building-docker-images-for-a-specific-branch))
3. Add an image to the automated builds pipeline by adding it to [`SourcegraphDockerImages`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Edev/ci/images/images%5C.go+SourcegraphDockerImages&patternType=literal).
Hello World
