#!/usr/bin/env bash

# This file contains common environment variables and setup logic for all test
# scripts. It assumes that the following environment variables are set by the
# Makefile:
#  - PLATFORM
#  - TAG
#  - SHA
#  - REGISTRY
#  - IMAGE
#  - INSTALLER_IMAGE
#  - ARTIFACTS
#  - TALOSCTL
#  - INTEGRATION_TEST
#  - MODULE_SIG_VERIFY
#  - KERNEL_MODULE_SIGNING_PUBLIC_KEY
#  - SHORT_INTEGRATION_TEST
#  - CUSTOM_CNI_URL
#  - KUBECTL
#  - KUBESTR
#  - HELM
#  - CLUSTERCTL
#  - CILIUM_CLI

#
# Some environment variables set in this file (e. g. TALOS_VERSION and KUBERNETES_VERSION)
# are referenced by https://github.com/siderolabs/cluster-api-templates.
# See other e2e-*.sh scripts.

set -eoux pipefail

TMP="/tmp/e2e/${PLATFORM}"
mkdir -p "${TMP}"

# Talos

export TALOSCONFIG="${TMP}/talosconfig"
TALOS_VERSION=$(cut -d "." -f 1,2 <<< "${TAG}")
export TALOS_VERSION

# Kubernetes

export KUBECONFIG="${TMP}/kubeconfig"
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-1.30.0}

export NAME_PREFIX="talos-e2e-${SHA}-${PLATFORM}"
export TIMEOUT=1200
export NUM_NODES=${TEST_NUM_NODES:-6}

# default values, overridden by talosctl cluster create tests
PROVISIONER=
CLUSTER_NAME=

cleanup_capi() {
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig delete cluster "${NAME_PREFIX}"
}

# Create a cluster via CAPI.
function create_cluster_capi {
  trap cleanup_capi EXIT

  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig apply -f "${TMP}/cluster.yaml"

  # Wait for first controlplane machine to have a name
  timeout=$(($(date +%s) + TIMEOUT))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name="${NAME_PREFIX}" --all-namespaces -o json | jq -re '.items[0].metadata.name | select (.!=null)')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
    ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name="${NAME_PREFIX}" --all-namespaces
  done

  FIRST_CP_NODE=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name="${NAME_PREFIX}" --all-namespaces -o json | jq -r '.items[0].metadata.name')

  # Wait for first controlplane machine to have a talosconfig ref
  timeout=$(($(date +%s) + TIMEOUT))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine "${FIRST_CP_NODE}" -o json | jq -re '.spec.bootstrap.configRef.name | select (.!=null)')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  FIRST_CP_TALOSCONFIG=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine "${FIRST_CP_NODE}" -o json | jq -re '.spec.bootstrap.configRef.name')

  # Wait for talosconfig in cm then dump it out
  timeout=$(($(date +%s) + TIMEOUT))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig "${FIRST_CP_TALOSCONFIG}" -o jsonpath='{.status.talosConfig}')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig "${FIRST_CP_TALOSCONFIG}" -o jsonpath='{.status.talosConfig}' > "${TALOSCONFIG}"

  # Wait until we have an IP for first controlplane node
  timeout=$(($(date +%s) + TIMEOUT))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' "${FIRST_CP_NODE}")" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  MASTER_IP=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' "${FIRST_CP_NODE}")
  "${TALOSCTL}" config endpoint "${MASTER_IP}"
  "${TALOSCTL}" config node "${MASTER_IP}"

  # Wait for the kubeconfig from first cp node
  timeout=$(($(date +%s) + TIMEOUT))
  until get_kubeconfig; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  # Wait for nodes to check in
  timeout=$(($(date +%s) + TIMEOUT))
  until ${KUBECTL} get nodes -o go-template='{{ len .items }}' | grep ${NUM_NODES} >/dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -o wide && :
    sleep 10
  done

  # Wait for nodes to be ready
  timeout=$(($(date +%s) + TIMEOUT))
  until ${KUBECTL} wait --timeout=1s --for=condition=ready=true --all nodes > /dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -o wide && :
    sleep 10
  done

  # Verify that we have an HA controlplane
  timeout=$(($(date +%s) + TIMEOUT))
  until ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' && :
    sleep 10
  done
}

TEST_SHORT=()
TEST_RUN=("-test.run" ".")

function run_talos_integration_test {
  case "${SHORT_INTEGRATION_TEST:-no}" in
    no)
      ;;
    *)
      TEST_SHORT=("-test.short")
      ;;
  esac

  case "${INTEGRATION_TEST_RUN:-no}" in
    no)
      ;;
    *)
      TEST_RUN=("-test.run" "${INTEGRATION_TEST_RUN}")
      ;;
  esac

  "${INTEGRATION_TEST}" -test.v -talos.failfast -talos.talosctlpath "${TALOSCTL}" -talos.kubectlpath "${KUBECTL}" -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}" -talos.image "${REGISTRY}/siderolabs/talos" "${EXTRA_TEST_ARGS[@]}" "${TEST_RUN[@]}" "${TEST_SHORT[@]}"
}

function run_talos_integration_test_docker {
  case "${SHORT_INTEGRATION_TEST:-no}" in
    no)
      ;;
    *)
      TEST_SHORT=("-test.short")
      ;;
  esac

  case "${INTEGRATION_TEST_RUN:-no}" in
    no)
      ;;
    *)
      TEST_RUN=("-test.run" "${INTEGRATION_TEST_RUN}")
      ;;
  esac

  "${INTEGRATION_TEST}" -test.v -talos.talosctlpath "${TALOSCTL}" -talos.kubectlpath "${KUBECTL}" -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}" -talos.image "${REGISTRY}/siderolabs/talos" "${EXTRA_TEST_ARGS[@]}" "${TEST_RUN[@]}" "${TEST_SHORT[@]}"
}

