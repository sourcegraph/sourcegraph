#!/usr/bin/env bash

set -eu

if [ "$VERSION" = "" ]; then
  echo "❌ Need \$VERSION to be set to promote images"
  exit 1
fi

if [ "$#" -lt 1 ]; then
  echo "❌ Usage: $0 gitserver blobstore <image-name-without-registry...> ..."
  exit 1
fi

echo -e "## Release: image promotions" > ./annotations/image_promotions.md
echo -e "\n| Name | From | To |\n|---|---|---|" >> ./annotations/image_promotions.md
for name in "${@:1}"; do
  echo "--- Copying ${name} from private registry to public registry"

  docker pull "${INTERNAL_REGISTRY}/${name}:${VERSION}"
  docker tag "${INTERNAL_REGISTRY}/${name}:${VERSION}" "${PUBLIC_REGISTRY}/${name}:${VERSION}"
  docker push "${PUBLIC_REGISTRY}/${name}:${VERSION}"

  echo -e "| ${name} | \`${INTERNAL_REGISTRY}/${name}:${VERSION}\` | \`${PUBLIC_REGISTRY}/${name}:${VERSION}\` |" >>./annotations/image_promotions.md
done
