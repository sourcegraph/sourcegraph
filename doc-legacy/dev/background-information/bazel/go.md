# Bazel for Go 

See also: [How-to use `go:generate` with Bazel](./go_generate.md)

## TL;DR 

- Commands:
  - `bazel configure` to update buildfiles after changing imports, adding or deleting files. 
  - `bazel run :gazelle-update-repos` to reflect changes made to `go.mod`.
  - `bazel test //... --config go-short` to run all Go unit tests.
    - 💡 Run `ibazel test //... --config go-short` in another terminal to have Bazel detect your changes and re-run the tests automatically when you save your modifications.
- Rules: 
  - All `go_test` have `race = "on"` enabled.
  - All `go_test` rules are providing the following defaults, unless explicitly defined 
    - `timeout = "short"` 
    - `tags = ["go"]` 
- Avoid:
  - Having test code that explictly depends on being aware of the Sourcegraph git repository.
  - Committing files with spaces in their names. 

## Overview 

Bazel and Go is pretty straightforward. With a few minor exceptions, you can still use your normal Go tooling and only care about Bazel 
in CI, as long as you remember to run `bazel configure` before pushing your changes. 

The rules interfacing Go are named [`rules_go`]() and they provide all the plumbing to call the Go compiler and run the tests. [Gazelle]() is the tool that parses you Go code and the `go.mod` file to scaffold the build files for you. It's tempting to think about it as a generator that could be fully automated in CI: it is not the case. 

Instead, it's there to do the grunt work for you, and in 95% of the cases, it does a great job at it, which gives this wrong idea that it could be automated and hidden away. It cannot know for example if your Go tests are very long and should be given a long timeout, nor if your tests are flaky and should be retried until you fix the problem. Or that you're constructing files being emdedded from another program being run.

## Rules for go

The rules you'll see for Go are [`go_library`](https://github.com/bazelbuild/rules_go/blob/master/docs/go/core/rules.md#go_library), [`go_binary`](https://github.com/bazelbuild/rules_go/blob/master/docs/go/core/rules.md#go_binary) and [`go_test`](https://github.com/bazelbuild/rules_go/blob/master/docs/go/core/rules.md#go_test). 

`go_library` is where most of the work happens: 

If the code uses `go:embed` Go directives, the `embedsrcs` attribute of the `go_library` rule will enable you to specify what should go in there. If the files that need to be embedded are static, Gazelle will pick them up and will generate the correct `embedsrcs` attribute for you.

Similarly, if you want to pass symbol definition at build time, you can use the `x_defs` attribute to pass a fully qualified path to the symbol and the value it should be given.

Finally, you'll see `visibility` attribute a lot in those rules. In 99% of the cases, you should never to have to change those, unless you know precisely what you're doing. The reason this exists is to prevent in very large codebases others to create dependencies on your packages without you being informed (because that would change the `BUILD.bazel` file of _your_ package and you'd see it).

### Conventions and defaults 

By default, all `go_test` are tagged with `"go"`. They also have by default the `timeout = "short"` attribute set. Those default values are Sourcegraph specific and are defined in [`go_defs.bzl`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/dev/go_defs.bzl). This enables to filter and run only those tests with the `--test_tag_filters=go --test_timeout_filters=short`, which is alias for convenience as `--config go-short`. 

The `timeout` attribute tells Bazel that running those tests (for the entire package) should not exceed 60s (in case of a `"short"` timeout). If you get too close to that threshold, you'll see a warning displayed and if it goes over the threshold it will instead fail with a `TIMEOUT` error. 

Read more [about the test tags and timeout attributes in the Bazel documentation](https://bazel.build/reference/test-encyclopedia).

#### Example:

Given the following `go_test` 

```
go_test(
    name = "errors_test",
    timeout = "short",
    srcs = [
        "errors_test.go",
        "filter_test.go",
        "warning_test.go",
    ],
    embed = [":errors"],
    deps = [
        "@com_github_stretchr_testify//assert",
        "@com_github_stretchr_testify//require",
    ],
)
```

The above default conventions will expand it to: 

```
go_test(
    name = "errors_test",
    # ------ default attributes which are injected -----------------
    timeout = "short",
    tags = ["go"],
    # --------------------------------------------------------------
    srcs = [
        "errors_test.go",
        "filter_test.go",
        "warning_test.go",
    ],
    embed = [":errors"],
    deps = [
        "@com_github_stretchr_testify//assert",
        "@com_github_stretchr_testify//require",
    ],
)
```
