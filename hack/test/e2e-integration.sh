#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMPPLATFORM}

NAME_PREFIX="talos-e2e-${SHA}-${PLATFORM}"

## Cleanup the platform resources upon any exit
cleanup() {
  e2e_run "KUBECONFIG=${TMP}/kubeconfig kubectl delete cluster ${NAME_PREFIX}"
}

trap cleanup EXIT

## Download kustomize and template out capi cluster, then deploy it
e2e_run "KUBECONFIG=${TMP}/kubeconfig kubectl apply -f ${TMPPLATFORM}/cluster.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until [ -n \"\${STATUS_TALOSCONFIG}\" ]; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
           STATUS_TALOSCONFIG=\$( KUBECONFIG=${TMP}/kubeconfig kubectl get talosconfig ${NAME_PREFIX}-controlplane-0 -o jsonpath='{.status.talosConfig}' ) 
         done
         echo \"\${STATUS_TALOSCONFIG}\" > ${TALOSCONFIG}"

## Wait until we have an IP for master 0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until [ -n \"\${MASTER_0_IP}\" ]; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           sleep 10
           MASTER_0_IP=\$( KUBECONFIG=${TMP}/kubeconfig kubectl get machine -o go-template --template='{{range .status.addresses}}{{if eq .type \"ExternalIP\"}}{{.address}}{{end}}{{end}}' ${NAME_PREFIX}-controlplane-0 )
         done
         echo \${MASTER_0_IP} > ${TMP}/master0ip"

## Target master 0 for osctl
e2e_run "MASTER_0_IP=\$( cat ${TMP}/master0ip )
         /bin/osctl config target \${MASTER_0_IP}"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until /bin/osctl kubeconfig ${TMPPLATFORM}; do
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
