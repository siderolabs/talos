#!/bin/bash

set -eou pipefail

## If we take longer than 5m in docker, we're probably boned anyways
TIMEOUT=300

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

${OSCTL} cluster create --name integration
${OSCTL} config target 10.5.0.2

## Fetch kubeconfig
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
	 until osctl kubeconfig > ${KUBECONFIG}
	 do
	   if  [[ \$(date +%s) -gt \$timeout ]] 
	   then
	     exit 1
	   fi
	   sleep 2
	 done"

## Wait for all nodes to report in
run "timeout=\$((\$(date +%s) + ${TIMEOUT})) 
     until kubectl get nodes -o json | jq '.items | length' | grep 4 >/dev/null
	 do 
	   if  [[ \$(date +%s) -gt \$timeout ]]
	   then
	     exit 1
	   fi
	   kubectl get nodes -o wide
	   sleep 5
	 done"

## Deploy needed manifests
run "kubectl apply -f /manifests/psp.yaml -f /manifests/calico.yaml -f /manifests/coredns.yaml"

## Wait for all nodes ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

##  Verify that we have an HA controlplane
run "kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 >/dev/null"
