#!/usr/bin/env bash

## Setting up inputs/tools
gsutil="$1"
executor_binary="$2"

## Script begins here
set -eu

GIT_COMMIT="$(git rev-parse HEAD)"

## Prepare artifacts
mkdir -p workdir/linux-amd64
workdir_abs="$(pwd)/workdir"
trap 'rm -Rf $workdir_abs' EXIT

cp "$executor_binary" workdir/linux-amd64
echo >>workdir/info.txt
# We need --no-mailmap to prevent git to try reading the .mailmap at the root of the repository,
# which will lead to a "too many levels of symbolic links" noisy warning.
git log --no-mailmap -n1 >>"workdir/info.txt"
sha256sum workdir/linux-amd64/executor >>workdir/linux-amd64/executor_SHA256SUM

# Set GCP Project to sourcegraph-dev, that's where the GCS bucket lives.
export CLOUDSDK_CORE_PROJECT="sourcegraph-dev"

echo "Uploading binaries for this commit"
"$gsutil" cp -r workdir/* "gs://sourcegraph-artifacts/executor/${GIT_COMMIT}"

if [ "${BUILDKITE_BRANCH}" = "main" ]; then
  echo "Uploading binaries as latest"
  # Drop the latest folder, as we'll overwrite it.
  "$gsutil" rm -rf gs://sourcegraph-artifacts/executor/latest || true
  "$gsutil" cp -r workdir/* gs://sourcegraph-artifacts/executor/latest/
fi

# If this is a tagged release, we want to create a directory for it.
if [ "${EXECUTOR_IS_TAGGED_RELEASE}" = "true" ]; then
  # This is a failsafe, to avoid dropping the entire folder whenever we
  # run this code without a proper tag, typically when testing releases.
  #
  # Without it, the tag will be empty and the gsutil rm -rf below will
  # drop the entire folder.
  if [ -z "$BUILDKITE_TAG" ] || [ "$BUILDKITE_TAG" = "" ]; then
    # But if a version is set and matches releases numbering scheme, we can
    # still use this. It's
    if [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo ":warning: inferring \$BUILDKITE_TAG from \$VERSION"
      export BUILDKITE_TAG="v${VERSION}"
    else
      echo ":red_circle: inferring \$BUILDKITE_TAG from \$VERSION"
      exit 1
    fi
  fi

  echo "Uploading binaries for the ${BUILDKITE_TAG} tag"
  # Drop the tag if existing, allowing for rebuilds.
  "$gsutil" rm -rf "gs://sourcegraph-artifacts/executor/${BUILDKITE_TAG}" || true
  "$gsutil" cp -r workdir/* "gs://sourcegraph-artifacts/executor/${BUILDKITE_TAG}/"
fi

# Just in case.
"$gsutil" iam ch allUsers:objectViewer gs://sourcegraph-artifacts
