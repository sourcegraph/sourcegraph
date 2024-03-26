#!/usr/bin/env bash

# Run `sg wolfi lock` to update all package lockfiles for Wolfi base images.
# Push a new branch to GitHub, and open a PR.
# Can be run from any base branch, and will create an appropriate PR.

set -eu -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."

# Update hashes for all base images
bazel \
  --bazelrc=.bazelrc \
  --bazelrc=.aspect/bazelrc/ci.bazelrc \
  --bazelrc=.aspect/bazelrc/ci.sourcegraph.bazelrc \
  run //dev/sg -- wolfi lock
# Print diff
git diff wolfi-images/*.lock.json

# Git and GitHub config
BRANCH_NAME="wolfi-auto-update/${BUILDKITE_BRANCH}"
TIMESTAMP=$(TZ=UTC date "+%Y-%m-%d %H:%M:%S UTC")
PR_TITLE="Auto-update package lockfiles for Wolfi base images"
# PR_REVIEWER="sourcegraph/security"
PR_LABELS="SSDLC,wolfi-auto-update"
PR_BODY="Automatically generated PR to update package lockfiles for Wolfi base images.

Built from Buildkite run [#${BUILDKITE_BUILD_NUMBER}](https://buildkite.com/sourcegraph/sourcegraph/builds/${BUILDKITE_BUILD_NUMBER}).
## Test Plan
- CI build verifies image functionality
- [ ] Confirm PR should be backported to release branch"

# Commit changes to dev/oci-deps.bzl
# Delete branch if it exists; catch status code if not
git branch -D "${BRANCH_NAME}" || :
git checkout -b "${BRANCH_NAME}"
git add wolfi-images/*.lock.json
git commit -m "Auto-update package lockfiles for Wolfi base images at ${TIMESTAMP}"
git push --force -u origin "${BRANCH_NAME}"
echo ":git: Successfully commited changes and pushed to branch ${BRANCH_NAME}"

# Check if an update PR already exists
if gh pr list --head "${BRANCH_NAME}" --state open | grep -q "${PR_TITLE}"; then
  echo ":github: A pull request already exists - editing it"
  gh pr edit "${BRANCH_NAME}" --body "${PR_BODY}"
else
  # If not, create a new PR from the branch main
  # TODO: Once validated add '--reviewer "${PR_REVIEWER}"'
  gh pr create --title "${PR_TITLE}" --head "${BRANCH_NAME}" --base "${BUILDKITE_BRANCH}" --body "${PR_BODY}" --label "${PR_LABELS}"
  echo ":github: Created a new pull request from branch '${BRANCH_NAME}' with title '${PR_TITLE}'"
fi
