#!/usr/bin/env bash

echo "--- shfmt (ensure shell-scripts are formatted consistently)"

trap "echo ^^^ +++" ERR

set -ex

cd "$(dirname "${BASH_SOURCE[0]}")"/../..

shfmt -d .
