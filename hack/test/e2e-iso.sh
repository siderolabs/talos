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
    --iso-path=${ARTIFACTS}/talos-amd64.iso \
    --masters=1 \
    --workers=0 \
    --mtu=1450 \
    --memory=2048 \
    --cpus=2.0 \
    --cidr=172.20.2.0/24 \
    --with-apply-config \
    --install-image=${REGISTRY:-ghcr.io}/talos-systems/installer:${TAG} \
    --cni-bundle-url=${ARTIFACTS}/talosctl-cni-bundle-'${ARCH}'.tar.gz \
    ${REGISTRY_MIRROR_FLAGS}

  "${TALOSCTL}" config node "${NODE}"
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}"
}

create_cluster
sleep 5
destroy_cluster
