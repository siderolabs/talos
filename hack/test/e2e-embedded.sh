#!/usr/bin/env bash

set -eoux pipefail

source ./hack/test/e2e.sh

PROVISIONER=qemu
CLUSTER_NAME=e2e-embedded

NODE="172.20.3.2"

function build_iso_embedded {
  # build the ISO with embedded config
  cp hack/test/patches/watchdog.yaml ${ARTIFACTS}/embedded.yaml
  make image-iso IMAGER_ARGS="--embedded-config-path=/out/embedded.yaml" PLATFORM=linux/amd64
}

function create_cluster {
  "${TALOSCTL}" cluster create \
    --provisioner="${PROVISIONER}" \
    --name="${CLUSTER_NAME}" \
    --iso-path=${ARTIFACTS}/metal-amd64.iso \
    --controlplanes=1 \
    --workers=0 \
    --mtu=1430 \
    --memory=2048 \
    --cpus=2.0 \
    --cidr=172.20.3.0/24 \
    --skip-injecting-config \
    --wait=false \
    --cni-bundle-url=${ARTIFACTS}/talosctl-cni-bundle-'${ARCH}'.tar.gz
}

function destroy_cluster() {
  "${TALOSCTL}" cluster destroy \
    --name "${CLUSTER_NAME}" \
    --provisioner "${PROVISIONER}" \
    --save-cluster-logs-archive-path="/tmp/logs-${CLUSTER_NAME}.tar.gz" \
    --save-support-archive-path="/tmp/support-${CLUSTER_NAME}.zip"
}

trap destroy_cluster SIGINT EXIT

build_iso_embedded
create_cluster

# wait for the Talos API to be up
for i in $(seq 1 30); do
  if "${TALOSCTL}" -n "${NODE}" get disks --insecure &>/dev/null; then
    break
  fi
  sleep 1
done

# verify that the config is applied
"${TALOSCTL}" -n "${NODE}" get watchdogtimerstatus timer --insecure
