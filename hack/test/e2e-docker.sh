#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

PROVISIONER=docker
CLUSTER_NAME=e2e-${PROVISIONER}

function create_cluster {
  build_registry_mirrors

  "${TALOSCTL}" cluster create \
    --provisioner="${PROVISIONER}" \
    --name="${CLUSTER_NAME}" \
    --kubernetes-version=${KUBERNETES_VERSION} \
    --image="${IMAGE}" \
    --masters=1 \
    --workers=1 \
    --mtu=1450 \
    --memory=2048 \
    --cpus=2.0 \
    --with-init-node=false \
    --docker-host-ip=127.0.0.1 \
    --endpoint=127.0.0.1 \
    ${REGISTRY_MIRROR_FLAGS} \
    --crashdump

  "${TALOSCTL}" config node 10.5.0.2
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy --name "${CLUSTER_NAME}" --provisioner "${PROVISIONER}"
}

create_cluster
get_kubeconfig
${KUBECTL} config set-cluster e2e-docker --server https://10.5.0.2:6443
run_talos_integration_test_docker
run_kubernetes_integration_test

# Unlike other local e2e tests, we don't destroy the cluster there as it is used by CAPI and AWS/GCP e2e tests later.
# destroy_cluster
