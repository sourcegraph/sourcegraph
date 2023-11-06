#!/usr/bin/env bash

set -e

echo "--- pnpm install in root"
./dev/ci/pnpm-install-with-retry.sh

echo "--- generate"
pnpm run generate

echo "--- test"

JEST_JUNIT_OUTPUT_NAME="pnpm-test-junit.xml"
export JEST_JUNIT_OUTPUT_NAME
JEST_JUNIT_OUTPUT_DIR="./test-reports"
export JEST_JUNIT_OUTPUT_DIR
mkdir -p "$JEST_JUNIT_OUTPUT_DIR"

# Limit the number of workers to prevent the default of 1 worker per core from
# causing OOM on the buildkite nodes that have 96 CPUs. 4 matches the CPU limits
# in infrastructure/kubernetes/ci/buildkite/buildkite-agent/buildkite-agent.Deployment.yaml
pnpm run test --maxWorkers 4 --verbose --testResultsProcessor jest-junit "$@"
