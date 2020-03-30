#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

# We need to override this here since e2e.sh will set it to ${TMP}/capi/kubeconfig.
export KUBECONFIG="/tmp/e2e/docker/kubeconfig"

# CAPI

export CAPI_VERSION="0.2.9"
export CAPI_COMPONENTS="https://github.com/kubernetes-sigs/cluster-api/releases/download/v${CAPI_VERSION}/cluster-api-components.yaml"

# CABPT

export CABPT_NS="cabpt-system"

# Install envsubst
apk add --no-cache gettext

export AWS_B64ENCODED_CREDENTIALS=${AWS_SVC_ACCT}
cat ${PWD}/hack/test/capi/components-capa.yaml| envsubst | ${KUBECTL} apply -f -

export GCP_B64ENCODED_CREDENTIALS=${GCE_SVC_ACCT}
cat ${PWD}/hack/test/capi/components-capg.yaml| envsubst | ${KUBECTL} apply -f -

cat ${PWD}/hack/test/capi/components-provider.yaml | ${KUBECTL} apply -f -
${KUBECTL} apply -f ${CAPI_COMPONENTS}

# Wait for the talosconfig
timeout=$(($(date +%s) + ${TIMEOUT}))
until ${KUBECTL} wait --timeout=1s --for=condition=Ready -n ${CABPT_NS} pods --all; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  echo 'Waiting to CABPT pod to be available...'
  sleep 5
done
