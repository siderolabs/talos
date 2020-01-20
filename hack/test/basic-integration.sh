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

"${OSCTL}" cluster create --provisioner firecracker --name basic-integration \
  --masters=3 --mtu 1440 --memory 1536 --cpus 4.0 --cidr 172.20.0.0/24  \
  --init-node-as-endpoint --wait \
   --install-image docker.io/autonomy/installer:v0.4.0-alpha.1 # TODO: fixme (how?)

"${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}"

mkdir -p ${TMP}/${TALOS_PLATFORM}
"${OSCTL}" kubeconfig ${TMP}/${TALOS_PLATFORM}
./hack/test/conformance.sh
