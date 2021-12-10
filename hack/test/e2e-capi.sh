#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

export CAPI_VERSION="${CAPI_VERSION:-1.0.2}"
export CABPT_VERSION="${CABPT_VERSION:-0.5.0}"
export CACPPT_VERSION="${CACPPT_VERSION:-0.4.0}"
export CAPA_VERSION="${CAPA_VERSION:-1.1.0}"
export CAPG_VERSION="${CAPG_VERSION:-1.0.0}"

# We need to override this here since e2e.sh will set it to ${TMP}/capi/kubeconfig.
export KUBECONFIG="/tmp/e2e/docker/kubeconfig"

# CABPT
export CABPT_NS="cabpt-system"

# Install envsubst
apk add --no-cache gettext

# Env vars for cloud accounts
set +x
export GCP_B64ENCODED_CREDENTIALS=${GCE_SVC_ACCT}
export AWS_B64ENCODED_CREDENTIALS=${AWS_SVC_ACCT}
set -x

${CLUSTERCTL} init \
    --core "cluster-api:v${CAPI_VERSION}" \
    --control-plane "talos:v${CACPPT_VERSION}" \
    --infrastructure "aws:v${CAPA_VERSION},gcp:v${CAPG_VERSION}" \
    --bootstrap "talos:v${CABPT_VERSION}"

# Wait for the talosconfig
timeout=$(($(date +%s) + ${TIMEOUT}))
until ${KUBECTL} wait --timeout=1s --for=condition=Ready -n ${CABPT_NS} pods --all; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  echo 'Waiting to CABPT pod to be available...'
  sleep 5
done
