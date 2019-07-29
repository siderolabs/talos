#!/bin/bash

set -eou pipefail

export TMP="/tmp/e2e"

cleanup() {
	rm -rf ${TMP}
}
trap cleanup EXIT

./hack/test/osctl-cluster-create.sh

exit 0
