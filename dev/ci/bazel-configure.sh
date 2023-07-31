#!/usr/bin/env bash

set -eu

# We run :gazelle since currently `bazel configure` tries to execute something with go and it doesn't exist on the bazel agent
echo "--- :bazel: Running bazel configure"
bazel --bazelrc=.bazelrc --bazelrc=.aspect/bazelrc/ci.bazelrc --bazelrc=.aspect/bazelrc/ci.sourcegraph.bazelrc configure

# We disable exit on error here, since we want to catch the exit code and interpret it
set +e

echo "--- Checking if BUILD.bazel files were updated"
git diff --exit-code

EXIT_CODE=$?

# if we get a non-zero exit code, bazel configure updated files
if [[ $EXIT_CODE -ne 0 ]]; then
  mkdir -p ./anntations
  cat <<-'END' > ./annotations/bazel-configure.md
  BUILD.bazel files need to be updated to match the repository state. You should run the following command and commit the result

  ```
  bazel configure
  ```

  #### For more information please see the [Bazel FAQ](https://docs.sourcegraph.com/dev/background-information/bazel#faq)

END
fi

echo "--- :bazel::go: Running gofmt"
unformatted=$(bazel run @go_sdk//:bin/gofmt -- -l .)

if [[ $unformatted != "" ]]; then
  mkdir -p ./anntations
  cat <<-END > ./annotations/bazel-gofmt.md
  The following files were found to not be formatted according to `gofmt`:

  \`\`\`
  "${unformatted}"
  \`\`\`

  To automatically format these files run:

  \`\`\`
  bazel run @go_sdk//:bin/gofmt -- -w .
  \`\`\`
END
  EXIT_CODE=1
fi


exit "$EXIT_CODE"
