#!/usr/bin/env bash
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

set -euo pipefail

TALOS_PKGS=$(cat pkg/machinery/gendata/data/pkgs)
TALOS_TOOLS=$(cat pkg/machinery/gendata/data/tools)
TAG=$(cat pkg/machinery/gendata/data/tag)

if [[ "$TAG" =~ -(alpha|beta) ]]; then
	EXTENSIONS_BRANCH="main"
else
	MINOR=$(echo "$TAG" | sed -E 's/^v([0-9]+\.[0-9]+)\..*/\1/')
	EXTENSIONS_BRANCH="release-${MINOR}"
fi

echo "TAG=${TAG}, checking against extensions branch ${EXTENSIONS_BRANCH}"
echo "Talos: PKGS=${TALOS_PKGS} TOOLS=${TALOS_TOOLS}"

EXT_MAKEFILE=$(gh api "repos/siderolabs/extensions/contents/Makefile?ref=${EXTENSIONS_BRANCH}" -H "Accept: application/vnd.github.raw+json")

EXT_PKGS=$(awk '/^PKGS \?= /{print $3; exit}' <<< "$EXT_MAKEFILE")
EXT_TOOLS=$(awk '/^TOOLS \?= /{print $3; exit}' <<< "$EXT_MAKEFILE")

echo "Extensions (${EXTENSIONS_BRANCH}): PKGS=${EXT_PKGS} TOOLS=${EXT_TOOLS}"

FAIL=0

if [ "$TALOS_PKGS" != "$EXT_PKGS" ]; then
	echo "ERROR: PKGS mismatch: talos=${TALOS_PKGS} extensions=${EXT_PKGS}"
	FAIL=1
fi

if [ "$TALOS_TOOLS" != "$EXT_TOOLS" ]; then
	echo "ERROR: TOOLS mismatch: talos=${TALOS_TOOLS} extensions=${EXT_TOOLS}"
	FAIL=1
fi

if [ "${FAIL}" -eq 1 ]; then
	echo "Extensions metadata is out of sync with talos. Update the Makefile in siderolabs/extensions@${EXTENSIONS_BRANCH}."
	exit 1
fi

echo "OK: extensions metadata is in sync with talos."
