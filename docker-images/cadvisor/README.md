# Sourcegraph cAdvisor

Learn more about the Sourcegraph cAdvisor distribution in the [cAdvisor documentation](https://sourcegraph.com/docs/dev/background-information/observability/cadvisor).

## Updating the image

The base images for cAdvisor are hosted on [gcr.io/cadvisor/cadvisor](https://gcr.io/cadvisor/cadvisor). Note that the images are not tagged by architecture. Historically, the image with the version tag (e.g. `v0.46.0`) is a multi-arch image which you can verify by selecting it and viewing the manifest. It should contain information about supported architectures, including:

```
"platform": {
  "architecture": "amd64",
  "os": "linux"
}
```

You can also verify this manually:

1. Exec in the container with `docker run -it --entrypoint /bin/sh --platform linux/x86_64 <image name>` (note that `--platform linux/x86_64` is required on Arm-based Macs to ensure it pulls the correct image)
2. Run `uname -a` in the container
3. Ensure it's `x86_64` architecture.

For example:

```
docker run -it --entrypoint /bin/sh --platform linux/x86_64 gcr.io/cadvisor/cadvisor@sha256:8938726fe00fd7a3889f7c4fb50a54b728f1d02fb5f6cbdbea604824ad11ff3f
Unable to find image 'gcr.io/cadvisor/cadvisor@sha256:8938726fe00fd7a3889f7c4fb50a54b728f1d02fb5f6cbdbea604824ad11ff3f' locally
gcr.io/cadvisor/cadvisor@sha256:8938726fe00fd7a3889f7c4fb50a54b728f1d02fb5f6cbdbea604824ad11ff3f: Pulling from cadvisor/cadvisor
df9b9388f04a: Already exists
32357bb75a40: Already exists
4f4fb700ef54: Already exists
a80659b3f11d: Already exists
769b92fe592c: Already exists
6ab71a81e6dc: Already exists
Digest: sha256:8938726fe00fd7a3889f7c4fb50a54b728f1d02fb5f6cbdbea604824ad11ff3f
Status: Downloaded newer image for gcr.io/cadvisor/cadvisor@sha256:8938726fe00fd7a3889f7c4fb50a54b728f1d02fb5f6cbdbea604824ad11ff3f
/ # uname -a
Linux c74b74199c86 5.10.104-linuxkit #1 SMP Thu Mar 17 17:08:06 UTC 2022 x86_64 Linux
```

To update sourcegraph/cadvisor, modify by the digest in the Dockerfile's `FROM:` line to the digest of the multi-arch image you identified above, and update the `com.sourcegraph.cadvisor.version` label.

The image is built by Buildkite CI, so after merging it will be built and deployed automatically.
Hello World
