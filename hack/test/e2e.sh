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
#  - SHORT_INTEGRATION_TEST
#  - CUSTOM_CNI_URL
#  - KUBECTL
#  - KUBESTR
#  - HELM
#  - CILIUM_CLI

set -eoux pipefail

TMP="/tmp/e2e/${PLATFORM}"
mkdir -p "${TMP}"

# Talos

export TALOSCONFIG="${TMP}/talosconfig"
TALOS_VERSION=$(cut -d "." -f 1,2 <<< "${TAG}")
export TALOS_VERSION

# Kubernetes

export KUBECONFIG="${TMP}/kubeconfig"
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-1.34.1}

export NAME_PREFIX="talos-e2e-${SHA}-${PLATFORM}"
export TIMEOUT=1200

# default values, overridden by talosctl cluster create tests
PROVISIONER=
CLUSTER_NAME=

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

  "${INTEGRATION_TEST}" \
    -test.v \
    -talos.failfast \
    -talos.talosctlpath "${TALOSCTL}" \
    -talos.kubectlpath "${KUBECTL}" \
    -talos.helmpath "${HELM}" \
    -talos.kubestrpath "${KUBESTR}" \
    -talos.provisioner "${PROVISIONER}" \
    -talos.name "${CLUSTER_NAME}" \
    -talos.image "${REGISTRY}/siderolabs/talos" \
    ${EXTRA_TEST_ARGS:-} \
    "${TEST_RUN[@]}" \
    "${TEST_SHORT[@]}"
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

  "${INTEGRATION_TEST}" \
    -test.v \
    -talos.failfast \
    -talos.talosctlpath "${TALOSCTL}" \
    -talos.kubectlpath "${KUBECTL}" \
    -talos.helmpath "${HELM}" \
    -talos.kubestrpath "${KUBESTR}" \
    -talos.provisioner "${PROVISIONER}" \
    -talos.name "${CLUSTER_NAME}" \
    -talos.image "${REGISTRY}/siderolabs/talos" \
    ${EXTRA_TEST_ARGS:-} \
    "${TEST_RUN[@]}" \
    "${TEST_SHORT[@]}"
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
  if [[ "${REGISTRY_MIRROR_FLAGS:-yes}" == "no" ]]; then
    REGISTRY_MIRROR_FLAGS=()

    return
  fi

  if [[ "${CI:-false}" == "true" ]]; then
    REGISTRY_MIRROR_FLAGS=()

    for registry in docker.io registry.k8s.io quay.io gcr.io ghcr.io; do
      local service="registry-${registry//./-}.ci.svc"
      addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

      REGISTRY_MIRROR_FLAGS+=("--registry-mirror=${registry}=http://${addr}:5000")
    done
  fi
}

function install_and_run_cilium_cni_tests {
  get_kubeconfig

  case "${WITH_KUBESPAN:-false}" in
    true)
      CILIUM_NODE_ENCRYPTION=false
      CILIUM_TEST_EXTRA_ARGS=("--test=!node-to-node-encryption,!check-log-errors,!pod-to-pod-encryption-v2")
      ;;
    *)
      CILIUM_NODE_ENCRYPTION=true
      CILIUM_TEST_EXTRA_ARGS=("--test=!check-log-errors")
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

  # ref: https://github.com/cilium/cilium-cli/releases/tag/v0.16.14
  ${KUBECTL} delete ns --ignore-not-found cilium-test-1

  ${KUBECTL} create ns cilium-test-1
  ${KUBECTL} label ns cilium-test-1 pod-security.kubernetes.io/enforce=privileged

  # --external-target added, as default 'one.one.one.one' is buggy, and CloudFlare status is of course "all healthy"
  ${CILIUM_CLI} connectivity test --test-namespace cilium-test --external-target google.com --timeout=20m "${CILIUM_TEST_EXTRA_ARGS[@]}"; ${KUBECTL} delete ns cilium-test-1
}
