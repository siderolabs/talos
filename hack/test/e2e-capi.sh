#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

export CABPT_VERSION="0.2.0-alpha.0"
export CAPA_VERSION="0.5.2"

# We need to override this here since e2e.sh will set it to ${TMP}/capi/kubeconfig.
export KUBECONFIG="/tmp/e2e/docker/kubeconfig"

# CABPT
export CABPT_NS="cabpt-system"

# Install envsubst
apk add --no-cache gettext

# Env vars for cloud accounts
export GCP_B64ENCODED_CREDENTIALS=${GCE_SVC_ACCT}
export AWS_B64ENCODED_CREDENTIALS=${AWS_SVC_ACCT}

cat << EOF > /tmp/e2e/clusterctl.yaml
providers:
  - name: "talos"
    url: "https://github.com/talos-systems/cluster-api-bootstrap-provider-talos/releases/latest/bootstrap-components.yaml"
    type: "BootstrapProvider"
EOF

${CLUSTERCTL} init \
    --config /tmp/e2e/clusterctl.yaml \
    --control-plane "-" \
    --infrastructure "aws:v${CAPA_VERSION}" \
    --bootstrap "talos:v${CABPT_VERSION}"

cat ${PWD}/hack/test/capi/components-capg.yaml| envsubst | ${KUBECTL} apply -f -

# Wait for the talosconfig
timeout=$(($(date +%s) + ${TIMEOUT}))
until ${KUBECTL} wait --timeout=1s --for=condition=Ready -n ${CABPT_NS} pods --all; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  echo 'Waiting to CABPT pod to be available...'
  sleep 5
done
