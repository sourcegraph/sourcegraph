#!/usr/bin/env bash

set -euf -o pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."
REPO_DIR=$(pwd)

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

tmpdir=$(mktemp -d -t wolfi-bin.XXXXXXXX)
builddir=$(mktemp -d -t wolfi-build.XXXXXXXX)
function cleanup() {
  echo "Removing $tmpdir and $builddir"
  rm -rf "$tmpdir"
  rm -rf "$builddir"
}
trap cleanup EXIT

# TODO: Install these binaries as part of the buildkite base image
(
  cd "$tmpdir"
  mkdir bin

  # Install apko from Sourcegraph cache
  # Source: https://github.com/chainguard-dev/apko/releases/download/v0.10.0/apko_0.10.0_linux_amd64.tar.gz
  wget https://storage.googleapis.com/package-repository/ci-binaries/apko_0.10.0_linux_amd64.tar.gz
  tar zxf apko_0.10.0_linux_amd64.tar.gz
  mv apko_0.10.0_linux_amd64/apko bin/apko

  # Install apk from Sourcegraph cache
  # Source: https://gitlab.alpinelinux.org/api/v4/projects/5/packages/generic//v2.12.11/x86_64/apk.static
  wget https://storage.googleapis.com/package-repository/ci-binaries/apk-v2.12.11.tar.gz
  tar zxf apk-v2.12.11.tar.gz
  chmod +x apk
  mv apk bin/apk
)

export PATH="$tmpdir/bin:$PATH"

if [ $# -eq 0 ]; then
  echo "No arguments supplied - provide the base image name to build"
  exit 0
fi

name=${1%/}
# Soft-fail if file doesn't exist, as CI step is triggered whenever base image configs are changed - including deletions/renames
if [ ! -f "wolfi-images/${name}.yaml" ]; then
  echo "File '${name}.yaml' does not exist cwd: '${PWD}'"
  exit 222
fi

# If this is a branch, check if branch-specific package repo exists on GCS
branch_repo_exists="false"
if [[ "$IS_MAIN" != "true" ]]; then
  dest_path="gs://$GCS_BUCKET/$BRANCH_PATH/$TARGET_ARCH/"
  if gsutil -q -u "$GCP_PROJECT" stat "${dest_path}APKINDEX.tar.gz"; then
    echo "A branch-specific package repo exists for this branch at ${dest_path}"
    branch_repo_exists="true"
  else
    echo "No branch-specific package repo exists for this branch, not updating apko configs"
  fi
fi

tag=${2-latest}

echo "Setting up build dir..."
cp -r "wolfi-images/" "$builddir"
cd "$builddir/wolfi-images"

# Export date for apko (defaults to 0 for reproducibility)
SOURCE_DATE_EPOCH="$(date +%s)"
export SOURCE_DATE_EPOCH

# On branches, if we modify a package then we'd like that modified version to be included in any base images built.
# This is a bit hacky, but we do this by modifying the base image configs and passing the branch-specific repo to apko.
add_custom_repo_cmd=()
if [[ "$IS_MAIN" != "true" && "$branch_repo_exists" == "true" ]]; then
  add_custom_repo_cmd=("--repository-append" "@branch https://packages.sgdev.org/$BRANCH_PATH" "--keyring-append" "https://packages.sgdev.org/sourcegraph-melange-dev.rsa.pub")
  echo "Adding custom repo command: ${add_custom_repo_cmd[*]}"

  # Read the branch-specific package repo and extract the names of packages that have been modified
  modified_packages=()
  while IFS= read -r line; do
    modified_packages+=("$line")
  done < <(gsutil ls gs://package-repository/"$BRANCH_PATH"/x86_64/\*.apk | sed -E 's/.*\/x86_64\/([a-zA-Z0-9_-]+)-[0-9]+\..*/\1/')

  echo "List of modified packages to include in branch image: ${modified_packages[*]}"

  # In the base image configs, find and replace the packages which have been modified
  for element in "${modified_packages[@]}"; do
    echo "Replacing '$element@sourcegraph' with '$element@branch' in '${name}.yaml'"
    sed -i "s/$element@sourcegraph/$element@branch/g" "${name}.yaml"
  done

  echo -e "\nUpdated image config:"
  echo "------------"
  cat "${name}.yaml"
  echo -e "------------\n"
fi

# Build base image with apko
echo " * Building base image '$name' with apko..."
image_name="sourcegraph-wolfi/${name}-base"
tarball="sourcegraph-wolfi-${name}-base.tar"
apko build --debug "${add_custom_repo_cmd[@]}" \
  "${name}.yaml" \
  "$image_name:latest" \
  "$tarball" ||
  (echo "*** Build failed ***" && exit 1)

# Tag image and upload to GCP Artifact Registry
echo " * Loading built image into docker daemon..."
docker load <"$tarball"

# https://github.com/chainguard-dev/apko/issues/529
# there is an unexpcted behaviour in upstream
# where the arch is always appended to the tag
# hardcode for now as we only support linux/amd64 anyway
local_image_name="$image_name:latest-amd64"

# Push to internal dev repo
echo "* Pushing image to internal dev repo..."
docker tag "$local_image_name" "us.gcr.io/sourcegraph-dev/wolfi-${name}-base:$tag"
docker push "us.gcr.io/sourcegraph-dev/wolfi-${name}-base:$tag"
docker tag "$local_image_name" "us.gcr.io/sourcegraph-dev/wolfi-${name}-base:latest"
docker push "us.gcr.io/sourcegraph-dev/wolfi-${name}-base:latest"

# Push to Dockerhub only on main branch
if [[ "$IS_MAIN" == "true" ]]; then
  echo "* Pushing image to prod repo..."
  docker tag "$local_image_name" "sourcegraph/wolfi-${name}-base:$tag"
  docker push "sourcegraph/wolfi-${name}-base:$tag"
  docker tag "$local_image_name" "sourcegraph/wolfi-${name}-base:latest"
  docker push "sourcegraph/wolfi-${name}-base:latest"
fi

# Show image usage message on branches
if [[ "$IS_MAIN" != "true" ]]; then
  if [[ -n "$BUILDKITE" ]]; then
    mkdir -p ./annotations
    file="${name} image.md"
    cat <<-EOF > "${REPO_DIR}/annotations/${file}"

<strong>:octopus: ${name} image &bull; [View job output](#${BUILDKITE_JOB_ID})</strong>
<br />
<br />
Run the \`${name}\` base image locally using:

\`\`\`bash
docker pull us.gcr.io/sourcegraph-dev/wolfi-${name}-base:${tag}
\`\`\`

EOF
  fi
fi
