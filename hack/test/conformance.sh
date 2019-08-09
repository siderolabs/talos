#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Run CIS conformance
echo "Master CIS Conformance:"
e2e_run "export KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi
         kubectl apply -f /e2emanifests/cis-kube-bench-master.yaml
         kubectl wait --timeout=300s --for=condition=complete job/kube-bench-master > /dev/null
         kubectl logs job/kube-bench-master"

echo "Worker CIS Conformance:"
e2e_run "export KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi
         kubectl apply -f /e2emanifests/cis-kube-bench-node.yaml
         kubectl wait --timeout=300s --for=condition=complete job/kube-bench-node > /dev/null
         kubectl logs job/kube-bench-node"

# Download sonobuoy and run kubernetes conformance
e2e_run "apt-get update && apt-get install wget
		 wget --quiet -O /tmp/sonobuoy.tar.gz ${SONOBUOY_URL}
		 tar -xf /tmp/sonobuoy.tar.gz -C /usr/local/bin
		 sonobuoy run --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi \
        --wait \
        --skip-preflight \
        --plugin e2e \
        --kube-conformance-image-version latest
		 results=\$(sonobuoy retrieve --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi)
		 sonobuoy e2e --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi \$results"
