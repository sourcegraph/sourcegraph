# vim: filetype=bash
#!/usr/bin/env bash

set -eu

NEW_VERSION="$1"
if ! [[ "$NEW_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Provided version is invalid: '$NEW_VERSION'"
    exit 1
fi

# Ensure we're at the root of the repository.
if [ ! -d ".git" ]; then
  echo "This command must run at the root of the sourcegraph repository."
  echo "Please run it again with:"
  echo "  bazel run :release_patch --run_under=\"cd $PWD &&\""
  exit 1
fi

release_branch="wip_${NEW_VERSION}"

echo "Checking out a new branch named $release_branch"
git checkout -b "$release_branch"

# Update the buildfile for schema_descriptions so it has our new schema.
buildozer "add outs schema-descriptions/${NEW_VERSION}-internal_database_schema.codeinsights.json schema-descriptions/${NEW_VERSION}-internal_database_schema.codeintel.json schema-descriptions/${NEW_VERSION}-internal_database_schema.json" //cmd/migrator:schema_descriptions

# TODO: this is merely for being able to iterate while it's still WIP
# ultimately, we will be creating a tag for internal releases.
# Update the shell script powering that target
echo "${NEW_VERSION}" >> cmd/migrator/wip_git_versions.txt
git add cmd/migrator/BUILD.bazel cmd/migrator/wip_git_versions.txt

# Add the newly generated schemas
git add internal/database/*.json
git add internal/database/*.md

git commit -m "release_patch: build ${NEW_VERSION}"

git push origin "$release_branch"
gh pr create -f -t "PRETEND RELEASE WIP: release_patch: build ${NEW_VERSION}" # -l "wip_release"
