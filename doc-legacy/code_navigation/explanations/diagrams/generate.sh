#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")"

set -ex

declare mermaid_diagrams=(
  upload-states
  index-states
)

# Install mermaid util
pnpm install

# Generate mermaid diagrams
for diagram in "${mermaid_diagrams[@]}"; do
  pnpm mmdc -i "$(pwd)/${diagram}.mermaid" -o "$(pwd)/${diagram}.svg"

  # Make the generated id deterministic so CI won't see superflouous changes
  sed -i '' "s/mermaid-[0-9]\{1,\}/mermaid-${diagram}/g" "${diagram}.svg"
done
