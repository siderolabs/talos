#!/bin/bash

docker run \
	--rm \
	-it \
	--network dev_talosbr \
	--volumes-from ${VOLUMES_FROM:-master-1}:ro \
	-v "${PWD}/../../build/osctl-linux-amd64:/bin/osctl:ro" \
	-v "${PWD}/talosconfig:/root/.talos/config" \
	-v "${PWD}/kubeconfig":/root/.kube/config \
	-v "${PWD}/manifests":/manifests \
	k8s.gcr.io/hyperkube:${HYPERKUBE_TAG:-v1.14.0} bash
