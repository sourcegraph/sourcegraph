# Overview of the Bazel configuration for client

## Table of contents

This guide goes over the details of the Bazel integration with the client code.
Check out the **Bazel for teammates in a hurry** section [here](./index.md) first.

- [npm dependencies with Bazel](#npm-dependecies-with-bazel)
  - [running Node.js programs with `rules_js`](#running-nodejs-programs-with-rules_js)
- [WORKSPACE file, which inits the Bazel project](#workspace-file)
- [Root BUILD.bazel file, which defines many shared config targets for client packages](#root-build-bazel-file)
  - [`js_library` target definition](#js_library-target-definition)
  - [What is `js_library`](#what-is-js_library)
  - [What is build graph and info providers?](#what-is-build-graph-and-info-providers)
  - [`ts_config`](#ts_config)
  - [npm binary imports](#npm-binary-imports)
    - [Under the hood](#under-the-hood)
- [Gazelle for Javascript/Typescript code](#gazelle-for-javascript-typescript-code)
  - [Gazelle configuration](#gazelle-configuration)
- [Structure of the simplest client package](#structure-of-the-simplest-client-package)
  - [How to create nested BUILD files to leverage caching](#how-to-create-nested-build-files-to-leverage-caching)
- [Client integration tests integration with Bazel](#client-integration-tests-integration-with-bazel)
- [ESLint custom rule implementation for type-aware linting](#eslint-custom-rule-implementation-for-type-aware-linting)
- [Debugging tips](#debugging-tips)

---

## npm dependecies with Bazel

[rules_js](https://docs.aspect.build/rules/aspect_rules_js/docs) is a tool used for managing Node.js programs in Bazel. It simplifies the downloading of third-party libraries using Bazel's built-in remote downloader and "[repository cache](https://bazel.build/run/build#repository-cache)". This works by converting the dependencies from `pnpm-lock.yaml` into Bazel's internal format, allowing Bazel to handle the downloads.

### Running Node.js programs with `rules_js`

Bazel operates with a different file layout than Node.js. The challenge is to make Bazel and Node.js work together 
without re-writing npm packages or disrupting Node.js resolution algorithm. `rules_js` accomplishes this by running 
JavaScript tools in the Bazel output tree. It uses `pnpm` to create a `node_modules` directory in `bazel-out`, 
enabling smooth resolution of dependencies.

Basically, any bazel rule-sets (in our example `rules_js`) serve as entry points/targets for bazel build, in order
to create an internal build-graph and by this to establish a right cache and output generation. You can think about 
this as entry points in bundlers world like esbuild (but of course rules in bazel can be more complex
rather just entry points, it could be macros, custom rules with some additional effect, etc).

This approach comes with several benefits, like solving TypeScript's `rootDirs` issue. However, it does require 
Bazel rules/macro authors to adjust paths and ensure sources are copied to `bazel-out` before execution. 
This requires setting a `BAZEL_BINDIR` environment variable for each action.

## Workspace file

The workspace file serves two main purposes:

1. **Project Identification**: The presence of a WORKSPACE file in a directory identifies that directory as the root of 
a Bazel workspace. It's a marker file, telling Bazel that the directory and its subdirectories contain a software project
that can be built and tested. Without the WORKSPACE file, Bazel wouldn't know which directory constitutes the root of 
the project.

2. **Dependency Management**: The WORKSPACE file is where you can define external dependencies your project might
need. You specify these dependencies with workspace rules. These rules tell Bazel where to find and how to build the 
dependencies.

For example:

```py
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "aspect_rules_js",
    sha256 = "0b69e0967f8eb61de60801d6c8654843076bf7ef7512894a692a47f86e84a5c2",
    strip_prefix = "rules_js-1.27.1",
    url = "https://github.com/aspect-build/rules_js/releases/download/v1.27.1/rules_js-v1.27.1.tar.gz",
)
```

1. `load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")`: loads the `http_archive` rule 
from the Bazel built-in tools.
2. `http_archive(...)` downloads a specific version of the `rules_js` package from the provided URL, 
checks that its SHA-256 checksum matches the expected value, strips the leading directory from the paths of
the files in the package, and then makes the package available under the name `aspect_rules_js`.
This external repository can then be used in BUILD files, which describe how individual software components 
are built and tested. This snippet is usually copy-pasted from the releases section of rule 
(e.g., [rules_js](https://github.com/aspect-build/rules_js/releases))

## Root BUILD.bazel file

The `BUILD.bazel` file standing at the root of the repository primarily configures various rules for our monorepo
including linking npm packages, setting up static code analysis tools like ESLint, defining TypeScript and Babel
configs, along with Gazelle for dependency management and BUILD file generation when applicable.

### `js_library` target definition

One of the first Javascript related snippets in the root `BUILD.bazel` file is the rule `js_library` used to define 
a target named `postcss_config_js`:

```py
load("@aspect_rules_js//js:defs.bzl", "js_library")

js_library(
    name = "postcss_config_js",
    srcs = ["postcss.config.js"],
    deps = [
        "//:node_modules/autoprefixer",
        "//:node_modules/postcss-custom-media",
        "//:node_modules/postcss-focus-visible",
        "//:node_modules/postcss-inset",
    ],
)
```

1. `load("@aspect_rules_js//js:defs.bzl", "js_library")` imports the `js_library` rule from the `defs.bzl` file 
located in the `js` directory of the `aspect_rules_js` third party repository that we fetched in the WORKSPACE file above.
2. `js_library(...)` initiates the declaration of a JS library.
3. `name = "postcss_config_js"` assigns the name to the JS library. It's the _target_ name. This is how other rules 
will refer to this library if they depend on it or how we can reference this target in your bazel commands 
(e.g., `bazel build //:postcss_config_js`).
4. `srcs = ["postcss.config.js"]` specifies the list of source files that are included in this library.
5. `deps = [...]` defines dependencies of this target. This may include other `js_library` targets or other targets 
with JS files.

### What is `js_library`?

`js_library` is a rule provided by [rules_js](https://docs.aspect.build/rules/aspect_rules_js). 
It assembles together JS sources and their transitive and npm dependencies into a single unit.
In concrete terms, it takes the form of a `JsInfo` provider, added to the build graph.

See [docs](https://docs.aspect.build/rules/aspect_rules_js/docs/js_library) for more details.

### What are the build graph and info providers?

In the Bazel build system, everything is represented in a structure called a build graph. Each node in the graph 
represents a build target (for example `js_library` rule declares `postcss_config_js` as one of this target),
and the edges between nodes represent dependencies. When a target is built, Bazel analyses the dependencies (edges) 
and builds those first, ensuring that everything a target needs to build correctly is in place before the build begins.

In case of the `js_library` rule, it produces a node that represents a collection of JavaScript sources, 
which might be individual JS files or packages, and these nodes are linked by their dependencies. 
For example, our `postcss_config_js` node would be connected to the `autoprefixer`, `postcss-custom-media`,
`postcss-focus-visible`, and `postcss-inset` nodes in the build graph. So if Bazel wants to build `postcss_config_js`,
it knows that it has to build those nodes (targets) first. 

`JsInfo` is a Bazel info provider. Info Providers supply metadata or additional information about the build target. 
In our example, `JsInfo` is like a property or an attribute attached to each node created with `js_library` in
the build graph. This attribute contains essential information about what files this target provides once it
is built. It can include various pieces of data, such as the names and locations of the JavaScript source files, 
the dependencies that the target has, and any other metadata that Bazel or other parts of the build system might
need to know to properly build and use the target.

While on a daily basis, you'll never interact with the `JSInfo` provider, it's a crucial concept to understand
when interacting with custom rules that wraps in Bazel various client tasks that we may want to perform in a build. 

### ts_config

`ts_config` is a rule provided by [rules_ts](https://docs.aspect.build/rules/aspect_rules_ts/) enables a `tsconfig.json`
file to extend another one in the build graph, ensuring Bazel recognizes the extended configuration and makes it 
available to dependent targets.

```py
load("@aspect_rules_ts//ts:defs.bzl", "ts_config")

ts_config(
    name = "tsconfig",
    src = "tsconfig.base.json",
    deps = [
        "//:node_modules/@sourcegraph/tsconfig",
    ],
)
```

See [docs](https://docs.aspect.build/rules/aspect_rules_ts/docs/rules#ts_config) for more details.

### npm binary imports

```py
load("@npm//:eslint/package_json.bzl", eslint_bin = "bin")

eslint_bin.eslint_binary(
    name = "eslint",
    testonly = True,
    visibility = ["//visibility:public"],
)
```

The `load()` function loads the `eslint` npm package that was installed by `rules_js` based on `pnpm-lock.yaml`. 
The path `"@npm//:eslint/package_json.bzl"` is indicating the location of the `eslint` package within the Bazel-managed 
npm dependencies. It's loading the `bin` macro which is generated by `rules_js` for the `eslint` package and aliasing it
as `eslint_bin` for convenience and clarity..

After that, `eslint_bin.eslint_binary()` is used to create a new Bazel build target. This target will execute the 
eslint binary when run (bazel way of doing `pnpm eslint`).

#### Under the hood

By running `bazel query '//:eslint' | grep Outputs:` we can find out what files `rules_js` generate as a compatibility 
layer between eslint binary and Bazel:

```sh
INFO: Found 1 target...
  Outputs: [bazel-out/darwin_arm64-fastbuild/bin/eslint_node_bin/node]
  Outputs: [bazel-out/darwin_arm64-fastbuild/bin/eslint.sh]
  Outputs: [bazel-out/darwin_arm64-fastbuild/bin/eslint.sh.runfiles_manifest]
  Outputs: [bazel-out/darwin_arm64-fastbuild/bin/eslint.sh.runfiles/MANIFEST]
  Outputs: [bazel-out/darwin_arm64-fastbuild/internal/_middlemen/eslint.sh-runfiles]
```

`bazel-out/darwin_arm64-fastbuild/bin/eslint.sh` is the bash script wrapper that prepares arguments for the eslint 
binary, executes it, and handles exit codes in a Bazel-compatible way. If we need to debug issues with the binary
we're trying to run, looking at the underlying bash script and printing information to stdout helps a lot.

See **I want to execute a js binary with bazel. How do I do that?** for more details [here](./web.md)

## Gazelle for Javascript/Typescript code

[Gazelle](https://github.com/bazelbuild/bazel-gazelle) is a BUILD file generator for Bazel. It partially automates the
creation and management of Bazel's BUILD files by analyzing project source code and dependencies. We say partially,
because it can generate targets for you, populate `srcs` and `deps`, but it stops there. It won't be able to understand
for example that a given script is an integration test runner that depends on a server container image for example. 
It's best to think about it as a skeleton generator, that you can run again to update things that can be inferred. 

In the context of TypeScript and JavaScript, Gazelle uses Tree-sitter, a parser generator tool, to interpret the code.
It navigates the abstract syntax tree (AST) produced by Tree-sitter to detect dependencies, including type-only imports
in TypeScript, and reflects these relationships in the generated BUILD files.

Use `bazel configure` to update source file lists for `ts_project` rules in BUILD files. This command inspects all 
nested folders starting from the current one and non-destructively updates `ts_project` targets. You can use a `# keep` 
directive to force the tool to leave existing BUILD contents alone.

```py
ts_project(
  srcs = [
    "index.ts",
    "manual.ts" # keep # <- gazelle won't touch this.
  ]
)
```

💡 Run `bazel configure` from the specific folder (e.g., `client/common`) to update only a subset of nested BUILD files
instead of crawling the whole monorepo.

### Gazelle configuration

We can configure Gazelle using directives, which are specially-formatted comments in BUILD files that govern the tool's
behavior when visiting files within the Bazel package rooted at that file. See the list of available JS directives
[here](https://github.com/aspect-build/aspect-cli/blob/main/docs/help/topics/directives.md).

By default Gazelle for JS produces two targets: sources and tests based on globs used in config. We can use this
[undocumented](https://github.com/aspect-build/aspect-cli/blob/5.5.2/gazelle/js/tests/groups_deps/BUILD.in) directive to
define custom targets:

```py
# gazelle:js_custom_files e2e *.e2e.ts
# gazelle:js_custom_files pos *.po.ts
```

In our monorepo most of the JS related Gazelle directives are defined in the root BUILD file and in `./client/BUILD.bazel`.

## Structure of the simplest client package

Based on `./client/common/BUILD.bazel`, one of the smallest client packages.

1. Linking `node_modules`.

    ```py
    load("@npm//:defs.bzl", "npm_link_all_packages")

    npm_link_all_packages(name = "node_modules")
    ```

    This macro will expand to a rule for each npm package, which creates part of the `bazel-bin/[path/to/package]/node_modules` tree.

2. Defining the [ts_config](https://docs.aspect.build/rules/aspect_rules_ts/docs/rules/#ts_config) target.

    ```py
    load("@aspect_rules_ts//ts:defs.bzl", "ts_config")

    ts_config(
        name = "tsconfig",
        src = "tsconfig.json",
        visibility = ["//client:__subpackages__"],
        deps = [
            "//:tsconfig",
            "//client/extension-api-types:tsconfig",
        ],
    )
    ```

    With its dependencies: root `tsconfig.json` and configs used in project references field. This target is available to all the other
    client packages via `//client:__subpackages__` visibility value.

3. Defining the [ts_project](https://docs.aspect.build/rules/aspect_rules_ts/docs/rules/#ts_project) targets.

    ```py
    load("//dev:defs.bzl", "ts_project")

    ts_project(
        name = "commmon_lib",
        tsconfig = ":tsconfig",
        srcs = [...],
        deps = [...]
    )
    ```

    Compiles one TypeScript project using `tsc --project`.

    These targets should be generated by running Gazelle via `bazel configure`. E.g., `cd client/common && bazel configure` to update
    only this package's BUILD files. If we remove `ts_project` targets and run this command, targets will be regenerated. The only
    thing we have to do manually here is specify the `tsconfig` attribute on targets: `tsconfig = ":tsconfig"`. Otherwise, the default
    in-memory config will be used, which will probably cause errors because it differs from our setup.

4. Defining the [npm_package](https://docs.aspect.build/rules/aspect_rules_js/docs/npm_package#npm_package) target.

    ```py
    load("//dev:defs.bzl", "npm_package")

    npm_package(
        name = "common_pkg",
        srcs = [
            "package.json",
            ":common_lib",
        ],
    )
    ```

    This target packages `srcs` into a directory (a tree artifact) and provides an `NpmPackageInfo`.

    In practice, it means that with Bazel, the dependency packages are treated as independent NPM packages. This provides a level of
    isolation and consistency across the entire monorepo, while also maintaining the interoperability of packages in a way that is
    similar to a traditional multi-repo setup. In non-Bazel setups, we use TypeScript project references and bundlers to directly
    link to the source code of a dependency package when it's needed by the current package.

5. Defining the `vitest_test` target.

    ```py
    load("//dev:defs.bzl", "vitest_test")

    vitest_test(
        name = "test",
        data = [
            ":common_tests",
        ],
    )
    ```

    Allows to run Vitest tests inside of Bazel via `bazel test //client/common:test`.

### How to create nested BUILD files to leverage caching

In Bazel, build targets are organized using BUILD files, indicating a set of sources and their dependencies. 
For increased efficiency and effective caching, it's beneficial to create distinct BUILD files for each feature.
Bazel employs caching to bypass repetitive tasks. When alterations occur in files or dependencies, only the directly
impacted targets are rebuilt. Using a separate BUILD file for each feature improves the granularity of your build
targets, ensuring that modifications in one feature won't provoke a rebuild of unrelated parts.

1. Navigate to the directory of the new feature.
2. Create an empty BUILD file.
3. Update Gazelle glob for JS file if needed. E.g., `# gazelle:js_files **/*.{ts,tsx}`.
4. Run `bazel configure` from your terminal.
5. Ensure the correct `tsconfig` attribute values are set on generated targets. E.g., if it's a nested BUILD file for a
    `common` package, the value would look like this: `tsconfig = "//client/common:tsconfig"`.

Gazelle will automatically remove sources added to the new BUILD file from the BUILD file higher in the file tree.

## Client integration tests integration with Bazel

```py
mocha_test(
    name = "integration-tests",
    timeout = "moderate",
    data = ["//client/web:app"],
    env = {
        "WEB_BUNDLE_PATH": "$(rootpath //client/web:bundle)",
    },
    is_percy_enabled = True,
    tags = [
        "no-sandbox",
        "requires-network",
    ],
    tests = [test.replace(".ts", ".js") for test in glob(["**/*.test.ts"])],
    deps = [":integration_tests"],
)
```

The `mocha_test` is used to create a bundle of test files and then configure and execute them. See `dev/mocha.bzl` 
for implementation details.

1. Test bundling.

    We use the `esbuild()` rule from Aspect rules to bundle JS test files removing import statements. Here, the `external` parameter is set to `NON_BUNDLED` to prevent certain dependencies from being included in the bundle, since these are either managed by Mocha or have issues being bundled.

2. If Percy is not enabled we just need to run the Mocha CLI with certain arguments.

    The `args` parameter is populated with Mocha configurations such as the path to the Mocha config file, the path to the generated test bundles, and the number of retries. The `data`` parameter is also expanded to include non-bundled dependencies and the bundle itself.
    `data` represents files which should be available on the dist during the action execution.

3. If Percy is enabled we run a customer Javascript script that runs Mocha tests with Percy and sets required env variables.

    The reason why the setup here is complicated is that Percy builds need to know the current commit and branch name which busts Bazel cache and we do not want to run integration tests on every commit. To avoid busting Bazel cache we rely on stamping.

    - [Stamping](https://bazel.build/reference/glossary#stamping): Stamping is a method in Bazel for embedding various build-time variables into the final binary or rule outputs. It is typically used to incorporate data like the current Git commit SHA or build number into the output. `stamp = 1` indicates that the build stamping is enabled for the `js_run_binary` rule.
    - [js_run_binary](https://docs.aspect.build/rules/aspect_rules_js/docs/js_run_binary): The `js_run_binary` rule is used to execute JavaScript programs using Node.js as a part of the build process. It's designed to work with Bazel's hermeticity, meaning the build process won't be affected by the external system state.
    - [build_test](https://github.com/bazelbuild/bazel-skylib/blob/main/docs/build_test_doc.md): `build_test` is a simple rule from Bazel Skylib that asserts certain properties about the build. It can be used to check if certain targets are present or absent, to ensure certain properties are maintained in the build configuration, or even to check if certain targets are error-free.

    In `mocha_test`, the `js_run_binary` is used in combination with `build_test` to create a specific setup for running Mocha tests with Percy enabled. The same could be achieved only with one target `js_test` but it does not support stamping, which is needed here to pass certain build-time variables (like the Git branch and commit hash) to Percy. These variables are required for Percy to correctly create and associate visual diff reports. The output of `js_run_binary` is then used as a target for `build_test` to ensure the binary runs correctly.

    The volatile env variables (`BUILDKITE_COMMIT` and `BUILDKITE_BRANCH`), which are generated by `dev/bazel_buildkite_stamp_vars.sh` are then used by the Node.js script (`client/shared/dev/runMochaTestsWithPercy.js`) as env variables in the Percy CLI command.

## ESLint custom rule implementation for type-aware linting

- TODO: npm_translate_lock with public_hoist_packages in the WORKSPACE file

## Debugging tips

If you're new to Bazel, you may find debugging a bit tricky at first. Here's a guide with some practical tips to get you started.

### Understanding Sandboxing

Sandboxing in Bazel is a method used to isolate processes, restricting what files they can access. This ensures that every build is hermetic, meaning it strictly controls dependencies and helps reproduce builds.

To inspect sandboxing, use `bazel info output_base` in your terminal. This command returns the path to Bazel's working directory. From there, navigate to the `sandbox` folder to find the sandboxes created for the current build. Exploring these files gives you a clearer idea of the environment in which each build rule executes, making it easier to diagnose any issues.

Note: The strictness of sandboxing can vary between operating systems. Linux's systems typically have more stringent sandboxing than MacOS.

### Modifying Existing Bazel Rules

Bazel permits the modification of existing rules. To locate these, use the command `bazel info output_base` to find the output base directory. Navigate to the `external` directory within this base directory to find locally saved Bazel rules.

### Opting Out of Sandboxing

There are scenarios where you might need to disable sandboxing, such as when a process needs to access the rest of your system or when you want to speed up your build. To do this, use `bazel run` or add `tags = ["no-sandbox"]` to the relevant rule in your BUILD file. Be careful with this, as it may lead to non-hermetic builds.

### Using Print Statements and Logs for Debugging

Different debugging methods are available depending on the scripting and programming languages you're working with:

- Starlark is the language used for writing Bazel build files and rules. You can insert [print()](https://bazel.build/rules/lib/globals/all#print) statements in your Starlark code. The output of these print statements is displayed in the console when Bazel executes the Starlark code.
- Add `echo` statements to Bazel generated bash scripts (see [this section](#under-the-hood) for more details) to better understand the file structure and current execution context.
- For JavaScript code, it was helpful to add `console.log()` statements to `node_modules` sources (e.g., `eslint` binary) located under `bazel-bin` to better understand the file structure and current execution context.