function run_kubernetes_conformance_test {
  "${TALOSCTL}" conformance kubernetes --mode="${1}"
}

function run_kubernetes_integration_test {
  "${TALOSCTL}" health --run-e2e
}

function run_control_plane_cis_benchmark {
  ${KUBECTL} apply -f "${PWD}/hack/test/cis/kube-bench-master.yaml"
  ${KUBECTL} wait --timeout=300s --for=condition=complete job/kube-bench-master > /dev/null
  ${KUBECTL} logs job/kube-bench-master
}

function run_worker_cis_benchmark {
  ${KUBECTL} apply -f "${PWD}/hack/test/cis/kube-bench-node.yaml"
  ${KUBECTL} wait --timeout=300s --for=condition=complete job/kube-bench-node > /dev/null
  ${KUBECTL} logs job/kube-bench-node
}

function get_kubeconfig {
  rm -f "${TMP}/kubeconfig"
  "${TALOSCTL}" kubeconfig "${TMP}"
}

function dump_cluster_state {
  nodes=$(${KUBECTL} get nodes -o jsonpath="{.items[*].status.addresses[?(@.type == 'InternalIP')].address}" | tr '[:space:]' ',')
  "${TALOSCTL}" -n "${nodes}" services
  ${KUBECTL} get nodes -o wide
  ${KUBECTL} get pods --all-namespaces -o wide
}

function build_registry_mirrors {
  if [[ "${CI:-false}" == "true" ]]; then
    REGISTRY_MIRROR_FLAGS=()

    for registry in docker.io registry.k8s.io quay.io gcr.io ghcr.io registry.dev.talos-systems.io; do
      local service="registry-${registry//./-}.ci.svc"
      addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

      REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${registry}=http://${addr}:5000")
    done
  else
    # use the value from the environment, if present
    REGISTRY_MIRROR_FLAGS=(${REGISTRY_MIRROR_FLAGS:-})
  fi
}

function run_csi_tests {
  ${HELM} repo add rook-release https://charts.rook.io/release
  ${HELM} repo update
  ${HELM} upgrade --install --version=v1.8.2 --set=pspEnable=false --create-namespace --namespace rook-ceph rook-ceph rook-release/rook-ceph
  ${HELM} upgrade --install --version=v1.8.2 --set=pspEnable=false --create-namespace --namespace rook-ceph rook-ceph-cluster rook-release/rook-ceph-cluster

  ${KUBECTL} label ns rook-ceph pod-security.kubernetes.io/enforce=privileged
  # wait for the controller to populate the status field
  sleep 30
  ${KUBECTL} --namespace rook-ceph wait --timeout=900s --for=jsonpath='{.status.phase}=Ready' cephclusters.ceph.rook.io/rook-ceph
  ${KUBECTL} --namespace rook-ceph wait --timeout=900s --for=jsonpath='{.status.state}=Created' cephclusters.ceph.rook.io/rook-ceph
  # .status.ceph is populated later only
  sleep 60
  ${KUBECTL} --namespace rook-ceph wait --timeout=900s --for=jsonpath='{.status.ceph.health}=HEALTH_OK' cephclusters.ceph.rook.io/rook-ceph
  # hack until https://github.com/kastenhq/kubestr/issues/101 is addressed
  KUBERNETES_SERVICE_HOST="" KUBECONFIG="${TMP}/kubeconfig" "${KUBESTR}" fio --storageclass ceph-block --size 10G
}

function install_and_run_cilium_cni_tests {
  get_kubeconfig

  case "${WITH_KUBESPAN:-false}" in
    true)
      CILIUM_NODE_ENCRYPTION=no
      CILIUM_TEST_EXTRA_ARGS=("--test="!node-to-node-encryption"")
      ;;
    *)
      CILIUM_NODE_ENCRYPTION=yes
      CILIUM_TEST_EXTRA_ARGS=()
      ;;
  esac

  case "${CILIUM_INSTALL_TYPE:-none}" in
    strict)
      ${CILIUM_CLI} install \
        --set=ipam.mode=kubernetes \
        --set=kubeProxyReplacement=true \
        --set=encryption.nodeEncryption=${CILIUM_NODE_ENCRYPTION} \
        --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
        --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
        --set=cgroup.autoMount.enabled=false \
        --set=cgroup.hostRoot=/sys/fs/cgroup \
        --set=k8sServiceHost=localhost \
        --set=k8sServicePort=13336
      ;;
    *)
      # explicitly setting kubeProxyReplacement=disabled since by the time cilium cli runs talos
      # has not yet applied the kube-proxy manifests
      ${CILIUM_CLI} install \
        --set=ipam.mode=kubernetes \
        --set=kubeProxyReplacement=false \
        --set=encryption.nodeEncryption=${CILIUM_NODE_ENCRYPTION} \
        --set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
        --set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
        --set=cgroup.autoMount.enabled=false \
        --set=cgroup.hostRoot=/sys/fs/cgroup
      ;;
  esac

  ${CILIUM_CLI} status --wait --wait-duration=10m

  ${KUBECTL} delete ns --ignore-not-found cilium-test

  ${KUBECTL} create ns cilium-test
  ${KUBECTL} label ns cilium-test pod-security.kubernetes.io/enforce=privileged

  # --external-target added, as default 'one.one.one.one' is buggy, and CloudFlare status is of course "all healthy"
  ${CILIUM_CLI} connectivity test --test-namespace cilium-test --external-target google.com "${CILIUM_TEST_EXTRA_ARGS[@]}"; ${KUBECTL} delete ns cilium-test
}
