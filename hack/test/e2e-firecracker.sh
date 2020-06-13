#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=firecracker
CLUSTER_NAME=e2e-${PROVISIONER}

case "${REGISTRY:-false}" in
  registry.ci.svc:5000)
    REGISTRY_ADDR=`python -c "import socket; print socket.gethostbyname('registry.ci.svc')"`
    FIRECRACKER_FLAGS="--registry-mirror ${REGISTRY}=http://${REGISTRY_ADDR}:5000 --with-bootloader-emulation"
    INSTALLER_TAG="${TAG}"
    ;;
  *)
    FIRECRACKER_FLAGS=
    INSTALLER_TAG="latest"
    ;;
esac

function create_cluster {
  "${TALOSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --masters=3 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 2.0 \
    --cidr 172.20.0.0/24 \
    --install-image ${REGISTRY:-docker.io}/autonomy/installer:${INSTALLER_TAG} \
    --with-init-node=false \
    ${FIRECRACKER_FLAGS}
}

create_cluster
get_kubeconfig
run_talos_integration_test
run_kubernetes_integration_test
