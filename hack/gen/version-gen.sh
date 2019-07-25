#!/bin/sh

OUTPUT=$1
BUILT=`date -Iseconds`

cat > "$OUTPUT" <<EOF
// Code generated automatically by version-gen.sh DO NOT EDIT.

package version

const Tag="${TAG}"
const SHA="${SHA}"
const Built="${BUILT}"
EOF
