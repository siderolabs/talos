#!/bin/bash

source ./hack/test/e2e-runner.sh

# ## Run CIS conformance
# echo "Master CIS Conformance:"
# e2e_run "export KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi
#          kubectl apply -f /e2emanifests/cis-kube-bench-master.yaml
#          kubectl wait --timeout=300s --for=condition=complete job/kube-bench-master > /dev/null
#          kubectl logs job/kube-bench-master"

# echo "Worker CIS Conformance:"
# e2e_run "export KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi
#          kubectl apply -f /e2emanifests/cis-kube-bench-node.yaml
#          kubectl wait --timeout=300s --for=condition=complete job/kube-bench-node > /dev/null
#          kubectl logs job/kube-bench-node"

# Download sonobuoy and run kubernetes conformance
e2e_run "apt-get update && apt-get install wget
         wget --quiet -O /usr/local/bin/sonobuoy http://139.178.69.21:8080/assets/sonobuoy
         chmod 755 /usr/local/bin/sonobuoy
         sonobuoy run --kubeconfig ${KUBECONFIG} \
            --wait \
            --skip-preflight \
            --plugin e2e \
            --plugin-env e2e.E2E_USE_GO_RUNNER=true \
            --kube-conformance-image-version v1.16.0-rc.2 \
            --sonobuoy-image schnake/sonobuoy:testTaint
         results=\$(sonobuoy retrieve --kubeconfig ${KUBECONFIG})
         sonobuoy e2e --kubeconfig ${KUBECONFIG} \$results"

#e2e_run "timeout=\$((\$(date +%s) + 3600))
#         while true; do sleep 60; [[ \$(date +%s) -gt \$timeout ]] && exit; done"
