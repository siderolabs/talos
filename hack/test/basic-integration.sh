#!/bin/bash

set -eou pipefail

TMP="/tmp/e2e"
TALOS_IMG="docker.io/autonomy/talos:${TAG}"

export TALOSCONFIG="${TMP}/talosconfig"

case "${CI:-false}" in
  true)
    ENDPOINT="docker"
    ;;
  *)
    ENDPOINT="127.0.0.1"
    ;;
esac

case $(uname -s) in
  Linux*)
    OSCTL="${PWD}/${ARTIFACTS}/osctl-linux-amd64"
    INTEGRATION_TEST="${PWD}/${ARTIFACTS}/integration-test-linux-amd64"
    ;;
  Darwin*)
    OSCTL="${PWD}/${ARTIFACTS}/osctl-darwin-amd64"
    INTEGRATION_TEST="${PWD}/${ARTIFACTS}/integration-test-darwin-amd64"
    ;;
  *)
    exit 1
    ;;
esac

mkdir -p "${TMP}"

case ${PROVISIONER} in
  docker)
    "${OSCTL}" cluster create \
      --provisioner docker \
      --image "${TALOS_IMG}" \
      --name basic-integration \
      --masters=3 \
      --mtu 1500 \
      --memory 2048 \
      --cpus 4.0 \
      --wait \
      --endpoint "${ENDPOINT}"

    "${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}" -talos.k8sendpoint "${ENDPOINT}:6443"

    mkdir -p ${TMP}/${TALOS_PLATFORM}
    "${OSCTL}" kubeconfig ${TMP}/${TALOS_PLATFORM}
    ./hack/test/conformance.sh
    ;;

  firecracker)
    "${OSCTL}" cluster create \
      --provisioner firecracker \
      --name basic-integration \
      --masters=3 \
      --mtu 1500 \
      --memory 2048 \
      --cpus 2.0 \
      --cidr 172.20.0.0/24 \
      --init-node-as-endpoint \
      --wait \
      --install-image docker.io/autonomy/installer:latest

      "${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}"
    ;;

  *)
    echo "unknown provisioner: ${PROVISIONER}"
    exit 1
    ;;
esac
