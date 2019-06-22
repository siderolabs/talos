#!/bin/bash

set -eou pipefail

export TMP="$(mktemp -d)"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"

## ClusterAPI Provider Talos (CAPT)
CAPT_VERSION="0.1.0-alpha.1"
PROVIDER_COMPONENTS="https://github.com/talos-systems/cluster-api-provider-talos/releases/download/v${CAPT_VERSION}/provider-components.yaml"
KUSTOMIZE_VERSION="1.0.11"
KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64"
SONOBUOY_VERSION="0.14.3"
SONOBUOY_URL="https://github.com/heptio/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz"

## Total number of nodes we'll be waiting to come up
NUM_NODES=4
MASTER_IPS="139.178.69.76" #,139.178.69.77,139.178.69.78"

## Long timeout due to packet provisioning times
TIMEOUT=900

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
 e2e_run "kubectl delete machine talos-test-cluster-master-0"
 e2e_run "kubectl scale machinedeployment talos-test-cluster-workers --replicas=0"
 e2e_run "kubectl delete machinedeployment talos-test-cluster-workers"
 e2e_run "kubectl delete cluster talos-test-cluster"

 ${OSCTL} cluster destroy --name integration
 rm -rf ${TMP}
}
trap cleanup EXIT

./hack/test/osctl-docker-create.sh

## Drop in capi stuff
wget -O ${PWD}/hack/test/manifests/provider-components.yaml ${PROVIDER_COMPONENTS}
sed -i "s/{{PACKET_AUTH_TOKEN}}/${PACKET_AUTH_TOKEN}/" ${PWD}/hack/test/manifests/provider-components.yaml
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
		   sleep 5
		 done"

## Wait for cluster-api-provider-talos-controller-manager-0 to be ready
e2e_run "kubectl wait --timeout=${TIMEOUT}s --for=condition=Ready -n cluster-api-provider-talos-system pod/cluster-api-provider-talos-controller-manager-0"

## Create cluster and create machines in packet
## TODO: Accept list of IPs as env var for the master-ips bit.
git clone --branch v${CAPT_VERSION} https://github.com/talos-systems/cluster-api-provider-talos.git ${TMP}/cluster-api-provider-talos
sed -i "s/\[x.x.x.x, y.y.y.y, z.z.z.z\]/\[${MASTER_IPS}\]/" ${TMP}/cluster-api-provider-talos/config/samples/cluster-deployment/packet/master-ips.yaml
sed -i "s/{{PROJECT_ID}}/${PACKET_PROJECT_ID}/g; s/{{PXE_SERVER}}/${PACKET_PXE_SERVER}/g;" ${TMP}/cluster-api-provider-talos/config/samples/cluster-deployment/packet/platform-config-*.yaml

## Download kustomize and template out capi cluster, then deploy it
e2e_run "apt-get update && apt-get install wget
		 wget -O /usr/local/bin/kustomize ${KUSTOMIZE_URL}
  	     chmod +x /usr/local/bin/kustomize
         kustomize build ${TMP}/cluster-api-provider-talos/config/samples/cluster-deployment/packet > /e2emanifests/packet-cluster.yaml
		 kubectl apply -f /e2emanifests/packet-cluster.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]] 
		   then
		     exit 1
		   fi
		   sleep 5
		 done"

e2e_run "kubectl get cm -n cluster-api-provider-talos-system talos-test-cluster-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}-capi"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until /bin/osctl --talosconfig ${TALOSCONFIG}-capi kubeconfig > ${KUBECONFIG}-capi
		 do 
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 5
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
		   sleep 5
		 done"

##  Apply psp and flannel
e2e_run "KUBECONFIG=${KUBECONFIG}-capi kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml"

##  Wait for nodes ready
e2e_run "KUBECONFIG=${KUBECONFIG}-capi kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

# ## Verify that we have an HA controlplane
# e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
# 		 until KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 > /dev/null
# 		 do 
# 		   if  [[ \$(date +%s) -gt \$timeout ]]
# 		   then
# 			exit 1
# 		   fi
# 		   KUBECONFIG=${KUBECONFIG}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length'
# 		   sleep 5
# 		 done"

## Download sonobuoy and run conformance
e2e_run "apt-get update && apt-get install wget
		 wget -O /tmp/sonobuoy.tar.gz ${SONOBUOY_URL}
		 tar -xvf /tmp/sonobuoy.tar.gz -C /usr/local/bin
		 sonobuoy run --kubeconfig ${KUBECONFIG}-capi --wait --skip-preflight --kube-conformance-image-version v1.14.3 --plugin e2e"

exit 0
