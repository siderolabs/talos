#!/bin/bash

set -eou pipefail

cleanup() {
    sonobuoy delete > /dev/null
}

trap cleanup EXIT

usage() {
    echo "$0 [conformance|quick]"
}

wait_for_results() {
    kubectl wait --timeout=60s --for=condition=ready pod/sonobuoy -n heptio-sonobuoy
    while sonobuoy status | grep 'Sonobuoy is still running'; do sleep 10; done
    # Sleep in order to avoid 'error retrieving results: error: tmp/sonobuoy no such file or directory'
    sleep 60
    sonobuoy retrieve ../../../build
}

if [ "$#" -ne 1 ]; then
    trap - EXIT
    usage
    exit 1
fi

case $1 in
conformance)
    kubectl apply -f ./sonobuoy-conformance.yaml
    wait_for_results
    ;;
quick)
    kubectl apply -f ./sonobuoy-quick.yaml
    wait_for_results
    ;;
esac
