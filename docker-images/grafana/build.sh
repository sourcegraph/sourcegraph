#!/usr/bin/env bash

set -ex

cd "$(dirname "${BASH_SOURCE[0]}")"

# We build out of tree to prevent triggering dev watch scripts when we copy go
# files.
BUILDDIR=$(mktemp -d -t sgdockerbuild_XXXXXXX)
cleanup() {
  rm -rf "$BUILDDIR"
}
trap cleanup EXIT

# Copy assets
cp -R . "$BUILDDIR"

# Build args for Go cross-compilation.
export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux
export CGO_ENABLED=0

# Cross-compile monitoring generator before building the image.
pushd "../../monitoring"
go build \
  -trimpath \
  -o "$BUILDDIR"/.bin/monitoring-generator .

# Final pre-build stage.
pushd "$BUILDDIR"

# Enable image build caching via CACHE=true
BUILD_CACHE="--no-cache"
if [[ "$CACHE" == "true" ]]; then
  BUILD_CACHE=""
fi

# shellcheck disable=SC2086
docker build ${BUILD_CACHE} -f Dockerfile -t "${IMAGE:-sourcegraph/grafana}" . \
  --progress=plain \
  --build-arg COMMIT_SHA \
  --build-arg DATE \
  --build-arg VERSION

# cd out of $BUILDDIR for cleanup
popd
