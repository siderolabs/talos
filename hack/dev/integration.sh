#!/bin/bash

set -eou pipefail

{
    sleep 300
    echo "Timed out waiting for integration tests"
    kill $$
} &

until ./osctl.sh kubeconfig > kubeconfig; do cat kubeconfig; sleep 5; done
until ./kubectl.sh get nodes -o json | jq '.items | length' | grep 4; do ./kubectl.sh get nodes; sleep 5; done

exit 0
