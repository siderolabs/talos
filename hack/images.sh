#!/usr/bin/env bash

set -e

CRANE="${CRANE:-crane}"
REGISTRY="${REGISTRY:-ghcr.io}"
USERNAME="${USERNAME:-siderolabs}"
TAG="${TAG:-latest}"

IMAGES="installer talos imager"

for image in $IMAGES; do
  ref="${REGISTRY}/${USERNAME}/${image}:${TAG}"
  digest=$(${CRANE} digest ${ref})

  echo "${ref}@${digest}"
done
