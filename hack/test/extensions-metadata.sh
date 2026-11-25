#!/usr/bin/env bash

# Writes the extensions metadata (the image-digests list) published by the
# siderolabs/extensions CI for the checked out extensions commit, so that test
# runs reuse the extension images built there instead of building them again.
#
# Extensions CI publishes ghcr.io/siderolabs/extensions:<git describe> for every
# push to main. If the checked out commit was not published yet (its CI is still
# running), the closest published first-parent ancestor is used instead.

set -eou pipefail

EXTENSIONS_DIR="${EXTENSIONS_DIR:-_out/extensions}"
EXTENSIONS_IMAGE="${EXTENSIONS_IMAGE:-ghcr.io/siderolabs/extensions}"
EXTENSIONS_LOOKBACK="${EXTENSIONS_LOOKBACK:-20}"

if [[ "$(git -C "${EXTENSIONS_DIR}" rev-parse --is-shallow-repository)" == "true" ]]; then
  git -C "${EXTENSIONS_DIR}" fetch --prune --unshallow --tags
else
  git -C "${EXTENSIONS_DIR}" fetch --prune --tags
fi

HEAD_COMMIT=$(git -C "${EXTENSIONS_DIR}" rev-parse HEAD)
IMAGE_REF=

for commit in $(git -C "${EXTENSIONS_DIR}" rev-list --first-parent --max-count="${EXTENSIONS_LOOKBACK}" HEAD); do
  tag=$(git -C "${EXTENSIONS_DIR}" describe --tag --always --match 'v[0-9]*' "${commit}")

  if crane manifest "${EXTENSIONS_IMAGE}:${tag}" > /dev/null 2>&1; then
    IMAGE_REF="${EXTENSIONS_IMAGE}:${tag}"

    if [[ "${commit}" != "${HEAD_COMMIT}" ]]; then
      echo "::warning::extensions image for HEAD ${HEAD_COMMIT} is not published yet, using ${IMAGE_REF} (commit ${commit})"
    fi

    break
  fi

  echo "extensions image ${EXTENSIONS_IMAGE}:${tag} (commit ${commit}) is not published"
done

if [[ -z "${IMAGE_REF}" ]]; then
  echo "::error::no published extensions image found in the last ${EXTENSIONS_LOOKBACK} commits of ${EXTENSIONS_DIR}"

  exit 1
fi

echo "using extensions image ${IMAGE_REF}"

mkdir -p "${EXTENSIONS_DIR}/_out"
crane export "${IMAGE_REF}" - | tar -xO image-digests > "${EXTENSIONS_DIR}/_out/extensions-metadata"

grep -E '/libvirtd:' "${EXTENSIONS_DIR}/_out/extensions-metadata"
