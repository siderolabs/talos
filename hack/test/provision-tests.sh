#!/bin/bash

set -eou pipefail

case "${REGISTRY:-false}" in
  registry.ci.svc:5000)
    REGISTRY_ADDR=`python -c "import socket; print socket.gethostbyname('registry.ci.svc')"`
    INTEGRATION_TEST_FLAGS="-talos.provision.registry-mirror ${REGISTRY}=http://${REGISTRY_ADDR}:5000 -talos.provision.target-installer-registry=${REGISTRY}"
    ;;
  *)
    INTEGRATION_TEST_FLAGS=
    ;;
esac


"${INTEGRATION_TEST}" -test.v -talos.osctlpath "${OSCTL}" -talos.provision.mem 2048 -talos.provision.cpu 2 ${INTEGRATION_TEST_FLAGS}
