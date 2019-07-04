#!/bin/bash

set -eou pipefail

export TMP="$(mktemp -d)"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"


cleanup() {
	${OSCTL} cluster destroy --name "${DRONE_COMMIT_SHA:0:7}"
	rm -rf ${TMP}
}
trap cleanup EXIT

./hack/test/osctl-cluster-create.sh

${OSCTL} config generate cluster.local 1.2.3.4,2.3.4.5,3.4.5.6

exit 0
