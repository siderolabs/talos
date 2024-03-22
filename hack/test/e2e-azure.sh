#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

cp "${ARTIFACTS}/e2e-azure-talosconfig" "${TALOSCONFIG}"
cp "${ARTIFACTS}/e2e-azure-kubeconfig" "${KUBECONFIG}"

# set the talosconfig to use the first controlplane ip
CONTROLPLANE0_NODE=$(${TALOSCTL} config info -o json | jq -r '.endpoints[0]')
${TALOSCTL} config node "${CONTROLPLANE0_NODE}"

run_talos_integration_test
run_kubernetes_integration_test
