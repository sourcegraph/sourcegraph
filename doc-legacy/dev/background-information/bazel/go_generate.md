# How-to use `go:generate` with Bazel

## TL;DR

- `go:generate` directive produces unpredictable outputs, which is incompatible with Bazel
- `go:generate` is just about calling a program that outputs files
- We can use Bazel `genrule` to achieve the exact same thing.
- We want to write the files back to the source tree, with the `write_generated_to_source_files` macro.

Common `go:generate` directives that have their own macro available:

- [`stringer`](https://pkg.go.dev/golang.org/x/tools/cmd/stringer), [examples](https://sourcegraph.com/search?q=context%3Aglobal+repo%3A%5Egithub%5C.com%2Fsourcegraph%2Fsourcegraph%24+lang%3AStarlark+go_stringer&patternType=standard&sm=1&groupBy=path).

### Overview

#### What `go:generate` does exactly?

Go compensates for its inherent rigidity by enabling developers to use `go:generate` in order to generate files (and often code) on the fly. While this is a powerful technique, it's actually very simple: all Go does when you call `go generate` is to simply execute commands it finds scattered in source files, behind a `//go:generate ...` comment.

It doesn't do anything else. Writing the outputs of the command in the source tree is left as an implementation detail to the binary being called:

```
// main.go
package main


//go:generate sh -c "ls -al > list.txt"
```

When `go generate` sees this, it will simply execute `sh -c "ls -al"` in the parent folder of `main.go` which will create a `list.txt` file. Yes, you can also call a shell script from a `go:generate` directive, because all it does is calling a binary.

In the real world, it's often used to generate Go code from a GraphQL schema, generate Markdown documentation, etc ... it's really neat.

But this flexibility comes at a cost, like in the example above, you can call _anything_ in a `go:generate` directive. The binary could make network calls, generate different outputs after being bumped to a new version or run `git` commands it wants.

#### Why Bazel will never support `go:generate`

Due to the aforementioned flexibility, there is no way for Bazel to determine what will come out of running that binary or what it needs to produce correct outputs. So it's not possible to support it "as is", by design.

> Ok, in that case, why don't you let me use `go:generate` outside of Bazel, and we run `go generate` in CI to check if the repository is dirty (i.e. files have changed after running the command) to ensure we always merge pull-requests with fully updated generated files.

Well, because we have zero control on what are the inputs and outputs, it also means we cannot cache the generation results, since we don't know what they are nor if we need to generate them again. So if a `go:generate` command takes 30s to run, it means that _every_ build in CI will spend 30s to regenerate those files, just to ensure they are correct.

> 💡 We regularly average 1500 builds per week on the monorepo, so that's 750 minutes of CI spent per week, on something that probably changes once a month.

And because this requires to run `go` outside Bazel, it means we can't risk using it on Bazel agents, due to it leaving breadcrumbs that would affect further builds. We therefore have to run in a stateless agent to ensure hermeticity, which is naturally slower.

#### What to do instead?

Now that we're clear on the fact that `go:generate` is merely about running a binary and writing something back to filesystem, we can utilize [`genrule`](https://bazel.build/reference/be/general#genrule) in Bazel to accomplish the same result.

Here is a quick example on how `genrule` works:

```
genrule(
    name = "concat_all_files",
    srcs = [
        "//some:files",  # a filegroup with multiple files in it ==> $(locations)
        "//other:gen",   # a genrule with a single output ==> $(location)
    ],
    outs = ["concatenated.txt"],
    cmd = "cat $(locations //some:files) $(location //other:gen) > $@",
)
```

If you take a step back, forget for a moment all the Bazel lingo (`$(locations ...)`, `$@`, ...) you can see that all we're doing here, is simply calling `cat` with a bunch of files and storing the output in another file.

Bazel sandboxes everything, thus you need to use `$(locations ...)` for Bazel to inject the proper paths in your final command. While `$@` may look scary, it is nothing else than the values you put into `outs`.

In above example, the _inputs_ and _outputs_ are explicit, so Bazel can perfectly cache this, because it assumes that if the inputs are the same, the outputs will be the same (if your command doesn't do that, you're in for trouble).

So we can use `genrule` to replace our wild `go:generate` directives and herd them back into being deterministic.

## Rewriting a `go:generate` directive as a `genrule`

Let's take as an example, [github.com/Khan/genqlient` that we're using.

The `go:generate` directive is `//go:generate go run github.com/Khan/genqlient genql.yaml`. It takes a YAML file and generates, from looking at the code, an `operations.go` file.

So if we naively turn that into a `genrule`, we can write:

```
genrule(
    # name of our rule
    name = "generate_genql_yaml",

    # Our inputs, what do we need to generate the outputs
    srcs = [
        "genql.yaml",
    ],

    # What we're creating, our single output
    outs = ["_operations.go"],

    # Our command to run, we need to use execpath to get the path to the binary.
    # We saw in the original go:generate directive, that it takes genql.yaml as its first argument
    # so by using $(location ...), we ask Bazel to get its path.
    cmd = "$(execpath @com_github_khan_genqlient//:genqlient) $(location genql.yaml)",

    # We need to inform Bazel, that we need this binary for our cmd, otherwise, it won't find it.
    tools = ["@com_github_khan_genqlient//:genqlient"],
)
```

When we build our defined target `cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml` we get the following:

```
$ bazel build cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml (1 packages loaded, 2 targets configured).
INFO: Found 1 target...
ERROR: /Users/tech/work/sourcegraph/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel:32:8: Executing genrule //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed: (Exit 1): bash failed:
 error executing command (from target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml) /bin/bash -c ... (remaining 1 argument skipped)

Use --sandbox_debug to see verbose messages from the sandbox and retain the sandbox build root for debugging
cmd/frontend/graphqlbackend/schema.graphql did not match any files
Target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed to build
Use --verbose_failures to see the command lines of failed build steps.
INFO: Elapsed time: 0.543s, Critical Path: 0.16s
INFO: 2 processes: 2 internal.
FAILED: Build did NOT complete successfully
```

The line that tells us what went wrong is `cmd/frontend/graphqlbackend/schema.graphql did not match any files`. When running the `genqlient` generator, it exits with an error, because it cannot find the `schema.graphql` file.

Perhaps we need more inputs? If we take a peek at the `genrql.yaml` file we can learn more about what's going on:

```
# genql.yaml
schema:
  - ../../../../../../cmd/frontend/graphqlbackend/schema.graphql
  - ../../../../../../cmd/frontend/graphqlbackend/guardrails.graphql
operations:
  - operations.graphql
generated: operations.go
optional: pointer
```

Ah yes, we're missing a few inputs. Let's add them:

```
genrule(
    # name of our rule
    name = "generate_genql_yaml",

    # our inputs, what do we need to generate the outputs
    srcs = [
        "genql.yaml",
        "operations.graphql",
        "//cmd/frontend/graphqlbackend:schema.graphql",
        "//cmd/frontend/graphqlbackend:guardrails.graphql",
    ],

    # What we're creating, our single output.
    outs = ["operations.go"],

    # Our command to run, we need to use execpath to get the path to the binary
    cmd = "$(execpath @com_github_khan_genqlient//:genqlient) $(location genql.yaml)",

    # We need to inform Bazel, that we need this binary for our cmd, otherwise, it won't find it.
    tools = ["@com_github_khan_genqlient//:genqlient"],
)
```

This time round, when we build out target , we get:

```
$ bazel build cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml (2 packages loaded, 5 targets configured).
INFO: Found 1 target...
ERROR: /Users/tech/work/sourcegraph/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel:32:8: declared output 'cmd/frontend/internal/guardrails/dotcom/operations.go' was not created by genrule. This is pr
obably because the genrule actually didn't create this output, or because the output was a directory and the genrule was run remotely (note that only the contents of declared file outputs are copied from genrules run remotely)
ERROR: /Users/tech/work/sourcegraph/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel:32:8: Executing genrule //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed: not all outputs were c
reated or valid
Target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed to build
Use --verbose_failures to see the command lines of failed build steps.
INFO: Elapsed time: 0.315s, Critical Path: 0.05s
INFO: 2 processes: 1 internal, 1 darwin-sandbox.
FAILED: Build did NOT complete successfully
```

The `ERROR` lines mention that `operations.go` was not created. Bazel considers our build to be successful if and only if, our `cmd` exits with 0 and that it can find the declared outputs (the ones declared in the attribute `outs`). In our case, it did exit with 0, but we're missing the output.

See, `genqlient` is a bit peculiar, you cannot configure where it should put the files. It simply puts them in the current working directory. So perhaps, it's not where Bazel expects it.

The `cmd` attribute is just a shell command, so why not put a `find` in there to see where that `operations.go` went?

```
genrule(
    # ...
    cmd = "$(execpath @com_github_khan_genqlient//:genqlient) $(location genql.yaml); echo HERE; find . -name operations.go; echo HERE;",
    # ...
)
```

And we get:

```
$ bazel build cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml --sandbox_debug
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml (1 packages loaded, 3 targets configured).
INFO: Found 1 target...
INFO: From Executing genrule //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml:
HERE
./cmd/frontend/internal/guardrails/dotcom/operations.go
HERE
ERROR: /Users/tech/work/sourcegraph/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel:32:8: declared output 'cmd/frontend/internal/guardrails/dotcom/operations.go' was not created by genrule. This is pr
obably because the genrule actually didn't create this output, or because the output was a directory and the genrule was run remotely (note that only the contents of declared file outputs are copied from genrules run remotely)
ERROR: /Users/tech/work/sourcegraph/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel:32:8: Executing genrule //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed: not all outputs were c
reated or valid
Target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml failed to build
Use --verbose_failures to see the command lines of failed build steps.
INFO: Elapsed time: 0.191s, Critical Path: 0.05s
INFO: 2 processes: 1 internal, 1 darwin-sandbox.
FAILED: Build did NOT complete successfully
```

Ah! We see it now:

```
HERE
./cmd/frontend/internal/guardrails/dotcom/operations.go
HERE
```

Bazel expects to find outputs at a particular path, that's the obscure `$@` we saw earlier in the first `genrule` example. If we look back at our `genrule` definition, we never mentioned it!

We can edit the `cmd` attribute as following:

```
genrule(
    # ...
    cmd = "$(execpath @com_github_khan_genqlient//:genqlient) $(location genql.yaml) && mv cmd/frontend/internal/guardrails/dotcom/operations.go $@",
    # ...
)
```

And this time, when we build it, it works:

```
$ bazel build cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml (0 packages loaded, 0 targets configured).
INFO: Found 1 target...
Target //cmd/frontend/internal/guardrails/dotcom:generate_genql_yaml up-to-date:
  bazel-bin/cmd/frontend/internal/guardrails/dotcom/operations.go
INFO: Elapsed time: 0.132s, Critical Path: 0.00s
INFO: 1 process: 1 internal.
INFO: Build completed successfully, 1 total action
```

Great, we did it! Still, there is something left. If we delete `operations.go`, run the build command again, nothing appears in the folder.

This is due to fact that Bazel doesn't write back outputs to your source tree. It knows about it, you can even use it as inputs for other rules. For example, you could rewrite the `go_library` rule that builds that package to use it:

```
go_library(
    name = "dotcom",
    srcs = [
        "dotcom.go",
        # "operations.go",
        ":generate_gengql_yaml"
    ],
    importpath = "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/guardrails/dotcom",
    visibility = ["//cmd/frontend:__subpackages__"],
    deps = [
        "//internal/httpcli",
        "//internal/trace",
        "@com_github_khan_genqlient//graphql",
    ],
)
```

And then build it:

```
$ rm cmd/frontend/internal/guardrails/operations.go
$ bazel build cmd/frontend/internal/guardrails/dotcom:dotcom
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:dotcom (0 packages loaded, 0 targets configured).
INFO: Found 1 target...
Target //cmd/frontend/internal/guardrails/dotcom:dotcom up-to-date:
  bazel-bin/cmd/frontend/internal/guardrails/dotcom/dotcom.a
INFO: Elapsed time: 0.120s, Critical Path: 0.00s
INFO: 1 process: 1 internal.
INFO: Build completed successfully, 1 total action
```

It works! You could stop here, and call it a day, but that would create issues with our code editors, as it wouldn't be able to find the symbols and functions declared in that `operations.go` file. If someone tried to build the code with normal Go tooling (like running a quick `go test` which is very convenient) they'll encouter some problems as the tooling will fail.

Solution exists to make `gopls` aware of the Bazel generated files, but it's not very convenient and breaks standard Go tooling. Another approach is to write that generated file back to our source tree, exactly like we originally did with the `go:generate` statement.

### Writing the newly generated file back on disk

Luckily, that step is really straightforward, we can use the `write_generated_to_source_files` macro to copy back our sandboxed output into our source tree. We can edit our `BUILD.bazel` as following

```
# must be at the top of the file, before everything else
load("//dev:write_generated_to_source_files.bzl", "write_generated_to_source_files") #

go_library(
  name = "dotcom",
  # ...
)

genrule(
  name = "generate_gengql_yaml"
  # ...
)

write_generated_to_source_files(
  # We need a name, as usual. Convention is to call them write.
  name = "write_genql_yaml",

  # From which target are we getting outputs from
  target = ":generate_genql_yaml",

  # Which files do we want to write back to the source tree
  output_files = {"operations.go": "operations.go"},

  # Tag this so we can find all of them easily
  tags = ["go_generate"],
)
```

We can ask Bazel to copy our files to the source tree with `bazel run`:

```
$ bazel run cmd/frontend/internal/guardrails/dotcom:write_genql_yaml
ERROR: Traceback (most recent call last):
        File "/Users/tech/work/other/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel", line 32, column 32, in <toplevel>
                write_generated_to_source_files(
        File "/Users/tech/work/other/dev/write_generated_to_source_files.bzl", line 10, column 17, in write_generated_to_source_files
                fail("{} and {} must differ so we can detect source files needing to be regenerated".format(dest,orig))
Error in fail: operations.go and operations.go must differ so we can detect source files needing to be regenerated
ERROR: Skipping 'cmd/frontend/internal/guardrails/dotcom:write_genql_yaml': no such target '//cmd/frontend/internal/guardrails/dotcom:write_genql_yaml': target 'write_genql_yaml' not declared in package 'cmd/frontend/internal/guardrails/dot
com' defined by /Users/tech/work/other/cmd/frontend/internal/guardrails/dotcom/BUILD.bazel (Tip: use `query "//cmd/frontend/internal/guardrails/dotcom:*"` to see all the targets in that package)
WARNING: Target pattern parsing failed.
WARNING: No targets found to run. Will continue anyway
INFO: Analyzed 0 targets (1 packages loaded, 0 targets configured).
INFO: Found 0 targets...
ERROR: command succeeded, but there were errors parsing the target pattern
INFO: Elapsed time: 0.125s, Critical Path: 0.00s
INFO: 1 process: 1 internal.
ERROR: Build failed. Not running target
FAILED: Build did NOT complete successfully
```

Ah, we got an error here: 

```
Error in fail: operations.go and operations.go must differ so we can detect source files needing to be regenerated
```

This is happening because we can't have the input file, the one we generate, be named the same as the one we're going to write back on disk, because it messes up the automatically generated test that ensure that files are up to date. 

Let's fix this, by editing our initial `genrule` so the output has a different name: 

```
genrule(
  name = "generate_gengql_yaml"
  outs = ["_operations.go"], # <- this is our change, we added a '_' to distinguish the generated file from the one written back to the source tree.
  # ...
)
```

And reflect that change on our `write_generated_to_source_files`:


```
write_generated_to_source_files(
  name = "write_genql_yaml",
  # ...
  output_files = {"operations.go": "_operations.go"}, # <- this is our change, the key is how the file will be named on disk, the other one is the file we're taking from the outputs of the input target.
  # ... 
)
```

And now if we run it again:

```
$ bazel run cmd/frontend/internal/guardrails/dotcom:write_genql_yaml
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml (1 packages loaded, 6 targets configured).
INFO: Found 1 target...
Target //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml up-to-date:
  bazel-bin/cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_update.sh
INFO: Elapsed time: 0.177s, Critical Path: 0.02s
INFO: 6 processes: 5 internal, 1 local.
INFO: Running command line: bazel-bin/cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_update.sh
INFO: Build completed successfully, 6 total actions
Copying file /private/var/tmp/_bazel_tech/0812c8e6e121162f0d18a505d289a8d2/execroot/__main__/bazel-out/darwin_arm64-fastbuild/bin/cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_update.sh.runfiles/__main__/cmd/frontend/internal/gua
rdrails/dotcom/write_genql_yaml_copy/_operations.go to cmd/frontend/internal/guardrails/dotcom/operations.go in /Users/tech/work/other
```

It worked as we want, we're good!

As a convenience for everyone, we can add our target to `dev/BUILD.bazel`, in the `write_all_generated` rule:

```
# dev/BUILD.bazel
write_source_files(
    name = "write_all_generated",
    additional_update_targets = [
        "//lib/codeintel/lsif/protocol:write_symbol_kind",
        # ...
        # We add this:
        "//cmd/frontend/internal/guardrails/dotcom:write_genql_yaml",
    ],
)
```

Now when anyone wants to run our generators, all they have to do is to execute:

```
$ bazel run //dev:write_all_generated
```

### What happens if the outputs gets outdated?

The macro `write_generated_to_source_files`, doesn't just wrap a few details about copying files back to the source tree, it also creates test targets that ensure that our file is correct.

Let's add a dumb change to pretend the file on disk is out of sync with the generated file: 

```
# operations.go
// Code generated by github.com/Khan/genqlient, DO NOT EDIT.

package dotcom // FOOBAR FOOBAR FOOBAR

```

And now we can test the `:write_genql_yaml_tests` target, which is created for us by the `write_generated_to_source_files` macro. 

```
$ bazel test cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_tests
INFO: Analyzed target //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test (0 packages loaded, 0 targets configured).
INFO: Found 1 test target...
FAIL: //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test (see /private/var/tmp/_bazel_tech/0812c8e6e121162f0d18a505d289a8d2/execroot/__main__/bazel-out/darwin_arm64-fastbuild/testlogs/cmd/frontend/internal/guardrails/dotcom/wri
te_genql_yaml_test/test.log)
INFO: From Testing //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test:
==================== Test output for //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test:
3c3
< package dotcom
---
> package dotcom // FOOBAR FOOBAR FOOBAR
FAIL: files "cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_copy/_operations.go" and "cmd/frontend/internal/guardrails/dotcom/operations.go" differ. 

@//cmd/frontend/internal/guardrails/dotcom:operations.go is out of date. To update this and other generated files, run:

    bazel run @//dev:write_all_generated

To update *only* this file, run:

    bazel run //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml


================================================================================
Target //cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test up-to-date:
  bazel-bin/cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_test-test.sh
INFO: Elapsed time: 0.232s, Critical Path: 0.07s
INFO: 2 processes: 1 internal, 1 darwin-sandbox.
//cmd/frontend/internal/guardrails/dotcom:write_genql_yaml_test          FAILED in 0.1s
  /private/var/tmp/_bazel_tech/0812c8e6e121162f0d18a505d289a8d2/execroot/__main__/bazel-out/darwin_arm64-fastbuild/testlogs/cmd/frontend/internal/guardrails/dotcom/write_genql_yaml_test/test.log

Executed 1 out of 1 test: 1 fails locally.
INFO: Build completed, 1 test FAILED, 2 total actions
```

Right, we see our newly added `FOOBAR` comment, and Bazel picks that up. Please note that we don't have to remember the name of that target, we could simply run `bazel test //cmd/frontend/internal/guardrails/...` to get the same results (along with a few other tests).

> 💡 This is how the CI works, it simply runs `bazel test //...` so it will automatically catch any target getting out of sync.
