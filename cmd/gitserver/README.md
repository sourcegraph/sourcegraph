# gitserver

Mirrors repositories from their code host. All other Sourcegraph services talk to gitserver when they need data from git. Requests for fetch operations, however, go through repo-updater.

gitserver exposes an "exec" API over HTTP for running git commands against
clones of repositories. gitserver also exposes APIs for the management of
clones.

The management of clones comprises most of the complexity in gitserver since:

- We want to avoid concurrent clones and fetches of the same repository.
- We want to limit the number of concurrent clones and fetches.
- When adding/removing/modifying a clone, concurrent attempts to run commands
  needs to be gracefully dealt with.
- We need to be robust against the many ways git clones can degrade. (gc,
  interrupted clones)

Additionally we have invested heavily in the observability of
gitserver. Nearly every operation Sourcegraph does runs one or more git
commands. So we have detailed observability in prometheus, net/event,
jaeger, honeycomb and stderr logs.

We normalize repository names when storing them on disk. Always use
`protocol.NormalizeRepo`. The `$GIT_DIR` of a repository is at
`reposRoot/normalized_name/.git`.

When doing an operation on a file or directory which may be concurrently
read/written please use atomic filesystem patterns. This usually involves
heavy use of `os.Rename`. Search for existing uses of `os.Rename` to see
examples.

## Scaling

gitserver's memory usage consists of short lived git subprocesses.

This is an IO and compute heavy service since most Sourcegraph requests will trigger 1 or more git commands. As such we shard requests for a repo to a specific replica. This allows us to horizontally scale out the service.

The service is stateful (maintaining git clones). However, it only contains data mirrored from upstream code hosts.

## Perforce depots

Syncing of Perforce depots is accomplished by either `p4-fusion` or `git p4` (deprecated), both of which clone Perforce depots into Git repositories in `gitserver`.

### p4-fusion in development

To use `p4-fusion` while developing Sourcegraph, there are a couple of options.

#### Docker

[Run `gitserver` in a Docker container](https://sourcegraph.com/docs/dev/background-information/sg#run-gitserver-in-a-docker-container). This is the option that gives an experience closest to a deployed Sourcegraph instance, and will work for any platform/OS on which you're developing (running `sg start`).

#### Bazel

Native binaries are provided through Bazel, built via Nix in [our fork of p4-fusion](https://github.com/sourcegraph/p4-fusion/actions/workflows/nix-build-and-upload.yaml). It can be invoked either through `./dev/p4-fusion-dev` or directly with `bazel run //dev/tools:p4-fusion`.

#### Native binary executable

The `p4-fusion` native binary has been built on Linux and macOS, but is untested on Windows.

Read the [comprehensive instructions](https://sourcegraph.com/docs/dev/background-information/build_p4_fusion).

If you do go the native binary route, you may also want to enable using the wrapper shell script that detects when the process has been killed and outputs an error so that the calling process can handle it.

That wrapper shell script is `p4-fusion-wrapper-detect-kill.sh`, and in order to use it:

1. Rename the `p4-fusion` binary executable to `p4-fusion-binary` and move it to a location in the `PATH`.
1. Copy the shell script `p4-fusion-wrapper-detect-kill.sh` to a location in the `PATH`, renaming it `p4-fusion`.
1. Copy the shell script `process-stats-watcher.sh` to a location in the `PATH`.
1. Ensure all three of those are executable.

After those steps, when a native `gitserver` process runs `p4-fusion`, it will run the wrapper shell script, which will itself run the `p4-fusion-binary` executable, and the `process-stats-watcher.sh` executable.
Hello World
