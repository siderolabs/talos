#!/bin/bash

docker run --rm -it --network dev_talosbr -v "${PWD}/kubeconfig":/root/.kube/config -v "${PWD}/manifests":/manifests k8s.gcr.io/hyperkube:${HYPERKUBE_TAG:-v1.14.0} kubectl $@
