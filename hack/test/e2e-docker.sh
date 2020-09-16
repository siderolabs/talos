#!/bin/bash

set -eou pipefail

export PROVISIONER=docker
export CLUSTER_NAME=e2e-${PROVISIONER}

source ./hack/test/e2e.sh


function create_cluster {
  make_tmp

  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --image "${IMAGE}" \
    --masters=3 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 2.0 \
    --with-init-node=false \
    --docker-host-ip=127.0.0.1 \
    --endpoint=127.0.0.1 \
    ${REGISTRY_MIRROR_FLAGS} \
    --crashdump

  "${TALOSCTL}" config node 10.5.0.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}"
   clean_tmp
}

destroy_cluster
create_cluster
get_kubeconfig
${KUBECTL} config set-cluster e2e-docker --server https://10.5.0.2:6443
run_talos_integration_test_docker
run_kubernetes_integration_test
