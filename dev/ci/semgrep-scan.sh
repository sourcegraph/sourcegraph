#!/usr/bin/env bash

set -e

echo -e "--- :construction: cloning Semgrep rules\n"

# clone the semgrep repo rules
gh repo clone sourcegraph/security-semgrep-rules

echo -e "--- :lock::semgrep: running Semgrep scan\n"

set -x

# verify security-semgrep-rules/semgrep-rules/ directory is present or print error
if [ ! -d "security-semgrep-rules/semgrep-rules/" ]; then
  echo ":red_circle: Semgrep rules directory not found. Reachout to security team at #discuss-security for support :red_circle:"
fi

# run semgrep scan on changeset using CI subcommand
# || true is used to prevent build from failing if semgrep scan reports on blocking findings
# reference: https://semgrep.dev/docs/semgrep-ci/configuring-blocking-and-errors-in-ci/#configuration-options-for-blocking-findings-and-errors
semgrep ci -f 'security-semgrep-rules/semgrep-rules/' --metrics=off --oss-only --sarif -o results.sarif --exclude='semgrep-rules' --baseline-commit "$(git merge-base main HEAD)" || true

echo -e "--- :rocket: reporting scan results to GitHub\n"

# upload SARIF results to code scanning API
encoded_sarif=$(gzip -c results.sarif | base64 -w0)

# upload SARIF results to code scanning API
if [ "$BUILDKITE_PULL_REQUEST" = "false" ]; then
  gh api \
    --method POST \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    /repos/sourcegraph/sourcegraph/code-scanning/sarifs \
    -f commit_sha="$BUILDKITE_COMMIT" \
    -f ref="refs/heads/$BUILDKITE_BRANCH" \
    -f sarif="$encoded_sarif" \
    -f tool_name="ci semgrep"
else
  gh api \
    --method POST \
    -H "Accept: application/vnd.github+json" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    /repos/sourcegraph/sourcegraph/code-scanning/sarifs \
    -f commit_sha="$BUILDKITE_COMMIT" \
    -f ref="refs/pull/$BUILDKITE_PULL_REQUEST/head" \
    -f sarif="$encoded_sarif" \
    -f tool_name="ci semgrep"
fi

echo -e "--- :white_check_mark::semgrep: Semgrep scan job is complete\n"
