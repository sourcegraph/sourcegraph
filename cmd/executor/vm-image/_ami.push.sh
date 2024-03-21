#!/usr/bin/env bash

set -eu

## Setting up inputs/tools
gcloud="$1"

## ---

# Fail if we don't have the awscli binary available.
if ! command -v aws; then
  echo "aws-cli is not available, aborting."
  exit 1
fi

# Fail if we don't have jq binary available
# TODO I think we can get this from aspect stuff.
if ! command -v jq; then
  echo "jq is not available, aborting."
  exit 1
fi

export AWS_ACCESS_KEY_ID="${AWS_EXECUTOR_AMI_ACCESS_KEY}"
export AWS_SECRET_ACCESS_KEY="${AWS_EXECUTOR_AMI_SECRET_KEY}"

# Point to GCP boot disk image/AMI built by //cmd/executor/vm-image:ami.build
NAME="${IMAGE_FAMILY}"
if [ "${RELEASE_INTERNAL:-}" != "true" ]; then
  NAME="${NAME}-${BUILDKITE_BUILD_NUMBER}"
fi

GOOGLE_IMAGE_NAME="${NAME}"

# Mark GCP boot disk as released and make it usable outside of Sourcegraph.
echo "Publishing GCP compute image"
"$gcloud" compute images add-iam-policy-binding --project=sourcegraph-ci "${GOOGLE_IMAGE_NAME}" --member='allAuthenticatedUsers' --role='roles/compute.imageUser'
echo "Made GCP compute image public"
"$gcloud" compute images update --project=sourcegraph-ci "${GOOGLE_IMAGE_NAME}" --family="${IMAGE_FAMILY}"
echo "Added GCP compute image to image family ${IMAGE_FAMILY}"

# Set the AMIs to public.
if [ "${EXECUTOR_IS_TAGGED_RELEASE}" = "true" ]; then
  REGIONS=$(jq -r '.[]' <cmd/executor/vm-image/aws_regions.json)
  for region in $REGIONS; do
    echo "Getting AMI ID in region ${region}"
    AWS_AMI_ID=$(aws ec2 --region="${region}" describe-images --filter "Name=name,Values=${NAME}" --query 'Images[*].[ImageId]' --output text)
    echo "Found AMI! ID: ${AWS_AMI_ID}"
    # Make executor AMI usable outside of Sourcegraph.
    aws ec2 --region="${region}" modify-image-attribute --image-id "${AWS_AMI_ID}" --launch-permission "Add=[{Group=all}]"
    echo "Published AMI ${AWS_AMI_ID}"
  done
else
  echo "Getting AMI ID in region us-west-2"
  AWS_AMI_ID=$(aws ec2 --region="us-west-2" describe-images --filter "Name=name,Values=${NAME}" --query 'Images[*].[ImageId]' --output text)
  echo "Found AMI! ID: ${AWS_AMI_ID}"
  # Make executor AMI usable outside of Sourcegraph.
  aws ec2 --region="us-west-2" modify-image-attribute --image-id "${AWS_AMI_ID}" --launch-permission "Add=[{Group=all}]"
  echo "Published AMI ${AWS_AMI_ID}"
fi
