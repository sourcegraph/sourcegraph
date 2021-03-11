#!/usr/bin/env bash

set -e

echo "--- docsite check (lint Markdown files in doc/)"

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# Check broken links, etc., in Markdown files in doc/.

echo
echo

./dev/docsite.sh check || {
  echo
  echo Errors found in Markdown documentation files. Fix the errors in doc/ and try again.
  echo
  echo "^^^ +++"
  exit 1
}
