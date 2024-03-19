#!/usr/bin/env bash

set -eu
EXIT_CODE=0

echo "~~~ :aspect: :stethoscope: Agent Health check"
/etc/aspect/workflows/bin/agent_health_check

aspectRC="/tmp/aspect-generated.bazelrc"
rosetta bazelrc > "$aspectRC"
bazelrc=(--bazelrc="$aspectRC")

#shellcheck disable=SC2317
# generates and uploads all bazel diffs checked by this script
# in a single buildkite artifact, to be applied by subsequent
# buildkite steps.
function generate_diff_artifact() {
  if [[ $EXIT_CODE -ne 0 ]]; then
    temp=$(mktemp -d -t "buildkite-$BUILDKITE_BUILD_NUMBER-XXXXXXXX")
    trap 'rm -rf -- "$temp"' EXIT

    mv ./annotations/* "$temp/"
    git clean -ffdx

    bazel "${bazelrc[@]}" configure >/dev/null 2>&1
    bazel "${bazelrc[@]}" run //:gazelle-update-repos >/dev/null 2>&1

    git diff > bazel-configure.diff

    # restore annotations
    mkdir -p ./annotations
    mv "$temp"/* ./annotations/
  fi
}

trap generate_diff_artifact EXIT

echo "--- :bazel: Running bazel run //:gazelle"
bazel "${bazelrc[@]}" run //:gazelle

echo "--- Checking if BUILD.bazel files were updated"
# Account for the possibility of a BUILD.bazel to be totally new, and thus untracked.
git ls-files --exclude-standard --others | grep BUILD.bazel | xargs git add --intent-to-add

git diff --exit-code || EXIT_CODE=$? # do not fail on non-zero exit

# if we get a non-zero exit code, bazel run //:gazelle updated files
if [[ $EXIT_CODE -ne 0 ]]; then
  mkdir -p ./annotations
  cat <<-'END' > ./annotations/bazel-prechecks.md
  #### Missing BUILD.bazel files

  BUILD.bazel files need to be updated to match the repository state. You should run the following command and commit the result

  ```
  sg bazel configure
  ```

  #### For more information please see the [Bazel FAQ](https://sourcegraph.com/docs/dev/background-information/bazel/faq)

END
  exit "$EXIT_CODE"
fi

echo "--- :bazel: Running bazel run //:gazelle-update-repos"
bazel "${bazelrc[@]}" run //:gazelle-update-repos

echo "--- Checking if deps.bzl was updated"
git diff --exit-code || EXIT_CODE=$? # do not fail on non-zero exit

# if we get a non-zero exit code, bazel run //:gazelle-update-repos updated files
if [[ $EXIT_CODE -ne 0 ]]; then
  mkdir -p ./annotations
  cat <<-'END' > ./annotations/bazel-prechecks.md
  #### Missing deps.bzl updates

  `deps.bzl` needs to be updated to match the repository state. You should run the following command and commit the result

  ```
  sg bazel configure godeps
  ```

  #### For more information please see the [Bazel FAQ](https://sourcegraph.com/docs/dev/background-information/bazel/faq)

END
  exit "$EXIT_CODE"
fi

echo "--- :bazel::go: Running gofmt"
unformatted=$(bazel "${bazelrc[@]}" run @go_sdk//:bin/gofmt -- -l .)

if [[ ${unformatted} != "" ]]; then
  mkdir -p ./annotations
  tee -a ./annotations/bazel-prechecks.md <<-END
  #### Unformatted Go files

  The following files were found to not be formatted according to \`gofmt\`:

  \`\`\`
  ${unformatted}
  \`\`\`

  To automatically format these files run:

  \`\`\`
  bazel run @go_sdk//:bin/gofmt -- -w .
  \`\`\`
END

  if [[ $EXIT_CODE -eq 0 ]]; then
    # We;re using 100 as a Soft fail exit code, so we only want to use it the exit code isn't a hard fail code ie. not 0
    EXIT_CODE=100
  fi
fi

exit "$EXIT_CODE"
