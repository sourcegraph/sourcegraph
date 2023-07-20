#!/usr/bin/env bash

set -e
export SONAR_SCANNER_OPTS="-server"

export SONAR_TOKEN="${SONAR_TOKEN}"

if [ "$SONAR_TOKEN" = "" ];
then
  echo "Please set the SONAR_TOKEN environment variable"
  exit 1
fi

set -x

echo "--- :arrow_down: verifying Sonarcloud binary"
echo ""
/usr/local/bin/sonar-scanner --version
echo ""

echo "--- :lock: running Sonarcloud scan"
echo ""
/usr/local/bin/sonar-scanner \
  -Dsonar.organization=test-shiva-surya \
  -Dsonar.projectKey=test-shiva-surya_sourcegraph \
  -Dsonar.sources=. \
  -Dsonar.host.url=https://sonarcloud.io \
  -Dsonar.sourceEncoding=UTF-8 \
  -Dsonar.pullrequest.key=$BUILDKITE_PULL_REQUEST \
  -Dsonar.pullrequest.branch=$BUILDKITE_BRANCH \
  -Dsonar.pullrequest.base=$BUILDKITE_PULL_REQUEST_BASE_BRANCH
