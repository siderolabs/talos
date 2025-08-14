#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=docker
CLUSTER_NAME=e2e-${PROVISIONER}

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create docker \
    --name="${CLUSTER_NAME}" \
    --kubernetes-version=${KUBERNETES_VERSION} \
    --image="${IMAGE}" \
    --workers=1 \
    --mtu=1430 \
    "${REGISTRY_MIRROR_FLAGS[@]}"

  "${TALOSCTL}" config node 10.5.0.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}" --save-support-archive-path=/tmp/support-${CLUSTER_NAME}.zip
}

trap destroy_cluster SIGINT EXIT

create_cluster
get_kubeconfig
${KUBECTL} config set-cluster e2e-docker --server https://10.5.0.2:6443
run_talos_integration_test_docker
run_kubernetes_integration_test
