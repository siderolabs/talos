#!/bin/bash
set -eou pipefail

PLATFORM=""

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p ${TMP}
cp ${PWD}/hack/test/manifests/provider-components.yaml ${TMP}/provider-components.yaml

## Installs envsubst command
apk add --no-cache gettext

## Template out aws components
## Using a local copy until v0.5.0 of the provider is cut.
export AWS_B64ENCODED_CREDENTIALS=${AWS_SVC_ACCT}
cat ${PWD}/hack/test/manifests/capa-components.yaml| envsubst > ${TMP}/capa-components.yaml

## Template out aws components
## Using a local copy until new release of the provider is cut.
export AZURE_SUBSCRIPTION_ID_B64="$( cat ${TMP}/svc-acct.json | jq -r '.subscriptionId' | base64 -w0 )"
export AZURE_TENANT_ID_B64="$( cat ${TMP}/svc-acct.json | jq -r '.tenantId' | base64 -w0 )"
export AZURE_CLIENT_ID_B64="$( cat ${TMP}/svc-acct.json | jq -r '.clientId' | base64 -w0 )"
export AZURE_CLIENT_SECRET_B64="$( cat ${TMP}/svc-acct.json | jq -r '.clientSecret' | base64 -w0 )"
cat ${PWD}/hack/test/manifests/capz-components.yaml| envsubst > ${TMP}/capz-components.yaml

## Template out gcp components
export GCP_B64ENCODED_CREDENTIALS=${GCE_SVC_ACCT} 
cat ${PWD}/hack/test/manifests/capg-components.yaml| envsubst > ${TMP}/capg-components.yaml
##Until next alpha release, keep a local copy of capg-components.yaml. 
##They've got an incorrect image pull policy.
##curl -L ${CAPG_COMPONENTS} | envsubst > ${TMP}/capg-components.yaml

## Drop in capi stuff
e2e_run "kubectl apply -f ${TMP}/provider-components.yaml"
e2e_run "kubectl apply -f ${CAPI_COMPONENTS}"
e2e_run "kubectl apply -f ${TMP}/capa-components.yaml"
e2e_run "kubectl apply -f ${TMP}/capz-components.yaml"
e2e_run "kubectl apply -f ${TMP}/capg-components.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
         until KUBECONFIG=${TMP}/kubeconfig kubectl wait --timeout=1s --for=condition=Ready -n ${CABPT_NS} pods --all; do
           [[ \$(date +%s) -gt \$timeout ]] && exit 1
           echo 'Waiting to CABPT pod to be available...'
           sleep 10
         done"
