#!/bin/bash

set -eou pipefail

export TALOS_IMG="docker.io/autonomy/talos:${TAG}"
export TMP="$(mktemp -d)"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"

## ClusterAPI Provider Talos (CAPT)
CAPT_VERSION="0.1.0-alpha.2"
PROVIDER_COMPONENTS="https://github.com/talos-systems/cluster-api-provider-talos/releases/download/v${CAPT_VERSION}/provider-components.yaml"
KUSTOMIZE_VERSION="1.0.11"
KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64"
SONOBUOY_VERSION="0.15.0"
SONOBUOY_URL="https://github.com/heptio/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz"

## Total number of nodes we'll be waiting to come up
NUM_NODES=6
MASTER_IPS=""

## GCE-specific vars
GCE_PROJECT_NAME="talos-testbed"
GCE_IMAGE_NAME="talos-e2e"

## Long timeout due to packet provisioning times
TIMEOUT=9000

e2e_run() {
	docker run \
	 	--rm \
		--interactive \
	 	--net=integration \
		--entrypoint=bash \
		--mount type=bind,source=${TMP},target=${TMP} \
		--mount type=bind,source=${PWD}/hack/dev/manifests,target=/manifests \
		--mount type=bind,source=${PWD}/hack/test/manifests,target=/e2emanifests \
	 	-v ${OSCTL}:/bin/osctl:ro \
	 	-e KUBECONFIG=${KUBECONFIG} \
	 	-e TALOSCONFIG=${TALOSCONFIG} \
	 	k8s.gcr.io/hyperkube:${KUBERNETES_VERSION} -c "${1}"
}

cleanup() {
 e2e_run "kubectl delete machine talos-e2e-master-0 talos-e2e-master-1 talos-e2e-master-2
          kubectl scale machinedeployment talos-e2e-workers --replicas=0
          kubectl delete machinedeployment talos-e2e-workers
          kubectl delete cluster talos-e2e"
 ${OSCTL} cluster destroy --name integration
 rm -rf ${TMP}
}
trap cleanup EXIT

./hack/test/osctl-cluster-create.sh

## Drop in capi stuff
# wget --quiet -O ${PWD}/hack/test/manifests/provider-components.yaml ${PROVIDER_COMPONENTS}
sed -i "s/{{PACKET_AUTH_TOKEN}}/${PACKET_AUTH_TOKEN}/" ${PWD}/hack/test/manifests/provider-components.yaml
sed -i "s#{{GCE_SVC_ACCT}}#${GCE_SVC_ACCT}#" ${PWD}/hack/test/manifests/capi-secrets.yaml
cat ${PWD}/hack/test/manifests/capi-secrets.yaml
e2e_run "kubectl apply -f /e2emanifests/provider-components.yaml -f /e2emanifests/capi-secrets.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl wait --timeout=1s --for=condition=Ready -n cluster-api-provider-talos-system pod/cluster-api-provider-talos-controller-manager-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   echo 'Waiting to CAPT pod to be available...'
		   sleep 10
		 done"

## Download kustomize and template out capi cluster, then deploy it
e2e_run "kubectl apply -f /e2emanifests/gce-cluster.yaml"		   

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl get cm -n cluster-api-provider-talos-system talos-e2e-master-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done
         kubectl get cm -n cluster-api-provider-talos-system talos-e2e-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}-capi"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until /bin/osctl --talosconfig ${TALOSCONFIG}-capi kubeconfig > ${KUBECONFIG}-capi
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done"

##  Wait for nodes to check in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -o json | jq '.items | length' | grep ${NUM_NODES} >/dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -o wide
		   sleep 10
		 done"

##  Apply psp and flannel
e2e_run "KUBECONFIG=${KUBECONFIG}-capi kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml"

##  Wait for nodes ready
e2e_run "KUBECONFIG=${KUBECONFIG}-capi kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

## Verify that we have an HA controlplane
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 > /dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length'
		   sleep 10
		 done"

## Download sonobuoy and run conformance
e2e_run "apt-get update && apt-get install wget
		 wget --quiet -O /tmp/sonobuoy.tar.gz ${SONOBUOY_URL}
		 tar -xf /tmp/sonobuoy.tar.gz -C /usr/local/bin
		 sonobuoy run --kubeconfig ${KUBECONFIG}-capi --wait --skip-preflight --plugin e2e
		 results=\$(sonobuoy retrieve --kubeconfig ${KUBECONFIG}-capi)
		 sonobuoy e2e --kubeconfig ${KUBECONFIG}-capi \$results"

exit 0
