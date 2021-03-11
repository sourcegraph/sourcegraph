#!/usr/bin/env bash

set -euf -o pipefail

DEV_PRIVATE_PATH=$PWD/../dev-private

if [ ! -d "$DEV_PRIVATE_PATH" ]; then
  echo "Expected to find github.com/sourcegraph/dev-private checked out to $DEV_PRIVATE_PATH, but path wasn't a directory" 1>&2
  exit 1
fi

# Warn if dev-private needs to be updated.
required_commit="158d265ddcc7c7233421e35d4997b27300de39b8"
if ! git -C "$DEV_PRIVATE_PATH" merge-base --is-ancestor $required_commit HEAD; then
  echo "Error: You need to update dev-private to a commit that incorporates https://github.com/sourcegraph/dev-private/commit/$required_commit."
  echo
  echo "Try running:"
  echo
  echo "    cd $DEV_PRIVATE_PATH && git pull"
  echo
  exit 1
fi

# shellcheck disable=SC1090
source "$DEV_PRIVATE_PATH/enterprise/dev/env"

if [ -z "${DEV_NO_CONFIG-}" ]; then
  export SITE_CONFIG_FILE=${SITE_CONFIG_FILE:-$DEV_PRIVATE_PATH/enterprise/dev/site-config.json}
  export EXTSVC_CONFIG_FILE=${EXTSVC_CONFIG_FILE:-$DEV_PRIVATE_PATH/enterprise/dev/external-services-config.json}
  export GLOBAL_SETTINGS_FILE=${GLOBAL_SETTINGS_FILE:-$PWD/dev/global-settings.json}
  export SITE_CONFIG_ALLOW_EDITS=true
  export GLOBAL_SETTINGS_ALLOW_EDITS=true
  export EXTSVC_CONFIG_ALLOW_EDITS=true
fi

SOURCEGRAPH_LICENSE_GENERATION_KEY=$(cat "$DEV_PRIVATE_PATH"/enterprise/dev/test-license-generation-key.pem)
export SOURCEGRAPH_LICENSE_GENERATION_KEY

export PRECISE_CODE_INTEL_UPLOAD_AWS_ENDPOINT=http://localhost:9000
export DISABLE_CNCF=notonmybox

export EXECUTOR_FRONTEND_URL=http://localhost:3080
export EXECUTOR_FRONTEND_USERNAME=executor
export EXECUTOR_FRONTEND_PASSWORD=hunter2
export EXECUTOR_QUEUE_URL=http://localhost:3191
export EXECUTOR_USE_FIRECRACKER=false
export EXECUTOR_IMAGE_ARCHIVE_PATH=$HOME/.sourcegraph/images

export WATCH_ADDITIONAL_GO_DIRS="enterprise/cmd enterprise/dev enterprise/internal"
export ENTERPRISE_ONLY_COMMANDS=" precise-code-intel-worker executor-queue executor "
export ENTERPRISE_COMMANDS="frontend repo-updater ${ENTERPRISE_ONLY_COMMANDS}"
export ENTERPRISE=1
./dev/start.sh "$@"
