# FAQ

## General

### `sg bazel configure` prints out a warning about TSConfig

Everytime you run `sg bazel configure`, you'll see a warning:

```
$ sg bazel configure
Updating BUILD files for protobuf, go, javascript
2023/11/16 12:32:52 Failed to load base tsconfig file @sourcegraph/tsconfig: open /Users/thorstenball/work/sourcegraph/@sourcegraph/tsconfig: no such file or directory
```

The warning can be ignored, and [is a known bug](https://github.com/aspect-build/aspect-cli/issues/524).

### The analysis cache is being busted because of `--action_env`

Typically you'll see this (in CI or locally):

```
INFO: Build option --action_env has changed, discarding analysis cache.
```

- If you added a `build --action_env=VAR` to one of the `bazelrc`s, and `$VAR` is not stable across builds, it will break the analysis cache. You should never pass a variable that is not stable, otherwise, the cache being busted is totally expected and there is no way around it.
  - Use `build --action_env=VAR=123` instead to pin it down if it's not stable in your environment.
- If you added a `test --action_env=VAR`, running `bazel build [...]` will have a different `--action_env` and because the analysis cache is the same for `build` and `test` that will automatically bust the cache.
  - Use `build --test_env=VAR` instead, so that env is used only in tests, and doesn't affect builds, while avoiding to bust the cache.

### My JetBrains IDE becomes unresponsive after Bazel builds

By default, JetBrains IDEs such as GoLand will try and index the files in your project workspace. If you run Bazel locally, the resulting artifacts will be indexed, which will likely hog the full heap size that the IDE is allocated.

There is no reason to index these files, so you can just exclude them from indexing by right-clicking artifact directories, then choosing **Mark directory as** &rarr; **Excluded** from the context menu. A restart is required to stop the indexing process.

### My local `sg bazel configure` or `./dev/ci/bazel-prechecks.sh` run has diff with a result of Bazel CI step

This could happen when there are any files which are not tracked by Git. These files affect the run of `sg bazel configure` and typically add more items to `BUILD.bazel` file.

Solution: run `git clean -ffdx` then run `sg bazel configure` again.

### How do I clean up all local Bazel cache?

1. The simplest way to clean up the cache is to use the clean command: `bazel clean`. This command will remove all output files and the cache in the bazel-* directories within your workspace. Use the `--expunge` flag to remove the entire working tree, including the cache directory, and force a full rebuild.
2. To manually clear the global Bazel cache, you need to remove the respective folders from your machine. On macOS, the global cache is typically located at either `~/.cache/bazel` or `/var/tmp/_bazel_$(whoami)`.

### Where do I find Bazel rules locally on disk?

Use `bazel info output_base` to find the output base directory. From there go to the `external` folder to find Bazel rules saved locally.

### How do I build a container on MacOS

Our containers are only built for `linux/amd64`, therefore we need to cross-compile on MacOS to produce correct images. This is automatically handled by Bazel, so there should be no difference between the command to build containers on Linux and MacOS.

Example:

```
# Create and load a tarball that can be loaded in Docker of the worker service:
bazel run //cmd/worker:image_tarball
```

You can also use the same configuration flag to run the container tests on MacOS:

```
bazel test //cmd/worker:image_test
```

### I am not able to run image tests locally as Docker is not detected

If you get an error like:

```
time="2024-02-28T06:31:07Z" level=fatal msg="error loading oci layout into daemon: error loading image: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?, %!s(MISSING)"
```

It might be because Docker Desktop was configured to use a different Docker socket during installation.

To fix this error, enable the checkbox under **Docker Desktop > Settings > Advanced > Allow the default Docker socket to be used (requires password)** and then restart Docker.

### Can I run integration tests (`bazel test //testing/...`) locally?

At the time of writing this documentation, it's not possible to do so, because we need to cross compile to produce `linux/amd64` container images, but the test runners need to run against your host architecture. If your host isn't `linux/amd64` you won't be able to run those tests.

This is caused by the fact that there is no straightforward way of telling Bazel to use a given toolchain for certain targets and another one for others, in a consistent fashion across the various binaries we produce (rust+go).

See [this issue](https://github.com/sourcegraph/sourcegraph/issues/52914) to track progress on this particular problem.

### When using nix, when protobuf gets compiled I get a C/C++ compiler issue `fatal error: 'algorithm' file not found` or some core header files are not found
Nix sets the CC environment variable to a clang version use by nix which is independent of the host system. You can verify this by running the following commands in your nix shell.
```
$ echo $CC
clang

$ which $CC
/nix/store/agjhf1m0xsvmdjkk8kc7bp3pic9lsfrb-clang-wrapper-11.1.0/bin/clang

$ cat bazel-sourcegraph/external/local_config_cc/cc_wrapper.sh | grep "# Call the C++ compiler" -A 2
# Call the C++ compiler
/nix/store/agjhf1m0xsvmdjkk8kc7bp3pic9lsfrb-clang-wrapper-11.1.0/bin/clang "$@"
```
Bazel runs a target called `locate_cc_config` which adheres to the CC environment variable. The variable defines the compiler to be used to perform C/C++ compilation. At time of writing, the compiler is incorrectly configured and the stdlib doesn't get referenced properly. Therefore, we currently recommend to unset the `CC` variable in your nix shell. The `locate_cc_config` will then find the system C/C++ compiler (which on my system resolved to `/usr/bin/gcc`) and compile protobuf.

You can also verify that the correct compiler is used by running the following command:
```
cat bazel-sourcegraph/external/local_config_cc/cc_wrapper.sh | grep "# Call the C++ compiler" -A 2
# Call the C++ compiler
/usr/bin/gcc "$@"
```

### On MacOS, you get `Bad CPU type for executable` when running Bazel

This typically happens when `bazel` tries to use `protoc` to compile protobuf definitions and you're on an arm64 mac. The full error will look like the following:
```
src/main/tools/process-wrapper-legacy.cc:80: "execvp(bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/com_google_protobuf_protoc_macos_aarch64/protoc.exe, ...)": Bad CPU type in executable
ERROR: /private/var/tmp/_bazel_ec2-user/c27d26456d2e68ea0aaccfcda2d35b4e/external/go_googleapis/google/api/BUILD.bazel:22:14: Generating Descriptor Set proto_library @go_googleapis//google/api:api_proto failed: (Exit 1): protoc.exe failed: error executing command (from target @go_googleapis//google/api:api_proto) bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/com_google_protobuf_protoc_macos_aarch64/protoc.exe --direct_dependencies google/api/launch_stage.proto ... (remaining 5 arguments skipped)
```

If you run the `file` utility on the `protoc` binary you'll see the CPU architecture mismatch.
```
file bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/com_google_protobuf_protoc_macos_aarch64/protoc.exe
bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/com_google_protobuf_protoc_macos_aarch64/protoc.exe: Mach-O 64-bit executable x86_64
```

To fix this, you need to install Rosetta 2. There are various ways of doing that, but below is the CLI way to do it.
```
/usr/sbin/softwareupdate --install-rosetta --agree-to-license
```

__Tested on Darwin 22.3.0 Darwin Kernel Version 22.3.0__

### On MacOS, your build fails with `xcrun failed with code 1. This most likely indicates that SDK version [10.10] for platform [MacOSX] is unsupported for the target version of xcode.`

Bazel uses `xcrun` to locate the SDK and toolchain for iOS/Mac compilation and xcrun is fails to produce a version or it cannot find the correct directory. This might happen even if `xcrun --show-sdk-path` shows a valid path and `xcrun --show-sdk-version` shows a valid version. At the time of this entry we haven't found the exact cause of this issue and various other bazel projects have encountered the same issue.

Nonetheless, there is a workaround! Pass the following CLI flag when you try to build a target `--macos_sdk_version=13.3`. With the flag bazel should be able to find the MacOS SDK and you should not get the error anymore. It's recommended to add `build --macos_sdk_version=13.3` to your `.bazelrc` file so that you don't have to add the CLI flag every time you invoke a build.

### I see `error: unable to open mailmap at .mailmap: Too many levels of symbolic links` when running my `bazel run` target (both locally and in CI)

If you see this, it most probably means that you have a `bazel run //something` that calls `git log`. Git will look for a `.mailmap` at the root of the repository, which we do have in the monorepo. Because
`bazel run` runs commands with a working directory which is in the runfiles, symbolic links are getting in the way.

While it says "error", it's to be noted that it doesn't prevent the Git command to continue.

The fix is pretty simple, assuming that you do have a `git log` command in the script that `bazel run //something` calls. Adding the `--no-mailmap` flag will prevent the error to show up:

```
git log --no-mailmap (...)
```

Please keep in mind that it will prevent `git` to properly map the email addresses for a few individuals, though in most cases that shouldn't be a problem as that's not what we're looking for when using `git log` in scripts.

## Queries

Bazel queries (`bazel query`, `bazel cquery` and `bazel aqueries`) are powerful tools that can assist you to visualize dependencies and understand how targets are being built or tested.

### Errors about not being able to fetch a manifest for an OCI rule

When running the following query:

```
bazel query 'kind("go_binary", rdeps(//..., //internal/database/migration/cliutil))'
```

We get the following error.

```
WARNING: Could not fetch the manifest. Either there was an authentication issue or trying to pull an image with OCI image media types.
Falling back to using `curl`. See https://github.com/bazelbuild/bazel/issues/17829 for the context.
INFO: Repository wolfi_redis_base_single instantiated at:
  /home/noah/Sourcegraph/sourcegraph/WORKSPACE:376:9: in <toplevel>
  /home/noah/Sourcegraph/sourcegraph/dev/oci_deps.bzl:73:13: in oci_deps
  /home/noah/.cache/bazel/_bazel_noah/8fd1d20666a46767e7f29541678514a0/external/rules_oci/oci/pull.bzl:133:18: in oci_pull
Repository rule oci_pull defined at:
  /home/noah/.cache/bazel/_bazel_noah/8fd1d20666a46767e7f29541678514a0/external/rules_oci/oci/private/pull.bzl:434:27: in <toplevel>
WARNING: Download from https://us.gcr.io/v2/sourcegraph-dev/wolfi-redis-base/manifests/sha256:08e80c858fe3ef9b5ffd1c4194a771b6fd45f9831ad40dad3b5f5b53af880582 failed: class com.google.devtools.build.lib.bazel.repository.downloader.UnrecoverableHttpException GET returned 401 Unauthorized
ERROR: An error occurred during the fetch of repository 'wolfi_redis_base_single':
   Traceback (most recent call last):
        File "/home/noah/.cache/bazel/_bazel_noah/8fd1d20666a46767e7f29541678514a0/external/rules_oci/oci/private/pull.bzl", line 357, column 46, in _oci_pull_impl
                mf, mf_len = downloader.download_manifest(rctx.attr.identifier, "manifest.json")
```

This query requires to analyse external dependencies for the [base images](containers.md), which are not yet publicly available.

Solution: `gcloud auth configure-docker us.gcr.io` to get access to the registry.

Solution: Pass the `--keep_going` additional flag to your `bazel query` command, so the evaluation doesn't stop at the first error.

## Networking

### Tests fail with `connection refused`
Any tests that make network calls on `localhost` need to be reachable from your Bazel build and test environment. If tests fail with errors like `error="dial tcp 127.0.0.1:6379: connect: connection refused"`, you most likely have to allow outbound networking for the sandbox environment.

This can be achieved by adding the attribute `tags = ["requires-network"]` to the `go_test` rule in the `BUILD.bazel` file of the test directory.

> NOTE: make sure to run `sg bazel configure` after adding the tag as it will probably move it to another line. Save yourself a failing build!

## Go

### It complains about some missing symbols, but I'm sure they are there since I can see my files

```
ERROR: /Users/tech/work/sourcegraph/internal/redispool/BUILD.bazel:3:11: GoCompilePkg internal/redispool/redispool.a failed: (Exit 1): builder failed: error executing command (from target //internal/redispool:redispool) bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/go_sdk/builder compilepkg -sdk external/go_sdk -installsuffix darwin_arm64 -src internal/redispool/redispool.go -src internal/redispool/sysreq.go ... (remaining 30 arguments skipped)

Use --sandbox_debug to see verbose messages from the sandbox and retain the sandbox build root for debugging
internal/redispool/redispool.go:78:13: undefined: RedisKeyValue
internal/redispool/redispool.go:94:13: undefined: RedisKeyValue
```

OR

```
~/work/sourcegraph U bzl/build-go $ bazel build //dev/sg
INFO: Analyzed target //dev/sg:sg (955 packages loaded, 16719 targets configured).
INFO: Found 1 target...
ERROR: /Users/tech/work/sourcegraph/internal/conf/confdefaults/BUILD.bazel:3:11: GoCompilePkg internal/conf/confdefaults/confdefaults.a failed: (Exit 1): builder failed: error executing command (from target //internal/conf/confdefaults:confdefaults) bazel-out/darwin_arm64-opt-exec-2B5CBBC6/bin/external/go_sdk/builder compilepkg -sdk external/go_sdk -installsuffix darwin_arm64 -src internal/conf/confdefaults/confdefaults.go -embedroot '' -embedroot ... (remaining 19 arguments skipped)

Use --sandbox_debug to see verbose messages from the sandbox and retain the sandbox build root for debugging
compilepkg: missing strict dependencies:
	/private/var/tmp/_bazel_tech/3eea80c6015362974b7d423d1f30cb62/sandbox/darwin-sandbox/10/execroot/__main__/internal/conf/confdefaults/confdefaults.go: import of "github.com/russellhaering/gosaml2/uuid"
No dependencies were provided.
Check that imports in Go sources match importpath attributes in deps.
Target //dev/sg:sg failed to build
Use --verbose_failures to see the command lines of failed build steps.
INFO: Elapsed time: 11.559s, Critical Path: 2.93s
INFO: 36 processes: 2 internal, 34 darwin-sandbox.
```


Solution: run `sg bazel configure` to update the buildfiles automatically.

### My go tests complains about missing testdata

In the case where your testdata lives in `../**`, Gazelle cannot see those on its own, and you need to create a filegroup manually, see https://github.com/sourcegraph/sourcegraph/pull/47605/commits/93c838aad5436dc69f6695cec933bfb84b8ba59a

### My go tests are timing out in CI but there is no output telling me where exactly it failed

By defaults, Go tests are run without the `-v` flag, which means that Go will only print a summary when the testing is complete or has failed. But in the case of Bazel timeouts, i.e. when the test target
has a time out on the Bazel side (`short` by default, so 60 seconds) Bazel will kill the Go test binary before it had the chance to flush out its outputs. As a result, you'll see empty logs, which is
very incovenient for debugging.

Solution: run `bazel test --config go-verbose-test` to force Bazel to run the tests with the verbose flag on for any `go_test` rule it encounters. If this only happens in CI, you can combine this solution with the _bazel-do_ feature of `sg`
which enables to fire a build running a single, specific test that you provide:

```
sg ci bazel test --config go-verbose-test //my/timing-out:target
```

This will print out the URL of the newly created build and the logs will show you exactly where the tests were when they timed out.

### Manually adding a `go_repository`

Sometimes Gazelle won't be able to generate a `go_repository` for your dependency and you'll need to fill in the attributes yourself. Most of the fields are easy to get, except when you need to provide values for the sum and version.

To retrieve these values:
1. Create a go.mod in the directory where the dependency is imported.
2. Run `go mod tidy`. This will populate the `go.mod` file and also generate a `go.sum` file.
3. You can then locate the version you should use for `go_repository` from the `go.mod` file and the sum from the `go.sum` file.
4. Delete the `go.mod` and `go.sum` files as they're no longer needed.

### How to update to the latest recommended bazelrc?

```
bazel run //.aspect/bazelrc:update_aspect_bazelrc_presets
```

## Rust

### I'm getting `Error in path: Not a regular file: docker-images/syntax-highlighter/Cargo.Bazel.lock` when I try to build `syntax-highlighter`

Below is a full example of this error:
```
ERROR: An error occurred during the fetch of repository 'crate_index':
   Traceback (most recent call last):
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/crates_repository.bzl", line 34, column 30, in _crates_repository_impl
                lockfiles = get_lockfiles(repository_ctx)
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/generate_utils.bzl", line 311, column 36, in get_lockfiles
                bazel = repository_ctx.path(repository_ctx.attr.lockfile) if repository_ctx.attr.lockfile else None,
Error in path: Not a regular file: /Users/william/code/sourcegraph/docker-images/syntax-highlighter/Cargo.Bazel.lock
ERROR: /Users/william/code/sourcegraph/WORKSPACE:197:18: fetching crates_repository rule //external:crate_index: Traceback (most recent call last):
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/crates_repository.bzl", line 34, column 30, in _crates_repository_impl
                lockfiles = get_lockfiles(repository_ctx)
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/generate_utils.bzl", line 311, column 36, in get_lockfiles
                bazel = repository_ctx.path(repository_ctx.attr.lockfile) if repository_ctx.attr.lockfile else None,
Error in path: Not a regular file: /Users/william/code/sourcegraph/docker-images/syntax-highlighter/fake.lock
ERROR: Error computing the main repository mapping: no such package '@crate_index//': Not a regular file: /Users/william/code/sourcegraph/docker-images/syntax-highlighter/Cargo.Bazel.lock
```
The error happens when the file specified in the lockfiles attribute of `crates_repository` (see WORKSPACE file for the definition) does not exist on disk. Currently this rule does not generate the file, instead it just generates the _content_ of the file. So to get passed this error you should create the file `touch docker-images/syntax-highlighter/Cargo.Bazel.lock`. With the file create it we can now populate `Cargo.Bazel.lock` with content using bazel by running `sg bazel configure rustdeps`.

### When I build `syntax-highlighter` it complains that the current `lockfile` is out of date
The error will look like this:
```
INFO: Repository crate_index instantiated at:
  /Users/william/code/sourcegraph/WORKSPACE:197:18: in <toplevel>
Repository rule crates_repository defined at:
  /private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/crates_repository.bzl:106:36: in <toplevel>
INFO: repository @crate_index' used the following cache hits instead of downloading the corresponding file.
 * Hash 'dc2d47b42cbe92ebdb144555603dad08eae505fc459bae5e2503647919067ac8' for https://github.com/bazelbuild/rules_rust/releases/download/0.16.1/cargo-bazel-aarch64-apple-darwin
If the definition of 'repository @crate_index' was updated, verify that the hashes were also updated.
ERROR: An error occurred during the fetch of repository 'crate_index':
   Traceback (most recent call last):
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/crates_repository.bzl", line 45, column 28, in _crates_repository_impl
                repin = determine_repin(
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/generate_utils.bzl", line 374, column 13, in determine_repin
                fail(("\n".join([
Error in fail: Digests do not match: Digest("3e9e0f927c955efa39a58c472a2eac60e3f89a7f3eafc7452e9acf23adf8ce5a") != Digest("ef858ae49063d5c22e0ee0b7632a8ced4994315395b17fb3c61f3e6bfb6deb27")

The current `lockfile` is out of date for 'crate_index'. Please re-run bazel using `CARGO_BAZEL_REPIN=true` if this is expected and the lockfile should be updated.
ERROR: /Users/william/code/sourcegraph/WORKSPACE:197:18: fetching crates_repository rule //external:crate_index: Traceback (most recent call last):
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/crates_repository.bzl", line 45, column 28, in _crates_repository_impl
                repin = determine_repin(
        File "/private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/rules_rust/crate_universe/private/generate_utils.bzl", line 374, column 13, in determine_repin
                fail(("\n".join([
Error in fail: Digests do not match: Digest("3e9e0f927c955efa39a58c472a2eac60e3f89a7f3eafc7452e9acf23adf8ce5a") != Digest("ef858ae49063d5c22e0ee0b7632a8ced4994315395b17fb3c61f3e6bfb6deb27")

The current `lockfile` is out of date for 'crate_index'. Please re-run bazel using `CARGO_BAZEL_REPIN=true` if this is expected and the lockfile should be updated.
ERROR: Error computing the main repository mapping: no such package '@crate_index//': Digests do not match: Digest("3e9e0f927c955efa39a58c472a2eac60e3f89a7f3eafc7452e9acf23adf8ce5a") != Digest("ef858ae49063d5c22e0ee0b7632a8ced4994315395b17fb3c61f3e6bfb6deb27")

The current `lockfile` is out of date for 'crate_index'. Please re-run bazel using `CARGO_BAZEL_REPIN=true` if this is expected and the lockfile should be updated.
```
Bazel uses a separate lock file to track the dependencies, which need to be updated. To update the `lockfile` run `CARGO_BAZEL_REPIN_ONLY=crate_index sg bazel configure rustdeps`. This command takes a while to execute as it fetches all the dependencies specified in `Cargo.lock` and populates `Cargo.Bazel.lock`. This command _might also fail_ in that case [see](#bazel-sync-authentication-failure-when-cloning-syntect).

### `syntax-highlighter` fails to build and has the error `failed to resolve: use of undeclared crate or module`
The error looks something like this:
```
error[E0433]: failed to resolve: use of undeclared crate or module `tree_sitter_all_languages`
  --> docker-images/syntax-highlighter/src/main.rs:57:15
   |
57 |         .get(&tree_sitter_all_languages::ParserId::Go);
   |               ^^^^^^^^^^^^^^^^^^^^^^^^^ use of undeclared crate or module `tree_sitter_all_languages`

error: aborting due to 2 previous errors
```
Bazel doesn't know about the module/crate being use in the rust code. If you do a git blame `Cargo.toml` you'll probably see that a new dependency has been added, but the build files were not updated. There are two ways to solve this:
1. Run `sg bazel configure builds rustdeps`. Once the commands have completed you can check that the dependency has been picked up and syntax-highlighter can be built by running `bazel build //docker-images/syntax-highlighter/...`. **Note** this will usually work if the dependency is an *external* dependency.
2. You're going to have to update the `BUILD.bazel` file yourself. Which one you might ask? From the above error we can see the file `src/main.rs` is where the error is encountered, so we need to tell *its BUILD.bazel* about the new dependency.
For the above dependency, the crate is defined in `docker-images/syntax-highlighter/crates`. You'll also see that each of those crates have their own `BUILD.bazel` files in them, which means we can reference them as targets! Take a peak at `tree-sitter-all-languages` `BUILD.bazel` file and take note of the name - that is its target. Now that we have the name of the target we can add it as a dep to `docker-images/syntax-highlighter`. In the snippet below the `syntax-highlighter` `rust_binary` rule is updated with the `tree-sitter-all-languages` dependency. Note that we need to refer to the full target path when adding it to the dep list in the `BUILD.bazel` file.
```
rust_binary(
    name = "syntect_server",
    srcs = ["src/main.rs"],
    aliases = aliases(),
    proc_macro_deps = all_crate_deps(
        proc_macro = True,
    ),
    deps = all_crate_deps(
        normal = True,
    ) + [
        "//docker-images/syntax-highlighter/crates/sg-syntax:sg-syntax",
        "//docker-images/syntax-highlighter/crates/tree-sitter-all-languages:tree-sitter-all-languages",
    ],
)
```

### `bazel sync` authentication failure when cloning `syntect`

When repinning dependencies with `CARGO_BAZEL_REPIN_ONLY=crate_index sg bazel configure rustdeps` it may fail with the following Cargo error:

```
STDERR ------------------------------------------------------------------------

    Updating crates.io index
    Updating git repository `https://github.com/sourcegraph/syntect`
error: failed to get `syntect` as a dependency of package `sg-syntax v0.1.0 (/var/folders/1r/5z42n9p52zv8rfp93gxc1vfr0000gn/T/.tmpyhlucq/crates/sg-syntax)`

Caused by:
  failed to load source for dependency `syntect`

Caused by:
  Unable to update https://github.com/sourcegraph/syntect?rev=7e02c5b4085e6d935b960b8106cdd85da04532d2#7e02c5b4

Caused by:
  failed to clone into: /private/var/tmp/_bazel_william/c92ec739369034d3064b6df55c419545/external/crate_index/.cargo_home/git/db/syntect-383b2f29eb0ef0d0

Caused by:
  failed to authenticate when downloading repository: ssh://git@github.com/sourcegraph/syntect

  * attempted ssh-agent authentication, but no usernames succeeded: `git`

  if the git CLI succeeds then `net.git-fetch-with-cli` may help here
  https://doc.rust-lang.org/cargo/reference/config.html#netgit-fetch-with-cli

Caused by:
  no authentication methods succeeded

Error: Failed to update lockfile: exit status: 101
```

You might be able to Git SSH clone that repository locally, yet in Bazel Cargo, it fails. This is because Bazel Cargo doesn't use your `~/.ssh/config` file and thus can't use your SSH private key. The error says you can set `net.git-fetch-with-cli` in `Cargo.toml` or configure a credential helper. All cargo settings also have environment variable variants, so you can do the repinning with `CARGO_NET_GIT_FETCH_WITH_CLI=true sg bazel configure rustdeps` without setting the value in the `Cargo.toml`.

## Docs

### `//docs:test` is not finding my documents after I added `BUILD.bazel` file in a child directory

When you add a `BUILD.bazel` to a directory Bazel will start recognizing that directory as a package. By default
nothing is exposed from the package - Yes, even plain files. So you need to tell Bazel that you would like to expose
the files in the directory to the outside world / other targets by using a filegroup target:

```
filegroup(
  name = "my_files",
  srcs = glob(
    [**/*],
  visibility = ["//doc:__pkg__"], # only targets in the //doc package can use it
)
```

We can see that all the docs are exposed by our doc by running `bazel cquery //<path/to/my/target>:my_files --output=files` - example:

```
bazel cquery "//doc/cli/references:doc_files" --output=files

INFO: Analyzed target //doc/cli/references:doc_files (100 packages loaded, 439 targets configured).
INFO: Found 1 target...
doc/cli/references/BUILD.bazel
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/admin.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/api.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/apply.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/exec.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/index.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/new.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/preview.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/remote.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/repositories.md
bazel-out/darwin_arm64-fastbuild/bin/doc/cli/references/batch/validate.md
<snip>
```

Now that our files are exposed by a target, we need to tell Bazel to expose it to `//doc:test`. In `/doc/BUILD.bazel`
update the `data` attribute:

```
sh_test(
    name = "test",
    size = "small",
    timeout = "moderate",
    srcs = ["test.sh"],
    args = ["$(location //dev/tools:docsite)"],
    data = [
        "//dev/tools:docsite",
        "//doc/cli/references:doc_files",
        "//doc/mydocs:my_files, # our target
    ] + glob(
        ["**/*"],
        ["test.sh"],
    ),
    tags = [
        "requires-network",
    ],
)
```
