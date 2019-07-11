#!/bin/bash

set -eou pipefail

export TMP="$(mktemp -d)"
export OSCTL="${PWD}/build/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"


cleanup() {
	${OSCTL} cluster destroy --name integration
	rm -rf ${TMP}
}
trap cleanup EXIT

export TALOS_IMG="docker.io/autonomy/talos:$(./bin/gitmeta image tag)"
./hack/test/osctl-cluster-create.sh

exit 0
