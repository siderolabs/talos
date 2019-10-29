#!/bin/bash
set -eou pipefail

PLATFORM=""

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMP}
cp ${PWD}/hack/test/manifests/provider-components.yaml ${TMP}/provider-components.yaml

## Template out gcp components
apk add --no-cache gettext
export GCP_B64ENCODED_CREDENTIALS=${GCE_SVC_ACCT} 
cat ${PWD}/hack/test/manifests/capg-components.yaml| envsubst > ${TMP}/capg-components.yaml
##Until next alpha release, keep a local copy of capg-components.yaml. 
##They've got an incorrect image pull policy.
##curl -L ${CAPG_COMPONENTS} | envsubst > ${TMP}/capg-components.yaml

## Drop in capi stuff
e2e_run "kubectl apply -f ${TMP}/provider-components.yaml"
e2e_run "kubectl apply -f ${CAPI_COMPONENTS}"
e2e_run "kubectl apply -f ${TMP}/capg-components.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until KUBECONFIG=${TMP}/kubeconfig kubectl wait --timeout=1s --for=condition=Ready -n ${CABPT_NS} pods --all; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           echo 'Waiting to CABPT pod to be available...'
           sleep 10
         done"
