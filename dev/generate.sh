#!/usr/bin/env bash

set -e
cd "$(dirname "${BASH_SOURCE[0]}")/.." # cd to repo root dir

# We'll exclude generating the CLI reference documentation by default due to the
# relatively high cost of fetching and building src-cli.
go list ./... | grep -v 'doc/cli/references' | xargs go generate -x
GOBIN="$PWD/.bin" go get golang.org/x/tools/cmd/goimports && ./.bin/goimports -w .
