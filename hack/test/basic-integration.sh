#!/bin/bash

set -eou pipefail


TALOS_IMG="docker.io/autonomy/talos:${TAG}"
OSCTL="${PWD}/${ARTIFACTS}/osctl-linux-amd64"
INTEGRATIONTEST="${PWD}/bin/integration-test"
TMP="/tmp/e2e"
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
    LOCALOSCTL="${PWD}/${ARTIFACTS}/osctl-linux-amd64"
    ;;
  Darwin*)
    LOCALOSCTL="${PWD}/${ARTIFACTS}/osctl-darwin-amd64"
    ;;
  *)
    exit 1
    ;;
esac

mkdir -p "${TMP}"

${LOCALOSCTL} cluster create --name integration --image ${TALOS_IMG} --masters=3 --mtu 1440 --cpus 4.0 --wait --endpoint "${ENDPOINT}"

"${INTEGRATIONTEST}" -test.v -talos.osctlpath "${LOCALOSCTL}" -talos.k8sendpoint "${ENDPOINT}:6443"
