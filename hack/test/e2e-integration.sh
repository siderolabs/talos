#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMPPLATFORM}

NAME_PREFIX="talos-e2e-${TAG}-${PLATFORM}"

## Cleanup the platform resources upon any exit
cleanup() {
 e2e_run "kubectl delete machine ${NAME_PREFIX}-master-0 ${NAME_PREFIX}-master-1 ${NAME_PREFIX}-master-2
          kubectl scale machinedeployment ${NAME_PREFIX}-workers --replicas=0
          kubectl delete machinedeployment ${NAME_PREFIX}-workers
          kubectl delete cluster ${NAME_PREFIX}"
}

trap cleanup EXIT

## Setup the cluster YAML.
sed "s/{{TAG}}/${TAG}/" ${PWD}/hack/test/manifests/${PLATFORM}-cluster.yaml > ${TMPPLATFORM}/cluster.yaml

## Download kustomize and template out capi cluster, then deploy it
e2e_run "kubectl apply -f ${TMPPLATFORM}/cluster.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until KUBECONFIG=${TMP}/kubeconfig kubectl get cm -n ${CAPI_NS} ${NAME_PREFIX}-master-0; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
         done
         kubectl get cm -n ${CAPI_NS} ${NAME_PREFIX}-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until /bin/osctl kubeconfig > ${KUBECONFIG}; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
         done"

## Wait for the init node to report in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 1 >/dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get nodes -o wide
           sleep 5
         done"

##  Apply psp and flannel
e2e_run "kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml"

##  Wait for nodes to check in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl get nodes -o go-template='{{ len .items }}' | grep ${NUM_NODES} >/dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get nodes -o wide
           sleep 10
         done"

## Wait for kube-proxy up
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl get po -n kube-system -l k8s-app=kube-proxy -o go-template='{{ len .items }}' | grep ${NUM_NODES} > /dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get po -n kube-system -l k8s-app=kube-proxy
           sleep 10
         done"

##  Wait for nodes ready
e2e_run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

## Verify that we have an HA controlplane
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get nodes -l node-role.kubernetes.io/master=''
           sleep 10
         done"

## Run conformance tests if var is not null
if [ ${CONFORMANCE:-"dontrun"} == "run" ]; then
  echo "Beginning conformance tests..."
  ./hack/test/conformance.sh
fi
