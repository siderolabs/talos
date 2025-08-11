#!/bin/bash
set -euo pipefail

SYFT_FORMAT_PRETTY=1 SYFT_FORMAT_SPDX_JSON_DETERMINISTIC_UUID=1 \
	go tool \
	github.com/anchore/syft/cmd/syft \
	scan --from dir "$1" \
	--select-catalogers "+sbom-cataloger,go" \
	--source-name "$NAME" --source-version "$TAG" \
	-o spdx-json > "/rootfs/usr/share/spdx/$2"
