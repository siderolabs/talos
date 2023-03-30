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
export TALOS_VERSION=v1.1

# Kubernetes

export KUBECONFIG="${TMP}/kubeconfig"
export KUBERNETES_VERSION=${KUBERNETES_VERSION:-1.27.0-beta.0}

export NAME_PREFIX="talos-e2e-${SHA}-${PLATFORM}"
export TIMEOUT=1200
export NUM_NODES=6

# default values, overridden by talosctl cluster create tests
PROVISIONER=
CLUSTER_NAME=

cleanup_capi() {
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig delete cluster ${NAME_PREFIX}
}

# Create a cluster via CAPI.
function create_cluster_capi {
  trap cleanup_capi EXIT

  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig apply -f ${TMP}/cluster.yaml

  # Wait for first controlplane machine to have a name
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name=${NAME_PREFIX} --all-namespaces -o json | jq -re '.items[0].metadata.name | select (.!=null)')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
    ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name=${NAME_PREFIX} --all-namespaces
  done

  FIRST_CP_NODE=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -l cluster.x-k8s.io/control-plane,cluster.x-k8s.io/cluster-name=${NAME_PREFIX} --all-namespaces -o json | jq -r '.items[0].metadata.name')

  # Wait for first controlplane machine to have a talosconfig ref
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine ${FIRST_CP_NODE} -o json | jq -re '.spec.bootstrap.configRef.name | select (.!=null)')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  FIRST_CP_TALOSCONFIG=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine ${FIRST_CP_NODE} -o json | jq -re '.spec.bootstrap.configRef.name')

  # Wait for talosconfig in cm then dump it out
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig ${FIRST_CP_TALOSCONFIG} -o jsonpath='{.status.talosConfig}')" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done
  ${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get talosconfig ${FIRST_CP_TALOSCONFIG} -o jsonpath='{.status.talosConfig}' > ${TALOSCONFIG}

  # Wait until we have an IP for first controlplane node
  timeout=$(($(date +%s) + ${TIMEOUT}))
  until [ -n "$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' ${FIRST_CP_NODE})" ]; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    sleep 10
  done

  MASTER_IP=$(${KUBECTL} --kubeconfig /tmp/e2e/docker/kubeconfig get machine -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}' ${FIRST_CP_NODE})
  "${TALOSCTL}" config endpoint "${MASTER_IP}"
  "${TALOSCTL}" config node "${MASTER_IP}"

  # Wait for the kubeconfig from first cp node
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
  until ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
    [[ $(date +%s) -gt $timeout ]] && exit 1
    ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' && :
    sleep 10
  done
}

function run_talos_integration_test {
  case "${SHORT_INTEGRATION_TEST:-no}" in
    yes|true|y)
      TEST_SHORT="-test.short"
      ;;
    *)
      TEST_SHORT=""
      ;;
    esac

  case "${INTEGRATION_TEST_RUN:-no}" in
    no)
      TEST_RUN="-test.run ."
      ;;
    *)
      TEST_RUN="-test.run ${INTEGRATION_TEST_RUN}"
      ;;
    esac

  "${INTEGRATION_TEST}" -test.v -talos.failfast -talos.talosctlpath "${TALOSCTL}" -talos.kubectlpath "${KUBECTL}" -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}" ${TEST_RUN} ${TEST_SHORT}
}

function run_talos_integration_test_docker {
  case "${SHORT_INTEGRATION_TEST:-no}" in
    yes|true|y)
      TEST_SHORT="-test.short"
      ;;
    *)
      TEST_SHORT=""
      ;;
    esac

  case "${INTEGRATION_TEST_RUN:-no}" in
    no)
      TEST_RUN="-test.run ."
      ;;
    *)
      TEST_RUN="-test.run ${INTEGRATION_TEST_RUN}"
      ;;
    esac

  "${INTEGRATION_TEST}" -test.v -talos.talosctlpath "${TALOSCTL}" -talos.kubectlpath "${KUBECTL}" -talos.k8sendpoint 127.0.0.1:6443 -talos.provisioner "${PROVISIONER}" -talos.name "${CLUSTER_NAME}" ${TEST_RUN} ${TEST_SHORT}
}

function run_kubernetes_conformance_test {
  "${TALOSCTL}" conformance kubernetes --mode="${1}"
}

function run_kubernetes_integration_test {
  "${TALOSCTL}" health --run-e2e
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
  rm -f "${TMP}/kubeconfig"
  "${TALOSCTL}" kubeconfig "${TMP}"
}

function dump_cluster_state {
  nodes=$(${KUBECTL} get nodes -o jsonpath="{.items[*].status.addresses[?(@.type == 'InternalIP')].address}" | tr [:space:] ',')
  "${TALOSCTL}" -n ${nodes} services
  ${KUBECTL} get nodes -o wide
  ${KUBECTL} get pods --all-namespaces -o wide
}

