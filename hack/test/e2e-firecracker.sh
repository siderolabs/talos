#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=firecracker
CLUSTER_NAME=e2e-${PROVISIONER}

function create_cluster {
  "${OSCTL}" cluster create \
    --provisioner "${PROVISIONER}" \
    --name "${CLUSTER_NAME}" \
    --masters=3 \
    --mtu 1500 \
    --memory 2048 \
    --cpus 2.0 \
    --cidr 172.20.0.0/24 \
    --init-node-as-endpoint \
    --wait \
    --install-image docker.io/autonomy/installer:latest
}

create_cluster
get_kubeconfig
run_talos_integration_test
run_kubernetes_integration_test
