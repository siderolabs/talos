#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=docker
CLUSTER_NAME=e2e-${PROVISIONER}

function create_cluster {
  "${TALOSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --image "${IMAGE}" \
    --masters=3 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 4.0 \
    --with-init-node=false \
    --crashdump

  "${TALOSCTL}" config node 10.5.0.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}"
}

create_cluster
get_kubeconfig
${KUBECTL} config set-cluster e2e-docker --server https://10.5.0.2:6443
run_talos_integration_test_docker
run_kubernetes_integration_test
destroy_cluster
