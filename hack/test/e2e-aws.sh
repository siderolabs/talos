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

# Patches with documents the terraform provider can't decode are applied with the in-tree talosctl.
if [ -n "${E2E_AWS_CONFIG_PATCH:-}" ]; then
  PATCH_ARGS=()

  IFS=':' read -r -a PATCHES <<< "${E2E_AWS_CONFIG_PATCH}"

  for patch in "${PATCHES[@]}"; do
    PATCH_ARGS+=("--patch" "${patch}")
  done

  NODES=$(${KUBECTL} get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}')

  ${TALOSCTL} --nodes "${NODES// /,}" patch machineconfig "${PATCH_ARGS[@]}"
fi

run_talos_integration_test

if [ "${SKIP_KUBERNETES_INTEGRATION_TEST:-false}" != "true" ]; then
  run_kubernetes_integration_test
fi
