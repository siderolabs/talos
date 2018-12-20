#!/bin/bash

set -eou pipefail

NAMESPACE=${NAMESPACE:-"cis-kube-bench"}

cleanup() {
    kubectl delete ns ${NAMESPACE}
}

trap cleanup EXIT

run_master_benchmark() {
    JOB_NAME=kube-bench-master
    kubectl apply -f cis-kube-bench-master.yaml -n ${NAMESPACE}
    kubectl wait --timeout=60s --for=condition=complete job/${JOB_NAME} -n ${NAMESPACE} > /dev/null
    kubectl logs job/${JOB_NAME} -n ${NAMESPACE} | jq . >../build/cis-${JOB_NAME}.json
}

run_node_benchmark() {
    JOB_NAME=kube-bench-node
    kubectl apply -f cis-kube-bench-node.yaml -n ${NAMESPACE}
    kubectl wait --timeout=60s --for=condition=complete job/${JOB_NAME} -n ${NAMESPACE} > /dev/null
    kubectl logs job/${JOB_NAME} -n ${NAMESPACE} | jq . >../build/cis-${JOB_NAME}.json
}

kubectl create ns ${NAMESPACE}

case $1 in
master)
    run_master_benchmark
    ;;
node)
    run_node_benchmark
    ;;
all)
    run_master_benchmark & run_node_benchmark
    wait
    ;;
*)
  ;;
esac
