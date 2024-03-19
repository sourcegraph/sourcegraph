#!/usr/bin/env bash

set -eu -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."

KEYS_DIR="/etc/sourcegraph/keys/"
GCP_PROJECT="sourcegraph-ci"
GCS_BUCKET="package-repository"
TARGET_ARCH="x86_64"
MAIN_BRANCH="main"
BRANCH="${BUILDKITE_BRANCH:-'default-branch'}"
IS_MAIN=$([ "$BRANCH" = "$MAIN_BRANCH" ] && echo "true" || echo "false")

# shellcheck disable=SC2001
BRANCH_PATH=$(echo "$BRANCH" | sed 's/[^a-zA-Z0-9_-]/-/g')
if [[ "$IS_MAIN" != "true" ]]; then
  BRANCH_PATH="branches/$BRANCH_PATH"
fi

tmpdir=$(mktemp -d -t melange-bin.XXXXXXXX)
function cleanup() {
  echo "Removing $tmpdir"
  rm -rf "$tmpdir"
}
trap cleanup EXIT

# TODO: Install these binaries as part of the buildkite base image
(
  cd "$tmpdir"
  mkdir bin

  # Install melange from Sourcegraph cache
  # Source: https://github.com/chainguard-dev/melange/releases/download/v0.2.0/melange_0.2.0_linux_amd64.tar.gz
  wget https://storage.googleapis.com/package-repository/ci-binaries/melange_0.2.0_linux_amd64.tar.gz
  tar zxf melange_0.2.0_linux_amd64.tar.gz
  mv melange_0.2.0_linux_amd64/melange bin/melange
)

export PATH="$tmpdir/bin:$PATH"

apkindex_build_dir=$(mktemp -d -t apkindex-build.XXXXXXXX)
pushd "$apkindex_build_dir"

# Fetch all APKINDEX fragments from bucket
gsutil -m cp "gs://$GCS_BUCKET/$BRANCH_PATH/$TARGET_ARCH/*.APKINDEX.fragment" ./

# Concat all fragments into a single APKINDEX and tar.gz it
touch placeholder.APKINDEX.fragment
cat ./*.APKINDEX.fragment >APKINDEX
touch DESCRIPTION
tar zcf APKINDEX.tar.gz APKINDEX DESCRIPTION

# Sign index, using separate keys from GCS for staging and prod repos
if [[ "$IS_MAIN" == "true" ]]; then
  key_path="$KEYS_DIR/sourcegraph-melange-prod.rsa"
else
  key_path="$KEYS_DIR/sourcegraph-melange-dev.rsa"
fi
melange sign-index --signing-key "$key_path" APKINDEX.tar.gz

# Upload signed APKINDEX archive
# Use no-cache to avoid index/packages getting out of sync
gsutil -u "$GCP_PROJECT" -h "Cache-Control:no-cache" cp APKINDEX.tar.gz "gs://$GCS_BUCKET/$BRANCH_PATH/$TARGET_ARCH/"
