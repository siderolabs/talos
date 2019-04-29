#!/bin/bash

set -eou pipefail

TMP="$(mktemp -d)"
OSCTL="${PWD}/build/osctl-linux-amd64"

export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"

cleanup() {
	${OSCTL} cluster destroy --name integration
	rm -rf ${TMP}
}

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

trap cleanup EXIT

${OSCTL} cluster create --name integration
${OSCTL} config target 10.5.0.2

run "until osctl kubeconfig > ${KUBECONFIG}; do cat ${KUBECONFIG}; sleep 5; done"
run "until kubectl get nodes -o json | jq '.items | length' | grep 4 >/dev/null; do kubectl get nodes -o wide; sleep 5; done"
# TODO(andrewrynhard): Uncomment the following after upgrading to Kubernetes 1.15.
#run "kubectl apply -f /manifests"
#run "kubectl wait --for=condition=ready=true --all nodes"
#run "kubectl wait --timeout=300s --for=condition=ready=true --all pods -n kube-system"
#run "kubectl wait --timeout=300s --for=condition=available=true --all deployments -n kube-system"

exit 0
