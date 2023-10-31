#!/usr/bin/env bash

# This script generates all the schema-descriptions files.

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
set -eu

OUTPUT="$1"
WIP_GIT_VERSIONS_TEXT="$2"

echo "Compiling schema descriptions ..."
mkdir -p "${OUTPUT}/schema-descriptions"

# See internal/database/migration/cliutil/drift-schemas/generate-all.sh
gcs_versions=(
  v3.20.0 v3.20.1
  v3.21.0 v3.21.1 v3.21.2
  v3.22.0 v3.22.1
  v3.23.0
  v3.24.0 v3.24.1
  v3.25.0 v3.25.1 v3.25.2
  v3.26.0 v3.26.1 v3.26.2 v3.26.3
  v3.27.0 v3.27.1 v3.27.2 v3.27.3 v3.27.4 v3.27.5
  v3.28.0
  v3.29.0 v3.29.1
  v3.30.0 v3.30.1 v3.30.2 v3.30.3 v3.30.4
  v3.31.0 v3.31.1 v3.31.2
  v3.32.0 v3.32.1
  v3.33.0 v3.33.1 v3.33.2
  v3.34.0 v3.34.1 v3.34.2
  v3.35.0 v3.35.1 v3.35.2
  v3.36.0 v3.36.1 v3.36.2 v3.36.3
  v3.37.0
  v3.38.0 v3.38.1
  v3.39.0 v3.39.1
  v3.40.0 v3.40.1 v3.40.2
  v3.41.0 v3.41.1
)
gcs_filenames=(
  internal_database_schema.json
  internal_database_schema.codeintel.json
  internal_database_schema.codeinsights.json
)

function download_gcs() {
  outfile="${OUTPUT}/schema-descriptions/${1}-${2}"
  # 3.20.0 is missing the codeintel and codeinsights schemas.
  if ! curl -fsSL "https://storage.googleapis.com/sourcegraph-assets/migrations/drift/${1}-${2}" 2>/dev/null >"${outfile}"; then
    rm "${outfile}"
  fi
}

for version in "${gcs_versions[@]}"; do
  for filename in "${gcs_filenames[@]}"; do
    download_gcs "${version}" "${filename}"
  done
done

function download_github() {
  local version
  version="$1"
  local github_url
  github_url="https://raw.githubusercontent.com/sourcegraph/sourcegraph/${version}/internal/database"

  curl -fsSL "$github_url/schema.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.json"
  curl -fsSL "$github_url/schema.codeintel.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.codeintel.json"
  curl -fsSL "$github_url/schema.codeinsights.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.codeinsights.json"
}

git_versions=(
  v3.42.0 v3.42.1 v3.42.2
  v3.43.0 v3.43.1 v3.43.2
  v4.0.0 v4.0.1
  v4.1.0 v4.1.1 v4.1.2 v4.1.3
  v4.2.0 v4.2.1
  v4.3.0 v4.3.1
  v4.4.0 v4.4.1 v4.4.2
  v4.5.0 v4.5.1
  v5.0.0 v5.0.1 v5.0.2 v5.0.3 v5.0.4
  v5.1.0 v5.1.1 v5.1.2 v5.1.3 v5.1.4 v5.1.5 v5.1.6 v5.1.7 v5.1.8 v5.1.9 v5.2.0)

for version in "${git_versions[@]}"; do
  download_github "${version}"
done

function wip_download_github() {
  local version
  version="$1"

  local github_url
  github_url="https://raw.githubusercontent.com/sourcegraph/sourcegraph/wip_${version}/internal/database"

  curl -fsSL "$github_url/schema.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.json"
  curl -fsSL "$github_url/schema.codeintel.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.codeintel.json"
  curl -fsSL "$github_url/schema.codeinsights.json" >"${OUTPUT}/schema-descriptions/${version}-internal_database_schema.codeinsights.json"
}

wip_git_versions=()

while IFS= read -r line; do
  wip_git_versions+=("$line")
done < "$WIP_GIT_VERSIONS_TEXT"

for version in "${wip_git_versions[@]}"; do
  wip_download_github "${version}"
done
