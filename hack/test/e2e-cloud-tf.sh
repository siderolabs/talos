#!/usr/bin/env bash

set -eou pipefail

# This script is used to run the end-to-end tests on a cloud provider using Terraform.

BUCKET_NAME="talos-ci-e2e"
TF_DIR="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}"

cp "${TF_SCRIPT_DIR}/hack/backend-aws.tf" "${TF_DIR}/backend.tf"

cp "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-generated"/* "${TF_DIR}"

CLUSTER_NAME=$(jq -e -r '.cluster_name' "${TF_DIR}/vars.json")
BACKEND_CONFIG_KEY="cloud-tf/${CLUSTER_NAME}-terraform.tfstate"

terraform -chdir="${TF_DIR}" \
    init \
    -backend-config="bucket=${BUCKET_NAME}" \
    -backend-config="key=${BACKEND_CONFIG_KEY}"

case "${TF_E2E_ACTION}" in
    "apply")
        terraform -chdir="${TF_DIR}" \
            apply \
            -auto-approve \
            -var-file="vars.json"

        terraform -chdir="${TF_DIR}" \
            output \
            -raw \
            talosconfig > "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-talosconfig"

        terraform -chdir="${TF_DIR}" \
            output \
            -raw \
            kubeconfig > "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-kubeconfig"
        ;;
    "destroy")
        terraform -chdir="${TF_DIR}" \
            apply \
            -destroy \
            -auto-approve \
            -var-file="vars.json" \
            -refresh="${TF_E2E_REFRESH_ON_DESTROY:-true}"

        aws s3api delete-object --bucket "${BUCKET_NAME}" --key "${BACKEND_CONFIG_KEY}"
        ;;
    *)
        echo "Unsupported action: ${TF_E2E_ACTION}"
        exit 1
        ;;
esac
