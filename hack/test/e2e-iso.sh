#!/bin/bash

set -eoux pipefail

source ./hack/test/e2e.sh

PROVISIONER=qemu
CLUSTER_NAME=e2e-iso

NODE="172.20.2.2"

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --iso-path=${ARTIFACTS}/talos-amd64.iso \
    --skip-injecting-config \
    --wait=false \
    --with-init-node \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --masters=1 \
    --workers=0 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 2.0 \
    --cidr 172.20.2.0/24 \
    --install-image ${REGISTRY:-ghcr.io}/talos-systems/installer:${TAG} \
    --cni-bundle-url ${ARTIFACTS}/talosctl-cni-bundle-'${ARCH}'.tar.gz \
    ${REGISTRY_MIRROR_FLAGS}

  "${TALOSCTL}" config node "${NODE}"
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}"
}

create_cluster
timeout -v --preserve-status 1m bash -c "until ${TALOSCTL} apply-config -n ${NODE} --insecure -f controlplane.yaml; do sleep 5; done"
sleep 5
timeout -v --preserve-status 2m  bash -c "until nc -z ${NODE} 50000; do sleep 5; done"
${TALOSCTL} bootstrap -n "${NODE}"
${TALOSCTL} health --wait-timeout=10m0s -n "${NODE}" --control-plane-nodes="${NODE}"
destroy_cluster
