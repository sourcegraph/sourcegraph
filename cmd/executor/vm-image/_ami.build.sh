#!/usr/bin/env bash

set -eu
## Setting up inputs/data
gcloud="$(pwd)/$1" # used in workdir folder, so need an absolute path
packer="$(pwd)/$2"
srccli="$3"
executor="$4"
executor_image="$5"

base="cmd/executor/vm-image/"

## Setting up the folder we're going to use with packer
mkdir -p workdir
workdir_abs="$(pwd)/workdir"
trap 'rm -Rf "$workdir_abs"' EXIT

cp "${base}/executor.pkr.hcl" workdir/
cp "${base}/aws_regions.json" workdir/
cp "${base}/install.sh" workdir/
cp "$executor" workdir/

# Copy src-cli, see //dev/tools:src-cli
cp "$srccli" workdir/

# Load the docker image, whose tag is going to be candidate,
# but we need to retag this with the version.
"$executor_image" # this is equivalent to a docker load --input=tarball and the base tag comes from the rule that builds it.

docker tag executor-vm:candidate "sourcegraph/executor-vm:$VERSION"
docker save --output workdir/executor-vm.tar "sourcegraph/executor-vm:$VERSION"

GCP_PROJECT="aspect-dev"
"$gcloud" secrets versions access latest --secret=e2e-builder-sa-key --quiet --project="$GCP_PROJECT" >"workdir/builder-sa-key.json"

export PKR_VAR_name
PKR_VAR_name="${IMAGE_FAMILY}-${BUILDKITE_BUILD_NUMBER}"
export PKR_VAR_image_family="${IMAGE_FAMILY}"
export PKR_VAR_tagged_release="${EXECUTOR_IS_TAGGED_RELEASE}"
export PKR_VAR_version="${VERSION}"
export PKR_VAR_src_cli_version=${SRC_CLI_VERSION}
export PKR_VAR_aws_access_key=${AWS_EXECUTOR_AMI_ACCESS_KEY}
export PKR_VAR_aws_secret_key=${AWS_EXECUTOR_AMI_SECRET_KEY}
# This should prevent some occurrences of Failed waiting for AMI failures:
# https://austincloud.guru/2020/05/14/long-running-packer-builds-failing/
export PKR_VAR_aws_max_attempts=480
export PKR_VAR_aws_poll_delay_seconds=5

cd workdir

export PKR_VAR_aws_regions
if [ "${EXECUTOR_IS_TAGGED_RELEASE}" = "true" ]; then
  PKR_VAR_aws_regions="$(jq -r '.' <aws_regions.json)"
else
  PKR_VAR_aws_regions='["us-west-2"]'
fi

"$packer" init executor.pkr.hcl
"$packer" build -force executor.pkr.hcl
