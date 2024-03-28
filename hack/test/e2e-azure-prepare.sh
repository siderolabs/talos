#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

REGION="eastus"

function cloud_image_upload() {
  CLOUD_IMAGES_EXTRA_ARGS=("--target-clouds=azure" "--architectures=amd64" "--azure-regions=${REGION}")

  make cloud-images CLOUD_IMAGES_EXTRA_ARGS="${CLOUD_IMAGES_EXTRA_ARGS[*]}"
}

function get_os_id() {
  jq -r ".[] | select(.cloud == \"azure\") | select(.region == \"${REGION}\") | select (.arch == \"amd64\") | .id" "${ARTIFACTS}/cloud-images.json"
}

cloud_image_upload

VM_OS_ID=$(get_os_id)

mkdir -p "${ARTIFACTS}/e2e-azure-generated"

NAME_PREFIX="talos-e2e-${SHA}-azure"

jq --null-input \
  --arg VM_OS_ID "${VM_OS_ID}" \
  --arg CLUSTER_NAME "${NAME_PREFIX}" \
  --arg TALOS_VERSION_CONTRACT "${TALOS_VERSION}" \
  --arg KUBERNETES_VERSION "${KUBERNETES_VERSION}" \
    '{
        vm_os_id: $VM_OS_ID,
        cluster_name: $CLUSTER_NAME,
        talos_version_contract: $TALOS_VERSION_CONTRACT,
        kubernetes_version: $KUBERNETES_VERSION
    }' \
  | jq -f hack/test/tfvars/azure.jq > "${ARTIFACTS}/e2e-azure-generated/vars.json"

cp hack/test/tfvars/*.yaml "${ARTIFACTS}/e2e-azure-generated"
