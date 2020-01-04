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

"${OSCTL}" cluster create --name basic-integration --image "${TALOS_IMG}" --masters=3 --mtu 1440 --cpus 4.0 --wait --endpoint "${ENDPOINT}"

trap "${OSCTL} cluster destroy --name basic-integration" EXIT

"${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}" -talos.k8sendpoint "${ENDPOINT}:6443"
