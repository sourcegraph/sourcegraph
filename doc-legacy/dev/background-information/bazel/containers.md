# Building container images with Bazel

Building containers with Bazel, and using Wolfi for the base images is faster, more reliable and provides much better caching capabilities. This allows us to build the containers in PRs pipelines, not only on the `main` branch. 
You'll find a lot of mentions of [OCI](https://opencontainers.org/) throughout this document, which refers to the standard for container formats and runtime.  

We use [`rules_oci`](https://github.com/bazel-contrib/rules_oci) and [Wolfi](https://github.com/wolfi-dev) to produce the container images. 

See [Bazel at Sourcegraph](./index.md) for general bazel info/setup.

## Why using Bazel to build containers 

Bazel being a build system, it's not just for compiling code, it can produce tarballs and all artefacts that we ship to  
customers. Our initial approach when we migrated to Bazel was to keep the existing Dockerfiles, and simply use the Bazel
produced binaries instead of the old ones. 

This sped up the CI significantly, but because Bazel is not aware of the image building process, every build on the `main` branch was recreating the Docker images, which is a time consuming process. In particular, the server image has been the slowest of them all, as it required to build all other service and to package them into a very large image that also contained all the third parties necessary to run them (such as `git`, `p4`, `universal-ctags`). 

All of these additional steps can fail, due to transient network issues, or a a particular URL becoming unavailable. By switching to Bazel to produce the container images, we are leveraging its reproduceability and cacheability in exactly the same way we do for building binaries. 

This results in more reliable and faster builds, fast enough that we can afford to build those images in PRs as Bazel will cache the result, meaning we don't rebuild them unless we have to, in a deterministic fashion. 

## Anatomy of a Bazel built containers

Containers are composed of multiple layers (conceptually, not talking about container layers): 

- Sourcegraph Base Image 
  - Distroless base, powered by Wolfi
  - Packages common to all services. 
- Per Service Base Image
  - Packages specific to a given service 
- Service specific outputs (`pkg_tar` rules) 
  - Configuration files, if needed
  - Binaries (targeting `linux/amd64`) 
- Image configuration  (`oci_image` rules) 
  - Environment variables 
  - Entrypoint
- Image tests (`container_structure_test` rules) 
  - Check for correct permissions
  - Check presence of necessary packages 
  - Check that binaries are runnable inside the image
- Default publishing target (`oci_push` rules) 
  - They all refer to our internal registry.
  - Please note that only enterprise variant is published. 

The first two layers are handled by Wolfi and the rest if handled by Bazel.

### Wolfi 

See [the dedicated page for Wolfi](../wolfi/index.md). 

### Bazel

Any output that should go in the image has to be declared with a `pkg_tar` rule. Example:

```
go_binary(
    name = "worker",
    embed = [":worker_lib"],
    visibility = ["//visibility:public"],
)

pkg_tar(
    name = "tar_worker",
    srcs = [":worker"],
)
```

Will create a tarball containing the outputs from the `:frontend` target, which can then be added to an image, through the `tars` attribute of the `oci_image` rule. Example: 

```
oci_image(
    name = "image",
    # (...)
    tars = [":tar_worker"],
)
```

We can add this way as many tarballs we want. In practice, it's best to prefer having multiple smaller tarballs instead of of a big one, as it enabled to cache them individually, to avoid having to rebuild all of them on a tiny change. 

The `oci_image` rule is used to express other aspect of the image we're building, such as the `base` image to use, the `entry_point`, environment variables and which `user` should the image run the entry point with. Example: 

```
oci_image(
    name = "image",
    base = "@wolfi_base",
    entrypoint = [
        "/sbin/tini",
        "--",
        "/worker",
    ],
    env = {
        "MYVAR": "foobar",
    },
    tars = [":tar_worker"],
    user = "sourcegraph",
)
```

💡 Convention: we define environment variables on the the `oci_image` rule. We could hypothetically define some of them in the base image, on the Wolfi layer, but we much prefer to have everything easily readable in the Buildfile of a given service. 

The definition for `@wolfi_base` (and other images) is located in [`dev/oci_deps.bzl`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/oci_deps.bzl).  

Once we have an image being defined, we need an additional rule to turn it into a tarball that can be fed to `docker load` and later on be published. It defines the default tags for that image. Example: 

```
oci_tarball(
    name = "image_tarball",
    image = ":image",
    repo_tags = ["worker:candidate"],
)
```

At this point, we can also write [container tests](https://github.com/GoogleContainerTools/container-structure-test), with the `container_structure_test` rule: 

```
container_structure_test(
    name = "image_test",
    timeout = "short",
    configs = ["image_test.yaml"],
    driver = "docker",
    image = ":image",
    tags = ["requires-network"],
)
```

```
schemaVersion: "2.0.0"
commandTests:
  - name: "binary is runnable"
    command: "/worker"
    envVars:
      - key: "SANITY_CHECK"
        value: "true"

  - name: "not running as root"
    command: "/usr/bin/id"
    args:
      - -u
    excludedOutput: ["^0"]
    exitCode: 0
```

We can now build this image _locally_ and run those tests as well (please note that if you're working locally on a Linux/amd64 machine, you don't need the `--config darwin-docker` flag). 

💡 The image building process is much faster than the old Docker build scripts, and because most actions are cached, this makes it very easy to iterate locally on both the image definition and the container structure tests.

Example: 

```
# Create a tarball that can be loaded in Docker of the worker service:
bazel build //cmd/worker:image_tarball --config darwin-docker

# Load the image in Docker: 
docker load --input $(bazel cquery //cmd/worker:image_tarball  --config darwin-docker --output=files)

# Run the container structure tests 
bazel test //cmd/worker:image_test --config darwin-docker
```

Finally, _if and only if_ we want our image to be released on registries, we need to add the `oci_push` rule. It will take care of definining which registry to push on, as well as tagging the image, through a process referred as `stamping` that we will cover a bit further. 

Apart from the `image` attribute which refers to the above `oci_rule`, `repository` refers to the internal (development) registry. Example:


```
oci_push(
    name = "candidate_push",
    image = ":image",
    repository = image_repository("worker"),
)
```

### Pushing images on registries 

Image are never pushed anywhere, unless we are on the `main` branch. Because we are definining a `container_structure_test` rulee, the `bazel test //...` job in CI will always build your image, even in branches. They will just be cached and never pushed.

On the `main` branch (or if branch is `main_dry_run` runtype), a final CI job, named `Push OCI/Wolfi` will select all `oci_push` rules in the repository and will stamp them before finally pushing them on the standard registries.

### Stamping

Stamping refers to the process of marking artifacts in varied ways, so we can identify and how it was produced. It used at various levels in our pipeline, with the most two notables ones being the `Version` global that we ship within all our Go binaries and the image tags. 

Example of stamps for Go rules: 

```
go_library(
    name = "worker_lib",
    # (...) 
    x_defs = {
        "github.com/sourcegraph/sourcegraph/internal/version.version": "{STABLE_VERSION}",
        "github.com/sourcegraph/sourcegraph/internal/version.timestamp": "{VERSION_TIMESTAMP}",
    },
)
```

Here we're stamping the `worker_lib` target by assigning `STABLE_VERSION` to `internal/version.version`. This is the equivalent of:

```
go build \
  -ldflags "-X github.com/sourcegraph/sourcegraph/internal/version.version=$VERSION -X github.com/sourcegraph/sourcegraph/internal/version.timestamp=$(date +%s)" # (...)
```

When we are building and testing our targets, we do not stamp our binaries with any specific versions. This enables to cache all outputs. But when we're releasing them, we do want to stamp them before releasing them in the wild. 

