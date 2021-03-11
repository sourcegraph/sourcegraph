#!/usr/bin/env bash

echo "--- template inlines"

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

set -euf -o pipefail
unset CDPATH

# Fails and prints matches if any HTML template files contain inline
# scripts or styles.
main() {
  local template_dir=cmd/frontend/internal/app/templates
  if [[ ! -d "${template_dir}" ]]; then
    echo "Could not find directory ${template_dir}; did it move?"
    echo "^^^ +++"
    exit 1
  fi
  local found
  found=$(grep -EHnr '(<script|<style|style=)' "${template_dir}" | grep -v '<script src=' | grep -v '<script ignore-csp' | grep -v '<div ignore-csp' | grep -v '<style ignore-csp' | grep -v '<iframe ignore-csp' || echo -n)

  if [[ ! "$found" == "" ]]; then
    echo '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!'
    echo '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!'
    echo 'Found instances of inline script and style tags in HTML templates. These violate our CSP. Fix these!'
    echo '(See http://www.html5rocks.com/en/tutorials/security/content-security-policy/ for more info about CSP.)'
    echo '<script src="foo"> tags are OK, and <link rel="stylesheet" href=""> tags are OK. To make the former pass'
    echo 'this check script, put the src attribute immediately after "<script". (This script just uses a simple grep.)'
    echo '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!'
    echo '!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!'
    echo "$found"
    echo "^^^ +++"
    exit 1
  fi

  exit 0
}

main "$@"
