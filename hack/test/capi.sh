#!/bin/bash
set -eou pipefail

PLATFORM=""

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMP}

## Drop in capi stuff
sed "s/{{PACKET_AUTH_TOKEN}}/${PACKET_AUTH_TOKEN}/" ${PWD}/hack/test/manifests/provider-components.yaml > ${TMP}/provider-components.yaml

sed -e "s#{{GCE_SVC_ACCT}}#${GCE_SVC_ACCT}#" \
    -e "s#{{AZURE_SVC_ACCT}}#${AZURE_SVC_ACCT}#" \
    -e "s#{{AWS_SVC_ACCT}}#${AWS_SVC_ACCT}#" ${PWD}/hack/test/manifests/capi-secrets.yaml > ${TMP}/capi-secrets.yaml

e2e_run "kubectl apply -f ${TMP}/provider-components.yaml -f ${TMP}/capi-secrets.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         pod='pod/cluster-api-provider-talos-controller-manager-0'
         until KUBECONFIG=${TMP}/kubeconfig kubectl wait --timeout=1s --for=condition=Ready -n ${CAPI_NS} \${pod}; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           echo 'Waiting to CAPT pod to be available...'
           sleep 10
         done"