function build_registry_mirrors {
  if [[ "${CI:-false}" == "true" ]]; then
    REGISTRY_MIRROR_FLAGS=

    for registry in docker.io registry.k8s.io quay.io gcr.io ghcr.io registry.dev.talos-systems.io; do
      local service="registry-${registry//./-}.ci.svc"
      local addr=`python3 -c "import socket; print(socket.gethostbyname('${service}'))"`

      REGISTRY_MIRROR_FLAGS="${REGISTRY_MIRROR_FLAGS} --registry-mirror ${registry}=http://${addr}:5000"
    done
  else
    # use the value from the environment, if present
    REGISTRY_MIRROR_FLAGS=${REGISTRY_MIRROR_FLAGS:-}
  fi
}

function run_extensions_test {
  # e2e-qemu creates 3 controlplanes
  # use a worker node to test extensions
  "${TALOSCTL}" config node 172.20.1.5

  echo "Testing firmware extensions..."
  ${TALOSCTL} ls /lib/firmware | grep amd-ucode
  ${TALOSCTL} ls /lib/firmware | grep bnx2x
  ${TALOSCTL} ls /lib/firmware | grep i915
  ${TALOSCTL} ls /lib/firmware | grep intel-ucode

  echo "Testing kernel modules tree extension..."
  ${TALOSCTL} get extensions modules.dep
  KERNEL_VERSION=$(${TALOSCTL} get extensions modules.dep -o json | jq -r '.spec.metadata.version')
  ${TALOSCTL} ls /lib/modules/${KERNEL_VERSION}/extras/ | grep gasket
  ${TALOSCTL} read /lib/modules/${KERNEL_VERSION}/modules.dep | grep -E gasket
  ${TALOSCTL} ls /lib/modules/${KERNEL_VERSION}/extras/ | grep drbd
  ${TALOSCTL} read /lib/modules/${KERNEL_VERSION}/modules.dep | grep -E drbd

  echo "Testing iscsi-tools extensions service..."
  ${TALOSCTL} services ext-iscsid | grep -E "STATE\s+Running"
  ${TALOSCTL} services ext-tgtd | grep -E "STATE\s+Running"

  echo "Testing gVsisor..."
  ${KUBECTL} apply -f ${PWD}/hack/test/gvisor/manifest.yaml
  sleep 10
  ${TALOSCTL} read /proc/sys/user/max_user_namespaces
  ${TALOSCTL} get extensions
  ${TALOSCTL} get mc -o yaml
  ${KUBECTL} get pods
  ${KUBECTL} describe pod nginx-gvisor
  ${KUBECTL} wait --for=condition=ready pod nginx-gvisor --timeout=2m

  echo "Testing hello-world extension service..."
  ${TALOSCTL} services ext-hello-world | grep -E "STATE\s+Running"
  curl http://172.20.1.5/ | grep Hello

  # set talosctl config back to the first controlplane
  "${TALOSCTL}" config node 172.20.1.2
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
  KUBERNETES_SERVICE_HOST= KUBECONFIG="${TMP}/kubeconfig" ${KUBESTR} fio --storageclass ceph-block --size 10G
}

function validate_virtio_modules {
  ${TALOSCTL} read /proc/modules | grep -q virtio
}

function install_and_run_cilium_cni_tests {
  get_kubeconfig

  case "${CILIUM_INSTALL_TYPE:-none}" in
    strict)
      ${CILIUM_CLI} install \
        --helm-set=ipam.mode=kubernetes \
        --helm-set=kubeProxyReplacement=strict \
        --helm-set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
        --helm-set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
        --helm-set=cgroup.autoMount.enabled=false \
        --helm-set=cgroup.hostRoot=/sys/fs/cgroup \
        --helm-set=k8sServiceHost=172.20.1.1 \
        --helm-set=k8sServicePort=6443 \
        --wait-duration=10m
      ;;
    *)
      # explicitly setting kubeProxyReplacement=disabled since by the time cilium cli runs talos
      # has not yet applied the kube-proxy manifests
      ${CILIUM_CLI} install \
        --helm-set=ipam.mode=kubernetes \
        --helm-set=kubeProxyReplacement=disabled \
        --helm-set=securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
        --helm-set=securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
        --helm-set=cgroup.autoMount.enabled=false \
        --helm-set=cgroup.hostRoot=/sys/fs/cgroup \
        --wait-duration=10m
      ;;
  esac

  ${CILIUM_CLI} status

  # use kube-system namespace since it doesn't have a PSS restriction
  ${CILIUM_CLI} connectivity test --test-namespace kube-system
}
