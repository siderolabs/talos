# NB: There is a known bug that causes CRD scaling issues in 1.15 kubectl or later.
export KUBERNETES_VERSION=v1.17.0
export TALOS_IMG="docker.io/autonomy/talos:${TAG}"
export TMP="/tmp/e2e"
export TMPPLATFORM="${TMP}/${PLATFORM}"
export OSCTL="${PWD}/${ARTIFACTS}/osctl-linux-amd64"
export INTEGRATION_TEST="${PWD}/${ARTIFACTS}/integration-test-linux-amd64"
export TALOSCONFIG="${TMPPLATFORM}/talosconfig"
export KUBECONFIG="${TMPPLATFORM}/kubeconfig"

## Long timeout due to provisioning times
export TIMEOUT=9000

## Total number of nodes we'll be waiting to come up (3 Masters, 3 Workers)
export NUM_NODES=6

## ClusterAPI Bootstrap Provider Talos (CABPT)
export CABPT_VERSION="0.1.0-alpha.0"
export CABPT_COMPONENTS="https://github.com/talos-systems/cluster-api-bootstrap-provider-talos/releases/download/v${CABPT_VERSION}/provider-components.yaml"

## ClusterAPI (CAPI)
export CAPI_VERSION="0.2.6"
export CAPI_COMPONENTS="https://github.com/kubernetes-sigs/cluster-api/releases/download/v${CAPI_VERSION}/cluster-api-components.yaml"

## ClusterAPI Provider GCP (CAPG)
export CAPG_VERSION="0.2.0-alpha.2"
export CAPG_COMPONENTS="https://github.com/kubernetes-sigs/cluster-api-provider-gcp/releases/download/v${CAPG_VERSION}/infrastructure-components.yaml"

export KUSTOMIZE_VERSION="3.1.0"
export KUSTOMIZE_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64"
export SONOBUOY_VERSION="0.16.5"
export SONOBUOY_URL="https://github.com/heptio/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz"
export CABPT_NS="cabpt-system"

e2e_run() {
  docker run \
         --rm \
         --interactive \
         --net=basic-integration \
         --entrypoint=/bin/bash \
         --mount type=bind,source=${TMP},target=${TMP} \
         --mount type=bind,source=${PWD}/hack/test/manifests,target=/e2emanifests \
         -v ${OSCTL}:/bin/osctl:ro \
         -v ${INTEGRATION_TEST}:/bin/integration-test:ro \
         -e KUBECONFIG=${KUBECONFIG} \
         -e TALOSCONFIG=${TALOSCONFIG} \
         k8s.gcr.io/hyperkube:${KUBERNETES_VERSION} -c "${1}"
}
