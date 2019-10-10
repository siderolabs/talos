#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMPPLATFORM}

NAME_PREFIX="talos-e2e-${TAG}-${PLATFORM}"

## Cleanup the platform resources upon any exit
cleanup() {
  e2e_run "KUBECONFIG=${TMP}/kubeconfig kubectl delete machine ${NAME_PREFIX}-master-0 ${NAME_PREFIX}-master-1 ${NAME_PREFIX}-master-2
           KUBECONFIG=${TMP}/kubeconfig kubectl scale machinedeployment ${NAME_PREFIX}-workers --replicas=0
           KUBECONFIG=${TMP}/kubeconfig kubectl delete machinedeployment ${NAME_PREFIX}-workers
           KUBECONFIG=${TMP}/kubeconfig kubectl delete cluster ${NAME_PREFIX}"
}

trap cleanup EXIT

## Download kustomize and template out capi cluster, then deploy it
e2e_run "KUBECONFIG=${TMP}/kubeconfig kubectl apply -f ${TMPPLATFORM}/cluster.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until KUBECONFIG=${TMP}/kubeconfig kubectl get cm -n ${CAPI_NS} ${NAME_PREFIX}-master-0; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
         done
         KUBECONFIG=${TMP}/kubeconfig kubectl get cm -n ${CAPI_NS} ${NAME_PREFIX}-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until /bin/osctl kubeconfig > ${KUBECONFIG}; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
         done"

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
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl wait --timeout=1s --for=condition=ready=true --all nodes > /dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get nodes -o wide
           sleep 10
         done"

## Verify that we have an HA controlplane
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until kubectl get nodes -l node-role.kubernetes.io/master='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           kubectl get nodes -l node-role.kubernetes.io/master=''
           sleep 10
         done"

## Print nodes so we know everything is healthy
echo "E2E setup complete. List of nodes: "
e2e_run "kubectl get nodes -o wide"

## Run conformance tests if var is not null
if [ ${CONFORMANCE:-"dontrun"} == "run" ]; then
  echo "Beginning conformance tests..."
  ./hack/test/conformance.sh
fi
