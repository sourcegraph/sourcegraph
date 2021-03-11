#!/usr/bin/env bash

# This script ensures OSS sourcegraph does not import enterprise sourcegraph.

echo "--- go enterprise import"

trap "echo ^^^ +++" ERR

set -euxf -o pipefail

prefix=github.com/sourcegraph/sourcegraph/enterprise
# shellcheck disable=SC2016
template='{{with $pkg := .}}{{ range $pkg.Imports }}{{ printf "%s imports %s\n" $pkg.ImportPath .}}{{end}}{{end}}'

if go list ./../../... |
  grep -v "^$prefix" |
  xargs go list -f "$template" |
  grep "$prefix"; then
  echo "Error: OSS is not allowed to import enterprise"
  echo "^^^ +++"
  exit 1
fi
