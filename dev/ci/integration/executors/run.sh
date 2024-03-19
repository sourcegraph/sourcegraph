#!/usr/bin/env bash

# This script runs the executors-e2e test suite against a candidate server image.

cd "$(dirname "${BASH_SOURCE[0]}")/../../../.."
root_dir=$(pwd)
set -ex

TMP_WORK_DIR=$(mktemp -d)
TMP_DIR="${TMP_WORK_DIR}/executor-tmp"
mkdir "${TMP_DIR}"
DATA="${TMP_WORK_DIR}/data"
mkdir "${DATA}"
mkdir "${DATA}/data"
mkdir "${DATA}/config"

# we get the ID / GID here that the container should map *it's ID/GID* for root, so that when it writes files as root to a mapped in volume
# the file permissions will have the correct ID/GID set so that the current running user still have permissions to alter the files
EXECUTOR_UID="$(id -u)"
EXECUTOR_GID="$(id -g)"
export EXECUTOR_UID
export EXECUTOR_GID

cleanup() {
  pushd "$root_dir"/dev/ci/integration/executors/ 1>/dev/null
  docker-compose logs >"${root_dir}/docker-compose.log"
  # We have to remove the directory here since the container creates files in that directory as root, and
  # we can't remove outside of the container
  docker-compose exec server /bin/sh -c "rm -rf /var/opt/sourcegraph/*"
  docker-compose down --volumes --timeout 30 # seconds
  docker volume rm executors-e2e || true
  popd 1>/dev/null
}
trap cleanup EXIT

export REGISTRY=${REGISTRY:-"us.gcr.io/sourcegraph-dev"}
export POSTGRES_IMAGE="${REGISTRY}/postgres-12-alpine:${CANDIDATE_VERSION}"
export SERVER_IMAGE="${REGISTRY}/server:${CANDIDATE_VERSION}"
export EXECUTOR_IMAGE="${REGISTRY}/executor:${CANDIDATE_VERSION}"
export EXECUTOR_FRONTEND_PASSWORD="hunter2hunter2hunter2"
export SOURCEGRAPH_LICENSE_GENERATION_KEY="${SOURCEGRAPH_LICENSE_GENERATION_KEY:-""}"
export TMP_DIR
export DATA
if [ -n "${DOCKER_GATEWAY_HOST}" ]; then
  DOCKER_HOST="tcp://${DOCKER_GATEWAY_HOST:-host.docker.internal}:2375"
  export DOCKER_HOST
fi
# Executor docker compose maps this explicitly because Non-aspect agents use Docker in Docker (DIND) and have this env var explicitly set.
#
# On Aspect this var is NOT set because we use "normal" docker (aka non DND) which uses a unix socket, but this var still needs to be mapped in
# so we explicitly set it here.
if [ -z "${DOCKER_HOST}" ]; then
  DOCKER_HOST="unix:///var/run/docker.sock"
  export DOCKER_HOST
fi

# Need to pull this image pre-execution as the docker executor doesn't have a
# credential to pull this image.
BATCHESHELPER_IMAGE="${REGISTRY}/batcheshelper:${CANDIDATE_VERSION}"
docker pull "${BATCHESHELPER_IMAGE}"

echo "--- :terminal: Start server with executor"
pushd "dev/ci/integration/executors" 1>/dev/null

# Temporary workaround, see https://github.com/sourcegraph/sourcegraph/issues/44816
envsubst >"${TMP_WORK_DIR}/site-config.json" <./tester/config/site-config.json
docker volume create executors-e2e
docker container create --name temp -v executors-e2e:/data busybox
docker cp "${TMP_WORK_DIR}/site-config.json" temp:/data
rm "${TMP_WORK_DIR}/site-config.json"
docker rm temp

docker-compose up -d

echo "--- :terminal: Wait for server to be up"
URL="http://localhost:7080"
timeout 120s bash -c "until curl --output /dev/null --silent --head --fail $URL; do
    echo Waiting 5s for $URL...
    sleep 5
done"

# ==========================

echo "--- :terminal: Run test.sh"
./test.sh
