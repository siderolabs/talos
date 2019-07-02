#!/bin/bash

set -eou pipefail

## If we take longer than 5m in docker, we're probably boned anyways
TIMEOUT=300

CONTAINER_ID=$(docker ps -f label=io.drone.build.number=${DRONE_BUILD_NUMBER} -f label=io.drone.repo.namespace=${DRONE_REPO_NAMESPACE} -f label=io.drone.repo.name=${DRONE_REPO_NAME} -f label=io.drone.step.name=basic-integration --format='{{ .ID }}')

run() {
	docker run \
	 	--rm \
	 	--interactive \
	 	--net="${DRONE_COMMIT_SHA:0:7}" \
		--entrypoint=bash \
		--volumes-from=${CONTAINER_ID} \
	 	-e KUBECONFIG=${KUBECONFIG} \
	 	-e TALOSCONFIG=${TALOSCONFIG} \
	 	k8s.gcr.io/hyperkube:${KUBERNETES_VERSION} -c "${1}"
}

${OSCTL} cluster create --name "${DRONE_COMMIT_SHA:0:7}"
${OSCTL} config target 10.5.0.2

## Fetch kubeconfig
run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
	 until ${OSCTL} kubeconfig > ${KUBECONFIG}
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
MANIFESTS="${PWD}/hack/dev/manifests"
run "kubectl apply -f ${MANIFESTS}/psp.yaml -f ${MANIFESTS}/flannel.yaml -f ${MANIFESTS}/coredns.yaml"

## Wait for all nodes ready
run "kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

##  Verify that we have an HA controlplane
run "kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 >/dev/null"
