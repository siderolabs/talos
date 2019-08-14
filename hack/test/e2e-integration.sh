#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p $TMP

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
sed "s/{{TAG}}/${TAG}/" ${PWD}/hack/test/manifests/${PLATFORM}-cluster.yaml > ${TMP}/${PLATFORM}-cluster.yaml

## Download kustomize and template out capi cluster, then deploy it
e2e_run "kubectl apply -f ${TMP}/${PLATFORM}-cluster.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl get cm -n cluster-api-provider-talos-system ${NAME_PREFIX}-master-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done
         kubectl get cm -n cluster-api-provider-talos-system ${NAME_PREFIX}-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}-${PLATFORM}-capi"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until /bin/osctl --talosconfig ${TALOSCONFIG}-${PLATFORM}-capi kubeconfig > ${KUBECONFIG}-${PLATFORM}-capi
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done"

## Wait for the init node to report in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
     until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 1 >/dev/null
	 do
	   if  [[ \$(date +%s) -gt \$timeout ]]
	   then
	     exit 1
	   fi
	   kubectl get nodes -o wide
	   sleep 5
	 done"

##  Apply psp and flannel
e2e_run "KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml"

##  Wait for nodes to check in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -o json | jq '.items | length' | grep ${NUM_NODES} >/dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -o wide
		   sleep 10
		 done"

## Wait for kube-proxy up
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get po -n kube-system -l k8s-app=kube-proxy -o json | jq '.items | length' | grep ${NUM_NODES} > /dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get po -n kube-system -l k8s-app=kube-proxy
		   sleep 10
		 done"

##  Wait for nodes ready
e2e_run "KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

## Verify that we have an HA controlplane
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 > /dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length'
		   sleep 10
		 done"

## Run conformance tests if var is not null
if [ ${CONFORMANCE:-"dontrun"} == "run" ]; then
  echo "Beginning conformance tests..."
  ./hack/test/conformance.sh
fi
