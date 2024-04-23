#!/usr/bin/env bash

set -eoux pipefail

INTEGRATION_TEST_FLAGS=()

case "${CI:-false}" in
  true)
    MIRROR_FLAG=()

    for registry in docker.io k8s.gcr.io registry.k8s.io quay.io gcr.io ghcr.io; do
      service="registry-${registry//./-}.ci.svc"
      addr=$(python3 -c "import socket; print(socket.gethostbyname('${service}'))")

      MIRROR_FLAG+=("${registry}=http://${addr}:5000")
    done

    MIRROR_FLAGS="${MIRROR_FLAG[*]}"

    INTEGRATION_TEST_FLAGS+=("-talos.provision.target-installer-registry=${REGISTRY}" "-talos.provision.registry-mirror=${MIRROR_FLAGS// /,}")
    ;;
  *)
    ;;
esac

if [ "${INTEGRATION_TEST_RUN:-undefined}" != "undefined" ]; then
  INTEGRATION_TEST_FLAGS+=("-test.run=${INTEGRATION_TEST_RUN}")
fi

if [ "${INTEGRATION_TEST_TRACK:-undefined}" != "undefined" ]; then
  INTEGRATION_TEST_FLAGS+=("-talos.provision.cidr=172.$(( INTEGRATION_TEST_TRACK + 21 )).0.0/24")
fi

case "${CUSTOM_CNI_URL:-false}" in
  false)
    ;;
  *)
    INTEGRATION_TEST_FLAGS+=("-talos.provision.custom-cni-url=${CUSTOM_CNI_URL}")
    ;;
esac

"${INTEGRATION_TEST}" -test.v \
  -talos.talosctlpath "${TALOSCTL}" \
  -talos.provision.mtu 1430  \
  -talos.provision.cni-bundle-url "${ARTIFACTS}/talosctl-cni-bundle-\${ARCH}.tar.gz" \
  "${INTEGRATION_TEST_FLAGS[@]}"
