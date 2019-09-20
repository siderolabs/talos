#!/bin/bash

# Example: kubernetes-1.16.0
KUBE_TAG="${1}"

export GOPROXY=https://proxy.golang.org

TMP=$(mktemp -d)

cd ${TMP}

trap "rm -rf ${TMP}" EXIT

for PKG in k8s.io/api k8s.io/apiextensions-apiserver k8s.io/apimachinery k8s.io/apiserver k8s.io/cli-runtime k8s.io/client-go k8s.io/cloud-provider k8s.io/cluster-bootstrap k8s.io/code-generator k8s.io/component-base k8s.io/cri-api k8s.io/csi-translation-lib k8s.io/kube-aggregator k8s.io/kube-controller-manager k8s.io/kube-proxy k8s.io/kube-scheduler k8s.io/kubectl k8s.io/kubelet k8s.io/legacy-cloud-providers k8s.io/metrics k8s.io/sample-apiserver;
do
  rm go.mod go.sum
  go mod init wtf > /dev/null 2>&1

  go get $PKG@$KUBE_TAG > /dev/null 2>&1

  GREP=$( cat go.mod | grep $PKG | wc -l )
  if [ $GREP -gt 0 ]
  then
    GREPOUT="$( cat go.mod | grep $PKG )"
    echo "$PKG => $GREPOUT"
  else
    echo ""
  fi
done
