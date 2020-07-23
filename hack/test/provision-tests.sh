#!/bin/bash

set -eou pipefail

case "${CI:-false}" in
  true)
    REGISTRY="127.0.0.1:5000"
    REGISTRY_ADDR=`docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' registry`
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
