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

if [ "${INTEGRATION_TEST_RUN:-undefined}" != "undefined" ]; then
  INTEGRATION_TEST_FLAGS="${INTEGRATION_TEST_FLAGS} -test.run ${INTEGRATION_TEST_RUN}"
fi

"${INTEGRATION_TEST}" -test.v -talos.talosctlpath "${TALOSCTL}" -talos.provision.mem 2048 -talos.provision.cpu 2 ${INTEGRATION_TEST_FLAGS}
