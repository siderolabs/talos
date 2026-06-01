#!/usr/bin/env bash

set -eoux pipefail

source ./hack/test/e2e.sh

PROVISIONER=qemu
CLUSTER_NAME=e2e-iso

NODE="172.20.2.2"

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner="${PROVISIONER}" \
    --name="${CLUSTER_NAME}" \
    --kubernetes-version=${KUBERNETES_VERSION} \
    --iso-path=${ARTIFACTS}/metal-amd64.iso \
    --controlplanes=1 \
    --workers=0 \
    --mtu=1430 \
    --memory=2048 \
    --cpus=2.0 \
    --cidr=172.20.2.0/24 \
    --with-apply-config \
    --install-image="${INSTALLER_IMAGE}" \
    --cni-bundle-url=${ARTIFACTS}/talosctl-cni-bundle-'${ARCH}'.tar.gz \
    "${REGISTRY_MIRROR_FLAGS[@]}"

  "${TALOSCTL}" config node "${NODE}"
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy \
    --name "${CLUSTER_NAME}" \
    --provisioner "${PROVISIONER}" \
    --save-cluster-logs-archive-path="/tmp/logs-${CLUSTER_NAME}.tar.gz" \
    --save-support-archive-path="/tmp/support-${CLUSTER_NAME}.zip"
}

trap destroy_cluster SIGINT EXIT

create_cluster
sleep 5
