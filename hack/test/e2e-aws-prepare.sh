#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

REGION="us-east-1"

function cloud_image_upload() {
  CLOUD_IMAGES_EXTRA_ARGS=("--name-prefix=${1}" "--target-clouds=aws" "--architectures=amd64" "--aws-regions=${REGION}")

  make cloud-images CLOUD_IMAGES_EXTRA_ARGS="${CLOUD_IMAGES_EXTRA_ARGS[*]}"
}

function get_ami_id() {
  jq -r ".[] | select(.cloud == \"aws\") | select(.region == \"${REGION}\") | select (.arch == \"amd64\") | .id" "${ARTIFACTS}/cloud-images.json"
}

function cloud_image_upload_with_extensions() {
  case "${1}" in
    nvidia-oss)
      EXTENSIONS=$(jq -R < "${EXTENSIONS_METADATA_FILE}" | jq -rs 'map(select(. | (contains("nvidia") or contains("zfs")) and (contains("nvidia-fabricmanager") or contains("nonfree-kmod-nvidia") | not))) | .[] |= "--system-extension-image=" + . | join(" ")')
      ;;
    nvidia-oss-fabricmanager)
      EXTENSIONS=$(jq -R < "${EXTENSIONS_METADATA_FILE}" | jq -rs 'map(select(. | contains("nvidia") and (contains("nonfree-kmod-nvidia") | not))) | .[] |= "--system-extension-image=" + . | join(" ")')
      ;;
    nvidia-nonfree)
      EXTENSIONS=$(jq -R < "${EXTENSIONS_METADATA_FILE}" | jq -rs 'map(select(. | contains("nvidia") and (contains("nvidia-fabricmanager") or contains("nvidia-open-gpu-kernel-modules") | not))) | .[] |= "--system-extension-image=" + . | join(" ")')
      ;;
    nvidia-nonfree-fabricmanager)
      EXTENSIONS=$(jq -R < "${EXTENSIONS_METADATA_FILE}" | jq -rs 'map(select(. | contains("nvidia") and (contains("nvidia-open-gpu-kernel-modules") | not))) | .[] |= "--system-extension-image=" + . | join(" ")')
      ;;
    *)
      ;;
  esac

  make image-aws IMAGER_ARGS="${EXTENSIONS}" PLATFORM=linux/amd64
  cloud_image_upload "talos-e2e-${1}"
}

cloud_image_upload "talos-e2e"

AMI_ID=$(get_ami_id)

WORKER_GROUP=
NVIDIA_AMI_ID=

case "${E2E_AWS_TARGET:-default}" in
  default)
    ;;
  *)
    WORKER_GROUP="nvidia"
    cloud_image_upload_with_extensions "${E2E_AWS_TARGET}"
    NVIDIA_AMI_ID=$(get_ami_id)
    # cloud_image_upload_with_extensions "${E2E_AWS_TARGET}-fabricmanager"
    # NVIDIA_FM_AMI_ID=$(get_ami_id)
    ;;
esac

mkdir -p "${ARTIFACTS}/e2e-aws-generated"

NAME_PREFIX="talos-e2e-${SHA}-aws-${E2E_AWS_TARGET}"

jq --null-input \
  --arg WORKER_GROUP "${WORKER_GROUP}" \
  --arg AMI_ID "${AMI_ID}" \
  --arg NVIDIA_AMI_ID "${NVIDIA_AMI_ID}" \
  --arg CLUSTER_NAME "${NAME_PREFIX}" \
  --arg TALOS_VERSION_CONTRACT "${TALOS_VERSION}" \
  --arg KUBERNETES_VERSION "${KUBERNETES_VERSION}" \
    '{
        worker_group: $WORKER_GROUP,
        ami_id: $AMI_ID,
        nvidia_ami_id: $NVIDIA_AMI_ID,
        cluster_name: $CLUSTER_NAME,
        talos_version_contract: $TALOS_VERSION_CONTRACT,
        kubernetes_version: $KUBERNETES_VERSION
    }' \
  | jq -f hack/test/tfvars/aws.jq > "${ARTIFACTS}/e2e-aws-generated/vars.json"

cp hack/test/tfvars/*.yaml "${ARTIFACTS}/e2e-aws-generated"
