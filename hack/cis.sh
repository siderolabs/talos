#!/bin/bash

set -eou pipefail

SPEC_VERSION=${SPEC_VERSION:-"1.8"}
IMAGE=${IMAGE:-"aquasec/kube-bench:latest"}

cleanup() {
    kubectl delete pod ${POD_NAME} > /dev/null
}

trap cleanup EXIT

case $1 in
master)
    POD_NAME="kube-bench-master"
    kubectl run ${POD_NAME} --image=${IMAGE} --restart=Never --overrides="{ \"apiVersion\": \"v1\", \"spec\": { \"hostPID\": true, \"nodeSelector\": { \"node-role.kubernetes.io/master\": \"\" }, \"tolerations\": [ { \"key\": \"node-role.kubernetes.io/master\", \"operator\": \"Exists\", \"effect\": \"NoSchedule\" } ] } }" -- master --json --version ${SPEC_VERSION} > /dev/null
    sleep 5
    kubectl logs ${POD_NAME}
    ;;
node)
    POD_NAME="kube-bench-node"
    kubectl run ${POD_NAME} --image=${IMAGE} --restart=Never --overrides="{ \"apiVersion\": \"v1\", \"spec\": { \"hostPID\": true } }" -- node --json --version ${SPEC_VERSION} > /dev/null
    sleep 5
    kubectl logs ${POD_NAME}
    ;;
*)
  ;;
esac
