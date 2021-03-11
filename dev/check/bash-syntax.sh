#!/usr/bin/env bash

echo "--- bash syntax"

trap "echo ^^^ +++" ERR

set -e
cd "$(dirname "${BASH_SOURCE[0]}")"/../..

find dev -name '*.sh' -print0 | xargs -0 -n 1 bash -n
