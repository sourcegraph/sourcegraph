#!/usr/bin/env bash

set -e
echo "--- no alpine guard"

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

path_filter() {
  local IFS=
  local withPath="${*/#/ -o -path }"
  echo "${withPath# -o }"
}

set +e
ALPINE_MATCHES=$(git grep -e '\salpine\:' --and --not -e '^\s*//' --and --not -e 'CI\:LOCALHOST_OK' \
  ':(exclude)doc/admin/updates/docker_compose.md' \
  ':(exclude)docker-images/README.md' \
  ':(exclude)docker-images/alpine' \
  ':(exclude)doc/batch_changes/' \
  ':(exclude)web/src/enterprise/batches/create/CreateBatchChangePage.tsx' \
  ':(exclude)*vendor*' \
  ':(exclude)*testdata*')
set -e

if [ -n "$ALPINE_MATCHES" ]; then
  echo
  echo "Error: Found instances of \"alpine:\":"
  # shellcheck disable=SC2001
  echo "$ALPINE_MATCHES" | sed 's/^/  /'

  cat <<EOF

Using 'alpine' is forbidden. Use 'sourcegraph/alpine' instead which provides:

- Fixes DNS resolution in some deployment environments.
- A non-root 'sourcegraph' user.
- Static UID and GIDs that are consistent across all containers.
- Base packages like 'tini' and 'curl' that we expect in all containers.

You should use 'sourcegraph/alpine' even in build stages for consistency sake.
Use explicit 'USER root' and 'USER sourcegraph' sections when adding packages, etc.

If the linter is incorrect, either:
1) add the comment "CI:ALPINE_OK" to the line where "alpine" occurs, or
2) add an exclusion clause in the "git grep" command in  no-alpine-guard.sh

EOF
  echo "^^^ +++"
  exit 1
fi
