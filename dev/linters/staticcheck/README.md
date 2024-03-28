# Static check analyzers

We cannot use the staticcheck analyzer directly, since it's actually a collection of analyzers. So we have to pull out each "micro" analyzer and create a seperate bazel target with it.

Each bazel target actually just uses staticcheck.go and embeds the analyzer it SHOULD use when invoked as that target. Ex:

bazel build //dev/linters/staticcheck:SA6001
The above target will do the following:

Set AnalyzerName to SA6001 in staticcheck.go uses x_def
Set the importpath to github.com/sourcegraph/sourcegraph/dev/llinters/staticcheck/SA6001. This is very important, otherwise there won't be different libraries with different analyzer names, just one library with one analyzer set invoked by different targets.

## How to regenerate the analyzers

To regenerate the `BUILD.bazel` and `analyzer.bzl` files run `go generate` in `dev/linters/staticcheck`. This effectively runs `go run ./cmd/gen.go`
Hello World
