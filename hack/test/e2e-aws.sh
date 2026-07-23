#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

cp "${ARTIFACTS}/e2e-aws-talosconfig" "${TALOSCONFIG}"
cp "${ARTIFACTS}/e2e-aws-kubeconfig" "${KUBECONFIG}"

# set the talosconfig to use the first controlplane ip
CONTROLPLANE0_NODE=$(${TALOSCTL} config info -o json | jq -r '.endpoints[0]')
${TALOSCTL} config node "${CONTROLPLANE0_NODE}"

# Terraform waits for the Talos API, but AWS can publish the Classic ELB DNS
# record a little later. The integration tests use this kubeconfig directly.
consecutive_successes=0

for _ in {1..60}; do
  if ${KUBECTL} version --request-timeout=5s >/dev/null 2>&1; then
    consecutive_successes=$((consecutive_successes + 1))

    if [ "${consecutive_successes}" -eq 3 ]; then
      break
    fi
  else
    consecutive_successes=0
  fi

  sleep 2
done

if [ "${consecutive_successes}" -ne 3 ]; then
  echo "Kubernetes API did not become stable" >&2

  exit 1
fi

run_talos_integration_test
run_kubernetes_integration_test
