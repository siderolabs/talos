#!/usr/bin/env bash

set -eou pipefail

source ./hack/test/e2e.sh

cp "${ARTIFACTS}/e2e-aws-talosconfig" "${TALOSCONFIG}"
cp "${ARTIFACTS}/e2e-aws-kubeconfig" "${KUBECONFIG}"

# Wait for nodes to check in
timeout=$(($(date +%s) + TIMEOUT))
until ${KUBECTL} get nodes -o go-template='{{ len .items }}' | grep ${NUM_NODES} >/dev/null; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  ${KUBECTL} get nodes -o wide && :
  sleep 10
done

# Wait for nodes to be ready
timeout=$(($(date +%s) + TIMEOUT))
until ${KUBECTL} wait --timeout=1s --for=condition=ready=true --all nodes > /dev/null; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  ${KUBECTL} get nodes -o wide && :
  sleep 10
done

# Verify that we have an HA controlplane
timeout=$(($(date +%s) + TIMEOUT))
until ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' -o go-template='{{ len .items }}' | grep 3 > /dev/null; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  ${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' && :
  sleep 10
done

CONTROLPLANE0_NODE_NAME=$(${KUBECTL} get nodes -l node-role.kubernetes.io/control-plane='' -o jsonpath='{.items[0].metadata.name}')

# Wait until we have an IP for first controlplane node
timeout=$(($(date +%s) + TIMEOUT))
until [ -n "$(${KUBECTL} get nodes "${CONTROLPLANE0_NODE_NAME}" -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}')" ]; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  sleep 10
done


# lets get the ip of the first controlplane node
CONTROLPLANE0_NODE=$(${KUBECTL} get nodes "${CONTROLPLANE0_NODE_NAME}" -o go-template --template='{{range .status.addresses}}{{if eq .type "ExternalIP"}}{{.address}}{{end}}{{end}}')

# set the talosconfig to use the first controlplane ip
${TALOSCTL} config endpoint "${CONTROLPLANE0_NODE}"
${TALOSCTL} config node "${CONTROLPLANE0_NODE}"

run_talos_integration_test
run_kubernetes_integration_test
