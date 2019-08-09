export KUBERNETES_VERSION=v1.14.5
export TALOS_IMG="docker.io/autonomy/talos:${TAG}"
export TMP="/tmp/e2e"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"

## Long timeout due to provisioning times
export TIMEOUT=9000

## Total number of nodes we'll be waiting to come up (3 Masters + 3 Workers)
export NUM_NODES=6

## ClusterAPI Provider Talos (CAPT)
export CAPT_VERSION="0.1.0-alpha.2"
export PROVIDER_COMPONENTS="https://github.com/talos-systems/cluster-api-provider-talos/releases/download/v${CAPT_VERSION}/provider-components.yaml"
export KUSTOMIZE_VERSION="1.0.11"
export KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64"
export SONOBUOY_VERSION="0.15.1"
export SONOBUOY_URL="https://github.com/heptio/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz"

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
