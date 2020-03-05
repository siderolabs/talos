# This file contains common environment variables and setup logic for all test
# scripts. It assumes that the following environment variables are set by the
# Makefile:
#  - PLATFORM
#  - TAG
#  - SHA
#  - ARTIFACTS
#  - OSCTL
#  - INTEGRATION_TEST
#  - KUBECTL
#  - SONOBUOY

set -eoux pipefail

TMP="/tmp/e2e/${PLATFORM}"
mkdir -p "${TMP}"

# Talos

export TALOSCONFIG="${TMP}/talosconfig"

# Kubernetes

export KUBECONFIG="${TMP}/kubeconfig"

# Sonobuoy

export SONOBUOY_MODE=${SONOBUOY_MODE:-quick}

export NAME_PREFIX="talos-e2e-${SHA}-${PLATFORM}"
export TIMEOUT=1200
export NUM_NODES=6

# default values, overridden by osctl cluster create tests
PROVISIONER=
CLUSTER_NAME=

cleanup_capi() {
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig delete cluster ${NAME_PREFIX}
}

# Create a cluster via CAPI.
function create_cluster_capi {
  trap cleanup_capi EXIT

  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig apply -f ${TMP}/cluster.yaml

  # Wait for talosconfig in cm then dump it out
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig ${NAME_PREFIX}-controlplane-0 -o jsonpath='{.status.talosConfig}')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig ${NAME_PREFIX}-controlplane-0 -o jsonpath='{.status.talosConfig}' > ${TALOSCONFIG}

  # Wait until we have an IP for master 0
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' ${NAME_PREFIX}-controlplane-0)" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done
  ${OSCTL} config endpoint "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' ${NAME_PREFIX}-controlplane-0)"

  # Wait for the kubeconfig from capi master-0
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until get_kubeconfig; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  # Wait for nodes to check in
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until ${KUBECTL} get nodes -o go-template='{{ len .items }}' | grep ${NUM_NODES} >/dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -o wide && :
    sleep 10
  done

  # Wait for nodes to be ready
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until ${KUBECTL} wait --timeout=1s --for=condition=ready=true --all nodes > /dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -o wide && :
    sleep 10
  done

  # Verify that we have an HA controlplane
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until ${KUBECTL} get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -l node-role.kubernetes.io/master='' && :
    sleep 10
  done
}

function run_talos_integration_test {
  "${INTEGRATION_TEST}" -test.v -talos.failfast -talos.osctlpath "${OSCTL}" -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}"
}

function run_talos_integration_test_docker {
  "${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}" -talos.k8sendpoint ${ENDPOINT}:6443 -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}"
}

function run_kubernetes_integration_test {
  ${SONOBUOY} run \
    --kubeconfig ${KUBECONFIG} \
    --wait \
    --skip-preflight \
    --plugin e2e \
    --mode ${SONOBUOY_MODE}
  ${SONOBUOY} status --kubeconfig ${KUBECONFIG} --json | jq . | tee ${TMP}/sonobuoy-status.json
  if [ $(cat ${TMP}/sonobuoy-status.json | jq -r '.plugins[] | select(.plugin == "e2e") | ."result-status"') != 'passed' ]; then exit 1; fi
}

function run_control_plane_cis_benchmark {
  ${KUBECTL} apply -f ${PWD}/hack/test/cis/kube-bench-master.yaml
  ${KUBECTL} wait --timeout=300s --for=condition=complete job/kube-bench-master > /dev/null
  ${KUBECTL} logs job/kube-bench-master
}

function run_worker_cis_benchmark {
  ${KUBECTL} apply -f ${PWD}/hack/test/cis/kube-bench-node.yaml
  ${KUBECTL} wait --timeout=300s --for=condition=complete job/kube-bench-node > /dev/null
  ${KUBECTL} logs job/kube-bench-node
}

function get_kubeconfig {
  "${OSCTL}" kubeconfig "${TMP}"
}

function dump_cluster_state {
  nodes=$(${KUBECTL} get nodes -o jsonpath="{.items[*].status.addresses[?(@.type == 'InternalIP')].address}" | tr [:space:] ',')
  "${OSCTL}" -n ${nodes} services
  ${KUBECTL} get nodes -o wide
  ${KUBECTL} get pods --all-namespaces -o wide
}
