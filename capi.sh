#!/bin/bash

set -ex

TIMEOUT=60

rm -f kubeconfig
talosctl -n 172.20.0.2 kubeconfig
export KUBECONFIG=$PWD/kubeconfig

kubectl taint node talos-default-master-1 node-role.kubernetes.io/master:NoSchedule-

clusterctl init -b talos -c talos -i sidero

timeout=$(($(date +%s) + ${TIMEOUT}))
until kubectl wait --timeout=1s --for=condition=Ready -n sidero-system pods --all; do
  [[ $(date +%s) -gt $timeout ]] && exit 1
  echo 'Waiting to CABPT pod to be available...'
  sleep 5
done

## Update args to use 9091 for port
kubectl patch deploy -n sidero-system sidero-metadata-server --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args", "value": ["--port=9091"]}]'

## Tweak container port to match
kubectl patch deploy -n sidero-system sidero-metadata-server --type='json' -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/ports", "value": [{"containerPort": 9091,"name": "http"}]}]'

## Use host networking
kubectl patch deploy -n sidero-system sidero-metadata-server --type='json' -p='[{"op": "add", "path": "/spec/template/spec/hostNetwork", "value": true}]'

## Update args to specify the api endpoint to use for registration
kubectl patch deploy -n sidero-system sidero-controller-manager --type='json' -p='[{"op": "add", "path": "/spec/template/spec/containers/1/args", "value": ["--api-endpoint=172.20.0.2","--metrics-addr=127.0.0.1:8080","--enable-leader-election"]}]'

## Use host networking
kubectl patch deploy -n sidero-system sidero-controller-manager --type='json' -p='[{"op": "add", "path": "/spec/template/spec/hostNetwork", "value": true}]'
