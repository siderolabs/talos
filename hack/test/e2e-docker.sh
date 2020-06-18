#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

case "${CI:-false}" in
  true)
    ENDPOINT="docker"
    ;;
  *)
    ENDPOINT="127.0.0.1"
    ;;
esac

PROVISIONER=docker
CLUSTER_NAME=e2e-${PROVISIONER}

function create_cluster {
  "${OSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --image "${IMAGE}" \
    --masters=3 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 4.0 \
    --endpoint "${ENDPOINT}"
}

create_cluster
get_kubeconfig
${KUBECTL} config set-cluster e2e-docker --server https://${ENDPOINT}:6443
# run_talos_integration_test_docker
# run_kubernetes_integration_test
