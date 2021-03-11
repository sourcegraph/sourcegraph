#!/usr/bin/env bash

set -e

# For symbol tests
echo "--- build libsqlite"
./dev/libsqlite3-pcre/build.sh

# For searcher
echo "--- comby install"
./dev/comby-install-or-upgrade.sh

# For code insights test
./dev/codeinsights-db.sh &
export CODEINSIGHTS_PGDATASOURCE=postgres://postgres:password@127.0.0.1:5435/postgres
export DB_STARTUP_TIMEOUT=120s # codeinsights-db needs more time to start in some instances.

# Separate out time for go mod from go test
echo "--- go mod download"
go mod download

echo "--- go test"
go test -timeout 4m -coverprofile=coverage.txt -covermode=atomic -race ./...

# Test lib submodule
pushd lib >/dev/null

echo "--- go mod download lib"
go mod download

echo "--- go test lib"
go test -timeout 4m -coverprofile=coverage.txt -covermode=atomic -race ./...

popd >/dev/null

# Test enterprise/lib submodule
pushd enterprise/lib >/dev/null

echo "--- go mod download enterprise/lib"
go mod download

echo "--- go test enterprise/lib"
go test -timeout 4m -coverprofile=coverage.txt -covermode=atomic -race ./...

popd >/dev/null
