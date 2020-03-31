#!/usr/bin/env bash

# We want to build multiple go binaries, so we use a custom build step on CI.
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
set -eux

export OUTPUT=$(mktemp -d -t sgserver_XXXXXXX)
cleanup() {
    rm -rf "$OUTPUT"
}
trap cleanup EXIT

parallel_run() {
    ./dev/ci/parallel_run.sh "$@"
}
export -f parallel_run

# Environment for building linux binaries
export GO111MODULE=on
export GOARCH=amd64
export GOOS=linux
export CGO_ENABLED=0

# Additional images passed in here when this script is called externally by our
# enterprise build scripts.
export additional_images=${@:-github.com/sourcegraph/sourcegraph/cmd/frontend github.com/sourcegraph/sourcegraph/cmd/repo-updater}

# Overridable server package path for when this script is called externally by
# our enterprise build scripts.
export server_pkg=${SERVER_PKG:-github.com/sourcegraph/sourcegraph/cmd/server}

cp -a ./cmd/server/rootfs/. "$OUTPUT"
export BINDIR="$OUTPUT/usr/local/bin"
mkdir -p "$BINDIR"

go_build() {
    local package="$1"

    if [[ "${CI_DEBUG_PROFILE:-"false"}" == "true" ]]; then
        env time -v ./cmd/server/go-build.sh $package
    else
        ./cmd/server/go-build.sh $package
    fi
}
export -f go_build

echo "--- build go and symbols concurrently"

build_go_packages() {
   echo "--- go build"

   PACKAGES=(
    github.com/sourcegraph/sourcegraph/cmd/github-proxy \
    github.com/sourcegraph/sourcegraph/cmd/gitserver \
    github.com/sourcegraph/sourcegraph/cmd/query-runner \
    github.com/sourcegraph/sourcegraph/cmd/replacer \
    github.com/sourcegraph/sourcegraph/cmd/searcher \
    $additional_images
    \
    github.com/google/zoekt/cmd/zoekt-archive-index \
    github.com/google/zoekt/cmd/zoekt-sourcegraph-indexserver \
    github.com/google/zoekt/cmd/zoekt-webserver \
    \
    $server_pkg
   )

   parallel_run go_build {} ::: "${PACKAGES[@]}"
}
export -f build_go_packages

build_symbols() {
    echo "--- build sqlite for symbols"
    if [[ "${CI_DEBUG_PROFILE:-"false"}" == "true" ]]; then
        env OUTPUT="$BINDIR" time -v ./cmd/symbols/go-build.sh
    else
        env OUTPUT="$BINDIR" ./cmd/symbols/go-build.sh
    fi
}
export -f build_symbols

parallel_run {} ::: build_go_packages build_symbols

echo "--- ctags"
cp -a ./cmd/symbols/.ctags.d "$OUTPUT"
cp -a ./cmd/symbols/ctags-install-alpine.sh "$OUTPUT"
cp -a ./dev/libsqlite3-pcre/install-alpine.sh "$OUTPUT/libsqlite3-pcre-install-alpine.sh"

echo "--- lsif server"
cp -a ./cmd/lsif-server "$OUTPUT"

echo "--- prometheus config"
cp -r docker-images/prometheus/config "$OUTPUT/sg_config_prometheus"
mkdir "$OUTPUT/sg_prometheus_add_ons"
cp dev/prometheus/linux/prometheus_targets.yml "$OUTPUT/sg_prometheus_add_ons"

echo "--- grafana config"
cp -r docker-images/grafana/config "$OUTPUT/sg_config_grafana"
cp -r dev/grafana/linux "$OUTPUT/sg_config_grafana/provisioning/datasources"

echo "--- docker build"
docker build -f cmd/server/Dockerfile -t "$IMAGE" "$OUTPUT" \
    --progress=plain \
    --build-arg COMMIT_SHA \
    --build-arg DATE \
    --build-arg VERSION
