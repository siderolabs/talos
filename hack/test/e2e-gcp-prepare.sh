#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

REGION="us-central1"
ZONE="us-central1-a"
PROJECT_ID="${GOOGLE_PROJECT_ID}"

function cloud_image_upload() {
  CLOUD_IMAGES_EXTRA_ARGS=("--target-clouds=gcp" "--architectures=amd64")

  make cloud-images CLOUD_IMAGES_EXTRA_ARGS="${CLOUD_IMAGES_EXTRA_ARGS[*]}"
}

function get_image() {
  jq -r ".[] | select(.cloud == \"gcp\") | select (.arch == \"amd64\") | .id" "${ARTIFACTS}/cloud-images.json"
}

cloud_image_upload

GCP_IMAGE=$(get_image)

mkdir -p "${ARTIFACTS}/e2e-gcp-generated"

NAME_PREFIX="talos-e2e-${SHA}-gcp"

jq --null-input \
  --arg REGION "${REGION}" \
  --arg ZONE "${ZONE}" \
  --arg PROJECT_ID "${PROJECT_ID}" \
  --arg GCP_IMAGE "${GCP_IMAGE}" \
  --arg CLUSTER_NAME "${NAME_PREFIX}" \
  --arg TALOS_VERSION_CONTRACT "${TALOS_VERSION}" \
  --arg KUBERNETES_VERSION "${KUBERNETES_VERSION}" \
    '{
        region: $REGION,
        zone: $ZONE,
        project_id: $PROJECT_ID,
        gcp_image: $GCP_IMAGE,
        cluster_name: $CLUSTER_NAME,
        talos_version_contract: $TALOS_VERSION_CONTRACT,
        kubernetes_version: $KUBERNETES_VERSION
    }' \
  | jq -f hack/test/tfvars/gcp.jq > "${ARTIFACTS}/e2e-gcp-generated/vars.json"

cp hack/test/tfvars/*.yaml "${ARTIFACTS}/e2e-gcp-generated"
