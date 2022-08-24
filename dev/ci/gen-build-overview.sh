#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")"/../..
#set -euo pipefail

echo "--- Generating build overview annotation"
mkdir -p annotations

file="./annotations/Build overview.md"

if [[ ${BUILDKITE_PULL_REQUEST} -ne "false" ]]; then
    echo -e "Pull request [🔗]: \`${BUILDKITE_PULL_REQUEST}\`\n" >> "$file"
fi

echo -e "Build Number [🔗](${BUILDKITE_BUILD_URL}): \`${BUILDKITE_BUILD_NUMBER}\`\n" > "$file"
echo -e "Retry count: \`${BUILDKITE_RETRY_COUNT}\`\n" > "$file"
echo -e "Pipeline: ${BUILDKITE_PIPELINE_SLUG}\n" > "$file"
echo -e "Author: \`${BUILDKITE_BUILD_AUTHOR}\`\n" > "$file"
echo -e "Branch: \`${BUILDKITE_BRANCH}\`\n" > "$file"
echo -e "Commit: \`${BUILDKITE_COMMIT}\`\n" > "$file"
echo -e "\`\`\`\n" > "$file"
echo -e "${BUILDKITE_MESSAGE}\n" > "$file"
echo -e "\`\`\`\n" > "$file"
echo -e "Agent: \`${BUILDKITE_AGENT_NAME}\`\n" > "$file"
