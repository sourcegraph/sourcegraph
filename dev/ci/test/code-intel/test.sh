#!/usr/bin/env bash

# This script runs the codeintel-qa test utility against a candidate server image.

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."
SG_ROOT=$(pwd)
set -ex

function run_tests() {
  export SOURCEGRAPH_BASE_URL="$1"

  echo '--- initializing Sourcegraph instance'

  pushd internal/cmd/init-sg
  go build -o "${SG_ROOT}/init-sg"
  popd

  pushd dev/ci/test/code-intel
  "${SG_ROOT}/init-sg" initSG
  # Disable `-x` to avoid printing secrets
  set +x
  source /root/.profile
  export GITHUB_TOKEN="${GITHUB_USER_BOB_TOKEN}"
  set -x
  "${SG_ROOT}/init-sg" addRepos -config repos.json
  popd

  pushd dev/codeintel-qa
  echo '--- downloading testdata from GCS'
  ./scripts/download.sh

  echo '--- integration test ./dev/codeintel-qa/cmd/upload'
  go build ./cmd/upload
  ./upload --verbose --timeout=5m

  echo '--- integration test ./dev/codeintel-qa/cmd/query'
  go build ./cmd/query
  ./query
  popd
}

export IMAGE="us.gcr.io/sourcegraph-dev/server:$CANDIDATE_VERSION"

# us.gcr.io is a private registry, ensure we can pull
yes | gcloud auth configure-docker

# Setup single-server instance and run tests
./dev/ci/backend-integration-setup.sh run_tests
