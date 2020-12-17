#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=firecracker
CLUSTER_NAME=e2e-${PROVISIONER}

case "${CI:-false}" in
  true)
    REGISTRY="127.0.0.1:5000"
    REGISTRY_ADDR=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' registry`
    FIRECRACKER_FLAGS="--registry-mirror ${REGISTRY}=http://${REGISTRY_ADDR}:5000"
    INSTALLER_TAG="${TAG}"
    docker tag ${INSTALLER_IMAGE} 127.0.0.1:5000/talos-installer/installer:"${TAG}"
    docker push 127.0.0.1:5000/talos-systems/installer:"${TAG}"
    ;;
  *)
    FIRECRACKER_FLAGS="--with-bootloader=false"
    INSTALLER_TAG="latest"
    ;;
esac

case "${CUSTOM_CNI_URL:-false}" in
  false)
    CUSTOM_CNI_FLAG=
    ;;
  *)
    CUSTOM_CNI_FLAG="--custom-cni-url=${CUSTOM_CNI_URL}"
    ;;
esac

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --masters=3 \
    --mtu 1450 \
    --memory 2048 \
    --cpus 2.0 \
    --cidr 172.20.0.0/24 \
    --install-image ${REGISTRY:-ghcr.io}/talos-systems/installer:${INSTALLER_TAG} \
    --with-init-node=false \
    --crashdump \
    ${REGISTRY_MIRROR_FLAGS} \
    ${FIRECRACKER_FLAGS} \
    ${CUSTOM_CNI_FLAG}

  "${TALOSCTL}" config node 172.20.0.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}"
}

create_cluster
get_kubeconfig
run_talos_integration_test
run_kubernetes_integration_test
destroy_cluster
