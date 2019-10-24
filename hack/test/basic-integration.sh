#!/bin/bash

set -eou pipefail


export KUBERNETES_VERSION=v1.16.0
export TALOS_IMG="docker.io/autonomy/talos:${TAG}"
export TMP="/tmp/e2e"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"
export TIMEOUT=300
export OSCTL="${PWD}/build/osctl-linux-amd64"

case $(uname -s) in
  Linux*)
    export LOCALOSCTL="${PWD}/build/osctl-linux-amd64"
    ;;
  Darwin*)
    export LOCALOSCTL="${PWD}/build/osctl-darwin-amd64"
    ;;
  *)
    exit 1
    ;;
esac

## Create tmp dir
mkdir -p ${TMP}

run() {
  docker run \
         --rm \
         --interactive \
         --net=integration \
         --entrypoint=bash \
         --mount type=bind,source=${TMP},target=${TMP} \
         --mount type=bind,source=${PWD}/hack/dev/manifests,target=/manifests \
         -v ${OSCTL}:/bin/osctl:ro \
         -e KUBECONFIG=${KUBECONFIG} \
         -e TALOSCONFIG=${TALOSCONFIG} \
         k8s.gcr.io/hyperkube:${KUBERNETES_VERSION} -c "${1}"
}

${LOCALOSCTL} cluster create --name integration --image ${TALOS_IMG} --masters=3 --mtu 1440 --cpus 4.0
${LOCALOSCTL} config target 10.5.0.2

## Wait for bootkube to finish successfully.
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until osctl service bootkube | grep Finished >/dev/null; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       osctl service bootkube
       sleep 5
     done"

## Fetch kubeconfig
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until osctl kubeconfig > ${KUBECONFIG}; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       sleep 2
     done"

run "kubectl --kubeconfig ${KUBECONFIG} config set-cluster local --server https://10.5.0.2:6443"

## Wait for all nodes to report in
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until kubectl get nodes -o go-template='{{ len .items }}' | grep 4 >/dev/null; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       kubectl get nodes -o wide
       sleep 5
     done"

## Wait for all nodes ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

## Verify that we have an HA controlplane
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 >/dev/null; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       kubectl get nodes -o wide -l node-role.kubernetes.io/master=''
       sleep 5
     done"

# Wait for kube-proxy to report ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true pod -l k8s-app=kube-proxy -n kube-system"

# Wait for DNS addon to report ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true pod -l k8s-app=kube-dns -n kube-system"

run "osctl config target 10.5.0.2 && osctl -t 10.5.0.2 service etcd | grep Running"
run "osctl config target 10.5.0.3 && osctl -t 10.5.0.3 service etcd | grep Running"
run "osctl config target 10.5.0.4 && osctl -t 10.5.0.4 service etcd | grep Running"
run "osctl --target 10.5.0.2,10.5.0.3,10.5.0.4,10.5.0.5 containers"
run "osctl --target 10.5.0.2,10.5.0.3,10.5.0.4,10.5.0.5 services"
