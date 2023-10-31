#!/usr/bin/env bash

# TODO RFC 795
echo "PRETENDING TO FINALIZE RELEASE"

VERSION=$1
TITLE="WIP Release patch v${VERSION}"
echo "Querying for release PR with title '$TITLE'"

if [[ -z "$BUILDKITE_PULL_REQUEST" ]]; then
  echo "No PR found with title: $TITLE"
  exit 1
fi

echo "Approving release PR $BUILDKITE_PULL_REQUEST"
echo ">>> gh pr review --approve \"$BUILDKITE_PULL_REQUEST\""

echo "Merging release PR $BUILDKITE_PULL_REQUEST"
echo ">>> gh pr merge \"$BUILDKITE_PULL_REQUEST\""
