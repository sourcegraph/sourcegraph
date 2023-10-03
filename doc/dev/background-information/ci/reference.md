<!-- DO NOT EDIT: generated via: go generate ./enterprise/dev/ci -->

# Pipeline types reference

This is a reference outlining what CI pipelines we generate under different conditions.

To preview the pipeline for your branch, use `sg ci preview`.

For a higher-level overview, please refer to the [continuous integration docs](https://docs.sourcegraph.com/dev/background-information/ci).

## Run types

### Pull request

The default run type.

- Pipeline for `Go` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `ClientJetbrains` changes:
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Client` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
  - **Security Scanning**: Sonarcloud Scan
  - **Pipeline setup**: Trigger async

- Pipeline for `GraphQL` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `DatabaseSchema` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Docs` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Dockerfiles` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `ExecutorVMImage` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `ExecutorDockerRegistryMirror` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `CIScripts` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Terraform` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `SVG` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Shell` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `DockerImages` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `WolfiPackages` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan
  - **Dependency packages**: Build and sign repository index

- Pipeline for `WolfiBaseImages` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Security Scanning**: Sonarcloud Scan

- Pipeline for `Protobuf` changes:
  - Perform bazel prechecks
  - Tests
  - Async BackCompat Tests
  - **Linters and static analysis**: Run sg lint
  - **Security Scanning**: Sonarcloud Scan

### Manually Triggered External Build

The run type for branches matching `_manually_triggered_external/`.
You can create a build of this run type for your changes using:

```sh
sg ci build _manually_triggered_external
```

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Push final images

### Release branch nightly healthcheck build

The run type for environment including `{"RELEASE_NIGHTLY":"true"}`.

Base pipeline (more steps might be included based on branch changes):

- Trigger 5.1 release branch healthcheck build
- Trigger 5.0 release branch healthcheck build

### Browser extension nightly release build

The run type for environment including `{"BEXT_NIGHTLY":"true"}`.

Base pipeline (more steps might be included based on branch changes):

- Test (client/browser)
- Puppeteer tests for chrome extension
- Test (all)
- E2E for chrome extension

### VS Code extension nightly release build

The run type for environment including `{"VSCE_NIGHTLY":"true"}`.

Base pipeline (more steps might be included based on branch changes):

- Tests for VS Code extension

### App release build

The run type for branches matching `app/release` (exact match).

Base pipeline (more steps might be included based on branch changes):

- App release

### App insiders build

The run type for branches matching `app/insiders` (exact match).

Base pipeline (more steps might be included based on branch changes):

- App release

### Tagged release

The run type for tags starting with `v`.

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images, Build executor image, Build executor binary, Build docker registry mirror image
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Publish executor image, Publish executor binary, Publish docker registry mirror image, Push final images

### Release branch

The run type for branches matching `^[0-9]+\.[0-9]+$` (regexp match).

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images, Build executor image, Build executor binary, Build docker registry mirror image
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Push final images

### Browser extension release build

The run type for branches matching `bext/release` (exact match).

Base pipeline (more steps might be included based on branch changes):

- Test (client/browser)
- Puppeteer tests for chrome extension
- Test (all)
- E2E for chrome extension
- Extension release
- Extension release
- npm Release

### VS Code extension release build

The run type for branches matching `vsce/release` (exact match).

Base pipeline (more steps might be included based on branch changes):

- Tests for VS Code extension
- Extension release

### Release branch (TODO RFC 795)

The run type for branches matching `^wip_v[0-9]+\.[0-9]+\.[0-9]+$` (regexp match).

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images, Build executor image, Build executor binary
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Push final images

### Main branch

The run type for branches matching `main` (exact match).

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images, Build executor image, Build executor binary
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Publish executor image, Publish executor binary, Push final images

### Main dry run

The run type for branches matching `main-dry-run/`.
You can create a build of this run type for your changes using:

```sh
sg ci build main-dry-run
```

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images, Build executor image, Build executor binary
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Push final images

### Patch image

The run type for branches matching `docker-images-patch/`, requires a branch argument in the second branch path segment.
You can create a build of this run type for your changes using:

```sh
sg ci build docker-images-patch
```

### Patch image without testing

The run type for branches matching `docker-images-patch-notest/`, requires a branch argument in the second branch path segment.
You can create a build of this run type for your changes using:

```sh
sg ci build docker-images-patch-notest
```

### Build executor without testing

The run type for branches matching `executor-patch-notest/`.
You can create a build of this run type for your changes using:

```sh
sg ci build executor-patch-notest
```

Base pipeline (more steps might be included based on branch changes):

- Build executor-vm
- Scan executor-vm
- Build executor image
- Build docker registry mirror image
- Build executor binary
- executor-vm
- Publish executor image
- Publish docker registry mirror image
- Publish executor binary

### Build all candidates without testing

The run type for branches matching `docker-images-candidates-notest/`.
You can create a build of this run type for your changes using:

```sh
sg ci build docker-images-candidates-notest
```

Base pipeline (more steps might be included based on branch changes):

- **Image builds**: Build Docker images
- **Publish images**: Push final images, dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine

### Backend integration tests

The run type for branches matching `backend-integration/`.
You can create a build of this run type for your changes using:

```sh
sg ci build backend-integration
```

Base pipeline (more steps might be included based on branch changes):

- **Pipeline setup**: Trigger async
- **Image builds**: Build Docker images
- Perform bazel prechecks
- Tests
- Async BackCompat Tests
- **Linters and static analysis**: Run sg lint
- **Client checks**: Upload Storybook to Chromatic, Enterprise build, Build (client/jetbrains), Tests for VS Code extension, Stylelint (all)
- **Security Scanning**: Sonarcloud Scan
- **Publish candidate images**: Push candidate Images
- **End-to-end tests**: Executors E2E
- **Publish images**: dind, executor-vm, alpine-3.14, codeinsights-db, codeintel-db, postgres-12-alpine, Push final images

### Bazel command

The run type for branches matching `bazel-do/`.
You can create a build of this run type for your changes using:

```sh
sg ci build bazel-do
```
