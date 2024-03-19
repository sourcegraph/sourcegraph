#!/usr/bin/env bash

cd "$(dirname "${BASH_SOURCE[0]}")"

set -ex

declare mermaid_diagrams=(
  definitions
  executor
  extension-definitions
  extension-hover
  extension-references
  hover
  references
  upload
)

# Install mermaid util
pnpm
mermaid="../../../../../node_modules/.bin/mmdc"

# Generate mermaid diagrams
for diagram in "${mermaid_diagrams[@]}"; do
  "${mermaid}" -i "${diagram}.mermaid" -o "${diagram}.svg"

  # Make the generated id deterministic so CI won't see superflouous changes
  sed -i '' "s/mermaid-[0-9]\{1,\}/mermaid-${diagram}/g" "${diagram}.svg"
done
