#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

# This script is used to run the end-to-end tests on a cloud provider using Terraform.

if [[ "${CI}" != "true" ]]; then
    echo "This script is only meant to be run in CI."
    exit 1
fi

BUCKET_NAME="talos-ci-e2e"

cp "${TF_SCRIPT_DIR}/hack/backend-aws.tf" "${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}/backend.tf"

cp "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-generated"/* "${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}"

terraform -chdir="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}" \
    init \
    -backend-config="bucket=${BUCKET_NAME}" \
    -backend-config="key=cloud-tf/${TF_E2E_TEST_TYPE}-${GITHUB_SHA}-${GITHUB_RUN_NUMBER}-terraform.tfstate"

case "${TF_E2E_ACTION}" in
    "apply")
        terraform -chdir="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}" \
            apply \
            -auto-approve \
            -var-file="vars.json"

        terraform -chdir="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}" \
            output \
            -raw \
            talosconfig > "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-talosconfig"

        terraform -chdir="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}" \
            output \
            -raw \
            kubeconfig > "${ARTIFACTS}/e2e-${TF_E2E_TEST_TYPE}-kubeconfig"
        ;;
    "destroy")
        terraform -chdir="${TF_SCRIPT_DIR}/examples/terraform/${TF_E2E_TEST_TYPE}" \
            apply \
            -destroy \
            -auto-approve \
            -var-file="vars.json" \
            -refresh="${TF_E2E_REFRESH_ON_DESTROY:-true}"

        aws s3api delete-object --bucket "${BUCKET_NAME}" --key "cloud-tf/${TF_E2E_TEST_TYPE}-${GITHUB_SHA}-terraform.tfstate"
        ;;
    *)
        echo "Unsupported action: ${TF_E2E_ACTION}"
        exit 1
        ;;
esac
