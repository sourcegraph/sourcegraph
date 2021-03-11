#!/usr/bin/env bash

echo "--- lint dependencies"

trap "echo ^^^ +++" ERR

set -ex
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

export GOBIN="$PWD/.bin"
export PATH=$GOBIN:$PATH
export GO111MODULE=on

if [ $# -eq 0 ]; then
  pkgs=('./...')
else
  pkgs=("$@")
fi

echo "--- lint"
"./dev/golangci-lint.sh" --config .golangci.enforced.yml run "${pkgs[@]}"
