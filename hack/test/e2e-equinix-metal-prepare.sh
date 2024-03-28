#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

export AWS_DEFAULT_REGION="us-east-1"
export BUCKET_NAME="talos-ci-e2e"

export EQUINIX_METRO="dc"

TEMP_DIR=$(mktemp -d)

INSTALLER_IMAGE_NAME=$(cut -d ":" -f1 <<< "${INSTALLER_IMAGE}")
INSTALLER_IMAGE_TAG=$(cut -d ":" -f2 <<< "${INSTALLER_IMAGE}")

rm -rf "${ARTIFACTS}/v2"

function cleanup() {
    rm -rf "${TEMP_DIR}"
}

trap cleanup SIGINT EXIT

function generate_ipxe_script() {
  CONSOLE="console=ttyS1,115200n8"

  [[ "${1}" == "arm64" ]] && CONSOLE="console=ttyAMA0,115200"

  cat > "${ARTIFACTS}/ipxe-${1}" << EOF
#!ipxe

kernel https://${BUCKET_NAME}.s3.amazonaws.com/vmlinuz-${1} talos.platform=equinixMetal console=tty0 ${CONSOLE} init_on_alloc=1 slab_nomerge pti=on consoleblank=0 nvme_core.io_timeout=4294967295 printk.devkmsg=on ima_template=ima-ng ima_appraise=fix ima_hash=sha512
initrd https://${BUCKET_NAME}.s3.amazonaws.com/initramfs-${1}.xz
boot
EOF
}

function upload_artifact() {
    aws s3 cp --acl public-read "${ARTIFACTS}/${1}" "s3://${BUCKET_NAME}/${1}"
}

shamove() {
  SHA=$(sha256sum "${1}" | cut -d " " -f1)

  mv "${1}" "${ARTIFACTS}/v2/${INSTALLER_IMAGE_NAME}/${2}/sha256:${SHA}"
}

# adapted from https://github.com/jpetazzo/registrish/
function generate_oci() {
    crane pull --format=oci "${INSTALLER_IMAGE}" "${TEMP_DIR}/${INSTALLER_IMAGE_NAME}"

    mkdir -p "${ARTIFACTS}/v2/${INSTALLER_IMAGE_NAME}/manifests" "${ARTIFACTS}/v2/${INSTALLER_IMAGE_NAME}/blobs"

    find "${TEMP_DIR}/${INSTALLER_IMAGE_NAME}/blobs" -type f | while read -r FILE; do
        # gzip files are blobs
        if gzip -t "${FILE}"; then
            shamove "${FILE}" blobs
        else
            # json files with architecture are blobs
            if [[ $(jq 'select(.architecture != null)' "${FILE}") != "" ]]; then
                shamove "${FILE}" blobs

                continue
            fi

            # copying over the index file as tag
            [[ $(jq '.mediaType=="application/vnd.oci.image.index.v1+json"' "${FILE}") == "true" ]] && cp "${FILE}" "${ARTIFACTS}/v2/${INSTALLER_IMAGE_NAME}/manifests/${INSTALLER_IMAGE_TAG}"

            # anything else is other manifests referenced by the index
            shamove "${FILE}" manifests
        fi
done
}

# adapted from https://github.com/jpetazzo/registrish/
function upload_oci() {
    # remove any existing container image data
    aws s3 rm "s3://${BUCKET_NAME}/v2/" --recursive

    aws s3 sync "${ARTIFACTS}/v2/" \
        "s3://${BUCKET_NAME}/v2/" \
        --acl public-read \
        --exclude '*/manifests/*'

    find "${ARTIFACTS}/v2/" -path '*/manifests/*' -print0 | while IFS= read -r -d '' MANIFEST; do
        CONTENT_TYPE=$(jq -r .mediaType < "${MANIFEST}")

        if [ "$CONTENT_TYPE" = "null" ]; then
            CONTENT_TYPE="application/vnd.docker.distribution.manifest.v1+prettyjws"
        fi

        aws s3 cp "${MANIFEST}" \
            "s3://${BUCKET_NAME}/${MANIFEST/${ARTIFACTS}\//}" \
            --acl public-read \
            --content-type "${CONTENT_TYPE}" \
            --metadata-directive REPLACE
    done
}

# generate ipxe script for both amd64 and arm64
generate_ipxe_script "amd64"
generate_ipxe_script "arm64"

upload_artifact "ipxe-amd64"
upload_artifact "ipxe-arm64"

upload_artifact vmlinuz-amd64
upload_artifact initramfs-amd64.xz
upload_artifact vmlinuz-arm64
upload_artifact initramfs-arm64.xz

generate_oci
upload_oci

mkdir -p "${ARTIFACTS}/e2e-equinix-metal-generated"

NAME_PREFIX="talos-e2e-${SHA}-equinix-metal"

jq --null-input \
  --arg CLUSTER_NAME "${NAME_PREFIX}" \
  --arg EM_API_TOKEN "${EM_API_TOKEN}" \
  --arg EM_PROJECT_ID "${EM_PROJECT_ID}" \
  --arg TALOS_VERSION_CONTRACT "${TALOS_VERSION}" \
  --arg KUBERNETES_VERSION "${KUBERNETES_VERSION}" \
  --arg EM_REGION "${EQUINIX_METRO}" \
  --arg INSTALL_IMAGE "${BUCKET_NAME}.s3.amazonaws.com/${INSTALLER_IMAGE_NAME}:${INSTALLER_IMAGE_TAG}" \
  --arg IPXE_SCRIPT_URL_AMD64 "https://${BUCKET_NAME}.s3.amazonaws.com/ipxe-amd64" \
  --arg IPXE_SCRIPT_URL_ARM64 "https://${BUCKET_NAME}.s3.amazonaws.com/ipxe-arm64" \
    '{
        cluster_name: $CLUSTER_NAME,
        em_api_token: $EM_API_TOKEN,
        em_project_id: $EM_PROJECT_ID,
        talos_version_contract: $TALOS_VERSION_CONTRACT,
        kubernetes_version: $KUBERNETES_VERSION,
        em_region: $EM_REGION,
        ipxe_script_url_amd64: $IPXE_SCRIPT_URL_AMD64,
        ipxe_script_url_arm64: $IPXE_SCRIPT_URL_ARM64,
        install_image: $INSTALL_IMAGE
    }' \
  | jq -f hack/test/tfvars/equinix-metal.jq > "${ARTIFACTS}/e2e-equinix-metal-generated/vars.json"

cp hack/test/tfvars/*.yaml "${ARTIFACTS}/e2e-equinix-metal-generated"
