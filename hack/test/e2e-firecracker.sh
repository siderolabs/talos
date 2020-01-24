#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

function create_cluster {
  "${OSCTL}" cluster create \
    --provisioner firecracker \
    --name e2e-firecracker \
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
