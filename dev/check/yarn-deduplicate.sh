#!/usr/bin/env bash
set -eu -o pipefail

echo "--- check yarn.lock for duplicates"

# Prevent duplicates in yarn.lock/node_modules that lead to errors and bloated bundle sizes

# mutex is necessary since CI runs various yarn installs in parallel
yarn --mutex network --frozen-lockfile --ignore-scripts

echo "Checking for duplicate dependencies in yarn.lock"
yarn run -s yarn-deduplicate --fail --list --strategy fewer ./yarn.lock || {
  echo 'yarn.lock contains duplicate dependencies. Please run "yarn deduplicate" and commit the result.'
  echo "^^^ +++"
  exit 1
}
