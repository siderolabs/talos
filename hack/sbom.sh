#!/bin/bash
set -euo pipefail

# We need to normalize name here too remove spaces, make it lowercase, etc. to be consistent with the image name used in the syft SBOM cataloger.
NORMALIZED_NAME=$(echo "$NAME" | tr '[:upper:]' '[:lower:]' | tr -s ' ' '-')

SYFT_FORMAT_PRETTY=1 SYFT_FORMAT_SPDX_JSON_DETERMINISTIC_UUID=1 \
	go tool \
	github.com/anchore/syft/cmd/syft \
	scan --from dir "$1" \
	--select-catalogers "+sbom-cataloger,go" \
	--source-name "$NORMALIZED_NAME" --source-version "$TAG" \
	-o spdx-json > "/rootfs/usr/share/spdx/$2"
