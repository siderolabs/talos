#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

REGION="us-east-1"

AMI_ID=$(jq -r ".[] | select(.region == \"${REGION}\") | select (.arch == \"amd64\") | .id" "${ARTIFACTS}/cloud-images.json")

mkdir -p "${ARTIFACTS}/e2e-aws-generated"

NAME_PREFIX="talos-e2e-${SHA}-aws"

jq --null-input --arg AMI_ID "${AMI_ID}" --arg CLUSTER_NAME "${NAME_PREFIX}" --arg KUBERNETES_VERSION "${KUBERNETES_VERSION}" '{ami_id: $AMI_ID, cluster_name: $CLUSTER_NAME, kubernetes_version: $KUBERNETES_VERSION}' \
  | jq -f hack/test/tfvars/aws.jq > "${ARTIFACTS}/e2e-aws-generated/vars.json"
