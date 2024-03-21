#!/usr/bin/env bash

set -eu

cwd="$(pwd)"
_gh="${cwd}/$1"
_md2mdx="${cwd}/$2"

# Ensure we leave the output tree clean.
trap 'rm -Rf ${cwd}/_clone' EXIT

mkdir -p _clone/
git clone --quiet git@github.com:sourcegraph/docs.git _clone/

# See //monitoring:generate_config
cp -R -L monitoring/outputs/docs/* _clone/docs/admin/observability
cp -R -L doc/cli/** _clone/docs/cli/

# From now on, we're inside the cloned sourcegraph/docs repo.
pushd _clone

# Create a fresh branch from `main`
_current_date="$(date "+%Y-%m-%d/%H-%M-%S")"
_branch="sync/${_current_date}"
git checkout -b "$_branch"

# A bit unecessary, but better be safe than sorry
cd docs/

# Rename unstaged changes (for if the file exists in the docs)
for f in $(git ls-files --modified | grep '.md' | grep -v '.mdx'); do
  # Turn .md into .mdx
  "$_md2mdx" "${f}" > "${f}x"
done

# Rename untracked changes (for if the file doesn't exists yet in the docs)
for f in $(git ls-files --others | grep '.md' | grep -v '.mdx'); do
  # Turn .md into .mdx
  "$_md2mdx" "${f}" > "${f}x"
done

cd ..

find docs/admin/observability -name "*.bazel" -print0 | xargs --null --no-run-if-empty rm
find docs/cli -name "*.bazel" -print0 | xargs --null --no-run-if-empty rm

# Remove the markdown files we generated in favour of their mdx counterpart
find docs/admin/observability -name "*.md" -print0 | xargs --null --no-run-if-empty rm
find docs/cli -name "*.md" -print0 | xargs --null --no-run-if-empty rm

git add .
git diff
git commit -m "🤖 sync'ing generated docs"

git push origin "$_branch"

# For some reason, the GH_TOKEN takes precedence over GITHUB_TOKEN (RIP my dear sanity)
# TODO(burmudar): fix this token in the agent image so that we don't have to do this.
# https://github.com/sourcegraph/sourcegraph/issues/61315
unset GH_TOKEN
export GITHUB_TOKEN="$BUILDKITE_GITHUBDOTCOM_TOKEN"
"$_gh" pr create \
  --reviewer MaedahBatool \
  --title "🤖 Sync generated docs from sourcegraph/sourcegraph (${_current_date})" \
  --body "This is an automated pull request, created by //doc:generated:push on sourcegraph/sourcegraph"
