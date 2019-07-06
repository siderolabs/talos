#!/bin/bash

set -eou pipefail

export TMP="$(mktemp -d)"
export OSCTL="${PWD}/out/linux_amd64/osctl-linux-amd64"
export TALOSCONFIG="${TMP}/talosconfig"
export KUBECONFIG="${TMP}/kubeconfig"


cleanup() {
	${OSCTL} cluster destroy --name integration
	rm -rf ${TMP}
}
trap cleanup EXIT
docker load <./out/linux_amd64/talos-amd64.tar
./hack/test/osctl-cluster-create.sh
cd /tmp
${OSCTL} config generate cluster.local 1.2.3.4,2.3.4.5,3.4.5.6

exit 0
