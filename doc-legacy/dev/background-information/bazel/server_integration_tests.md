# Server integration tests with Bazel

Integration tests are very useful as they allow you to explore much wider codepaths than traditional unit testing. There are trade-offs though: it's harder to decipher from the failures what exactly went wrong and they take longer to set up. 

## Tests running against the server image (`server:candidate`) 

Every CI build produces a `server:candidate` container image, containing the Sourcegraph server built with the latest code of the currently built commit. It's never published unless you're on the `main` branch.

It's very common to run tests against the server, as it's the most practical way to test in an environment similar to what the customers will run. 

To ease the process of setting it up, we provide under the `testing/` folder a helper shell script that takes care of setting up and tearing down the server container, requiring only environment variables, binaries and/or scripts relevant to your e2e test. 
 
## Overview 

An e2e test against the `server:candidate` image is composed of: 

- A test target, using the `server_integration_test` rule, defined in `testing/BUILD.bazel`. 
- A wrapper shell script, defined in `testing/[your-test-name].sh`.

The `server_integration_test` rule will automatically add a dependency against `//cmd/server:image_tarball` which is the target that builds the `server:candidate` container image, that will be used during the lifetime of the test to instanciate a server, by running the container in Docker. 

Assuming you have named you new e2e test target `"foo_test"`, you'll be able to run with a single command the e2e test: `bazel test //testing:foo_test`. This will build the server image for you, launch it, run your script and finally delete the running container. 

### Caveat 

See this [tracking-issue](https://github.com/sourcegraph/sourcegraph/issues/53637) to follow the work around fixing the problem below.

### Environment variables 

Right now, most of the tests require precise environment variables which are only available in CI or stored in secrets. Unless you're the owner of the test in question, we recommend you iterate in CI if you're fixing a test or adding a new one. 

### Cross-compilation issues

For the moment, it's not always possible to run such tests on a MacOS machine. Building the server image, which runs a `linux/amd64` OS, requires cross-compiling the code. This is mostly fine, but some test binaries also requires compilation (the backend integration tests for example, are written in Go), which in turns requires to build those binaries targeting the host platform and not `linux/amd64`. 

Therefore, two options are available to iterate locally: 

- You have access to a linux machine, in which case you're good to go, you can use the command above exaclty as it is. 
- You don't have access to a linux machine, in which case you'll want to write your tests by running the server locally on your own, develop your tests like this, and finally integrate them with Bazel in the `//testing` package once you're done.

## Using the `server_integration_test` rule

### Adding the rule in `testing/BUILD.bazel` 

The `server_integration_test` rule is a actually a [_macro_](https://bazel.build/extending/macros) of [`sh_test`](https://bazel.build/reference/be/shell#sh_test). In simple terms, a macro in Bazel is a function that will output code calling other rules, usually doing manipulation for you to avoid having to repeat yourself, or to pass default values. 

In practice, all of what the `server_integration_test` macro does, is to inject the code that declare the dependency toward the `server:candidate` and a helper shell script that hides away the loading/tearing down of the container image.

Here is an example: 

```
server_integration_test(
    # name is how the test will be called, `bazel test //testing:foo_test` here.
    name = "foo_test",

    # runner_src points toward the wrapping script that powers your test.
    runner_src = [":foo_test.sh"],

    # args defines the additional arguments given the script when run. 
    # We'll need to call our `foo_test` binary in the wrapping script, so we pass its 
    # path to the script.
    args = [
        "$(location //foo:foo_test)", 
    ],

    # Bazel cannot infer just from looking at the args what the dependencies are, so we
    # need to explicitly say that we depend on `foo_test` as well. And Bazel will build it 
    # for us before calling our new e2e test.
    data = [
        "//foo:foo_test",
    ],

    # We can define environment variables which will be exported and availble to both our 
    # script and any binary that call during its lifetime.
    env = {
        "FOO": "2",
    },

    # If we depend on variable being defined in the environment, but cannot specify them statically,
    # for example a TOKEN, we can use `env_inherit` to say to just fetch it from outside.
    env_inherit = [
        "FOO_ACCESS_KEY_ID",
        "FOO_SECRET_KEY",
    ],

    # Very important: explicitly define a PORT to run the server on. 
    # Bazel will happily run concurrent instances of the server when it's executing our tests, 
    # because it's trying to do as much as possible at once, to maximize the resource consumption 
    # and lower the wall clock time. 
    # 
    # Therefore, it's extremely important that we do not reuse the same port across server_integration_test 
    # definitions, otherwise the tests will be extremely unreliable and very hard to debug, as another 
    # test may affect our own tests accidentally. 
    port = "7087",
)
```

Now we have defined our test target, it's time to write the script that will call your tests, regardless of the language you wrote them with, while setting up and tearing down the server image.

### Writing the test runner script

We'll now write `testing/foo_test.sh`, which will take care of setting up and tearing down the server image once it has ran our tests against the server.
You're free to pick whatever language you want to write a test, as long as it is built and runnable by Bazel. It can be a shell script, Go code or 
NodeJs code. You can check the other wrapping scripts for examples for each of those. 

There are two crucial elements to respect for a working test runner script: 

- Your test runner, the binary that the wrapper we're writing here is calling, _must_ allow for the Sourcegraph URL to be configured. Otherwise we cannot run these server tests concurrently. 
- The `args` attributes you passed in the `server_integration_test` are not the first arguments for this script. The first two arguments are the image tarball and the container name. This means that any argument you passed will be availble, starting at `$3` and so on. 

Here is an example: 

```
#!/usr/bin/env bash

# Exit if any command exits with non-zero or if any variable is undefined 
set -eu

# Load the helper script, providing the plumbing to set up and tear down the container image.
source ./testing/tools/integration_runner.sh || exit 1

# We're getting those values from the `args` attribute. The first two ones are always provided by the 
# `server_integration_test` macro. It's a bit inconvenient and awkward, as the numbering starts at $3 for your arguments,
# but we'll revisit this at some point.
tarball="$1"
image_name="$2"

# So here, are your own `args` you defined. In our case, it's just the `footest` binary.
footest="$3"

# Define at which URL the server will be listening to
url="http://localhost:$PORT"

# If you need to use a different name for environment variables, you can do it here. 
# Typically, this is because we're exposing a secret with a different name than the one
# being used by the server. 
# 
# See the following example: 
# ---------------------------
# Backend integration tests uses a specific GITHUB_TOKEN that is available as GHE_GITHUB_TOKEN
# because it refers to our internal GitHub enterprise instance used for testing.
GITHUB_TOKEN="$GHE_GITHUB_TOKEN"
export GITHUB_TOKEN

ALLOW_SINGLE_DOCKER_CODE_INSIGHTS="true"
export ALLOW_SINGLE_DOCKER_CODE_INSIGHTS

run_server_image "$tarball" "$image_name" "$url" "$PORT"

# Now, let's call the test runner that we provided in the server_integration_test rule definition:
# It's mandatory for your test runner to be able to be configured to run against a specific url, 
# because we may change the port in the Buildfile.
echo "--- foo tests"
"$footest" --url "$url"

echo "--- done"
```

## Going further 

Once you're less confused by the Bazel jargon that is sprinkled all over the place, it'll become clear that Bazel is simply providing 
an hermetic way of running commands, meaning you can include whatever you want in your wrapping script, such as creating an 
admin user with `initSg` for example. 

The best way to learn is to see how the other tests are written.
