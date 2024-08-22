#!/usr/bin/env bash

set -eou pipefail

export USER_DISKS_MOUNTS="/var/lib/extra,/var/lib/p1,/var/lib/p2"

# shellcheck source=/dev/null
source ./hack/test/e2e.sh

PROVISIONER=qemu
CLUSTER_NAME="e2e-${PROVISIONER}"

QEMU_FLAGS=()

case "${CI:-false}" in
  false)
    QEMU_FLAGS+=("--with-bootloader=false")
    ;;
  *)
    ;;
esac

case "${CUSTOM_CNI_URL:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--custom-cni-url=${CUSTOM_CNI_URL}")
    ;;
esac

case "${WITH_UEFI:-none}" in
  none)
    ;;
  *)
    QEMU_FLAGS+=("--with-uefi=${WITH_UEFI}")
    ;;
esac

case "${WITH_VIRTUAL_IP:-false}" in
  true)
    QEMU_FLAGS+=("--use-vip")
    ;;
esac

case "${WITH_CLUSTER_DISCOVERY:-true}" in
  false)
    QEMU_FLAGS+=("--with-cluster-discovery=false" "--kubeprism-port=0") # disable both KubePrism and cluster discovery
    ;;
esac

case "${WITH_KUBESPAN:-false}" in
  true)
    QEMU_FLAGS+=("--with-kubespan")
    ;;
esac

case "${WITH_CONTROL_PLANE_PORT:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--control-plane-port=${WITH_CONTROL_PLANE_PORT}")
    ;;
esac

case "${VIA_MAINTENANCE_MODE:-false}" in
  false)
    ;;
  *)
    # apply config via maintenance mode
    QEMU_FLAGS+=("--skip-injecting-config" "--with-apply-config")
    ;;
esac

case "${DISABLE_DHCP_HOSTNAME:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--disable-dhcp-hostname")
    ;;
esac

case "${WITH_NETWORK_CHAOS:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--with-network-chaos" "--with-network-packet-loss=0.01" "--with-network-latency=15ms" "--with-network-jitter=5ms")
    ;;
esac

case "${WITH_FIREWALL:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--with-firewall=${WITH_FIREWALL}")
    ;;
esac

case "${USE_DISK_IMAGE:-false}" in
  false)
    ;;
  *)
    zstd -d < _out/metal-amd64.raw.zst > _out/metal-amd64.raw
    QEMU_FLAGS+=("--disk-image-path=_out/metal-amd64.raw")
    ;;
esac

case "${WITH_DISK_ENCRYPTION:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--encrypt-ephemeral" "--encrypt-state" "--disk-encryption-key-types=kms")
    ;;
esac

case "${WITH_CONFIG_PATCH:-false}" in
  false)
    ;;
  *)
    [[ ! ${WITH_CONFIG_PATCH} =~ ^@ ]] && echo "WITH_CONFIG_PATCH variable should start with @" && exit 1

    for i in ${WITH_CONFIG_PATCH//:/ }; do
      QEMU_FLAGS+=("--config-patch=${i}")
    done
    ;;
esac

case "${WITH_CONFIG_PATCH_WORKER:-false}" in
  false)
    ;;
  *)
    [[ ! ${WITH_CONFIG_PATCH_WORKER} =~ ^@ ]] && echo "WITH_CONFIG_PATCH_WORKER variable should start with @" && exit 1

    for i in ${WITH_CONFIG_PATCH_WORKER//:/ }; do
      QEMU_FLAGS+=("--config-patch-worker=${i}")
    done
    ;;
esac

case "${WITH_SKIP_K8S_NODE_READINESS_CHECK:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--skip-k8s-node-readiness-check")
    ;;
esac

case "${WITH_CUSTOM_CNI:-none}" in
  false)
    ;;
  cilium)
    QEMU_FLAGS+=("--kubeprism-port=13336")
    ;;
esac

case "${WITH_TRUSTED_BOOT_ISO:-false}" in
  false)
    ;;
  *)
    INSTALLER_IMAGE=${INSTALLER_IMAGE}-amd64-secureboot
    QEMU_FLAGS+=("--iso-path=_out/metal-amd64-secureboot.iso" "--with-tpm2" "--encrypt-ephemeral" "--encrypt-state" "--disk-encryption-key-types=tpm")
    ;;
esac

case "${WITH_SIDEROLINK_AGENT:-false}" in
  false)
    ;;
  *)
    QEMU_FLAGS+=("--with-siderolink=${WITH_SIDEROLINK_AGENT}")
    ;;
esac

case "${WITH_APPARMOR_LSM_ENABLED:-false}" in
  false)
    ;;
  *)
    cat <<EOF > "${TMP}/kernel-security.patch"
machine:
  install:
    extraKernelArgs:
      - security=apparmor
EOF

    QEMU_FLAGS+=("--config-patch=@${TMP}/kernel-security.patch")
    ;;
esac

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner="${PROVISIONER}" \
    --name="${CLUSTER_NAME}" \
    --kubernetes-version="${KUBERNETES_VERSION}" \
    --controlplanes=3 \
    --workers="${QEMU_WORKERS:-1}" \
    --disk=15360 \
    --extra-disks="${QEMU_EXTRA_DISKS:-0}" \
    --extra-disks-size="${QEMU_EXTRA_DISKS_SIZE:-5120}" \
    --mtu=1430 \
    --memory=2048 \
    --memory-workers="${QEMU_MEMORY_WORKERS:-2048}" \
    --cpus="${QEMU_CPUS:-2}" \
    --cpus-workers="${QEMU_CPUS_WORKERS:-2}" \
    --cidr=172.20.1.0/24 \
    --user-disk=/var/lib/extra:350MB \
    --user-disk=/var/lib/p1:350MB:/var/lib/p2:350MB \
    --install-image="${INSTALLER_IMAGE}" \
    --with-init-node=false \
    --cni-bundle-url="${ARTIFACTS}/talosctl-cni-bundle-\${ARCH}.tar.gz" \
    "${REGISTRY_MIRROR_FLAGS[@]}" \
    "${QEMU_FLAGS[@]}"

  "${TALOSCTL}" config node 172.20.1.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}"
}

create_cluster

case "${WITH_CUSTOM_CNI:-none}" in
  cilium)
    install_and_run_cilium_cni_tests
    ;;
  *)
    ;;
esac

case "${TEST_MODE:-default}" in
  fast-conformance)
    run_kubernetes_conformance_test fast
    ;;
  *)
    get_kubeconfig
    run_talos_integration_test
    run_kubernetes_integration_test

    if [ "${WITH_TEST:-none}" != "none" ]; then
      "${WITH_TEST}"
    fi
    ;;
esac


destroy_cluster
