#!/bin/bash

set -eou pipefail

export KUBERNETES_VERSION=v1.15.2
export TALOS_IMG="docker.io/autonomy/talos:${TAG}"
export TMP="/tmp/e2e"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"
export TIMEOUT=300

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

${OSCTL} cluster create --name integration --image ${TALOS_IMG} --mtu 1440 --cpus 4.0
${OSCTL} config target 10.5.0.2

## Fetch kubeconfig
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until osctl kubeconfig > ${KUBECONFIG}; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       sleep 2
     done"

## Wait for the init node to report in
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until kubectl get node master-1 >/dev/null; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       kubectl get nodes -o wide
       sleep 5
     done"

## Deploy needed manifests
run "kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml -f /manifests/coredns.yaml"

## Wait for all nodes to report in
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until kubectl get nodes -o go-template='{{ len .items }}' | grep 4 >/dev/null; do
       [[ \$(date +%s) -gt \$timeout ]] && exit 1
       kubectl get nodes -o wide
       sleep 5
     done"

## Wait for all nodes ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

##  Verify that we have an HA controlplane
run "kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 >/dev/null"
