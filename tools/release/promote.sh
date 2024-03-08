#!/usr/bin/env bash

set -eu

if [ "$VERSION" = "" ]; then
  echo "Need \$VERSION to be set to promote images"
  exit 1
fi

INTERNAL_REGISTRY="us-central1-docker.pkg.dev/sourcegraph-ci/rfc795-internal"
PUBLIC_REGISTRY="us-central1-docker.pkg.dev/sourcegraph-ci/rfc795-public"

images=(
"alpine-3.14"
"batcheshelper"
"blobstore"
"bundled-executor"
"cadvisor"
"codeinsights-db"
"codeintel-db"
"cody-gateway"
"dind"
"embeddings"
"executor"
"executor-kubernetes"
"executor-vm"
"frontend"
"gitserver"
"grafana"
"indexed-searcher"
"initcontainer"
"jaeger-agent"
"jaeger-all-in-one"
"loadtest"
"migrator"
"migrator-airgapped"
"msp-example"
"node-exporter"
"opentelemetry-collector"
"pings"
"postgres-12-alpine"
"postgres_exporter"
"precise-code-intel-worker"
"prometheus"
# "prometheus-gcp" # TODO check about this one
"qdrant"
"redis-cache"
"redis-store"
"redis_exporter"
"repo-updater"
"scip-ctags"
"search-indexer"
"searcher"
"server"
"sg"
"symbols"
"syntax-highlighter"
"telemetry-gateway"
"worker")

for name in "${images[@]}"; do
  echo "--- Copying ${name} from private registry to public registry"
  docker pull "${INTERNAL_REGISTRY}/${name}:${VERSION}"
  docker tag "${INTERNAL_REGISTRY}/${name}:${VERSION}" "${PUBLIC_REGISTRY}/${name}:${VERSION}"
  docker push "${PUBLIC_REGISTRY}/${name}:${VERSION}"
done
