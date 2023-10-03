#!/usr/bin/env bash

set -eu
# Ensure we're at the root of the repository.
if [ ! -d ".git" ]; then
  echo "This command must run at the root of the sourcegraph repository."
  echo "Please run it again with:"
  echo "  bazel run :release_patch --run_under=\"cd $PWD &&\""
  exit 1
fi

release_branch="wip_{{new_version}}"

echo "Checking out a new branch named {{new_version}}"
git checkout -b "$release_branch"

# Update the buildfile for schema_descriptions so it has our new schema.
buildozer 'add outs schema-descriptions/{{new_version}}-internal_database_schema.codeinsights.json schema-descriptions/{{new_version}}-internal_database_schema.codeintel.json schema-descriptions/{{new_version}}-internal_database_schema.json' //cmd/migrator:schema_descriptions

# Update the shell script powering that target
echo "{{new_version}}" >> cmd/migrator/wip_git_versions.txt

git add cmd/migrator/BUILD.bazel cmd/migrator/wip_git_versions.txt
git commit -m "release_patch: build {{new_version}}"

git push origin "$release_branch"
gh pr create -f -t "PRETEND RELEASE WIP: release_patch: build {{new_version}}"
