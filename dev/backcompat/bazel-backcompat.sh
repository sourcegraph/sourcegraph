#!/usr/bin/env bash
set -eu

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

bazelrcs=(--bazelrc=.bazelrc)
current_commit=$(git rev-parse HEAD)
tag="5.3.0"

function restore_current_commit() {
  git checkout --force "${current_commit}"
}

EXIT_CODE=0
git diff --quiet --exit-code || EXIT_CODE=$? # do not fail on non-zero exit
if [[ $EXIT_CODE -ne 0 ]]; then
  echo "🚨 WARNING: Backcompat tests does destructive operations on the repository. Please make sure your changes are commited"
fi

if [[ ${CI:-} == "true" ]]; then
  echo "~~~ :aspect: :stethoscope: Agent Health check"
  /etc/aspect/workflows/bin/agent_health_check

  aspectRC="/tmp/aspect-generated.bazelrc"
  rosetta bazelrc > "${aspectRC}"
  echo -e "\ntry-import %workspace%/.aspect/bazelrc/ci.sourcegraph.bazelrc\n"  >> "$aspectRC"
  bazelrcs=(--bazelrc="${aspectRC}")
else
  if [[ $EXIT_CODE -ne 0 ]]; then
    echo "The following files have changes:"
    git diff --name-only --exit-code
  else
    trap restore_current_commit EXIT
  fi
fi

echo "--- :git: fetch origin v${tag}"
# For back-compat tests, the shared fetching code doesn't guarantee that
# the tag has been fetched (because not all tests require tags), so fetch it here.
git fetch origin tag "v${tag}"

echo "--- :git::rewind: checkout v${tag}"
# --no-overlay makes so that git ensures the files match what is in the tree exactly, removing files that do not match
git checkout --force "v${tag}"

echo "--- :git: checkout migrations, patches and scripts at ${current_commit}"
# --no-overlay makes so that git ensures the files match what is in the tree exactly, removing files that do not match
git checkout --force --no-overlay "${current_commit}" -- migrations/ dev/backcompat/patch_flakes.sh dev/backcompat/patches dev/backcompat/flakes.json

if [[ -d "dev/backcompat/patches/${tag}" ]]; then
  echo "--- :adhesive_bandage: apply patches from dev/backcompat/patches/${tag}"
  for patch in dev/backcompat/patches/"${tag}"/*.patch; do
    echo "applying patch '${patch}'"
    git apply "${patch}"
  done
fi

echo "--- :snowflake: patch flake for tag ${tag}"
./dev/backcompat/patch_flakes.sh ${tag}

echo "--- :bazel: bazel test"
bazel "${bazelrcs[@]}" \
  test --test_tag_filters=go -- \
  //cmd/... \
  //lib/... \
  //internal/... \
  -//cmd/migrator/...
