#!/usr/bin/env bash

set -e

echo "~~~ :aspect: :stethoscope: Agent Health check"
/etc/aspect/workflows/bin/agent_health_check

aspectRC="/tmp/aspect-generated.bazelrc"
rosetta bazelrc > "$aspectRC"
bazelrc=(--bazelrc="$aspectRC")

echo "--- :books: Annotating build with Glossary"
buildkite-agent annotate --style info <./dev/ci/glossary.md

echo "--- :bazel: Build pipeline generator"
bazel "${bazelrc[@]}" \
  build \
  //dev/ci:ci

pipeline_gen="$(bazel "${bazelrc[@]}" cquery //dev/ci:ci --output files)"

echo "--- :writing_hand: Generate pipeline"
$pipeline_gen | tee generated-pipeline.yml

echo ""
echo "--- :arrow_up: Upload pipeline"
buildkite-agent pipeline upload generated-pipeline.yml
