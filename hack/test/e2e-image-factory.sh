#!/usr/bin/env bash

set -eou pipefail

# shellcheck source=/dev/null
source ./hack/test/e2e.sh

PROVISIONER=qemu
CLUSTER_NAME="e2e-${PROVISIONER}"
LOG_ARCHIVE_SUFFIX="${GITHUB_STEP_NAME:-e2e-${PROVISIONER}}"

FACTORY_HOSTNAME=${FACTORY_HOSTNAME:-factory.talos.dev}
PXE_FACTORY_HOSTNAME=${PXE_FACTORY_HOSTNAME:-pxe.factory.talos.dev}
FACTORY_SCHEME=${FACTORY_SCHEME:-https}
INSTALLER_IMAGE_NAME=${INSTALLER_IMAGE_NAME:-installer}

case "${FACTORY_BOOT_METHOD:-iso}" in
  iso)
    QEMU_FLAGS+=("--iso-path=${FACTORY_SCHEME}://${FACTORY_HOSTNAME}/image/${FACTORY_SCHEMATIC}/${FACTORY_VERSION}/metal-amd64.iso")
    ;;
  disk-image)
    QEMU_FLAGS+=("--disk-image-path=${FACTORY_SCHEME}://${FACTORY_HOSTNAME}/image/${FACTORY_SCHEMATIC}/${FACTORY_VERSION}/metal-amd64.raw.xz")
    ;;
  ipxe)
    QEMU_FLAGS+=("--ipxe-boot-script=${FACTORY_SCHEME}://${PXE_FACTORY_HOSTNAME}/pxe/${FACTORY_SCHEMATIC}/${FACTORY_VERSION}/metal-amd64")
    ;;
  secureboot-iso)
    QEMU_FLAGS+=("--iso-path=${FACTORY_SCHEME}://${FACTORY_HOSTNAME}/image/${FACTORY_SCHEMATIC}/${FACTORY_VERSION}/metal-amd64-secureboot.iso" "--with-tpm2" "--encrypt-ephemeral" "--encrypt-state" "--disk-encryption-key-types=tpm")
    INSTALLER_IMAGE_NAME=installer-secureboot
    ;;
  *)
    echo "unknown factory boot method: ${FACTORY_BOOT_METHOD}"
    exit 1
    ;;
esac

function assert_secureboot {
  if [[ "${FACTORY_BOOT_METHOD:-iso}" != "secureboot-iso" ]]; then
    return
  fi

  ${TALOSCTL} get securitystate -o json
  ${TALOSCTL} get securitystate -o json | jq -e '.spec.secureBoot == true'
}

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner="${PROVISIONER}" \
    --name="${CLUSTER_NAME}" \
    --kubernetes-version="${KUBERNETES_VERSION}" \
    --controlplanes=3 \
    --workers="${QEMU_WORKERS:-1}" \
    --disk=15360 \
    --mtu=1430 \
    --memory=2048 \
    --memory-workers="${QEMU_MEMORY_WORKERS:-2048}" \
    --cpus="${QEMU_CPUS:-2}" \
    --cpus-workers="${QEMU_CPUS_WORKERS:-2}" \
    --cidr=172.20.1.0/24 \
    --cni-bundle-url="${ARTIFACTS}/talosctl-cni-bundle-\${ARCH}.tar.gz" \
    --skip-injecting-config \
    --with-apply-config \
    --talos-version="${FACTORY_VERSION}" \
    --install-image="${FACTORY_HOSTNAME}/${INSTALLER_IMAGE_NAME}/${FACTORY_SCHEMATIC}:${FACTORY_VERSION}" \
    "${REGISTRY_MIRROR_FLAGS[@]}" \
    "${QEMU_FLAGS[@]}"

    ${TALOSCTL} config node 172.20.1.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy \
    --name "${CLUSTER_NAME}" \
    --provisioner "${PROVISIONER}" \
    --save-cluster-logs-archive-path="/tmp/logs-${LOG_ARCHIVE_SUFFIX}.tar.gz" \
    --save-support-archive-path="/tmp/support-${LOG_ARCHIVE_SUFFIX}.zip"
}

trap destroy_cluster SIGINT EXIT

create_cluster

${TALOSCTL} health --run-e2e
${TALOSCTL} version | grep "${FACTORY_VERSION}"
${TALOSCTL} get extensions | grep "${FACTORY_SCHEMATIC}"
assert_secureboot

if [[ "${FACTORY_UPGRADE:-false}" == "true" ]]; then
    ${TALOSCTL} upgrade -i "${FACTORY_HOSTNAME}/${INSTALLER_IMAGE_NAME}/${FACTORY_UPGRADE_SCHEMATIC:-$FACTORY_SCHEMATIC}:${FACTORY_UPGRADE_VERSION:-$FACTORY_VERSION}"
    ${TALOSCTL} version | grep "${FACTORY_UPGRADE_VERSION:-$FACTORY_VERSION}"
    ${TALOSCTL} get extensions | grep "${FACTORY_UPGRADE_SCHEMATIC:-$FACTORY_SCHEMATIC}"
    assert_secureboot
fi
