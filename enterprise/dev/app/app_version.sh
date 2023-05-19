#!/usr/bin/env bash
set -eu

cd "$(dirname "${BASH_SOURCE[0]}")"/../../.. || exit 1

create_version() {
    local sha
    # In a GitHub action this can result in an empty sha
    sha=$(git rev-parse --short HEAD)
    if [[ -z ${sha} ]]; then
      sha=${BUILDKITE_COMMIT:-""}
    fi

    local build="insiders"
    if [[ ${BUILDKITE_BRANCH:-""} == "app-release/stable" ]]; then
      build=${BUILDKITE_BUILD_NUMBER:-"release"}
    fi
    echo "$(date '+%Y.%-m.%-d')+${build}.${sha}"
}

if [[ ${CI:-""} == "true" ]]; then
  version=${VERSION:-$(create_version)}
else
  version=${VERSION:-"0.0.0+dev"}
fi

echo "${version}"
