#!/usr/bin/env bash

echo "--- go generate"

trap "echo ^^^ +++" ERR

set -eo pipefail

main() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.."

  export GOBIN="$PWD/.bin"
  export PATH=$GOBIN:$PATH
  export GO111MODULE=on

  # Runs generate.sh and ensures no files changed. This relies on the go
  # generation that ran are idempotent.
  ./dev/generate.sh
  git diff --exit-code -- . ':!go.sum'
}

main "$@"
